package inputhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/input"
	"github.com/stretchr/testify/assert"
)

func TestInHTTP_Init(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
		check   func(*testing.T, *InHTTP)
	}{
		{
			name: "default configuration",
			config: map[string]any{
				"Name": "",
				"Tag":  "",
			},
			wantErr: false,
			check: func(t *testing.T, h *InHTTP) {
				assert.Equal(t, "http", h.name)
				assert.Equal(t, "http", h.tag)
				assert.Equal(t, "0.0.0.0", h.listenAddr)
				assert.Equal(t, 8080, h.port)
				assert.Equal(t, DefaultHttpBufferSize, h.bufferSize)
			},
		},
		{
			name: "custom configuration",
			config: map[string]any{
				"Name":       "custom_http",
				"Tag":        "custom_tag",
				"Listen":     "127.0.0.1",
				"Port":       9090,
				"BufferSize": int64(1024 * 1024),
			},
			wantErr: false,
			check: func(t *testing.T, h *InHTTP) {
				assert.Equal(t, "custom_http", h.name)
				assert.Equal(t, "custom_tag", h.tag)
				assert.Equal(t, "127.0.0.1", h.listenAddr)
				assert.Equal(t, 9090, h.port)
				assert.Equal(t, int64(1024*1024), h.bufferSize)
			},
		},
		{
			name: "invalid port type",
			config: map[string]any{
				"Port": "8080",
			},
			wantErr: true,
		},
		{
			name: "invalid buffer size type",
			config: map[string]any{
				"BufferSize": "1024",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &InHTTP{}
			err := h.Init(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.check != nil {
				tt.check(t, h)
			}
		})
	}
}

func TestInHTTP_HandleReq(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		contentLength  int64
		expectedStatus int
		expectedLines  int
	}{
		{
			name:           "valid POST request",
			method:         "POST",
			body:           "line1\nline2\nline3",
			expectedStatus: http.StatusOK,
			expectedLines:  3,
		},
		{
			name:           "invalid method",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "empty body",
			method:         "POST",
			body:           "",
			expectedStatus: http.StatusOK,
			expectedLines:  0,
		},
		{
			name:           "body with empty lines",
			method:         "POST",
			body:           "line1\n\nline2\n\n",
			expectedStatus: http.StatusOK,
			expectedLines:  2,
		},
		{
			name:           "body too large",
			method:         "POST",
			body:           "large body",
			contentLength:  DefaultHttpBufferSize + 1,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &InHTTP{
				bufferSize: DefaultHttpBufferSize,
				wg:         &sync.WaitGroup{},
			}

			// Create output channel
			output := make(chan internal.Event)
			h.outputCh = output

			// Create test request
			body := bytes.NewBufferString(tt.body)
			req := httptest.NewRequest(tt.method, "/", body)
			if tt.contentLength > 0 {
				req.ContentLength = tt.contentLength
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Handle request
			h.handleReq(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// If successful POST, verify events
			if tt.method == "POST" && tt.expectedStatus == http.StatusOK {
				// Count received events
				receivedLines := 0
				done := make(chan bool)
				go func() {
					for range output {
						receivedLines++
					}
					done <- true
				}()

				// Wait for the server to process the lines
				time.Sleep(time.Second * 1)

				// Close output channel to end counting
				close(output)
				<-done

				assert.Equal(t, tt.expectedLines, receivedLines)
			}
		})
	}
}

func TestInHTTP_StartAndExit(t *testing.T) {
	h := &InHTTP{
		addr:       "localhost:42069", // Use random port
		bufferSize: DefaultHttpBufferSize,
		wg:         &sync.WaitGroup{},
		server:     &http.Server{},
	}

	output := make(chan internal.Event, 1000)
	ctx := context.Background()

	// Start server
	err := h.Start(ctx, output)
	assert.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test server is running by sending request
	resp, err := http.Post("http://"+h.server.Addr, "appliation/json", bytes.NewBufferString("test"))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Exit server
	err = h.Exit()
	assert.NoError(t, err)

	// Verify server has stopped by trying to connect
	_, err = http.Post("http://"+h.server.Addr, "text/plain", bytes.NewBufferString("test"))
	assert.Error(t, err)
}

func TestAddMetadata(t *testing.T) {
	event := &internal.Event{}
	inputContent := &InHTTP{
		name: "test_http",
		tag:  "test_tag",
	}

	input.AddMetadata(event, inputContent)

	assert.Equal(t, "test_http", event.Metadata.InputSource)
	assert.Equal(t, "test_tag", event.Metadata.Tag)
}
