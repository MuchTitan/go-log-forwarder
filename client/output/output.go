package output

import "log-forwarder-client/parser"

var ValidOutputs = []string{"splunk", "stdout"}

type Output interface {
	Write(data parser.ParsedData) error
}

type OutState struct {
	State map[string]interface{}
	Name  string
}
