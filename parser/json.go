package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MuchTitan/go-log-forwarder/global"
	"github.com/MuchTitan/go-log-forwarder/util"
)

type Json struct {
	name       string
	timeKey    string
	timeFormat string
}

func (j *Json) Name() string {
	return j.name
}

func (j *Json) Init(config map[string]interface{}) error {
	j.name = util.MustString(config["Name"])
	if j.name == "" {
		j.name = "json"
	}

	j.timeKey = util.MustString(config["TimeKey"])

	j.timeFormat = util.MustString(config["TimeFormat"])
	if j.timeFormat != "" {
		_, err := time.Parse(j.timeFormat, time.Now().String())
		if err != nil {
			return fmt.Errorf("not a valid time format in Json Parser err: %w", err)
		}
	} else {
		j.timeFormat = time.RFC3339
	}

	return nil
}

func (j *Json) Process(event *global.Event) bool {
	var parsedData map[string]interface{}
	err := json.Unmarshal([]byte(event.RawData), &parsedData)
	if err != nil {
		return false
	}
	event.ParsedData = parsedData

	if j.timeFormat != "" && j.timeKey != "" {
		ExtractTime(event, j.timeKey, j.timeFormat)
	}
	return true
}

func (j *Json) Exit() error {
	return nil
}
