package input

import "encoding/json"

var ValidInputs = []string{"tail"}

type Input interface {
	Read() <-chan [][]byte // array contains data at index 0 and metadata at index 1
	Stop()
}

func buildMetadata(metadata map[string]interface{}) ([]byte, error) {
	data, err := json.Marshal(metadata)
	return data, err
}
