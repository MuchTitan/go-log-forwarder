package config

import (
	"fmt"
	"log"
	"log-forwarder-client/output"
	"os"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

var ValidPrefix = []string{"IN", "OUT", "FILTER"}

func getPrefix(input string) (string, string, error) {
	splittedInput := strings.Split(input, "_")
	prefix := splittedInput[0]
	if !slices.Contains(ValidPrefix, prefix) {
		return "", "", fmt.Errorf("Invalid Prefix provided in %s", input)
	}
	suffix := splittedInput[1]
	return prefix, suffix, nil
}

func ParseOutConfig(path string) {
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

func handleMethodTypeOut(method string, value map[string]interface{}) output.Output {
	switch method {
	case "splunk":
	default:
		log.Fatalf("Unhandeld out method %s", method)
	}
	return nil
}
