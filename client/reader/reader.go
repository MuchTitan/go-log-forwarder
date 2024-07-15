// reader.go
package reader

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nxadm/tail"
	"github.com/sirupsen/logrus"
)

type Reader struct {
	Path         string
	CancelFunc   context.CancelFunc
	DoneCh       chan struct{}
	Lines        []*LineData
	LastSendLine int
	DBId         int
	Config       *Config
	Logger       *logrus.Logger
}
type LineData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}

type Config struct {
	ServerURL string
	DB        *sql.DB
}

func New(path string, config *Config, logger *logrus.Logger) *Reader {
	return &Reader{
		Path:   path,
		Lines:  []*LineData{},
		Config: config,
		Logger: logger,
	}
}

func (r *Reader) Start(ctx context.Context) error {
	if r.DoneCh != nil {
		return fmt.Errorf("reader already started")
	}

	t, err := tail.TailFile(r.Path, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		return fmt.Errorf("failed to tail file: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	r.CancelFunc = cancel
	r.DoneCh = make(chan struct{})

	go r.processLines(ctx, t.Lines)

	r.Logger.WithField("Path", r.Path).Info("Starting Reader")

	return nil
}

func (r *Reader) processLines(ctx context.Context, lines <-chan *tail.Line) {
	defer close(r.DoneCh)

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				r.Logger.Error("Line channel closed unexpectedly")
				return
			}
			if err := r.processLine(line); err != nil {
				r.Logger.WithError(err).Error("Failed to process line")
			}
		}
	}
}

func (r *Reader) processLine(line *tail.Line) error {
	data := &LineData{
		FilePath:  r.Path,
		Data:      line.Text,
		Num:       line.Num,
		Timestamp: time.Now().Unix(),
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
	resp, err := http.Post(r.Config.ServerURL, "application/json", bytes.NewBuffer(data))
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
