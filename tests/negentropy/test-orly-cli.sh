#!/bin/bash
#
# Test orly CLI sync against strfry relay
#
# This mimics the exact workflow David described, but using orly:
#   orly sync wss://relay.example.com --filter '{"kinds": [0, 3, 1984, 10000, 30000]}' --dir down
#
# Usage:
#   docker compose up -d
#   ./test-orly-cli.sh
#

set -e

STRFRY_HOST="${STRFRY_HOST:-localhost}"
STRFRY_PORT="${STRFRY_PORT:-7777}"
ORLY_CONTAINER="${ORLY_CONTAINER:-negentropy-sync-runner-1}"

# Test filter (same kinds David uses for Brainstorm)
FILTER='{"kinds": [0, 3, 1984, 10000, 30000]}'

echo "========================================"
echo "orly CLI sync test against strfry"
echo "========================================"
echo ""
echo "Target: ws://strfry:7777"
echo "Filter: $FILTER"
echo ""

# Run orly sync command
echo "Running: orly sync ws://strfry:7777 --filter '$FILTER' --dir down"
echo ""

docker exec -it "$ORLY_CONTAINER" /app/orly sync ws://strfry:7777 --filter "$FILTER" --dir down --verbose

echo ""
echo "Sync complete!"
