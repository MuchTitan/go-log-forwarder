package input

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"log-forwarder-client/util"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
)

const DefaultHttpBufferSize int64 = 5 << 20 // 5MB

type InHTTP struct {
	sendCh     chan util.Event
	server     *http.Server
	wg         *sync.WaitGroup
	addr       string
	ListenAddr string `mapstructure:"Listen"`
	InputTag   string `mapstructure:"Tag"`
	Port       int    `mapstructure:"Port"`
	BufferSize int64  `mapstructure:"BufferSize"`
	VerifyTLS  bool   `mapstructure:"VerifyTLS"`
}

func ParseHttp(input map[string]interface{}) (InHTTP, error) {
	httpObject := InHTTP{}
	err := mapstructure.Decode(input, &httpObject)
	if err != nil {
		return httpObject, err
	}

	httpObject.ListenAddr = cmp.Or(httpObject.ListenAddr, "0.0.0.0")
	httpObject.Port = cmp.Or(httpObject.Port, 8080)
	httpObject.BufferSize = cmp.Or(httpObject.BufferSize, DefaultHttpBufferSize)

	httpObject.addr = fmt.Sprintf("%s:%d", httpObject.ListenAddr, httpObject.Port)
	httpObject.server = &http.Server{
		Addr:        httpObject.addr,
		Handler:     http.DefaultServeMux,
		ReadTimeout: time.Second * 30,
	}
	httpObject.wg = &sync.WaitGroup{}

	httpObject.sendCh = make(chan util.Event)

	return httpObject, nil
}

func (h InHTTP) GetTag() string {
	if h.InputTag == "" {
		return "*"
	}
	return h.InputTag
}

func (h InHTTP) handleReq(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Content-Length header
	if r.ContentLength > h.BufferSize {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Read the entire body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	logLines := make(chan util.Event, 1000)

	h.wg.Add(1)

	go func() {
		defer h.wg.Done()
		for event := range logLines {
			h.sendCh <- event
		}
	}()

	linenumber := 0

	lines := bytes.Split(body, []byte{'\n'})
	currTime := time.Now().Unix()

	for _, line := range lines {
		line = bytes.TrimSuffix(line, []byte{'\r'})
		// Check if there is an empty line
		if slices.Equal(line, []byte{}) {
			continue
		}
		linenumber++
		logLines <- util.Event{
			RawData:     line,
			Time:        currTime,
			InputSource: "http",
			InputTag:    h.GetTag(),
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Successfully processed %d lines", linenumber)
}

func (h InHTTP) Start() {
	http.HandleFunc("/", h.handleReq)
	go func() {
		slog.Info("Starting http input", "Addr", h.addr)
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("An error occured during the http input", "Addr", h.addr, "error", err)
		}
	}()
}

func (h InHTTP) Read() <-chan util.Event {
	return h.sendCh
}

func (h InHTTP) Stop() {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second*30)
	defer shutdownCancel()
	if err := h.server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Error during http input server shutdown", "error", err)
	}
	h.wg.Wait()
	close(h.sendCh)
}
