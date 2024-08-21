// reader.go
package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log-forwarder-client/tail"
	"net/http"

	"github.com/sirupsen/logrus"
)

type Reader struct {
	Path         string
	ServerURL    string
	CancelFunc   context.CancelFunc
	DoneCh       chan struct{}
	Lines        []*LineData
	LastSendLine int
	DBId         int
	Logger       *logrus.Logger
}

type LineData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}

type ReaderState struct {
	Path         string
	LastSendLine int
	DBId         int
}

func (r *Reader) GetState() ReaderState {
	return ReaderState{
		Path:         r.Path,
		LastSendLine: r.LastSendLine,
		DBId:         r.DBId,
	}
}

func (r *Reader) SetState(state ReaderState) {
	r.Path = state.Path
	r.LastSendLine = state.LastSendLine
	r.DBId = state.DBId
}

func New(path string, ServerURL string, logger *logrus.Logger) *Reader {
	return &Reader{
		Path:      path,
		Lines:     []*LineData{},
		ServerURL: ServerURL,
		Logger:    logger,
	}
}

func (r *Reader) Start(ctx context.Context) error {
	if r.DoneCh != nil {
		return fmt.Errorf("reader already started")
	}

	tailFile := tail.NewFileTail(r.Path, tail.TailConfig{
		ReOpen: true,
		Offset: int64(r.LastSendLine),
	})

	ctx1, cancel := context.WithCancel(ctx)
	t, err := tailFile.Start(ctx1)
	if err != nil {
		return fmt.Errorf("Coundnt start reader: %w", err)
	}
	r.CancelFunc = cancel
	r.DoneCh = make(chan struct{})

	r.Logger.WithField("Path", r.Path).Info("Starting Reader")

	go r.processLines(t)

	return nil
}

func (r *Reader) processLines(tail <-chan tail.Line) {
	defer close(r.DoneCh)

	for {
		select {
		case line, ok := <-tail:
			if !ok {
				return
			}
			if err := r.processLine(line); err != nil {
				r.Logger.WithError(err).Error("Failed to process line")
			}
		}
	}
}

func (r *Reader) processLine(line tail.Line) error {
	data := &LineData{
		FilePath:  r.Path,
		Data:      line.LineData,
		Num:       int(line.LineNum),
		Timestamp: line.Timestamp.Unix(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal line data: %w", err)
	}

	if err := r.postData(jsonData); err != nil {
		r.Lines = append(r.Lines, data)
		return fmt.Errorf("failed to post data: %w", err)
	}

	r.LastSendLine = data.Num
	return nil
}

func (r *Reader) postData(data []byte) error {
	resp, err := http.Post(r.ServerURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (r *Reader) Stop() {
	if r.CancelFunc != nil {
		r.Logger.WithField("Path", r.Path).Info("Stopping Reader")
		r.CancelFunc()
		<-r.DoneCh
	}
}
