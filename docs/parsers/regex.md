# REGEX Parser Configuration

## Overview

This document describes the configuration parameters for the `regex` parser of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `regex` parser in the YAML configuration file:

```yaml
parsers:
  - Type: regex
    Name: "my_regex_parser"
    Match: "*_tag_*"
    Pattern: "^(?<host>[^ ]*) [^ ]* (?<user>[^ ]*) \[(?<time>[^\]]*)\] "(?<method>\S+)(?: +(?<path>[^\"]*?)(?: +\S*)?)?" (?<code>[^ ]*) (?<size>[^ ]*)(?: "(?<referer>[^\"]*)" "(?<agent>[^\"]*)")?$" # This would parse a Apache HTTP Server log line
```

### Configuration Parameters

If you want to extract the timestamp from a log line you need to specify both TimeFormat and TimeKey

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -       | Must be set to `regex` to use the regex parser. |
| **Name**         | string  | No       | `regex` | The name of the parser instance. |
| **Match**        | string  | No       | `*`     | A string that matches a one ore more tags defiend on an input. It supports `*` as a wildcards |
| **Pattern**      | string  | Yes      | -       | The regex pattern that should be applied to the log line. |
| **AllowEmpty**   | boolean | No       | `true`  | Wether or not the parser should skip empty fields. |
| **TimeFormat**   | string  | No       | -       | A time format to parse a timestamp into a valid internaly represantation. |
| **TimeKey**      | string  | No       | -       | The key under which the timestamp is found. |