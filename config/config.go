package config

import (
	"fmt"
	"io"
	"github.com/MuchTitan/go-log-forwarder/engine"
	"github.com/MuchTitan/go-log-forwarder/filter"
	"github.com/MuchTitan/go-log-forwarder/input"
	"github.com/MuchTitan/go-log-forwarder/output"
	"github.com/MuchTitan/go-log-forwarder/parser"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration
type Config struct {
	System  SystemConfig             `yaml:"System"`
	Inputs  []map[string]interface{} `yaml:"Inputs"`
	Parsers []map[string]interface{} `yaml:"Parsers"`
	Filters []map[string]interface{} `yaml:"Filters"`
	Outputs []map[string]interface{} `yaml:"Outputs"`
}

// SystemConfig holds system-wide configuration
type SystemConfig struct {
	LogLevel string `yaml:"logLevel"`
	LogFile  string `yaml:"logFile"`
}

func (c *SystemConfig) GetLogLevel() int {
	switch strings.ToUpper(c.LogLevel) {
	case "DEBUG":
		return -4
	case "WARNING":
		return 4
	case "ERROR":
		return 8
	default:
		// Default LogLevel Info
		return 0
	}
}

// Engine is extended to include configuration
type PluginEngine struct {
	*engine.Engine
	config Config
}

// NewPluginEngine creates a new engine with configuration
func NewPluginEngine(configPath string) (*PluginEngine, error) {
	engine := &PluginEngine{
		Engine: engine.NewEngine(),
	}

	if err := engine.loadConfig(configPath); err != nil {
		return nil, err
	}

	if err := engine.initializePlugins(); err != nil {
		return nil, err
	}

	return engine, nil
}

func (e *PluginEngine) loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Replace environment variables
	expandedData := os.Expand(string(data), os.Getenv)

	if err := yaml.Unmarshal([]byte(expandedData), &e.config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Setup logging
	if err := e.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	return nil
}

func (e *PluginEngine) setupLogging() error {
	writers := []io.Writer{os.Stderr}

	if e.config.System.LogFile != "" {
		file, err := os.OpenFile(e.config.System.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		writers = append(writers, file)
	}

	// Create multi-writer
	writer := io.MultiWriter(writers...)

	// Set log level based on config
	var level int
	if e.config.System.LogLevel != "" {
		level = e.config.System.GetLogLevel()
	}

	opts := &slog.HandlerOptions{
		Level: slog.Level(level),
	}
	logger := slog.New(slog.NewJSONHandler(writer, opts))
	slog.SetDefault(logger)
	return nil
}

func (e *PluginEngine) initializePlugins() error {
	// Initialize inputs
	for _, inputConfig := range e.config.Inputs {
		if err := e.initializeInput(inputConfig); err != nil {
			return fmt.Errorf("failed to initialize input: %w", err)
		}
	}

	// Initialize parsers
	for _, parserConfig := range e.config.Parsers {
		if err := e.initializeParser(parserConfig); err != nil {
			return fmt.Errorf("failed to initialize parser: %w", err)
		}
	}

	// Initialize filters
	for _, filterConfig := range e.config.Filters {
		if err := e.initializeFilter(filterConfig); err != nil {
			return fmt.Errorf("failed to initialize filter: %w", err)
		}
	}

	// Initialize outputs
	for _, outputConfig := range e.config.Outputs {
		if err := e.initializeOutput(outputConfig); err != nil {
			return fmt.Errorf("failed to initialize output: %w", err)
		}
	}

	return nil
}

func (e *PluginEngine) initializeInput(config map[string]interface{}) error {
	var inputObject input.Plugin

	switch config["Type"] {
	case "tail":
		inputObject = &input.Tail{}
	case "tcp":
		inputObject = &input.TCP{}
	case "http":
		inputObject = &input.InHTTP{}
	default:
		return fmt.Errorf("unknown input type: %s", config["type"])
	}

	if err := inputObject.Init(config); err != nil {
		return err
	}

	e.RegisterInput(inputObject)
	return nil
}

func (e *PluginEngine) initializeParser(config map[string]interface{}) error {
	var parserObject parser.Plugin

	switch config["Type"] {
	case "json":
		parserObject = &parser.Json{}
	case "regex":
		parserObject = &parser.Regex{}
	default:
		return fmt.Errorf("unknown filter type: %s", config["type"])
	}

	if err := parserObject.Init(config); err != nil {
		return err
	}

	e.RegisterParser(parserObject)
	return nil
}

func (e *PluginEngine) initializeFilter(config map[string]interface{}) error {
	var filterObject filter.Plugin

	switch config["Type"] {
	case "grep":
		filterObject = &filter.Grep{}
	default:
		return fmt.Errorf("unknown filter type: %s", config["type"])
	}

	if err := filterObject.Init(config); err != nil {
		return err
	}

	e.RegisterFilter(filterObject)
	return nil
}

func (e *PluginEngine) initializeOutput(config map[string]interface{}) error {
	var outputObject output.Plugin

	switch config["Type"] {
	case "stdout":
		outputObject = &output.Stdout{}
	case "splunk":
		outputObject = &output.Splunk{}
	case "counter":
		outputObject = &output.Counter{}
	case "gelf":
		outputObject = &output.GELF{}
	default:
		return fmt.Errorf("unknown output type: %s", config["type"])
	}

	if err := outputObject.Init(config); err != nil {
		return err
	}

	e.RegisterOutput(outputObject)
	return nil
}
