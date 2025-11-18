package config

import (
	"fmt"
	"strconv"
	"strings"
)

// CLIConfig holds plugin configurations from CLI flags
type CLIConfig struct {
	Inputs  []map[string]any
	Parsers []map[string]any
	Filters []map[string]any
	Outputs []map[string]any
}

// ParsePluginConfig parses a CLI flag value in "key=value,key2=value2" format
// into a map suitable for plugin initialization
func ParsePluginConfig(flagValue string) (map[string]any, error) {
	if flagValue == "" {
		return nil, fmt.Errorf("empty plugin configuration")
	}

	config := make(map[string]any)
	pairs := strings.Split(flagValue, ",")

	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair: %s", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("empty key in pair: %s", pair)
		}

		// Try to convert to appropriate type
		config[key] = parseValue(value)
	}

	return config, nil
}

// parseValue attempts to parse a string value into the appropriate type
func parseValue(value string) any {
	// Try boolean
	if value == "true" || value == "false" {
		boolVal, _ := strconv.ParseBool(value)
		return boolVal
	}

	// Try integer
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}

	// Try float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}

	// Default to string
	return value
}

// ValidatePluginConfig performs basic validation on plugin configuration
func ValidatePluginConfig(pluginType string, config map[string]any) error {
	// Check that Type field exists
	if _, ok := config["Type"]; !ok {
		return fmt.Errorf("missing required field 'Type' for %s plugin", pluginType)
	}

	typeStr, ok := config["Type"].(string)
	if !ok {
		return fmt.Errorf("Type field must be a string for %s plugin", pluginType)
	}

	if typeStr == "" {
		return fmt.Errorf("Type field cannot be empty for %s plugin", pluginType)
	}

	// Additional validation could be added here based on plugin type
	return nil
}

// MergeConfigs combines config file and CLI configurations
// CLI configurations are appended to config file configurations
func MergeConfigs(fileConfig Config, cliConfig CLIConfig) Config {
	merged := fileConfig

	// Append CLI inputs
	if len(cliConfig.Inputs) > 0 {
		merged.Inputs = append(merged.Inputs, cliConfig.Inputs...)
	}

	// Append CLI parsers
	if len(cliConfig.Parsers) > 0 {
		merged.Parsers = append(merged.Parsers, cliConfig.Parsers...)
	}

	// Append CLI filters
	if len(cliConfig.Filters) > 0 {
		merged.Filters = append(merged.Filters, cliConfig.Filters...)
	}

	// Append CLI outputs
	if len(cliConfig.Outputs) > 0 {
		merged.Outputs = append(merged.Outputs, cliConfig.Outputs...)
	}

	return merged
}
