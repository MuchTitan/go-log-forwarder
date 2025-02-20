package config

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal/database"
	"github.com/MuchTitan/go-log-forwarder/internal/engine"
	"github.com/MuchTitan/go-log-forwarder/internal/filter"
	"github.com/MuchTitan/go-log-forwarder/internal/input"
	"github.com/MuchTitan/go-log-forwarder/internal/output"
	"github.com/MuchTitan/go-log-forwarder/internal/parser"
	"github.com/sirupsen/logrus"

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
	DBFile   string `yaml:"dbFile"`
}

func (c *SystemConfig) GetLogLevel() logrus.Level {
	switch strings.ToUpper(c.LogLevel) {
	case "TRACE":
		return logrus.TraceLevel
	case "DEBUG":
		return logrus.DebugLevel
	case "WARNING":
		return logrus.WarnLevel
	case "ERROR":
		return logrus.ErrorLevel
	default:
		// Default LogLevel Info
		return logrus.InfoLevel
	}
}

// Engine is extended to include configuration
type PluginEngine struct {
	*engine.Engine
	config    Config
	DbManager *database.DBManager
}

// NewPluginEngine creates a new engine with configuration
func NewPluginEngine(configPath string) (*PluginEngine, error) {
	engine := &PluginEngine{
		Engine: engine.NewEngine(),
	}

	if err := engine.loadConfig(configPath); err != nil {
		return nil, err
	}

	dbManager, err := database.NewDBManager(engine.config.System.DBFile)
	if err != nil {
		return nil, err
	}
	engine.DbManager = dbManager

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
	expandedData := os.ExpandEnv(string(data))

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

	// Set log level based on config
	logrus.SetLevel(e.config.System.GetLogLevel())

	// Create multi-writer
	writer := io.MultiWriter(writers...)
	logrus.SetOutput(writer)

	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339, // Use RFC3339 format (2006-01-02T15:04:05Z07:00)
	})

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

	switch strings.ToLower(config["Type"].(string)) {
	case "tail":
		inputObject = &input.Tail{
			DbManager: e.DbManager,
		}
	case "tcp":
		inputObject = &input.TCP{}
	case "http":
		inputObject = &input.InHTTP{}
	default:
		return fmt.Errorf("unknown input type: %s", config["Type"])
	}

	if err := inputObject.Init(config); err != nil {
		return err
	}

	e.RegisterInput(inputObject)
	return nil
}

func (e *PluginEngine) initializeParser(config map[string]interface{}) error {
	var parserObject parser.Plugin

	switch strings.ToLower(config["Type"].(string)) {
	case "json":
		parserObject = &parser.Json{}
	case "regex":
		parserObject = &parser.Regex{}
	default:
		return fmt.Errorf("unknown filter type: %s", config["Type"])
	}

	if err := parserObject.Init(config); err != nil {
		return err
	}

	e.RegisterParser(parserObject)
	return nil
}

func (e *PluginEngine) initializeFilter(config map[string]interface{}) error {
	var filterObject filter.Plugin

	switch strings.ToLower(config["Type"].(string)) {
	case "grep":
		filterObject = &filter.Grep{}
	default:
		return fmt.Errorf("unknown filter type: %s", config["Type"])
	}

	if err := filterObject.Init(config); err != nil {
		return err
	}

	e.RegisterFilter(filterObject)
	return nil
}

func (e *PluginEngine) initializeOutput(config map[string]interface{}) error {
	var outputObject output.Plugin

	switch strings.ToLower(config["Type"].(string)) {
	case "stdout":
		outputObject = &output.Stdout{}
	case "splunk":
		outputObject = &output.Splunk{}
	case "counter":
		outputObject = &output.Counter{}
	case "gelf":
		outputObject = &output.GELF{}
	default:
		return fmt.Errorf("unknown output type: %s", config["Type"])
	}

	if err := outputObject.Init(config); err != nil {
		return err
	}

	e.RegisterOutput(outputObject)
	return nil
}
