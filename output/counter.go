package output

import (
	"encoding/json"
	"fmt"
	"log-forwarder-client/util"
	"os"
	"sync"

	"github.com/mitchellh/mapstructure"
)

type OutputToCounts struct {
	mu     *sync.Mutex
	counts map[string]uint64
}

func NewOutputToCounts() *OutputToCounts {
	return &OutputToCounts{
		mu:     &sync.Mutex{},
		counts: make(map[string]uint64),
	}
}

func (oc *OutputToCounts) IncrementCounter(output string) uint64 {
	oc.mu.Lock()
	if _, exists := oc.counts[output]; !exists {
		oc.counts[output] = 0
	}
	oc.counts[output]++
	oc.mu.Unlock()
	return oc.counts[output]
}

type Counter struct {
	OutToCounts *OutputToCounts
	OutputMatch string `mapstructure:"Match"`
}

func ParseCounter(input map[string]interface{}) (Counter, error) {
	counterObject := Counter{
		OutToCounts: NewOutputToCounts(),
	}
	err := mapstructure.Decode(input, &counterObject)
	if err != nil {
		return counterObject, err
	}

	return counterObject, nil
}

func (c Counter) Write(event util.Event) error {
	count := c.OutToCounts.IncrementCounter(fmt.Sprintf("%s;%s", event.InputSource, event.InputTag))
	data := map[string]uint64{
		"count": count,
	}
	jsonData, _ := json.Marshal(data)
	_, err := fmt.Fprintln(os.Stdout, string(jsonData))
	return err
}

func (c Counter) GetMatch() string {
	if c.OutputMatch == "" {
		return "*"
	}
	return c.OutputMatch
}
