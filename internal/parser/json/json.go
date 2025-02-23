package parserjson

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/parser"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
)

type Json struct {
	name       string
	timeKey    string
	timeFormat string
}

func (j *Json) Name() string {
	return j.name
}

func (j *Json) Init(config map[string]any) error {
	j.name = util.MustString(config["Name"])
	if j.name == "" {
		j.name = "json"
	}

	j.timeKey = util.MustString(config["TimeKey"])

	j.timeFormat = util.MustString(config["TimeFormat"])
	if j.timeFormat != "" {
		timeStr := time.Now().Format(j.timeFormat)
		if timeStr == "invalid" {
			return fmt.Errorf("not a valid time format in json Parser")
		}
	} else {
		j.timeFormat = time.RFC3339
	}

	return nil
}

func (j *Json) Process(event *internal.Event) bool {
	var parsedData map[string]any
	err := json.Unmarshal([]byte(event.RawData), &parsedData)
	if err != nil {
		return false
	}
	event.ParsedData = parsedData

	if j.timeFormat != "" && j.timeKey != "" {
		parser.ExtractTime(event, j.timeKey, j.timeFormat)
	}
	return true
}

func (j *Json) Exit() error {
	return nil
}
