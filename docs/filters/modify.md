# MODIFY Filter Configuration

## Overview

This document describes the configuration parameters for the `modify` filter of the Go log-forwarder package. The modify filter allows you to transform log data by adding, removing, or modifying fields based on specified conditions.

## Configuration

Below is an example of how to configure the `modify` filter in the YAML configuration file:

```yaml
filters:
  - Type: modify
    Name: "my_modify_filter"
    Match: "*_tag_*"
    Condition:
      key_exists: 
        - "required_field"
        - "another_required_field"
      key_match:
        - "error.*"
        - "warning.*"
      value_equals:
        - "ERROR"
        - "WARNING"
    Set:
      new_field: "new_value"
    Add:
      additional_field: "additional_value"
    Rename:
      old_field: "new_field_name"
    Remove:
      - "field_to_remove"
      - "another_field"
    RemoveRegex:
      - "temp_.*"
      - ".*_debug"
    RemoveWildcard:
      - "test*"
      - "*_temp"
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -       | Must be set to `modify` to use the modify filter. |
| **Name**         | string  | No       | `modify`| The name of the filter instance. |
| **Match**        | string  | No       | `*`     | A string that matches one or more tags defined on an input. It supports `*` as wildcards. |
| **Condition**    | map     | Yes      | -       | A map of conditions that must be met for the modifications to be applied. Each condition can have multiple values. See [Conditions](#conditions) for details. |
| **Set**          | map     | No       | -       | A map of key-value pairs to set or overwrite in the log data. |
| **Add**          | map     | No       | -       | A map of key-value pairs to add to the log data if the key doesn't exist. |
| **Rename**       | map     | No       | -       | A map of old key to new key for renaming fields. Only renames if the new key doesn't exist. |
| **HardRename**   | map     | No       | -       | A map of old key to new key for forced renaming. Overwrites the new key if it exists. |
| **Remove**       | array   | No       | -       | An array of field names to remove from the log data. |
| **RemoveRegex**  | array   | No       | -       | An array of regex patterns to match against field names for removal. Uses [RE2](https://github.com/google/re2/wiki/Syntax) syntax. |
| **RemoveWildcard**| array  | No       | -       | An array of wildcard patterns to match against field names for removal. Supports `*` and `?` wildcards. |

### Conditions

The `Condition` parameter supports the following condition types, each of which can have multiple values:

| Condition Type        | Description |
|----------------------|-------------|
| **key_exists**       | Checks if any of the specified keys exist in the log data. |
| **no_key_exists**    | Checks if none of the specified keys exist in the log data. |
| **key_match**        | Checks if any key matches any of the specified regex patterns. |
| **no_key_match**     | Checks if no key matches any of the specified regex patterns. |
| **key_equals**       | Checks if any key exactly equals any of the specified values. |
| **no_key_equal**     | Checks if no key equals any of the specified values. |
| **value_equals**     | Checks if any string value equals any of the specified values. |
| **no_value_equals**  | Checks if no string value equals any of the specified values. |

### Examples

1. Multiple key existence checks:
```yaml
filters:
  - Type: modify
    Condition:
      key_exists:
        - "timestamp"
        - "level"
        - "message"
    Set:
      log_level: "INFO"
    Remove:
      - "debug_info"
```

2. Complex transformation with multiple conditions:
```yaml
filters:
  - Type: modify
    Condition:
      key_match:
        - "error.*"
        - "warning.*"
      value_equals:
        - "ERROR"
        - "WARNING"
    Rename:
      error_message: "message"
    Add:
      severity: "ERROR"
    RemoveRegex:
      - "temp_.*"
      - ".*_debug"
```

3. Wildcard-based cleanup with multiple conditions:
```yaml
filters:
  - Type: modify
    Condition:
      value_equals:
        - "test"
        - "debug"
      no_key_exists:
        - "production"
        - "live"
    RemoveWildcard:
      - "test*"
      - "*_temp"
```

### Warning

1. At least one condition must be specified.
2. At least one modification operation (Set, Add, Rename, Remove, etc.) must be configured.
3. The filter will only apply modifications if all specified conditions are met (for each condition type, at least one value must match).
4. For regex patterns, use the [RE2](https://github.com/google/re2/wiki/Syntax) syntax.
5. When multiple values are specified for a condition, the condition is met if any of the values match. 