#!/bin/bash
#
# Comprehensive Negentropy Sync Test Suite
# Tests NIP-77 negentropy sync between strfry (client) and ORLY (server).
#
# Strfry has a built-in `sync` command that uses the negentropy protocol.
# ORLY serves NIP-77 via its embedded negentropy handler.
#
# This script runs from the HOST and uses `docker compose exec` to
# interact with containers.
#
# Scenarios tested:
#   1. Seed strfry with events
#   2. Strfry pushes events to ORLY (strfry --dir up)
#   3. Seed ORLY with new events
#   4. Strfry pulls events from ORLY (strfry --dir down)
#   5. Bidirectional sync
#   6. Final consistency verification
#
# Usage:
#   cd tests/negentropy
#   docker compose build
#   docker compose up -d
#   ./comprehensive-test.sh
#   docker compose down -v

set -euo pipefail

# Change to the directory containing docker-compose.yml
cd "$(dirname "$0")"

# Configuration
STRFRY_WS="ws://strfry:7777"
ORLY_WS="ws://orly-relay-1:3334"
SEED_COUNT=200
EXTRA_COUNT=100
VERBOSE="${VERBOSE:-false}"

# Test results
PASSED=0
FAILED=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass()  { echo -e "${GREEN}[PASS]${NC} $1"; PASSED=$((PASSED + 1)); }
log_fail()  { echo -e "${RED}[FAIL]${NC} $1"; FAILED=$((FAILED + 1)); }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_phase() { echo ""; echo "========================================"; echo -e "${YELLOW}PHASE: $1${NC}"; echo "========================================"; }

# Run a command in the test-runner container
run_test() {
    docker compose exec -T test-runner sh -c "$1"
}

# Run a command in the strfry container
run_strfry() {
    docker compose exec -T strfry sh -c "$1"
}

# Count events on a relay via WebSocket from test-runner.
# Sends a REQ, reads until EOSE, counts EVENT messages.
# Usage: count_events <ws_url> [filter_json]
count_events() {
    local url=$1
    local filter=${2:-'{}'}

    # IMPORTANT: We use { printf ...; sleep 60; } to keep stdin open.
    # Without this, websocat sends a close frame when stdin EOF is hit,
    # and the relay may not have sent all events yet.
    #
    # awk counts EVENT messages and exits on EOSE (breaking the pipe).
    # timeout is a safety net in case EOSE never arrives.
    local result
    result=$(run_test "{ printf '[\"REQ\",\"c\",%s]\n' '${filter}'; sleep 60; } | timeout 20 websocat '${url}' 2>/dev/null | awk 'BEGIN{c=0;f=0} /EOSE/{f=1; print c; exit} /EVENT/{c++} END{if(f==0) print c}'") || true

    # Trim whitespace; default to 0 if empty
    result=$(echo "${result}" | tr -d '[:space:]')
    echo "${result:-0}"
}

# Generate and send events to a relay
# Usage: generate_events <relay_ws_url> <count>
generate_events() {
    local url=$1
    local count=$2

    log_info "Generating $count events and sending to $url ..."
    run_test "event-generator -count $count -relay '$url' -batch 50" 2>&1 | while IFS= read -r line; do
        if [ "$VERBOSE" = "true" ]; then
            echo "  $line"
        fi
    done

    # Give the relay time to process
    sleep 3
}

# Wait for a relay to be healthy (via docker compose health check)
wait_for_services() {
    log_info "Checking service health..."

    local services=("strfry" "orly-relay-1" "test-runner")
    for svc in "${services[@]}"; do
        local status
        status=$(docker compose ps --format '{{.Health}}' "$svc" 2>/dev/null || echo "unknown")
        if [ "$status" = "healthy" ] || [ "$svc" = "test-runner" ]; then
            log_info "  $svc: ready"
        else
            log_warn "  $svc: $status (may not be ready)"
        fi
    done
}

# ============================================================
# Phase 1: Seed strfry with events
# ============================================================
phase1_seed_strfry() {
    log_phase "1. SEED STRFRY - Generate $SEED_COUNT events"

    generate_events "$STRFRY_WS" "$SEED_COUNT"

    local count
    count=$(count_events "$STRFRY_WS" '{"limit":10000}')
    log_info "Strfry has $count events"

    # Replaceable events (kind 0, 3, 10000, 10001) get deduplicated per pubkey,
    # so stored count is lower than sent count. With 3 test users and ~30%
    # replaceable kinds, expect roughly 70% stored.
    local min_expected=$((SEED_COUNT / 2))
    if [ "$count" -ge "$min_expected" ]; then
        log_pass "Strfry seeded with $count events (sent $SEED_COUNT, some replaceable)"
    else
        log_fail "Strfry only has $count events (expected >= $min_expected from $SEED_COUNT sent)"
    fi
}

# ============================================================
# Phase 2: Strfry pushes events to ORLY
# ============================================================
phase2_strfry_push_to_orly() {
    log_phase "2. STRFRY PUSH - Push events from strfry to ORLY"

    local orly_before
    orly_before=$(count_events "$ORLY_WS" '{"limit":10000}')
    log_info "ORLY has $orly_before events before sync"

    log_info "Running: strfry sync $ORLY_WS --dir up"
    run_strfry "/app/strfry --config=/etc/strfry.conf sync $ORLY_WS --dir up" 2>&1 || true

    sleep 5

    local orly_after
    orly_after=$(count_events "$ORLY_WS" '{"limit":10000}')
    log_info "ORLY has $orly_after events after sync (was $orly_before)"

    if [ "$orly_after" -gt "$orly_before" ]; then
        local synced=$((orly_after - orly_before))
        log_pass "Pushed $synced events from strfry to ORLY"
    else
        log_fail "No events pushed to ORLY (still $orly_after)"
    fi
}

# ============================================================
# Phase 3: Seed ORLY with new events
# ============================================================
phase3_seed_orly() {
    log_phase "3. SEED ORLY - Generate $EXTRA_COUNT new events on ORLY"

    local orly_before
    orly_before=$(count_events "$ORLY_WS" '{"limit":10000}')
    log_info "ORLY has $orly_before events before seeding"

    generate_events "$ORLY_WS" "$EXTRA_COUNT"

    local orly_after
    orly_after=$(count_events "$ORLY_WS" '{"limit":10000}')
    log_info "ORLY now has $orly_after events (was $orly_before)"

    if [ "$orly_after" -gt "$orly_before" ]; then
        local added=$((orly_after - orly_before))
        log_pass "ORLY stored $added new events ($orly_after total)"
    else
        log_fail "ORLY count didn't increase (still $orly_after)"
    fi
}

# ============================================================
# Phase 4: Strfry pulls new events from ORLY
# ============================================================
phase4_strfry_pull_from_orly() {
    log_phase "4. STRFRY PULL - Pull new events from ORLY to strfry"

    local strfry_before
    strfry_before=$(count_events "$STRFRY_WS" '{"limit":10000}')
    log_info "Strfry has $strfry_before events before sync"

    log_info "Running: strfry sync $ORLY_WS --dir down"
    run_strfry "/app/strfry --config=/etc/strfry.conf sync $ORLY_WS --dir down" 2>&1 || true

    sleep 5

    local strfry_after
    strfry_after=$(count_events "$STRFRY_WS" '{"limit":10000}')
    log_info "Strfry has $strfry_after events after sync (was $strfry_before)"

    if [ "$strfry_after" -gt "$strfry_before" ]; then
        local synced=$((strfry_after - strfry_before))
        log_pass "Pulled $synced events from ORLY to strfry"
    else
        log_fail "No new events pulled to strfry (still $strfry_after)"
    fi
}

# ============================================================
# Phase 5: Bidirectional sync
# ============================================================
phase5_bidirectional() {
    log_phase "5. BIDIRECTIONAL - Sync both directions"

    # Add unique events to both sides
    log_info "Adding 50 events to strfry..."
    generate_events "$STRFRY_WS" 50

    log_info "Adding 50 events to ORLY..."
    generate_events "$ORLY_WS" 50

    local strfry_before orly_before
    strfry_before=$(count_events "$STRFRY_WS" '{"limit":10000}')
    orly_before=$(count_events "$ORLY_WS" '{"limit":10000}')

    log_info "Before bidirectional sync: strfry=$strfry_before, ORLY=$orly_before"

    log_info "Running: strfry sync $ORLY_WS --dir both"
    run_strfry "/app/strfry --config=/etc/strfry.conf sync $ORLY_WS --dir both" 2>&1 || true

    sleep 5

    local strfry_after orly_after
    strfry_after=$(count_events "$STRFRY_WS" '{"limit":10000}')
    orly_after=$(count_events "$ORLY_WS" '{"limit":10000}')

    log_info "After bidirectional sync: strfry=$strfry_after, ORLY=$orly_after"

    local diff=$((strfry_after - orly_after))
    if [ "${diff#-}" -le 50 ]; then
        log_pass "Bidirectional sync achieved consistency (diff: $diff)"
    else
        log_fail "Event counts still differ significantly (diff: $diff)"
    fi
}

# ============================================================
# Phase 6: Final verification
# ============================================================
phase6_final_verification() {
    log_phase "6. FINAL VERIFICATION"

    local strfry_total orly_total
    strfry_total=$(count_events "$STRFRY_WS" '{"limit":10000}')
    orly_total=$(count_events "$ORLY_WS" '{"limit":10000}')

    log_info "Final event counts:"
    log_info "  strfry:       $strfry_total"
    log_info "  orly-relay-1: $orly_total"

    # Both should have a reasonable number of events
    if [ "$strfry_total" -gt 0 ] && [ "$orly_total" -gt 0 ]; then
        log_pass "Both relays have events (strfry=$strfry_total, ORLY=$orly_total)"
    else
        log_fail "One or both relays are empty"
    fi

    # Check consistency
    local diff=$((strfry_total - orly_total))
    if [ "${diff#-}" -le 50 ]; then
        log_pass "Relays are consistent (diff: $diff)"
    else
        log_warn "Relays differ by $diff events"
    fi
}

# ============================================================
# Main
# ============================================================
main() {
    echo "========================================"
    echo "Negentropy (NIP-77) Interop Test Suite"
    echo "strfry (client) <-> ORLY (server)"
    echo "========================================"
    echo ""
    echo "Config:"
    echo "  Seed events: $SEED_COUNT"
    echo "  Extra events: $EXTRA_COUNT"
    echo ""

    wait_for_services

    phase1_seed_strfry
    phase2_strfry_push_to_orly
    phase3_seed_orly
    phase4_strfry_pull_from_orly
    phase5_bidirectional
    phase6_final_verification

    echo ""
    echo "========================================"
    echo "TEST SUMMARY"
    echo "========================================"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""

    if [ "$FAILED" -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        exit 1
    fi
}

case "${1:-}" in
    --verbose|-v)
        VERBOSE=true
        main
        ;;
    --help|-h)
        echo "Usage: $0 [--verbose|-v] [--help|-h]"
        echo ""
        echo "Run from the tests/negentropy directory with containers up:"
        echo "  docker compose build"
        echo "  docker compose up -d"
        echo "  $0"
        echo "  docker compose down -v"
        exit 0
        ;;
    *)
        main
        ;;
esac
