package router

import (
	"context"
	"database/sql"
	"log-forwarder-client/database"
	"log-forwarder-client/filter"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log/slog"
	"sync"
	"time"
)

type Router struct {
	input      input.Input
	outputs    []output.Output
	parser     parser.Parser
	filter     filter.Filter
	wg         *sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *slog.Logger
	retryQueue *RetryQueue
	db         *sql.DB
	dbID       int64
}

func NewRouter(inWg *sync.WaitGroup, parentCtx context.Context, logger *slog.Logger) *Router {
	ctx, cancel := context.WithCancel(parentCtx)
	db := database.GetDB()
	return &Router{
		wg:         inWg,
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		retryQueue: NewRetryQueue(logger),
		db:         db,
	}
}

func (r *Router) AddOutput(output output.Output) {
	r.outputs = append(r.outputs, output)
}

func (r *Router) SetInput(input input.Input) {
	r.input = input
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

func (r *Router) StartHandlerLoop() {
	defer r.wg.Done()
	// Read from input and route to outputs
	for data := range r.input.Read() {
		// Apply parser
		parsedData, err := r.parser.Apply(data)
		if err != nil {
			r.logger.Warn("Coundnt parse input data", "error", err, "data", data)
			continue
		}

		// Apply filter
		if r.filter != nil {
			var pass bool
			parsedData, pass = r.filter.Apply(parsedData)
			if !pass {
				continue
			}
		}

		// If the data passes all filters, send it to outputs
		statusOutputs := []output.Output{}
		for _, output := range r.outputs {
			err = output.Write(parsedData)
			if err != nil {
				statusOutputs = append(statusOutputs, output)
			}

			// Add data to retryQueue if necassary
			if len(statusOutputs) > 0 {
				r.retryQueue.AddRetryData(parsedData, statusOutputs)
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
		r.logger.Error("Coundnt start router no outputs are defiend")
		return
	}
	r.wg.Add(1)
	go r.StartHandlerLoop()
	go r.retryQueue.RetryHandlerLoop()
	go r.StateHandlerLoop()
}

func (r *Router) Stop() {
	r.cancel()
	r.retryQueue.Stop()
	r.wg.Wait()
	r.input.SaveState()
}

func (r *Router) StateHandlerLoop() {
	r.wg.Add(1)
	defer r.wg.Done()

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.input.SaveState()
		}
	}
}
