#!/usr/bin/env bash
# Pure Go build with purego - no CGO needed
# libsecp256k1 is loaded dynamically at runtime if available
export CGO_ENABLED=0

# Verify libsecp256k1.so exists in repo (should be at repo root)
if [ -f "libsecp256k1.so" ]; then
    chmod +x libsecp256k1.so 2>/dev/null || true
fi

# Set LD_LIBRARY_PATH if library is available
if [ -f "libsecp256k1.so" ]; then
    export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)"
fi

go test -v ./... -bench=. -run=xxx -benchmem