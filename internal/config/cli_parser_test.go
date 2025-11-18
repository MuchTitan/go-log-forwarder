package config

import (
	"testing"
)

func TestParsePluginConfig(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantType  string
		wantCount int
	}{
		{
			name:      "valid input config",
			input:     "Type=tail,Glob=/var/log/*.log,Tag=app",
			wantErr:   false,
			wantType:  "tail",
			wantCount: 3,
		},
		{
			name:      "valid with boolean",
			input:     "Type=tail,EnableDB=true,Port=5140",
			wantErr:   false,
			wantType:  "tail",
			wantCount: 3,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format no equals",
			input:   "Type=tail,invalid",
			wantErr: true,
		},
		{
			name:    "empty key",
			input:   "Type=tail,=value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePluginConfig(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePluginConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != tt.wantCount {
					t.Errorf("ParsePluginConfig() got %d fields, want %d", len(got), tt.wantCount)
				}
				if got["Type"] != tt.wantType {
					t.Errorf("ParsePluginConfig() Type = %v, want %v", got["Type"], tt.wantType)
				}
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  interface{}
	}{
		{"boolean true", "true", true},
		{"boolean false", "false", false},
		{"integer", "5140", 5140},
		{"float", "3.14", 3.14},
		{"string", "hello", "hello"},
		{"path", "/var/log/*.log", "/var/log/*.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseValue(tt.input)
			if got != tt.want {
				t.Errorf("parseValue() = %v (type %T), want %v (type %T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestValidatePluginConfig(t *testing.T) {
	tests := []struct {
		name       string
		pluginType string
		config     map[string]any
		wantErr    bool
	}{
		{
			name:       "valid config",
			pluginType: "input",
			config:     map[string]any{"Type": "tail", "Glob": "/var/log/*.log"},
			wantErr:    false,
		},
		{
			name:       "missing Type field",
			pluginType: "input",
			config:     map[string]any{"Glob": "/var/log/*.log"},
			wantErr:    true,
		},
		{
			name:       "empty Type value",
			pluginType: "input",
			config:     map[string]any{"Type": ""},
			wantErr:    true,
		},
		{
			name:       "Type not a string",
			pluginType: "input",
			config:     map[string]any{"Type": 123},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePluginConfig(tt.pluginType, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePluginConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMergeConfigs(t *testing.T) {
	fileConfig := Config{
		Inputs: []map[string]any{
			{"Type": "tail", "Glob": "/var/log/app.log"},
		},
		Outputs: []map[string]any{
			{"Type": "splunk"},
		},
	}

	cliConfig := CLIConfig{
		Inputs: []map[string]any{
			{"Type": "tcp", "Port": 5140},
		},
		Outputs: []map[string]any{
			{"Type": "stdout"},
		},
	}

	merged := MergeConfigs(fileConfig, cliConfig)

	if len(merged.Inputs) != 2 {
		t.Errorf("MergeConfigs() got %d inputs, want 2", len(merged.Inputs))
	}

	if len(merged.Outputs) != 2 {
		t.Errorf("MergeConfigs() got %d outputs, want 2", len(merged.Outputs))
	}

	// Verify file config comes first
	if merged.Inputs[0]["Type"] != "tail" {
		t.Errorf("MergeConfigs() first input should be from file config")
	}

	// Verify CLI config is appended
	if merged.Inputs[1]["Type"] != "tcp" {
		t.Errorf("MergeConfigs() second input should be from CLI config")
	}
}
