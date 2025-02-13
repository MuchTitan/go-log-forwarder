package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/filter"
	"github.com/MuchTitan/go-log-forwarder/internal/input"
	"github.com/MuchTitan/go-log-forwarder/internal/output"
	"github.com/MuchTitan/go-log-forwarder/internal/parser"
)

type Engine struct {
	inputs   []input.Plugin
	parsers  []parser.Plugin
	filters  []filter.Plugin
	outputs  []output.Plugin
	pipeline chan internal.Event
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewEngine() *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		pipeline: make(chan internal.Event, 1000),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterInput adds an input plugin to the engine
func (e *Engine) RegisterInput(input input.Plugin) {
	e.inputs = append(e.inputs, input)
}

// RegisterParser adds an parser plugin to the engine
func (e *Engine) RegisterParser(parser parser.Plugin) {
	e.parsers = append(e.parsers, parser)
}

// RegisterFilter adds a filter plugin to the engine
func (e *Engine) RegisterFilter(filter filter.Plugin) {
	e.filters = append(e.filters, filter)
}

// RegisterOutput adds an output plugin to the engine
func (e *Engine) RegisterOutput(output output.Plugin) {
	e.outputs = append(e.outputs, output)
}

// Start begins the processing pipeline
func (e *Engine) Start() error {
	// Start input plugins
	for _, in := range e.inputs {
		e.wg.Add(1)
		go func(in input.Plugin) {
			defer e.wg.Done()
			if err := in.Start(e.ctx, e.pipeline); err != nil {
				// TODO: Implement proper error handling (error channel?)
				slog.Error("[Engine] Coundnt start input", "input", in.Name(), "error", err)
			}
		}(in)
	}

	// Start processing worker
	e.wg.Add(1)
	go e.processRecords()

	return nil
}

// processRecords handles the main processing pipeline
func (e *Engine) processRecords() {
	defer e.wg.Done()

	buffer := make([]internal.Event, 0, 200)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return

		case event := <-e.pipeline:
			processedEvent := &event

			for _, parser := range e.parsers {
				if ok := parser.Process(processedEvent); ok {
					break
				}
			}

			// Apply filters
			for _, filter := range e.filters {
				if !filter.MatchTag(event.Metadata.Tag) {
					continue
				}
				var err error
				processedEvent, err = filter.Process(processedEvent)
				if err != nil {
					slog.Error("[Engine] Coundnt filter event", "filter", filter.Name(), "error", err)
					continue
				}
				if processedEvent == nil {
					// Event was filtered out
					break
				}
			}

			if processedEvent != nil {
				buffer = append(buffer, *processedEvent)
			}

			// Flush if buffer is full
			if len(buffer) >= 100 {
				e.flush(buffer)
				buffer = buffer[:0]
			}

		case <-ticker.C:
			// Periodic flush
			if len(buffer) > 0 {
				e.flush(buffer)
				buffer = buffer[:0]
			}
		}
	}
}

// flush writes records to all output plugins
func (e *Engine) flush(records []internal.Event) {
	for _, output := range e.outputs {
		if err := output.Write(records); err != nil {
			slog.Error("[Engine] Coundnt write to output", "writer", output.Name(), "error", err)
		}
	}
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() error {
	e.cancel()
	e.wg.Wait()

	// Cleanup plugins
	for _, input := range e.inputs {
		input.Exit()
	}
	for _, filter := range e.filters {
		filter.Exit()
	}
	for _, output := range e.outputs {
		output.Flush()
		output.Exit()
	}

	return nil
}
