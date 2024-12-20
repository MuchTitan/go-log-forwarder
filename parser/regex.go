package parser

import (
	"fmt"
	"log-forwarder-client/util"
	"regexp"

	"github.com/mitchellh/mapstructure"
)

type Regex struct {
	re          *regexp.Regexp
	Types       map[string]string `mapstructure:"Types"`
	FilterMatch string            `mapstructure:"Match"`
	Pattern     string            `mapstructure:"Pattern"`
	TimeKey     string            `mapstructure:"TimeKey"`
	TimeFormat  string            `mapstructure:"TimeFormat"`
	AllowEmpty  bool              `mapstructure:"AllowEmpty"`
}

func ParseRegex(input map[string]interface{}) (Regex, error) {
	regex := Regex{}
	err := mapstructure.Decode(input, &regex)
	if err != nil {
		return regex, err
	}

	if regex.Pattern == "" {
		return regex, fmt.Errorf("For regex parser is not Pattern defiend")
	}

	regex.re = regexp.MustCompile(regex.Pattern)

	return regex, nil
}

func (r Regex) Apply(data *util.Event) error {
	lineData := string(data.RawData)
	matches := r.re.FindStringSubmatch(lineData)

	if matches == nil {
		return fmt.Errorf("no matches found for line data: '%s'", lineData)
	}

	// Extract named groups
	decodedData := make(map[string]interface{})
	for i, name := range r.re.SubexpNames() {
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
	for i, name := range r.re.SubexpNames() {
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

	err := ExtractTimeKey(r.TimeKey, r.TimeFormat, data)
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
