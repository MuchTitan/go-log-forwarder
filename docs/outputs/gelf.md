# GELF Output Configuration

## Overview

This document describes the configuration parameters for the `gelf` output of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `gelf` output in the YAML configuration file:

```yaml
outputs:
  - Type: gelf
    Name: my_gelf_output
    Match: "*_tag_*"
    Token: ${gelf_token}
    EventIndex: your_index
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -         | Must be set to `gelf` to use the gelf output. |
| **Name**         | string  | No       | `gelf`  | The name of the output instance. |
| **Match**        | string  | No       | `*`       | A string that matches a one ore more tags defiend on an input. It supports `*` as a wildcards. |
| **Mode**         | string  | No       | `udp`  | In which mode the geld output should send. Available options are `udp` and `tcp`. |
| **HostKey**      | string  | Yes       | -     | Key which its value is used as the name of the host, source or application that sent this message. |
| **Host**         | string  | No       | `localhost` | The ip address or hostname of the target gelf service. |
| **Port**         | integear | No       | `12201`   | The port of the target gelf service. |