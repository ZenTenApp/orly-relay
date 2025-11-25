#!/usr/bin/env bash
# Pure Go build with purego - no CGO needed
# libsecp256k1 is loaded dynamically at runtime if available
export CGO_ENABLED=0

# Download libsecp256k1.so from nostr repository if not present
if [ ! -f "libsecp256k1.so" ]; then
    wget -q https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so -O libsecp256k1.so 2>/dev/null || true
    chmod +x libsecp256k1.so 2>/dev/null || true
fi

# Set LD_LIBRARY_PATH if library is available
if [ -f "libsecp256k1.so" ]; then
    export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)"
fi

go test -v ./... -bench=. -run=xxx -benchmem