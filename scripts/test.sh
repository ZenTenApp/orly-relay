#!/usr/bin/env bash
# Pure Go build with purego - no CGO needed
# libsecp256k1 is loaded dynamically at runtime if available
export CGO_ENABLED=0

# Download libsecp256k1.so from nostr repository if not present
if [ ! -f "libsecp256k1.so" ]; then
    echo "Downloading libsecp256k1.so from nostr repository..."
    wget -q https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so -O libsecp256k1.so || {
        echo "Warning: Failed to download libsecp256k1.so - tests may fail"
    }
    chmod +x libsecp256k1.so 2>/dev/null || true
fi

# Set LD_LIBRARY_PATH to include current directory
if [ -f "libsecp256k1.so" ]; then
    export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)"
fi

go mod tidy
go test ./...
cd pkg/crypto
go mod tidy
go test ./...
cd ../database
go mod tidy
go test ./...
cd ../encoders
go mod tidy
go test ./...
cd ../protocol
go mod tidy
go test ./...
cd ../utils
go mod tidy
go test ./...
cd ../acl
go mod tidy
go test ./...