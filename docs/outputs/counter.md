# COUNTER Output Configuration

## Overview

This document describes the configuration parameters for the `counter` output of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `counter` output in the YAML configuration file:

```yaml
outputs:
  - Type: counter
    Name: "my_counter_output"
    Match: "*_tag_*"
```

### Configuration Parameters


| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -         | Must be set to `counter` to use the counter output. |
| **Name**         | string  | No       | `counter` | The name of the output instance. |
| **Match**        | string  | No       | `*`       | A string that matches a one ore more tags defiend on an input. It supports `*` as a wildcards |