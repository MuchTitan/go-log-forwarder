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
	"time"

	"github.com/fsnotify/fsnotify"
)

// TailFile holds the file and necessary channels for tailing
type TailFile struct {
	filePath   string
	logger     *slog.Logger
	file       *os.File
	watcher    *fsnotify.Watcher
	ctx        context.Context
	cancel     context.CancelFunc
	doneCh     chan struct{}
	sendCh     chan util.Event
	offset     int64 // Holds the current file offset
	startLine  int64 // The line number to start reading from
	lineNumber int64 // Tracks the current line number
	db         *sql.DB
}

type Tail struct {
	glob         string
	runningTails map[string]*TailFile
	logger       *slog.Logger
	sendChan     chan util.Event // chan for all writes from the file tails
	ctx          context.Context
	cancel       context.CancelFunc
	db           *sql.DB
}

type TailFileState struct {
	LastSendLine int64
	Checksum     []byte
	InodeNumber  uint64
}

func NewTail(glob string, logger *slog.Logger) (Tail, error) {
	ctx, cancel := context.WithCancel(context.Background())
	tail := Tail{
		glob:         glob,
		logger:       logger,
		runningTails: make(map[string]*TailFile),
		sendChan:     make(chan util.Event),
		ctx:          ctx,
		cancel:       cancel,
		db:           database.GetDB(),
	}
	return tail, nil
}

// NewTailFile creates a new TailFile instance starting from a specific line
func NewTailFile(filePath string, sendCh chan util.Event, logger *slog.Logger, parentCtx context.Context, db *sql.DB) (*TailFile, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Initialize fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize watcher: %w", err)
	}

	// Add the file to the watcher
	err = watcher.Add(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to watch file: %w", err)
	}

	// Create ctx for this FileTail
	ctx, cancel := context.WithCancel(parentCtx)

	return &TailFile{
		filePath:   filePath,
		file:       file,
		logger:     logger,
		watcher:    watcher,
		doneCh:     make(chan struct{}),
		sendCh:     sendCh,
		ctx:        ctx,
		cancel:     cancel,
		lineNumber: 0,
		db:         db,
	}, nil
}

func (tf *TailFile) SaveTailFileStateToDB() error {
	state, err := tf.GetTailFileState()
	if err != nil {
		tf.logger.Warn("coundnt retrieve state", "error", err, "path", tf.filePath)
		return err
	}
	_, err = tf.db.Exec(`
        INSERT OR REPLACE INTO tail_file_state (filepath, last_send_line, checksum, inode_number)
        VALUES (?, ?, ?, ?)`,
		tf.filePath, state.LastSendLine, state.Checksum, state.InodeNumber)
	return err
}

func (tf *TailFile) GetTailFileStateFromDB() (TailFileState, error) {
	var state TailFileState
	row := tf.db.QueryRow("SELECT last_send_line, checksum, inode_number FROM tail_file_state WHERE filepath = ?", tf.filePath)
	err := row.Scan(&state.LastSendLine, &state.Checksum, &state.InodeNumber)
	if err != nil {
		return state, err
	}
	return state, nil
}

func GetFileInfo(path string) (uint64, []byte, error) {
	var err error
	var inodeNumber uint64
	var checksum []byte

	inodeNumber, _ = util.GetInodeNumber(path)

	checksum, err = util.CreateChecksumForFirstThreeLines(path)

	return inodeNumber, checksum, err
}

func (tf TailFile) GetTailFileState() (TailFileState, error) {
	state := TailFileState{
		LastSendLine: tf.lineNumber,
	}

	inodeNumber, checksum, err := GetFileInfo(tf.filePath)
	if err != nil {
		return state, err
	}
	state.InodeNumber = inodeNumber
	state.Checksum = checksum

	return state, nil
}

func CheckTailFileStates(dbState TailFileState, path string) bool {
	inodeNumber, checksum, err := GetFileInfo(path)
	if err != nil {
		return false
	}

	// Check if the file still has the same starting lines and has the same inodeNumber
	if slices.Compare(dbState.Checksum, checksum) == 0 && dbState.InodeNumber == inodeNumber {
		return true
	}

	return false
}

// Stop stops the file tailing and closes resources
func (tf *TailFile) stop() {
	tf.watcher.Close()
	tf.file.Close()
	if tf.ctx != nil {
		tf.cancel()
		<-tf.doneCh
	}
	tf.logger.Debug("Stopping file tail", "path", tf.filePath)
}

// start monitors for changes using fsnotify
func (tf *TailFile) start() {
	// Load state from the database
	state, err := tf.GetTailFileStateFromDB()
	if err == nil {
		if CheckTailFileStates(state, tf.filePath) {
			tf.startLine = state.LastSendLine
			tf.logger.Debug("Resuming tailing", "path", tf.filePath, "startLine", tf.startLine)
		} else {
		}
	} else {
		tf.logger.Debug("Could not load state, starting from the beginning", "filepath", tf.filePath)
	}

	// Start by reading all existing lines up to the target line
	tf.readExistingLines()

	for {
		select {
		case <-tf.ctx.Done():
			close(tf.doneCh)
			return
		case event := <-tf.watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				tf.readNewLines()
			}
		case err := <-tf.watcher.Errors:
			if err == nil {
				continue
			}
			tf.logger.Error("Watcher error", "error", err, "path", tf.filePath)
		}
	}
}

func (tf *TailFile) readExistingLines() {
	_, err := tf.file.Seek(0, io.SeekStart)
	if err != nil {
		tf.logger.Error("Error seeking in file", "error", err, "path", tf.filePath)
		return
	}

	reader := bufio.NewReader(tf.file)

	for {
		lineRaw, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				tf.logger.Error("Error reading from file", "error", err)
			}
			currentOffset, _ := tf.file.Seek(0, io.SeekCurrent)
			tf.offset = currentOffset
			return
		}
		tf.lineNumber++
		line := strings.TrimSpace(lineRaw)

		result := tf.BuildResult(line)

		if tf.lineNumber > tf.startLine {
			select {
			case <-tf.ctx.Done():
				return
			case tf.sendCh <- result:
				// Line sent successfully
			}
		}
	}
}

func (tf *TailFile) readNewLines() {
	_, err := tf.file.Seek(tf.offset, io.SeekStart)
	if err != nil {
		tf.logger.Error("Error seeking in file", "error", err, "path", tf.filePath)
		return
	}

	reader := bufio.NewReader(tf.file)

	for {
		lineRaw, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				tf.logger.Error("Error reading from file", "error", err)
			}
			currentOffset, _ := tf.file.Seek(0, io.SeekCurrent)
			tf.offset = currentOffset
			return
		}
		tf.lineNumber++
		line := strings.TrimSpace(lineRaw)

		result := tf.BuildResult(line)

		select {
		case <-tf.ctx.Done():
			return
		case tf.sendCh <- result:
			// Line sent successfully
		}
	}
}

func (t *Tail) getDirContent(glob string) ([]string, error) {
	filepathsFromGlob, err := filepath.Glob(glob)
	if err != nil {
		return []string{}, err
	}

	filepaths := []string{}
	for _, filepath := range filepathsFromGlob {
		fileInfo, err := os.Stat(filepath)
		if err != nil {
			t.logger.Error("Coundnt retrieve fileinfo while getting directory content", "error", err, "path", filepath)
			continue
		}
		// If filepath is dir dont add to return array
		if fileInfo.IsDir() {
			continue
		}
		// If File is not readable dont add to return array
		if _, err := os.Open(filepath); err != nil {
			continue
		}

		filepaths = append(filepaths, filepath)
	}

	return filepaths, nil
}

func (t Tail) Read() <-chan util.Event {
	return t.sendChan
}

func (t *Tail) Watch() {
	err := t.checkDirectory()
	if err != nil {
		t.logger.Error("Failed to check directory", "error", err, "path", t.glob)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if err := t.checkDirectory(); err != nil {
				t.logger.Error("Failed to check directory", "error", err, "path", t.glob)
			}
		}
	}
}

func getKeysFromMap[T any | *TailFile](input map[string]T) []string {
	out := []string{}
	for key := range input {
		out = append(out, key)
	}
	return out
}

func (t *Tail) checkDirectory() error {
	files, err := t.getDirContent(t.glob)
	if err != nil {
		return err
	}

	for _, file := range files {
		if _, exists := t.runningTails[file]; !exists {
			if err := t.startTail(file); err != nil {
				t.logger.Error("Failed to start tail", "error", err, "path", t.glob)
			}
		}
	}

	t.logger.Debug("running file tails", "tails", getKeysFromMap(t.runningTails))

	for file, tail := range t.runningTails {
		if !slices.Contains(files, file) {
			tail.stop()
			delete(t.runningTails, file)
		}
	}

	return nil
}

func (t *Tail) startTail(path string) error {
	tail, err := NewTailFile(path, t.sendChan, t.logger, t.ctx, t.db)
	if err != nil {
		return err
	}

	go tail.start()

	t.runningTails[path] = tail
	return nil
}

func (t Tail) Start() {
	go util.ExecutePeriodically(t.ctx, 30, t.SaveState)
	go t.Watch()
}

func (t Tail) Stop() {
	if t.ctx != nil {
		t.cancel()
	}

	for _, tail := range t.runningTails {
		tail.stop()
	}

	t.SaveState()

	close(t.sendChan)
}

func (t Tail) SaveState() {
	for _, tail := range t.runningTails {
		err := tail.SaveTailFileStateToDB()
		if err != nil {
			t.logger.Error("Failed to save state", "error", err, "path", tail.filePath)
		}
	}
}

func (tf *TailFile) BuildResult(data string) util.Event {
	var result util.Event
	result.RawData = []byte(data)
	result.Time = time.Now().Unix()

	metadata := util.Metadata{
		FileName:   tf.filePath,
		LineNumber: tf.lineNumber,
	}
	result.Metadata = metadata

	return result
}
