package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"github.com/MuchTitan/go-log-forwarder/global"
	"github.com/MuchTitan/go-log-forwarder/util"
	"log/slog"

	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type GELF struct {
	name   string
	match  string
	host   string
	port   int
	mode   string
	buffer []*gelf.Message
	writer gelf.Writer
}

func (g *GELF) Name() string {
	return g.name
}

func (g *GELF) Init(config map[string]interface{}) error {
	g.name = util.MustString(config["Name"])
	if g.name == "" {
		g.name = "gelf"
	}

	g.match = util.MustString(config["Match"])
	if g.match == "" {
		g.match = "*"
	}

	g.host = util.MustString(config["Host"])
	if g.host == "" {
		g.host = "127.0.0.1"
	}

	g.mode = util.MustString(config["Mode"])
	if g.mode == "" {
		g.mode = "udp"
	}
	if g.mode != "udp" && g.mode != "tcp" {
		return fmt.Errorf("mode: '%v' is not implemented", g.mode)
	}

	if portStr := config["Port"]; portStr != "" {
		var ok bool
		if g.port, ok = portStr.(int); !ok {
			return errors.New("cant convert port to int")
		}
	} else {
		g.port = 12201
	}

	g.buffer = make([]*gelf.Message, 0, 100)

	return g.setupWriter()
}

func (g *GELF) setupWriter() error {
	addr := fmt.Sprintf("%s:%d", g.host, g.port)
	var w gelf.Writer
	var err error

	switch g.mode {
	case "udp":
		w, err = gelf.NewUDPWriter(addr)
	case "tcp":
		w, err = gelf.NewTCPWriter(addr)
	default:
		return fmt.Errorf("unsupported mode: %s", g.mode)
	}

	if err != nil {
		return fmt.Errorf("failed to create %s writer: %w", g.mode, err)
	}

	g.writer = w
	return nil
}

func (g *GELF) Write(events []global.Event) error {
	for _, event := range events {
		var jsonData string
		if event.ParsedData != nil {
			tmp, _ := json.Marshal(event.ParsedData)
			jsonData = string(tmp)
		} else {
			jsonData = event.RawData
		}

		msg := gelf.Message{
			Version:  "1.1",
			Host:     "github.com/MuchTitan/go-log-forwarder",
			Short:    jsonData,
			TimeUnix: float64(event.Timestamp.Unix()),
			Level:    gelf.LOG_INFO, // Info level by default
			Extra:    make(map[string]interface{}),
		}

		g.buffer = append(g.buffer, &msg)

		if len(g.buffer) > 100 {
			if err := g.Flush(); err != nil {
				slog.Error("coudnt flush gelf output", "error", err)
			}
		}
	}
	return nil
}

func (g *GELF) Flush() error {
	for _, data := range g.buffer {
		err := g.writer.WriteMessage(data)
		if err != nil {
			return err
		}
	}
	g.buffer = g.buffer[:0]
	return nil
}

func (g *GELF) Exit() error {
	if g.writer != nil {
		if closer, ok := g.writer.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}
