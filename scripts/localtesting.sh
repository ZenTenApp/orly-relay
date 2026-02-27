#!/bin/bash
set -euo pipefail

# localtesting.sh - Start a local ORLY relay + smesh dev server for testing.
#
# Requires:
#   SUPERUSER - your npub (used for ORLY_ADMINS and ORLY_OWNERS)
#   SMESH_SRC - (optional) path to smesh source, defaults to ~/src/smesh
#
# Runs relay on ws://localhost:3334 and smesh on http://localhost:5173

TESTDIR="/tmp/orly-localtest"
PIDDIR="$TESTDIR/pids"
DATADIR="$TESTDIR/data"
LOGDIR="$TESTDIR/logs"
BINARY="$TESTDIR/orly"
SCRIPTDIR="$(cd "$(dirname "$0")" && pwd)"
REPODIR="$(cd "$SCRIPTDIR/.." && pwd)"
SMESH_SRC="${SMESH_SRC:-$HOME/src/smesh}"

if [[ -z "${SUPERUSER:-}" ]]; then
    echo "error: SUPERUSER environment variable not set"
    echo "Set it to your npub, e.g.:"
    echo "  export SUPERUSER=npub1..."
    exit 1
fi

if [[ ! -d "$SMESH_SRC" ]]; then
    echo "error: smesh source not found at $SMESH_SRC"
    echo "Set SMESH_SRC to the correct path"
    exit 1
fi

# Check for existing instance
if [[ -f "$PIDDIR/relay.pid" ]]; then
    pid=$(cat "$PIDDIR/relay.pid")
    if kill -0 "$pid" 2>/dev/null; then
        echo "error: local testing already running (relay PID $pid)"
        echo "Run scripts/stoptesting.sh first, or scripts/updatetesting.sh to rebuild"
        exit 1
    fi
fi

mkdir -p "$PIDDIR" "$DATADIR" "$LOGDIR"

# Build relay
echo "Building relay..."
cd "$REPODIR"
CGO_ENABLED=0 go build -o "$BINARY" ./cmd/orly
echo "Relay built: $BINARY"

# Start relay launcher in background
echo "Starting relay..."
ORLY_DATA_DIR="$DATADIR" \
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
ORLY_WEB_DISABLE=false \
"$BINARY" launcher > "$LOGDIR/relay.log" 2>&1 &

RELAY_PID=$!
echo "$RELAY_PID" > "$PIDDIR/relay.pid"
echo "Relay launcher started (PID $RELAY_PID), logs: $LOGDIR/relay.log"

# Start smesh dev server in background
echo "Starting smesh dev server..."
cd "$SMESH_SRC"
bun run dev > "$LOGDIR/smesh.log" 2>&1 &

SMESH_PID=$!
echo "$SMESH_PID" > "$PIDDIR/smesh.pid"
echo "Smesh dev server started (PID $SMESH_PID), logs: $LOGDIR/smesh.log"

# Wait for relay to be ready
echo -n "Waiting for relay..."
for i in $(seq 1 30); do
    if curl -s -o /dev/null http://127.0.0.1:3334 2>/dev/null; then
        echo " ready"
        break
    fi
    if ! kill -0 "$RELAY_PID" 2>/dev/null; then
        echo " failed (process exited)"
        echo "Check logs: $LOGDIR/relay.log"
        exit 1
    fi
    echo -n "."
    sleep 1
done

echo ""
echo "Local testing environment running:"
echo "  Relay:  ws://localhost:3334"
echo "  Smesh:  http://localhost:5173"
echo "  Owner:  $SUPERUSER"
echo "  Data:   $DATADIR"
echo "  Logs:   $LOGDIR/"
echo ""
echo "Commands:"
echo "  scripts/updatetesting.sh  - rebuild and restart relay"
echo "  scripts/stoptesting.sh    - stop everything"
