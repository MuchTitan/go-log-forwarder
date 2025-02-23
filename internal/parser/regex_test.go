package parser

import (
	"regexp"
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/stretchr/testify/assert"
)

func TestRegexParser_Init(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]any
		wantError  bool
		wantParser Regex
	}{
		{
			name: "valid config with all fields",
			config: map[string]any{
				"Name":       "custom_regex",
				"Pattern":    `(?P<level>\w+)\s+(?P<message>.+)`,
				"TimeKey":    "timestamp",
				"TimeFormat": time.RFC3339,
				"AllowEmpty": true,
			},
			wantError: false,
			wantParser: Regex{
				name:       "custom_regex",
				timeKey:    "timestamp",
				timeFormat: time.RFC3339,
				allowEmpty: true,
			},
		},
		{
			name: "empty name defaults to regex",
			config: map[string]any{
				"Name":    "",
				"Pattern": `(?P<level>\w+)`,
			},
			wantError: false,
			wantParser: Regex{
				name:       "regex",
				timeFormat: time.RFC3339,
			},
		},
		{
			name: "invalid regex pattern",
			config: map[string]any{
				"Name":    "test",
				"Pattern": `(?P<invalid`,
			},
			wantError: true,
		},
		{
			name: "invalid time format",
			config: map[string]any{
				"Name":       "test",
				"Pattern":    `(?P<level>\w+)`,
				"TimeFormat": "invalid",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &Regex{}
			err := parser.Init(tt.config)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantParser.name, parser.name)
				assert.Equal(t, tt.wantParser.timeKey, parser.timeKey)
				assert.Equal(t, tt.wantParser.timeFormat, parser.timeFormat)
				assert.Equal(t, tt.wantParser.allowEmpty, parser.allowEmpty)
				assert.NotNil(t, parser.re)
			}
		})
	}
}

func TestRegexParser_Process(t *testing.T) {
	tests := []struct {
		name        string
		parser      *Regex
		inputEvent  *internal.Event
		wantSuccess bool
		wantParsed  map[string]any
	}{
		{
			name: "basic log pattern",
			parser: &Regex{
				re:         regexp.MustCompile(`(?P<level>\w+)\s+(?P<message>.+)`),
				allowEmpty: false,
			},
			inputEvent: &internal.Event{
				RawData: "INFO this is a test message",
			},
			wantSuccess: true,
			wantParsed: map[string]any{
				"level":   "INFO",
				"message": "this is a test message",
			},
		},
		{
			name: "with timestamp",
			parser: &Regex{
				re:         regexp.MustCompile(`(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)\s+(?P<level>\w+)\s+(?P<message>.+)`),
				timeKey:    "timestamp",
				timeFormat: time.RFC3339,
			},
			inputEvent: &internal.Event{
				RawData: "2024-02-20T15:04:05Z INFO test message",
			},
			wantSuccess: true,
			wantParsed: map[string]any{
				"timestamp": "2024-02-20T15:04:05Z",
				"level":     "INFO",
				"message":   "test message",
			},
		},
		{
			name: "empty fields with allowEmpty=false",
			parser: &Regex{
				re:         regexp.MustCompile(`(?P<level>\w*)\s+(?P<message>.*)`),
				allowEmpty: false,
			},
			inputEvent: &internal.Event{
				RawData: " test",
			},
			wantSuccess: true,
			wantParsed: map[string]any{
				"message": "test",
			},
		},
		{
			name: "empty fields with allowEmpty=true",
			parser: &Regex{
				re:         regexp.MustCompile(`(?P<level>\w*)\s+(?P<message>.*)`),
				allowEmpty: true,
			},
			inputEvent: &internal.Event{
				RawData: " test",
			},
			wantSuccess: true,
			wantParsed: map[string]any{
				"level":   "",
				"message": "test",
			},
		},
		{
			name: "no match",
			parser: &Regex{
				re: regexp.MustCompile(`(?P<level>ERROR)\s+(?P<message>.+)`),
			},
			inputEvent: &internal.Event{
				RawData: "INFO test message",
			},
			wantSuccess: false,
			wantParsed:  nil,
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

func TestRegexParser_Name(t *testing.T) {
	parser := &Regex{name: "test_parser"}
	assert.Equal(t, "test_parser", parser.Name())
}

func TestRegexParser_Exit(t *testing.T) {
	parser := &Regex{}
	assert.NoError(t, parser.Exit())
}
