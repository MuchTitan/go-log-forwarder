package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log-forwarder-client/util"
	"time"
)

var AvailableParser []Parser

type Parser interface {
	GetMatch() string
	Apply(*util.Event) error
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

func ExtractTimeKey(timeKey, timeFormat string, data *util.Event) error {
	if time, exists := data.ParsedData[timeKey]; exists {
		var timeStr string
		var ok bool
		if timeStr, ok = time.(string); !ok {
			return fmt.Errorf("Cound parse timeKey into string")
		}
		parsedTime, err := ParseTime(timeStr, timeFormat)
		if err != nil {
			return err
		}
		delete(data.ParsedData, timeKey)
		data.Time = parsedTime
	}
	return nil
}
