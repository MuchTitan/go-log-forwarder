package internal

import (
	"testing"
)

// TestPluginTypeString tests PluginType.String() for all types
func TestPluginTypeString(t *testing.T) {
	tests := []struct {
		pluginType PluginType
		expected   string
	}{
		{INPUTTAIL, "input_tail"},
		{INPUTHTTP, "input_http"},
		{INPUTTCP, "input_tcp"},
		{PARSERJSON, "parser_json"},
		{PARSERREGEX, "parser_regex"},
		{FILTERGREP, "filter_grep"},
		{FILTERMODIFY, "filter_modify"},
		{OUTPUTSPLUNK, "output_splunk"},
		{OUTPUTGELF, "output_gelf"},
		{OUTPUTCOUNTER, "output_counter"},
		{OUTPUTSTDOUT, "output_stdout"},
		{PluginType(999), "Unhandled Plugin 999"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.pluginType.String()
			if result != tt.expected {
				t.Errorf("PluginType.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestToPluginType tests string to PluginType conversion
func TestToPluginType(t *testing.T) {
	// Currently ToPluginType always returns 999999999 for any input
	result := ToPluginType("anything")
	if result != 999999999 {
		t.Errorf("ToPluginType() = %v, want 999999999", result)
	}

	result = ToPluginType("input_tail")
	if result != 999999999 {
		t.Errorf("ToPluginType() = %v, want 999999999", result)
	}
}
