#!/bin/bash
# Run Badger benchmark with reduced cache sizes to avoid OOM

# Set reasonable cache sizes for benchmark
export ORLY_DB_BLOCK_CACHE_MB=256   # Reduced from 1024MB
export ORLY_DB_INDEX_CACHE_MB=128   # Reduced from 512MB
export ORLY_QUERY_CACHE_SIZE_MB=128 # Reduced from 512MB

# Clean up old data
rm -rf /tmp/benchmark_db_badger

echo "Running Badger benchmark with reduced cache sizes:"
echo "  Block Cache: ${ORLY_DB_BLOCK_CACHE_MB}MB"
echo "  Index Cache: ${ORLY_DB_INDEX_CACHE_MB}MB"
echo "  Query Cache: ${ORLY_QUERY_CACHE_SIZE_MB}MB"
echo ""

# Run benchmark
./benchmark -events "${1:-1000}" -workers "${2:-4}" -datadir /tmp/benchmark_db_badger
