#!/bin/bash

# Define the target directory
TARGET_DIR="./logs"
mkdir -p "$TARGET_DIR"

# Number of files to generate
FILE_COUNT=500

# Function to generate a random string
generate_random_string() {
    cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 10 | head -n 1
}

# Function to generate a random array of strings
generate_string_array() {
    local count=$(shuf -i 2-5 -n 1)
    local array="["
    for ((i = 1; i <= count; i++)); do
        if [ $i -gt 1 ]; then
            array+=","
        fi
        array+="\"$(generate_random_string)\""
    done
    array+="]"
    echo "$array"
}

# Function to generate a nested object
generate_nested_object() {
    jq -cn \
        --arg timestamp "$(date +%s%N)" \
        --arg status "$(shuf -e "success" "error" "warning" -n 1)" \
        --argjson count $(shuf -i 1-100 -n 1) \
        --argjson is_valid $(shuf -i 0-1 -n 1) \
        '{
            timestamp: $timestamp,
            status: $status,
            metrics: {
                count: $count,
                is_valid: ($is_valid == 1)
            }
        }'
}

# Function to generate a single complex JSON line
generate_complex_json_line() {
    jq -cn \
        --arg id "$(uuidgen)" \
        --arg name "$(generate_random_string)" \
        --argjson active $(shuf -i 0-1 -n 1) \
        --argjson score $(shuf -i 0-100 -n 1) \
        --argjson tags "$(generate_string_array)" \
        --argjson nested "$(generate_nested_object)" \
        --argjson coordinates "[$(shuf -i 0-100 -n 1),$(shuf -i 0-100 -n 1)]" \
        '{
            id: $id,
            name: $name,
            active: ($active == 1),
            score: $score,
            tags: $tags,
            metadata: $nested,
            location: {
                type: "point",
                coordinates: $coordinates
            },
            timestamp: (now | floor)
        }'
}

# Function to generate a single file
generate_file() {
    local file_num=$1
    local file_name="${TARGET_DIR}/complex_file_${file_num}.log"
    local line_count=$(shuf -i 25-75 -n 1)

    rm -f "$file_name"
    : >"$file_name"

    # Generate and write lines
    for ((j = 1; j <= line_count; j++)); do
        generate_complex_json_line >>"$file_name"
    done

    # Print line count to stdout for aggregation
    echo "$line_count"
}

export -f generate_file
export -f generate_complex_json_line
export -f generate_random_string
export -f generate_string_array
export -f generate_nested_object
export TARGET_DIR

# Check if GNU Parallel is installed
if ! command -v parallel &>/dev/null; then
    echo "GNU Parallel is not installed. Please install it first."
    echo "On Ubuntu/Debian: sudo apt-get install parallel"
    echo "On MacOS: brew install parallel"
    exit 1
fi

# Generate files in parallel and sum line counts
ALL_LINES=$(seq 1 $FILE_COUNT | parallel --bar --jobs 12 generate_file | awk '{s+=$1} END {print s}')

echo "Wrote $ALL_LINES complex log lines into $FILE_COUNT test log files"

