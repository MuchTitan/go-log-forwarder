package outputcounter

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
)

type Counter struct {
	match string
	name  string
	mu    sync.Mutex
	count uint64
}

func (c *Counter) Name() string {
	return c.name
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
		if !util.TagMatch(event.Metadata.Tag, c.match) {
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

func (c *Counter) Flush() error {
	return nil
}

func (c *Counter) Exit() error {
	return nil
}
