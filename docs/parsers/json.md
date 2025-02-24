# JSON Parser Configuration

## Overview

This document describes the configuration parameters for the `json` parser of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `json` parser in the YAML configuration file:

```yaml
parsers:
  - Type: json
    Name: "my_json_parser"
    Match: "*_tag_*"
```

### Configuration Parameters

If you want to extract the timestamp from a log line you need to specify both TimeFormat and TimeKey

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -       | Must be set to `json` to use the json parser. |
| **Name**         | string  | No       | `json`  | The name of the parser instance. |
| **Match**        | string  | No       | `*`     | A string that matches a one ore more tags defiend on an input. It supports `*` as a wildcards |
| **TimeFormat**   | string  | No       | -       | A time format to parse a timestamp into a valid internaly represantation. |
| **TimeKey**      | string  | No       | -       | The key under which the timestamp is found. |