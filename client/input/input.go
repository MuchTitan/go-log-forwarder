package input

import "encoding/json"

var ValidInputs = []string{"tail"}

type Input interface {
	Start()
	Read() <-chan [][]byte // array contains data at index 0 and metadata at index 1
	Stop()
	SaveState()
}

func buildMetadata(metadata map[string]interface{}) ([]byte, error) {
	return json.Marshal(metadata)
}
