package input

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log-forwarder/global"
	"log-forwarder/util"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Tail struct {
	name           string
	glob           string
	tag            string
	fileEventCh    chan string
	debounceTimers map[string]*time.Timer
	state          map[string]*filestate
	fileStats      map[string]fileInfo
	wg             sync.WaitGroup
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
}

type filestate struct {
	name         string
	offset       int64
	lastReadLine int
}

type fileInfo struct {
	modTime time.Time
	size    int64
	inode   uint64
}

func newFilestate(path string) *filestate {
	return &filestate{
		name:         path,
		offset:       0,
		lastReadLine: 0,
	}
}

func (t *Tail) Name() string {
	return t.name
}

func (t *Tail) Tag() string {
	return t.tag
}

func getFileID(info os.FileInfo) (uint64, error) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino, nil
	}
	return 0, fmt.Errorf("failed to get file inode")
}

func (t *Tail) Init(config map[string]interface{}) error {
	t.glob = util.MustString(config["Glob"])
	if t.glob == "" {
		return fmt.Errorf("no glob provided for tail input")
	}

	t.name = util.MustString(config["Name"])
	if t.name == "" {
		t.name = "tail"
	}

	t.tag = util.MustString(config["Tag"])
	if t.tag == "" {
		t.tag = "tail"
	}

	t.state = make(map[string]*filestate)
	t.debounceTimers = make(map[string]*time.Timer)
	t.fileEventCh = make(chan string, 1000)
	t.wg = sync.WaitGroup{}
	t.mu = sync.Mutex{}
	return nil
}

func (t *Tail) Start(parentCtx context.Context, output chan<- global.Event) error {
	t.wg.Add(1)
	t.ctx, t.cancel = context.WithCancel(parentCtx)
	go t.fileStatLoop(t.ctx)
	slog.Info("Starting tail input", "glob", t.glob)
	go func() {
		for {
			select {
			case <-t.ctx.Done():
				t.mu.Lock()
				for _, timer := range t.debounceTimers {
					timer.Stop()
				}
				t.mu.Unlock()
				return

			case path, ok := <-t.fileEventCh:
				if !ok {
					return
				}
				go t.readFileWithDebounce(path, output)
			}
		}
	}()
	return nil
}

func (t *Tail) Exit() error {
	slog.Info("Stopping tail input", "glob", t.glob)
	if t.cancel != nil {
		t.cancel()
	}
	t.wg.Wait()
	close(t.fileEventCh)
	return nil
}

func (t *Tail) fileStatLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	defer t.wg.Done()

	t.fileStats = make(map[string]fileInfo)

	sendFileEvent := func(path string) {
		select {
		case t.fileEventCh <- path:
		default:
			slog.Warn("file event overflow")
		}
	}

	processFile := func(path string) error {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		inode, err := getFileID(info)
		if err != nil {
			return err
		}

		currentInfo := fileInfo{
			modTime: info.ModTime(),
			size:    info.Size(),
			inode:   inode,
		}

		prevInfo, exists := t.fileStats[absPath]
		if !exists {
			// New file
			t.fileStats[absPath] = currentInfo
			sendFileEvent(absPath)
			return nil
		}

		// Check if file has been recreated or modified
		if currentInfo.inode != prevInfo.inode ||
			currentInfo.modTime != prevInfo.modTime ||
			(currentInfo.size < prevInfo.size) {
			// File was either recreated or truncated
			t.mu.Lock()
			delete(t.state, absPath) // Reset file state for recreated files
			t.mu.Unlock()

			t.fileStats[absPath] = currentInfo
			sendFileEvent(absPath)
		} else if currentInfo.size > prevInfo.size {
			// File has grown
			t.fileStats[absPath] = currentInfo
			sendFileEvent(absPath)
		}

		return nil
	}

	// Initial file processing
	matches, err := filepath.Glob(t.glob)
	if err != nil {
		slog.Error("couldn't get files for glob", "error", err)
		return
	}

	for _, path := range matches {
		if err := processFile(path); err != nil {
			slog.Error("error processing file", "error", err, "path", path)
		}
	}

	// Continuous monitoring
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			matches, err := filepath.Glob(t.glob)
			if err != nil {
				slog.Error("couldn't get files for glob", "error", err)
				continue
			}
			for _, path := range matches {
				if err := processFile(path); err != nil {
					slog.Error("error processing file", "error", err, "path", path)
				}
			}
		}
	}
}

func (t *Tail) readFileWithDebounce(path string, output chan<- global.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Stop existing timer if any
	if timer, exists := t.debounceTimers[path]; exists {
		timer.Stop()
	}

	// Create new timer
	timer := time.AfterFunc(time.Second, func() {
		t.mu.Lock()
		delete(t.debounceTimers, path) // Clean up the timer reference
		t.mu.Unlock()

		// Only start reading if context is not cancelled
		select {
		case <-t.ctx.Done():
			return
		default:
			t.wg.Add(1)
			if err := t.readFile(path, output); err != nil {
				slog.Error("couldn't read from file", "error", err, "path", path)
			}
		}
	})

	t.debounceTimers[path] = timer
}

func (t *Tail) readFile(path string, output chan<- global.Event) error {
	defer t.wg.Done()

	select {
	case <-t.ctx.Done():
		return nil
	default:
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error while opening file: %v", err)
	}
	defer file.Close()

	// Get current file size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting file stats: %v", err)
	}

	t.mu.Lock()
	var currentFileState *filestate
	if state, exists := t.state[path]; !exists {
		currentFileState = &filestate{name: path}
	} else {
		currentFileState = state
	}
	t.mu.Unlock()

	// If we're at EOF and the file has been truncated, reset to beginning
	if currentFileState.offset > fileInfo.Size() {
		currentFileState.offset = 0
	}

	// Seek to the saved offset
	file.Seek(currentFileState.offset, io.SeekStart)
	reader := bufio.NewReader(file)

	for {
		select {
		case <-t.ctx.Done():
			currOffset, _ := file.Seek(0, io.SeekCurrent)
			t.mu.Lock()
			currentFileState.offset = currOffset
			t.state[path] = currentFileState
			t.mu.Unlock()
			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				currOffset, _ := file.Seek(0, io.SeekCurrent)
				t.mu.Lock()
				currentFileState.offset = currOffset
				t.state[path] = currentFileState
				t.mu.Unlock()
				return nil
			}
			return fmt.Errorf("error reading file: %v", err)
		}

		line = strings.TrimSpace(line)
		currentFileState.lastReadLine++

		if len(line) == 0 {
			continue
		}

		event := global.Event{
			Timestamp: time.Now(),
			RawData:   line,
			Metadata: global.Metadata{
				Source:  path,
				LineNum: currentFileState.lastReadLine,
			},
		}
		AddMetadata(&event, t)

		select {
		case output <- event:
		case <-t.ctx.Done():
			return nil
		}
	}
}
