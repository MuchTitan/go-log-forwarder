package parser

import (
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/stretchr/testify/assert"
)

func TestJsonParser_Process(t *testing.T) {
	tests := []struct {
		name        string
		parser      Json
		inputEvent  *internal.Event
		wantSuccess bool
		wantParsed  map[string]any
	}{
		{
			name: "valid json with timestamp",
			parser: Json{
				timeKey:    "timestamp",
				timeFormat: time.RFC3339,
			},
			inputEvent: &internal.Event{
				RawData: `{"timestamp":"2024-02-20T15:04:05Z","message":"test log"}`,
			},
			wantSuccess: true,
			wantParsed: map[string]any{
				"timestamp": "2024-02-20T15:04:05Z",
				"message":   "test log",
			},
		},
		{
			name:   "invalid json",
			parser: Json{},
			inputEvent: &internal.Event{
				RawData: `{"invalid json`,
			},
			wantSuccess: false,
			wantParsed:  nil,
		},
		{
			name:   "empty json",
			parser: Json{},
			inputEvent: &internal.Event{
				RawData: `{}`,
			},
			wantSuccess: true,
			wantParsed:  map[string]any{},
		},
		{
			name:   "nested json",
			parser: Json{},
			inputEvent: &internal.Event{
				RawData: `{"data":{"nested":"value"},"message":"test"}`,
			},
			wantSuccess: true,
			wantParsed: map[string]any{
				"data": map[string]any{
					"nested": "value",
				},
				"message": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success := tt.parser.Process(tt.inputEvent)
			assert.Equal(t, tt.wantSuccess, success)

			if tt.wantSuccess {
				assert.Equal(t, tt.wantParsed, tt.inputEvent.ParsedData)
			}
		})
	}
}
