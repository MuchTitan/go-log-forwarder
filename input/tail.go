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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
)

// Tail represents a file tailing system that monitors files matching a glob pattern
// and sends their content through a channel. It supports multiple files and maintains
// their reading state persistently.
type Tail struct {
	Glob            string    `mapstructure:"Glob"`
	InputTag        string    `mapstructure:"Tag"`
	FilenameKey     string    `mapstructure:"FilenameKey"`
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

type debounceState struct {
	timer  *time.Timer
	cancel context.CancelFunc
}

// ParseTail creates a new Tail instance from a configuration map.
// It initializes the necessary channels, maps, and file watcher.
// Returns an error if the configuration is invalid or watcher creation fails.
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

// persistStates handles the batch persistence of file states to the database.
// It runs in a separate goroutine and processes state updates in batches
// to optimize database operations.
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

// GetTailFileStateFromDB retrieves the saved state of a file from the database.
// Returns the state and true if found, or an empty state and false if not found.
// Parameters:
//   - path: The file path to look up
//   - inodeNum: The inode number of the file for verification
func (t *Tail) getTailFileStateFromDB(path string, inodeNum uint64) (TailFileState, bool) {
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

// readFile reads from a file starting at the given state's seek offset.
// It sends each line through the sendChan and updates the file state.
// Returns the updated state and any error encountered.
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
			Metadata: t.buildMetadata(state),
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

// handleNewFileTail processes a new file to be tailed.
// It either recovers the previous state from the database or creates a new state
// if the file hasn't been seen before or has changed.
func (t *Tail) handleNewFileTail(path string) {
	currINodeNum, err := util.GetInodeNumber(path)
	if err != nil {
		t.logger.Error("Coundnt get InodeNum", "error", err, "path", path)
		return
	}
	if savedState, exists := t.getTailFileStateFromDB(path, currINodeNum); exists {
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

func (t *Tail) handleWriteEventWithDebounce(event *fsnotify.Event) {
	const debounceDuration = 200 * time.Millisecond

	// Cancel any existing debounce operation for this file
	if stateVal, exists := t.timers.Load(event.Name); exists {
		state := stateVal.(*debounceState)
		state.cancel() // Cancel previous goroutine
		state.timer.Stop()
	}

	// Create new timer and context for this debounce operation
	timer := time.NewTimer(debounceDuration)
	ctx, cancel := context.WithCancel(t.ctx)

	t.timers.Store(event.Name, &debounceState{
		timer:  timer,
		cancel: cancel,
	})

	go func() {
		defer cancel() // Clean up when done
		select {
		case <-timer.C:
			t.wg.Add(1)
			t.handleWriteEvent(event)
			t.timers.Delete(event.Name)
		case <-ctx.Done():
			return
		}
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

func (t *Tail) buildMetadata(state TailFileState) map[string]interface{} {
	output := make(map[string]interface{})

	if t.FilenameKey != "" {
		output[t.FilenameKey] = state.path
	}

	return output
}

// Start begins monitoring files that match the configured glob pattern.
// It starts multiple goroutines to:
// - Watch for file system events
// - Persist file states
// - Monitor goroutine count
// - Process initial files
func (t Tail) Start() {
	t.logger.Info("Starting file tail", "glob", t.Glob)
	go func() {
		rootPath := filepath.Dir(t.Glob)
		err := t.watcher.Add(rootPath)
		if err != nil {
			t.logger.Error("error while adding root path to watcher", "error", err, "rootPath", rootPath)
			return
		}

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
					t.handleWriteEventWithDebounce(&event)
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

// Stop gracefully shuts down the tail operation.
// It cancels the context, closes the watcher, and waits for all goroutines to finish.
func (t Tail) Stop() {
	if t.ctx != nil {
		t.cancel()
	}
	t.watcher.Close()
	t.wg.Wait()
	close(t.sendChan)
	close(t.stateUpdateChan)
}

// GetTag returns the configured input tag or "*" if none is set.
func (t Tail) GetTag() string {
	if t.InputTag == "" {
		return "*"
	}
	return t.InputTag
}
