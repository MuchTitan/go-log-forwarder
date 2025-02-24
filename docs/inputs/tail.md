# Tail Input Configuration

## Overview

The `tail` input plugin reads log files and streams new lines as they are appended to the file. It is configured via a YAML configuration file.

## Configuration

Below is an example of how to configure the `tail` input in the YAML configuration file:

```yaml
inputs:
  - Type: tail
    Name: "my_tail_input"
    Glob: "./logs/*.log"
    Tag: "log_tag"
    CleanUpThreshold: 3
    EnableDB: true
    DBFile: "./state.db"
```

### Configuration Parameters

| Parameter          | Type     | Required | Default | Description |
|-------------------|---------|----------|---------|-------------|
| **Type**         | string  | Yes      | -       | Must be set to `tail` to use the tail input. |
| **Name**         | string  | No       | `tail`  | The name of the input instance. |
| **Glob**         | string  | Yes      | -       | The file path pattern to watch (e.g., `./logs/*.log`). |
| **Tag**          | string  | No       | `tail`  | A tag associated with the log events. |
| **CleanUpThreshold** | integer | No   | `3`     | Number of old database entries to keep. |
| **EnableDB**     | boolean | No       | `false` | If `true`, enables state persistence in an SQLite database. |
| **DBFile**       | string  | No       | Auto    | Path to the SQLite database file for storing file states. If not provided, a default is generated based on the glob pattern. |

## Behavior

- The plugin monitors files matching the `Glob` pattern.
- New lines appended to the files are sent as log events.
- If `EnableDB` is `true`, file state is saved, allowing the plugin to resume reading from the last known position upon restart.
- Uses debounce timers to avoid excessive processing of file events.

## Usage

Ensure that the provided `Glob` pattern correctly matches the target log files. If using persistence, verify that the `DBFile` path is writable.

For troubleshooting, enable logging to inspect file event processing and state management behavior.
