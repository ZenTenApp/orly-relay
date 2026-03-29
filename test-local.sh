#!/bin/sh
# Local end-to-end test: relay + smesh3 + bridge + mailpit
# Usage: ./test-local.sh [--latency MS] [--jitter MS]
# Then open: http://127.0.0.1:8090 (smesh) and http://127.0.0.1:8025 (mailpit)
set -e
cd "$(dirname "$0")"

DATADIR=/tmp/orly-test
BRIDGEDIR="$HOME/.config/orly-bridge-test"
mkdir -p "$DATADIR" "$BRIDGEDIR"

# Parse --latency and --jitter
LATENCY=0
JITTER=0
while [ $# -gt 0 ]; do
  case "$1" in
    --latency) LATENCY="$2"; shift 2 ;;
    --jitter) JITTER="$2"; shift 2 ;;
    *) shift ;;
  esac
done

PID_PROXY=""
cleanup() {
  echo "shutting down..."
  kill $PID_ORLY $PID_BRIDGE $PID_MAILPIT $PID_PROXY 2>/dev/null
  wait 2>/dev/null
}
trap cleanup EXIT INT TERM

# 1. mailpit — catches all outbound email (web UI on :8025, SMTP on :25)
echo "starting mailpit on :25 (SMTP) and :8025 (web UI)..."
sudo "$HOME/.local/bin/mailpit" --smtp 127.0.0.1:25 --listen 127.0.0.1:8025 &
PID_MAILPIT=$!
sleep 1

# 2. orly — relay on :3334, smesh3 on :8090
echo "starting orly (relay :3334, smesh3 :8090)..."
ORLY_DATA_DIR="$DATADIR" \
ORLY_LISTEN=127.0.0.1 \
ORLY_SMESH3_DIR="$PWD/app/smesh3" \
ORLY_SMESH3_PORT=8090 \
ORLY_LOG_LEVEL=info \
  /tmp/orly-local &
PID_ORLY=$!
sleep 2

# 2b. latency proxy (optional)
BRIDGE_RELAY_PORT=3334
if [ "$LATENCY" -gt 0 ] 2>/dev/null; then
  echo "starting latency proxy on :3335 -> :3334 (${LATENCY}ms +${JITTER}ms jitter)..."
  python3 test/latency_proxy.py --listen 127.0.0.1:3335 --target 127.0.0.1:3334 \
    --latency "$LATENCY" --jitter "$JITTER" &
  PID_PROXY=$!
  sleep 1
  BRIDGE_RELAY_PORT=3335
fi

# 3. bridge — standalone, connects to local relay (or proxy)
echo "starting bridge (relay ws://127.0.0.1:$BRIDGE_RELAY_PORT, domain bridge.test)..."
ORLY_BRIDGE_DOMAIN=bridge.test \
ORLY_BRIDGE_RELAY_URL=ws://127.0.0.1:$BRIDGE_RELAY_PORT \
ORLY_BRIDGE_PUBLIC_RELAY_URL=ws://127.0.0.1:$BRIDGE_RELAY_PORT \
ORLY_BRIDGE_SMTP_PORT=2525 \
ORLY_BRIDGE_SMTP_HOST=127.0.0.1 \
ORLY_BRIDGE_SMTP_RELAY_HOST=127.0.0.1 \
ORLY_BRIDGE_SMTP_RELAY_PORT=2525 \
ORLY_BRIDGE_DATA_DIR="$BRIDGEDIR" \
ORLY_LOG_LEVEL=debug \
  /tmp/orly-local bridge &
PID_BRIDGE=$!
sleep 2

# Print bridge identity
if [ -f "$BRIDGEDIR/bridge.nsec" ]; then
  echo ""
  echo "=== bridge identity ==="
  cat "$BRIDGEDIR/bridge.nsec"
  echo ""
fi

echo ""
echo "=== ready ==="
echo "smesh:   http://127.0.0.1:8090"
echo "mailpit: http://127.0.0.1:8025"
echo "relay:   ws://127.0.0.1:3334"
if [ "$LATENCY" -gt 0 ] 2>/dev/null; then
  echo "proxy:   ws://127.0.0.1:3335 (${LATENCY}ms +${JITTER}ms)"
fi
echo "bridge:  npub14jr5zjp8ahx8jqsxcuh6xym256gaqy4gvljzlsa9fzpsnyhsftaq0dt3gd"
echo ""
echo "DM the bridge npub above. Send 'status' or 'subscribe' to start."
echo "Press Ctrl+C to stop all services."
wait
