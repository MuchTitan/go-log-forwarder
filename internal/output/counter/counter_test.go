package outputcounter

import (
	"sync"
	"testing"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/stretchr/testify/assert"
)

func TestCounter_Init(t *testing.T) {
	c := &Counter{}
	config := map[string]any{
		"Name":  "test_counter",
		"Match": "test.*",
	}
	err := c.Init(config)
	assert.NoError(t, err)
	assert.Equal(t, "test_counter", c.Name())
	assert.Equal(t, "test.*", c.match)
}

func TestCounter_DefaultInit(t *testing.T) {
	c := &Counter{}
	err := c.Init(map[string]any{})
	assert.NoError(t, err)
	assert.Equal(t, "counter", c.Name())
	assert.Equal(t, "*", c.match)
}

func TestCounter_IncrementCounter(t *testing.T) {
	c := &Counter{mu: sync.Mutex{}}
	c.Init(map[string]any{})

	assert.Equal(t, uint64(1), c.IncrementCounter())
	assert.Equal(t, uint64(2), c.IncrementCounter())
}

func TestCounter_Write(t *testing.T) {
	c := &Counter{}
	c.Init(map[string]any{"Match": "test"})

	events := []internal.Event{
		{Metadata: internal.Metadata{Tag: "test"}},
		{Metadata: internal.Metadata{Tag: "test"}},
		{Metadata: internal.Metadata{Tag: "ignore"}},
	}
	err := c.Write(events)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), c.count)
}

func TestCounter_FlushExit(t *testing.T) {
	c := &Counter{}
	assert.NoError(t, c.Flush())
	assert.NoError(t, c.Exit())
}
