#!/usr/bin/env bash
set -euo pipefail

# Runs the ORLY relay with CPU profiling enabled and opens the resulting
# pprof profile in a local web UI.
#
# Usage:
#   ./profile.sh [duration_seconds]
#
# - Builds the relay.
# - Starts it with ORLY_PPROF=cpu and minimal logging.
# - Waits for the profile path printed at startup.
# - Runs for DURATION seconds (default 10), then stops the relay to flush the
#   CPU profile to disk.
# - Launches `go tool pprof -http=:8000` for convenient browsing.
#
# Notes:
# - The profile file path is detected from the relay's stdout/stderr lines
#   emitted by github.com/pkg/profile, typically like:
#     profile: cpu profiling enabled, path: /tmp/profile123456/cpu.pprof
# - You can change DURATION by passing a number of seconds as the first arg
#   or by setting DURATION env var.

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
cd "$REPO_ROOT"

DURATION="${1:-${DURATION:-10}}"
PPROF_HTTP_PORT="${PPROF_HTTP_PORT:-8000}"

# Load generation controls
LOAD_ENABLED="${LOAD_ENABLED:-1}"               # set to 0 to disable load
# Use the benchmark main package in cmd/benchmark as the load generator
BENCHMARK_PKG_DIR="$REPO_ROOT/cmd/benchmark"
BENCHMARK_BIN="${BENCHMARK_BIN:-}"            # if empty, we will build to $RUN_DIR/benchmark
BENCHMARK_EVENTS="${BENCHMARK_EVENTS:-}"      # optional override for -events
BENCHMARK_DURATION="${BENCHMARK_DURATION:-}"  # optional override for -duration (e.g. 30s); defaults to DURATION seconds

BIN="$REPO_ROOT/git.smesh.lol/orly"
LOG_DIR="${LOG_DIR:-$REPO_ROOT/cmd/benchmark/reports}"
mkdir -p "$LOG_DIR"
RUN_TS="$(date +%Y%m%d_%H%M%S)"
RUN_DIR="$LOG_DIR/profile_run_${RUN_TS}"
mkdir -p "$RUN_DIR"
LOG_FILE="$RUN_DIR/relay.log"
LOAD_LOG_FILE="$RUN_DIR/load.log"

echo "[profile.sh] Building relay binary (pure Go + purego)..."
CGO_ENABLED=0 go build -o "$BIN" .

# Copy libsecp256k1.so next to binary if available (runtime optional)
if [[ -f "pkg/crypto/p8k/libsecp256k1.so" ]]; then
    cp pkg/crypto/p8k/libsecp256k1.so "$(dirname "$BIN")/"
fi

# Ensure we clean up the child process on exit
RELAY_PID=""
LOAD_PID=""
cleanup() {
  if [[ -n "$LOAD_PID" ]] && kill -0 "$LOAD_PID" 2>/dev/null; then
    echo "[profile.sh] Stopping load generator (pid=$LOAD_PID) ..."
    kill -INT "$LOAD_PID" 2>/dev/null || true
    sleep 0.5
    kill -TERM "$LOAD_PID" 2>/dev/null || true
  fi
  if [[ -n "$RELAY_PID" ]] && kill -0 "$RELAY_PID" 2>/dev/null; then
    echo "[profile.sh] Stopping relay (pid=$RELAY_PID) ..."
    kill -INT "$RELAY_PID" 2>/dev/null || true
    # give it a moment to exit and flush profile
    sleep 1
    kill -TERM "$RELAY_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# Start the relay with CPU profiling enabled. Capture both stdout and stderr.
echo "[profile.sh] Starting relay with CPU profiling enabled ..."
(
  ORLY_LOG_LEVEL=off \
  ORLY_LISTEN="${ORLY_LISTEN:-127.0.0.1}" \
  ORLY_PORT="${ORLY_PORT:-3334}" \
  ORLY_PPROF=cpu \
  "$BIN"
) >"$LOG_FILE" 2>&1 &
RELAY_PID=$!
echo "[profile.sh] Relay started with pid $RELAY_PID; logging to $LOG_FILE"

# Wait until the profile path is printed. Timeout after reasonable period.
PPROF_FILE=""
START_TIME=$(date +%s)
TIMEOUT=30

echo "[profile.sh] Waiting for profile path to appear in relay output ..."
while :; do
  if grep -Eo "/tmp/profile[^ ]+/cpu\.pprof" "$LOG_FILE" >/dev/null 2>&1; then
    PPROF_FILE=$(grep -Eo "/tmp/profile[^ ]+/cpu\.pprof" "$LOG_FILE" | tail -n1)
    break
  fi
  NOW=$(date +%s)
  if (( NOW - START_TIME > TIMEOUT )); then
    echo "[profile.sh] ERROR: Timed out waiting for profile path in $LOG_FILE" >&2
    echo "Last 50 log lines:" >&2
    tail -n 50 "$LOG_FILE" >&2
    exit 1
  fi
  sleep 0.3
done

echo "[profile.sh] Detected profile file: $PPROF_FILE"

# Optionally start load generator to exercise the relay
if [[ "$LOAD_ENABLED" == "1" ]]; then
  # Build benchmark binary if not provided
  if [[ -z "$BENCHMARK_BIN" ]]; then
    BENCHMARK_BIN="$RUN_DIR/benchmark"
    echo "[profile.sh] Building benchmark load generator (pure Go + purego) ($BENCHMARK_PKG_DIR) ..."
    cd "$BENCHMARK_PKG_DIR"
    CGO_ENABLED=0 go build -o "$BENCHMARK_BIN" .
    # Copy libsecp256k1.so if available (runtime optional)
    if [[ -f "$REPO_ROOT/pkg/crypto/p8k/libsecp256k1.so" ]]; then
        cp "$REPO_ROOT/pkg/crypto/p8k/libsecp256k1.so" "$RUN_DIR/"
    fi
    cd "$REPO_ROOT"
  fi
  BENCH_DB_DIR="$RUN_DIR/benchdb"
  mkdir -p "$BENCH_DB_DIR"
  DURATION_ARG="${BENCHMARK_DURATION:-${DURATION}s}"
  EXTRA_EVENTS=""
  if [[ -n "$BENCHMARK_EVENTS" ]]; then
    EXTRA_EVENTS="-events=$BENCHMARK_EVENTS"
  fi
  echo "[profile.sh] Starting benchmark load generator for duration $DURATION_ARG ..."
  RELAY_URL="ws://${ORLY_LISTEN:-127.0.0.1}:${ORLY_PORT:-3334}"
  echo "[profile.sh] Using relay URL: $RELAY_URL"
  (
    "$BENCHMARK_BIN" -relay-url="$RELAY_URL" -net-workers="${NET_WORKERS:-2}" -net-rate="${NET_RATE:-20}" -duration="$DURATION_ARG" $EXTRA_EVENTS \
      >"$LOAD_LOG_FILE" 2>&1 &
  )
  LOAD_PID=$!
  echo "[profile.sh] Load generator started (pid=$LOAD_PID); logging to $LOAD_LOG_FILE"
else
  echo "[profile.sh] LOAD_ENABLED=0; not starting load generator."
fi

echo "[profile.sh] Letting the relay run for ${DURATION}s to collect CPU samples ..."
sleep "$DURATION"

# Stop the relay to flush the CPU profile
cleanup
# Disable trap so we don't double-kill
trap - EXIT

# Wait briefly to ensure the profile file is finalized
for i in {1..20}; do
  if [[ -s "$PPROF_FILE" ]]; then
    break
  fi
  sleep 0.2
done

if [[ ! -s "$PPROF_FILE" ]]; then
  echo "[profile.sh] WARNING: Profile file exists but is empty or missing: $PPROF_FILE" >&2
fi

# Launch pprof HTTP UI
echo "[profile.sh] Launching pprof web UI (http://localhost:${PPROF_HTTP_PORT}) ..."
exec go tool pprof -http=":${PPROF_HTTP_PORT}" "$BIN" "$PPROF_FILE"
