#!/bin/bash
#
# Test strfry CLI sync against orly relay
#
# This mimics the exact workflow David described:
#   strfry sync wss://relay.orly.dev --filter '{"kinds": [0, 3, 1984, 10000, 30000]}' --dir down
#
# Usage:
#   docker compose up -d
#   ./test-strfry-cli.sh
#

set -e

ORLY_HOST="${ORLY_HOST:-localhost}"
ORLY_PORT="${ORLY_PORT:-3334}"
STRFRY_CONTAINER="${STRFRY_CONTAINER:-negentropy-strfry-1}"

# Test filter (same kinds David uses for Brainstorm)
FILTER='{"kinds": [0, 3, 1984, 10000, 30000]}'

echo "========================================"
echo "strfry CLI sync test against orly"
echo "========================================"
echo ""
echo "Target: ws://${ORLY_HOST}:${ORLY_PORT}"
echo "Filter: $FILTER"
echo ""

# Run strfry sync command
echo "Running: strfry sync ws://orly:3334 --filter '$FILTER' --dir down"
echo ""

docker exec -it "$STRFRY_CONTAINER" /app/strfry sync ws://orly:3334 --filter "$FILTER" --dir down

echo ""
echo "Sync complete!"
