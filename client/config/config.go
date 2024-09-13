package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type Config struct {
	ServerUrl  string `json:"serverUrl"`
	ServerPort int    `json:"serverPort"`
	DbFile     string `json:"dbFile"`
	LogLevel   string `json:"LogLevel"`
}

var (
	cfg  *Config
	once sync.Once
)

func LoadConfig() *Config {
	once.Do(func() {
		file, err := os.Open("config.json")
		if err != nil {
			log.Fatalf("Failed to open config file: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		cfg = &Config{}
		err = decoder.Decode(cfg)
		if err != nil {
			log.Fatalf("Failed to decode config file: %v", err)
		}
	})
	return cfg
}

func (c *Config) GetLogLevel() int {
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

func Get() *Config {
	if cfg == nil {
		LoadConfig()
	}
	return cfg
}
