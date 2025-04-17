package internal

import (
	"fmt"
	"time"
)

type Event struct {
	Timestamp  time.Time
	RawData    string
	ParsedData map[string]any
	Metadata   Metadata
}

type ErrorEvent struct {
	Err   error
	Match string
	Type  PluginType
	Data  any
}

type Metadata struct {
	Source      string
	Host        string
	Tag         string
	LineNum     int
	InputSource PluginType
}

// Plugin interface that all plugins must implement
type Plugin interface {
	Name() string
	Type() PluginType
	Init(config map[string]any) error
	Exit() error
}

type PluginType int

const (
	INPUTTAIL PluginType = iota
	INPUTHTTP
	INPUTTCP
	PARSERJSON
	PARSERREGEX
	FILTERGREP
	FILTERMODIFY
	OUTPUTSPLUNK
	OUTPUTGELF
	OUTPUTCOUNTER
	OUTPUTSTDOUT
)

func (o PluginType) String() string {
	switch o {
	case INPUTTAIL:
		return "input_tail"
	case INPUTHTTP:
		return "input_http"
	case INPUTTCP:
		return "input_tcp"
	case PARSERJSON:
		return "parser_json"
	case PARSERREGEX:
		return "parser_regex"
	case FILTERGREP:
		return "filter_grep"
	case FILTERMODIFY:
		return "filter_modify"
	case OUTPUTSPLUNK:
		return "output_splunk"
	case OUTPUTGELF:
		return "output_gelf"
	case OUTPUTCOUNTER:
		return "output_counter"
	case OUTPUTSTDOUT:
		return "output_stdout"
	default:
		return fmt.Sprintf("Unhandled Plugin %d", o)
	}
}

func ToPluginType(input string) PluginType {
	switch input {
	default:
		return 999999999
	}
}
