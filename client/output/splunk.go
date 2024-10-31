package output

import (
	"bytes"
	"cmp"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log-forwarder-client/database"
	"log-forwarder-client/util"
	"log/slog"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
)

type Splunk struct {
	db              *sql.DB
	logger          *slog.Logger
	httpClient      *http.Client
	EventFields     map[string]interface{} `mapstructure:"EventFields"`
	SplunkToken     string                 `mapstructure:"Token"`
	Host            string                 `mapstructure:"Host"`
	EventHost       string                 `mapstructure:"EventHost"`
	EventSourceType string                 `mapstructure:"EventSourceType"`
	EventIndex      string                 `mapstructure:"EventIndex"`
	OutputMatch     string                 `mapstructure:"Match"`
	Port            int                    `mapstructure:"Port"`
	VerifyTLS       bool                   `mapstructure:"VerifyTLS"`
	SendRaw         bool                   `mapstructure:"SendRaw"`
}

type SplunkPostData struct {
	Time       int64       `json:"time"`
	Event      interface{} `json:"event"` // here lives the data
	Index      string      `json:"index"`
	Source     string      `json:"source"`
	Sourcetype string      `json:"sourcetype"`
	Host       string      `json:"host"`
}

func ParseSplunk(input interface{}, logger *slog.Logger) (Splunk, error) {
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

	splunk.db = database.GetDB()

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
	splunk.logger = logger
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
			s.logger.Warn("Trying to send to splunk Parsed Data without a defiend Parser. Sending raw data.")
			return nil
			// eventData = string(data.RawData)
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
		s.logger.Debug("Coundnt parse data in to JSON format", "error", err)
		return err
	}

	err = s.SendDataToSplunk(postDataRaw)
	if err != nil {
		s.logger.Warn("Coundnt send to splunk", "error", err)
		return err
	}
	return nil
}

func (s *Splunk) SendDataToSplunk(data []byte) error {
	serverURL := fmt.Sprintf("https://%s:%d/services/collector", s.Host, s.Port)
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("HTTP post failed: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
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
