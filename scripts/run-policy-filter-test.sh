#!/bin/bash
set -euo pipefail

# Policy Filter Integration Test
# This script runs the relay with the example policy and tests event filtering

# Config
PORT=${PORT:-34568}
URL=${URL:-ws://127.0.0.1:${PORT}}
LOG=/tmp/orly-policy-filter.out
PID=/tmp/orly-policy-filter.pid
DATADIR=$(mktemp -d)
CONFIG_DIR="$HOME/.config/ORLY_POLICY_TEST"

cleanup() {
  trap - EXIT
  if [[ -f "$PID" ]]; then
    kill -INT "$(cat "$PID")" 2>/dev/null || true
    rm -f "$PID"
  fi
  rm -rf "$DATADIR"
  rm -rf "$CONFIG_DIR"
}
trap cleanup EXIT

echo "🧪 Policy Filter Integration Test"
echo "=================================="

# Create config directory
mkdir -p "$CONFIG_DIR"

# Generate keys using Go helper
echo "🔑 Generating test keys..."
KEYGEN_TMP=$(mktemp)
cat > "$KEYGEN_TMP.go" <<'EOF'
package main

import (
	"encoding/json"
	"fmt"
	p256k1signer "p256k1.mleku.dev/signer"
	"git.smesh.lol/orly/pkg/encoders/hex"
)

func main() {
	// Generate allowed signer
	allowedSigner := p256k1signer.NewP256K1Signer()
	if err := allowedSigner.Generate(); err != nil {
		panic(err)
	}
	allowedPubkeyHex := hex.Enc(allowedSigner.Pub())
	allowedSecHex := hex.Enc(allowedSigner.Sec())

	// Generate unauthorized signer
	unauthorizedSigner := p256k1signer.NewP256K1Signer()
	if err := unauthorizedSigner.Generate(); err != nil {
		panic(err)
	}
	unauthorizedPubkeyHex := hex.Enc(unauthorizedSigner.Pub())
	unauthorizedSecHex := hex.Enc(unauthorizedSigner.Sec())

	result := map[string]string{
		"allowedPubkey":      allowedPubkeyHex,
		"allowedSec":         allowedSecHex,
		"unauthorizedPubkey": unauthorizedPubkeyHex,
		"unauthorizedSec":    unauthorizedSecHex,
	}

	jsonBytes, _ := json.Marshal(result)
	fmt.Println(string(jsonBytes))
}
EOF

# Run from the project root directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"
KEYS=$(go run -tags=cgo "$KEYGEN_TMP.go" 2>&1 | grep -E '^\{.*\}$' || true)
rm -f "$KEYGEN_TMP.go"
cd - > /dev/null

ALLOWED_PUBKEY=$(echo "$KEYS" | jq -r '.allowedPubkey')
ALLOWED_SEC=$(echo "$KEYS" | jq -r '.allowedSec')
UNAUTHORIZED_PUBKEY=$(echo "$KEYS" | jq -r '.unauthorizedPubkey')
UNAUTHORIZED_SEC=$(echo "$KEYS" | jq -r '.unauthorizedSec')

echo "✅ Generated keys:"
echo "   Allowed pubkey: $ALLOWED_PUBKEY"
echo "   Unauthorized pubkey: $UNAUTHORIZED_PUBKEY"

# Create policy JSON with generated keys
echo "📝 Creating policy.json..."
cat > "$CONFIG_DIR/policy.json" <<EOF
{
  "kind": {
    "whitelist": [4678, 10306, 30520, 30919]
  },
  "rules": {
    "4678": {
      "description": "Zenotp message events",
      "script": "$CONFIG_DIR/validate4678.js",
      "privileged": true
    },
    "10306": {
      "description": "End user whitelist changes",
      "read_allow": [
        "$ALLOWED_PUBKEY"
      ],
      "privileged": true
    },
    "30520": {
      "description": "Zenotp events",
      "write_allow": [
        "$ALLOWED_PUBKEY"
      ],
      "privileged": true
    },
    "30919": {
      "description": "Zenotp events",
      "write_allow": [
        "$ALLOWED_PUBKEY"
      ],
      "privileged": true
    }
  }
}
EOF

echo "✅ Policy file created at: $CONFIG_DIR/policy.json"

# Build relay and test client
echo "🔨 Building relay..."
go build -o orly .

# Start relay
echo "🚀 Starting relay on ${URL} with policy enabled..."
ORLY_APP_NAME="ORLY_POLICY_TEST" \
ORLY_DATA_DIR="$DATADIR" \
ORLY_PORT=${PORT} \
ORLY_POLICY_ENABLED=true \
ORLY_ACL_MODE=none \
ORLY_AUTH_TO_WRITE=true \
ORLY_LOG_LEVEL=info \
./orly >"$LOG" 2>&1 & echo $! >"$PID"

# Wait for relay to start
sleep 3
if ! ps -p "$(cat "$PID")" >/dev/null 2>&1; then
  echo "❌ Relay failed to start; logs:" >&2
  sed -n '1,200p' "$LOG" >&2
  exit 1
fi

echo "✅ Relay started (PID: $(cat "$PID"))"

# Build test client
echo "🔨 Building test client..."
go build -o cmd/policyfiltertest/policyfiltertest ./cmd/policyfiltertest

# Export keys for test client
export ALLOWED_PUBKEY
export ALLOWED_SEC
export UNAUTHORIZED_PUBKEY
export UNAUTHORIZED_SEC

# Run tests
echo "🧪 Running policy filter tests..."
set +e
cmd/policyfiltertest/policyfiltertest -url "${URL}" -allowed-pubkey "$ALLOWED_PUBKEY" -allowed-sec "$ALLOWED_SEC" -unauthorized-pubkey "$UNAUTHORIZED_PUBKEY" -unauthorized-sec "$UNAUTHORIZED_SEC"
TEST_RESULT=$?
set -e

# Check logs for "policy rule is inactive" messages
echo "📋 Checking logs for policy rule inactivity..."
if grep -q "policy rule is inactive" "$LOG"; then
  echo "⚠️  WARNING: Found 'policy rule is inactive' messages in logs"
  grep "policy rule is inactive" "$LOG" | head -5
else
  echo "✅ No 'policy rule is inactive' messages found (good)"
fi

# Check logs for policy filtered events
echo "📋 Checking logs for policy filtered events..."
if grep -q "policy filtered out event" "$LOG"; then
  echo "✅ Found policy filtered events (expected):"
  grep "policy filtered out event" "$LOG" | head -5
fi

if [ $TEST_RESULT -eq 0 ]; then
  echo "✅ All tests passed!"
  exit 0
else
  echo "❌ Tests failed with exit code $TEST_RESULT"
  echo "📋 Last 50 lines of relay log:"
  tail -50 "$LOG"
  exit $TEST_RESULT
fi

