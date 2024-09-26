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
	SendRaw         bool                   // Wether or not the event should be send raw
	eventKey        string                 // Key for a single value
	eventHost       string                 // Source Host (default: hostname)
	eventSourceType string                 // SourceType of the send event
	eventIndex      string                 // Index to which it should send
	eventField      map[string]interface{} // Additional key value pairs that should be send with every event
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

func (s *Splunk) Filter(input tail.LineData) tail.LineData {
	return input
}

func (s *Splunk) Send(inChan chan tail.LineData) chan error {
	if s.eventHost == "" {
		s.eventHost = utils.GetHostname()
	}

	if s.Port == 0 {
		s.Port = 8088
	}

	errChan := make(chan error)

	go func() {
		for lineData := range inChan {
			encodedLinesMap := utils.StructToMap(lineData)

			eventData := utils.MergeMaps(encodedLinesMap, s.eventField)

			postData := SplunkPostData{
				Index:      s.eventIndex,
				Host:       s.eventHost,
				Source:     "log-forwarder",
				Sourcetype: s.eventSourceType,
				Event:      eventData,
			}

			postDataRaw, err := json.Marshal(postData)
			if err != nil {
				errChan <- err
			}
			errChan <- s.SendDataToSplunk(postDataRaw)
		}
		close(errChan)
	}()
	return errChan
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
