package input

import (
	"context"
	"log-forwarder-client/util"
	"log/slog"

	"github.com/mitchellh/mapstructure"
)

type InHTTP struct {
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *slog.Logger
	doneCh     chan struct{}
	sendCh     chan util.Event
	ListenAddr string `mapstructure:"Listen"`
	Port       int    `mapstructure:"Port"`
	InputTag   string `mapstructure:"Tag"`
	VerifyTLS  bool   `mapstructure:"VerifyTLS"`
}

func ParseHttp(input map[string]interface{}, logger *slog.Logger) (InHTTP, error) {
	http := InHTTP{}
	err := mapstructure.Decode(input, &http)
	if err != nil {
		return http, err
	}
	http.ctx, http.cancel = context.WithCancel(context.Background())
	http.logger = logger

	http.sendCh = make(chan util.Event)
	http.doneCh = make(chan struct{})

	return http, nil
}

func (h InHTTP) GetTag() string {
	if h.InputTag == "" {
		return "*"
	}
	return h.InputTag
}

func (h InHTTP) Start() {}

func (h InHTTP) Read() <-chan util.Event {
	return h.sendCh
}

func (h InHTTP) Stop() {}
