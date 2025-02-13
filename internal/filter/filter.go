package filter

import (
	"github.com/MuchTitan/go-log-forwarder/internal"
)

type Plugin interface {
	internal.Plugin
	Process(record *internal.Event) (*internal.Event, error)
	MatchTag(inputTag string) bool
}
