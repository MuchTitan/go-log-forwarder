package output

import (
	"bytes"
	"cmp"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log-forwarder-client/config"
	"log-forwarder-client/parser"
	"log-forwarder-client/utils"
	"log/slog"
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
	logger          *slog.Logger
}

type SplunkPostData struct {
	Time       int64                  `json:"time"`
	Event      map[string]interface{} `json:"event"` // here lives the data
	Index      string                 `json:"index"`
	Source     string                 `json:"source"`
	Sourcetype string                 `json:"sourcetype"`
	Host       string                 `json:"host"`
}

// NewSplunk is a constructor function that creates and returns a new instance of the Splunk struct.
// It ensures that the essential fields, such as SplunkToken and EventIndex, are provided, and applies
// sensible defaults for other fields when values are not explicitly given.
//
// Parameters:
//   - host (string): The hostname or IP address of the Splunk server. Defaults to "localhost" if empty.
//   - port (int): The port of the Splunk server. Defaults to 8088 if zero is provided.
//   - token (string): The authentication token for Splunk. This is a required parameter.
//   - verifyTLS (bool): Indicates whether to verify the TLS certificate for Splunk. (Default: false)
//   - eventKey (string): The key for a single event value.
//   - eventHost (string): The source host for events. Defaults to the system hostname if empty.
//   - eventSourceType (string): The source type of the event. Defaults to "JSON" if empty.
//   - eventIndex (string): The index to which the events will be sent. This is a required parameter.
//   - eventField (map[string]interface{}): Additional key-value pairs to be included with every event.
//
// Returns:
//   - Splunk: A a newly initialized Splunk instance.
//
// Panics:
//   - If the token is empty, indicating that no Splunk token was provided.
//   - If the eventIndex is empty, indicating that no Splunk index was provided.
func NewSplunk(host string, port int, token string, verifyTLS bool, eventKey, eventHost, eventSourceType, eventIndex string, eventField map[string]interface{}, logger *slog.Logger) Splunk {
	if token == "" {
		panic("No Splunk token provided")
	}

	if eventIndex == "" {
		panic("No Splunk index provided")
	}

	return Splunk{
		Host:            cmp.Or(host, "localhost"),
		Port:            cmp.Or(port, 8088),
		SplunkToken:     token,
		VerifyTLS:       verifyTLS,
		EventKey:        eventKey,
		EventHost:       cmp.Or(eventHost, utils.GetHostname()),
		EventSourceType: cmp.Or(eventSourceType, "JSON"),
		EventIndex:      eventIndex,
		EventField:      eventField,
		logger:          logger,
	}
}

func (s Splunk) Write(data parser.ParsedData) error {
	logger := config.GetLogger()

	eventData := utils.MergeMaps(data.Data, s.EventField)
	eventData = utils.MergeMaps(data.Data, data.Metadata)

	var timeValue int64
	if data.Time != 0 {
		timeValue = data.Time
	} else {
		timeValue = time.Now().Unix()
	}

	postData := SplunkPostData{
		Time:       timeValue,
		Index:      s.EventIndex,
		Host:       s.EventHost,
		Source:     "log-forwarder",
		Sourcetype: s.EventSourceType,
		Event:      eventData,
	}

	postDataRaw, err := json.Marshal(postData)
	if err != nil {
		logger.Debug("Coundnt parse data in to JSON format", "error", err)
		return err
	}

	err = s.SendDataToSplunk(postDataRaw)
	if err != nil {
		logger.Debug("Coundnt send data", "error", err)
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

func (s Splunk) GetState() OutState {
	state := OutState{
		Name: "splunk",
		State: map[string]interface{}{
			"Host":            s.Host,
			"Port":            s.Port,
			"SplunkToken":     s.SplunkToken,
			"VerifyTLS":       s.VerifyTLS,
			"EventKey":        s.EventKey,
			"EventHost":       s.EventHost,
			"EventSourceType": s.EventSourceType,
			"EventIndex":      s.EventIndex,
			"EventField":      s.EventField,
		},
	}
	return state
}
