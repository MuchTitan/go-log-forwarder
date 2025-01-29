package parser

import (
	"github.com/MuchTitan/go-log-forwarder/global"
	"time"
)

type Plugin interface {
	global.Plugin
	Process(record *global.Event) bool
}

func ExtractTime(event *global.Event, timeKey, timeFormat string) {
	if timeValue, exists := event.ParsedData[timeKey].(string); exists {
		time, err := time.Parse(timeFormat, timeValue)
		if err != nil {
			return
		}
		event.Timestamp = time
	}
}
