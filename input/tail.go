package input

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/MuchTitan/go-log-forwarder/database"
	"github.com/MuchTitan/go-log-forwarder/global"
	"github.com/MuchTitan/go-log-forwarder/util"
)

type Tail struct {
	name             string
	glob             string
	tag              string
	cleanUpThreshold int
	fileEventCh      chan string
	fileStateCh      chan filestate
	debounceTimers   map[string]*time.Timer
	state            map[string]*filestate
	fileStats        map[string]fileInfo
	wg               sync.WaitGroup
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	dbEnabled        bool
	DbManager        *database.DBManager
}

type filestate struct {
	name         string
	offset       int64
	inode        uint64
	lastReadLine int
}

type fileInfo struct {
	modTime time.Time
	size    int64
	inode   uint64
}

func (t *Tail) Name() string {
	return t.name
}

func (t *Tail) Tag() string {
	return t.tag
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

	if tmpThreshold, ok := config["CleanUpThreshold"].(int); ok {
		t.cleanUpThreshold = tmpThreshold
	} else {
		t.cleanUpThreshold = 3
	}

	if t.DbManager != nil {
		t.dbEnabled = true
		if err := t.createDBTables(); err != nil {
			return err
		}
	}

	t.state = make(map[string]*filestate)
	t.debounceTimers = make(map[string]*time.Timer)
	t.fileEventCh = make(chan string, 1000)
	t.fileStateCh = make(chan filestate)
	t.wg = sync.WaitGroup{}
	t.mu = sync.Mutex{}
	return nil
}

func getFileID(info os.FileInfo) (uint64, error) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino, nil
	}
	return 0, fmt.Errorf("failed to get file inode")
}

func (t *Tail) Start(parentCtx context.Context, output chan<- global.Event) error {
	t.wg.Add(1)
	if t.dbEnabled {
		t.wg.Add(1)
		t.persistStates()
	}
	t.ctx, t.cancel = context.WithCancel(parentCtx)
	go t.fileStatLoop(t.ctx)
	slog.Info("[Tail] Starting", "glob", t.glob)
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
	slog.Info("[Tail] Stopping", "glob", t.glob)
	if t.cancel != nil {
		t.cancel()
	}
	t.wg.Wait()
	close(t.fileEventCh)
	close(t.fileStateCh)
	deletedCount, err := t.cleanUpOldDbEntries()
	if err != nil {
		return err
	}
	slog.Info("cleaned old entries in tail_files db", "amount", deletedCount)
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

	sendFileState := func(state filestate) {
		if t.dbEnabled {
			t.fileStateCh <- state
		}
	}

	t.mu.Lock()
	var currentFileState *filestate
	if state, exists := t.state[path]; !exists {
		inode, err := getFileID(fileInfo)
		if err != nil {
			slog.Error("error getting inode", "file", path)
		}
		dbState, err := t.getFileStateFromDB(path, inode)
		if err != nil {
			currentFileState = &filestate{name: path, inode: inode}
			slog.Debug("did not find a saved file state in db", "path", path, "inode", inode, "error", err)
		} else {
			currentFileState = &dbState
		}
	} else {
		currentFileState = state
	}
	t.mu.Unlock()

	// If we're at EOF and the file has been truncated, reset to beginning
	if currentFileState.offset > fileInfo.Size() {
		currentFileState.offset = 0
		currentFileState.lastReadLine = 0
		if err := t.deleteFileFromDB(currentFileState.name, currentFileState.inode); err != nil {
			slog.Error("error during file state deleting", "error", err)
		}
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
				currentFileState.offset = currOffset
				t.state[path] = currentFileState
				sendFileState(*t.state[path])
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

func (t *Tail) createDBTables() error {
	query := `CREATE TABLE IF NOT EXISTS tail_files (
        path TEXT NOT NULL,
        offset INTEGER NOT NULL,
        lastReadLine INTEGER NOT NULL,
        inodenumber INTEGER NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (path, inodenumber)
    )`
	_, err := t.DbManager.ExecuteWrite(query)
	if err != nil {
		return fmt.Errorf("could not create db table tail_files: %v", err)
	}
	return nil
}

func (t *Tail) persistStates() {
	ticker := time.NewTicker(time.Millisecond * 100)
	var updates []filestate
	persistStates := func(tx *sql.Tx) error {
		if len(updates) == 0 {
			return nil
		}

		stmt, err := tx.Prepare(`
            INSERT OR REPLACE INTO tail_files (path, offset, lastReadLine, inodenumber, updated_at) VALUES ($1,$2,$3,$4,$5)
			`)
		if err != nil {
			slog.Error("Failed to prepare statement", "error", err)
			tx.Rollback()
		}

		for _, update := range updates {
			_, err := stmt.Exec(
				update.name,
				update.offset,
				update.lastReadLine,
				update.inode,
				time.Now(),
			)
			if err != nil {
				slog.Error("Failed to execute statement", "error", err)
				continue
			}
		}

		stmt.Close()
		err = tx.Commit()
		if err != nil {
			slog.Error("Failed to commit transaction", "error", err)
			tx.Rollback()
		}

		updates = updates[:0] // Clear the slice
		return nil
	}

	go func() {
		defer t.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-t.ctx.Done():
				t.DbManager.ExecuteWriteTx(persistStates)
				return
			case state := <-t.fileStateCh:
				updates = append(updates, state)
			case <-ticker.C:
				t.DbManager.ExecuteWriteTx(persistStates)
			}
		}
	}()
}

func (t *Tail) cleanUpOldDbEntries() (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -t.cleanUpThreshold).Format("2006-01-02 15:04:05")
	query := "DELETE FROM tail_files WHERE updated_at < datetime($1)"
	res, err := t.DbManager.ExecuteWrite(query, cutoffDate)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (t *Tail) deleteFileFromDB(path string, inode uint64) error {
	query := `DELETE FROM tail_files WHERE path = $1 AND inodenumber = $2`
	_, err := t.DbManager.ExecuteWrite(query, path, inode)
	if err != nil {
		return fmt.Errorf("could not delete file entry from tail_files: %v", err)
	}

	return nil
}

func (t *Tail) getFileStateFromDB(path string, inode uint64) (filestate, error) {
	query := `SELECT path, offset, lastReadLine, inodenumber from tail_files WHERE path = $1 AND inodenumber = $2`
	row := t.DbManager.QueryRow(query, path, inode)
	currentState := filestate{}
	err := row.Scan(&currentState.name, &currentState.offset, &currentState.lastReadLine, &currentState.inode)
	if err != nil {
		return filestate{}, fmt.Errorf("could not parse row into struct: %v", err)
	}
	return currentState, nil
}
