# TCP Input Configuration

## Overview

This document describes the configuration parameters for the `tcp` input of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `tcp` input in the YAML configuration file:

```yaml
inputs:
  - Type: tcp
    Name: "my_tcp_input"
    Tag: "tcp_tag"
    BufferSize: 128000 #128KB
    ListenAddr: "127.0.0.1"
    Port: 8080
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -       | Must be set to `tcp` to use the tcp input. |
| **Name**         | string  | No       | `tcp`  | The name of the input instance. |
| **Tag**          | string  | No       | `tcp`  | A tag associated with the log events. |
| **ListenAddr**   | string  | No       | `0.0.0.0` | The Address on which the tcp input should listen on |
| **Port**         | int     | No       | 6666 | The Port on which the tcp input should listen on |
| **BufferSize**   | int     | No       | 64000 | The size of the read buffer in bytes. |
| **Timeout**      | int     | No       | 10 | The connection timeout duration in minutes. |