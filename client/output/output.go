package output

import "log-forwarder-client/parser"

var ValidOutputs = []string{"splunk", "stdout"}

type Output interface {
	Write(data parser.ParsedData) error
	GetState() OutState
}

type OutState struct {
	State map[string]interface{}
	Name  string
}
