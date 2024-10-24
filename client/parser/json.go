package parser

import (
	"encoding/json"
	"log-forwarder-client/util"
	"log/slog"

	"github.com/mitchellh/mapstructure"
)

type Json struct {
	logger      *slog.Logger
	ParserMatch string `mapstructure:"Match"`
	TimeKey     string `mapstructure:"TimeKey"`
	TimeFormat  string `mapstructure:"TimeFormat"`
}

func ParseJson(input map[string]interface{}, logger *slog.Logger) (Json, error) {
	jsonObject := Json{}
	err := mapstructure.Decode(input, &jsonObject)
	if err != nil {
		return jsonObject, err
	}
	jsonObject.logger = logger
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
