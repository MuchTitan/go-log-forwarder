package input

import (
	"context"
	"os"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

type Plugin interface {
	internal.Plugin
	Start(ctx context.Context, output chan<- internal.Event) error
	Tag() string
}

func AddMetadata(event *internal.Event, in Plugin) {
	hostname, _ := os.Hostname()
	event.Metadata.InputSource = in.Type()
	event.Metadata.Tag = in.Tag()
	event.Metadata.Host = hostname
}
