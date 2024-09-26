package output

import (
	"encoding/json"
	"log-forwarder-client/tail"
)

var ValidOutputs = []string{"Splunk", "PostgreSQL"}

type Output interface {
	Filter([]byte) []byte
	Send(tail.LineData) ([]byte, error)
	Retry([]byte) error
}

type postData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}

func encodeLineToBytes(line tail.LineData) ([]byte, error) {
	// Create postData from Line
	pd := postData{
		FilePath:  line.Filepath,
		Data:      line.LineData,
		Num:       int(line.LineNum),
		Timestamp: line.Time.Unix(), // Convert time.Time to Unix timestamp (int64)
	}

	// Encode postData to JSON
	jsonData, err := json.Marshal(pd)
	if err != nil {
		return []byte{}, err
	}

	return jsonData, nil
}
