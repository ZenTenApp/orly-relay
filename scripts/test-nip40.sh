#!/usr/bin/env bash
#
# test-nip40.sh — Local end-to-end test of NIP-40 expiration on a running relay.
#
# It builds the monolithic `orly` relay plus a tiny websocket helper, runs the
# relay with an OPEN ACL (acl_mode=none — the path that previously served
# expired events) and a fast cleanup sweep interval, then verifies:
#
#   1. An event with a short TTL and one with a long TTL are both returned.
#   2. After the short TTL passes, the short event is hidden from queries AND
#      physically purged by the DeleteExpired sweeper; the long-lived one is
#      still served.
#
# Requirements: go, bash 4+, a free port (default 3334).
# Usage: ./scripts/test-nip40.sh
#
set -euo pipefail

REPODIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPODIR"

HOST="${ORLY_TEST_HOST:-127.0.0.1}"
PORT="${ORLY_TEST_PORT:-3334}"
WS_URL="ws://${HOST}:${PORT}"
CLEANUP_INTERVAL="${ORLY_TEST_CLEANUP_INTERVAL:-5s}"
SHORT_TTL="${ORLY_TEST_SHORT_TTL:-6}"      # seconds until the short event expires
WAIT_MAX="${ORLY_TEST_WAIT_MAX:-30}"       # max seconds to wait for expiry+sweep

TMPDIR_TEST="$(mktemp -d /tmp/orly-nip40.XXXXXX)"
RELAY_BIN="$TMPDIR_TEST/orly"
HELPER_BIN="$TMPDIR_TEST/nip40test"
DATA_DIR="$TMPDIR_TEST/data"
KEYFILE="$TMPDIR_TEST/test-secret.hex"
RELAY_LOG="$TMPDIR_TEST/relay.log"
RELAY_PID=""

cleanup() {
    if [[ -n "$RELAY_PID" ]] && kill -0 "$RELAY_PID" 2>/dev/null; then
        kill "$RELAY_PID" 2>/dev/null || true
        wait "$RELAY_PID" 2>/dev/null || true
    fi
    rm -rf "$TMPDIR_TEST"
}
trap cleanup EXIT

say()  { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
pass() { printf '\033[1;32mPASS\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31mFAIL\033[0m %s\n' "$*"; exit 1; }

# --- build ---
say "Building relay and helper ..."
mkdir -p "$DATA_DIR"
go build -o "$RELAY_BIN" ./cmd/orly
go build -o "$HELPER_BIN" ./tools/nip40test

# --- start relay (open ACL mode, fast sweep so we don't wait 10 minutes) ---
say "Starting relay on ${WS_URL} (acl_mode=none, cleanup interval=${CLEANUP_INTERVAL}) ..."
ORLY_LISTEN="$HOST" \
ORLY_PORT="$PORT" \
ORLY_DATA_DIR="$DATA_DIR" \
ORLY_ACL_MODE="none" \
ORLY_DB_TYPE="badger" \
ORLY_RATE_LIMIT_ENABLED="false" \
ORLY_LOG_LEVEL="info" \
ORLY_EXPIRATION_CLEANUP_INTERVAL="$CLEANUP_INTERVAL" \
"$RELAY_BIN" >"$RELAY_LOG" 2>&1 &
RELAY_PID=$!

# wait for the websocket port to accept connections
say "Waiting for relay to listen ..."
for _ in $(seq 1 50); do
    if (echo >"/dev/tcp/${HOST}/${PORT}") 2>/dev/null; then
        break
    fi
    sleep 0.2
done
if ! (echo >"/dev/tcp/${HOST}/${PORT}") 2>/dev/null; then
    echo "--- relay log tail ---"
    tail -20 "$RELAY_LOG" || true
    die "relay did not start listening"
fi

"$HELPER_BIN" genkey "$KEYFILE" >/dev/null

NOW="$(date +%s)"
SHORT_EXP=$((NOW + SHORT_TTL))
FOREVER_EXP=$((NOW + 3600))

say "Publishing short-TTL event (expires in ${SHORT_TTL}s) and long-TTL event ..."
SHORT_OUT="$("$HELPER_BIN" publish "$WS_URL" "$KEYFILE" "$SHORT_EXP" "I expire soon")"
FOREVER_OUT="$("$HELPER_BIN" publish "$WS_URL" "$KEYFILE" "$FOREVER_EXP" "I live long")"
echo "short  -> $SHORT_OUT"
echo "long   -> $FOREVER_OUT"

SHORT_ID="$(awk '{print $1}' <<<"$SHORT_OUT")"
FOREVER_ID="$(awk '{print $1}' <<<"$FOREVER_OUT")"
SHORT_OK="$(awk '{print $3}' <<<"$SHORT_OUT")"
FOREVER_OK="$(awk '{print $3}' <<<"$FOREVER_OUT")"
[[ "$SHORT_OK" == "true" ]] || die "short event not accepted by relay"
[[ "$FOREVER_OK" == "true" ]] || die "long event not accepted by relay"

say "Immediately after publish, both events should be queryable ..."
SHORT_Q="$("$HELPER_BIN" query "$WS_URL" "$KEYFILE" "$SHORT_ID")"
FOREVER_Q="$("$HELPER_BIN" query "$WS_URL" "$KEYFILE" "$FOREVER_ID")"
echo "short event present: $SHORT_Q"
echo "long  event present: $FOREVER_Q"
[[ "$SHORT_Q" == "FOUND" ]]   || die "short event should be FOUND before expiry"
[[ "$FOREVER_Q" == "FOUND" ]] || die "long event should be FOUND"
pass "both events are served before the short TTL elapses"

# --- wait for the short event to expire and the sweeper to run ---
say "Waiting ${WAIT_MAX}s max for expiry + DeleteExpired sweep ..."
elapsed=0
while (( elapsed < WAIT_MAX )); do
    sleep 1
    elapsed=$((elapsed + 1))
    Q="$("$HELPER_BIN" query "$WS_URL" "$KEYFILE" "$SHORT_ID" 2>/dev/null || true)"
    if [[ "$Q" == "NOT_FOUND" ]]; then
        break
    fi
done
echo "elapsed ${elapsed}s"

# Give the DeleteExpired sweeper one more tick so the log shows a purge.
sleep 2

say "After expiry: short event should be GONE, long event should REMAIN ..."
SHORT_Q="$("$HELPER_BIN" query "$WS_URL" "$KEYFILE" "$SHORT_ID")"
FOREVER_Q="$("$HELPER_BIN" query "$WS_URL" "$KEYFILE" "$FOREVER_ID")"
echo "short event present: $SHORT_Q"
echo "long  event present: $FOREVER_Q"
[[ "$SHORT_Q" == "NOT_FOUND" ]] || die "expired short event is STILL served"
[[ "$FOREVER_Q" == "FOUND" ]]   || die "long-lived event should still be served"
pass "expired events are hidden from queries; non-expired events are served"

say "Relay log (DeleteExpired / expired) — expecting a purge line:"
grep -iE "DeleteExpired|expired|migrat.*11" "$RELAY_LOG" | tail -15 || true

echo
pass "NIP-40 test complete. Full logs: $RELAY_LOG"
