package input

import (
	"context"
	"log/slog"
)

type InHTTP struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *slog.Logger
	doneCh  chan struct{}
	sendCh  chan [2][]byte
	address string
	port    int
}
