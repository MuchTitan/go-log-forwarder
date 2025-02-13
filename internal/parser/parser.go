package parser

import (
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

type Plugin interface {
	internal.Plugin
	Process(record *internal.Event) bool
}

func ExtractTime(event *internal.Event, timeKey, timeFormat string) {
	if timeValue, ok := event.ParsedData[timeKey].(string); ok {
		time, err := time.Parse(timeFormat, timeValue)
		if err != nil {
			return
		}
		event.Timestamp = time
	}
}
