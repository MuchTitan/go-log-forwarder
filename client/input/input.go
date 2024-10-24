package input

import (
	"encoding/json"
	"log-forwarder-client/util"
)

var AvailableInputs []Input

type Input interface {
	Start()
	GetTag() string
	Read() <-chan util.Event
	Stop()
}

func buildMetadata(metadata map[string]interface{}) ([]byte, error) {
	return json.Marshal(metadata)
}
