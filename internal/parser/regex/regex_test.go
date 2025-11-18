package parserregex

import (
	"regexp"
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

// TestRegexType tests Type getter
func TestRegexType(t *testing.T) {
	r := &Regex{}

	if r.Type() != internal.PARSERREGEX {
		t.Errorf("Type() = %v, want PARSERREGEX", r.Type())
	}
}

// TestRegexName tests Name getter
func TestRegexName(t *testing.T) {
	r := &Regex{name: "test-regex"}

	if r.Name() != "test-regex" {
		t.Errorf("Name() = %v, want test-regex", r.Name())
	}
}

// TestRegexInit tests initialization
func TestRegexInit(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		wantError bool
		validate  func(*testing.T, *Regex)
	}{
		{
			name: "valid simple pattern",
			config: map[string]any{
				"Name":    "test",
				"Pattern": `(?P<level>\w+): (?P<message>.+)`,
			},
			wantError: false,
			validate: func(t *testing.T, r *Regex) {
				if r.name != "test" {
					t.Errorf("name = %v, want test", r.name)
				}
				if r.re == nil {
					t.Error("regex pattern not compiled")
				}
				if !r.allowEmpty {
					t.Error("allowEmpty should default to true")
				}
				if r.timeFormat != time.RFC3339 {
					t.Errorf("timeFormat = %v, want RFC3339", r.timeFormat)
				}
			},
		},
		{
			name: "default name",
			config: map[string]any{
				"Pattern": `test`,
			},
			wantError: false,
			validate: func(t *testing.T, r *Regex) {
				if r.name != "regex" {
					t.Errorf("default name = %v, want regex", r.name)
				}
			},
		},
		{
			name: "with time configuration",
			config: map[string]any{
				"Pattern":    `(?P<time>.+) (?P<msg>.+)`,
				"TimeKey":    "time",
				"TimeFormat": "2006-01-02 15:04:05",
			},
			wantError: false,
			validate: func(t *testing.T, r *Regex) {
				if r.timeKey != "time" {
					t.Error("timeKey not set")
				}
				if r.timeFormat != "2006-01-02 15:04:05" {
					t.Error("timeFormat not set")
				}
			},
		},
		{
			name: "AllowEmpty false",
			config: map[string]any{
				"Pattern":    `test`,
				"AllowEmpty": false,
			},
			wantError: false,
			validate: func(t *testing.T, r *Regex) {
				if r.allowEmpty {
					t.Error("allowEmpty should be false")
				}
			},
		},
		{
			name: "invalid regex pattern",
			config: map[string]any{
				"Pattern": `[invalid`,
			},
			wantError: true,
		},
		{
			name: "invalid time format",
			config: map[string]any{
				"Pattern":    `test`,
				"TimeFormat": "invalid",
			},
			wantError: true, // Time format validation returns error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Regex{}
			err := r.Init(tt.config)

			if tt.wantError && err == nil {
				t.Error("Init() expected error, got nil")
			} else if !tt.wantError && err != nil {
				t.Errorf("Init() unexpected error: %v", err)
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, r)
			}
		})
	}
}

// TestRegexProcess tests parsing functionality
func TestRegexProcess(t *testing.T) {
	tests := []struct {
		name        string
		regex       *Regex
		rawData     string
		expectMatch bool
		validate    func(*testing.T, *internal.Event)
	}{
		{
			name: "simple named groups",
			regex: &Regex{
				name:       "test",
				re:         regexp.MustCompile(`(?P<level>\w+): (?P<message>.+)`),
				allowEmpty: true,
			},
			rawData:     "INFO: This is a message",
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["level"] != "INFO" {
					t.Errorf("level = %v, want INFO", e.ParsedData["level"])
				}
				if e.ParsedData["message"] != "This is a message" {
					t.Errorf("message = %v, want 'This is a message'", e.ParsedData["message"])
				}
			},
		},
		{
			name: "no match",
			regex: &Regex{
				name:       "test",
				re:         regexp.MustCompile(`(?P<level>\w+): (?P<message>.+)`),
				allowEmpty: true,
			},
			rawData:     "This doesn't match the pattern",
			expectMatch: false,
		},
		{
			name: "empty capture groups with allowEmpty true",
			regex: &Regex{
				name:       "test",
				re:         regexp.MustCompile(`(?P<field1>\w*) (?P<field2>\w*)`),
				allowEmpty: true,
			},
			rawData:     " ",
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if _, exists := e.ParsedData["field1"]; !exists {
					t.Error("field1 should exist even if empty")
				}
				if _, exists := e.ParsedData["field2"]; !exists {
					t.Error("field2 should exist even if empty")
				}
			},
		},
		{
			name: "empty capture groups with allowEmpty false",
			regex: &Regex{
				name:       "test",
				re:         regexp.MustCompile(`(?P<field1>\w+)? (?P<field2>\w+)?`),
				allowEmpty: false,
			},
			rawData:     "value1 ",
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["field1"] != "value1" {
					t.Error("field1 should have value1")
				}
				if _, exists := e.ParsedData["field2"]; exists {
					t.Error("field2 should not exist when empty and allowEmpty is false")
				}
			},
		},
		{
			name: "multiple named groups",
			regex: &Regex{
				name:       "test",
				re:         regexp.MustCompile(`(?P<date>\S+) (?P<time>\S+) (?P<level>\w+) (?P<message>.+)`),
				allowEmpty: true,
			},
			rawData:     "2024-01-01 12:00:00 ERROR Something went wrong",
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if len(e.ParsedData) != 4 {
					t.Errorf("expected 4 fields, got %d", len(e.ParsedData))
				}
				if e.ParsedData["date"] != "2024-01-01" {
					t.Error("date field incorrect")
				}
				if e.ParsedData["message"] != "Something went wrong" {
					t.Error("message field incorrect")
				}
			},
		},
		{
			name: "pattern without named groups",
			regex: &Regex{
				name:       "test",
				re:         regexp.MustCompile(`\w+ \w+`),
				allowEmpty: true,
			},
			rawData:     "hello world",
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if len(e.ParsedData) != 0 {
					t.Errorf("expected 0 fields without named groups, got %d", len(e.ParsedData))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &internal.Event{
				RawData: tt.rawData,
			}

			result := tt.regex.Process(event)

			if result != tt.expectMatch {
				t.Errorf("Process() = %v, want %v", result, tt.expectMatch)
			}

			if tt.expectMatch && tt.validate != nil {
				tt.validate(t, event)
			}
		})
	}
}

// TestRegexProcess_TimeExtraction tests time parsing
func TestRegexProcess_TimeExtraction(t *testing.T) {
	r := &Regex{
		name:       "test",
		re:         regexp.MustCompile(`(?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) (?P<msg>.+)`),
		timeKey:    "timestamp",
		timeFormat: "2006-01-02 15:04:05",
		allowEmpty: true,
	}

	event := &internal.Event{
		RawData: "2024-01-15 10:30:45 Test message",
	}

	result := r.Process(event)
	if !result {
		t.Fatal("Process() should match")
	}

	// Check that timestamp is in parsed data
	if _, exists := event.ParsedData["timestamp"]; !exists {
		t.Error("timestamp should be in ParsedData")
	}

	// The ExtractTime function should have been called
	// We can't easily test the actual time parsing without mocking,
	// but we can verify the data structure is correct
	if event.ParsedData["msg"] != "Test message" {
		t.Error("msg field incorrect")
	}
}

// TestRegexExit tests Exit method
func TestRegexExit(t *testing.T) {
	r := &Regex{}
	err := r.Exit()
	if err != nil {
		t.Errorf("Exit() unexpected error: %v", err)
	}
}
