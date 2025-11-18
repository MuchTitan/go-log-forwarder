package outputstdout

import (
	"bytes"
	"os"
	"testing"
	"text/template"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/stretchr/testify/assert"
)

func captureStdout(f func()) string {
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestStdoutInit(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]any
		wantError bool
		validate  func(*testing.T, *Stdout)
	}{
		{
			name:      "default config",
			config:    map[string]any{},
			wantError: false,
			validate: func(t *testing.T, s *Stdout) {
				assert.Equal(t, "stdout", s.name)
				assert.Equal(t, "*", s.match)
				assert.Equal(t, "json", s.format)
			},
		},
		{
			name: "custom config",
			config: map[string]any{
				"Name":   "custom-stdout",
				"Match":  "logs.*",
				"Format": "plain",
			},
			wantError: false,
			validate: func(t *testing.T, s *Stdout) {
				assert.Equal(t, "custom-stdout", s.name)
				assert.Equal(t, "logs.*", s.match)
				assert.Equal(t, "plain", s.format)
			},
		},
		{
			name: "invalid format",
			config: map[string]any{
				"Format": "invalid",
			},
			wantError: true,
		},
		{
			name: "with JSON indent",
			config: map[string]any{
				"Format":     "json",
				"JsonIndent": true,
			},
			wantError: false,
			validate: func(t *testing.T, s *Stdout) {
				assert.True(t, s.jsonIndent)
			},
		},
		{
			name: "with colors",
			config: map[string]any{
				"Colors": true,
			},
			wantError: false,
			validate: func(t *testing.T, s *Stdout) {
				assert.True(t, s.colors)
			},
		},
		{
			name: "invalid JSON indent type",
			config: map[string]any{
				"Format":     "json",
				"JsonIndent": "invalid",
			},
			wantError: true,
		},
		{
			name: "invalid colors type",
			config: map[string]any{
				"Colors": "invalid",
			},
			wantError: true,
		},
		{
			name: "invalid template",
			config: map[string]any{
				"Template": "{{.Invalid",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Stdout{}
			err := s.Init(tt.config)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, s)
				}
			}
		})
	}
}

func TestStdoutGetters(t *testing.T) {
	s := &Stdout{
		name:  "test-stdout",
		match: "test.*",
	}

	assert.Equal(t, "test-stdout", s.Name())
	assert.Equal(t, "test.*", s.GetMatch())
	assert.Equal(t, internal.OUTPUTSTDOUT, s.Type())
}

func TestStdoutWriteJSON(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "json"})
	assert.NoError(t, err)

	events := []internal.Event{
		{
			Timestamp: time.Now(),
			Metadata:  internal.Metadata{Tag: "test"},
			ParsedData: map[string]any{
				"message": "hello",
			},
		},
	}

	output := captureStdout(func() {
		_ = s.Write(events)
	})

	assert.Contains(t, output, `"message":"hello"`)
	assert.Contains(t, output, `"tag":"test"`)
}

func TestStdoutWriteJSONIndent(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "json", "JsonIndent": true})
	assert.NoError(t, err)

	events := []internal.Event{
		{
			Timestamp: time.Now(),
			Metadata:  internal.Metadata{Tag: "test", LineNum: 42, Source: "/var/log/test.log"},
			ParsedData: map[string]any{
				"message": "hello",
			},
		},
	}

	output := captureStdout(func() {
		_ = s.Write(events)
	})

	// Indented JSON should contain newlines
	assert.Contains(t, output, "\n")
	assert.Contains(t, output, `"lineNum": 42`)
	assert.Contains(t, output, `"path": "/var/log/test.log"`)
}

func TestStdoutWritePlain(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "plain"})
	assert.NoError(t, err)

	events := []internal.Event{
		{
			Timestamp: time.Now(),
			Metadata:  internal.Metadata{Tag: "test"},
			ParsedData: map[string]any{
				"level":   "info",
				"message": "test log",
			},
		},
	}

	output := captureStdout(func() {
		_ = s.Write(events)
	})

	assert.Contains(t, output, "[test]")
	assert.Contains(t, output, "level=info")
	assert.Contains(t, output, "message=test log")
}

func TestStdoutWritePlainNoData(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "plain"})
	assert.NoError(t, err)

	events := []internal.Event{
		{
			Timestamp:  time.Now(),
			Metadata:   internal.Metadata{Tag: "test"},
			RawData:    "raw log line",
			ParsedData: nil,
		},
	}

	output := captureStdout(func() {
		_ = s.Write(events)
	})

	assert.Contains(t, output, "[test]")
	assert.Contains(t, output, "RawData=raw log line")
}

func TestStdoutWriteTemplate(t *testing.T) {
	tmpl := "{{.Timestamp}} - {{.Tag}} - {{.Data.message}}"
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "template", "Template": tmpl})
	assert.NoError(t, err)

	events := []internal.Event{
		{
			Timestamp: time.Now(),
			Metadata:  internal.Metadata{Tag: "custom"},
			ParsedData: map[string]any{
				"message": "templated output",
			},
		},
	}

	output := captureStdout(func() {
		_ = s.Write(events)
	})

	assert.Contains(t, output, " - custom - templated output")
}

func TestStdoutWriteTagMismatch(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Match": "test.*", "Format": "json"})
	assert.NoError(t, err)

	events := []internal.Event{
		{
			Timestamp:  time.Now(),
			Metadata:   internal.Metadata{Tag: "other"},
			ParsedData: map[string]any{"data": "value"},
		},
	}

	output := captureStdout(func() {
		err := s.Write(events)
		assert.NoError(t, err)
	})

	// Should not output anything since tag doesn't match
	assert.Empty(t, output)
}

func TestStdoutWriteWithColors(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "plain", "Colors": true})
	assert.NoError(t, err)

	events := []internal.Event{
		{
			Timestamp:  time.Now(),
			Metadata:   internal.Metadata{Tag: "test"},
			ParsedData: map[string]any{"level": "error", "message": "failed"},
		},
	}

	output := captureStdout(func() {
		_ = s.Write(events)
	})

	// Should contain ANSI color codes
	assert.Contains(t, output, "\033[")
}

func TestStdoutFormatJSON(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "json"})
	assert.NoError(t, err)

	event := internal.Event{
		Timestamp: time.Now(),
		Metadata:  internal.Metadata{Tag: "json-test"},
		ParsedData: map[string]any{
			"data": "value",
		},
	}

	output, err := s.formatJSON(event)
	assert.NoError(t, err)
	assert.Contains(t, output, `"data":"value"`)
	assert.Contains(t, output, `"tag":"json-test"`)
}

func TestStdoutFormatTemplate(t *testing.T) {
	s := &Stdout{}
	tmpl := template.Must(template.New("output").Parse("Log: {{.Tag}} - {{.Data.message}}"))
	s.template = tmpl
	s.format = "template"

	event := internal.Event{
		Timestamp: time.Now(),
		Metadata:  internal.Metadata{Tag: "templated"},
		ParsedData: map[string]any{
			"message": "template test",
		},
	}

	output, err := s.formatTemplate(event)
	assert.NoError(t, err)
	assert.Equal(t, "Log: templated - template test", output)
}

func TestStdoutFormatTemplateNil(t *testing.T) {
	s := &Stdout{}
	s.format = "template"
	s.template = nil

	event := internal.Event{
		Timestamp:  time.Now(),
		Metadata:   internal.Metadata{Tag: "test"},
		ParsedData: map[string]any{"data": "value"},
	}

	_, err := s.formatTemplate(event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template not configured")
}

func TestStdoutFormatPlain(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Format": "plain"})
	assert.NoError(t, err)

	event := internal.Event{
		Timestamp: time.Now(),
		Metadata:  internal.Metadata{Tag: "plain-test"},
		ParsedData: map[string]any{
			"key": "value",
		},
	}

	output, err := s.formatPlain(event)
	assert.NoError(t, err)
	assert.Contains(t, output, "[plain-test]")
	assert.Contains(t, output, "key=value")
}

func TestStdoutColorize(t *testing.T) {
	s := &Stdout{}

	colored := s.colorize("error: something went wrong")
	assert.Contains(t, colored, "\033[31m") // Red for errors

	colored = s.colorize("warn: be careful")
	assert.Contains(t, colored, "\033[33m") // Yellow for warnings

	colored = s.colorize("info: all good")
	assert.Contains(t, colored, "\033[32m") // Green for info

	colored = s.colorize("debug: normal log")
	assert.Contains(t, colored, "\033[34m") // Blue for default
}

func TestStdoutMatchTag(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Match": "test*"})
	assert.NoError(t, err)

	assert.True(t, s.MatchTag("test-event"))
	assert.False(t, s.MatchTag("other-event"))
}

func TestStdoutErrorChannel(t *testing.T) {
	s := &Stdout{}
	ch := make(chan internal.ErrorEvent, 1)

	s.SetErrorChannel(ch)
	assert.NotNil(t, s.errorCh)

	// WriteErrorEvent should not error
	err := s.WriteErrorEvent(internal.ErrorEvent{})
	assert.NoError(t, err)
}

func TestStdoutFlush(t *testing.T) {
	s := &Stdout{}
	result, err := s.Flush()
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestStdoutExit(t *testing.T) {
	s := &Stdout{}
	assert.NoError(t, s.Exit())
}
