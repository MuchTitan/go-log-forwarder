package output

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log-forwarder-client/util"
	"log/slog"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
)

type Splunk struct {
	httpClient        *http.Client
	EventFields       map[string]interface{} `mapstructure:"EventFields"`
	SplunkToken       string                 `mapstructure:"Token"`
	Host              string                 `mapstructure:"Host"`
	EventHost         string                 `mapstructure:"EventHost"`
	EventSourceType   string                 `mapstructure:"EventSourceType"`
	EventIndex        string                 `mapstructure:"EventIndex"`
	OutputMatch       string                 `mapstructure:"Match"`
	CompressingMethod string                 `mapstructure:"Compress"`
	Port              int                    `mapstructure:"Port"`
	VerifyTLS         bool                   `mapstructure:"VerifyTLS"`
	SendRaw           bool                   `mapstructure:"SendRaw"`
}

type SplunkPostData struct {
	Event      interface{} `json:"event"`
	Index      string      `json:"index"`
	Source     string      `json:"source"`
	Sourcetype string      `json:"sourcetype"`
	Host       string      `json:"host"`
	Time       int64       `json:"time"`
}

func ParseSplunk(input interface{}) (Splunk, error) {
	splunk := Splunk{}
	err := mapstructure.Decode(input, &splunk)
	if err != nil {
		return splunk, fmt.Errorf("Coundnt parse Splunk config. Error: %w", err)
	}

	// Check for misconfiguration
	if splunk.EventIndex == "" {
		return splunk, fmt.Errorf("Cant output to splunk without a Index")
	}

	if splunk.SplunkToken == "" {
		return splunk, fmt.Errorf("Cant output to splunk without a Token")
	}

	// Setup Defaults
	splunk.EventHost = cmp.Or(splunk.EventHost, util.GetHostname())
	splunk.EventSourceType = cmp.Or(splunk.EventSourceType, "JSON")
	splunk.CompressingMethod = cmp.Or(splunk.CompressingMethod, "gzip")

	// Setup TLS
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !splunk.VerifyTLS,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   time.Second * 30,
	}

	splunk.httpClient = client
	return splunk, nil
}

func (s Splunk) GetMatch() string {
	if s.OutputMatch == "" {
		return "*"
	}
	return s.OutputMatch
}

func (s Splunk) Write(data util.Event) error {
	var eventData interface{}
	if !s.SendRaw {
		if data.ParsedData == nil {
			slog.Warn("Trying to send to splunk Parsed Data without a defiend Parser. Sending raw data.")
			return nil
		}
		eventData = util.MergeMaps(data.ParsedData, s.EventFields)
	} else {
		eventData = string(data.RawData)
	}

	postData := SplunkPostData{
		Time:       data.Time,
		Index:      s.EventIndex,
		Host:       s.EventHost,
		Source:     "log-forwarder",
		Sourcetype: s.EventSourceType,
		Event:      eventData,
	}

	postDataRaw, err := json.Marshal(postData)
	if err != nil {
		slog.Debug("Coundnt parse data in to JSON format", "error", err)
		return err
	}

	if s.CompressingMethod == "gzip" {
		var gzippedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&gzippedData)
		_, err := gzipWriter.Write(postDataRaw)
		if err != nil {
			slog.Debug("Failed to gzip data", "error", err)
			return err
		}
		// Ensure all data is written and the writer is closed
		gzipWriter.Close()

		// Send the gzipped data
		err = s.SendDataToSplunk(gzippedData.Bytes())
		if err != nil {
			slog.Debug("Coundnt send gzipped data to splunk", "error", err)
			return err
		}
	} else {
		err = s.SendDataToSplunk(postDataRaw)
		if err != nil {
			slog.Debug("Coundnt send to splunk", "error", err)
			return err
		}
	}

	return nil
}

func (s *Splunk) SendDataToSplunk(data []byte) error {
	var serverURL string
	if s.SendRaw {
		serverURL = fmt.Sprintf("https://%s:%d/services/collector/raw", s.Host, s.Port)
	} else {
		serverURL = fmt.Sprintf("https://%s:%d/services/collector", s.Host, s.Port)
	}
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	if s.CompressingMethod == "gzip" {
		req.Header.Add("Content-Encoding", "gzip")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Splunk %s", s.SplunkToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
