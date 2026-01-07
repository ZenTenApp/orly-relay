#!/bin/bash
# Build the WasmDB WASM module for browser use
#
# Output: wasmdb.wasm in the repository root
#
# Usage:
#   ./scripts/build-wasm.sh
#   ./scripts/build-wasm.sh --output /path/to/output/wasmdb.wasm

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

OUTPUT_PATH="${REPO_ROOT}/wasmdb.wasm"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --output|-o)
            OUTPUT_PATH="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "Building WasmDB WASM module..."
echo "Output: ${OUTPUT_PATH}"

cd "${REPO_ROOT}"

# Build with optimizations
GOOS=js GOARCH=wasm go build \
    -ldflags="-s -w" \
    -o "${OUTPUT_PATH}" \
    ./cmd/wasmdb

# Get the size
SIZE=$(du -h "${OUTPUT_PATH}" | cut -f1)
echo "Build complete: ${OUTPUT_PATH} (${SIZE})"

# Copy wasm_exec.js from Go installation if not present
WASM_EXEC="${REPO_ROOT}/wasm_exec.js"
if [ ! -f "${WASM_EXEC}" ]; then
    GO_ROOT=$(go env GOROOT)
    # Try lib/wasm first (newer Go versions), then misc/wasm
    if [ -f "${GO_ROOT}/lib/wasm/wasm_exec.js" ]; then
        cp "${GO_ROOT}/lib/wasm/wasm_exec.js" "${WASM_EXEC}"
        echo "Copied wasm_exec.js to ${WASM_EXEC}"
    elif [ -f "${GO_ROOT}/misc/wasm/wasm_exec.js" ]; then
        cp "${GO_ROOT}/misc/wasm/wasm_exec.js" "${WASM_EXEC}"
        echo "Copied wasm_exec.js to ${WASM_EXEC}"
    else
        echo "Warning: wasm_exec.js not found in Go installation"
        echo "Checked: ${GO_ROOT}/lib/wasm/wasm_exec.js"
        echo "Checked: ${GO_ROOT}/misc/wasm/wasm_exec.js"
        echo "You'll need to copy it manually from your Go installation"
    fi
fi

echo "Done!"
