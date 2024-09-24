package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type ApplicationConfig struct {
	ServerUrl  string `json:"serverUrl"`
	ServerPort int    `json:"serverPort"`
	DbFile     string `json:"dbFile"`
	LogLevel   string `json:"logLevel"`
}

var (
	cfg  *ApplicationConfig
	once sync.Once
)

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

func Get() *ApplicationConfig {
	if cfg == nil {
		LoadConfig()
	}
	return cfg
}
