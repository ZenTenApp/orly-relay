#!/bin/bash
# Build and deploy ORLY relay binaries
#
# IMPORTANT: relay.orly.dev is amd64 (x86_64), NOT arm64.
# This script builds the UNIFIED binary (./cmd/orly) which includes
# launcher, db, acl, and relay subcommands.
#
# For the recommended single-command upgrade flow, use:
#   ./scripts/upgrade.sh
#
# This script is retained for deploying to multiple hosts or custom targets.
#
# Usage:
#   ./scripts/build-and-deploy.sh [relay.orly.dev|new.orly.dev|both] [--restart]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build-amd64"

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

# Read version
VERSION=$(cat "$PROJECT_DIR/pkg/version/version" | tr -d '[:space:]')
log_info "Version: $VERSION"

# Build for amd64 (relay.orly.dev is x86_64)
log_info "Building amd64 unified binary..."
cd "$PROJECT_DIR"
mkdir -p "$BUILD_DIR"

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# Build web UI if bun is available
if command -v bun &>/dev/null && [[ -d "app/web" ]]; then
    log_info "Building web UI..."
    (cd app/web && bun install --silent && bun run build)
fi

# Build unified binary (includes launcher, db, acl subcommands)
log_info "Building orly (unified binary with subcommands)..."
go build -ldflags "-s -w" -o "$BUILD_DIR/orly" ./cmd/orly

log_info "Build complete. Binary in $BUILD_DIR"
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

log_info "Done! Version $VERSION deployed."
