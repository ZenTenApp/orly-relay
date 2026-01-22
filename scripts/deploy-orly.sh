#!/bin/bash
# ORLY Deployment Script
# Usage: curl -sSL https://relay.orly.dev/deploy.sh | bash -s -- [options]
#
# Options:
#   --binaries-url URL    URL to download binaries tarball from
#   --local-path PATH     Local path to binaries (for scp-based deploy)
#   --host HOST           Target host (default: relay.orly.dev)
#   --restart             Restart service after deployment
#   --rollback            Rollback to previous release
#   --list                List available releases
#   --help                Show this help

set -e

# Configuration
DEPLOY_HOST="${DEPLOY_HOST:-relay.orly.dev}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Binary names and their symlink mappings
declare -A BINARIES=(
    ["orly"]="orly-relay"
    ["orly-db"]="orly-db-badger"
    ["orly-acl"]="orly-acl-follows"
    ["orly-launcher"]="orly-launcher"
    ["orly-sync-negentropy"]="orly-sync-negentropy"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    head -15 "$0" | tail -13
    exit 0
}

# Parse arguments
BINARIES_URL=""
LOCAL_PATH=""
DO_RESTART=false
DO_ROLLBACK=false
DO_LIST=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --binaries-url)
            BINARIES_URL="$2"
            shift 2
            ;;
        --local-path)
            LOCAL_PATH="$2"
            shift 2
            ;;
        --host)
            DEPLOY_HOST="$2"
            shift 2
            ;;
        --restart)
            DO_RESTART=true
            shift
            ;;
        --rollback)
            DO_ROLLBACK=true
            shift
            ;;
        --list)
            DO_LIST=true
            shift
            ;;
        --help|-h)
            show_help
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            ;;
    esac
done

# List releases
if $DO_LIST; then
    log_info "Available releases on $DEPLOY_HOST:"
    ssh "$DEPLOY_HOST" 'ls -la ~/.local/bin/releases/ 2>/dev/null | tail -n +4' || log_error "Failed to list releases"

    log_info "Current symlinks:"
    ssh "$DEPLOY_HOST" 'ls -la ~/.local/bin/orly* 2>/dev/null | grep "^l"' || log_warn "No symlinks found"
    exit 0
fi

# Rollback to previous release
if $DO_ROLLBACK; then
    log_info "Rolling back to previous release on $DEPLOY_HOST..."

    # Get the two most recent releases
    RELEASES=$(ssh "$DEPLOY_HOST" 'ls -1t ~/.local/bin/releases/ | head -2')
    CURRENT=$(echo "$RELEASES" | head -1)
    PREVIOUS=$(echo "$RELEASES" | tail -1)

    if [ -z "$PREVIOUS" ] || [ "$CURRENT" = "$PREVIOUS" ]; then
        log_error "No previous release found to rollback to"
        exit 1
    fi

    log_info "Rolling back from $CURRENT to $PREVIOUS"

    # Update symlinks to previous release
    ssh "$DEPLOY_HOST" "bash -c '
        RELEASES_DIR=~/.local/bin/releases
        BIN_DIR=~/.local/bin
        PREVIOUS=\"$PREVIOUS\"

        cd \$BIN_DIR
        for link in orly-*; do
            if [ -L \"\$link\" ]; then
                target=\$(readlink \"\$link\")
                binary=\$(basename \"\$target\")
                if [ -f \"\$RELEASES_DIR/\$PREVIOUS/\$binary\" ]; then
                    ln -sf \"\$RELEASES_DIR/\$PREVIOUS/\$binary\" \"\$link\"
                    echo \"Updated \$link -> \$RELEASES_DIR/\$PREVIOUS/\$binary\"
                fi
            fi
        done
    '"

    if $DO_RESTART; then
        log_info "Restarting service..."
        ssh "root@$DEPLOY_HOST" "systemctl restart orly" || log_warn "Failed to restart (may need root)"
    fi

    log_info "Rollback complete"
    exit 0
fi

# Deploy new release
log_info "Deploying to $DEPLOY_HOST (release: $TIMESTAMP)"

# Create release directory
ssh "$DEPLOY_HOST" "mkdir -p ~/.local/bin/releases/$TIMESTAMP"

if [ -n "$LOCAL_PATH" ]; then
    # Deploy from local path
    log_info "Deploying from local path: $LOCAL_PATH"

    for binary in "${!BINARIES[@]}"; do
        symlink_name="${BINARIES[$binary]}"
        if [ -f "$LOCAL_PATH/$binary" ]; then
            log_info "Uploading $binary as $symlink_name..."
            scp "$LOCAL_PATH/$binary" "$DEPLOY_HOST:~/.local/bin/releases/$TIMESTAMP/$symlink_name"
            ssh "$DEPLOY_HOST" "chmod +x ~/.local/bin/releases/$TIMESTAMP/$symlink_name"
        fi
    done

elif [ -n "$BINARIES_URL" ]; then
    # Deploy from URL
    log_info "Downloading binaries from $BINARIES_URL"

    ssh "$DEPLOY_HOST" "bash -c '
        cd ~/.local/bin/releases/$TIMESTAMP
        curl -sSL \"$BINARIES_URL\" | tar xz
        chmod +x *
    '"

else
    log_error "No binaries source specified. Use --local-path or --binaries-url"
    exit 1
fi

# Update symlinks
log_info "Updating symlinks..."
ssh "$DEPLOY_HOST" "bash -c '
    RELEASES_DIR=~/.local/bin/releases
    BIN_DIR=~/.local/bin
    TIMESTAMP=\"$TIMESTAMP\"

    cd \$BIN_DIR
    for binary in \$RELEASES_DIR/\$TIMESTAMP/*; do
        name=\$(basename \"\$binary\")
        ln -sf \"\$binary\" \"\$name\"
        echo \"  \$name -> \$binary\"
    done
'"

# Show deployed binaries
log_info "Deployed binaries:"
ssh "$DEPLOY_HOST" "ls -la ~/.local/bin/releases/$TIMESTAMP/"

# Restart service if requested
if $DO_RESTART; then
    log_info "Restarting orly service..."
    ssh "root@$DEPLOY_HOST" "systemctl restart orly" && log_info "Service restarted" || log_warn "Failed to restart (may need root)"
fi

# Cleanup old releases (keep last 5)
log_info "Cleaning up old releases (keeping last 5)..."
ssh "$DEPLOY_HOST" 'bash -c '\''
    RELEASES_DIR=~/.local/bin/releases
    cd "$RELEASES_DIR"
    ls -1t | tail -n +6 | while read old; do
        echo "  Removing old release: $old"
        rm -rf "$old"
    done
'\'''

log_info "Deployment complete!"
log_info "To restart the service: ssh root@$DEPLOY_HOST 'systemctl restart orly'"
