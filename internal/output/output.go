package output

import "github.com/MuchTitan/go-log-forwarder/internal"

type Plugin interface {
	internal.Plugin
	Write(records []internal.Event) error
	Flush() error
}
