package config

import (
	"testing"
)

// TestInputPlugins tests CLI configuration for all input plugin types
func TestInputPlugins(t *testing.T) {
	tests := []struct {
		name      string
		cliInput  string
		wantType  string
		wantError bool
	}{
		{
			name:      "tail input",
			cliInput:  "Type=tail,Glob=/var/log/*.log,Tag=test-tail,EnableDB=false",
			wantType:  "tail",
			wantError: false,
		},
		{
			name:      "tcp input",
			cliInput:  "Type=tcp,Port=5140,Tag=test-tcp",
			wantType:  "tcp",
			wantError: false,
		},
		{
			name:      "http input",
			cliInput:  "Type=http,Tag=test-http",
			wantType:  "http",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParsePluginConfig(tt.cliInput)
			if (err != nil) != tt.wantError {
				t.Errorf("ParsePluginConfig() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if err := ValidatePluginConfig("input", config); err != nil {
					t.Errorf("ValidatePluginConfig() failed: %v", err)
				}

				if config["Type"] != tt.wantType {
					t.Errorf("Type = %v, want %v", config["Type"], tt.wantType)
				}
			}
		})
	}
}

// TestParserPlugins tests CLI configuration for all parser plugin types
func TestParserPlugins(t *testing.T) {
	tests := []struct {
		name      string
		cliInput  string
		wantType  string
		wantError bool
	}{
		{
			name:      "json parser",
			cliInput:  "Type=json",
			wantType:  "json",
			wantError: false,
		},
		{
			name:      "regex parser",
			cliInput:  "Type=regex,Pattern=^(?P<timestamp>\\d{4}-\\d{2}-\\d{2})",
			wantType:  "regex",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParsePluginConfig(tt.cliInput)
			if (err != nil) != tt.wantError {
				t.Errorf("ParsePluginConfig() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if err := ValidatePluginConfig("parser", config); err != nil {
					t.Errorf("ValidatePluginConfig() failed: %v", err)
				}

				if config["Type"] != tt.wantType {
					t.Errorf("Type = %v, want %v", config["Type"], tt.wantType)
				}
			}
		})
	}
}

// TestFilterPlugins tests CLI configuration for all filter plugin types
func TestFilterPlugins(t *testing.T) {
	tests := []struct {
		name      string
		cliInput  string
		wantType  string
		wantError bool
	}{
		{
			name:      "grep filter",
			cliInput:  "Type=grep,Pattern=error",
			wantType:  "grep",
			wantError: false,
		},
		{
			name:      "grep filter with regex",
			cliInput:  "Type=grep,Pattern=ERROR|WARN",
			wantType:  "grep",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParsePluginConfig(tt.cliInput)
			if (err != nil) != tt.wantError {
				t.Errorf("ParsePluginConfig() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if err := ValidatePluginConfig("filter", config); err != nil {
					t.Errorf("ValidatePluginConfig() failed: %v", err)
				}

				if config["Type"] != tt.wantType {
					t.Errorf("Type = %v, want %v", config["Type"], tt.wantType)
				}
			}
		})
	}
}

// TestOutputPlugins tests CLI configuration for all output plugin types
func TestOutputPlugins(t *testing.T) {
	tests := []struct {
		name      string
		cliInput  string
		wantType  string
		wantError bool
	}{
		{
			name:      "stdout output",
			cliInput:  "Type=stdout,Match=*",
			wantType:  "stdout",
			wantError: false,
		},
		{
			name:      "splunk output",
			cliInput:  "Type=splunk,Token=abc123,EventIndex=main,Match=*",
			wantType:  "splunk",
			wantError: false,
		},
		{
			name:      "counter output",
			cliInput:  "Type=counter,Match=*",
			wantType:  "counter",
			wantError: false,
		},
		{
			name:      "gelf output",
			cliInput:  "Type=gelf,Host=localhost,Port=12201,Match=*",
			wantType:  "gelf",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParsePluginConfig(tt.cliInput)
			if (err != nil) != tt.wantError {
				t.Errorf("ParsePluginConfig() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if err := ValidatePluginConfig("output", config); err != nil {
					t.Errorf("ValidatePluginConfig() failed: %v", err)
				}

				if config["Type"] != tt.wantType {
					t.Errorf("Type = %v, want %v", config["Type"], tt.wantType)
				}
			}
		})
	}
}

// TestComplexConfiguration tests a complete CLI configuration with multiple plugins
func TestComplexConfiguration(t *testing.T) {
	cliConfig := CLIConfig{}

	// Add multiple inputs
	inputs := []string{
		"Type=tail,Glob=/var/log/app.log,Tag=app",
		"Type=tcp,Port=5140,Tag=network",
		"Type=http,Tag=webhook",
	}

	for _, input := range inputs {
		config, err := ParsePluginConfig(input)
		if err != nil {
			t.Fatalf("Failed to parse input '%s': %v", input, err)
		}
		if err := ValidatePluginConfig("input", config); err != nil {
			t.Fatalf("Invalid input config: %v", err)
		}
		cliConfig.Inputs = append(cliConfig.Inputs, config)
	}

	// Add parsers
	parsers := []string{
		"Type=json",
		"Type=regex,Pattern=test",
	}

	for _, parser := range parsers {
		config, err := ParsePluginConfig(parser)
		if err != nil {
			t.Fatalf("Failed to parse parser '%s': %v", parser, err)
		}
		if err := ValidatePluginConfig("parser", config); err != nil {
			t.Fatalf("Invalid parser config: %v", err)
		}
		cliConfig.Parsers = append(cliConfig.Parsers, config)
	}

	// Add filters
	filters := []string{
		"Type=grep,Pattern=ERROR",
	}

	for _, filter := range filters {
		config, err := ParsePluginConfig(filter)
		if err != nil {
			t.Fatalf("Failed to parse filter '%s': %v", filter, err)
		}
		if err := ValidatePluginConfig("filter", config); err != nil {
			t.Fatalf("Invalid filter config: %v", err)
		}
		cliConfig.Filters = append(cliConfig.Filters, config)
	}

	// Add multiple outputs
	outputs := []string{
		"Type=stdout,Match=*",
		"Type=counter,Match=*",
		"Type=splunk,Token=test,EventIndex=logs,Match=app",
	}

	for _, output := range outputs {
		config, err := ParsePluginConfig(output)
		if err != nil {
			t.Fatalf("Failed to parse output '%s': %v", output, err)
		}
		if err := ValidatePluginConfig("output", config); err != nil {
			t.Fatalf("Invalid output config: %v", err)
		}
		cliConfig.Outputs = append(cliConfig.Outputs, config)
	}

	// Verify counts
	if len(cliConfig.Inputs) != 3 {
		t.Errorf("Expected 3 inputs, got %d", len(cliConfig.Inputs))
	}
	if len(cliConfig.Parsers) != 2 {
		t.Errorf("Expected 2 parsers, got %d", len(cliConfig.Parsers))
	}
	if len(cliConfig.Filters) != 1 {
		t.Errorf("Expected 1 filter, got %d", len(cliConfig.Filters))
	}
	if len(cliConfig.Outputs) != 3 {
		t.Errorf("Expected 3 outputs, got %d", len(cliConfig.Outputs))
	}
}

// TestTypeConversions verifies automatic type conversion for plugin configs
func TestTypeConversions(t *testing.T) {
	tests := []struct {
		name       string
		cliInput   string
		fieldName  string
		wantValue  interface{}
		wantType   string
	}{
		{
			name:      "boolean true",
			cliInput:  "Type=tail,EnableDB=true",
			fieldName: "EnableDB",
			wantValue: true,
			wantType:  "bool",
		},
		{
			name:      "boolean false",
			cliInput:  "Type=tail,EnableDB=false",
			fieldName: "EnableDB",
			wantValue: false,
			wantType:  "bool",
		},
		{
			name:      "integer port",
			cliInput:  "Type=tcp,Port=5140",
			fieldName: "Port",
			wantValue: 5140,
			wantType:  "int",
		},
		{
			name:      "string value",
			cliInput:  "Type=tail,Glob=/var/log/*.log",
			fieldName: "Glob",
			wantValue: "/var/log/*.log",
			wantType:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParsePluginConfig(tt.cliInput)
			if err != nil {
				t.Fatalf("ParsePluginConfig() error = %v", err)
			}

			value, ok := config[tt.fieldName]
			if !ok {
				t.Fatalf("Field %s not found in config", tt.fieldName)
			}

			if value != tt.wantValue {
				t.Errorf("Field %s = %v (type %T), want %v (type %T)",
					tt.fieldName, value, value, tt.wantValue, tt.wantValue)
			}
		})
	}
}

// TestHybridConfiguration tests merging of file config and CLI config
func TestHybridConfiguration(t *testing.T) {
	// Simulate file config
	fileConfig := Config{
		Inputs: []map[string]any{
			{"Type": "tail", "Glob": "/var/log/syslog"},
		},
		Parsers: []map[string]any{
			{"Type": "json"},
		},
		Outputs: []map[string]any{
			{"Type": "splunk", "Token": "file-token"},
		},
	}

	// Create CLI config
	cliConfig := CLIConfig{
		Inputs: []map[string]any{
			{"Type": "tcp", "Port": 5140},
		},
		Outputs: []map[string]any{
			{"Type": "stdout", "Match": "*"},
		},
	}

	// Merge configs
	merged := MergeConfigs(fileConfig, cliConfig)

	// Verify file config comes first
	if len(merged.Inputs) != 2 {
		t.Errorf("Expected 2 inputs after merge, got %d", len(merged.Inputs))
	}
	if merged.Inputs[0]["Type"] != "tail" {
		t.Errorf("First input should be from file config (tail), got %v", merged.Inputs[0]["Type"])
	}
	if merged.Inputs[1]["Type"] != "tcp" {
		t.Errorf("Second input should be from CLI config (tcp), got %v", merged.Inputs[1]["Type"])
	}

	// Verify outputs merged
	if len(merged.Outputs) != 2 {
		t.Errorf("Expected 2 outputs after merge, got %d", len(merged.Outputs))
	}

	// Verify parsers from file only (no CLI parsers)
	if len(merged.Parsers) != 1 {
		t.Errorf("Expected 1 parser after merge, got %d", len(merged.Parsers))
	}
}
