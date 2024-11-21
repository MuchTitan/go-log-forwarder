package filter

import (
	"encoding/json"
	"fmt"
	"log-forwarder-client/util"
	"regexp"

	"github.com/mitchellh/mapstructure"
)

type Grep struct {
	FilterMatch string   `mapstructure:"Match"`
	Op          string   `mapstructure:"Op"`      // Available Operation are "and" and "or"
	Regex       []string `mapstructure:"Regex"`   // Postitive Match sends the log
	Exclude     []string `mapstructure:"Exclude"` // Postitive Match doesent send the log
}

func ParseGrep(input map[string]interface{}) (Grep, error) {
	grep := Grep{}
	err := mapstructure.Decode(input, &grep)
	if err != nil {
		return grep, err
	}

	if grep.Op == "" {
		grep.Op = "and"
	}

	if grep.Op != "and" && grep.Op != "or" {
		return grep, fmt.Errorf("Unsupported Logic Operator '%s' in Grep Filter", grep.Op)
	}

	return grep, nil
}

func (g Grep) Apply(data *util.Event) bool {
	matches := 0

	// Check each pattern
	for _, regexString := range g.Regex {
		pattern, err := regexp.Compile(regexString)
		if err != nil {
			return false
		}
		byteParsedData, _ := json.Marshal(data.ParsedData)
		if pattern.MatchString(string(byteParsedData)) {
			matches++
			// If LogicalOp is "or" and one pattern matches, return true
			if g.Op == "or" {
				return true
			}
		}
	}

	for _, regexString := range g.Exclude {
		pattern, err := regexp.Compile(regexString)
		if err != nil {
			return false
		}
		byteParsedData, _ := json.Marshal(data.ParsedData)
		if pattern.MatchString(string(byteParsedData)) {
			matches++
			// If LogicalOp is "or" and one pattern matches, return true
			if g.Op == "or" {
				return true
			}
		}
	}

	return g.Op == "and" && matches == (len(g.Regex)+len(g.Exclude))
}

func (g Grep) GetMatch() string {
	if g.FilterMatch == "" {
		return "*"
	}
	return g.FilterMatch
}
