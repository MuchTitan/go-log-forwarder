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
	retry   *RetryQueue
}

func NewRouter(inWg *sync.WaitGroup, parentCtx context.Context) *Router {
	ctx, cancel := context.WithCancel(parentCtx)
	return &Router{
		wg:     inWg,
		ctx:    ctx,
		cancel: cancel,
		logger: config.GetLogger(),
		retry:  NewRetryQueue(config.GetLogger()),
	}
}

func (r *Router) SetInput(input input.Input) {
	r.input = input
}

func (r *Router) AddOutput(output output.Output) {
	r.outputs = append(r.outputs, output)
}

func (r *Router) SetParser(parser parser.Parser) {
	if r.parser != nil {
		r.logger.Warn("More than one Parser is for an input defiend")
		return
	}
	r.parser = parser
}

func (r *Router) SetFilter(filter filter.Filter) {
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
		// Apply parser
		parsedData, err := r.parser.Apply(data)
		if err != nil {
			r.logger.Warn("Coundnt parse input data", "error", err, "data", data)
			continue
		}

		// Apply filter
		pass := true
		if r.filter != nil {
			parsedData, pass = r.filter.Apply(parsedData)
		}

		// If the data passes all filters, send it to outputs
		if pass {
			statusOutputs := []output.Output{}
			for _, output := range r.outputs {
				err = output.Write(parsedData)
				if err != nil {
					statusOutputs = append(statusOutputs, output)
				}

				// Add data to retryQueue if necassary
				if len(statusOutputs) > 0 {
					r.retry.AddRetryData(parsedData, statusOutputs)
				}
			}
		}
	}
}

func (r *Router) Start() {
	if r.input == nil {
		r.logger.Error("Coundnt start router no input is defiend")
		return
	}
	if r.parser == nil {
		r.logger.Error("Coundnt start router no parser is defiend")
		return
	}
	if len(r.outputs) < 1 {
		r.logger.Error("Coundnt start router not enough outputs are defiend", "outputs length", len(r.outputs))
		return
	}
	r.wg.Add(1)
	go r.startHandlerLoop(r.input)
	go r.retry.RetryHandlerLoop()
}

func (r *Router) Stop() {
	r.cancel()
	r.retry.Stop()
	r.wg.Wait()
}
