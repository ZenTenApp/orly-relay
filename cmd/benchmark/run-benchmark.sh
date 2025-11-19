#!/bin/bash

# Wrapper script to run the benchmark suite and automatically shut down when complete

set -e

# Determine docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Clean old data directories (may be owned by root from Docker)
if [ -d "data" ]; then
    echo "Cleaning old data directories..."
    if ! rm -rf data/ 2>/dev/null; then
        # If normal rm fails (permission denied), provide clear instructions
        echo ""
        echo "ERROR: Cannot remove data directories due to permission issues."
        echo "This happens because Docker creates files as root."
        echo ""
        echo "Please run one of the following to clean up:"
        echo "  sudo rm -rf data/"
        echo "  sudo chown -R \$(id -u):\$(id -g) data/ && rm -rf data/"
        echo ""
        echo "Then run this script again."
        exit 1
    fi
fi

# Stop any running containers from previous runs
echo "Stopping any running containers..."
$DOCKER_COMPOSE down 2>/dev/null || true

# Create fresh data directories with correct permissions
echo "Preparing data directories..."

# Clean Neo4j data to prevent "already running" errors
if [ -d "data/neo4j" ]; then
    echo "Cleaning Neo4j data directory..."
    rm -rf data/neo4j/*
fi

mkdir -p data/{next-orly-badger,next-orly-dgraph,next-orly-neo4j,dgraph-zero,dgraph-alpha,neo4j,neo4j-logs,khatru-sqlite,khatru-badger,relayer-basic,strfry,nostr-rs-relay,postgres}
chmod 777 data/{next-orly-badger,next-orly-dgraph,next-orly-neo4j,dgraph-zero,dgraph-alpha,neo4j,neo4j-logs,khatru-sqlite,khatru-badger,relayer-basic,strfry,nostr-rs-relay,postgres}

echo "Building fresh Docker images..."
# Force rebuild to pick up latest code changes
$DOCKER_COMPOSE build --no-cache benchmark-runner next-orly-badger next-orly-dgraph next-orly-neo4j

echo ""
echo "Starting benchmark suite..."
echo "This will automatically shut down all containers when the benchmark completes."
echo ""

# Run docker compose with flags to exit when benchmark-runner completes
$DOCKER_COMPOSE up --exit-code-from benchmark-runner --abort-on-container-exit

echo ""
echo "Benchmark suite has completed and all containers have been stopped."
echo "Check the ./reports/ directory for results."
