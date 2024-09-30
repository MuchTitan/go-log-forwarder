package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type ParsedData struct {
	Data     map[string]interface{}
	Metadata map[string]interface{}
	Time     int64
}

type Parser interface {
	Apply(data [][]byte) (ParsedData, error)
}

func ParseTime(inTime, timeFormat string) (int64, error) {
	time, err := time.Parse(timeFormat, inTime)
	if err != nil {
		return 0, err
	}

	return time.Unix(), nil
}

func DecodeMetadata(metadata []byte) (map[string]interface{}, error) {
	var decodedMetadata map[string]interface{}
	metadataJsonDecoder := json.NewDecoder(bytes.NewReader(metadata))
	err := metadataJsonDecoder.Decode(&decodedMetadata)
	if err != nil {
		return decodedMetadata, err
	}
	return decodedMetadata, nil
}

func ExtractTimeKey(timeKey, timeFormat string, data ParsedData) (ParsedData, error) {
	if time, exists := data.Data[timeKey]; exists {
		var timeStr string
		var ok bool
		if timeStr, ok = time.(string); !ok {
			return data, fmt.Errorf("Cound parse timeKey into string")
		}
		parsedTime, err := ParseTime(timeStr, timeFormat)
		if err != nil {
			return data, err
		}
		delete(data.Data, timeKey)
		fmt.Println(data.Data)
		data.Time = parsedTime
		return data, nil
	}
	return data, nil
}
