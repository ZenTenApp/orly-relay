#!/usr/bin/env bash
# Pure Go build with purego - no CGO needed
# libsecp256k1 is loaded dynamically at runtime if available
export CGO_ENABLED=0

# Verify libsecp256k1.so exists in repo (should be at repo root)
if [ ! -f "libsecp256k1.so" ]; then
    echo "Warning: libsecp256k1.so not found in repo - tests may use fallback crypto"
else
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