package config

import (
	"log"
	"log-forwarder-client/output"
	"os"

	"github.com/BurntSushi/toml"
)

func ParseInConfig(path string) {
	rawData, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Coudnt read config file: %v", err)
	}

	var wholeCfg map[string]map[string]interface{}
	err = toml.Unmarshal(rawData, &wholeCfg)

	for key, value := range wholeCfg {
		prefix, methodType, err := getPrefix(key)
		if err != nil {
			log.Fatalln(err)
		}

		if prefix == "OUT" {
			handleMethodTypeOut(methodType, value)
		}
	}
}

func handleMethodTypeIn(method string, value map[string]interface{}) output.Output {
	switch method {
	case "splunk":
	default:
		log.Fatalf("Unhandeld out method %s", method)
	}
	return nil
}
