#!/bin/bash

# Run wasmdb tests using Node.js with fake-indexeddb
# This script builds the test binary and runs it in Node.js

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTDATA_DIR="$SCRIPT_DIR/testdata"
WASM_FILE="$TESTDATA_DIR/wasmdb_test.wasm"

# Ensure Node.js dependencies are installed
if [ ! -d "$TESTDATA_DIR/node_modules" ]; then
    echo "Installing Node.js dependencies..."
    cd "$TESTDATA_DIR"
    npm install
    cd - > /dev/null
fi

# Build the test binary
echo "Building WASM test binary..."
GOOS=js GOARCH=wasm CGO_ENABLED=0 go test -c -o "$WASM_FILE" "$SCRIPT_DIR"

# Run the tests
echo "Running tests in Node.js..."
node "$TESTDATA_DIR/run_wasm_tests.mjs" "$WASM_FILE" "$@"
