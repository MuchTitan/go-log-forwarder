package input

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log-forwarder-client/database"
	"log-forwarder-client/util"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
)

type Tail struct {
	Glob            string    `mapstructure:"Glob"`
	InputTag        string    `mapstructure:"Tag"`
	files           *sync.Map // Thread-safe map for file states
	timers          *sync.Map
	watcher         *fsnotify.Watcher
	logger          *slog.Logger
	sendChan        chan util.Event // chan for all writes from the file tails
	ctx             context.Context
	cancel          context.CancelFunc
	wg              *sync.WaitGroup
	db              *sql.DB
	stateUpdateChan chan stateUpdate
}

type stateUpdate struct {
	path  string
	state TailFileState
}

type TailFileState struct {
	path         string
	seekOffset   int64
	lastSendLine int64
	checksum     []byte
	iNodeNumber  uint64
}

func (t *Tail) logGoroutineCount() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.logger.Info("Current goroutine count", "count", runtime.NumGoroutine())
		}
	}
}

func ParseTail(input map[string]interface{}, logger *slog.Logger) (Tail, error) {
	tail := Tail{
		logger:          logger,
		files:           &sync.Map{},
		timers:          &sync.Map{},
		sendChan:        make(chan util.Event),
		stateUpdateChan: make(chan stateUpdate, 1000),
		wg:              &sync.WaitGroup{},
	}
	err := mapstructure.Decode(input, &tail)
	if err != nil {
		return tail, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return tail, err
	}

	tail.watcher = watcher

	if tail.Glob == "" {
		return tail, fmt.Errorf("No Glob provided in tail input")
	}

	if _, err := filepath.Glob(tail.Glob); err != nil {
		return tail, fmt.Errorf("Malformed Glob provided in tail input")
	}

	tail.ctx, tail.cancel = context.WithCancel(context.Background())
	tail.db = database.GetDB()

	return tail, nil
}

func (t *Tail) persistStates() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer t.wg.Done()

	var updates []stateUpdate
	for {
		select {
		case <-t.ctx.Done():
			return
		case update := <-t.stateUpdateChan:
			updates = append(updates, update)
		case <-ticker.C:
			if len(updates) == 0 {
				continue
			}

			// Begin transaction
			tx, err := t.db.Begin()
			if err != nil {
				t.logger.Error("Failed to begin transaction", "error", err)
				continue
			}

			stmt, err := tx.Prepare(`
				INSERT OR REPLACE INTO tail_file_state (filepath, seek_offset, checksum, last_send_line, inode_number)
				VALUES (?, ?, ?, ?, ?)
			`)
			if err != nil {
				t.logger.Error("Failed to prepare statement", "error", err)
				tx.Rollback()
				continue
			}

			for _, update := range updates {
				_, err := stmt.Exec(
					update.path,
					update.state.seekOffset,
					update.state.checksum,
					update.state.lastSendLine,
					update.state.iNodeNumber,
				)
				if err != nil {
					t.logger.Error("Failed to execute statement", "error", err)
					continue
				}
			}

			stmt.Close()
			err = tx.Commit()
			if err != nil {
				t.logger.Error("Failed to commit transaction", "error", err)
				tx.Rollback()
				continue
			}

			updates = updates[:0] // Clear the slice
		}
	}
}

func (t *Tail) GetTailFileStateFromDB(path string, inodeNum uint64) (TailFileState, bool) {
	var state TailFileState
	err := t.db.QueryRow(`
		SELECT filepath, seek_offset,checksum, last_send_line, inode_number
		FROM tail_file_state
        WHERE filepath = ? AND inode_number = ?
	`, path, inodeNum).Scan(&state.path, &state.seekOffset, &state.checksum, &state.lastSendLine, &state.iNodeNumber)

	if err == sql.ErrNoRows {
		return TailFileState{}, false
	}
	if err != nil {
		t.logger.Error("Failed to query file state", "error", err)
		return TailFileState{}, false
	}

	return state, true
}

func (t *Tail) updateFileState(path string, state TailFileState) {
	t.files.Store(path, state)
	// Send state update to be processed in batch
	select {
	case t.stateUpdateChan <- stateUpdate{path: path, state: state}:
	default:
		t.logger.Warn("State update channel full, skipping persistence")
	}
}

func isPathDirectory(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return stat.IsDir(), nil
}

func (t *Tail) readFile(path string, state TailFileState) (TailFileState, error) {
	file, err := os.Open(path)
	if err != nil {
		return state, fmt.Errorf("error while opening file: %v", err)
	}
	defer file.Close()

	// Get current file size
	fileInfo, err := file.Stat()
	if err != nil {
		return state, fmt.Errorf("error getting file stats: %v", err)
	}

	// If we're at EOF and the file has been truncated, reset to beginning
	if state.seekOffset > fileInfo.Size() {
		state.seekOffset = 0
	}

	// Seek to the saved offset
	file.Seek(state.seekOffset, io.SeekStart)

	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				currOffset, _ := file.Seek(0, io.SeekCurrent)
				state.seekOffset = currOffset
				return state, nil
			}
			t.logger.Error("cant read from file", "filepath", path)
		}

		state.lastSendLine++

		line = strings.TrimSpace(line)

		if len(line) < 1 {
			continue
		}

		t.sendChan <- util.Event{
			RawData:  []byte(line),
			InputTag: t.GetTag(),
			Time:     time.Now().Unix(),
		}
	}
}

func CheckIfPathIsStillTheSameFile(path string, currState TailFileState) bool {
	newFileINodeNumber, _ := util.GetInodeNumber(path)
	newCheckSum, _ := util.CreateChecksumForFirstLine(path)

	return (newFileINodeNumber == currState.iNodeNumber) && slices.Equal(newCheckSum, currState.checksum)
}

func newFileTailState(path string) TailFileState {
	inodeNum, _ := util.GetInodeNumber(path)
	checksum, _ := util.CreateChecksumForFirstLine(path)
	return TailFileState{
		path:        path,
		checksum:    checksum,
		iNodeNumber: inodeNum,
	}
}

func (t *Tail) handleNewFileTail(path string) {
	currINodeNum, err := util.GetInodeNumber(path)
	if err != nil {
		t.logger.Error("Coundnt get InodeNum", "error", err, "path", path)
		return
	}
	if savedState, exists := t.GetTailFileStateFromDB(path, currINodeNum); exists {
		// Verify if the file is still the same
		if CheckIfPathIsStillTheSameFile(path, savedState) {
			newState, err := t.readFile(path, savedState)
			if err != nil {
				t.logger.Error("Couldn't read from file with saved state", "error", err)
				return
			}
			t.updateFileState(path, newState)
			return
		}
	}

	// If no saved state or file changed, start fresh
	newState := newFileTailState(path)
	newState, err = t.readFile(path, newState)
	if err != nil {
		t.logger.Error("Couldn't read from file", "error", err)
	}
	t.updateFileState(path, newState)
}

func (t *Tail) handleWriteEvent(event *fsnotify.Event) {
	defer t.wg.Done()
	if stateVal, found := t.files.Load(event.Name); found {
		state := stateVal.(TailFileState)
		if !CheckIfPathIsStillTheSameFile(event.Name, state) {
			t.handleNewFileTail(event.Name)
			return
		}

		newState, err := t.readFile(event.Name, state)
		if err != nil {
			t.logger.Error("Couldn't read from file", "error", err, "path", event.Name)
			return
		}
		t.updateFileState(event.Name, newState)
	} else {
		t.handleNewFileTail(event.Name)
	}
}

func (t *Tail) HandleWriteEventWithDebounce(event *fsnotify.Event) {
	const debounceDuration = 200 * time.Millisecond

	// Retrieve or create a timer for this file
	timerVal, exists := t.timers.Load(event.Name)
	var timer *time.Timer
	if exists {
		timer = timerVal.(*time.Timer)
	} else {
		timer = time.NewTimer(debounceDuration)
		timer.Stop()
		t.timers.Store(event.Name, timer)
	}

	timer.Reset(debounceDuration)

	// Add a goroutine to execute the write event handler once the debounce duration expires
	go func() {
		<-timer.C

		t.wg.Add(1)
		t.handleWriteEvent(event)
	}()
}

func (t *Tail) handleRemoveEvent(event *fsnotify.Event) {
	defer t.wg.Done()
	// Stop the timer associated with this file if it exists
	if timerVal, found := t.timers.Load(event.Name); found {
		timer := timerVal.(*time.Timer)
		timer.Stop()
		t.timers.Delete(event.Name) // Remove the timer from the map
	}

	t.files.Delete(event.Name)
}

func (t *Tail) handleCreateEvent(event *fsnotify.Event) {
	isDir, err := isPathDirectory(event.Name)
	if err != nil {
		t.logger.Warn("Couldn't check if filepath is directory in tail", "error", err, "path", event.Name)
		return
	}

	// Check if the directory matches the glob pattern
	matchesGlob, err := filepath.Match(t.Glob, event.Name)
	if err != nil {
		t.logger.Warn("Error matching glob pattern", "error", err, "pattern", t.Glob, "path", event.Name)
		return
	}

	if !isDir && matchesGlob {
		err = t.watcher.Add(event.Name)
		if err != nil {
			t.logger.Error("Failed to add file to watcher", "error", err, "path", event.Name)
		}
	}
}

func (t *Tail) readInitialFiles() {
	defer t.wg.Done()
	matches, _ := filepath.Glob(t.Glob)
	for _, match := range matches {
		select {
		case <-t.ctx.Done():
			return
		default:
			t.handleNewFileTail(match)
		}
	}
}

func (t Tail) Start() {
	t.logger.Info("Starting file tail", "glob", t.Glob)
	go func() {
		rootPath := filepath.Dir(t.Glob)
		err := t.watcher.Add(rootPath)
		if err != nil {
			t.logger.Error("error while adding root path to watcher", "error", err, "rootPath", rootPath)
			return
		}

		go t.logGoroutineCount()
		t.wg.Add(2)
		go t.persistStates()
		t.readInitialFiles()
		for {
			select {
			case <-t.ctx.Done():
				return
			case event, ok := <-t.watcher.Events:
				if !ok {
					return
				}

				switch {
				case event.Has(fsnotify.Remove):
					t.wg.Add(1)
					t.handleRemoveEvent(&event)
				case event.Has(fsnotify.Create):
					t.handleCreateEvent(&event)
				case event.Has(fsnotify.Write):
					t.HandleWriteEventWithDebounce(&event)
				}

			case err, ok := <-t.watcher.Errors:
				if !ok {
					return
				}

				if err == nil {
					continue
				}
				if err == fsnotify.ErrEventOverflow {
					t.logger.Warn("watcher has a event overflow")
					continue
				}
				t.logger.Error("watcher error", "error", err)
			}
		}
	}()
}

func (t Tail) Read() <-chan util.Event {
	return t.sendChan
}

func (t Tail) Stop() {
	if t.ctx != nil {
		t.cancel()
	}
	t.watcher.Close()
	t.wg.Wait()
	close(t.sendChan)
	close(t.stateUpdateChan)
}

func (t Tail) GetTag() string {
	if t.InputTag == "" {
		return "*"
	}
	return t.InputTag
}
