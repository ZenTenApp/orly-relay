#!/bin/bash
# Build ARM64 binaries and deploy to relay
# Usage: ./scripts/build-and-deploy.sh [relay.orly.dev|new.orly.dev|both] [--restart]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build-arm64"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Parse arguments
TARGET="${1:-relay.orly.dev}"
RESTART_FLAG=""
if [[ "$2" == "--restart" ]] || [[ "$1" == "--restart" ]]; then
    RESTART_FLAG="--restart"
fi

# Build for ARM64
log_info "Building ARM64 binaries..."
cd "$PROJECT_DIR"
mkdir -p "$BUILD_DIR"

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=arm64

# Build all binaries
log_info "Building orly (main relay)..."
go build -o "$BUILD_DIR/orly" .

log_info "Building orly-db..."
go build -o "$BUILD_DIR/orly-db" ./cmd/orly-db

log_info "Building orly-acl..."
go build -o "$BUILD_DIR/orly-acl" ./cmd/orly-acl

log_info "Building orly-launcher..."
go build -o "$BUILD_DIR/orly-launcher" ./cmd/orly-launcher

log_info "Building orly-sync-negentropy..."
go build -o "$BUILD_DIR/orly-sync-negentropy" ./cmd/orly-sync-negentropy

log_info "Build complete. Binaries in $BUILD_DIR"
ls -la "$BUILD_DIR"

# Deploy function
deploy_to() {
    local host="$1"
    log_info "Deploying to $host..."
    "$SCRIPT_DIR/deploy-orly.sh" --host "$host" --local-path "$BUILD_DIR" $RESTART_FLAG
}

# Deploy based on target
case "$TARGET" in
    both)
        deploy_to "relay.orly.dev"
        deploy_to "new.orly.dev"
        ;;
    relay.orly.dev|new.orly.dev)
        deploy_to "$TARGET"
        ;;
    --restart)
        # --restart was first arg, deploy to default
        deploy_to "relay.orly.dev"
        ;;
    *)
        log_warn "Unknown target: $TARGET. Using relay.orly.dev"
        deploy_to "relay.orly.dev"
        ;;
esac

log_info "Done!"
