# CLI Configuration Tests

This directory contains comprehensive tests for the CLI configuration feature.

## Test Files

### `cli_parser_test.go`
Unit tests for core CLI parsing functions:
- `TestParsePluginConfig` - Tests parsing of key=value format
- `TestParseValue` - Tests automatic type conversion (bool, int, float, string)
- `TestValidatePluginConfig` - Tests validation logic
- `TestMergeConfigs` - Tests config file + CLI merging

### `integration_test.go`
Integration tests for all plugin types:
- **Input Plugins**: tail, tcp, http
- **Parser Plugins**: json, regex
- **Filter Plugins**: grep
- **Output Plugins**: stdout, splunk, counter, gelf
- **Complex Configurations**: Multiple plugins of each type
- **Type Conversions**: Boolean, integer, string values
- **Hybrid Mode**: Merging file config with CLI config

## Running Tests

### Run All Tests
```bash
# From project root
go test -v ./internal/config/

# Or use the test script
chmod +x run_tests.sh
./run_tests.sh
```

### Run Specific Test Files
```bash
# Unit tests only
go test -v ./internal/config/cli_parser_test.go ./internal/config/cli_parser.go ./internal/config/config.go

# Integration tests only
go test -v ./internal/config/integration_test.go ./internal/config/cli_parser.go ./internal/config/config.go
```

### Run Specific Tests
```bash
# Test all input plugins
go test -v ./internal/config/ -run TestInputPlugins

# Test all output plugins
go test -v ./internal/config/ -run TestOutputPlugins

# Test type conversions
go test -v ./internal/config/ -run TestTypeConversions
```

### Generate Coverage Report
```bash
go test -coverprofile=coverage.out ./internal/config/
go tool cover -html=coverage.out -o coverage.html
```

## Test Coverage

The tests cover:

### ✅ All Input Types
- **Tail**: File tailing with glob patterns
- **TCP**: Network input on specified port
- **HTTP**: HTTP webhook receiver

### ✅ All Parser Types
- **JSON**: JSON log parsing
- **Regex**: Pattern-based parsing

### ✅ All Filter Types
- **Grep**: Pattern matching filter

### ✅ All Output Types
- **Stdout**: Console output
- **Splunk**: Splunk HEC output
- **Counter**: Event counter
- **GELF**: Graylog Extended Log Format

### ✅ Configuration Scenarios
- CLI-only configuration
- Config file-only configuration
- Hybrid mode (file + CLI)
- Multiple plugins of same type
- Type conversions (bool, int, string)

## Example Test Cases

### Input Plugin Test
```go
cliInput := "Type=tail,Glob=/var/log/*.log,Tag=test-tail,EnableDB=false"
config, err := ParsePluginConfig(cliInput)
// Validates: Type=tail, Glob is string, EnableDB is bool
```

### Output Plugin Test
```go
cliInput := "Type=splunk,Token=abc123,EventIndex=main,Match=*"
config, err := ParsePluginConfig(cliInput)
// Validates: Type=splunk, Token is string, Match is string
```

### Hybrid Configuration Test
```go
fileConfig := Config{Inputs: [...]}
cliConfig := CLIConfig{Inputs: [...]}
merged := MergeConfigs(fileConfig, cliConfig)
// Verifies: File config plugins come first, CLI appended
```

## Continuous Integration

These tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Run Tests
  run: |
    go test -v ./internal/config/
    go test -coverprofile=coverage.out ./internal/config/
```

## Adding New Tests

When adding new plugin types:

1. Add test case to appropriate `Test*Plugins` function in `integration_test.go`
2. Include example CLI configuration string
3. Verify type conversion works correctly
4. Update this README with the new plugin type

## Troubleshooting

### Windows Go + WSL Filesystem Issue
If you get "Incorrect function" errors with Windows Go, use WSL:
```bash
wsl
cd /home/robin/dev/go-log-forwarder
./run_tests.sh
```

### Test Failures
- Check that all plugin types in test match actual plugin implementations
- Verify Type field is always included in CLI config strings
- Ensure type conversions match expected values (bool, int, float, string)
