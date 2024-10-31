package parser

import (
	"fmt"
	"log-forwarder-client/util"
	"log/slog"
	"regexp"

	"github.com/mitchellh/mapstructure"
)

type Regex struct {
	logger      *slog.Logger
	FilterMatch string            `mapstructure:"Match"`
	Pattern     string            `mapstructure:"Pattern"`
	TimeKey     string            `mapstructure:"TimeKey"`
	TimeFormat  string            `mapstructure:"TimeFormat"`
	Types       map[string]string `mapstructure:"Types"`
	AllowEmpty  bool              `mapstructure:"AllowEmpty"`
}

func ParseRegex(input map[string]interface{}, logger *slog.Logger) (Regex, error) {
	regex := Regex{}
	err := mapstructure.Decode(input, &regex)
	if err != nil {
		return regex, err
	}

	if regex.Pattern == "" {
		return regex, fmt.Errorf("For regex parser is not Pattern defiend")
	}
	regex.logger = logger

	return regex, nil
}

func (r Regex) Apply(data *util.Event) error {
	re, err := regexp.Compile(r.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %v", err)
	}

	lineData := string(data.RawData)
	matches := re.FindStringSubmatch(lineData)

	if matches == nil {
		return fmt.Errorf("no matches found for line data: '%s'", lineData)
	}

	// Extract named groups
	decodedData := make(map[string]interface{})
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			if r.AllowEmpty {
				decodedData[name] = matches[i]
				continue
			}
			value := matches[i]
			if value != "" {
				decodedData[name] = value
			}
		}
	}
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			if r.AllowEmpty {
				decodedData[name] = matches[i]
				continue
			}
			value := matches[i]
			if value != "" {
				decodedData[name] = value
			}
		}
	}

	data.ParsedData = decodedData
	data.ParsedData = util.MergeMaps(data.ParsedData, data.Metadata)

	if r.TimeKey == "" || r.TimeFormat == "" {
		return nil
	}

	err = ExtractTimeKey(r.TimeKey, r.TimeFormat, data)
	if err != nil {
		fmt.Println(err)
	}

	return nil
}

func (r Regex) GetMatch() string {
	if r.FilterMatch == "" {
		return "*"
	}

	return r.FilterMatch
}
