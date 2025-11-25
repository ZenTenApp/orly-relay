#!/bin/bash
set -euo pipefail

# scripts/benchmark.sh - Run full benchmark suite on a relay at a configurable address
#
# Usage:
#   ./scripts/benchmark.sh [relay_address] [relay_port]
#
# Example:
#   ./scripts/benchmark.sh localhost 3334
#   ./scripts/benchmark.sh nostr.example.com 8080
#
# If relay_address and relay_port are not provided, defaults to localhost:3334

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
cd "$REPO_ROOT"

# Default values
RELAY_ADDRESS="${1:-localhost}"
RELAY_PORT="${2:-3334}"
RELAY_URL="ws://${RELAY_ADDRESS}:${RELAY_PORT}"
BENCHMARK_EVENTS="${BENCHMARK_EVENTS:-10000}"
BENCHMARK_WORKERS="${BENCHMARK_WORKERS:-8}"
BENCHMARK_DURATION="${BENCHMARK_DURATION:-60s}"
REPORTS_DIR="${REPORTS_DIR:-$REPO_ROOT/cmd/benchmark/reports}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RUN_DIR="${REPORTS_DIR}/run_${TIMESTAMP}"

# Ensure the benchmark binary is built
BENCHMARK_BIN="${REPO_ROOT}/cmd/benchmark/benchmark"
if [[ ! -x "$BENCHMARK_BIN" ]]; then
    echo "Building benchmark binary (pure Go + purego)..."
    cd "$REPO_ROOT/cmd/benchmark"
    CGO_ENABLED=0 go build -o "$BENCHMARK_BIN" .
    # Download libsecp256k1.so from nostr repository (runtime optional)
    wget -q https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so \
        -O "$(dirname "$BENCHMARK_BIN")/libsecp256k1.so" 2>/dev/null || \
        echo "Warning: Failed to download libsecp256k1.so (optional for performance)"
    chmod +x "$(dirname "$BENCHMARK_BIN")/libsecp256k1.so" 2>/dev/null || true
    cd "$REPO_ROOT"
fi

# Create output directory
mkdir -p "${RUN_DIR}"

echo "=================================================="
echo "Nostr Relay Benchmark"
echo "=================================================="
echo "Timestamp: $(date)"
echo "Target Relay: ${RELAY_URL}"
echo "Events per test: ${BENCHMARK_EVENTS}"
echo "Concurrent workers: ${BENCHMARK_WORKERS}"
echo "Test duration: ${BENCHMARK_DURATION}"
echo "Output directory: ${RUN_DIR}"
echo "=================================================="

# Function to wait for relay to be ready
wait_for_relay() {
    local url="$1"
    local max_attempts=30
    local attempt=0
    
    echo "Waiting for relay to be ready at ${url}..."
    
    while [ $attempt -lt $max_attempts ]; do
        # Try to get HTTP status code with curl
        local status=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 5 "http://${RELAY_ADDRESS}:${RELAY_PORT}" || echo 000)
        
        case "$status" in
            101|200|400|404|426)
                echo "Relay is ready! (HTTP ${status})"
                return 0
                ;;
        esac
        
        attempt=$((attempt + 1))
        echo "  Attempt ${attempt}/${max_attempts}: Relay not ready yet (HTTP ${status})..."
        sleep 2
    done
    
    echo "ERROR: Relay failed to become ready after ${max_attempts} attempts"
    return 1
}

# Function to run benchmark against the relay
run_benchmark() {
    local output_file="${RUN_DIR}/benchmark_results.txt"
    
    echo ""
    echo "=================================================="
    echo "Testing relay at ${RELAY_URL}"
    echo "=================================================="
    
    # Wait for relay to be ready
    if ! wait_for_relay "${RELAY_ADDRESS}:${RELAY_PORT}"; then
        echo "ERROR: Relay is not responding, aborting..."
        echo "RELAY_URL: ${RELAY_URL}" > "${output_file}"
        echo "STATUS: FAILED - Relay not responding" >> "${output_file}"
        echo "ERROR: Connection failed" >> "${output_file}"
        return 1
    fi
    
    # Run the benchmark
    echo "Running benchmark against ${RELAY_URL}..."
    
    # Create temporary directory for benchmark data
    TEMP_DATA_DIR="/tmp/benchmark_${TIMESTAMP}"
    mkdir -p "${TEMP_DATA_DIR}"
    
    # Run benchmark and capture both stdout and stderr
    if "${BENCHMARK_BIN}" \
        -relay-url="${RELAY_URL}" \
        -datadir="${TEMP_DATA_DIR}" \
        -events="${BENCHMARK_EVENTS}" \
        -workers="${BENCHMARK_WORKERS}" \
        -duration="${BENCHMARK_DURATION}" \
        # > "${output_file}"
         2>&1; then
        echo "✓ Benchmark completed successfully"
        # Add relay identification to the report
        echo "" >> "${output_file}"
        echo "RELAY_URL: ${RELAY_URL}" >> "${output_file}"
        echo "TEST_TIMESTAMP: $(date -Iseconds)" >> "${output_file}"
        echo "BENCHMARK_CONFIG:" >> "${output_file}"
        echo "  Events: ${BENCHMARK_EVENTS}" >> "${output_file}"
        echo "  Workers: ${BENCHMARK_WORKERS}" >> "${output_file}"
        echo "  Duration: ${BENCHMARK_DURATION}" >> "${output_file}"    else
        echo "✗ Benchmark failed"
        echo "" >> "${output_file}"
        echo "RELAY_URL: ${RELAY_URL}" >> "${output_file}"
        echo "STATUS: FAILED" >> "${output_file}"
        echo "TEST_TIMESTAMP: $(date -Iseconds)" >> "${output_file}"
    fi
    
    # Clean up temporary data
    rm -rf "${TEMP_DATA_DIR}"
    
    return 0
}

# Main execution
echo "Starting relay benchmark..."
run_benchmark

# Display results
if [ -f "${RUN_DIR}/benchmark_results.txt" ]; then
    echo ""
    echo "=================================================="
    echo "Benchmark Results Summary"
    echo "=================================================="
    # Extract key metrics from the benchmark report
    if grep -q "STATUS: FAILED" "${RUN_DIR}/benchmark_results.txt"; then
        echo "Status: FAILED"
        grep "ERROR:" "${RUN_DIR}/benchmark_results.txt" | head -1 || echo "Error: Unknown failure"
    else
        echo "Status: COMPLETED"
        
        # Extract performance metrics
        grep "Events/sec:" "${RUN_DIR}/benchmark_results.txt" | head -3 || true
        grep "Success Rate:" "${RUN_DIR}/benchmark_results.txt" | head -3 || true
        grep "Avg Latency:" "${RUN_DIR}/benchmark_results.txt" | head -3 || true
        grep "P95 Latency:" "${RUN_DIR}/benchmark_results.txt" | head -3 || true
        grep "Memory:" "${RUN_DIR}/benchmark_results.txt" | head -3 || true
    fi
    
    echo ""
    echo "Full results available in: ${RUN_DIR}/benchmark_results.txt"
fi

echo ""
echo "=================================================="
echo "Benchmark Completed!"
echo "=================================================="
echo "Results directory: ${RUN_DIR}"
echo "Benchmark finished at: $(date)"