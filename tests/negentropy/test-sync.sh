#!/bin/bash
#
# Negentropy Interop Test Script
# Tests NIP-77 negentropy sync between strfry and orly
#
# Usage:
#   ./test-sync.sh
#
# Prerequisites:
#   - docker compose up -d (containers running)
#   - websocat and jq installed (for event generation)
#

set -e

STRFRY_URL="ws://localhost:7777"
ORLY_URL="ws://localhost:3334"
TEST_EVENTS=10
PASSED=0
FAILED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED++))
}

# Generate a test private key (for signing events)
# This is a fixed test key - DO NOT use in production
TEST_PRIVKEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
TEST_PUBKEY="a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1"

# Check if services are running
check_services() {
    log_info "Checking if services are running..."

    if ! curl -s http://localhost:7777 > /dev/null 2>&1; then
        log_fail "strfry is not running on port 7777"
        exit 1
    fi
    log_pass "strfry is running"

    if ! curl -s http://localhost:3334 > /dev/null 2>&1; then
        log_fail "orly is not running on port 3334"
        exit 1
    fi
    log_pass "orly is running"
}

# Generate and send a test event to a relay
# Uses websocat if available, otherwise uses netcat
send_event() {
    local relay_url=$1
    local kind=$2
    local content=$3
    local timestamp=$(date +%s)

    # Create a simple unsigned event (strfry will accept it in test mode)
    # For real testing, we'd need proper signing
    local event_json=$(cat <<EOF
["EVENT", {
    "id": "$(echo -n "${timestamp}${content}" | sha256sum | cut -d' ' -f1)",
    "pubkey": "${TEST_PUBKEY}",
    "created_at": ${timestamp},
    "kind": ${kind},
    "tags": [],
    "content": "${content}",
    "sig": "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
}]
EOF
)

    echo "$event_json" | websocat -n1 "$relay_url" 2>/dev/null || true
}

# Count events on a relay matching a filter
count_events() {
    local relay_url=$1
    local filter=$2

    # Send REQ and count EVENT responses before EOSE
    local count=$(echo "[\"REQ\", \"count\", $filter]" | \
        timeout 5 websocat -n "$relay_url" 2>/dev/null | \
        grep -c '"EVENT"' || echo "0")

    echo "$count"
}

# Test 1: Seed strfry with events
test_seed_strfry() {
    log_info "Test 1: Seeding strfry with $TEST_EVENTS test events..."

    for i in $(seq 1 $TEST_EVENTS); do
        send_event "$STRFRY_URL" 1 "Test event $i from strfry seed"
        sleep 0.1
    done

    sleep 2  # Wait for events to be processed

    local count=$(count_events "$STRFRY_URL" '{"kinds": [1], "limit": 100}')
    log_info "strfry event count: $count"

    if [ "$count" -ge "$TEST_EVENTS" ]; then
        log_pass "strfry seeded with $count events"
    else
        log_fail "strfry only has $count events (expected >= $TEST_EVENTS)"
    fi
}

# Test 2: Run orly sync to pull from strfry
test_orly_sync_from_strfry() {
    log_info "Test 2: Running orly sync from strfry (strfry -> orly)..."

    # Use the sync-runner container to run sync
    docker compose exec -T sync-runner /app/orly sync ws://strfry:7777 --filter '{"kinds": [1]}' --dir down -v

    sleep 3  # Wait for sync to complete

    local orly_count=$(count_events "$ORLY_URL" '{"kinds": [1], "limit": 100}')
    log_info "orly event count after sync: $orly_count"

    if [ "$orly_count" -ge "$TEST_EVENTS" ]; then
        log_pass "orly synced $orly_count events from strfry"
    else
        log_fail "orly only synced $orly_count events (expected >= $TEST_EVENTS)"
    fi
}

# Test 3: Seed orly with new events and sync back to strfry
test_orly_sync_to_strfry() {
    log_info "Test 3: Seeding orly and syncing back to strfry (orly -> strfry)..."

    local new_events=5
    for i in $(seq 1 $new_events); do
        send_event "$ORLY_URL" 1 "Test event $i from orly for reverse sync"
        sleep 0.1
    done

    sleep 2

    # Check orly has the new events
    local orly_count=$(count_events "$ORLY_URL" '{"kinds": [1], "limit": 100}')
    log_info "orly event count: $orly_count"

    # Run strfry sync from orly (using strfry's sync command)
    # Note: strfry sync command format is: strfry sync <url> --filter <filter> --dir down
    docker compose exec -T strfry /app/strfry sync ws://orly:3334 --filter '{"kinds": [1]}' --dir down || true

    sleep 3

    local strfry_count=$(count_events "$STRFRY_URL" '{"kinds": [1], "limit": 100}')
    log_info "strfry event count after reverse sync: $strfry_count"

    local expected=$((TEST_EVENTS + new_events))
    if [ "$strfry_count" -ge "$expected" ]; then
        log_pass "strfry has $strfry_count events after bidirectional sync"
    else
        log_fail "strfry only has $strfry_count events (expected >= $expected)"
    fi
}

# Test 4: Verify event consistency
test_event_consistency() {
    log_info "Test 4: Verifying event consistency between relays..."

    local strfry_count=$(count_events "$STRFRY_URL" '{"kinds": [1], "limit": 100}')
    local orly_count=$(count_events "$ORLY_URL" '{"kinds": [1], "limit": 100}')

    log_info "strfry: $strfry_count events, orly: $orly_count events"

    # After bidirectional sync, both should have similar counts
    local diff=$((strfry_count - orly_count))
    if [ "${diff#-}" -le 2 ]; then  # Allow small difference
        log_pass "Event counts are consistent (diff: $diff)"
    else
        log_fail "Event counts differ by $diff"
    fi
}

# Run all tests
main() {
    echo "========================================"
    echo "Negentropy Interop Test Suite"
    echo "strfry <-> orly (NIP-77)"
    echo "========================================"
    echo

    check_services
    echo

    test_seed_strfry
    echo

    test_orly_sync_from_strfry
    echo

    test_orly_sync_to_strfry
    echo

    test_event_consistency
    echo

    echo "========================================"
    echo "Results: $PASSED passed, $FAILED failed"
    echo "========================================"

    if [ "$FAILED" -gt 0 ]; then
        exit 1
    fi
}

main "$@"
