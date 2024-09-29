package output

import (
	"bytes"
	"cmp"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log-forwarder-client/utils"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
)

type Splunk struct {
	Host            string                 `mapstructure:"Host"`
	Port            int                    `mapstructure:"Port"`
	SplunkToken     string                 `mapstructure:"Token"`
	VerifyTLS       bool                   `mapstructure:"VerifyTLS"`
	EventKey        string                 `mapstructure:"EventKey"`        // Key for a single value
	EventHost       string                 `mapstructure:"EventHost"`       // Source Host (default: hostname)
	EventSourceType string                 `mapstructure:"EventSourceType"` // SourceType of the send event
	EventIndex      string                 `mapstructure:"EventIndex"`      // Index to which it should send
	EventField      map[string]interface{} `mapstructure:"EventField"`      // Additional key value pairs that should be send with every event
}

type SplunkPostData struct {
	Time       int64                  `json:"time"`
	Event      map[string]interface{} `json:"event"` // here lives the data
	Index      string                 `json:"index"`
	Source     string                 `json:"source"`
	Sourcetype string                 `json:"sourcetype"`
	Host       string                 `json:"host"`
}

func (s Splunk) Write(data map[string]interface{}) {
	if s.SplunkToken == "" {
		panic("No Splunk token provided")
	}

	if s.EventIndex == "" {
		panic("No Splunk index provided")
	}

	s.EventHost = cmp.Or(s.EventHost, utils.GetHostname())
	s.Host = cmp.Or(s.Host, "localhost")
	s.Port = cmp.Or(s.Port, 8088)
	s.EventSourceType = cmp.Or(s.EventSourceType, "JSON")

	eventData := utils.MergeMaps(data, s.EventField)

	postData := SplunkPostData{
		Time:       time.Now().Unix(),
		Index:      s.EventIndex,
		Host:       s.EventHost,
		Source:     "log-forwarder",
		Sourcetype: s.EventSourceType,
		Event:      eventData,
	}

	postDataRaw, err := json.Marshal(postData)
	if err != nil {
		return
	}

	err = s.SendDataToSplunk(postDataRaw)
	if err != nil {
		return
	}
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

func SplunkParseConfig(input map[string]interface{}) Splunk {
	var splunk Splunk
	err := mapstructure.Decode(input, &splunk)
	if err != nil {
		fmt.Println("Error:", err)
		return splunk
	}

	return splunk
}
