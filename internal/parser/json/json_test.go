package parserjson

import (
	"testing"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

// TestJsonGetters tests simple getter methods
func TestJsonGetters(t *testing.T) {
	j := &Json{
		name: "test-json",
	}

	if j.Name() != "test-json" {
		t.Errorf("Name() = %v, want test-json", j.Name())
	}

	if j.Type() != internal.PARSERJSON {
		t.Errorf("Type() = %v, want PARSERJSON", j.Type())
	}
}

// TestJsonInit tests initialization
func TestJsonInit(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		wantError bool
		validate  func(*testing.T, *Json)
	}{
		{
			name: "basic config",
			config: map[string]any{
				"Name": "test",
			},
			wantError: false,
			validate: func(t *testing.T, j *Json) {
				if j.name != "test" {
					t.Errorf("name = %v, want test", j.name)
				}
				if j.timeFormat != "2006-01-02T15:04:05Z07:00" { // RFC3339
					t.Error("default timeFormat should be RFC3339")
				}
			},
		},
		{
			name:      "default name",
			config:    map[string]any{},
			wantError: false,
			validate: func(t *testing.T, j *Json) {
				if j.name != "json" {
					t.Errorf("default name = %v, want json", j.name)
				}
			},
		},
		{
			name: "with time configuration",
			config: map[string]any{
				"Name":       "test",
				"TimeKey":    "timestamp",
				"TimeFormat": "2006-01-02 15:04:05",
			},
			wantError: false,
			validate: func(t *testing.T, j *Json) {
				if j.timeKey != "timestamp" {
					t.Error("timeKey not set correctly")
				}
				if j.timeFormat != "2006-01-02 15:04:05" {
					t.Error("timeFormat not set correctly")
				}
			},
		},
		{
			name: "invalid time format",
			config: map[string]any{
				"TimeFormat": "invalid",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &Json{}
			err := j.Init(tt.config)

			if tt.wantError && err == nil {
				t.Error("Init() expected error, got nil")
			} else if !tt.wantError && err != nil {
				t.Errorf("Init() unexpected error: %v", err)
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, j)
			}
		})
	}
}

// TestJsonProcess tests JSON parsing
func TestJsonProcess(t *testing.T) {
	tests := []struct {
		name        string
		json        *Json
		rawData     string
		expectMatch bool
		validate    func(*testing.T, *internal.Event)
	}{
		{
			name: "simple JSON object",
			json: &Json{
				name: "test",
			},
			rawData:     `{"level":"INFO","message":"test message"}`,
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["level"] != "INFO" {
					t.Errorf("level = %v, want INFO", e.ParsedData["level"])
				}
				if e.ParsedData["message"] != "test message" {
					t.Errorf("message = %v, want 'test message'", e.ParsedData["message"])
				}
			},
		},
		{
			name: "nested JSON object",
			json: &Json{
				name: "test",
			},
			rawData:     `{"user":{"name":"John","age":30},"action":"login"}`,
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["action"] != "login" {
					t.Error("action field not parsed correctly")
				}
				// Nested objects are parsed as map[string]any
				if user, ok := e.ParsedData["user"].(map[string]any); ok {
					if user["name"] != "John" {
						t.Error("nested user.name not parsed correctly")
					}
				} else {
					t.Error("user field should be a nested object")
				}
			},
		},
		{
			name: "JSON with arrays",
			json: &Json{
				name: "test",
			},
			rawData:     `{"items":["item1","item2","item3"],"count":3}`,
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if items, ok := e.ParsedData["items"].([]any); ok {
					if len(items) != 3 {
						t.Errorf("items array length = %d, want 3", len(items))
					}
				} else {
					t.Error("items should be an array")
				}
			},
		},
		{
			name: "JSON with numbers",
			json: &Json{
				name: "test",
			},
			rawData:     `{"count":42,"price":19.99,"active":true}`,
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["count"] != float64(42) {
					t.Error("count should be 42")
				}
				if e.ParsedData["active"] != true {
					t.Error("active should be true")
				}
			},
		},
		{
			name: "invalid JSON",
			json: &Json{
				name: "test",
			},
			rawData:     `{invalid json}`,
			expectMatch: false,
		},
		{
			name: "empty JSON object",
			json: &Json{
				name: "test",
			},
			rawData:     `{}`,
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if len(e.ParsedData) != 0 {
					t.Errorf("empty JSON should result in empty map, got %d fields", len(e.ParsedData))
				}
			},
		},
		{
			name: "JSON with null values",
			json: &Json{
				name: "test",
			},
			rawData:     `{"field1":"value","field2":null}`,
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["field2"] != nil {
					t.Error("field2 should be nil")
				}
			},
		},
		{
			name: "complex nested structure",
			json: &Json{
				name: "test",
			},
			rawData:     `{"data":{"nested":{"deep":"value"}},"array":[1,2,3]}`,
			expectMatch: true,
			validate: func(t *testing.T, e *internal.Event) {
				data, ok := e.ParsedData["data"].(map[string]any)
				if !ok {
					t.Fatal("data should be a map")
				}
				nested, ok := data["nested"].(map[string]any)
				if !ok {
					t.Fatal("nested should be a map")
				}
				if nested["deep"] != "value" {
					t.Error("deeply nested value not parsed correctly")
				}
			},
		},
		{
			name: "not JSON at all",
			json: &Json{
				name: "test",
			},
			rawData:     `plain text that is not JSON`,
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &internal.Event{
				RawData: tt.rawData,
			}

			result := tt.json.Process(event)

			if result != tt.expectMatch {
				t.Errorf("Process() = %v, want %v", result, tt.expectMatch)
			}

			if tt.expectMatch && tt.validate != nil {
				tt.validate(t, event)
			}
		})
	}
}

// TestJsonProcess_TimeExtraction tests time parsing
func TestJsonProcess_TimeExtraction(t *testing.T) {
	j := &Json{
		name:       "test",
		timeKey:    "timestamp",
		timeFormat: "2006-01-02T15:04:05Z07:00",
	}

	event := &internal.Event{
		RawData: `{"timestamp":"2024-01-15T10:30:45Z","message":"test"}`,
	}

	result := j.Process(event)
	if !result {
		t.Fatal("Process() should parse valid JSON")
	}

	// Check that timestamp is in parsed data
	if _, exists := event.ParsedData["timestamp"]; !exists {
		t.Error("timestamp should be in ParsedData")
	}

	// Verify message field
	if event.ParsedData["message"] != "test" {
		t.Error("message field incorrect")
	}
}

// TestJsonExit tests cleanup
func TestJsonExit(t *testing.T) {
	j := &Json{}
	err := j.Exit()
	if err != nil {
		t.Errorf("Exit() failed: %v", err)
	}
}
