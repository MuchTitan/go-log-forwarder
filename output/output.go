package output

import "github.com/MuchTitan/go-log-forwarder/global"

type Plugin interface {
	global.Plugin
	Write(records []global.Event) error
	Flush() error
}
