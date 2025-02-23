package filter

import (
	"testing"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/stretchr/testify/assert"
)

func TestGrepProcess(t *testing.T) {
	tests := []struct {
		name        string
		grep        *Grep
		input       *internal.Event
		expectNil   bool
		expectError bool
	}{
		{
			name: "matching single regex with 'or'",
			grep: &Grep{
				op:    "or",
				regex: []string{"error.*"},
			},
			input: &internal.Event{
				ParsedData: map[string]any{
					"message": "error occurred in system",
				},
			},
			expectNil:   false,
			expectError: false,
		},
		{
			name: "non-matching regex with 'and'",
			grep: &Grep{
				op:    "and",
				regex: []string{"error.*", "critical.*"},
			},
			input: &internal.Event{
				ParsedData: map[string]any{
					"message": "error occurred in system",
				},
			},
			expectNil:   true,
			expectError: false,
		},
		{
			name: "exclude pattern match",
			grep: &Grep{
				op:      "or",
				exclude: []string{"debug.*"},
			},
			input: &internal.Event{
				ParsedData: map[string]any{
					"message": "debug message",
				},
			},
			expectNil:   false,
			expectError: false,
		},
		{
			name: "invalid regex pattern",
			grep: &Grep{
				op:    "or",
				regex: []string{"[invalid"},
			},
			input: &internal.Event{
				ParsedData: map[string]any{
					"message": "test message",
				},
			},
			expectNil:   true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.grep.Process(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}
