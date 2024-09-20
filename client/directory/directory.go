// directory.go
package directory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log-forwarder-client/tail"
	"log-forwarder-client/utils"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"go.etcd.io/bbolt"
)

type DirectoryState struct {
	Path              string
	Time              time.Time
	RunningTails      map[string]*tail.File
	DBId              int
	ServerURL         string
	Logger            *slog.Logger
	sendChan          chan tail.Line
	LinesFailedToSend [][]byte
}

type postData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}

func NewDirectoryState(path string, ServerURL string, logger *slog.Logger) *DirectoryState {
	return &DirectoryState{
		Path:              path,
		ServerURL:         ServerURL,
		Logger:            logger,
		RunningTails:      make(map[string]*tail.File),
		Time:              time.Now(),
		sendChan:          make(chan tail.Line),
		LinesFailedToSend: [][]byte{},
	}
}

func (d *DirectoryState) getDirContent(glob string) []string {
	filepaths, err := filepath.Glob(glob)
	if err != nil {
		return []string{}
	}
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

func encodeLineToBytes(line tail.Line) ([]byte, error) {
	// Create postData from Line
	pd := postData{
		FilePath:  line.FilePath,
		Data:      line.LineData,
		Num:       int(line.LineNum),
		Timestamp: line.Timestamp.Unix(), // Convert time.Time to Unix timestamp (int64)
	}

	// Encode postData to JSON
	jsonData, err := json.Marshal(pd)
	if err != nil {
		return []byte{}, err
	}

	return jsonData, nil
}

func (d *DirectoryState) lineDataHandler(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case lineData := <-d.sendChan:
			// TODO: Handle the potential encoding error
			data, _ := encodeLineToBytes(lineData)
			d.Logger.Debug("linedata", "data", lineData)
			err := d.postData(data)
			if err != nil {
				d.Logger.Debug("[First Send] coundnt send line", "error", err)
				d.LinesFailedToSend = append(d.LinesFailedToSend, data)
			}
		}
	}
}

func (d *DirectoryState) retryLineData() {
	for {
		if len(d.LinesFailedToSend) > 15 {
			d.Logger.Warn("There are Lines that coundnt be send", "amount", len(d.LinesFailedToSend))
		}
		for i, data := range d.LinesFailedToSend {
			err := d.postData(data)
			if err != nil {
				d.Logger.Debug("[Retry] coundnt send line", "error", err)
			} else {
				d.LinesFailedToSend = utils.RemoveIndexFromSlice(d.LinesFailedToSend, i)
			}
		}
		time.Sleep(time.Second * 3)
	}
}

func (d *DirectoryState) Watch(ctx context.Context) {
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
			return
		case <-ticker.C:
			if err := d.checkDirectory(ctx); err != nil {
				d.Logger.Error("Failed to check directory", "error", err, "path", d.Path)
			}
		}
	}
}

func getKeysFromMap[T any | *tail.File](input map[string]T) []string {
	out := []string{}
	for key := range input {
		out = append(out, key)
	}
	return out
}

func (d *DirectoryState) checkDirectory(ctx context.Context) error {
	files := d.getDirContent(d.Path)

	for _, file := range files {
		if _, exists := d.RunningTails[file]; !exists {
			if err := d.startTail(ctx, file); err != nil {
				d.Logger.Error("Failed to start tail", "error", err, "path", d.Path)
			}
		}
	}

	d.Logger.Debug("running readers", "tails", getKeysFromMap(d.RunningTails))

	for file, r := range d.RunningTails {
		if !slices.Contains(files, file) {
			r.Stop()
			delete(d.RunningTails, file)
		}
	}

	return nil
}

func (d *DirectoryState) startTail(ctx context.Context, path string) error {
	tail := tail.NewFileTail(path, d.Logger, d.sendChan, tail.TailConfig{
		ReOpenValue:  true,
		LastSendLine: 0,
	})

	err := tail.Start(ctx)
	if err != nil {
		return fmt.Errorf("Coundnt start tail for %s: %w", path, err)
	}

	d.RunningTails[path] = tail
	return nil
}

func (d *DirectoryState) Stop() {
	for _, tail := range d.RunningTails {
		tail.Stop()
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
		tails := make(map[string]int64)
		for path, r := range d.RunningTails {
			tails[path] = r.GetState()
		}
		state["RunningTails"] = tails

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
			// assign the converted value to d.LinesFailedToSend
			d.LinesFailedToSend = linesFailedToSend
		}

		// Load running readers' states
		if tailsRaw, ok := state["RunningTails"].(map[string]interface{}); ok {
			for path, lastSendLineRaw := range tailsRaw {
				// check the current line count of the file path
				currentLines, err := utils.CountLines(path)
				if err != nil {
					return fmt.Errorf("Coundnt read current line count: %w", err)
				}
				// create a new tail instance with offset 0 (Resetting Line Count)
				tail := tail.NewFileTail(path, d.Logger, d.sendChan, tail.TailConfig{
					ReOpenValue:  true,
					LastSendLine: 0,
				})

				if lastSendLine, ok := lastSendLineRaw.(float64); ok {
					if int(lastSendLine) >= currentLines {
						tail.UpdateLastSendLine(int64(lastSendLine))
					} else {
						d.Logger.Info("Resetting Line Count")
					}
				}

				//TODO: implement better error handling
				err = tail.Start(ctx)
				if err != nil {
					d.Logger.Error("Coundnt start tailing from saved state", "path", path)
					continue
				}
				d.RunningTails[path] = tail
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
