package router

import (
	"context"
	"log-forwarder-client/config"
	"log-forwarder-client/filter"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log/slog"
	"sync"
)

type Router struct {
	input   input.Input
	outputs []output.Output
	parser  parser.Parser
	filter  filter.Filter
	wg      *sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *slog.Logger
}

func NewRouter(inWg *sync.WaitGroup, parentCtx context.Context) *Router {
	ctx, cancel := context.WithCancel(parentCtx)
	cfg := config.GetApplicationConfig()
	return &Router{
		wg:     inWg,
		ctx:    ctx,
		cancel: cancel,
		logger: cfg.Logger,
	}
}

func (r *Router) AddInput(input input.Input) {
	r.input = input
}

func (r *Router) AddOutput(output output.Output) {
	r.outputs = append(r.outputs, output)
}

func (r *Router) AddParser(parser parser.Parser) {
	if r.parser != nil {
		r.logger.Warn("More than one Parser is for an input defiend")
		return
	}
	r.parser = parser
}

func (r *Router) AddFilter(filter filter.Filter) {
	if r.filter != nil {
		r.logger.Warn("More than one Filter is for an input defiend")
		return
	}
	r.filter = filter
}

func (r *Router) startHandlerLoop(in input.Input) {
	defer r.wg.Done()
	// Read from input and route to outputs
	for data := range in.Read() {
		// Init variables
		var parsedData map[string]interface{}
		var err error

		// Apply parser
		if r.parser != nil {
			parsedData, err = r.parser.Apply(data)
			if err != nil {
				r.logger.Warn("Coundnt parse input data", "data", data)
				continue
			}
		}

		// Apply filter
		pass := true
		if r.filter != nil {
			parsedData, pass = r.filter.Apply(parsedData)
		}

		// If the data passes all filters, send it to outputs
		if pass {
			for _, output := range r.outputs {
				output.Write(parsedData)
			}
		}
	}
}

func (r *Router) Start() {
	if r.input == nil {
		r.logger.Error("Coundnt start router no input is defiend")
		return
	}
	if len(r.outputs) < 1 {
		r.logger.Error("Coundnt start router not enough outputs are defiend", "outputs length", len(r.outputs))
		return
	}
	r.wg.Add(1)
	go r.startHandlerLoop(r.input)
}

func (r *Router) Stop() {
	r.cancel()
	r.wg.Wait()
}
