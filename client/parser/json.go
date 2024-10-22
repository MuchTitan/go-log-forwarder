package parser

import (
	"encoding/json"
	"log-forwarder-client/util"
)

type Json struct {
	InputMatch string
	TimeKey    string
	TimeFormat string
}

func (j Json) Apply(data *util.Event) error {
	var decodedData map[string]interface{}
	err := json.Unmarshal(data.RawData, &decodedData)
	if err != nil {
		return err
	}

	data.ParsedData = decodedData

	util.AppendParsedDataWithMetadata(data)

	if j.TimeKey == "" || j.TimeFormat == "" {
		return nil
	}

	err = ExtractTimeKey(j.TimeKey, j.TimeFormat, data)
	if err != nil {
		return err
	}

	return nil
}

func (j Json) GetMatch() string {
	if j.InputMatch == "" {
		return "*"
	}
	return j.InputMatch
}
