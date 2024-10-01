package input

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log-forwarder-client/config"
	"log-forwarder-client/utils"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	sendCh     chan [][]byte
	offset     int64 // Holds the current file offset
	startLine  int64 // The line number to start reading from
	lineNumber int64 // Tracks the current line number
}

type Tail struct {
	path         string
	runningTails map[string]*TailFile
	logger       *slog.Logger
	sendChan     chan [][]byte // chan for all writes from the file tails
	ctx          context.Context
	waitGroup    *sync.WaitGroup
}

type TailFileState struct {
	LastSendLine int64
	Checksum     []byte
	InodeNumber  uint64
}

// NewTailFile creates a new TailFile instance starting from a specific line
func NewTailFile(filePath string, sendCh chan [][]byte, logger *slog.Logger, startLine int64, parentCtx context.Context) (*TailFile, error) {
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
		startLine:  startLine,
		lineNumber: 0,
	}, nil
}

func (tf TailFile) GetState() (TailFileState, error) {
	state := TailFileState{
		LastSendLine: tf.lineNumber,
	}
	var err error
	state.InodeNumber, err = utils.GetInodeNumber(tf.filePath)
	if err != nil {
		return state, err
	}

	state.Checksum, err = utils.CreateChecksumForFirstThreeLines(tf.filePath)
	if err != nil {
		return state, err
	}

	return state, nil
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

		var result [][]byte
		result = append(result, []byte(line))

		metadata, err := buildMetadata(map[string]interface{}{
			"filepath":   tf.filePath,
			"linenumber": tf.lineNumber,
		})
		if err != nil {
			tf.logger.Warn("Coundnt build metadata", "error", err)
		}

		result = append(result, metadata)

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

		var result [][]byte
		result = append(result, []byte(line))

		metadata, err := buildMetadata(map[string]interface{}{
			"filepath":   tf.filePath,
			"linenumber": tf.lineNumber,
		})
		if err != nil {
			tf.logger.Warn("Coundnt build metadata", "error", err)
		}

		result = append(result, metadata)
		select {
		case <-tf.ctx.Done():
			return
		case tf.sendCh <- result:
			// Line sent successfully
		}
	}
}

func NewTail(glob string, wg *sync.WaitGroup, parentCtx context.Context) Tail {
	cfg := config.GetApplicationConfig()
	tail := Tail{
		path:         glob,
		logger:       cfg.Logger,
		runningTails: make(map[string]*TailFile),
		sendChan:     make(chan [][]byte),
		waitGroup:    wg,
		ctx:          parentCtx,
	}
	go tail.Watch()
	return tail
}

func (t *Tail) getDirContent(glob string) []string {
	filepathsFromGlob, err := filepath.Glob(glob)
	if err != nil {
		return []string{}
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

	return filepaths
}

func (t Tail) Read() <-chan [][]byte {
	return t.sendChan
}

func (t *Tail) Watch() {
	err := t.checkDirectory()
	if err != nil {
		t.logger.Error("Failed to check directory", "error", err, "path", t.path)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	t.waitGroup.Add(1)
	defer t.waitGroup.Done()
	for {
		select {
		case <-t.ctx.Done():
			t.Stop()
			return
		case <-ticker.C:
			if err := t.checkDirectory(); err != nil {
				t.logger.Error("Failed to check directory", "error", err, "path", t.path)
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
	files := t.getDirContent(t.path)

	for _, file := range files {
		if _, exists := t.runningTails[file]; !exists {
			if err := t.startTail(file); err != nil {
				t.logger.Error("Failed to start tail", "error", err, "path", t.path)
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
	tail, err := NewTailFile(path, t.sendChan, t.logger, 0, t.ctx)
	if err != nil {
		return err
	}

	go tail.start()

	t.runningTails[path] = tail
	return nil
}

func (t Tail) Stop() {
	for _, tail := range t.runningTails {
		tail.stop()
	}
	close(t.sendChan)
}
