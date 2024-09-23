package tail

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log-forwarder-client/utils"
	"log/slog"
	"os"
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
	sendCh     chan LineData
	offset     int64 // Holds the current file offset
	startLine  int64 // The line number to start reading from
	lineNumber int64 // Tracks the current line number
}

type TailFileState struct {
	LastSendLine int64
	Checksum     []byte
	InodeNumber  uint64
}

type LineData struct {
	Filepath string
	LineData string
	LineNum  int64
	Time     time.Time
}

// NewTailFile creates a new TailFile instance starting from a specific line
func NewTailFile(filePath string, logger *slog.Logger, sendCh chan LineData, startLine int64, parentCtx context.Context) (*TailFile, error) {
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

func (tf *TailFile) createLineData(data string) LineData {
	return LineData{
		Filepath: tf.filePath,
		LineData: strings.TrimSpace(data),
		LineNum:  tf.lineNumber,
		Time:     time.Now(),
	}
}

func (tf *TailFile) GetState() (TailFileState, error) {
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

// Start begins tailing the file from the specified line
func (tf *TailFile) Start() {
	tf.logger.Debug("Starting file tail", "path", tf.filePath)
	go tf.watchFile()
}

// Stop stops the file tailing and closes resources
func (tf *TailFile) Stop() {
	tf.watcher.Close()
	tf.file.Close()
	if tf.ctx != nil {
		tf.cancel()
		<-tf.doneCh
	}
	tf.logger.Debug("Stopping file tail", "path", tf.filePath)
}

// watchFile monitors for changes using fsnotify
func (tf *TailFile) watchFile() {
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
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				tf.logger.Error("Error reading from file", "error", err)
			}
			currentOffset, _ := tf.file.Seek(0, io.SeekCurrent)
			tf.offset = currentOffset
			return
		}
		tf.lineNumber++
		if tf.lineNumber > tf.startLine {
			select {
			case <-tf.ctx.Done():
				return
			case tf.sendCh <- tf.createLineData(line):
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
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				tf.logger.Error("Error reading from file", "error", err)
			}
			currentOffset, _ := tf.file.Seek(0, io.SeekCurrent)
			tf.offset = currentOffset
			return
		}
		tf.lineNumber++
		select {
		case <-tf.ctx.Done():
			return
		case tf.sendCh <- tf.createLineData(line):
			// Line sent successfully
		}
	}
}
