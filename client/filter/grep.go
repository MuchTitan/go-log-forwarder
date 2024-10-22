package filter

import (
	"encoding/json"
	"log-forwarder-client/util"
	"regexp"
)

type Grep struct {
	FilterMatch string
	Op          string   // Available Operation are "and" and "or"
	Regex       []string // Postitive Match sends the log
	Exclude     []string // Postitive Match doesent send the log
}

func (g Grep) Apply(data *util.Event) (bool, error) {
	matches := 0

	// Check each pattern
	for _, regexString := range g.Regex {
		pattern := regexp.MustCompile(regexString)
		byteParsedData, _ := json.Marshal(data.ParsedData)
		if pattern.MatchString(string(byteParsedData)) {
			matches++
			// If LogicalOp is "or" and one pattern matches, return true
			if g.Op == "or" {
				return true, nil
			}
		}
	}
	return g.Op == "and" && matches == (len(g.Regex)+len(g.Exclude)), nil
}

func (g Grep) GetMatch() string {
	if g.FilterMatch == "" {
		return "*"
	}
	return g.FilterMatch
}
