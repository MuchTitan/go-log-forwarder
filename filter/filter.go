package filter

import "log-forwarder/global"

type Plugin interface {
	global.Plugin
	Process(record *global.Event) (*global.Event, error)
	MatchTag(inputTag string) bool
}
