#!/bin/bash
# ORLY Relay Upgrade Script
#
# Single-command upgrade: reads version from pkg/version/version, builds the
# unified binary (with embedded web UI), deploys to the target host via rsync,
# and restarts the systemd service.
#
# Usage:
#   ./scripts/upgrade.sh                    # Build + deploy + restart relay.orly.dev
#   ./scripts/upgrade.sh --dry-run          # Show what would happen, don't execute
#   ./scripts/upgrade.sh --build-only       # Build locally, don't deploy
#   ./scripts/upgrade.sh --host new.orly.dev # Deploy to a different host
#   ./scripts/upgrade.sh --no-restart       # Deploy without restarting the service
#   ./scripts/upgrade.sh --no-web           # Skip web UI rebuild (faster iteration)
#   ./scripts/upgrade.sh --skip-deploy      # Build and show version, don't deploy
#
# The typical workflow is:
#   1. Edit pkg/version/version (bump the version)
#   2. Run: ./scripts/upgrade.sh
#   3. The script builds, deploys, and restarts automatically.
#
# Architecture:
#   - Builds the UNIFIED binary (./cmd/orly) which includes launcher subcommands
#   - Target arch is amd64 (relay.orly.dev is x86_64, NOT arm64)
#   - Web UI is rebuilt and embedded unless --no-web is passed
#   - Previous binary is preserved on the server for manual rollback

set -euo pipefail

# --- Configuration ---
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
VERSION_FILE="$PROJECT_DIR/pkg/version/version"

DEPLOY_HOST="relay.orly.dev"
DEPLOY_USER="root"
DEPLOY_IP="69.164.249.71"
REMOTE_BIN="/home/mleku/.local/bin/orly"
SERVICE_NAME="orly"
SSH_KEY="$HOME/.ssh/id_ed25519"
SSH_OPTS="-i $SSH_KEY -o IdentitiesOnly=yes"

# Build settings
TARGET_GOOS="linux"
TARGET_GOARCH="amd64"
BUILD_OUTPUT="$PROJECT_DIR/orly"

# Flags
DRY_RUN=false
BUILD_ONLY=false
NO_RESTART=false
NO_WEB=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${BLUE}[upgrade]${NC} $1"; }
ok()   { echo -e "${GREEN}[upgrade]${NC} $1"; }
warn() { echo -e "${YELLOW}[upgrade]${NC} $1"; }
err()  { echo -e "${RED}[upgrade]${NC} $1" >&2; }
bold() { echo -e "${BOLD}$1${NC}"; }

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)      DRY_RUN=true; shift ;;
        --build-only)   BUILD_ONLY=true; shift ;;
        --no-restart)   NO_RESTART=true; shift ;;
        --no-web)       NO_WEB=true; shift ;;
        --skip-deploy)  BUILD_ONLY=true; shift ;;
        --host)         DEPLOY_HOST="$2"; shift 2 ;;
        --help|-h)
            head -25 "$0" | tail -23
            exit 0
            ;;
        *)
            err "Unknown option: $1"
            exit 1
            ;;
    esac
done

# --- Preflight checks ---
cd "$PROJECT_DIR"

if [[ ! -f "$VERSION_FILE" ]]; then
    err "Version file not found: $VERSION_FILE"
    exit 1
fi

VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')
if [[ -z "$VERSION" ]]; then
    err "Version file is empty"
    exit 1
fi

# Check for required tools
for cmd in go rsync ssh; do
    if ! command -v "$cmd" &>/dev/null; then
        err "Required command not found: $cmd"
        exit 1
    fi
done

if [[ "$NO_WEB" == false ]] && ! command -v bun &>/dev/null; then
    warn "bun not found — skipping web UI build (use --no-web to suppress this warning)"
    NO_WEB=true
fi

# Show plan
echo ""
bold "ORLY Upgrade Plan"
echo "  Version:    $VERSION"
echo "  Binary:     unified (./cmd/orly)"
echo "  Target:     ${TARGET_GOOS}/${TARGET_GOARCH}"
echo "  Host:       $DEPLOY_HOST"
echo "  Web UI:     $(if $NO_WEB; then echo 'skip'; else echo 'rebuild'; fi)"
echo "  Restart:    $(if $NO_RESTART; then echo 'no'; else echo 'yes'; fi)"
if $DRY_RUN; then
    echo "  Mode:       DRY RUN (no changes)"
fi
echo ""

if $DRY_RUN; then
    log "Dry run — exiting."
    exit 0
fi

# --- Step 1: Build web UI ---
if [[ "$NO_WEB" == false ]]; then
    log "Building web UI..."
    WEB_DIR="$PROJECT_DIR/app/web"
    if [[ -d "$WEB_DIR" ]]; then
        (cd "$WEB_DIR" && bun install --silent && bun run build)
        ok "Web UI built"
    else
        warn "Web directory not found at $WEB_DIR — skipping"
    fi
else
    log "Skipping web UI build (--no-web)"
fi

# --- Step 2: Build unified binary ---
log "Building unified binary for ${TARGET_GOOS}/${TARGET_GOARCH}..."
CGO_ENABLED=0 GOOS="$TARGET_GOOS" GOARCH="$TARGET_GOARCH" \
    go build -ldflags "-s -w" -o "$BUILD_OUTPUT" ./cmd/orly

BINARY_SIZE=$(du -h "$BUILD_OUTPUT" | cut -f1)
ok "Built: $BUILD_OUTPUT ($BINARY_SIZE)"

# Verify the binary has the right architecture
BINARY_ARCH=$(file "$BUILD_OUTPUT" | grep -o 'x86-64\|aarch64\|ARM' || echo 'unknown')
if [[ "$TARGET_GOARCH" == "amd64" && "$BINARY_ARCH" != *"x86-64"* ]]; then
    warn "Binary architecture mismatch — expected x86-64, got: $BINARY_ARCH"
fi

if $BUILD_ONLY; then
    ok "Build complete. Binary: $BUILD_OUTPUT ($VERSION)"
    exit 0
fi

# --- Step 3: Deploy ---
log "Stopping service on $DEPLOY_HOST..."
ssh $SSH_OPTS "${DEPLOY_USER}@${DEPLOY_IP}" "systemctl stop $SERVICE_NAME" || warn "Service may not be running"

log "Backing up current binary..."
ssh $SSH_OPTS "${DEPLOY_USER}@${DEPLOY_IP}" \
    "cp -f $REMOTE_BIN ${REMOTE_BIN}.prev 2>/dev/null || true"

log "Deploying $VERSION to $DEPLOY_HOST..."
rsync -avz --compress -e "ssh $SSH_OPTS" \
    "$BUILD_OUTPUT" "${DEPLOY_USER}@${DEPLOY_IP}:${REMOTE_BIN}"

# Fix ownership (we deploy as root but binary should be owned by mleku)
ssh $SSH_OPTS "${DEPLOY_USER}@${DEPLOY_IP}" \
    "chown mleku:mleku $REMOTE_BIN && chmod +x $REMOTE_BIN"
ok "Deployed to $DEPLOY_HOST:$REMOTE_BIN"

# --- Step 4: Restart ---
if [[ "$NO_RESTART" == false ]]; then
    log "Starting service..."
    ssh $SSH_OPTS "${DEPLOY_USER}@${DEPLOY_IP}" "systemctl start $SERVICE_NAME"

    # Brief pause then verify
    sleep 3
    log "Verifying..."
    if ssh $SSH_OPTS "${DEPLOY_USER}@${DEPLOY_IP}" "systemctl is-active $SERVICE_NAME" | grep -q "active"; then
        ok "Service is running"
    else
        err "Service failed to start — check: ssh ${DEPLOY_USER}@${DEPLOY_IP} 'journalctl -u $SERVICE_NAME -n 50'"
        err "Rollback: ssh ${DEPLOY_USER}@${DEPLOY_IP} 'cp ${REMOTE_BIN}.prev ${REMOTE_BIN} && systemctl start $SERVICE_NAME'"
        exit 1
    fi

    # Show version from running relay
    REMOTE_VERSION=$(ssh $SSH_OPTS "${DEPLOY_USER}@${DEPLOY_IP}" "$REMOTE_BIN version" 2>/dev/null || echo "unknown")
    ok "Remote version: $REMOTE_VERSION"
else
    log "Skipping restart (--no-restart)"
    log "Start manually: ssh ${DEPLOY_USER}@${DEPLOY_IP} 'systemctl start $SERVICE_NAME'"
fi

echo ""
ok "Upgrade complete: $VERSION deployed to $DEPLOY_HOST"
echo ""
echo "  Rollback:  ssh ${DEPLOY_USER}@${DEPLOY_IP} 'cp ${REMOTE_BIN}.prev ${REMOTE_BIN} && systemctl restart $SERVICE_NAME'"
echo "  Logs:      ssh ${DEPLOY_USER}@${DEPLOY_IP} 'journalctl -u $SERVICE_NAME -f'"
echo ""
