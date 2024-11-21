package output

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log-forwarder-client/util"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type GELF struct {
	writer      gelf.Writer
	Host        string `mapstructure:"Host"`
	Mode        string `mapstructure:"Mode"`
	OutputMatch string `mapstructure:"Match"`
	Port        int    `mapstructure:"Port"`
}

func (g *GELF) SetupWriter() error {
	addr := fmt.Sprintf("%s:%d", g.Host, g.Port)
	var w gelf.Writer

	switch g.Mode {
	case "udp":
		udpWriter, err := gelf.NewUDPWriter(addr)
		if err != nil {
			return fmt.Errorf("failed to create UDP writer: %w", err)
		}
		w = udpWriter
	case "tcp":
		tcpWriter, err := gelf.NewTCPWriter(addr)
		if err != nil {
			return fmt.Errorf("failed to create TCP writer: %w", err)
		}
		w = tcpWriter
	default:
		return fmt.Errorf("unsupported mode: %s", g.Mode)
	}

	g.writer = w
	return nil
}

func ParseGELF(input map[string]interface{}) (GELF, error) {
	gelf := GELF{}
	err := mapstructure.Decode(input, &gelf)
	if err != nil {
		return gelf, err
	}

	gelf.Host = cmp.Or(gelf.Host, "127.0.0.1")
	gelf.Port = cmp.Or(gelf.Port, 12201)
	gelf.Mode = cmp.Or(gelf.Mode, "udp")

	if gelf.Mode != "udp" && gelf.Mode != "tcp" {
		return gelf, fmt.Errorf("Mode: '%v' is not implemented", gelf.Mode)
	}

	err = gelf.SetupWriter()
	if err != nil {
		return gelf, fmt.Errorf("failed to setup GELF writer: %w", err)
	}

	return gelf, nil
}

func (g GELF) Write(event util.Event) error {
	var jsonData []byte
	if event.ParsedData == nil {
		jsonData, _ = json.Marshal(event.ParsedData)
	} else {
		jsonData = event.RawData
	}

	// Convert the event to a GELF message
	msg := gelf.Message{
		Version:  "1.1",
		Host:     "log-forwarder",
		Short:    string(jsonData),
		TimeUnix: float64(event.Time),
		Level:    6, // Info level by default
		Extra:    make(map[string]interface{}),
	}

	return g.writer.WriteMessage(&msg)
}

func (g GELF) GetMatch() string {
	if g.OutputMatch == "" {
		return "*"
	}
	return g.OutputMatch
}
