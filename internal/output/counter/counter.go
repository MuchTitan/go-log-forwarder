package outputcounter

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
)

type Counter struct {
	name    string
	errorCh chan<- internal.ErrorEvent
	match   string
	mu      sync.Mutex
	count   uint64
}

func (c *Counter) Name() string {
	return c.name
}

func (c *Counter) Type() internal.PluginType {
	return internal.OUTPUTCOUNTER
}

func (c *Counter) GetMatch() string {
	return c.match
}

func (c *Counter) WriteErrorEvent(errEvent internal.ErrorEvent) error {
	return nil
}

func (c *Counter) SetErrorChannel(inputCh chan<- internal.ErrorEvent) {
	c.errorCh = inputCh
}

func (c *Counter) Init(config map[string]any) error {
	c.name = util.MustString(config["Name"])
	if c.name == "" {
		c.name = "counter"
	}

	c.match = util.MustString(config["Match"])
	if c.match == "" {
		c.match = "*"
	}

	c.mu = sync.Mutex{}

	return nil
}

func (c *Counter) IncrementCounter() uint64 {
	c.mu.Lock()
	c.count++
	c.mu.Unlock()
	return c.count
}

func (c *Counter) Write(events []internal.Event) error {
	for _, event := range events {
		if !util.GlobMatch(event.Metadata.Tag, c.match) {
			continue
		}
		count := c.IncrementCounter()
		data := map[string]any{
			"count": count,
		}
		jsonData, _ := json.Marshal(data)
		_, err := fmt.Println(string(jsonData))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Counter) Flush() (any, error) {
	return nil, nil
}

func (c *Counter) Exit() error {
	return nil
}
