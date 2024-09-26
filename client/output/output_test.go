package output

import (
	"log-forwarder-client/tail"
	"log-forwarder-client/utils"
	"testing"
	"time"
)

func TestSplunk_Send(t *testing.T) {
	// Create test data
	lineData := tail.LineData{
		Filepath: "/var/log/app.log",
		LineData: "Some log data",
		LineNum:  42,
		Time:     time.Now(),
	}

	inputChan := make(chan tail.LineData, 1)
	inputChan <- lineData
	close(inputChan)

	splunk := &Splunk{
		Host:        "localhost",
		Port:        8088,
		SplunkToken: "397eb6a0-140f-4b0c-a0ff-dd8878672729",
		SplunkEventConfig: SplunkEventConfig{
			SendRaw:         false,
			eventSourceType: "JSON",
			eventHost:       utils.GetHostname(),
			eventIndex:      "test",
			eventField:      map[string]interface{}{"foo": "bar", "extra": "field"},
		},
	}

	errChan := splunk.Send(inputChan)

	// Check error channel
	select {
	case err := <-errChan:
		t.Errorf("Unexpected error: %v", err)
	default:
		// No error expected
	}
	time.Sleep(time.Second * 5)
}
