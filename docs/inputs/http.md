# HTTP Input Configuration

## Overview

This document describes the configuration parameters for the `http` input of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `http` input in the YAML configuration file:

```yaml
inputs:
  - Type: http
    Name: "my_http_input"
    Tag: "http_tag"
    BufferSize: 128000 #128KB
    ListenAddr: "127.0.0.1"
    Port: 8080
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -       | Must be set to `http` to use the http input. |
| **Name**         | string  | No       | `http`  | The name of the input instance. |
| **Tag**          | string  | No       | `http`  | A tag associated with the log events. |
| **ListenAddr**   | string  | No       | `0.0.0.0` | The Address on which the tcp input should listen on |
| **Port**         | int     | No       | 8080 | The Port on which the tcp input should listen on |
| **BufferSize**   | int     | No       | 64000 | The size of the read buffer in bytes. |