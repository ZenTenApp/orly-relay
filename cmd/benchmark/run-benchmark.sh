#!/bin/bash

# Wrapper script to run the benchmark suite and automatically shut down when complete
#
# Usage:
#   ./run-benchmark.sh           # Use disk-based storage (default)
#   ./run-benchmark.sh --ramdisk # Use /dev/shm ramdisk for maximum performance

set -e

# Parse command line arguments
USE_RAMDISK=false
for arg in "$@"; do
    case $arg in
        --ramdisk)
            USE_RAMDISK=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --ramdisk    Use /dev/shm ramdisk storage instead of disk"
            echo "               This eliminates disk I/O bottlenecks for accurate"
            echo "               relay performance measurement."
            echo "  --help, -h   Show this help message"
            echo ""
            echo "Requirements for --ramdisk:"
            echo "  - /dev/shm must be available (tmpfs mount)"
            echo "  - At least 8GB available in /dev/shm recommended"
            echo "  - Increase size with: sudo mount -o remount,size=16G /dev/shm"
            exit 0
            ;;
        *)
            echo "Unknown option: $arg"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Determine docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Set data directory and compose files based on mode
if [ "$USE_RAMDISK" = true ]; then
    DATA_BASE="/dev/shm/benchmark"
    COMPOSE_FILES="-f docker-compose.yml -f docker-compose.ramdisk.yml"

    echo "======================================================"
    echo "  RAMDISK BENCHMARK MODE"
    echo "======================================================"

    # Check /dev/shm availability
    if [ ! -d "/dev/shm" ]; then
        echo "ERROR: /dev/shm is not available on this system."
        echo "This benchmark requires a tmpfs-mounted /dev/shm for RAM-based storage."
        exit 1
    fi

    # Check available space in /dev/shm (need at least 8GB for benchmarks)
    SHM_AVAILABLE_KB=$(df /dev/shm | tail -1 | awk '{print $4}')
    SHM_AVAILABLE_GB=$((SHM_AVAILABLE_KB / 1024 / 1024))
    echo "  Storage location: ${DATA_BASE}"
    echo "  Available RAM: ${SHM_AVAILABLE_GB}GB"
    echo "  This eliminates disk I/O bottlenecks for accurate"
    echo "  relay performance measurement."
    echo "======================================================"
    echo ""

    if [ "$SHM_AVAILABLE_KB" -lt 8388608 ]; then
        echo "WARNING: Less than 8GB available in /dev/shm (${SHM_AVAILABLE_GB}GB available)"
        echo "Benchmarks may fail if databases grow too large."
        echo "Consider increasing tmpfs size: sudo mount -o remount,size=16G /dev/shm"
        echo ""
        read -p "Continue anyway? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
else
    DATA_BASE="./data"
    COMPOSE_FILES="-f docker-compose.yml"

    echo "======================================================"
    echo "  DISK-BASED BENCHMARK MODE (default)"
    echo "======================================================"
    echo "  Storage location: ${DATA_BASE}"
    echo "  Tip: Use --ramdisk for faster benchmarks without"
    echo "       disk I/O bottlenecks."
    echo "======================================================"
    echo ""
fi

# Clean old data directories (may be owned by root from Docker)
if [ -d "${DATA_BASE}" ]; then
    echo "Cleaning old data directories at ${DATA_BASE}..."
    if ! rm -rf "${DATA_BASE}" 2>/dev/null; then
        # If normal rm fails (permission denied), try with sudo for ramdisk
        if [ "$USE_RAMDISK" = true ]; then
            echo "Need elevated permissions to clean ramdisk..."
            if ! sudo rm -rf "${DATA_BASE}" 2>/dev/null; then
                echo ""
                echo "ERROR: Cannot remove data directories."
                echo "Please run: sudo rm -rf ${DATA_BASE}"
                echo "Then run this script again."
                exit 1
            fi
        else
            # Provide clear instructions for disk-based mode
            echo ""
            echo "ERROR: Cannot remove data directories due to permission issues."
            echo "This happens because Docker creates files as root."
            echo ""
            echo "Please run one of the following to clean up:"
            echo "  sudo rm -rf ${DATA_BASE}/"
            echo "  sudo chown -R \$(id -u):\$(id -g) ${DATA_BASE}/ && rm -rf ${DATA_BASE}/"
            echo ""
            echo "Then run this script again."
            exit 1
        fi
    fi
fi

# Stop any running containers from previous runs
echo "Stopping any running containers..."
$DOCKER_COMPOSE $COMPOSE_FILES down 2>/dev/null || true

# Create fresh data directories with correct permissions
echo "Preparing data directories at ${DATA_BASE}..."

if [ "$USE_RAMDISK" = true ]; then
    # Create ramdisk directories
    mkdir -p "${DATA_BASE}"/{next-orly-badger,next-orly-dgraph,next-orly-neo4j,dgraph-zero,dgraph-alpha,neo4j,neo4j-logs,khatru-sqlite,khatru-badger,relayer-basic,strfry,nostr-rs-relay,rely-sqlite,postgres}
    chmod 777 "${DATA_BASE}"/{next-orly-badger,next-orly-dgraph,next-orly-neo4j,dgraph-zero,dgraph-alpha,neo4j,neo4j-logs,khatru-sqlite,khatru-badger,relayer-basic,strfry,nostr-rs-relay,rely-sqlite,postgres}
else
    # Create disk directories (relative path)
    mkdir -p data/{next-orly-badger,next-orly-dgraph,next-orly-neo4j,dgraph-zero,dgraph-alpha,neo4j,neo4j-logs,khatru-sqlite,khatru-badger,relayer-basic,strfry,nostr-rs-relay,rely-sqlite,postgres}
    chmod 777 data/{next-orly-badger,next-orly-dgraph,next-orly-neo4j,dgraph-zero,dgraph-alpha,neo4j,neo4j-logs,khatru-sqlite,khatru-badger,relayer-basic,strfry,nostr-rs-relay,rely-sqlite,postgres}
fi

echo "Building fresh Docker images..."
# Force rebuild to pick up latest code changes
$DOCKER_COMPOSE $COMPOSE_FILES build --no-cache benchmark-runner next-orly-badger next-orly-dgraph next-orly-neo4j rely-sqlite

echo ""
echo "Starting benchmark suite..."
echo "This will automatically shut down all containers when the benchmark completes."
echo ""

# Run docker compose with flags to exit when benchmark-runner completes
$DOCKER_COMPOSE $COMPOSE_FILES up --exit-code-from benchmark-runner --abort-on-container-exit

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    $DOCKER_COMPOSE $COMPOSE_FILES down 2>/dev/null || true

    if [ "$USE_RAMDISK" = true ]; then
        echo "Cleaning ramdisk data at ${DATA_BASE}..."
        rm -rf "${DATA_BASE}" 2>/dev/null || sudo rm -rf "${DATA_BASE}" 2>/dev/null || true
    fi
}

# Register cleanup on script exit
trap cleanup EXIT

echo ""
echo "Benchmark suite has completed and all containers have been stopped."
echo "Check the ./reports/ directory for results."
