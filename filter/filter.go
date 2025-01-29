package filter

import "github.com/MuchTitan/go-log-forwarder/global"

type Plugin interface {
	global.Plugin
	Process(record *global.Event) (*global.Event, error)
	MatchTag(inputTag string) bool
}
