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

func Get() *Config {
	if cfg == nil {
		LoadConfig()
	}
	return cfg
}
