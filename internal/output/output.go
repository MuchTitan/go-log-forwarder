package output

import "github.com/MuchTitan/go-log-forwarder/internal"

type Plugin interface {
	internal.Plugin
	Write(events []internal.Event) error
	WriteErrorEvent(internal.ErrorEvent) error
	Flush() (any, error)
	GetMatch() string
	SetErrorChannel(chan<- internal.ErrorEvent)
}
