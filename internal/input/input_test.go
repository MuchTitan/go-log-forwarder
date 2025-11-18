package input

import (
	"context"
	"os"
	"testing"

	"github.com/MuchTitan/go-log-forwarder/internal"
)

// Mock input for testing
type MockInputForHelper struct{}

func (m *MockInputForHelper) Name() string                                       { return "mock" }
func (m *MockInputForHelper) Type() internal.PluginType                          { return internal.INPUTTAIL }
func (m *MockInputForHelper) Tag() string                                        { return "test-tag" }
func (m *MockInputForHelper) Init(config map[string]any) error                   { return nil }
func (m *MockInputForHelper) Start(ctx context.Context, ch chan<- internal.Event) error { return nil }
func (m *MockInputForHelper) Exit() error                                        { return nil }

// TestAddMetadata tests the AddMetadata helper function
func TestAddMetadata(t *testing.T) {
	mockInput := &MockInputForHelper{}
	event := &internal.Event{}

	// Set hostname for predictable test
	hostname, _ := os.Hostname()

	AddMetadata(event, mockInput)

	// Verify metadata was set
	if event.Metadata.Tag != "test-tag" {
		t.Errorf("Expected tag 'test-tag', got '%s'", event.Metadata.Tag)
	}

	if event.Metadata.InputSource != internal.INPUTTAIL {
		t.Errorf("Expected InputSource INPUTTAIL, got %v", event.Metadata.InputSource)
	}

	if event.Metadata.Host != hostname {
		t.Errorf("Expected host '%s', got '%s'", hostname, event.Metadata.Host)
	}
}
