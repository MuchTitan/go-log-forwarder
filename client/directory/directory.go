// directory.go
package directory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log-forwarder-client/reader"
	"log-forwarder-client/utils"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

type DirectoryState struct {
	Path              string
	Time              time.Time
	RunningReaders    map[string]*reader.Reader
	DBId              int
	ServerURL         string
	Logger            *slog.Logger
	mu                sync.Mutex
	sendChan          chan []byte
	LinesFailedToSend [][]byte
}

func NewDirectoryState(path string, ServerURL string, logger *slog.Logger) *DirectoryState {
	return &DirectoryState{
		Path:              path,
		RunningReaders:    make(map[string]*reader.Reader),
		ServerURL:         ServerURL,
		Logger:            logger,
		Time:              time.Now(),
		sendChan:          make(chan []byte),
		LinesFailedToSend: [][]byte{},
	}
}

func (d *DirectoryState) getDirContent(glob string) []string {
	filepaths, _ := filepath.Glob(glob)
	return filepaths
}

func (d *DirectoryState) postData(data []byte) error {
	resp, err := http.Post(d.ServerURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (d *DirectoryState) lineDataHandler(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-d.sendChan:
			err := d.postData(data)
			if err != nil {
				d.Logger.Error("[First Send] coundnt send line", "error", err)
				d.LinesFailedToSend = append(d.LinesFailedToSend, data)
			}
		}
	}
}

func removeIndexFromSlice[T any](slice []T, index int) []T {
	if index < 0 || index >= len(slice) {
		return slice // Return the original slice if index is out of bounds
	}

	return append(slice[:index], slice[index+1:]...)
}

func (d *DirectoryState) retryLineData() {
	for {
		for i, data := range d.LinesFailedToSend {
			err := d.postData(data)
			if err != nil {
				d.Logger.Error("[Retry] coundnt send line", "error", err)
				d.LinesFailedToSend = append(d.LinesFailedToSend, data)
			} else {
				d.LinesFailedToSend = removeIndexFromSlice(d.LinesFailedToSend, i)
			}
		}
		time.Sleep(time.Second * 3)
	}
}

func (d *DirectoryState) Watch(ctx context.Context) error {
	err := d.checkDirectory(ctx)
	if err != nil {
		d.Logger.Error("Failed to check directory", "error", err, "path", d.Path)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go d.lineDataHandler(ctx)
	go d.retryLineData()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := d.checkDirectory(ctx); err != nil {
				d.Logger.Error("Failed to check directory", "error", err, "path", d.Path)
			}
		}
	}
}

func getKeysFromMap[T any | *reader.Reader](input map[string]T) []string {
	out := []string{}
	for key := range input {
		out = append(out, key)
	}
	return out
}

func (d *DirectoryState) checkDirectory(ctx context.Context) error {
	files := d.getDirContent(d.Path)

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, file := range files {
		if _, exists := d.RunningReaders[file]; !exists {
			if err := d.startReader(ctx, file); err != nil {
				d.Logger.Error("Failed to start reader", "error", err, "path", d.Path)
			}
		}
	}

	d.Logger.Debug("running readers", "readers", getKeysFromMap(d.RunningReaders))

	for file, r := range d.RunningReaders {
		if !slices.Contains(files, file) {
			r.Stop()
			delete(d.RunningReaders, file)
		}
	}

	return nil
}

func (d *DirectoryState) startReader(ctx context.Context, file string) error {
	r := reader.New(file, d.ServerURL, d.Logger, d.sendChan)

	if err := r.Start(ctx); err != nil {
		return fmt.Errorf("failed to start reader: %w", err)
	}

	d.RunningReaders[file] = r
	return nil
}

func (d *DirectoryState) Stop() {
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
			"Path":              d.Path,
			"Time":              d.Time.Format(time.RFC3339),
			"DBId":              d.DBId,
			"LinesFailedToSend": d.LinesFailedToSend,
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

		d.Logger.Debug("saving state", "state", state)

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
			d.Logger.Error("Failed to parse Time", "error", err)
			panic(err)
		} else {
			d.Time = parsedTime
		}
		d.DBId = int(state["DBId"].(float64))
		// Safely check if "LinesFailedToSend" exists and is a non-empty slice
		if lines, ok := state["LinesFailedToSend"].([]interface{}); ok {
			// Convert to [][]byte
			var linesFailedToSend [][]byte
			for _, line := range lines {
				if lineBytes, ok := line.([]byte); ok {
					linesFailedToSend = append(linesFailedToSend, lineBytes)
				}
			}
			// Now assign the converted value to d.LinesFailedToSend
			d.LinesFailedToSend = linesFailedToSend
		}

		// Load running readers' states
		if readersRaw, ok := state["RunningReaders"].(map[string]interface{}); ok {
			for path, readerStateRaw := range readersRaw {
				if readerStateMap, ok := readerStateRaw.(map[string]interface{}); ok {
					readerState := reader.ReaderState{
						Path: path,
					}
					currentLines, err := utils.CountLines(path)
					if err != nil {
						return fmt.Errorf("Coundnt read current line count: %w", err)
					}
					if lastSendLine, ok := readerStateMap["LastSendLine"].(float64); ok {
						if int(lastSendLine) >= currentLines {
							readerState.LastSendLine = int(lastSendLine)
						} else {
							d.Logger.Info("Resetting Line Count")
							readerState.LastSendLine = 0
						}
					}
					r := reader.New(path, d.ServerURL, d.Logger, d.sendChan)
					r.SetState(readerState)
					r.Start(ctx)
					d.RunningReaders[path] = r
				}
			}
		}
		d.Logger.Debug("loading state", "state", state)
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
