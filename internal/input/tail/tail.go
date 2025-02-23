package tail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/input"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
	"github.com/sirupsen/logrus"
)

type Tail struct {
	name               string
	glob               string
	dbFile             string
	tag                string
	cleanUpThreshold   int
	fileEventCh        chan string
	fileStateCh        chan fileState
	debounceTimers     map[string]*time.Timer
	state              map[string]*fileState
	fileStats          map[string]fileInfo
	wg                 sync.WaitGroup
	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	stateSavingEnabled bool
	repository         TailRepository
}

func (t *Tail) Name() string {
	return t.name
}

func (t *Tail) Tag() string {
	return t.tag
}

func (t *Tail) Init(config map[string]any) error {
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

	if tmpThreshold, ok := config["CleanUpThreshold"].(int); ok {
		t.cleanUpThreshold = tmpThreshold
	} else {
		t.cleanUpThreshold = 3
	}

	if colors, exists := config["EnableDB"]; exists {
		var ok bool
		if t.stateSavingEnabled, ok = colors.(bool); !ok {
			return errors.New("cant convert EnableDB parameter to bool")
		}
	}

	if t.stateSavingEnabled {
		if dbFile, ok := config["DBFile"].(string); ok {
			t.dbFile = dbFile
		} else {
			t.dbFile = filepath.Join("./", fmt.Sprintf("%s-%s.db", t.tag, GetGlobRoot(t.glob)))
		}
	}

	if t.stateSavingEnabled {
		t.repository = NewSQLiteTailRepository(t.dbFile)
		if err := t.repository.CreateTables(); err != nil {
			return err
		}
	}

	t.state = make(map[string]*fileState)
	t.debounceTimers = make(map[string]*time.Timer)
	t.fileEventCh = make(chan string, 1000)
	t.fileStateCh = make(chan fileState)
	t.wg = sync.WaitGroup{}
	t.mu = sync.Mutex{}
	return nil
}

func GetGlobRoot(glob string) string {
	glob = filepath.Clean(glob)

	wildcardIndex := strings.IndexAny(glob, "*?[{")
	if wildcardIndex == -1 {
		return glob
	}

	root := glob[:wildcardIndex]
	lastSlash := strings.LastIndex(root, string(filepath.Separator))
	if lastSlash == -1 {
		return "."
	}

	return glob[:lastSlash]
}

func getFileID(info os.FileInfo) (uint64, error) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino, nil
	}
	return 0, fmt.Errorf("failed to get file inode")
}

func (t *Tail) Start(parentCtx context.Context, output chan<- internal.Event) error {
	t.wg.Add(1)
	if t.stateSavingEnabled {
		t.wg.Add(1)
		t.persistStates()
	}

	t.ctx, t.cancel = context.WithCancel(parentCtx)
	go t.fileStatLoop(t.ctx)
	logrus.WithField("glob", t.glob).Info("Starting Tail Input")
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
	logrus.WithField("glob", t.glob).Info("Stopping Tail Input")
	if t.cancel != nil {
		t.cancel()
	}
	t.wg.Wait()
	close(t.fileEventCh)
	close(t.fileStateCh)
	deletedCount, err := t.repository.CleanupOldEntries(t.cleanUpThreshold)
	if err != nil {
		return err
	}
	logrus.Debugf("cleaned %d old entries in tail_files db", deletedCount)

	if err := t.repository.Close(); err != nil {
		logrus.WithError(err).Error("could not close db repostiory")
	}

	return nil
}

func (t *Tail) fileStatLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer t.wg.Done()
	defer ticker.Stop()

	t.fileStats = make(map[string]fileInfo)

	sendFileEvent := func(path string) {
		select {
		case t.fileEventCh <- path:
		default:
			logrus.Warn("file event overflow")
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
		logrus.WithError(err).Warn("could not get files for glob")
		return
	}

	for _, path := range matches {
		if err := processFile(path); err != nil {
			logrus.WithError(err).Warn("error processing file")
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
				logrus.WithError(err).Warn("could not get files for glob")
				continue
			}
			for _, path := range matches {
				if err := processFile(path); err != nil {
					logrus.WithError(err).Warn("error processing file")
				}
			}
		}
	}
}

func (t *Tail) readFileWithDebounce(path string, output chan<- internal.Event) {
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
				logrus.WithField("path", path).WithError(err).Warn("couldn't read from file")
			}
		}
	})

	t.debounceTimers[path] = timer
}

func (t *Tail) readFile(path string, output chan<- internal.Event) error {
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

	sendFileState := func(state fileState) {
		if t.stateSavingEnabled {
			t.fileStateCh <- state
		}
	}

	t.mu.Lock()
	var currentFileState *fileState
	if state, exists := t.state[path]; !exists {
		inode, err := getFileID(fileInfo)
		if err != nil {
			logrus.WithField("path", path).WithError(err).Error("could not get inode")
		}
		currentFileState = &fileState{Path: path, InodeNumber: inode}
		if t.stateSavingEnabled {
			dbState, err := t.repository.GetFileState(path, inode)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"path":  path,
					"inode": inode,
				}).WithError(err).Debug("did not find a saved file state in db")
			} else {
				currentFileState = dbState
			}
		}
	} else {
		currentFileState = state
	}
	t.mu.Unlock()

	// If we're at EOF and the file has been truncated, reset to beginning
	if currentFileState.Offset > fileInfo.Size() {
		currentFileState.Offset = 0
		currentFileState.LastReadLine = 0
		if err := t.repository.DeleteFileState(currentFileState.Path, currentFileState.InodeNumber); err != nil {
			logrus.WithError(err).Error("error during file state deleting")
		}
	}

	// Seek to the saved offset
	file.Seek(currentFileState.Offset, io.SeekStart)
	reader := bufio.NewReader(file)

	for {
		select {
		case <-t.ctx.Done():
			currOffset, _ := file.Seek(0, io.SeekCurrent)
			t.mu.Lock()
			currentFileState.Offset = currOffset
			t.state[path] = currentFileState
			sendFileState(*t.state[path])
			t.mu.Unlock()
			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				currOffset, _ := file.Seek(0, io.SeekCurrent)
				t.mu.Lock()
				currentFileState.Offset = currOffset
				t.state[path] = currentFileState
				sendFileState(*t.state[path])
				t.mu.Unlock()
				return nil
			}
			return fmt.Errorf("error reading file: %v", err)
		}

		line = strings.TrimSpace(line)
		currentFileState.LastReadLine++

		if len(line) == 0 {
			continue
		}

		event := internal.Event{
			Timestamp: time.Now(),
			RawData:   line,
			Metadata: internal.Metadata{
				Source:  path,
				LineNum: currentFileState.LastReadLine,
			},
		}
		input.AddMetadata(&event, t)

		select {
		case output <- event:
		case <-t.ctx.Done():
			return nil
		}
	}
}

func (t *Tail) persistStates() {
	ticker := time.NewTicker(time.Millisecond * 100)
	var updates []fileState

	go func() {
		defer t.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-t.ctx.Done():
				if len(updates) > 0 {
					t.repository.BatchUpsertFileStates(updates)
				}
				return
			case state := <-t.fileStateCh:
				updates = append(updates, state)
			case <-ticker.C:
				if len(updates) > 0 {
					t.repository.BatchUpsertFileStates(updates)
					updates = updates[:0]
				}
			}
		}
	}()
}
