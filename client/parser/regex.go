package parser

import (
	"fmt"
	"regexp"
)

type Regex struct {
	Pattern    string
	TimeKey    string
	TimeFormat string
	AllowEmpty bool
}

func (r Regex) Apply(data [][]byte) (ParsedData, error) {
	var parsedData ParsedData
	re, err := regexp.Compile(r.Pattern)
	if err != nil {
		return parsedData, fmt.Errorf("invalid regex pattern: %v", err)
	}

	if len(data) > 1 {
		decodeMetadata, err := DecodeMetadata(data[1])
		if err != nil {
			return parsedData, err
		}
		parsedData.Metadata = decodeMetadata
	}
	lineData := string(data[0])
	matches := re.FindStringSubmatch(lineData)

	if matches == nil {
		return parsedData, fmt.Errorf("no matches found for line data: %s", lineData)
	}

	// Extract named groups
	fields := make(map[string]interface{})
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			if r.AllowEmpty {
				fields[name] = matches[i]
				continue
			}
			value := matches[i]
			if value != "" {
				fields[name] = value
			}
		}
	}

	parsedData.Data = fields

	if r.TimeKey == "" || r.TimeFormat == "" {
		return parsedData, nil
	}

	parsedData, err = ExtractTimeKey(r.TimeKey, r.TimeFormat, parsedData)

	return parsedData, nil
}
