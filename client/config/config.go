package config

import (
	"log"
	"os"
)

var DefaultValues = map[string]string{
	"ConfigPath": "./conf",
	"ListenPort": "8000",
	"ServerUrl":  "127.0.0.1",
}

func Env(env string) string {
	if value, isThere := os.LookupEnv(env); isThere {
		return value
	}

	if defaultValue, ok := DefaultValues[env]; ok {
		log.Printf("Couldn't find value for %s. Using default: %s", env, defaultValue)
		return defaultValue
	}

	log.Printf("No default value set for %s", env)
	return "" // Return empty string if no default value is set
}
