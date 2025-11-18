package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

// Mock implementations for testing
type MockInput struct {
	name    string
	tag     string
	started bool
	exited  bool
}

func (m *MockInput) Name() string                                              { return m.name }
func (m *MockInput) Tag() string                                               { return m.tag }
func (m *MockInput) Type() internal.PluginType                                 { return internal.INPUTTAIL }
func (m *MockInput) Init(config map[string]any) error                          { return nil }
func (m *MockInput) Start(ctx context.Context, ch chan<- internal.Event) error { m.started = true; return nil }
func (m *MockInput) Exit() error                                               { m.exited = true; return nil }

type MockParser struct {
	name string
}

func (m *MockParser) Name() string                                { return m.name }
func (m *MockParser) Type() internal.PluginType                   { return internal.PARSERJSON }
func (m *MockParser) Init(config map[string]any) error            { return nil }
func (m *MockParser) Process(event *internal.Event) bool          { return true }
func (m *MockParser) Exit() error                                 { return nil }

type MockFilter struct {
	name string
}

func (m *MockFilter) Name() string                                              { return m.name }
func (m *MockFilter) MatchTag(tag string) bool                                  { return true }
func (m *MockFilter) Type() internal.PluginType                                 { return internal.FILTERGREP }
func (m *MockFilter) Init(config map[string]any) error                          { return nil }
func (m *MockFilter) Process(event *internal.Event) (*internal.Event, error)    { return event, nil }
func (m *MockFilter) Exit() error                                               { return nil }

type MockOutput struct {
	name         string
	match        string
	writeCount   int
	flushCount   int
	exited       bool
	errChan      chan<- internal.ErrorEvent
}

func (m *MockOutput) Name() string                                  { return m.name }
func (m *MockOutput) GetMatch() string                              { return m.match }
func (m *MockOutput) Type() internal.PluginType                     { return internal.OUTPUTSTDOUT }
func (m *MockOutput) Init(config map[string]any) error              { return nil }
func (m *MockOutput) Write(events []internal.Event) error           { m.writeCount += len(events); return nil }
func (m *MockOutput) Flush() (any, error)                           { m.flushCount++; return nil, nil }
func (m *MockOutput) Exit() error                                   { m.exited = true; return nil }
func (m *MockOutput) SetErrorChannel(ch chan<- internal.ErrorEvent) { m.errChan = ch }
func (m *MockOutput) WriteErrorEvent(event internal.ErrorEvent) error { return nil }

// TestNewEngine tests engine creation
func TestNewEngine(t *testing.T) {
	engine := NewEngine()

	if engine == nil {
		t.Fatal("NewEngine() returned nil")
	}

	// Verify default retry config
	if engine.maxRetries == 0 {
		t.Error("Default maxRetries not set")
	}
	if engine.retryBaseDelay == 0 {
		t.Error("Default retryBaseDelay not set")
	}
	if engine.retryMaxDelay == 0 {
		t.Error("Default retryMaxDelay not set")
	}
}

// TestRegisterInput tests input registration
func TestRegisterInput(t *testing.T) {
	engine := NewEngine()
	input := &MockInput{name: "test-input", tag: "test"}

	engine.RegisterInput(input)

	if len(engine.inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(engine.inputs))
	}
}

// TestRegisterParser tests parser registration
func TestRegisterParser(t *testing.T) {
	engine := NewEngine()
	parser := &MockParser{name: "test-parser"}

	engine.RegisterParser(parser)

	if len(engine.parsers) != 1 {
		t.Errorf("Expected 1 parser, got %d", len(engine.parsers))
	}
}

// TestRegisterFilter tests filter registration
func TestRegisterFilter(t *testing.T) {
	engine := NewEngine()
	filter := &MockFilter{name: "test-filter"}

	engine.RegisterFilter(filter)

	if len(engine.filters) != 1 {
		t.Errorf("Expected 1 filter, got %d", len(engine.filters))
	}
}

// TestRegisterOutput tests output registration
func TestRegisterOutput(t *testing.T) {
	engine := NewEngine()
	output := &MockOutput{name: "test-output", match: "*"}

	engine.RegisterOutput(output)

	if len(engine.outputs) != 1 {
		t.Errorf("Expected 1 output, got %d", len(engine.outputs))
	}
}

// TestSetRetryConfig tests retry configuration
func TestSetRetryConfig(t *testing.T) {
	engine := NewEngine()

	maxRetries := 5
	baseDelay := 2 * time.Second
	maxDelay := 60 * time.Second

	engine.SetRetryConfig(maxRetries, baseDelay, maxDelay)

	if engine.maxRetries != maxRetries {
		t.Errorf("Expected maxRetries %d, got %d", maxRetries, engine.maxRetries)
	}
	if engine.retryBaseDelay != baseDelay {
		t.Errorf("Expected retryBaseDelay %v, got %v", baseDelay, engine.retryBaseDelay)
	}
	if engine.retryMaxDelay != maxDelay {
		t.Errorf("Expected retryMaxDelay %v, got %v", maxDelay, engine.retryMaxDelay)
	}
}

// TestSetRetryConfig_DefaultsOnInvalid tests that invalid values don't change config
func TestSetRetryConfig_DefaultsOnInvalid(t *testing.T) {
	engine := NewEngine()

	original := engine.maxRetries
	engine.SetRetryConfig(0, 0, 0) // Invalid values

	// Should not change from original
	if engine.maxRetries != original {
		t.Error("maxRetries should not change with 0 value")
	}
}

// TestStartStop tests engine start and stop
func TestStartStop(t *testing.T) {
	engine := NewEngine()

	// Register mock plugins
	input := &MockInput{name: "test-input", tag: "test"}
	output := &MockOutput{name: "test-output", match: "*"}

	engine.RegisterInput(input)
	engine.RegisterOutput(output)

	// Start engine
	err := engine.Start()
	if err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	// Verify input started
	time.Sleep(10 * time.Millisecond) // Give goroutine time to start
	if !input.started {
		t.Error("Input was not started")
	}

	// Stop engine
	err = engine.Stop()
	if err != nil {
		t.Fatalf("Failed to stop engine: %v", err)
	}

	// Give it a moment to shutdown
	time.Sleep(50 * time.Millisecond)

	// Verify cleanup
	if !input.exited {
		t.Error("Input Exit() was not called")
	}
	if !output.exited {
		t.Error("Output Exit() was not called")
	}
	if output.flushCount < 1 {
		t.Error("Output Flush() was not called")
	}
}

// TestMultiplePlugins tests engine with multiple plugins
func TestMultiplePlugins(t *testing.T) {
	engine := NewEngine()

	// Register multiple inputs
	input1 := &MockInput{name: "input1", tag: "tag1"}
	input2 := &MockInput{name: "input2", tag: "tag2"}
	engine.RegisterInput(input1)
	engine.RegisterInput(input2)

	// Register multiple parsers
	parser1 := &MockParser{name: "parser1"}
	parser2 := &MockParser{name: "parser2"}
	engine.RegisterParser(parser1)
	engine.RegisterParser(parser2)

	// Register multiple filters
	filter1 := &MockFilter{name: "filter1"}
	filter2 := &MockFilter{name: "filter2"}
	engine.RegisterFilter(filter1)
	engine.RegisterFilter(filter2)

	// Register multiple outputs
	output1 := &MockOutput{name: "output1", match: "*"}
	output2 := &MockOutput{name: "output2", match: "tag1"}
	engine.RegisterOutput(output1)
	engine.RegisterOutput(output2)

	// Verify registration
	if len(engine.inputs) != 2 {
		t.Errorf("Expected 2 inputs, got %d", len(engine.inputs))
	}
	if len(engine.parsers) != 2 {
		t.Errorf("Expected 2 parsers, got %d", len(engine.parsers))
	}
	if len(engine.filters) != 2 {
		t.Errorf("Expected 2 filters, got %d", len(engine.filters))
	}
	if len(engine.outputs) != 2 {
		t.Errorf("Expected 2 outputs, got %d", len(engine.outputs))
	}
}

// TestEngineWithNoPlugins tests engine behavior with no plugins registered
func TestEngineWithNoPlugins(t *testing.T) {
	engine := NewEngine()

	// Try to start engine with no plugins
	err := engine.Start()
	if err != nil {
		t.Fatalf("Engine should start even with no plugins: %v", err)
	}

	// Stop immediately
	engine.Stop()
}

// TestDeleteEventFromQueue tests removing events from retry queue
func TestDeleteEventFromQueue(t *testing.T) {
	engine := NewEngine()

	// Add some events to the queue (with non-nil errors to avoid nil pointer issues)
	err1 := errors.New("test error 1")
	err2 := errors.New("test error 2")
	event1 := internal.ErrorEvent{Type: internal.OUTPUTSTDOUT, Match: "test", Err: err1}
	event2 := internal.ErrorEvent{Type: internal.OUTPUTSTDOUT, Match: "other", Err: err2}

	engine.failedEventsQueue = append(engine.failedEventsQueue, event1, event2)

	// Delete first event
	engine.DeleteEventFromQueue(event1)

	if len(engine.failedEventsQueue) != 1 {
		t.Errorf("Expected 1 event in queue, got %d", len(engine.failedEventsQueue))
	}

	if engine.failedEventsQueue[0].Match != "other" {
		t.Error("Wrong event remained in queue")
	}
}

// TestCompareErrorEvents tests error event comparison
func TestCompareErrorEvents(t *testing.T) {
	err1 := errors.New("test error")
	err2 := errors.New("test error")
	err3 := errors.New("different error")

	event1 := internal.ErrorEvent{
		Type:  internal.OUTPUTSTDOUT,
		Match: "test",
		Err:   err1,
	}

	event2 := internal.ErrorEvent{
		Type:  internal.OUTPUTSTDOUT,
		Match: "test",
		Err:   err2,
	}

	event3 := internal.ErrorEvent{
		Type:  internal.OUTPUTSTDOUT,
		Match: "different",
		Err:   err3,
	}

	if !compareErrorEvents(event1, event2) {
		t.Error("Identical events should compare as equal")
	}

	if compareErrorEvents(event1, event3) {
		t.Error("Different events should not compare as equal")
	}
}
