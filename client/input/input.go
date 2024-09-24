package input

import (
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type InputConfig struct {
	Path   string
	Output string
}

func ParseConfig(path string) InputConfig {
	var cfg InputConfig
	fileData, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Coundnt read Config file '%s'\nError: %v", path, err)
		os.Exit(1)
	}
	err = toml.Unmarshal(fileData, &cfg)
	if err != nil {
		log.Fatalf("Coundnt unmarshal '%s' into config\nError: %v", path, err)
	}

	return cfg
}
