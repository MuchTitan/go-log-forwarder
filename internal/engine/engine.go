package engine

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/filter"
	"github.com/MuchTitan/go-log-forwarder/internal/input"
	"github.com/MuchTitan/go-log-forwarder/internal/output"
	"github.com/MuchTitan/go-log-forwarder/internal/parser"
	"github.com/sirupsen/logrus"
)

type Engine struct {
	inputs      []input.Plugin
	parsers     []parser.Plugin
	filters     []filter.Plugin
	outputs     []output.Plugin
	pipeline    chan internal.Event
	errorEvents []internal.ErrorEvent
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	FailedEvents
}

type FailedEvents struct {
	errorCh chan internal.ErrorEvent
	// Add persistent queue for failed events
	failedEventsQueue []internal.ErrorEvent
	// Add mutex for thread-safe access to failed events
	failedEventsMutex sync.Mutex
	// Events dropped during the whole life time of the programm
	totaldroppedEvents int
	// Events dropped in the last minute
	droppedEventLastMinute int
	// Add retry configuration
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
}

func NewEngine() *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		pipeline:    make(chan internal.Event),
		errorEvents: []internal.ErrorEvent{},
		ctx:         ctx,
		cancel:      cancel,
		FailedEvents: FailedEvents{
			errorCh:           make(chan internal.ErrorEvent),
			failedEventsQueue: []internal.ErrorEvent{},
			maxRetries:        3,
			retryBaseDelay:    1 * time.Second,
			retryMaxDelay:     30 * time.Second,
		},
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
	output.SetErrorChannel(e.errorCh)
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
				logrus.WithError(err).Errorf("Coundnt start input: %s.", in.Name())
			}
		}(in)
	}

	// Start processing worker
	e.wg.Add(2)
	go e.processRecords()
	go e.processFailedEvents()

	return nil
}

func (e *Engine) processFailedEvents() {
	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	defer e.wg.Done()

	for {
		select {
		case <-e.ctx.Done():
			return
		case errorEvent := <-e.errorCh:
			e.failedEventsMutex.Lock()
			e.failedEventsQueue = append(e.failedEventsQueue, errorEvent)
			e.failedEventsMutex.Unlock()

			// Start retry process in a separate goroutine
			go e.retryFailedEvent(errorEvent)
		case <-ticker.C:
			e.failedEventsMutex.Lock()
			failedEventsCount := e.droppedEventLastMinute
			e.droppedEventLastMinute = 0
			e.failedEventsMutex.Unlock()
			if failedEventsCount > 10 {
				logrus.Warnf("Dropped %d events in the last minute", failedEventsCount)
			}
		}
	}
}

func (e *Engine) retryFailedEvent(errorEvent internal.ErrorEvent) {
	retryCount := 0
	delay := e.retryBaseDelay

	for retryCount < e.maxRetries {
		select {
		case <-e.ctx.Done():
			return
		case <-time.After(delay):
			// Find the original output plugin
			for _, output := range e.outputs {
				if output.Type() == errorEvent.Type && output.GetMatch() == errorEvent.Match {
					// Try to write the event again
					if err := output.WriteErrorEvent(errorEvent); err == nil {
						// Success! Remove from failed events queue
						logrus.WithField("plugin", errorEvent.Type.String()).WithField("retryAttempt", retryCount).Debug("Retry succeeded!")
						e.DeleteEventFromQueue(errorEvent)
					}
					break
				}
			}

			// Exponential backoff with jitter
			delay = min(time.Duration(float64(delay)*1.5), e.retryMaxDelay)
			retryCount++
		}
	}

	e.DeleteEventFromQueue(errorEvent)

	e.failedEventsMutex.Lock()
	e.droppedEventLastMinute++
	e.totaldroppedEvents++
	e.failedEventsMutex.Unlock()

	// If we've exhausted all retries, log the failure
	logrus.WithError(errorEvent.Err).
		WithField("plugin", errorEvent.Type.String()).
		WithField("retries", retryCount).
		Debug("Failed to process event after maximum retries")
}

func (e *Engine) DeleteEventFromQueue(errorEvent internal.ErrorEvent) {
	e.failedEventsMutex.Lock()
	for i, ev := range e.failedEventsQueue {
		if compareErrorEvents(ev, errorEvent) {
			e.failedEventsQueue = slices.Delete(e.failedEventsQueue, i, i+1)
			break
		}
	}
	e.failedEventsMutex.Unlock()
}

// Helper function to compare error events - implement this based on your ErrorEvent struct
func compareErrorEvents(a, b internal.ErrorEvent) bool {
	// Compare relevant fields that would identify the same event
	// For example:
	return a.Type == b.Type &&
		a.Match == b.Match &&
		a.Err.Error() == b.Err.Error() &&
		reflect.DeepEqual(a.Data, b.Data)
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
					logrus.WithError(err).Errorf("Coundnt filter event")
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
			logrus.WithError(err).WithField("writer", output.Name()).Error("[Engine] Coundnt write to output")
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

	logrus.Warnf("During the lifetime of the programm it dropped %d events", e.totaldroppedEvents)

	return nil
}

// SetRetryConfig sets the retry configuration for the engine
func (e *Engine) SetRetryConfig(maxRetries int, retryBaseDelay, retryMaxDelay time.Duration) {
	if maxRetries > 0 {
		e.maxRetries = maxRetries
	}
	if retryBaseDelay > time.Duration(0) {
		e.retryBaseDelay = retryBaseDelay
	}
	if retryMaxDelay > time.Duration(0) {
		e.retryMaxDelay = retryMaxDelay
	}
}
