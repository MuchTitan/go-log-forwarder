package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log-forwarder-client/utils"
	"log/slog"
	"os"
	"sync"
)

type ApplicationConfig struct {
	DBFile   string `json:"dbFile"`
	LogLevel string `json:"logLevel"`
	Logger   *slog.Logger
}

var (
	cfg  *ApplicationConfig
	once sync.Once
)

type LogOut interface {
	io.Writer
}

func setupLogger(LogLevel int) *slog.Logger {
	// Open log file
	logFile, err := os.OpenFile("application.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	var logOut LogOut = utils.NewMultiWriter(os.Stdout, logFile)
	opts := &slog.HandlerOptions{
		Level: slog.Level(LogLevel),
	}
	logger := slog.New(slog.NewJSONHandler(logOut, opts))
	return logger
}

func LoadConfig() *ApplicationConfig {
	once.Do(func() {
		file, err := os.Open("config.json")
		if err != nil {
			log.Fatalf("Failed to open config file: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		cfg = &ApplicationConfig{}
		err = decoder.Decode(cfg)
		if err != nil {
			log.Fatalf("Failed to decode config file: %v", err)
		}
		cfg.Logger = setupLogger(cfg.GetLogLevel())
	})
	return cfg
}

func (c *ApplicationConfig) GetLogLevel() int {
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

func GetApplicationConfig() *ApplicationConfig {
	if cfg == nil {
		LoadConfig()
	}
	return cfg
}

func GetLogger() *slog.Logger {
	if cfg == nil {
		LoadConfig()
	}
	return cfg.Logger
}
