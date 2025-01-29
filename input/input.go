package input

import (
	"context"
	"github.com/MuchTitan/go-log-forwarder/global"
	"os"
)

type Plugin interface {
	global.Plugin
	Start(ctx context.Context, output chan<- global.Event) error
	Tag() string
}

func AddMetadata(event *global.Event, in Plugin) {
	hostname, _ := os.Hostname()
	event.Metadata.InputSource = in.Name()
	event.Metadata.Tag = in.Tag()
	event.Metadata.Host = hostname
}
