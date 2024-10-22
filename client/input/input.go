package input

import (
	"encoding/json"
	"log-forwarder-client/util"
)

var ValidInputs = []string{"tail"}

type Input interface {
	Start()
	Read() <-chan util.Event
	Stop()
}

func buildMetadata(metadata map[string]interface{}) ([]byte, error) {
	return json.Marshal(metadata)
}
