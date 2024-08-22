// directory.go
package directory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log-forwarder-client/reader"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"go.etcd.io/bbolt"

	"github.com/sirupsen/logrus"
)

type DirectoryState struct {
	Path           string
	Time           time.Time
	RunningReaders map[string]*reader.Reader
	DBId           int
	ServerURL      string
	Logger         *logrus.Logger
	mu             sync.Mutex
}

func NewDirectoryState(path string, ServerURL string, logger *logrus.Logger) *DirectoryState {
	return &DirectoryState{
		Path:           path,
		RunningReaders: make(map[string]*reader.Reader),
		ServerURL:      ServerURL,
		Logger:         logger,
		Time:           time.Now(),
	}
}

func getDirContent(glob string) ([]string, error) {
	filepaths, err := filepath.Glob(glob)
	if err != nil {
	}
	return filepaths, nil
}

func (d *DirectoryState) Watch(ctx context.Context) error {
	err := d.checkDirectory()
	if err != nil {
		d.Logger.WithError(err).WithField("Directory", d.Path).Error("Failed to check directory")
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := d.checkDirectory(); err != nil {
				d.Logger.WithError(err).WithField("Directory", d.Path).Error("Failed to check directory")
			}
		}
	}
}

func (d *DirectoryState) checkDirectory() error {
	files, err := getDirContent(d.Path)
	if err != nil {
		return fmt.Errorf("failed to get directory content: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, file := range files {
		if _, exists := d.RunningReaders[file]; !exists {
			if err := d.startReader(file); err != nil {
				d.Logger.WithError(err).WithField("file", file).Error("Failed to start reader")
			}
		}
	}

	for file, r := range d.RunningReaders {
		if !slices.Contains(files, file) {
			r.Stop()
			delete(d.RunningReaders, file)
		}
	}

	return nil
}

func (d *DirectoryState) startReader(file string) error {
	r := reader.New(file, d.ServerURL, d.Logger)

	if err := r.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start reader: %w", err)
	}

	d.RunningReaders[file] = r
	return nil
}

func (d *DirectoryState) WaitForShutdown() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, reader := range d.RunningReaders {
		reader.Stop()
	}
}

func (d *DirectoryState) SaveState(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("DirectoryState"))
		if err != nil {
			return err
		}

		state := map[string]interface{}{
			"Path": d.Path,
			"Time": d.Time.Format(time.RFC3339),
			"DBId": d.DBId,
		}

		// Save running readers' states
		readers := make(map[string]reader.ReaderState)
		for path, r := range d.RunningReaders {
			readers[path] = r.GetState()
		}
		state["RunningReaders"] = readers

		encoded, err := json.Marshal(state)
		if err != nil {
			return err
		}

		return b.Put([]byte("state"), encoded)
	})
}

func (d *DirectoryState) LoadState(db *bbolt.DB, ctx context.Context) error {
	return db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("DirectoryState"))
		if b == nil {
			return nil // No state saved yet
		}

		encoded := b.Get([]byte("state"))
		if encoded == nil {
			return nil // No state saved yet
		}

		var state map[string]interface{}
		if err := json.Unmarshal(encoded, &state); err != nil {
			return err
		}

		d.Path = state["Path"].(string)
		if parsedTime, err := parseTime(state["Time"].(string)); err != nil {
			d.Logger.WithField("Directory: ", d.Path).Error("Failed to parse Time: %w", err)
		} else {
			d.Time = parsedTime
		}
		d.DBId = int(state["DBId"].(float64))

		// Load running readers' states
		if readersRaw, ok := state["RunningReaders"].(map[string]interface{}); ok {
			for path, readerStateRaw := range readersRaw {
				if readerStateMap, ok := readerStateRaw.(map[string]interface{}); ok {
					readerState := reader.ReaderState{
						Path: path,
					}
					currentLines, err := countLines(path)
					if err != nil {
						return fmt.Errorf("Coundnt read current line count: %w", err)
					}
					if lastSendLine, ok := readerStateMap["LastSendLine"].(float64); ok {
						if int(lastSendLine) >= currentLines {
							readerState.LastSendLine = int(lastSendLine)
						} else {
							fmt.Println("Resetting Line Count")
							readerState.LastSendLine = 0
						}
					}
					if dbID, ok := readerStateMap["DBId"].(float64); ok {
						readerState.DBId = int(dbID)
					}
					r := reader.New(path, d.ServerURL, d.Logger)
					r.SetState(readerState)
					r.Start(ctx)
					d.RunningReaders[path] = r
				}
			}
		}

		return nil
	})
}

func parseTime(timeStr string) (time.Time, error) {
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Now(), fmt.Errorf("failed to parse Time: %w", err)
	}
	return parsedTime, nil
}

func countLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lines, nil
}
