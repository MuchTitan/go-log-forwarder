package output

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal"
	"github.com/MuchTitan/go-log-forwarder/internal/util"
)

type Splunk struct {
	name        string
	token       string
	match       string
	host        string
	eventHost   string
	sourceType  string
	index       string
	port        int
	compress    bool
	verifyTLS   bool
	sendRaw     bool
	httpClient  *http.Client
	eventFields map[string]interface{}
	buffer      bytes.Buffer
}

func (s *Splunk) MatchTag(inputTag string) bool {
	return util.TagMatch(inputTag, s.match)
}

type splunkEvent struct {
	Event      interface{} `json:"event"`
	Index      string      `json:"index"`
	Source     string      `json:"source"`
	Sourcetype string      `json:"sourcetype"`
	Host       string      `json:"host"`
	Time       int64       `json:"time"`
}

func (s *Splunk) Name() string {
	return s.name
}

func (s *Splunk) Init(config map[string]interface{}) error {
	// Required fields
	s.token = util.MustString(config["Token"])
	if s.token == "" {
		return errors.New("splunk token is required")
	}

	s.index = util.MustString(config["EventIndex"])
	if s.index == "" {
		return errors.New("splunk index is required")
	}

	// Optional fields with defaults
	s.name = util.MustString(config["Name"])
	if s.name == "" {
		s.name = "splunk"
	}

	s.match = util.MustString(config["Match"])
	if s.match == "" {
		s.match = "*"
	}

	s.host = util.MustString(config["Host"])
	if s.host == "" {
		s.host = "localhost"
	}

	s.eventHost = util.MustString(config["EventHost"])
	if s.eventHost == "" {
		hostname, _ := os.Hostname()
		s.eventHost = hostname
	}

	s.sourceType = util.MustString(config["EventSourcetype"])
	if s.sourceType == "" {
		s.sourceType = "JSON"
	}

	if port, exists := config["Port"]; exists {
		var ok bool
		if s.port, ok = port.(int); !ok {
			return errors.New("cant convert port to int")
		}
	} else {
		s.port = 8088
	}

	s.compress = config["Compress"] == true
	s.verifyTLS = config["VerifyTLS"] == true
	s.sendRaw = config["SendRaw"] == true

	// Setup TLS
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !s.verifyTLS,
		},
	}

	s.httpClient = &http.Client{
		Transport: tr,
		Timeout:   time.Second * 30,
	}

	s.buffer = bytes.Buffer{}
	return nil
}

func AppendMetadata(splunkevent *splunkEvent, event *internal.Event) {
	currData := splunkevent.Event.(map[string]interface{})
	currData["source"] = event.Metadata.Source
	currData["lineNum"] = event.Metadata.LineNum
	splunkevent.Event = currData
}

func (s *Splunk) newSplunkEvent(event internal.Event) splunkEvent {
	splunkevent := splunkEvent{
		Index:      s.index,
		Source:     s.eventHost,
		Sourcetype: s.sourceType,
		Host:       "Logs from GO Log",
		Time:       event.Timestamp.Unix(),
	}

	if s.sendRaw {
		splunkevent.Event = event.RawData
		return splunkevent
	}

	if len(event.ParsedData) != 0 {
		splunkevent.Event = util.MergeMaps(event.ParsedData, s.eventFields)
		AppendMetadata(&splunkevent, &event)
	}

	return splunkevent
}

func (s *Splunk) Write(events []internal.Event) error {
	// Convert all events to splunkEvents first
	splunkEvents := make([]splunkEvent, 0, len(events))
	for _, event := range events {
		splunkevent := s.newSplunkEvent(event)
		if splunkevent.Event == nil {
			continue
		}
		splunkEvents = append(splunkEvents, splunkevent)
	}

	data, err := json.Marshal(splunkEvents)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	s.buffer.Write(data)

	if s.buffer.Len() > 100 {
		if err := s.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Splunk) Flush() error {
	if s.buffer.Len() == 0 {
		return nil
	}

	url := fmt.Sprintf("https://%s:%d/services/collector", s.host, s.port)
	if s.sendRaw {
		url += "/raw"
	}

	var requestBody bytes.Buffer
	if s.compress {
		gz := gzip.NewWriter(&requestBody)
		if _, err := gz.Write(s.buffer.Bytes()); err != nil {
			return fmt.Errorf("error during gzip compress: %w", err)
		}
		if err := gz.Close(); err != nil {
			return err
		}
	} else {
		requestBody = s.buffer
	}
	tmp := s.buffer.String()
	s.buffer.Reset()

	req, _ := http.NewRequest("POST", url, &requestBody)
	req.Header.Set("Authorization", "Splunk "+s.token)
	req.Header.Set("Content-Type", "application/json")
	if s.compress {
		req.Header.Set("Content-Encoding", "gzip")
	}

	res, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		slog.Debug("splunk request", "req", map[string]interface{}{
			"url":     req.URL.String(),
			"method":  req.Method,
			"headers": req.Header,
			"body":    tmp,
		})
		return fmt.Errorf("splunk returned status: %s", res.Status)
	}

	return nil
}

func (s *Splunk) Exit() error {
	return nil
}
