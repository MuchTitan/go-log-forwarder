// reader.go
package reader

import (
	"context"
	"encoding/json"
	"fmt"
	"log-forwarder-client/tail"
	"log/slog"
)

type Reader struct {
	Path         string
	ServerURL    string
	CancelFunc   context.CancelFunc
	DoneCh       chan struct{}
	SendCh       chan []byte
	LastSendLine int
	Logger       *slog.Logger
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
}

func (r *Reader) GetState() ReaderState {
	return ReaderState{
		Path:         r.Path,
		LastSendLine: r.LastSendLine,
	}
}

func (r *Reader) SetState(state ReaderState) {
	r.Path = state.Path
	r.LastSendLine = state.LastSendLine
}

func New(path string, ServerURL string, logger *slog.Logger, sendChan chan []byte) *Reader {
	return &Reader{
		Path:      path,
		ServerURL: ServerURL,
		Logger:    logger,
		SendCh:    sendChan,
	}
}

func (r *Reader) Start(ctx context.Context) error {
	if r.DoneCh != nil {
		return fmt.Errorf("reader already started")
	}

	fileTail := tail.NewFileTail(r.Path, tail.TailConfig{
		ReOpen: true,
		Offset: int64(r.LastSendLine),
	}, r.Logger)

	ctx, cancel := context.WithCancel(ctx)
	r.CancelFunc = cancel
	r.DoneCh = make(chan struct{})

	r.Logger.Info("Starting Reader", "path", r.Path)

	go r.processLines(ctx, fileTail)

	return nil
}

func (r *Reader) processLines(ctx context.Context, tail *tail.File) {
	t, err := tail.Start(ctx)
	if err != nil {
		r.Logger.Info("Coundnt start tail for reader", "error", err, "path", r.Path)
	}
	for {
		select {
		case <-ctx.Done():
			tail.Stop()
			close(r.DoneCh)
			return
		case line, ok := <-t:
			if !ok {
				return
			}
			if err := r.processLine(line); err != nil {
				r.Logger.Error("Failed to process line", "error", err)
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

	r.SendCh <- jsonData

	r.LastSendLine = data.Num
	return nil
}

func (r *Reader) Stop() {
	if r.CancelFunc != nil {
		r.Logger.Info("Stopping Reader", "path", r.Path)
		r.CancelFunc()
		<-r.DoneCh
	}
}
