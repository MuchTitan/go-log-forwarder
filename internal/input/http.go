package input

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
	"github.com/sirupsen/logrus"
)

const DefaultHttpBufferSize int64 = 5 << 20 // 5MB

type InHTTP struct {
	name       string
	tag        string
	addr       string
	listenAddr string
	inputTag   string
	port       int
	bufferSize int64
	verifyTLS  bool
	server     *http.Server
	wg         *sync.WaitGroup
	ctx        context.Context
	outputCh   chan<- internal.Event
}

func (h *InHTTP) Name() string {
	return h.name
}

func (h *InHTTP) Tag() string {
	return h.tag
}

func (h *InHTTP) Init(config map[string]interface{}) error {
	h.name = util.MustString(config["Name"])
	if h.name == "" {
		h.name = "http"
	}

	h.tag = util.MustString(config["Tag"])
	if h.tag == "" {
		h.tag = "http"
	}

	h.listenAddr = util.MustString(config["Listen"])
	if h.listenAddr == "" {
		h.listenAddr = "0.0.0.0"
	}

	if port, exists := config["Port"]; exists {
		var ok bool
		if h.port, ok = port.(int); !ok {
			return errors.New("cant convert port to int")
		}
	} else {
		h.port = 8080
	}

	if bufferSize, exists := config["BufferSize"]; exists {
		var ok bool
		if h.bufferSize, ok = bufferSize.(int64); !ok {
			return errors.New("cant convert bufferSize to int")
		}
	}
	if h.bufferSize == 0 {
		h.bufferSize = DefaultHttpBufferSize
	}
	h.addr = fmt.Sprintf("%s:%d", h.listenAddr, h.port)
	h.server = &http.Server{
		Addr:        h.addr,
		Handler:     http.DefaultServeMux,
		ReadTimeout: time.Second * 30,
	}

	h.wg = &sync.WaitGroup{}

	return nil
}

func (h *InHTTP) handleReq(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Content-Length header
	if r.ContentLength > h.bufferSize {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Read the entire body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	logLines := make(chan internal.Event, 1000)
	defer close(logLines)

	h.wg.Add(1)

	go func() {
		defer h.wg.Done()
		for event := range logLines {
			h.outputCh <- event
		}
	}()

	linenumber := 0

	lines := bytes.Split(body, []byte{'\n'})
	currTime := time.Now()

	for _, line := range lines {
		line = bytes.TrimSuffix(line, []byte{'\r'})
		// Check if there is an empty line
		if slices.Equal(line, []byte{}) {
			continue
		}
		linenumber++
		event := internal.Event{
			RawData:   string(line),
			Timestamp: currTime,
			Metadata: internal.Metadata{
				Source:  r.RemoteAddr,
				LineNum: linenumber,
			},
		}

		AddMetadata(&event, h)
		logLines <- event
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Successfully processed %d lines", linenumber)
}

func (h *InHTTP) Start(ctx context.Context, output chan<- internal.Event) error {
	h.outputCh = output
	h.ctx = ctx
	http.HandleFunc("/", h.handleReq)
	go func() {
		logrus.WithField("Addr", h.addr).Info("Starting Http Input")
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithField("Addr", h.addr).WithError(err).Error("error during http input")
		}
	}()
	return nil
}

func (h *InHTTP) Exit() error {
	logrus.Info("Stopping Http Input")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second*60)
	defer shutdownCancel()
	if err := h.server.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("error during http server shutdown")
		return err
	}
	h.wg.Wait()
	return nil
}
