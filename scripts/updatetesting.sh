#!/bin/bash
set -euo pipefail

# updatetesting.sh - Rebuild the relay and restart it. Smesh hot-reloads automatically.
#
# Requires localtesting.sh to have been run first.

TESTDIR="/tmp/orly-localtest"
PIDDIR="$TESTDIR/pids"
LOGDIR="$TESTDIR/logs"
BINARY="$TESTDIR/orly"
SCRIPTDIR="$(cd "$(dirname "$0")" && pwd)"
REPODIR="$(cd "$SCRIPTDIR/.." && pwd)"

if [[ ! -f "$PIDDIR/relay.pid" ]]; then
    echo "error: no local testing instance found"
    echo "Run scripts/localtesting.sh first"
    exit 1
fi

RELAY_PID=$(cat "$PIDDIR/relay.pid")

if ! kill -0 "$RELAY_PID" 2>/dev/null; then
    echo "warning: relay process $RELAY_PID is not running"
    echo "Will rebuild and start fresh"
fi

# Rebuild relay
echo "Building relay..."
cd "$REPODIR"
CGO_ENABLED=0 go build -o "$BINARY" ./cmd/orly
echo "Relay built: $BINARY"

# Kill relay process tree (launcher + all children)
if kill -0 "$RELAY_PID" 2>/dev/null; then
    echo "Stopping relay (PID $RELAY_PID)..."
    # Kill the entire process group spawned by the launcher
    kill -- -"$RELAY_PID" 2>/dev/null || kill "$RELAY_PID" 2>/dev/null || true
    # Wait for it to exit
    for i in $(seq 1 10); do
        if ! kill -0 "$RELAY_PID" 2>/dev/null; then
            break
        fi
        sleep 0.5
    done
    # Force kill if still alive
    if kill -0 "$RELAY_PID" 2>/dev/null; then
        echo "Force killing relay..."
        kill -9 -- -"$RELAY_PID" 2>/dev/null || kill -9 "$RELAY_PID" 2>/dev/null || true
    fi
fi

if [[ -z "${SUPERUSER:-}" ]]; then
    echo "error: SUPERUSER environment variable not set"
    exit 1
fi

# Restart relay with same config
echo "Starting relay..."
ORLY_DATA_DIR="$TESTDIR/data" \
ORLY_PORT=3334 \
ORLY_LISTEN=127.0.0.1 \
ORLY_LOG_LEVEL=info \
ORLY_ADMINS="$SUPERUSER" \
ORLY_OWNERS="$SUPERUSER" \
ORLY_AUTH_REQUIRED=false \
ORLY_AUTH_TO_WRITE=false \
ORLY_LAUNCHER_DB_DRIVER=badger \
ORLY_LAUNCHER_DB_LISTEN=127.0.0.1:50061 \
ORLY_LAUNCHER_ACL_ENABLED=true \
ORLY_LAUNCHER_ACL_LISTEN=127.0.0.1:50062 \
ORLY_LAUNCHER_ACL_READY_TIMEOUT=600s \
ORLY_ACL_MODE=follows \
ORLY_DB_TYPE=grpc \
ORLY_GRPC_SERVER=127.0.0.1:50061 \
ORLY_ACL_TYPE=grpc \
ORLY_GRPC_ACL=127.0.0.1:50062 \
ORLY_NEGENTROPY_ENABLED=false \
ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED=false \
ORLY_FOLLOW_LIST_FREQUENCY=1h \
ORLY_DB_BLOCK_CACHE_MB=256 \
ORLY_DB_INDEX_CACHE_MB=128 \
ORLY_QUERY_CACHE_DISABLED=false \
ORLY_QUERY_CACHE_SIZE_MB=64 \
ORLY_SERIAL_CACHE_PUBKEYS=50000 \
ORLY_SERIAL_CACHE_EVENT_IDS=200000 \
ORLY_GC_ENABLED=false \
ORLY_MAX_CONN_PER_IP=50 \
ORLY_QUERY_RESULT_LIMIT=256 \
ORLY_WEB_DISABLE=true \
"$BINARY" launcher > "$LOGDIR/relay.log" 2>&1 &

NEW_PID=$!
echo "$NEW_PID" > "$PIDDIR/relay.pid"

# Wait for relay to be ready
echo -n "Waiting for relay..."
for i in $(seq 1 30); do
    if curl -s -o /dev/null http://127.0.0.1:3334 2>/dev/null; then
        echo " ready"
        break
    fi
    if ! kill -0 "$NEW_PID" 2>/dev/null; then
        echo " failed (process exited)"
        echo "Check logs: $LOGDIR/relay.log"
        exit 1
    fi
    echo -n "."
    sleep 1
done

echo "Relay updated and running (PID $NEW_PID)"
