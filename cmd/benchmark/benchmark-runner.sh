#!/bin/sh

# Benchmark runner script for testing multiple Nostr relay implementations
# This script coordinates testing all relays and aggregates results

set -e

# Configuration from environment variables
BENCHMARK_EVENTS="${BENCHMARK_EVENTS:-10000}"
BENCHMARK_WORKERS="${BENCHMARK_WORKERS:-8}"
BENCHMARK_DURATION="${BENCHMARK_DURATION:-60s}"
BENCHMARK_TARGETS="${BENCHMARK_TARGETS:-next-orly:8080,khatru-sqlite:3334,khatru-badger:3334,relayer-basic:7447,strfry:8080,nostr-rs-relay:8080}"
OUTPUT_DIR="${OUTPUT_DIR:-/reports}"

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Generate timestamp for this benchmark run
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RUN_DIR="${OUTPUT_DIR}/run_${TIMESTAMP}"
mkdir -p "${RUN_DIR}"

echo "=================================================="
echo "Nostr Relay Benchmark Suite"
echo "=================================================="
echo "Timestamp: $(date)"
echo "Events per test: ${BENCHMARK_EVENTS}"
echo "Concurrent workers: ${BENCHMARK_WORKERS}"
echo "Test duration: ${BENCHMARK_DURATION}"
echo "Graph traversal: ${BENCHMARK_GRAPH_TRAVERSAL:-false}"
echo "Output directory: ${RUN_DIR}"
echo "=================================================="

# Function to wait for relay to be ready
wait_for_relay() {
    local name="$1"
    local url="$2"
    local max_attempts=60
    local attempt=0
    
    echo "Waiting for ${name} to be ready at ${url}..."
    
    while [ $attempt -lt $max_attempts ]; do
        # Try wget first to obtain an HTTP status code
        local status=""
        status=$(wget --quiet --server-response --tries=1 --timeout=5 "http://${url}" 2>&1 | awk '/^  HTTP\//{print $2; exit}')
        
        # Fallback to curl to obtain an HTTP status code
        if [ -z "$status" ]; then
            status=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 5 "http://${url}" || echo 000)
        fi
        
        case "$status" in
            101|200|400|404|426)
                echo "${name} is ready! (HTTP ${status})"
                return 0
                ;;
        esac
        
        attempt=$((attempt + 1))
        echo "  Attempt ${attempt}/${max_attempts}: ${name} not ready yet (HTTP ${status:-none})..."
        sleep 2
    done
    
    echo "ERROR: ${name} failed to become ready after ${max_attempts} attempts"
    return 1
}

# Function to run benchmark against a specific relay
run_benchmark() {
    local relay_name="$1"
    local relay_url="$2"
    local output_file="$3"

    echo ""
    echo "=================================================="
    echo "Testing ${relay_name} at ws://${relay_url}"
    echo "=================================================="

    # Wait for relay to be ready
    if ! wait_for_relay "${relay_name}" "${relay_url}"; then
        echo "ERROR: ${relay_name} is not responding, skipping..."
        echo "RELAY: ${relay_name}" > "${output_file}"
        echo "STATUS: FAILED - Relay not responding" >> "${output_file}"
        echo "ERROR: Connection failed" >> "${output_file}"
        return 1
    fi

    # Run the benchmark
    echo "Running benchmark against ${relay_name}..."

    # Create temporary directory for this relay's data
    TEMP_DATA_DIR="/tmp/benchmark_${relay_name}_$$"
    mkdir -p "${TEMP_DATA_DIR}"

    # Run benchmark and capture both stdout and stderr
    if /app/benchmark \
        -datadir="${TEMP_DATA_DIR}" \
        -events="${BENCHMARK_EVENTS}" \
        -workers="${BENCHMARK_WORKERS}" \
        -duration="${BENCHMARK_DURATION}" \
        > "${output_file}" 2>&1; then

        echo "✓ Benchmark completed successfully for ${relay_name}"

        # Add relay identification to the report
        echo "" >> "${output_file}"
        echo "RELAY_NAME: ${relay_name}" >> "${output_file}"
        echo "RELAY_URL: ws://${relay_url}" >> "${output_file}"
        echo "TEST_TIMESTAMP: $(date -Iseconds)" >> "${output_file}"
        echo "BENCHMARK_CONFIG:" >> "${output_file}"
        echo "  Events: ${BENCHMARK_EVENTS}" >> "${output_file}"
        echo "  Workers: ${BENCHMARK_WORKERS}" >> "${output_file}"
        echo "  Duration: ${BENCHMARK_DURATION}" >> "${output_file}"

    else
        echo "✗ Benchmark failed for ${relay_name}"
        echo "" >> "${output_file}"
        echo "RELAY_NAME: ${relay_name}" >> "${output_file}"
        echo "RELAY_URL: ws://${relay_url}" >> "${output_file}"
        echo "STATUS: FAILED" >> "${output_file}"
        echo "TEST_TIMESTAMP: $(date -Iseconds)" >> "${output_file}"
    fi

    # Clean up temporary data
    rm -rf "${TEMP_DATA_DIR}"
}

# Function to run network graph traversal benchmark against a specific relay
run_graph_traversal_benchmark() {
    local relay_name="$1"
    local relay_url="$2"
    local output_file="$3"

    echo ""
    echo "=================================================="
    echo "Graph Traversal Benchmark: ${relay_name} at ws://${relay_url}"
    echo "=================================================="

    # Wait for relay to be ready
    if ! wait_for_relay "${relay_name}" "${relay_url}"; then
        echo "ERROR: ${relay_name} is not responding, skipping graph traversal..."
        echo "RELAY: ${relay_name}" > "${output_file}"
        echo "STATUS: FAILED - Relay not responding" >> "${output_file}"
        echo "ERROR: Connection failed" >> "${output_file}"
        return 1
    fi

    # Run the network graph traversal benchmark
    echo "Running network graph traversal benchmark against ${relay_name}..."

    # Create temporary directory for this relay's data
    TEMP_DATA_DIR="/tmp/graph_benchmark_${relay_name}_$$"
    mkdir -p "${TEMP_DATA_DIR}"

    # Run graph traversal benchmark via WebSocket
    if /app/benchmark \
        -graph-network \
        -relay-url="ws://${relay_url}" \
        -datadir="${TEMP_DATA_DIR}" \
        -workers="${BENCHMARK_WORKERS}" \
        > "${output_file}" 2>&1; then

        echo "✓ Graph traversal benchmark completed successfully for ${relay_name}"

        # Add relay identification to the report
        echo "" >> "${output_file}"
        echo "RELAY_NAME: ${relay_name}" >> "${output_file}"
        echo "RELAY_URL: ws://${relay_url}" >> "${output_file}"
        echo "TEST_TYPE: Graph Traversal (100k pubkeys, 3-degree follows)" >> "${output_file}"
        echo "TEST_TIMESTAMP: $(date -Iseconds)" >> "${output_file}"
        echo "BENCHMARK_CONFIG:" >> "${output_file}"
        echo "  Workers: ${BENCHMARK_WORKERS}" >> "${output_file}"

    else
        echo "✗ Graph traversal benchmark failed for ${relay_name}"
        echo "" >> "${output_file}"
        echo "RELAY_NAME: ${relay_name}" >> "${output_file}"
        echo "RELAY_URL: ws://${relay_url}" >> "${output_file}"
        echo "TEST_TYPE: Graph Traversal" >> "${output_file}"
        echo "STATUS: FAILED" >> "${output_file}"
        echo "TEST_TIMESTAMP: $(date -Iseconds)" >> "${output_file}"
    fi

    # Clean up temporary data
    rm -rf "${TEMP_DATA_DIR}"
}

# Function to generate aggregate report
generate_aggregate_report() {
    local aggregate_file="${RUN_DIR}/aggregate_report.txt"
    
    echo "Generating aggregate report..."
    
    cat > "${aggregate_file}" << EOF
================================================================
NOSTR RELAY BENCHMARK AGGREGATE REPORT
================================================================
Generated: $(date -Iseconds)
Benchmark Configuration:
  Events per test: ${BENCHMARK_EVENTS}
  Concurrent workers: ${BENCHMARK_WORKERS}
  Test duration: ${BENCHMARK_DURATION}
  
Relays tested: $(echo "${BENCHMARK_TARGETS}" | tr ',' '\n' | wc -l)

================================================================
SUMMARY BY RELAY
================================================================

EOF

    # Process each relay's results
    echo "${BENCHMARK_TARGETS}" | tr ',' '\n' | while IFS=':' read -r relay_name relay_port; do
        if [ -z "${relay_name}" ] || [ -z "${relay_port}" ]; then
            continue
        fi
        
        relay_file="${RUN_DIR}/${relay_name}_results.txt"
        
        echo "Relay: ${relay_name}" >> "${aggregate_file}"
        echo "----------------------------------------" >> "${aggregate_file}"
        
        if [ -f "${relay_file}" ]; then
            # Extract key metrics from the relay's report
            if grep -q "STATUS: FAILED" "${relay_file}"; then
                echo "Status: FAILED" >> "${aggregate_file}"
                grep "ERROR:" "${relay_file}" | head -1 >> "${aggregate_file}" || echo "Error: Unknown failure" >> "${aggregate_file}"
            else
                echo "Status: COMPLETED" >> "${aggregate_file}"
                
                # Extract performance metrics
                grep "Events/sec:" "${relay_file}" | head -3 >> "${aggregate_file}" || true
                grep "Success Rate:" "${relay_file}" | head -3 >> "${aggregate_file}" || true
                grep "Avg Latency:" "${relay_file}" | head -3 >> "${aggregate_file}" || true
                grep "P95 Latency:" "${relay_file}" | head -3 >> "${aggregate_file}" || true
                grep "Memory:" "${relay_file}" | head -3 >> "${aggregate_file}" || true
            fi
        else
            echo "Status: NO RESULTS FILE" >> "${aggregate_file}"
            echo "Error: Results file not found" >> "${aggregate_file}"
        fi
        
        echo "" >> "${aggregate_file}"
    done
    
    cat >> "${aggregate_file}" << EOF

================================================================
DETAILED RESULTS
================================================================

Individual relay reports are available in:
$(ls "${RUN_DIR}"/*_results.txt 2>/dev/null | sed 's|^|  - |' || echo "  No individual reports found")

================================================================
BENCHMARK COMPARISON TABLE
================================================================

EOF

    # Create a comparison table
    printf "%-20s %-10s %-15s %-15s %-15s\n" "Relay" "Status" "Peak Tput/s" "Avg Latency" "Success Rate" >> "${aggregate_file}"
    printf "%-20s %-10s %-15s %-15s %-15s\n" "----" "------" "-----------" "-----------" "------------" >> "${aggregate_file}"
    
    echo "${BENCHMARK_TARGETS}" | tr ',' '\n' | while IFS=':' read -r relay_name relay_port; do
        if [ -z "${relay_name}" ] || [ -z "${relay_port}" ]; then
            continue
        fi
        
        relay_file="${RUN_DIR}/${relay_name}_results.txt"
        
        if [ -f "${relay_file}" ]; then
            if grep -q "STATUS: FAILED" "${relay_file}"; then
                printf "%-20s %-10s %-15s %-15s %-15s\n" "${relay_name}" "FAILED" "-" "-" "-" >> "${aggregate_file}"
            else
                # Extract metrics for the table
                peak_tput=$(grep "Events/sec:" "${relay_file}" | head -1 | awk '{print $2}' || echo "-")
                avg_latency=$(grep "Avg Latency:" "${relay_file}" | head -1 | awk '{print $3}' || echo "-")
                success_rate=$(grep "Success Rate:" "${relay_file}" | head -1 | awk '{print $3}' || echo "-")
                
                printf "%-20s %-10s %-15s %-15s %-15s\n" "${relay_name}" "OK" "${peak_tput}" "${avg_latency}" "${success_rate}" >> "${aggregate_file}"
            fi
        else
            printf "%-20s %-10s %-15s %-15s %-15s\n" "${relay_name}" "NO DATA" "-" "-" "-" >> "${aggregate_file}"
        fi
    done
    
    echo "" >> "${aggregate_file}"
    echo "================================================================" >> "${aggregate_file}"
    echo "End of Report" >> "${aggregate_file}"
    echo "================================================================" >> "${aggregate_file}"
}

# Main execution
echo "Starting relay benchmark suite..."

# Check if graph traversal mode is enabled
BENCHMARK_GRAPH_TRAVERSAL="${BENCHMARK_GRAPH_TRAVERSAL:-false}"

# Parse targets and run benchmarks
echo "${BENCHMARK_TARGETS}" | tr ',' '\n' | while IFS=':' read -r relay_name relay_port; do
    if [ -z "${relay_name}" ] || [ -z "${relay_port}" ]; then
        echo "WARNING: Skipping invalid target: ${relay_name}:${relay_port}"
        continue
    fi

    relay_url="${relay_name}:${relay_port}"
    output_file="${RUN_DIR}/${relay_name}_results.txt"

    run_benchmark "${relay_name}" "${relay_url}" "${output_file}"

    # Small delay between tests
    sleep 5
done

# Run graph traversal benchmarks if enabled
if [ "${BENCHMARK_GRAPH_TRAVERSAL}" = "true" ]; then
    echo ""
    echo "=================================================="
    echo "Starting Graph Traversal Benchmark Suite"
    echo "=================================================="
    echo "This tests 100k pubkeys with 1-1000 follows each"
    echo "and performs 3-degree traversal queries"
    echo "=================================================="

    echo "${BENCHMARK_TARGETS}" | tr ',' '\n' | while IFS=':' read -r relay_name relay_port; do
        if [ -z "${relay_name}" ] || [ -z "${relay_port}" ]; then
            continue
        fi

        relay_url="${relay_name}:${relay_port}"
        output_file="${RUN_DIR}/${relay_name}_graph_traversal_results.txt"

        run_graph_traversal_benchmark "${relay_name}" "${relay_url}" "${output_file}"

        # Longer delay between graph traversal tests (they're more intensive)
        sleep 10
    done
fi

# Generate aggregate report
generate_aggregate_report

echo ""
echo "=================================================="
echo "Benchmark Suite Completed!"
echo "=================================================="
echo "Results directory: ${RUN_DIR}"
echo "Aggregate report: ${RUN_DIR}/aggregate_report.txt"
echo ""

# Display summary
if [ -f "${RUN_DIR}/aggregate_report.txt" ]; then
    echo "Quick Summary:"
    echo "=============="
    grep -A 10 "BENCHMARK COMPARISON TABLE" "${RUN_DIR}/aggregate_report.txt" | tail -n +4
fi

echo ""
echo "All benchmark files:"
ls -la "${RUN_DIR}/"
echo ""
echo "Benchmark suite finished at: $(date)"