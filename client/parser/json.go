package parser

import (
	"bytes"
	"encoding/json"
)

type Json struct {
	TimeKey string
}

func (j Json) Apply(data string) (map[string]interface{}, error) {
	jsonDecoder := json.NewDecoder(bytes.NewReader([]byte(data)))
	var decodedData map[string]interface{}
	err := jsonDecoder.Decode(&decodedData)
	if err != nil {
		return decodedData, err
	}

	return decodedData, nil
}
