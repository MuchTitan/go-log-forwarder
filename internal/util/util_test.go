package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagMatch(t *testing.T) {
	tests := []struct {
		name     string
		inputTag string
		match    string
		want     bool
	}{
		{"Exact match", "foo", "foo", true},
		{"Prefix match", "foobar", "foo*", true},
		{"Suffix match", "foobar", "*bar", true},
		{"Middle match", "foobarbaz", "foo*baz", true},
		{"Multiple wildcards", "foobarbaz", "f*bar*baz", true},
		{"No match", "foobar", "baz*", false},
		{"Empty pattern", "foobar", "", false},
		{"Empty input", "", "*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TagMatch(tt.inputTag, tt.match)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMergeMaps(t *testing.T) {
	m1 := map[string]any{"a": 1, "b": 2}
	m2 := map[string]any{"b": 3, "c": 4}

	result := MergeMaps(m1, m2)
	expected := map[string]any{"a": 1, "b": 3, "c": 4}
	assert.Equal(t, expected, result)
}

func TestMustString(t *testing.T) {
	assert.Equal(t, "hello", MustString("hello"))
	assert.Equal(t, "", MustString(nil))
	assert.Panics(t, func() { MustString(42) }, "Should panic on non-string type")
}
