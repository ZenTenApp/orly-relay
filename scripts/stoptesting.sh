#!/bin/bash
set -euo pipefail

# stoptesting.sh - Stop local testing relay and smesh dev server.
#
# Usage:
#   scripts/stoptesting.sh          # stop processes, keep data
#   scripts/stoptesting.sh --clean  # stop processes and remove all test data

TESTDIR="/tmp/orly-localtest"
PIDDIR="$TESTDIR/pids"
CLEAN=false

if [[ "${1:-}" == "--clean" ]]; then
    CLEAN=true
fi

stopped=0

# Stop relay
if [[ -f "$PIDDIR/relay.pid" ]]; then
    RELAY_PID=$(cat "$PIDDIR/relay.pid")
    if kill -0 "$RELAY_PID" 2>/dev/null; then
        echo "Stopping relay (PID $RELAY_PID)..."
        kill -- -"$RELAY_PID" 2>/dev/null || kill "$RELAY_PID" 2>/dev/null || true
        for i in $(seq 1 10); do
            if ! kill -0 "$RELAY_PID" 2>/dev/null; then
                break
            fi
            sleep 0.5
        done
        if kill -0 "$RELAY_PID" 2>/dev/null; then
            kill -9 -- -"$RELAY_PID" 2>/dev/null || kill -9 "$RELAY_PID" 2>/dev/null || true
        fi
        echo "Relay stopped"
        stopped=$((stopped + 1))
    else
        echo "Relay process $RELAY_PID not running (stale PID file)"
    fi
    rm -f "$PIDDIR/relay.pid"
fi

# Stop smesh
if [[ -f "$PIDDIR/smesh.pid" ]]; then
    SMESH_PID=$(cat "$PIDDIR/smesh.pid")
    if kill -0 "$SMESH_PID" 2>/dev/null; then
        echo "Stopping smesh (PID $SMESH_PID)..."
        kill -- -"$SMESH_PID" 2>/dev/null || kill "$SMESH_PID" 2>/dev/null || true
        for i in $(seq 1 5); do
            if ! kill -0 "$SMESH_PID" 2>/dev/null; then
                break
            fi
            sleep 0.5
        done
        if kill -0 "$SMESH_PID" 2>/dev/null; then
            kill -9 -- -"$SMESH_PID" 2>/dev/null || kill -9 "$SMESH_PID" 2>/dev/null || true
        fi
        echo "Smesh stopped"
        stopped=$((stopped + 1))
    else
        echo "Smesh process $SMESH_PID not running (stale PID file)"
    fi
    rm -f "$PIDDIR/smesh.pid"
fi

if [[ $stopped -eq 0 ]]; then
    echo "No running processes found"
fi

if [[ "$CLEAN" == true ]]; then
    echo "Removing test data: $TESTDIR"
    rm -rf "$TESTDIR"
    echo "Clean complete"
else
    # Clean up empty pids dir
    rmdir "$PIDDIR" 2>/dev/null || true
fi

echo "Local testing stopped"
