package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Json struct {
	TimeKey    string
	TimeFormat string
}

func (j Json) Apply(data [][]byte) (ParsedData, error) {
	jsonDecoder := json.NewDecoder(bytes.NewReader(data[0]))
	var parsedData ParsedData
	var decodedData map[string]interface{}
	err := jsonDecoder.Decode(&decodedData)
	if err != nil {
		return parsedData, err
	}

	parsedData.Data = decodedData

	// Check for metadata in input
	if len(data) > 1 {
		decodedMetadata, err := DecodeMetadata(data[1])
		if err != nil {
			return parsedData, err
		}

		for key, value := range decodedMetadata {
			if _, exists := parsedData.Data[key]; !exists {
				parsedData.Data[key] = value
			}
		}
	}

	if j.TimeKey == "" || j.TimeFormat == "" {
		return parsedData, nil
	}

	parsedData, err = ExtractTimeKey(j.TimeKey, j.TimeFormat, parsedData)
	if err != nil {
		fmt.Println(err)
	}

	return parsedData, nil
}
