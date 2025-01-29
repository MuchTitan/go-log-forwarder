package parser

import (
	"time"

	"github.com/MuchTitan/go-log-forwarder/global"
)

type Plugin interface {
	global.Plugin
	Process(record *global.Event) bool
}

func ExtractTime(event *global.Event, timeKey, timeFormat string) {
	if timeValue, ok := event.ParsedData[timeKey].(string); ok {
		time, err := time.Parse(timeFormat, timeValue)
		if err != nil {
			return
		}
		event.Timestamp = time
	}
}
