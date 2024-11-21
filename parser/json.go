package parser

import (
	"encoding/json"
	"log-forwarder-client/util"

	"github.com/mitchellh/mapstructure"
)

type Json struct {
	ParserMatch string `mapstructure:"Match"`
	TimeKey     string `mapstructure:"TimeKey"`
	TimeFormat  string `mapstructure:"TimeFormat"`
}

func ParseJson(input map[string]interface{}) (Json, error) {
	jsonObject := Json{}
	err := mapstructure.Decode(input, &jsonObject)
	if err != nil {
		return jsonObject, err
	}
	return jsonObject, nil
}

func (j Json) Apply(data *util.Event) error {
	var decodedData map[string]interface{}
	err := json.Unmarshal(data.RawData, &decodedData)
	if err != nil {
		return err
	}

	data.ParsedData = decodedData

	data.ParsedData = util.MergeMaps(data.ParsedData, data.Metadata)

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
	if j.ParserMatch == "" {
		return "*"
	}
	return j.ParserMatch
}
