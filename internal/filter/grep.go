package filter

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
)

type Grep struct {
	name    string
	match   string
	op      string   // Available Operation are "and" and "or"
	regex   []string // Postitive Match sends the log
	exclude []string // Postitive Match doesent send the log
}

func (g *Grep) Name() string {
	return g.name
}

func (g *Grep) MatchTag(inputTag string) bool {
	return util.TagMatch(inputTag, g.match)
}

func (g *Grep) Init(config map[string]any) error {
	g.op = util.MustString(config["Op"])
	if g.op == "" {
		g.op = "and"
	}

	g.name = util.MustString(config["Name"])
	if g.name == "" {
		g.name = "grep"
	}

	g.match = util.MustString(config["Match"])
	if g.match == "" {
		g.match = "*"
	}

	if regex, exists := config["Regex"]; exists {
		var ok bool
		if g.regex, ok = regex.([]string); !ok {
			return errors.New("cant convert regex patterns to string array")
		}
	}

	if exclude, exists := config["Exclude"]; exists {
		var ok bool
		if g.exclude, ok = exclude.([]string); !ok {
			return errors.New("cant convert exclude patterns to string array")
		}
	}

	if g.op != "and" && g.op == "or" {
		return fmt.Errorf("unsupported logic operator '%s' in Grep Filter", g.op)
	}

	return nil
}

func (g *Grep) Process(data *internal.Event) (*internal.Event, error) {
	matches := 0
	// Check each pattern
	for _, regexString := range g.regex {
		pattern, err := regexp.Compile(regexString)
		if err != nil {
			data = nil
			return data, err
		}
		byteParsedData, _ := json.Marshal(data.ParsedData)
		if pattern.MatchString(string(byteParsedData)) {
			matches++
			// If LogicalOp is "or" and one pattern matches, return true
			if g.op == "or" {
				return data, nil
			}
		}
	}

	for _, regexString := range g.exclude {
		pattern, err := regexp.Compile(regexString)
		if err != nil {
			data = nil
			return data, err
		}
		byteParsedData, _ := json.Marshal(data.ParsedData)
		if pattern.MatchString(string(byteParsedData)) {
			matches++
			// If LogicalOp is "or" and one pattern matches, return true
			if g.op == "or" {
				return data, nil
			}
		}
	}

	if g.op == "and" && matches != (len(g.regex)+len(g.exclude)) {
		data = nil
		return data, nil
	}

	return data, nil
}

func (g *Grep) Exit() error {
	return nil
}
