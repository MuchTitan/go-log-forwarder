package tail

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Line represents a line from the tailed file
type Line struct {
	LineNum   int64
	LineData  string
	Timestamp time.Time
}

// File is a struct to manage file tailing
type File struct {
	file        *os.File
	fileReader  *bufio.Reader
	path        string
	lastLineNum int64
	doneCh      chan struct{}
	cancel      context.CancelFunc
	reOpen      bool
	Logger      *slog.Logger
}

// TailConfig holds configuration for file tailing
type TailConfig struct {
	Offset int64
	ReOpen bool
}

// NewFileTail creates a new File instance
func NewFileTail(path string, config TailConfig, logger *slog.Logger) *File {
	return &File{
		path:        path,
		lastLineNum: config.Offset,
		reOpen:      config.ReOpen,
		Logger:      logger,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func openFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}
	return file, nil
}

func (f *File) skipLines() {
	for currentLine := int64(0); currentLine < f.lastLineNum; currentLine++ {
		if _, err := f.fileReader.ReadString('\n'); err != nil {
			break
		}
	}
}

func (f *File) ReOpen() {
	f.Logger.Info("Trying to ReOpen File", "path", f.path)
	for {
		file, err := openFile(f.path)
		if err == nil {
			f.file.Close()
			f.file = file
			f.fileReader = bufio.NewReader(f.file)
			f.lastLineNum = 0
			fmt.Printf("Reopened File: %s\n", f.path)
			return
		}
		time.Sleep(time.Second)
	}
}

// Start starts tailing the file and returns a channel with Line structs
func (f *File) Start(ctx context.Context) (<-chan Line, error) {
	file, err := openFile(f.path)
	if err != nil {
		return nil, err
	}

	f.file = file
	f.fileReader = bufio.NewReader(f.file)

	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	lineChan := make(chan Line)
	f.doneCh = make(chan struct{})

	go f.tailFile(ctx, lineChan)

	return lineChan, nil
}

func (f *File) Stop() {
	if f.cancel != nil {
		f.cancel()
		<-f.doneCh
	}
}

func (f *File) tailFile(ctx context.Context, lineChan chan<- Line) {
	defer close(lineChan)
	defer f.file.Close()

	f.skipLines()

	for {
		select {
		case <-ctx.Done():
			close(f.doneCh)
			return
		default:
			line, err := f.fileReader.ReadString('\n')
			if err != nil {
				if f.handleError(err) {
					continue
				}
				return
			}
			f.lastLineNum++
			data := Line{
				LineNum:   f.lastLineNum,
				LineData:  strings.TrimSuffix(line, "\n"),
				Timestamp: time.Now(),
			}

			lineChan <- data
			f.Logger.Debug("sending line data", "data", data, "path", f.path)
		}
	}
}

func (f *File) handleError(err error) bool {
	if f.reOpen && !fileExists(f.path) {
		f.ReOpen()
		return true
	}
	if err == io.EOF {
		time.Sleep(time.Second)
		return true
	}
	return false
}
