# STDOUT Output Configuration

## Overview

This document describes the configuration parameters for the `stdout` output of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `stdout` output in the YAML configuration file:

```yaml
outputs:
  - Type: stdout
    Name: my_stdout_output
    Match: "*_tag_*"
    Format: json
    Colors: true
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -         | Must be set to `stdout` to use the stdout output. |
| **Name**         | string  | No       | `stdout`  | The name of the output instance. |
| **Match**        | string  | No       | `*`       | A string that matches a one ore more tags defiend on an input. It supports `*` as a wildcards. |
| **Format**       | string  | No       | `json`    | The format what should be used. Available options `json`, `plain` and `template`. |
| **JsonIndent**   | boolean | No       | `false`   | Wether or not the output in json format should be indented. |
| **Template**     | string  | No       | -         | The template what should be used when selecting the `template` format. See [here](https://pkg.go.dev/text/template) on how to do it. |
| **Colors**        | boolean | No       | `false`  | Wether or not the output should be colorized. |