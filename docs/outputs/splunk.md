# SPLUNK Output Configuration

## Overview

This document describes the configuration parameters for the `splunk` output of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `splunk` output in the YAML configuration file:

```yaml
outputs:
  - Type: splunk
    Name: my_splunk_output
    Match: "*_tag_*"
    Token: ${splunk_token}
    EventIndex: your_index
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -         | Must be set to `splunk` to use the splunk output. |
| **Name**         | string  | No       | `splunk`  | The name of the output instance. |
| **Match**        | string  | No       | `*`       | A string that matches a one ore more tags defiend on an input. It supports `*` as a wildcards. |
| **Token**        | string  | Yes      | -         | The token for the Splunk HTTP Event Collector interface. |
| **EventIndex**   | string  | Yes      | -         | The name of the index on what the data should be indexed. |
| **Host**         | string  | No       | `localhost` | The ip address or hostname of the target Splunk service. |
| **Port**         | integear | No       | `8088`   | The port of the target Splunk service. |
| **Compress**     | boolean  | No       | `false`  | Wether or not the data should be compressed using `gzip` before sending. |
| **VerifyTLS**    | boolean  | No       | `false`  | Wether or not the log forwarder should verify tls. |
| **SendRaw**      | boolean  | No       | `false`  | Wether or not the data should be send without parsing. |
| **EventHost**    | string  | No       | `Hostname`| This field specifies the source field in a splunk event. |
| **EventSourcetype**| string  | No       | `JSON`  | This field specifies the sourcetype field in a splunk event. |
| **EventFields**         | map  | No       | -  | This can contain key value pairs that would be appended to very splunk event. This is not supported when `SendRaw` is enabled. |