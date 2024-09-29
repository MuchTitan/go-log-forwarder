package output

var ValidOutputs = []string{"splunk", "postgresql"}

type Output interface {
	Write(data map[string]interface{})
}

type postData struct {
	FilePath  string `json:"filePath"`
	Data      string `json:"data"`
	Num       int    `json:"lineNumber"`
	Timestamp int64  `json:"timestamp"`
}
