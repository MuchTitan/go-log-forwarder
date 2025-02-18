package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MuchTitan/go-log-forwarder/global"
	"github.com/MuchTitan/go-log-forwarder/util"
)

type Stdout struct {
	name       string
	format     string             // Output format (json, template, plain)
	template   *template.Template // Custom output template
	jsonIndent bool               // Whether to indent JSON output
	mutex      sync.Mutex         // Ensures atomic writes to stdout
	colors     bool               // Enable/disable colored output
	match      string
}

func (s *Stdout) Name() string {
	return s.name
}

func (s *Stdout) Init(config map[string]interface{}) error {
	// Set default format
	s.name = util.MustString(config["Name"])
	if s.name == "" {
		s.name = "stdout"
	}

	s.format = util.MustString(config["Format"])
	if s.format == "" {
		s.format = "json"
	}

	s.match = util.MustString(config["Match"])
	if s.match == "" {
		s.match = "*"
	}

	// Configure JSON indentation
	if indent, exists := config["Json_indent"]; exists {
		var ok bool
		if s.jsonIndent, ok = indent.(bool); !ok {
			return errors.New("cant convert json indent parameter to bool")
		}
	}

	// Configure colors
	if colors, exists := config["Colors"]; exists {
		var ok bool
		if s.colors, ok = colors.(bool); !ok {
			return errors.New("cant convert colors parameter to bool")
		}
	}

	// Parse custom template if provided
	if templateTmp, exists := config["Template"]; exists && templateTmp != "" {
		templateStr := util.MustString(templateTmp)
		tmpl, err := template.New("output").Parse(templateStr)
		if err != nil {
			return fmt.Errorf("failed to parse template: %v", err)
		}
		s.template = tmpl
		s.format = "template"
	}

	return nil
}

func (s *Stdout) Write(events []global.Event) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, event := range events {
		var output string
		var err error

		switch s.format {
		case "json":
			output, err = s.formatJSON(event)
		case "template":
			output, err = s.formatTemplate(event)
		case "plain":
			output, err = s.formatPlain(event)
		default:
			return fmt.Errorf("unknown format: %s", s.format)
		}

		if err != nil {
			return fmt.Errorf("failed to format record: %v", err)
		}

		if s.colors {
			output = s.colorize(output)
		}

		fmt.Fprintln(os.Stdout, output)
	}

	return nil
}

func (s *Stdout) formatJSON(event global.Event) (string, error) {
	// Create a formatted record with timestamp
	formatted := map[string]interface{}{
		"timestamp": event.Timestamp.Format(time.RFC3339),
		"tag":       event.Metadata.Tag,
		"data":      event.ParsedData,
	}

	if event.Metadata.LineNum != 0 {
		formatted["lineNum"] = event.Metadata.LineNum
	}

	if event.Metadata.Source != "" {
		formatted["path"] = event.Metadata.Source
	}

	var bytes []byte
	var err error

	if s.jsonIndent {
		bytes, err = json.MarshalIndent(formatted, "", "  ")
	} else {
		bytes, err = json.Marshal(formatted)
	}

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func (s *Stdout) formatTemplate(event global.Event) (string, error) {
	if s.template == nil {
		return "", fmt.Errorf("template not configured")
	}

	builder := &strings.Builder{}
	err := s.template.Execute(builder, struct {
		Timestamp time.Time
		Tag       string
		Data      map[string]interface{}
	}{
		Timestamp: event.Timestamp,
		Tag:       event.Metadata.Tag,
		Data:      event.ParsedData,
	})
	if err != nil {
		return "", err
	}

	return builder.String(), nil
}

func (s *Stdout) formatPlain(event global.Event) (string, error) {
	// Simple plain text format: timestamp [tag] key=value key=value ...
	var builder strings.Builder

	// Write timestamp and tag
	fmt.Fprintf(&builder, "%s [%s] ",
		event.Timestamp.Format(time.RFC3339),
		event.Metadata.Tag)

	if event.ParsedData != nil {
		// Write key-value pairs
		for key, value := range event.ParsedData {
			fmt.Fprintf(&builder, "%s=%v ", key, value)
		}
	} else {
		fmt.Fprintf(&builder, "RawData=%s", event.RawData)
	}

	return builder.String(), nil
}

func (s *Stdout) colorize(output string) string {
	// Simple colorization based on record type/content
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorGreen  = "\033[32m"
		colorYellow = "\033[33m"
		colorBlue   = "\033[34m"
	)

	switch {
	case strings.Contains(strings.ToLower(output), "error"):
		return colorRed + output + colorReset
	case strings.Contains(strings.ToLower(output), "warn"):
		return colorYellow + output + colorReset
	case strings.Contains(strings.ToLower(output), "info"):
		return colorGreen + output + colorReset
	default:
		return colorBlue + output + colorReset
	}
}

func (s *Stdout) MatchTag(inputTag string) bool {
	return util.TagMatch(inputTag, s.match)
}

func (s *Stdout) Flush() error {
	// No buffering, so no flush needed
	return nil
}

func (s *Stdout) Exit() error {
	// No cleanup needed
	return nil
}
