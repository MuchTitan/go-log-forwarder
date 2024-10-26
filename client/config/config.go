package config

import (
	"io"
	"log-forwarder-client/database"
	"log-forwarder-client/filter"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log-forwarder-client/util"
	"log/slog"
	"os"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

var cfg *SystemConfig

type RawConfig struct {
	System  map[string]interface{}   `yaml:"System"`
	Inputs  []map[string]interface{} `yaml:"Inputs"`
	Parsers []map[string]interface{} `yaml:"Parsers"`
	Filters []map[string]interface{} `yaml:"Filters"`
	Outputs []map[string]interface{} `yaml:"Outputs"`
}

type SystemConfig struct {
	Logger   *slog.Logger
	DBFile   string `mapstructure:"dbfile"`
	LogLevel string `mapstructure:"logLevel"`
	LogFile  string `mapstructure:"logfile"`
}

type LogOut interface {
	io.Writer
}

func (c *SystemConfig) GetLogLevel() int {
	switch c.LogLevel {
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

func SetupLogger(w io.Writer, level int) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.Level(level),
	}
	logger := slog.New(slog.NewJSONHandler(w, opts))
	return logger
}

func GetLogger() *slog.Logger {
	return cfg.Logger
}

func LoadSystemConfig(data map[string]interface{}) (*SystemConfig, error) {
	sysConfig := &SystemConfig{}
	err := mapstructure.Decode(data, sysConfig)
	if err != nil {
		return nil, err
	}

	// Setup Logger
	writer := util.NewMultiWriter(os.Stderr)
	if sysConfig.LogFile != "" {
		logFile, err := os.OpenFile(sysConfig.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		writer.AddWriter(logFile)

	}
	var logOut LogOut = writer
	sysConfig.Logger = SetupLogger(logOut, sysConfig.GetLogLevel())

	return sysConfig, nil
}

func LoadConfig(filepath string) *SystemConfig {
	data, err := os.ReadFile(filepath)
	if err != nil {
		panic("coundnt read config file")
	}

	// Replace environment variables in the raw string
	dataWithEnv := os.Expand(string(data), os.Getenv)

	var rawCfg RawConfig
	err = yaml.Unmarshal([]byte(dataWithEnv), &rawCfg)
	if err != nil {
		panic(err)
	}

	cfg, err = LoadSystemConfig(rawCfg.System)
	if err != nil {
		panic(err)
	}

	err = database.OpenDB(cfg.DBFile)
	if err != nil {
		cfg.Logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}

	DecodeInputs(rawCfg.Inputs)
	DecodeParser(rawCfg.Parsers)
	DecodeFilter(rawCfg.Filters)
	DecodeOutputs(rawCfg.Outputs)
	return cfg
}

func DecodeOutputs(outputsList []map[string]interface{}) {
	for _, outputCfg := range outputsList {
		switch outputCfg["Name"] {
		case "splunk":
			splunk, err := output.ParseSplunk(outputCfg, cfg.Logger)
			if err != nil {
				panic(err)
			}
			cfg.Logger.Debug("Loaded this output splunk config", "Config", splunk)
			output.AvailableOutputs = append(output.AvailableOutputs, splunk)

		case "stdout":
			stdout, err := output.ParseStdout(outputCfg, cfg.Logger)
			if err != nil {
				panic(err)
			}
			cfg.Logger.Debug("Loaded this output stdout config", "Config", stdout)
			output.AvailableOutputs = append(output.AvailableOutputs, stdout)

		default:
			cfg.Logger.Warn("Not a implemented Output", "Name", outputCfg["Name"])
		}
	}
}

func DecodeInputs(inputsList []map[string]interface{}) {
	for _, inputCfg := range inputsList {
		switch inputCfg["Name"] {
		case "tail":
			tail, err := input.ParseTail(inputCfg, cfg.Logger)
			if err != nil {
				panic(err)
			}
			cfg.Logger.Debug("Loaded this input tail config", "Config", tail)
			input.AvailableInputs = append(input.AvailableInputs, tail)

		case "http":
			http, err := input.ParseHttp(inputCfg, cfg.Logger)
			if err != nil {
				panic(err)
			}
			cfg.Logger.Debug("Loaded this input http config", "Config", http)
			input.AvailableInputs = append(input.AvailableInputs, http)

		default:
			cfg.Logger.Warn("Not a implemented Input", "Name", inputCfg["Name"])
		}
	}
}

func DecodeParser(parserList []map[string]interface{}) {
	for _, parserCfg := range parserList {
		switch parserCfg["Name"] {
		case "json":
			jsonObject, err := parser.ParseJson(parserCfg, cfg.Logger)
			if err != nil {
				panic(err)
			}
			cfg.Logger.Debug("Loaded this parser json config", "Config", jsonObject)
			parser.AvailableParser = append(parser.AvailableParser, jsonObject)
		case "regex":
			regex, err := parser.ParseRegex(parserCfg, cfg.Logger)
			if err != nil {
				panic(err)
			}
			cfg.Logger.Debug("Loaded this parser regex config", "Config", regex)
			parser.AvailableParser = append(parser.AvailableParser, regex)
		default:
			cfg.Logger.Warn("Not a implemented Parser", "Name", parserCfg["Name"])
		}
	}
}

func DecodeFilter(filterList []map[string]interface{}) {
	for _, filterCfg := range filterList {
		switch filterCfg["Name"] {
		case "grep":
			grep, err := filter.ParseGrep(filterCfg, cfg.Logger)
			if err != nil {
				panic(err)
			}
			cfg.Logger.Debug("Loaded this filter grep config", "Config", grep)
			filter.AvailableFilters = append(filter.AvailableFilters, grep)
		default:
			cfg.Logger.Warn("Not a implemented Filter", "Name", filterCfg["Name"])
		}
	}
}
