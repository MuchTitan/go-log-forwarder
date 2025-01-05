package output

import "log-forwarder/global"

type Plugin interface {
	global.Plugin
	Write(records []global.Event) error
	Flush() error
}
