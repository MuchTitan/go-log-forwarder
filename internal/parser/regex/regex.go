package parserregex

import (
	"fmt"
	"regexp"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/parser"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
)

type Regex struct {
	name       string
	re         *regexp.Regexp
	timeKey    string
	timeFormat string
	allowEmpty bool
}

func (r *Regex) Name() string {
	return r.name
}

func (r *Regex) Init(config map[string]any) error {
	r.name = util.MustString(config["Name"])
	if r.name == "" {
		r.name = "regex"
	}

	var err error
	r.re, err = regexp.Compile(util.MustString(config["Pattern"]))
	if err != nil {
		return err
	}

	if allowEmpty, ok := config["AllowEmpty"].(bool); ok {
		r.allowEmpty = allowEmpty
	} else {
		r.allowEmpty = true
	}

	r.timeKey = util.MustString(config["TimeKey"])

	r.timeFormat = util.MustString(config["TimeFormat"])
	if r.timeFormat != "" {
		timeStr := time.Now().Format(r.timeFormat)
		if timeStr == "invalid" {
			return fmt.Errorf("not a valid time format in Regex Parser")
		}
	} else {
		r.timeFormat = time.RFC3339
	}

	return nil
}

func (r *Regex) Process(event *internal.Event) bool {
	matches := r.re.FindStringSubmatch(event.RawData)
	if matches == nil {
		return false
	}

	// Extract named groups
	decodedData := make(map[string]any)
	for i, name := range r.re.SubexpNames() {
		if i != 0 && name != "" {
			value := matches[i]
			if r.allowEmpty {
				decodedData[name] = value
				continue
			}
			if value != "" {
				decodedData[name] = value
			}
		}
	}

	event.ParsedData = decodedData

	if r.timeFormat != "" && r.timeKey != "" {
		parser.ExtractTime(event, r.timeKey, r.timeFormat)
	}

	return true
}

func (r *Regex) Exit() error {
	return nil
}
