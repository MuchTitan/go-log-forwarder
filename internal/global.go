package internal

import (
	"io"
	"time"
)

type Event struct {
	Timestamp  time.Time
	RawData    string
	ParsedData map[string]any
	Metadata   Metadata
}

type Metadata struct {
	Source      string
	Host        string
	Tag         string
	LineNum     int
	InputSource string
}

// Plugin interface that all plugins must implement
type Plugin interface {
	Name() string
	Init(config map[string]any) error
	Exit() error
}

type MultiWriter struct {
	writers []io.Writer
}

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (mw *MultiWriter) AddWriter(w io.Writer) {
	mw.writers = append(mw.writers, w)
}

func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}