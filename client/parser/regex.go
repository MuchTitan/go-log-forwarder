package parser

import (
	"fmt"
	"log-forwarder-client/util"
	"regexp"
)

type Regex struct {
	InputMatch string
	Pattern    string
	TimeKey    string
	TimeFormat string
	AllowEmpty bool
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
	util.AppendParsedDataWithMetadata(data)

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
	if r.InputMatch == "" {
		return "*"
	}

	return r.InputMatch
}
