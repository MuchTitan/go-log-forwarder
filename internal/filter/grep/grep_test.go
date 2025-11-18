package filtergrep

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
				op:      "or",
				include: []string{"error.*"},
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
				op:      "and",
				include: []string{"error.*", "critical.*"},
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
				op:      "or",
				include: []string{"[invalid"},
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

// TestGrepGetters tests simple getter methods
func TestGrepGetters(t *testing.T) {
	g := &Grep{
		name:  "test-grep",
		match: "*.log",
	}

	if g.Name() != "test-grep" {
		t.Errorf("Name() = %v, want test-grep", g.Name())
	}

	if !g.MatchTag("test.log") {
		t.Error("MatchTag() should return true for matching tag")
	}

	if g.Type() != internal.FILTERGREP {
		t.Errorf("Type() = %v, want FILTERGREP", g.Type())
	}
}

// TestGrepInit tests initialization
func TestGrepInit(t *testing.T) {
	g := &Grep{}

	config := map[string]any{
		"Name":    "test",
		"Match":   "*",
		"Include": []string{"error"},
		"Op":      "or",
	}

	err := g.Init(config)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
}

// TestGrepExit tests cleanup
func TestGrepExit(t *testing.T) {
	g := &Grep{}
	err := g.Exit()
	if err != nil {
		t.Errorf("Exit() failed: %v", err)
	}
}
