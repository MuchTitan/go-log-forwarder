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
	"sync"
	"time"
)

type DirectoryState struct {
	path              string
	time              time.Time
	runningTails      map[string]*tail.TailFile
	dbID              int
	serverURL         string
	logger            *slog.Logger
	sendChan          chan tail.LineData // chan for all writes from the file tails
	ctx               context.Context
	waitGroup         *sync.WaitGroup
	linesFailedToSend [][]byte // Array of lines that didnt get send due to HTTP Errors
}

type postData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}

func NewDirectoryState(path string, ServerURL string, logger *slog.Logger, wg *sync.WaitGroup, ctx context.Context) *DirectoryState {
	return &DirectoryState{
		path:              path,
		serverURL:         ServerURL,
		logger:            logger,
		runningTails:      make(map[string]*tail.TailFile),
		time:              time.Now(),
		sendChan:          make(chan tail.LineData),
		waitGroup:         wg,
		ctx:               ctx,
		linesFailedToSend: [][]byte{},
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
	resp, err := http.Post(d.serverURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func encodeLineToBytes(line tail.LineData) ([]byte, error) {
	// Create postData from Line
	pd := postData{
		FilePath:  line.Filepath,
		Data:      line.LineData,
		Num:       int(line.LineNum),
		Timestamp: line.Time.Unix(), // Convert time.Time to Unix timestamp (int64)
	}

	// Encode postData to JSON
	jsonData, err := json.Marshal(pd)
	if err != nil {
		return []byte{}, err
	}

	return jsonData, nil
}

func (d *DirectoryState) lineDataHandler() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case lineData := <-d.sendChan:
			// TODO: Handle the potential encoding error
			data, _ := encodeLineToBytes(lineData)
			err := d.postData(data)
			if err != nil {
				d.logger.Debug("[First Send] coundnt send line", "error", err)
				d.linesFailedToSend = append(d.linesFailedToSend, data)
			}
		}
	}
}

func (d *DirectoryState) retryLineData() {
	for {
		if len(d.linesFailedToSend) > 15 {
			d.logger.Warn("There are Lines that coundnt be send", "amount", len(d.linesFailedToSend))
		}
		for i, data := range d.linesFailedToSend {
			err := d.postData(data)
			if err != nil {
				d.logger.Debug("[Retry] coundnt send line", "error", err)
			} else {
				d.linesFailedToSend = utils.RemoveIndexFromSlice(d.linesFailedToSend, i)
			}
		}
		time.Sleep(time.Second * 3)
	}
}

func (d *DirectoryState) Watch() {
	err := d.checkDirectory()
	if err != nil {
		d.logger.Error("Failed to check directory", "error", err, "path", d.path)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go d.lineDataHandler()
	go d.retryLineData()
	d.waitGroup.Add(1)
	defer d.waitGroup.Done()
	for {
		select {
		case <-d.ctx.Done():
			d.Stop()
			return
		case <-ticker.C:
			if err := d.checkDirectory(); err != nil {
				d.logger.Error("Failed to check directory", "error", err, "path", d.path)
			}
		}
	}
}

func getKeysFromMap[T any | *tail.TailFile](input map[string]T) []string {
	out := []string{}
	for key := range input {
		out = append(out, key)
	}
	return out
}

func (d *DirectoryState) checkDirectory() error {
	files := d.getDirContent(d.path)

	for _, file := range files {
		if _, exists := d.runningTails[file]; !exists {
			if err := d.startTail(file); err != nil {
				d.logger.Error("Failed to start tail", "error", err, "path", d.path)
			}
		}
	}

	d.logger.Debug("running file tails", "tails", getKeysFromMap(d.runningTails))

	for file, tail := range d.runningTails {
		if !slices.Contains(files, file) {
			tail.Stop()
			delete(d.runningTails, file)
		}
	}

	return nil
}

func (d *DirectoryState) startTail(path string) error {
	tail, err := tail.NewTailFile(path, d.logger, d.sendChan, 0, d.ctx)
	if err != nil {
		return err
	}

	tail.Start()

	d.runningTails[path] = tail
	return nil
}

func (d *DirectoryState) Stop() {
	for _, tail := range d.runningTails {
		tail.Stop()
	}
	close(d.sendChan)
}

func parseTime(timeStr string) (time.Time, error) {
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Now(), fmt.Errorf("failed to parse Time: %w", err)
	}
	return parsedTime, nil
}
