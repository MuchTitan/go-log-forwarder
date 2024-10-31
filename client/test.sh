#!/bin/bash

# Define the target directory
TARGET_DIR="./test"
mkdir -p "$TARGET_DIR"

# Number of files to generate
FILE_COUNT=500

# Function to generate a single JSON line
generate_json_line() {
    jq -cn \
        --arg key1 "$(date +%s%N)" \
        --arg key2 "$(uuidgen)" \
        --argjson active $(shuf -i 0-1 -n 1) \
        '{id: $key1, name: $key2, active: ($active == 1)}'
}

# Function to generate a single file
generate_file() {
    local file_num=$1
    local file_name="${TARGET_DIR}/file_${file_num}.log"
    local line_count=$(shuf -i 10-100 -n 1)

    # Create/truncate the file
    : >"$file_name"

    # Generate and write lines directly
    for ((j = 1; j <= line_count; j++)); do
        generate_json_line >>"$file_name"
    done

    echo "Created file_${file_num}.json with $line_count lines"
}

export -f generate_file
export -f generate_json_line
export TARGET_DIR

# Check if GNU Parallel is installed
if ! command -v parallel &>/dev/null; then
    echo "GNU Parallel is not installed. Please install it first."
    echo "On Ubuntu/Debian: sudo apt-get install parallel"
    echo "On MacOS: brew install parallel"
    exit 1
fi

# Generate files in parallel
# --jobs 0 means use all available CPU cores
# --bar shows a progress bar
seq 1 $FILE_COUNT |
    parallel --bar --jobs 12 generate_file

echo "Created $FILE_COUNT JSON files in $TARGET_DIR"
