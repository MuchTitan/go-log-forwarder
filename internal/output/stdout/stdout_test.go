package outputstdout

import (
	"bytes"
	"html/template"
	"os"
	"testing"
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
}

func TestStdoutMatchTag(t *testing.T) {
	s := &Stdout{}
	err := s.Init(map[string]any{"Match": "test*"})
	assert.NoError(t, err)

	assert.True(t, s.MatchTag("test-event"))
	assert.False(t, s.MatchTag("other-event"))
}

func TestStdoutFlush(t *testing.T) {
	s := &Stdout{}
	assert.NoError(t, s.Flush())
}

func TestStdoutExit(t *testing.T) {
	s := &Stdout{}
	assert.NoError(t, s.Exit())
}
