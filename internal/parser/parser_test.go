package parser

import (
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

// TestExtractTime tests time extraction from parsed data
func TestExtractTime(t *testing.T) {
	tests := []struct {
		name       string
		event      *internal.Event
		timeKey    string
		timeFormat string
		wantTime   bool
	}{
		{
			name: "valid time extraction",
			event: &internal.Event{
				ParsedData: map[string]any{
					"timestamp": "2024-01-15T10:30:00Z",
				},
			},
			timeKey:    "timestamp",
			timeFormat: time.RFC3339,
			wantTime:   true,
		},
		{
			name: "missing time key",
			event: &internal.Event{
				ParsedData: map[string]any{
					"other": "value",
				},
			},
			timeKey:    "timestamp",
			timeFormat: time.RFC3339,
			wantTime:   false,
		},
		{
			name: "invalid time format",
			event: &internal.Event{
				ParsedData: map[string]any{
					"timestamp": "invalid-time",
				},
			},
			timeKey:    "timestamp",
			timeFormat: time.RFC3339,
			wantTime:   false,
		},
		{
			name: "non-string time value",
			event: &internal.Event{
				ParsedData: map[string]any{
					"timestamp": 12345,
				},
			},
			timeKey:    "timestamp",
			timeFormat: time.RFC3339,
			wantTime:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialTime := tt.event.Timestamp
			
			ExtractTime(tt.event, tt.timeKey, tt.timeFormat)

			if tt.wantTime {
				if tt.event.Timestamp.IsZero() {
					t.Error("Expected timestamp to be set, but it's zero")
				}
				if tt.event.Timestamp.Equal(initialTime) {
					t.Error("Expected timestamp to change, but it didn't")
				}
			} else {
				// For failed extractions, timestamp should remain unchanged
				if !tt.event.Timestamp.Equal(initialTime) {
					t.Error("Expected timestamp to remain unchanged on failed extraction")
				}
			}
		})
	}
}
