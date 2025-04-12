package modify

import (
	"testing"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

func TestModify_Init(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name: "valid config with set operation and condition",
			config: map[string]any{
				"Name":  "test-modify",
				"Match": "test.*",
				"Condition": map[string]any{
					KEYEXISTS: "requiredKey",
				},
				"Set": map[string]any{
					"key1": "value1",
					"key2": "value2",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with add operation and condition",
			config: map[string]any{
				"Condition": map[string]any{
					KEYDOESNOTEXIST: "missingKey",
				},
				"Add": map[string]any{
					"newKey": "newValue",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with rename operation and condition",
			config: map[string]any{
				"Condition": map[string]any{
					KEYMATCH: "test.*",
				},
				"Rename": map[string]any{
					"oldKey": "newKey",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with hard rename operation and condition",
			config: map[string]any{
				"Condition": map[string]any{
					NOKEYMATCH: "test.*",
				},
				"HardRename": map[string]any{
					"oldKey": "newKey",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with remove operation and condition",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEQUALS: "testKey",
				},
				"Remove": []any{"key1", "key2"},
			},
			wantErr: false,
		},
		{
			name: "valid config with remove regex operation and condition",
			config: map[string]any{
				"Condition": map[string]any{
					KEYDOESNOTEQUAL: "excludedKey",
				},
				"RemoveRegex": []any{"test.*", ".*key"},
			},
			wantErr: false,
		},
		{
			name: "valid config with remove wildcard operation and condition",
			config: map[string]any{
				"Condition": map[string]any{
					VALUEEQUALS: "targetValue",
				},
				"RemoveWildcard": []any{"test*", "*key"},
			},
			wantErr: false,
		},
		{
			name: "invalid config with no condition",
			config: map[string]any{
				"Name": "test-modify",
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid config with no operations",
			config: map[string]any{
				"Name": "test-modify",
				"Condition": map[string]any{
					KEYEXISTS: "requiredKey",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid config with bad regex",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: "requiredKey",
				},
				"RemoveRegex": []any{"[invalid-regex"},
			},
			wantErr: true,
		},
		{
			name: "invalid config with invalid condition type",
			config: map[string]any{
				"Condition": map[string]any{
					"invalid_condition": "value",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid config with non-map condition",
			config: map[string]any{
				"Condition": "not_a_map",
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			wantErr: true,
		},
		{
			name: "valid config with multiple key_exists conditions",
			config: map[string]any{
				"Name":  "test-modify",
				"Match": "test.*",
				"Condition": map[string]any{
					KEYEXISTS: []any{"requiredKey1", "requiredKey2"},
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple key_match conditions",
			config: map[string]any{
				"Condition": map[string]any{
					KEYMATCH: []any{"test.*", "debug.*"},
				},
				"Add": map[string]any{
					"newKey": "newValue",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple value_equals conditions",
			config: map[string]any{
				"Condition": map[string]any{
					VALUEEQUALS: []any{"ERROR", "WARNING"},
				},
				"Rename": map[string]any{
					"oldKey": "newKey",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with mixed conditions",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS:   []any{"key1", "key2"},
					KEYMATCH:    []any{"test.*", "debug.*"},
					VALUEEQUALS: []any{"ERROR", "WARNING"},
				},
				"HardRename": map[string]any{
					"oldKey": "newKey",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Modify{}
			err := m.Init(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Modify.Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestModify_Process(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		inputData map[string]interface{}
		wantData  map[string]interface{}
		wantErr   bool
	}{
		{
			name: "set operation",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: "key1",
				},
				"Set": map[string]any{
					"key1": "newValue1",
					"key2": "newValue2",
				},
			},
			inputData: map[string]interface{}{
				"key1": "oldValue1",
				"key2": "oldValue2",
				"key3": "value3",
			},
			wantData: map[string]interface{}{
				"key1": "newValue1",
				"key2": "newValue2",
				"key3": "value3",
			},
			wantErr: false,
		},
		{
			name: "add operation",
			config: map[string]any{
				"Condition": map[string]any{
					KEYDOESNOTEXIST: "newKey1",
				},
				"Add": map[string]any{
					"newKey1": "newValue1",
					"newKey2": "newValue2",
				},
			},
			inputData: map[string]interface{}{
				"key1": "value1",
			},
			wantData: map[string]interface{}{
				"key1":    "value1",
				"newKey1": "newValue1",
				"newKey2": "newValue2",
			},
			wantErr: false,
		},
		{
			name: "rename operation",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: "oldKey",
				},
				"Rename": map[string]any{
					"oldKey": "newKey",
				},
			},
			inputData: map[string]interface{}{
				"oldKey": "value",
				"key2":   "value2",
			},
			wantData: map[string]interface{}{
				"newKey": "value",
				"key2":   "value2",
			},
			wantErr: false,
		},
		{
			name: "hard rename operation",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: "oldKey",
				},
				"HardRename": map[string]any{
					"oldKey": "newKey",
				},
			},
			inputData: map[string]interface{}{
				"oldKey": "value",
				"newKey": "existingValue",
			},
			wantData: map[string]interface{}{
				"newKey": "value",
			},
			wantErr: false,
		},
		{
			name: "remove operation",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: "key1",
				},
				"Remove": []any{"key1", "key2"},
			},
			inputData: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			wantData: map[string]interface{}{
				"key3": "value3",
			},
			wantErr: false,
		},
		{
			name: "remove regex operation",
			config: map[string]any{
				"Condition": map[string]any{
					KEYMATCH: "test.*",
				},
				"RemoveRegex": []any{"test.*", ".*key"},
			},
			inputData: map[string]interface{}{
				"test1":    "value1",
				"test2":    "value2",
				"somekey":  "value3",
				"otherkey": "value4",
				"valid":    "value5",
			},
			wantData: map[string]interface{}{
				"valid": "value5",
			},
			wantErr: false,
		},
		{
			name: "remove wildcard operation",
			config: map[string]any{
				"Condition": map[string]any{
					KEYMATCH: "test.*",
				},
				"RemoveWildcard": []any{"test*", "*key"},
			},
			inputData: map[string]interface{}{
				"test1":    "value1",
				"test2":    "value2",
				"somekey":  "value3",
				"otherkey": "value4",
				"valid":    "value5",
			},
			wantData: map[string]interface{}{
				"valid": "value5",
			},
			wantErr: false,
		},
		{
			name: "condition key exists",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: "requiredKey",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputData: map[string]interface{}{
				"requiredKey": "value",
				"key2":        "value2",
			},
			wantData: map[string]interface{}{
				"requiredKey": "value",
				"key1":        "value1",
				"key2":        "value2",
			},
			wantErr: false,
		},
		{
			name: "condition key does not exist",
			config: map[string]any{
				"Condition": map[string]any{
					KEYDOESNOTEXIST: "missingKey",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputData: map[string]interface{}{
				"key2": "value2",
			},
			wantData: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "condition key match",
			config: map[string]any{
				"Condition": map[string]any{
					KEYMATCH: "test.*",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputData: map[string]interface{}{
				"test1": "value",
				"key2":  "value2",
			},
			wantData: map[string]interface{}{
				"test1": "value",
				"key1":  "value1",
				"key2":  "value2",
			},
			wantErr: false,
		},
		{
			name: "condition no key match",
			config: map[string]any{
				"Condition": map[string]any{
					NOKEYMATCH: "test.*",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputData: map[string]interface{}{
				"key2": "value2",
			},
			wantData: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "condition value equals",
			config: map[string]any{
				"Condition": map[string]any{
					VALUEEQUALS: "targetValue",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputData: map[string]interface{}{
				"key2": "targetValue",
				"key3": "otherValue",
			},
			wantData: map[string]interface{}{
				"key1": "value1",
				"key2": "targetValue",
				"key3": "otherValue",
			},
			wantErr: false,
		},
		{
			name: "condition value does not equal",
			config: map[string]any{
				"Condition": map[string]any{
					VALUEDOESNOTEQUAl: "excludedValue",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputData: map[string]interface{}{
				"key2": "otherValue",
				"key3": "anotherValue",
			},
			wantData: map[string]interface{}{
				"key1": "value1",
				"key2": "otherValue",
				"key3": "anotherValue",
			},
			wantErr: false,
		},
		{
			name: "empty input data",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: "anyKey",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputData: map[string]interface{}{},
			wantData:  map[string]interface{}{},
			wantErr:   false,
		},
		{
			name: "multiple key_exists conditions",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: []any{"key1", "key2"},
				},
				"Set": map[string]any{
					"key1": "newValue1",
					"key2": "newValue2",
				},
			},
			inputData: map[string]interface{}{
				"key1": "oldValue1",
				"key2": "oldValue2",
				"key3": "value3",
			},
			wantData: map[string]interface{}{
				"key1": "newValue1",
				"key2": "newValue2",
				"key3": "value3",
			},
			wantErr: false,
		},
		{
			name: "multiple key_match conditions",
			config: map[string]any{
				"Condition": map[string]any{
					KEYMATCH: []any{"error.*", "warning.*"},
				},
				"Set": map[string]any{
					"error_message":   "newValue1",
					"warning_message": "newValue2",
				},
			},
			inputData: map[string]interface{}{
				"error_message":   "oldValue1",
				"warning_message": "oldValue2",
				"info_message":    "value3",
			},
			wantData: map[string]interface{}{
				"error_message":   "newValue1",
				"warning_message": "newValue2",
				"info_message":    "value3",
			},
			wantErr: false,
		},
		{
			name: "multiple value_equals conditions",
			config: map[string]any{
				"Condition": map[string]any{
					VALUEEQUALS: []any{"ERROR", "WARNING"},
				},
				"Set": map[string]any{
					"severity": "HIGH",
				},
			},
			inputData: map[string]interface{}{
				"level":   "ERROR",
				"message": "test message",
			},
			wantData: map[string]interface{}{
				"level":    "ERROR",
				"message":  "test message",
				"severity": "HIGH",
			},
			wantErr: false,
		},
		{
			name: "mixed conditions with multiple values",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS:   []any{"timestamp", "level"},
					KEYMATCH:    []any{"error.*", "warning.*"},
					VALUEEQUALS: []any{"ERROR", "WARNING"},
				},
				"Set": map[string]any{
					"processed": "true",
				},
			},
			inputData: map[string]interface{}{
				"timestamp":     "2024-01-01",
				"level":         "ERROR",
				"error_message": "test error",
			},
			wantData: map[string]interface{}{
				"timestamp":     "2024-01-01",
				"level":         "ERROR",
				"error_message": "test error",
				"processed":     "true",
			},
			wantErr: false,
		},
		{
			name: "no conditions met with multiple values",
			config: map[string]any{
				"Condition": map[string]any{
					KEYEXISTS: []any{"missing1", "missing2"},
				},
				"Set": map[string]any{
					"newField": "value",
				},
			},
			inputData: map[string]interface{}{
				"existing": "value",
			},
			wantData: map[string]interface{}{
				"existing": "value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Modify{}
			if err := m.Init(tt.config); err != nil {
				t.Fatalf("Modify.Init() error = %v", err)
			}

			event := &internal.Event{
				ParsedData: tt.inputData,
			}

			got, err := m.Process(event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Modify.Process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got == nil {
					t.Error("Modify.Process() returned nil event")
					return
				}

				if len(got.ParsedData) != len(tt.wantData) {
					t.Errorf("Modify.Process() got %d fields, want %d", len(got.ParsedData), len(tt.wantData))
					return
				}

				for k, v := range tt.wantData {
					if gotVal, exists := got.ParsedData[k]; !exists || gotVal != v {
						t.Errorf("Modify.Process() field %s = %v, want %v", k, gotVal, v)
					}
				}
			}
		})
	}
}

func TestModify_MatchTag(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]any
		inputTag string
		want     bool
	}{
		{
			name: "match all",
			config: map[string]any{
				"Match": "*",
				"Condition": map[string]any{
					KEYEXISTS: "anyKey",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputTag: "any.tag",
			want:     true,
		},
		{
			name: "match specific tag",
			config: map[string]any{
				"Match": "test.*",
				"Condition": map[string]any{
					KEYEXISTS: "anyKey",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputTag: "test.tag",
			want:     true,
		},
		{
			name: "no match",
			config: map[string]any{
				"Match": "test.*",
				"Condition": map[string]any{
					KEYEXISTS: "anyKey",
				},
				"Set": map[string]any{
					"key1": "value1",
				},
			},
			inputTag: "other.tag",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Modify{}
			if err := m.Init(tt.config); err != nil {
				t.Fatalf("Modify.Init() error = %v", err)
			}

			if got := m.MatchTag(tt.inputTag); got != tt.want {
				t.Errorf("Modify.MatchTag() = %v, want %v", got, tt.want)
			}
		})
	}
}
