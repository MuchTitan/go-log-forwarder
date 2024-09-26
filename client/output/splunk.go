package output

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log-forwarder-client/tail"
	"log-forwarder-client/utils"
	"net/http"
	"time"
)

type SplunkEventConfig struct {
	EventKey        string                 // Key for a single value
	EventHost       string                 // Source Host (default: hostname)
	EventSourceType string                 // SourceType of the send event
	EventIndex      string                 // Index to which it should send
	EventField      map[string]interface{} // Additional key value pairs that should be send with every event
}

type Splunk struct {
	Host        string
	Port        int
	SplunkToken string
	VerifyTLS   bool
	SplunkEventConfig
}

type SplunkPostData struct {
	Event      map[string]interface{} `json:"event"` // here lives the data
	Index      string                 `json:"index"`
	Source     string                 `json:"source"`
	Sourcetype string                 `json:"sourcetype"`
	Host       string                 `json:"host"`
}

func (s Splunk) Filter(input []byte) []byte {
	return input
}

func (s Splunk) Send(input tail.LineData) ([]byte, error) {
	if s.EventHost == "" {
		s.EventHost = utils.GetHostname()
	}

	if s.Port == 0 {
		s.Port = 8088
	}

	encodedLinesMap := utils.StructToMap(input)

	eventData := utils.MergeMaps(encodedLinesMap, s.EventField)

	postData := SplunkPostData{
		Index:      s.EventIndex,
		Host:       s.EventHost,
		Source:     "log-forwarder",
		Sourcetype: s.EventSourceType,
		Event:      eventData,
	}

	postDataRaw, err := json.Marshal(postData)
	if err != nil {
		return postDataRaw, err
	}

	err = s.SendDataToSplunk(postDataRaw)
	if err != nil {
		return postDataRaw, err
	}

	return postDataRaw, nil
}

func (s *Splunk) SendDataToSplunk(data []byte) error {
	serverURL := fmt.Sprintf("https://%s:%d/services/collector", s.Host, s.Port)
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Splunk %s", s.SplunkToken))

	// Setup TLS
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !s.VerifyTLS,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   time.Second * 30,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (s Splunk) Retry(data []byte) error {
	err := s.SendDataToSplunk(data)
	if err != nil {
		return err
	}
	return nil
}
