package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGetLogLevel tests all log level mappings
func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		wantName string
	}{
		{"trace level", "TRACE", "trace"},
		{"debug level", "DEBUG", "debug"},
		{"info level", "INFO", "info"},
		{"info level lowercase", "info", "info"},
		{"warning level", "WARNING", "warning"},
		{"error level", "ERROR", "error"},
		{"default to info", "INVALID", "info"},
		{"empty defaults to info", "", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := SystemConfig{LogLevel: tt.logLevel}
			level := cfg.GetLogLevel()
			if level.String() != tt.wantName {
				t.Errorf("GetLogLevel() = %v, want %v", level.String(), tt.wantName)
			}
		})
	}
}

// TestNewPluginEngineWithCLI_CliOnly tests engine creation with CLI-only config
func TestNewPluginEngineWithCLI_CliOnly(t *testing.T) {
	cliConfig := CLIConfig{
		Inputs: []map[string]any{
			{"Type": "tail", "Glob": "/tmp/test.log", "Tag": "test"},
		},
		Parsers: []map[string]any{
			{"Type": "json"},
		},
		Outputs: []map[string]any{
			{"Type": "stdout", "Match": "*"},
		},
	}

	// Use empty config path (CLI-only mode)
	engine, err := NewPluginEngineWithCLI("", cliConfig)
	if err != nil {
		t.Fatalf("NewPluginEngineWithCLI() failed: %v", err)
	}

	if engine == nil {
		t.Fatal("NewPluginEngineWithCLI() returned nil engine")
	}

	// Verify config was merged
	if len(engine.config.Inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(engine.config.Inputs))
	}
	if len(engine.config.Parsers) != 1 {
		t.Errorf("Expected 1 parser, got %d", len(engine.config.Parsers))
	}
	if len(engine.config.Outputs) != 1 {
		t.Errorf("Expected 1 output, got %d", len(engine.config.Outputs))
	}
}

// TestNewPluginEngineWithCLI_FileOnly tests engine creation with config file only
func TestNewPluginEngineWithCLI_FileOnly(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	configContent := `System:
  logLevel: DEBUG
  maxRetries: 5
  retryBaseDelay: 2s
  retryMaxDelay: 60s

Inputs:
  - Type: tail
    Glob: /tmp/*.log
    Tag: test

Parsers:
  - Type: json

Outputs:
  - Type: stdout
    Match: "*"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Create engine with config file only
	engine, err := NewPluginEngineWithCLI(configPath, CLIConfig{})
	if err != nil {
		t.Fatalf("NewPluginEngineWithCLI() failed: %v", err)
	}

	if engine == nil {
		t.Fatal("NewPluginEngineWithCLI() returned nil engine")
	}

	// Verify config was loaded
	if engine.config.System.LogLevel != "DEBUG" {
		t.Errorf("Expected log level DEBUG, got %s", engine.config.System.LogLevel)
	}
	if engine.config.System.MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", engine.config.System.MaxRetries)
	}
}

// TestNewPluginEngineWithCLI_Hybrid tests merging of file and CLI config
func TestNewPluginEngineWithCLI_Hybrid(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	configContent := `System:
  logLevel: INFO

Inputs:
  - Type: tail
    Glob: /var/log/app.log
    Tag: app

Outputs:
  - Type: counter
    Match: "*"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Create CLI config
	cliConfig := CLIConfig{
		Inputs: []map[string]any{
			{"Type": "tcp", "Port": 5140, "Tag": "network"},
		},
		Outputs: []map[string]any{
			{"Type": "stdout", "Match": "*"},
		},
	}

	// Create engine with both configs
	engine, err := NewPluginEngineWithCLI(configPath, cliConfig)
	if err != nil {
		t.Fatalf("NewPluginEngineWithCLI() failed: %v", err)
	}

	// Verify both configs are merged
	if len(engine.config.Inputs) != 2 {
		t.Errorf("Expected 2 inputs (1 from file + 1 from CLI), got %d", len(engine.config.Inputs))
	}
	if len(engine.config.Outputs) != 2 {
		t.Errorf("Expected 2 outputs (1 from file + 1 from CLI), got %d", len(engine.config.Outputs))
	}

	// Verify file config comes first
	if engine.config.Inputs[0]["Type"] != "tail" {
		t.Errorf("First input should be from file (tail), got %v", engine.config.Inputs[0]["Type"])
	}
	if engine.config.Inputs[1]["Type"] != "tcp" {
		t.Errorf("Second input should be from CLI (tcp), got %v", engine.config.Inputs[1]["Type"])
	}
}

// TestNewPluginEngine tests backward compatibility wrapper
func TestNewPluginEngine(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	configContent := `System:
  logLevel: ERROR

Inputs:
  - Type: tail
    Glob: /tmp/*.log
    Tag: test

Outputs:
  - Type: stdout
    Match: "*"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Use the backward compatibility wrapper
	engine, err := NewPluginEngine(configPath)
	if err != nil {
		t.Fatalf("NewPluginEngine() failed: %v", err)
	}

	if engine == nil {
		t.Fatal("NewPluginEngine() returned nil engine")
	}

	if engine.config.System.LogLevel != "ERROR" {
		t.Errorf("Expected log level ERROR, got %s", engine.config.System.LogLevel)
	}
}

// TestNewPluginEngineWithCLI_InvalidConfigPath tests error handling for bad config path
func TestNewPluginEngineWithCLI_InvalidConfigPath(t *testing.T) {
	// Try to create engine with non-existent config file (should not error in CLI mode)
	engine, err := NewPluginEngineWithCLI("/nonexistent/config.yaml", CLIConfig{
		Outputs: []map[string]any{
			{"Type": "stdout", "Match": "*"},
		},
	})

	// File doesn't exist but we have CLI config, so it should succeed
	if err != nil {
		t.Fatalf("NewPluginEngineWithCLI() should succeed with CLI config even if file doesn't exist: %v", err)
	}

	if engine == nil {
		t.Fatal("Engine should not be nil")
	}
}

// TestNewPluginEngineWithCLI_InvalidYAML tests error handling for malformed YAML
func TestNewPluginEngineWithCLI_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad_config.yaml")

	// Write invalid YAML
	invalidYAML := `System:
  logLevel: INFO
Inputs:
  - Type: tail
    Glob: /tmp/*.log
    - INVALID YAML STRUCTURE
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err := NewPluginEngineWithCLI(configPath, CLIConfig{})
	if err == nil {
		t.Error("NewPluginEngineWithCLI() should fail with invalid YAML")
	}
}

// TestNewPluginEngineWithCLI_DefaultValues tests that default values are set
func TestNewPluginEngineWithCLI_DefaultValues(t *testing.T) {
	cliConfig := CLIConfig{
		Outputs: []map[string]any{
			{"Type": "stdout", "Match": "*"},
		},
	}

	engine, err := NewPluginEngineWithCLI("", cliConfig)
	if err != nil {
		t.Fatalf("NewPluginEngineWithCLI() failed: %v", err)
	}

	// Verify default values are set
	if engine.config.System.MaxRetries != 3 {
		t.Errorf("Expected default MaxRetries 3, got %d", engine.config.System.MaxRetries)
	}
	if engine.config.System.RetryBaseDelay != 1*time.Second {
		t.Errorf("Expected default RetryBaseDelay 1s, got %v", engine.config.System.RetryBaseDelay)
	}
	if engine.config.System.RetryMaxDelay != 30*time.Second {
		t.Errorf("Expected default RetryMaxDelay 30s, got %v", engine.config.System.RetryMaxDelay)
	}
}

// TestMergeConfigs_EmptyCLI tests merging when CLI config is empty
func TestMergeConfigs_EmptyCLI(t *testing.T) {
	fileConfig := Config{
		Inputs: []map[string]any{
			{"Type": "tail"},
		},
		Outputs: []map[string]any{
			{"Type": "stdout"},
		},
	}

	merged := MergeConfigs(fileConfig, CLIConfig{})

	if len(merged.Inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(merged.Inputs))
	}
	if len(merged.Outputs) != 1 {
		t.Errorf("Expected 1 output, got %d", len(merged.Outputs))
	}
}

// TestMergeConfigs_EmptyFile tests merging when file config is empty
func TestMergeConfigs_EmptyFile(t *testing.T) {
	cliConfig := CLIConfig{
		Inputs: []map[string]any{
			{"Type": "tcp"},
		},
		Parsers: []map[string]any{
			{"Type": "json"},
		},
		Filters: []map[string]any{
			{"Type": "grep"},
		},
		Outputs: []map[string]any{
			{"Type": "stdout"},
		},
	}

	merged := MergeConfigs(Config{}, cliConfig)

	if len(merged.Inputs) != 1 {
		t.Errorf("Expected 1 input from CLI, got %d", len(merged.Inputs))
	}
	if len(merged.Parsers) != 1 {
		t.Errorf("Expected 1 parser from CLI, got %d", len(merged.Parsers))
	}
	if len(merged.Filters) != 1 {
		t.Errorf("Expected 1 filter from CLI, got %d", len(merged.Filters))
	}
	if len(merged.Outputs) != 1 {
		t.Errorf("Expected 1 output from CLI, got %d", len(merged.Outputs))
	}
}

// TestNewPluginEngineWithCLI_InvalidInputType tests error handling for unknown input type
func TestNewPluginEngineWithCLI_InvalidInputType(t *testing.T) {
	cliConfig := CLIConfig{
		Inputs: []map[string]any{
			{"Type": "invalid_input_type", "Tag": "test"},
		},
	}

	_, err := NewPluginEngineWithCLI("", cliConfig)
	if err == nil {
		t.Error("NewPluginEngineWithCLI() should fail with invalid input type")
	}
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

// TestNewPluginEngineWithCLI_InvalidParserType tests error handling for unknown parser type
func TestNewPluginEngineWithCLI_InvalidParserType(t *testing.T) {
	cliConfig := CLIConfig{
		Parsers: []map[string]any{
			{"Type": "invalid_parser_type"},
		},
	}

	_, err := NewPluginEngineWithCLI("", cliConfig)
	if err == nil {
		t.Error("NewPluginEngineWithCLI() should fail with invalid parser type")
	}
}

// TestNewPluginEngineWithCLI_InvalidFilterType tests error handling for unknown filter type
func TestNewPluginEngineWithCLI_InvalidFilterType(t *testing.T) {
	cliConfig := CLIConfig{
		Filters: []map[string]any{
			{"Type": "invalid_filter_type"},
		},
	}

	_, err := NewPluginEngineWithCLI("", cliConfig)
	if err == nil {
		t.Error("NewPluginEngineWithCLI() should fail with invalid filter type")
	}
}

// TestNewPluginEngineWithCLI_InvalidOutputType tests error handling for unknown output type
func TestNewPluginEngineWithCLI_InvalidOutputType(t *testing.T) {
	cliConfig := CLIConfig{
		Outputs: []map[string]any{
			{"Type": "invalid_output_type"},
		},
	}

	_, err := NewPluginEngineWithCLI("", cliConfig)
	if err == nil {
		t.Error("NewPluginEngineWithCLI() should fail with invalid output type")
	}
}

// TestNewPluginEngineWithCLI_EnvironmentVariables tests config with env var expansion
func TestNewPluginEngineWithCLI_EnvironmentVariables(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_TOKEN", "my-secret-token")
	defer os.Unsetenv("TEST_TOKEN")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	configContent := `System:
  logLevel: INFO

Outputs:
  - Type: splunk
    Token: ${TEST_TOKEN}
    EventIndex: test
    Match: "*"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	engine, err := NewPluginEngineWithCLI(configPath, CLIConfig{})
	if err != nil {
		t.Fatalf("NewPluginEngineWithCLI() failed: %v", err)
	}

	// Verify environment variable was expanded
	if len(engine.config.Outputs) != 1 {
		t.Fatalf("Expected 1 output, got %d", len(engine.config.Outputs))
	}

	if engine.config.Outputs[0]["Token"] != "my-secret-token" {
		t.Errorf("Expected token 'my-secret-token', got %v", engine.config.Outputs[0]["Token"])
	}
}
