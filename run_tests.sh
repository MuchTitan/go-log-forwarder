#!/bin/bash

# Coverage Test Script - All Modules
# Runs tests across all packages and displays coverage summary

echo "========================================="
echo "Running Tests - All Modules"
echo "========================================="
echo ""

# Run tests for all packages with coverage
go test -coverprofile=coverage.out ./... 2>&1 | grep -E "(PASS|FAIL|ok|coverage:)"

if [ $? -ne 0 ]; then
    echo ""
    echo "Some tests may have failed. Showing coverage anyway..."
fi

echo ""
echo "========================================="
echo "Coverage Report - All Modules"
echo "========================================="
echo ""

# Display coverage by function
go tool cover -func=coverage.out

echo ""
echo "========================================="
echo "Coverage Summary"
echo "========================================="
echo ""

# Extract and display total coverage
go tool cover -func=coverage.out | grep total:
