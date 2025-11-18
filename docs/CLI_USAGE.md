# CLI Configuration Guide

## Overview

The Go Log Forwarder now supports command-line configuration in addition to the traditional YAML config file. This allows for flexible deployment scenarios and easier testing.

## Command-Line Flags

### Configuration File
- `--cfg <path>` - Path to YAML configuration file (optional)

### Plugin Configuration Flags (Repeatable)
- `--input <config>` - Configure an input plugin
- `--parser <config>` - Configure a parser plugin  
- `--filter <config>` - Configure a filter plugin
- `--output <config>` - Configure an output plugin

### Configuration Format

Plugin configurations use key=value pairs separated by commas:
```
Type=<plugin_type>,Key1=Value1,Key2=Value2,...
```

**Important:** The `Type` field is always required.

## Usage Examples

### Example 1: CLI-Only Configuration

Run with only command-line flags (no config file):

```bash
./bin/main.out \
  --input Type=tail,Glob=/var/log/app.log,Tag=myapp \
  --parser Type=json \
  --output Type=stdout,Match=*
```

### Example 2: Config File Only (Traditional)

Run with config file (backward compatible):

```bash
./bin/main.out --cfg ./cfg/cfg.yaml
```

### Example 3: Hybrid Mode

Combine config file with additional CLI plugins:

```bash
./bin/main.out \
  --cfg ./cfg/cfg.yaml \
  --input Type=tcp,Port=5140,Tag=network \
  --output Type=stdout,Match=network
```

In hybrid mode, CLI plugins are **appended** to config file plugins.

### Example 4: Multiple Plugins

Add multiple plugins of the same type using repeated flags:

```bash
./bin/main.out \
  --input Type=tail,Glob=/var/log/app1.log,Tag=app1 \
  --input Type=tail,Glob=/var/log/app2.log,Tag=app2 \
  --input Type=tcp,Port=5140,Tag=network \
  --parser Type=json \
  --output Type=stdout,Match=app1 \
  --output Type=splunk,Token=xxx,EventIndex=logs,Match=*
```

## Available Plugins

### Input Plugins

#### Tail
```bash
--input Type=tail,Glob=/path/to/*.log,Tag=mytag,EnableDB=false
```

#### TCP
```bash
--input Type=tcp,Port=5140,Tag=network
```

#### HTTP
```bash
--input Type=http,Tag=http-events
```

### Parser Plugins

#### JSON
```bash
--parser Type=json
```

#### Regex
```bash
--parser Type=regex,Pattern=<regex_pattern>
```

### Filter Plugins

#### Grep
```bash
--filter Type=grep,Pattern=<search_pattern>
```

### Output Plugins

#### Stdout
```bash
--output Type=stdout,Match=*
```

#### Splunk
```bash
--output Type=splunk,Token=<token>,EventIndex=<index>,Match=*
```

#### Counter
```bash
--output Type=counter,Match=*
```

#### GELF
```bash
--output Type=gelf,Host=<host>,Port=<port>,Match=*
```

## Configuration Precedence

When both config file and CLI flags are provided:
1. Config file plugins are loaded first
2. CLI plugins are appended
3. All plugins run in the engine

To use only CLI configuration, omit the `--cfg` flag.

## Value Type Conversion

The CLI parser automatically converts values to appropriate types:
- `true`/`false` → boolean
- Numeric values → integer or float
- Everything else → string

## Error Handling

The application will exit with an error if:
- No configuration is provided (neither `--cfg` nor CLI flags)
- Plugin configuration is invalid (missing `Type` field)
- Configuration parsing fails (invalid format)

Example error:
```bash
$ ./bin/main.out
FATAL No configuration provided. Use --cfg to specify config file or use CLI flags
```

## Testing

Use the provided test script to verify all modes:

```bash
chmod +x test_cli.sh
./test_cli.sh
```

This will test:
1. CLI-only mode
2. Config file-only mode
3. Hybrid mode (config file + CLI)
4. Multiple plugins
5. Error handling
