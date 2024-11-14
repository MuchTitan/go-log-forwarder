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

	"github.com/mitchellh/mapstructure"
)

// Tail represents a file tailing system that monitors files matching a glob pattern
// and sends their content through a channel. It supports multiple files and maintains
// their reading state persistently.
type Tail struct {
	ctx             context.Context
	logger          *slog.Logger
	files           *sync.Map
	fileEventCH     chan string
	readIsRunning   *sync.Map
	sendChan        chan util.Event
	cancel          context.CancelFunc
	globalWg        *sync.WaitGroup
	db              *sql.DB
	stateUpdateChan chan stateUpdate
	FilenameKey     string `mapstructure:"FilenameKey"`
	Glob            string `mapstructure:"Glob"`
	InputTag        string `mapstructure:"Tag"`
}

type stateUpdate struct {
	path  string
	state TailFileState
}

type TailFileState struct {
	path         string
	checksum     []byte
	seekOffset   int64
	lastSendLine int64
	iNodeNumber  uint64
}

// ParseTail creates a new Tail instance from a configuration map.
// It initializes the necessary channels, maps, and file watcher.
// Returns an error if the configuration is invalid or watcher creation fails.
func ParseTail(input map[string]interface{}, logger *slog.Logger) (Tail, error) {
	tail := Tail{
		logger:          logger,
		files:           &sync.Map{},
		sendChan:        make(chan util.Event),
		fileEventCH:     make(chan string, 1000),
		readIsRunning:   &sync.Map{},
		stateUpdateChan: make(chan stateUpdate, 1000),
		globalWg:        &sync.WaitGroup{},
	}
	err := mapstructure.Decode(input, &tail)
	if err != nil {
		return tail, err
	}

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
	defer t.globalWg.Done()

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
	t.setFileReadingState(path, false)
	// Send state update to be processed in batch
	select {
	case t.stateUpdateChan <- stateUpdate{path: path, state: state}:
	default:
		t.logger.Warn("State update channel full, skipping persistence")
	}
}

func (t *Tail) sendFileEvent(path string) {
	select {
	case t.fileEventCH <- path:
	default:
		t.logger.Warn("file event overflow")
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

func (t *Tail) buildMetadata(state TailFileState) map[string]interface{} {
	output := make(map[string]interface{})

	if t.FilenameKey != "" {
		output[t.FilenameKey] = state.path
	}

	return output
}

func (t *Tail) fileStatLoop() {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	defer t.globalWg.Done()
	initialStats := make(map[string]os.FileInfo)

	matches, err := filepath.Glob(t.Glob)
	if err != nil {
		panic(err)
	}
	for _, path := range matches {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			initialStats[absPath] = info
			t.sendFileEvent(absPath)
		}
	}

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			matches, err := filepath.Glob(t.Glob)
			if err != nil {
				t.logger.Error("coundnt get files for glob", "error", err)
				continue
			}
			for _, path := range matches {
				absPath, err := filepath.Abs(path)
				if err != nil {
					continue
				}
				info, err := os.Stat(absPath)
				if err != nil {
					continue
				}
				if !info.IsDir() {
					prevInfo, exists := initialStats[absPath]
					if !exists || !prevInfo.ModTime().Equal(info.ModTime()) {
						initialStats[absPath] = info
						t.sendFileEvent(absPath)
					}
				}
			}
		}
	}
}

func (t *Tail) setFileReadingState(path string, state bool) {
	t.readIsRunning.Store(path, state)
}

func (t *Tail) isFileReading(path string) bool {
	if state, exists := t.readIsRunning.Load(path); exists {
		return state.(bool)
	}
	return false
}

func (t *Tail) HandleFileEvent(path string) {
	defer t.globalWg.Done()
	if t.isFileReading(path) {
		return
	}
	t.setFileReadingState(path, true)

	currINodeNum, err := util.GetInodeNumber(path)
	if err != nil {
		t.logger.Error("Cant get INodeNum", "error", err, "path", path)
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

// Start begins monitoring files that match the configured glob pattern.
// It starts multiple goroutines to:
// - Watch for file system events
// - Persist file states
// - Monitor goroutine count
// - Process initial files
func (t Tail) Start() {
	t.logger.Info("Starting file tail", "glob", t.Glob)
	t.globalWg.Add(2)
	go func() {
		go t.persistStates()
		go t.fileStatLoop()
		for {
			select {
			case <-t.ctx.Done():
				return
			case path, ok := <-t.fileEventCH:
				if !ok {
					return
				}
				t.globalWg.Add(1)
				go t.HandleFileEvent(path)
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
	t.logger.Info("Stopping file tail", "glob", t.Glob)
	if t.ctx != nil {
		t.cancel()
	}
	t.globalWg.Wait()
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
