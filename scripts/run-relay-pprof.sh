#!/usr/bin/env bash
set -euo pipefail

# Run the relay with CPU profiling enabled, wait 60s, then open the
# generated profile using `go tool pprof` web UI.
#
# Notes:
# - Builds a temporary relay binary in /tmp and deletes it on exit.
# - Uses the exact env requested, plus ORLY_PPROF=cpu and a deterministic
#   ORLY_PPROF_PATH inside a temp dir.
# - Profiles for DURATION seconds (default 60).
# - Launches pprof web UI at http://localhost:8000 and attempts to open browser.

DURATION="${DURATION:-60}"
HEALTH_PORT="${HEALTH_PORT:-18081}"
ROOT_DIR="/home/mleku/src/git.smesh.lol/orly"
LISTEN_HOST="${LISTEN_HOST:-10.0.0.2}"

cd "$ROOT_DIR"

# Refresh embedded web assets
reset || true
./scripts/update-embedded-web.sh || true

TMP_DIR="$(mktemp -d -t orly-pprof-XXXXXX)"
BIN_PATH="$TMP_DIR/git.smesh.lol/orly"
LOG_FILE="$TMP_DIR/relay.log"
PPROF_FILE=""
RELAY_PID=""
PPROF_DIR="$TMP_DIR/profiles"
mkdir -p "$PPROF_DIR"

cleanup() {
  # Try to stop relay if still running
  if [[ -n "${RELAY_PID}" ]] && kill -0 "${RELAY_PID}" 2>/dev/null; then
    kill "${RELAY_PID}" || true
    wait "${RELAY_PID}" || true
  fi
  rm -f "$BIN_PATH" 2>/dev/null || true
  rm -rf "$TMP_DIR" 2>/dev/null || true
}
trap cleanup EXIT

echo "[run-relay-pprof] Building relay binary ..."
GOFLAGS="${GOFLAGS:-}" go build -o "$BIN_PATH" .

echo "[run-relay-pprof] Starting relay with CPU profiling ..."
(
  ORLY_LOG_LEVEL=debug \
  ORLY_LISTEN="$LISTEN_HOST" \
  ORLY_PORT=3334 \
  ORLY_ADMINS=npub1fjqqy4a93z5zsjwsfxqhc2764kvykfdyttvldkkkdera8dr78vhsmmleku \
  ORLY_ACL_MODE=follows \
  ORLY_RELAY_ADDRESSES=test.orly.dev \
  ORLY_IP_BLACKLIST=192.71.213.188 \
  ORLY_HEALTH_PORT="$HEALTH_PORT" \
  ORLY_ENABLE_SHUTDOWN=true \
  ORLY_PPROF_HTTP=true \
  ORLY_OPEN_PPROF_WEB=true \
  "$BIN_PATH"
) >"$LOG_FILE" 2>&1 &
RELAY_PID=$!

# Wait for pprof HTTP server readiness
PPROF_BASE="http://${LISTEN_HOST}:6060"
echo "[run-relay-pprof] Waiting for pprof at ${PPROF_BASE} ..."
for i in {1..100}; do
  if curl -fsS "${PPROF_BASE}/debug/pprof/" -o /dev/null 2>/dev/null; then
    READY=1
    break
  fi
  sleep 0.2
done
if [[ -z "${READY:-}" ]]; then
  echo "[run-relay-pprof] ERROR: pprof HTTP server not reachable at ${PPROF_BASE}." >&2
  echo "[run-relay-pprof]       Check that ${LISTEN_HOST} is a local bindable address." >&2
  # Attempt to dump recent logs for context
  tail -n 100 "$LOG_FILE" || true
  # Try INT to clean up
  killall -INT git.smesh.lol/orly 2>/dev/null || true
  exit 1
fi

# Open the HTTP pprof UI
( xdg-open "${PPROF_BASE}/debug/pprof/" >/dev/null 2>&1 || true ) &

echo "[run-relay-pprof] Collecting CPU profile via HTTP for ${DURATION}s ..."
# The HTTP /debug/pprof/profile endpoint records CPU for the provided seconds
# and returns a pprof file without needing to stop the process.
curl -fsS --max-time $((DURATION+10)) \
  "${PPROF_BASE}/debug/pprof/profile?seconds=${DURATION}" \
  -o "$PPROF_DIR/cpu.pprof" || true

echo "[run-relay-pprof] Sending SIGINT (Ctrl+C) for graceful shutdown ..."
killall -INT git.smesh.lol/orly 2>/dev/null || true

# Wait up to ~60s for graceful shutdown so defers (pprof Stop) can run
for i in {1..300}; do
  if ! pgrep -x git.smesh.lol/orly >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

# Try HTTP shutdown if still running (ensures defer paths can run)
if pgrep -x git.smesh.lol/orly >/dev/null 2>&1; then
  echo "[run-relay-pprof] Still running, requesting /shutdown ..."
  curl -fsS --max-time 2 "http://10.0.0.2:${HEALTH_PORT}/shutdown" >/dev/null 2>&1 || true
  for i in {1..150}; do
    if ! pgrep -x git.smesh.lol/orly >/dev/null 2>&1; then
      break
    fi
    sleep 0.2
  done
fi
if pgrep -x git.smesh.lol/orly >/dev/null 2>&1; then
  echo "[run-relay-pprof] Escalating: sending SIGTERM ..."
  killall -TERM git.smesh.lol/orly 2>/dev/null || true
  for i in {1..150}; do
    if ! pgrep -x git.smesh.lol/orly >/dev/null 2>&1; then
      break
    fi
    sleep 0.2
  done
fi
if pgrep -x git.smesh.lol/orly >/dev/null 2>&1; then
  echo "[run-relay-pprof] Force kill: sending SIGKILL ..."
  killall -KILL git.smesh.lol/orly 2>/dev/null || true
fi

PPROF_FILE="$PPROF_DIR/cpu.pprof"
if [[ ! -s "$PPROF_FILE" ]]; then
  echo "[run-relay-pprof] ERROR: HTTP CPU profile not captured (file empty)." >&2
  echo "[run-relay-pprof] Hint: Ensure ORLY_PPROF_HTTP=true and port 6060 is reachable." >&2
  exit 1
fi

echo "[run-relay-pprof] Detected profile file: $PPROF_FILE"
echo "[run-relay-pprof] Launching 'go tool pprof' web UI on :8000 ..."

# Try to open a browser automatically, ignore failures
( sleep 0.6; xdg-open "http://localhost:8000" >/dev/null 2>&1 || true ) &

exec go tool pprof -http=:8000 "$BIN_PATH" "$PPROF_FILE"


