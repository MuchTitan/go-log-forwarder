# Go Log Forwarder Configuration

## Overview

The Go Log Forwarder is a flexible and extensible log processing system that can collect, parse, filter, and forward log data from various sources to multiple destinations. This document provides an overview of the system configuration and its components.

## System Configuration

The main configuration file is written in YAML format and consists of several sections:

```yaml
System:
  LogLevel: "info"
  LogFile: "/var/log/go-log-forwarder.log"
  MaxRetries: 3
  RetryBaseDelay: "1s"
  RetryMaxDelay: "30s"
```

### System Parameters

| Parameter        | Type     | Required | Default | Description |
|-----------------|---------|----------|---------|-------------|
| **LogLevel**    | string  | No       | `info`  | The logging level (trace, debug, info, warning, error) |
| **LogFile**     | string  | No       | -       | Path to the log file. If not specified, logs are only written to stderr |
| **MaxRetries**  | int     | No       | 3       | Maximum number of retry attempts for failed operations |
| **RetryBaseDelay** | duration | No | 1s | Base delay between retry attempts |
| **RetryMaxDelay** | duration | No | 30s | Maximum delay between retry attempts |

## Components

The system is composed of four main types of components:

### Inputs
Inputs are responsible for collecting log data from various sources. Available input types:
- [HTTP Input](inputs/http.md)
- [Tail Input](inputs/tail.md)
- [TCP Input](inputs/tcp.md)

### Parsers
Parsers transform raw log data into structured formats. Available parser types:
- [JSON Parser](parsers/json.md)
- [Regex Parser](parsers/regex.md)

### Filters
Filters process and modify log data based on specific criteria. Available filter types:
- [Grep Filter](filters/grep.md)
- [Modify Filter](filters/modify.md)

### Outputs
Outputs forward processed log data to various destinations. Available output types:
- [Stdout Output](outputs/stdout.md)
- [Splunk Output](outputs/splunk.md)
- [Counter Output](outputs/counter.md)
- [GELF Output](outputs/gelf.md)

## Configuration Examples

### Basic Configuration
```yaml
System:
  LogLevel: "info"

Inputs:
  - Type: tail
    Name: "access_logs"
    Path: "/var/log/nginx/access.log"
    Tag: "nginx.access"
    Match: "nginx_parser"

Parsers:
  - Type: json
    Name: "json_parser"
    Tag: "nginx_parser"

Outputs:
  - Type: stdout
    Name: "console_output"
    Match: "nginx_parser"
```

### Advanced Configuration
```yaml
System:
  LogLevel: "debug"
  LogFile: "/var/log/go-log-forwarder.log"
  MaxRetries: 5
  RetryBaseDelay: "2s"
  RetryMaxDelay: "1m"

Inputs:
  - Type: http
    Name: "http_input"
    Port: 8080
    Tag: "http.app"
    Match: "json_parser"
  - Type: tcp
    Name: "tcp_input"
    Port: 514
    Tag: "syslog"
    Match: "syslog_parser"
  - Type: tail
    Name: "nginx_logs"
    Path: "/var/log/nginx/access.log"
    Tag: "nginx.access"
    Match: "nginx_parser"

Parsers:
  - Type: regex
    Name: "nginx_parser"
    Tag: "nginx_parser"
    Pattern: '^(?P<ip>\S+) - (?P<user>\S+) \[(?P<timestamp>.*?)\] "(?P<method>\S+) (?P<path>\S+) (?P<protocol>\S+)" (?P<status>\d+) (?P<size>\d+)'
  - Type: json
    Name: "json_parser"
    Tag: "json_parser"
  - Type: regex
    Name: "syslog_parser"
    Tag: "syslog_parser"
    Pattern: '^(?P<timestamp>\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(?P<hostname>\S+)\s+(?P<program>\S+)\[(?P<pid>\d+)\]:\s+(?P<message>.*)'

Filters:
  - Type: grep
    Name: "error_filter"
    Pattern: "error"
    Match: "nginx_parser"

Outputs:
  - Type: splunk
    Name: "splunk_output"
    Host: "splunk.example.com"
    Port: 8088
    Token: "${SPLUNK_TOKEN}"
    Match: "nginx_parser"
  - Type: gelf
    Name: "graylog_output"
    Host: "graylog.example.com"
    Port: 12201
    Match: "json_parser"
  - Type: stdout
    Name: "console_output"
    Match: "syslog_parser"
```

## Environment Variables

The configuration file supports environment variable substitution using the `${VARIABLE_NAME}` syntax. This is useful for sensitive information like API keys and tokens.

Example:
```yaml
Outputs:
  - Type: splunk
    Token: "${SPLUNK_TOKEN}"
```

## Best Practices

1. **Log Rotation**: Configure appropriate log rotation for the system log file to prevent disk space issues.
2. **Error Handling**: Set appropriate retry parameters based on your network conditions and requirements.
3. **Security**: Use environment variables for sensitive information like API keys and tokens.