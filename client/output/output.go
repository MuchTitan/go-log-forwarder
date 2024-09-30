package output

import "log-forwarder-client/parser"

var ValidOutputs = []string{"splunk", "postgresql"}

type Output interface {
	Write(data parser.ParsedData)
}

type postData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}
