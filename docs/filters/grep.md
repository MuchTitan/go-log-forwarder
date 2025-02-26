# GREP Filter Configuration

## Overview

This document describes the configuration parameters for the `grep` filter of the Go log-forwarder package.

## Configuration

Below is an example of how to configure the `grep` filter in the YAML configuration file:

```yaml
filters:
  - Type: grep
    Name: "my_grep_filter"
    Match: "*_tag_*"
    Exclude: 
        - "(?:\d[ -]*?){13,16}" # This would filter every log line with a creditcard number in it
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -       | Must be set to `grep` to use the grep filter. |
| **Name**         | string  | No       | `grep`  | The name of the filter instance. |
| **Match**        | string  | No       | `*`     | A string that matches a one ore more tags defiend on an input. It supports `*` as a wildcards |
| **Op**           | string  | No       | `and`  | The operation that should be performed based on a the matches in the exclude and include. Available Options are `and` and `or` |
| **Include**      | string  | Yes       | -  | A Regex pattern in the [RE2](https://github.com/google/re2/wiki/Syntax) syntax applied to the log line. Filters logline out when not positive |
| **Exclude**      | string  | Yes       | -  | A Regex pattern in the [RE2](https://github.com/google/re2/wiki/Syntax) syntax applied to the log line. Filters logline out when positive |

### Warning

You need to defiend atleast one exclude or one include.