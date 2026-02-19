#!/usr/bin/env bash
# Generate a DKIM RSA-2048 key pair for the Nostr-Email bridge.
#
# Usage: ./scripts/generate-dkim.sh <domain> [selector]
#
# Generates:
#   dkim-private.pem  — private key (set ORLY_BRIDGE_DKIM_KEY to this path)
#   Prints the DNS TXT record to add for DKIM verification
#
# The selector defaults to "marmot" if not specified.

set -euo pipefail

DOMAIN="${1:-}"
SELECTOR="${2:-marmot}"

if [ -z "$DOMAIN" ]; then
    echo "Usage: $0 <domain> [selector]"
    echo ""
    echo "Example: $0 relay.example.com marmot"
    echo ""
    echo "This generates dkim-private.pem and prints the DNS TXT record."
    exit 1
fi

KEYFILE="dkim-private.pem"

if [ -f "$KEYFILE" ]; then
    echo "Error: $KEYFILE already exists. Remove it first or use a different directory."
    exit 1
fi

echo "Generating RSA-2048 key pair..."
openssl genrsa -out "$KEYFILE" 2048 2>/dev/null

# Extract public key in DER format, base64 encode (single line)
PUBKEY=$(openssl rsa -in "$KEYFILE" -pubout -outform DER 2>/dev/null | openssl base64 -A)

chmod 600 "$KEYFILE"

echo ""
echo "Private key written to: $KEYFILE"
echo "  Set ORLY_BRIDGE_DKIM_KEY=$(pwd)/$KEYFILE"
echo "  Set ORLY_BRIDGE_DKIM_SELECTOR=$SELECTOR"
echo ""
echo "Add this DNS TXT record:"
echo ""
echo "  ${SELECTOR}._domainkey.${DOMAIN}.  IN  TXT  \"v=DKIM1; k=rsa; p=${PUBKEY}\""
echo ""
echo "You can verify it after propagation with:"
echo "  dig TXT ${SELECTOR}._domainkey.${DOMAIN}"
