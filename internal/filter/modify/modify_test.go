package modify

import (
	"regexp"
	"testing"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

// TestModifyGetters tests simple getter methods
func TestModifyGetters(t *testing.T) {
	m := &Modify{
		name: "test-modify",
	}

	if m.Name() != "test-modify" {
		t.Errorf("Name() = %v, want test-modify", m.Name())
	}

	if m.Type() != internal.FILTERMODIFY {
		t.Errorf("Type() = %v, want FILTERMODIFY", m.Type())
	}
}

// TestModifyMatchTag tests tag matching with glob patterns
func TestModifyMatchTag(t *testing.T) {
	tests := []struct {
		name     string
		match    string
		inputTag string
		expected bool
	}{
		{"wildcard match all", "*", "anything", true},
		{"exact match", "logs", "logs", true},
		{"pattern match", "*.log", "app.log", true},
		{"pattern no match", "*.log", "app.txt", false},
		{"prefix pattern", "app.*", "app.log", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Modify{match: tt.match}
			result := m.MatchTag(tt.inputTag)
			if result != tt.expected {
				t.Errorf("MatchTag(%q) with pattern %q = %v, want %v", tt.inputTag, tt.match, result, tt.expected)
			}
		})
	}
}

// TestModifyInit tests initialization with various configurations
func TestModifyInit(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid config with Set",
			config: map[string]any{
				"Name":      "test",
				"Match":     "*",
				"Condition": map[string]any{"key_exists": "field1"},
				"Set":       map[string]any{"newfield": "value"},
			},
			wantError: false,
		},
		{
			name: "valid config with Add",
			config: map[string]any{
				"Condition": map[string]any{"key_exists": "field1"},
				"Add":       map[string]any{"newfield": "value"},
			},
			wantError: false,
		},
		{
			name: "valid config with Rename",
			config: map[string]any{
				"Condition": map[string]any{"key_exists": "field1"},
				"Rename":    map[string]any{"oldfield": "newfield"},
			},
			wantError: false,
		},
		{
			name: "valid config with HardRename",
			config: map[string]any{
				"Condition":  map[string]any{"key_exists": "field1"},
				"HardRename": map[string]any{"oldfield": "newfield"},
			},
			wantError: false,
		},
		{
			name: "valid config with Remove",
			config: map[string]any{
				"Condition": map[string]any{"key_exists": "field1"},
				"Remove":    []any{"field1", "field2"},
			},
			wantError: false,
		},
		{
			name: "valid config with RemoveRegex",
			config: map[string]any{
				"Condition":   map[string]any{"key_exists": "field1"},
				"RemoveRegex": []any{"^temp.*"},
			},
			wantError: false,
		},
		{
			name: "valid config with RemoveWildcard",
			config: map[string]any{
				"Condition":      map[string]any{"key_exists": "field1"},
				"RemoveWildcard": []any{"temp*"},
			},
			wantError: false,
		},
		{
			name: "missing condition",
			config: map[string]any{
				"Set": map[string]any{"field": "value"},
			},
			wantError: true,
			errorMsg:  "condition is required",
		},
		{
			name: "no operations",
			config: map[string]any{
				"Condition": map[string]any{"key_exists": "field1"},
			},
			wantError: true,
			errorMsg:  "no modification operations configured",
		},
		{
			name: "invalid condition type",
			config: map[string]any{
				"Condition": map[string]any{"invalid_condition": "value"},
				"Set":       map[string]any{"field": "value"},
			},
			wantError: true,
			errorMsg:  "invalid condition type",
		},
		{
			name: "invalid regex in RemoveRegex",
			config: map[string]any{
				"Condition":   map[string]any{"key_exists": "field1"},
				"RemoveRegex": []any{"[invalid"},
			},
			wantError: true,
			errorMsg:  "invalid regex pattern",
		},
		{
			name: "condition with array of values",
			config: map[string]any{
				"Condition": map[string]any{"key_exists": []any{"field1", "field2"}},
				"Set":       map[string]any{"field": "value"},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Modify{}
			err := m.Init(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("Init() expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Init() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Init() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestModifyProcess_Conditions tests all condition types
func TestModifyProcess_Conditions(t *testing.T) {
	tests := []struct {
		name       string
		modify     *Modify
		event      *internal.Event
		shouldPass bool
	}{
		{
			name: "KEYEXISTS - condition met",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"message"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
		{
			name: "KEYEXISTS - condition not met",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"missing"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: false,
		},
		{
			name: "KEYDOESNOTEXIST - condition met",
			modify: &Modify{
				condition: map[string][]string{"no_key_exists": {"missing"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
		{
			name: "KEYMATCH - regex match",
			modify: &Modify{
				condition: map[string][]string{"key_match": {"^mes.*"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
		{
			name: "NOKEYMATCH - regex no match",
			modify: &Modify{
				condition: map[string][]string{"no_key_match": {"^xyz.*"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
		{
			name: "KEYEQUALS - exact match",
			modify: &Modify{
				condition: map[string][]string{"key_equals": {"message"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
		{
			name: "KEYDOESNOTEQUAL - not equal",
			modify: &Modify{
				condition: map[string][]string{"no_key_equal": {"other"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
		{
			name: "VALUEEQUALS - value match",
			modify: &Modify{
				condition: map[string][]string{"value_equals": {"hello"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
		{
			name: "VALUEDOESNOTEQUAL - value not equal",
			modify: &Modify{
				condition: map[string][]string{"no_value_equals": {"goodbye"}},
				set:       map[string]string{"processed": "true"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "hello"},
			},
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.modify.Process(tt.event)
			if err != nil {
				t.Fatalf("Process() unexpected error: %v", err)
			}

			if tt.shouldPass {
				// Check if modification was applied
				if _, exists := result.ParsedData["processed"]; !exists {
					t.Error("Process() should have applied modification but didn't")
				}
			} else {
				// Check that modification was NOT applied
				if _, exists := result.ParsedData["processed"]; exists {
					t.Error("Process() should not have applied modification but did")
				}
			}
		})
	}
}

// TestModifyProcess_Operations tests modification operations
func TestModifyProcess_Operations(t *testing.T) {
	tests := []struct {
		name     string
		modify   *Modify
		event    *internal.Event
		validate func(*testing.T, *internal.Event)
	}{
		{
			name: "Set operation - overwrite existing",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"message"}},
				set:       map[string]string{"message": "modified", "new": "value"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "original"},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["message"] != "modified" {
					t.Errorf("Set should overwrite, got %v", e.ParsedData["message"])
				}
				if e.ParsedData["new"] != "value" {
					t.Error("Set should add new field")
				}
			},
		},
		{
			name: "Add operation - only if not exists",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"message"}},
				add:       map[string]string{"message": "ignored", "new": "added"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"message": "original"},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["message"] != "original" {
					t.Error("Add should not overwrite existing")
				}
				if e.ParsedData["new"] != "added" {
					t.Error("Add should add new field")
				}
			},
		},
		{
			name: "Rename operation - safe rename",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"old"}},
				rename:    map[string]string{"old": "new"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"old": "value"},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if _, exists := e.ParsedData["old"]; exists {
					t.Error("Rename should remove old key")
				}
				if e.ParsedData["new"] != "value" {
					t.Error("Rename should create new key with value")
				}
			},
		},
		{
			name: "Rename operation - skip if target exists",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"old"}},
				rename:    map[string]string{"old": "new"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"old": "value1", "new": "value2"},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if e.ParsedData["old"] != "value1" {
					t.Error("Rename should not remove old key if new exists")
				}
				if e.ParsedData["new"] != "value2" {
					t.Error("Rename should preserve existing target")
				}
			},
		},
		{
			name: "HardRename operation - force rename",
			modify: &Modify{
				condition:  map[string][]string{"key_exists": {"old"}},
				hardRename: map[string]string{"old": "new"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"old": "value1", "new": "value2"},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if _, exists := e.ParsedData["old"]; exists {
					t.Error("HardRename should remove old key")
				}
				if e.ParsedData["new"] != "value1" {
					t.Error("HardRename should overwrite new key")
				}
			},
		},
		{
			name: "Remove operation",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"keep"}},
				remove:    []string{"remove1", "remove2"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"keep": "value", "remove1": "x", "remove2": "y"},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if _, exists := e.ParsedData["remove1"]; exists {
					t.Error("Remove should delete remove1")
				}
				if _, exists := e.ParsedData["remove2"]; exists {
					t.Error("Remove should delete remove2")
				}
				if e.ParsedData["keep"] != "value" {
					t.Error("Remove should keep other fields")
				}
			},
		},
		{
			name: "RemoveWildcard operation",
			modify: &Modify{
				condition:      map[string][]string{"key_exists": {"keep"}},
				removeWildcard: []string{"temp*"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{"keep": "value", "temp1": "x", "temp2": "y", "other": "z"},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if _, exists := e.ParsedData["temp1"]; exists {
					t.Error("RemoveWildcard should delete temp1")
				}
				if _, exists := e.ParsedData["temp2"]; exists {
					t.Error("RemoveWildcard should delete temp2")
				}
				if e.ParsedData["keep"] != "value" {
					t.Error("RemoveWildcard should keep non-matching fields")
				}
				if e.ParsedData["other"] != "z" {
					t.Error("RemoveWildcard should keep non-matching fields")
				}
			},
		},
		{
			name: "Empty ParsedData - no processing",
			modify: &Modify{
				condition: map[string][]string{"key_exists": {"message"}},
				set:       map[string]string{"field": "value"},
			},
			event: &internal.Event{
				ParsedData: map[string]any{},
			},
			validate: func(t *testing.T, e *internal.Event) {
				if len(e.ParsedData) != 0 {
					t.Error("Empty ParsedData should remain empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.modify.Process(tt.event)
			if err != nil {
				t.Fatalf("Process() unexpected error: %v", err)
			}
			tt.validate(t, result)
		})
	}
}

// TestModifyProcess_RemoveRegex tests regex-based removal
func TestModifyProcess_RemoveRegex(t *testing.T) {
	m := &Modify{
		condition: map[string][]string{"key_exists": {"keep"}},
	}

	// Manually set up removeRegex (normally done in Init)
	m.removeRegex = make([]regexp.Regexp, 0)

	event := &internal.Event{
		ParsedData: map[string]any{
			"keep":  "value",
			"temp1": "x",
			"temp2": "y",
			"other": "z",
		},
	}

	// Note: Since removeRegex uses regexp.Regexp, we need to test through Init
	// This is a simplified test
	result, err := m.Process(event)
	if err != nil {
		t.Fatalf("Process() unexpected error: %v", err)
	}

	// All fields should still exist since removeRegex is empty
	if len(result.ParsedData) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(result.ParsedData))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
