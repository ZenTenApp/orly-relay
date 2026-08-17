#!/bin/bash
# deploy.sh — Mirror the GitHub Actions deployment flow locally.
#
# Replicates .github/workflows/deploy.yml step-for-step:
#   1. Build the unified ./cmd/orly binary for linux/amd64 with the same
#      flags the CI runner uses (CGO_ENABLED=0, ldflags "-s -w",
#      GOEXPERIMENT=greenteagc,jsonv2).
#   2. Ship the prebuilt binary to each target host over scp using an explicit
#      SSH key (the same key the CI secret DEPLOY_SSH_KEY provides).
#   3. SSH in as root, back up the previous binary, stop the service, install
#      the new binary, start the service, and verify it is active (health check).
#
# All host info comes from flags — there is NO default host.
#
# Usage:
#   ./scripts/deploy.sh \
#       --host dm1.zentext.me \
#       --host new.orly.dev \
#       --key ~/.ssh/id_ed25519
#
# Flags (each also falls back to its $ENV unless overridden on the CLI):
#   --host HOST        target host; repeatable for multiple hosts (required)
#   --key PATH         SSH private key to use (req; fallback SSH_KEY)
#                      passed with ssh/scp -i; never copied to ~/.ssh/id_ed25519
#   --ip HOST          explicit IP/host to keyscan (per run; fallback DEPLOY_IP)
#   --port N           ssh port (default 22; fallback DEPLOY_PORT)
#   --restart          restart service after install (default)
#   --no-restart       do not restart the service
#   --remote-bin PATH  remote binary path (default /root/orly; fallback REMOTE_BIN)
#   --service NAME     systemd unit name (default orly; fallback SERVICE)
#   --goos OS          build GOOS (default linux)
#   --goarch ARCH      build GOARCH (default amd64)
#   --goexperiment EXP build GOEXPERIMENT (default greenteagc,jsonv2)
#   --cgo 0|1          build CGO_ENABLED (default 0)
#   --version VER      version string to inject (default from pkg/version/version)
#   --commit SHA       git commit sha to inject (default from git rev-parse)
#   --build-date DATE  build timestamp to inject (default now)
#   -h, --help         show this help

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_OUTPUT="$PROJECT_DIR/orly"

# --- Configuration (flags win, env fills in the rest, then defaults) ---
HOSTS=()
DEPLOY_KEY="${SSH_KEY:-}"
SSH_USER="root"
DEPLOY_IP="${DEPLOY_IP:-}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
RESTART=true
REMOTE_BIN="${REMOTE_BIN:-/root/orly}"
SERVICE="${SERVICE:-orly}"

GO_VERSION="${GO_VERSION:-1.25.3}"
CGO_ENABLED="${CGO_ENABLED:-0}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
GOEXPERIMENT="${GOEXPERIMENT:-greenteagc,jsonv2}"

# Version information injected at build time via -ldflags (mirrors CI).
VERSION="${VERSION:-$(cat "$PROJECT_DIR/pkg/version/version" | tr -d '[:space:]')}"
COMMIT="${COMMIT:-$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log(){ echo -e "${BLUE}[deploy]${NC} $1"; }
ok(){  echo -e "${GREEN}[deploy]${NC} $1"; }
warn(){ echo -e "${YELLOW}[deploy]${NC} $1"; }
err(){ echo -e "${RED}[deploy]${NC} $1" >&2; }

usage() {
    sed -n '2,50p' "$0" | sed -e 's/^# \{0,1\}//'
    exit 0
}

# --- Parse flags ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        --host)        HOSTS+=("$2"); shift 2 ;;
        --key)         DEPLOY_KEY="$2"; shift 2 ;;
        --user)
            err "--user is not accepted. Deploy is root-only."
            exit 1
            ;;
        --ip)          DEPLOY_IP="$2"; shift 2 ;;
        --port)        DEPLOY_PORT="$2"; shift 2 ;;
        --restart)     RESTART=true; shift ;;
        --no-restart)  RESTART=false; shift ;;
        --remote-bin)  REMOTE_BIN="$2"; shift 2 ;;
        --service)     SERVICE="$2"; shift 2 ;;
        --goos)        GOOS="$2"; shift 2 ;;
        --goarch)      GOARCH="$2"; shift 2 ;;
        --goexperiment) GOEXPERIMENT="$2"; shift 2 ;;
        --cgo)         CGO_ENABLED="$2"; shift 2 ;;
        --version)     VERSION="$2"; shift 2 ;;
        --commit)      COMMIT="$2"; shift 2 ;;
        --build-date)  BUILD_DATE="$2"; shift 2 ;;
        -h|--help)     usage ;;
        *)
            err "Unknown option: $1"
            echo "Try: $0 --help"
            exit 1
            ;;
    esac
done

# --- Hosts: required, from flags only ---
if [[ ${#HOSTS[@]} -eq 0 ]]; then
    err "No target host(s) given. Use --host <host> (repeatable)."
    echo "Try: $0 --help"
    exit 1
fi

# --- Preflight ---
if [[ -z "$DEPLOY_KEY" ]]; then
    err "No SSH key provided. Use --key /path/to/key (deploys as $SSH_USER)."
    exit 1
fi
if [[ ! -f "$DEPLOY_KEY" ]]; then
    err "SSH key not found: $DEPLOY_KEY"
    exit 1
fi
for cmd in go scp ssh ssh-keyscan; do
    if ! command -v "$cmd" &>/dev/null; then
        err "Required command not found: $cmd"
        exit 1
    fi
done

# --- Step 1: Build ---
log "Building $GOOS/$GOARCH unified binary (./cmd/orly)..."
log "Version: $VERSION (commit: $COMMIT)"
cd "$PROJECT_DIR"
LDFLAGS="-s -w -X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildDate=$BUILD_DATE"
CGO_ENABLED="$CGO_ENABLED" GOOS="$GOOS" GOARCH="$GOARCH" GOEXPERIMENT="$GOEXPERIMENT" \
    go build -ldflags "$LDFLAGS" -o "$BUILD_OUTPUT" ./cmd/orly
BINARY_SIZE=$(du -h "$BUILD_OUTPUT" | cut -f1)
BINARY_ARCH=$(file "$BUILD_OUTPUT" | grep -o 'x86-64\|aarch64\|ARM' || echo 'unknown')
ok "Built: $BUILD_OUTPUT ($BINARY_SIZE, $BINARY_ARCH)"

# Smoke test the binary locally (as CI does).
if "$BUILD_OUTPUT" version >/dev/null 2>&1; then
    LOCAL_VERSION=$("$BUILD_OUTPUT" version 2>/dev/null || echo "unknown")
    ok "Local smoke test passed ($LOCAL_VERSION)"
else
    warn "Local smoke test failed — continuing anyway"
fi

# --- Prepare SSH (use the given key in place; never copy it to id_ed25519) ---
mkdir -p ~/.ssh && chmod 700 ~/.ssh
for host in "${HOSTS[@]}"; do
    ssh-keyscan -p "$DEPLOY_PORT" -H "${DEPLOY_IP:-$host}" >> ~/.ssh/known_hosts 2>/dev/null || true
done
chmod 644 ~/.ssh/known_hosts 2>/dev/null || true

SSH="ssh -p $DEPLOY_PORT -i $DEPLOY_KEY -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15"
SCP="scp -P $DEPLOY_PORT -i $DEPLOY_KEY -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"

# --- Step 2 + 3: Deploy and start/verify on each host ---
for host in "${HOSTS[@]}"; do
    log "==> Deploying to $host"

    log "  Backing up current binary..."
    $SSH "${SSH_USER}@$host" "cp -f $REMOTE_BIN ${REMOTE_BIN}.prev 2>/dev/null || true"

    if [[ "$RESTART" == "true" ]]; then
        log "  Stopping service..."
        $SSH "${SSH_USER}@$host" "systemctl stop $SERVICE" || true
        # Reap any leftover orly processes so the new install is the only binary.
        $SSH "${SSH_USER}@$host" "REMOTE_BIN='$REMOTE_BIN' bash -s" <<'REMOTE'
set -euo pipefail
for pid in /proc/[0-9]*; do
    exe=$(readlink -f "$pid/exe" 2>/dev/null || true)
    [ -n "$exe" ] || continue
    case "$exe" in
        *orly*) ;;
        *) continue ;;
    esac
    echo "  killing leftover pid=${pid##*/} exe=$exe"
    kill -TERM "${pid##*/}" 2>/dev/null || true
done
sleep 1
for pid in /proc/[0-9]*; do
    exe=$(readlink -f "$pid/exe" 2>/dev/null || true)
    [ -n "$exe" ] || continue
    case "$exe" in
        *orly*) ;;
        *) continue ;;
    esac
    echo "  killing leftover pid=${pid##*/} exe=$exe (SIGKILL)"
    kill -KILL "${pid##*/}" 2>/dev/null || true
done
REMOTE
    fi

    log "  Installing new binary..."
    $SSH "${SSH_USER}@$host" "mkdir -p $(dirname "$REMOTE_BIN")"
    $SCP "$BUILD_OUTPUT" "${SSH_USER}@$host:$REMOTE_BIN"
    $SSH "${SSH_USER}@$host" "chmod +x $REMOTE_BIN"

    if [[ "$RESTART" == "true" ]]; then
        log "  Starting service..."
        $SSH "${SSH_USER}@$host" "systemctl start $SERVICE"
        sleep 3
        if $SSH "${SSH_USER}@$host" "systemctl is-active $SERVICE" | grep -q active; then
            ok "  ✔ $SERVICE active on $host"
        else
            err "Service failed to start on $host — journalctl -u $SERVICE -n 50"
            err "Rollback: ssh -p $DEPLOY_PORT $SSH_USER@$host 'cp ${REMOTE_BIN}.prev ${REMOTE_BIN} && systemctl restart $SERVICE'"
            exit 1
        fi
        log "  Verifying single $REMOTE_BIN as root..."
        $SSH "${SSH_USER}@$host" "REMOTE_BIN='$REMOTE_BIN' SERVICE='$SERVICE' bash -s" <<'REMOTE'
set -euo pipefail
unit_user=$(systemctl show -p User --value "$SERVICE" 2>/dev/null || true)
if [ -n "$unit_user" ] && [ "$unit_user" != "root" ]; then
    echo "ERROR: systemd unit $SERVICE User=$unit_user (must be root)" >&2
    exit 1
fi
found=0
bad=0
for pid in /proc/[0-9]*; do
    exe=$(readlink -f "$pid/exe" 2>/dev/null || true)
    [ -n "$exe" ] || continue
    case "$exe" in
        *orly*) ;;
        *) continue ;;
    esac
    found=$((found+1))
    user=$(stat -c %U "$pid" 2>/dev/null || echo unknown)
    cmd=$(tr '\0' ' ' < "$pid/cmdline" 2>/dev/null || true)
    echo "  pid=${pid##*/} user=$user exe=$exe cmd=$cmd"
    if [ "$exe" != "$REMOTE_BIN" ]; then
        echo "ERROR: unexpected orly executable: $exe (pid ${pid##*/})" >&2
        bad=1
    fi
    if [ "$user" != "root" ]; then
        echo "ERROR: orly process not running as root: user=$user pid=${pid##*/}" >&2
        bad=1
    fi
done
if [ "$found" -eq 0 ]; then
    echo "ERROR: no orly process found after start" >&2
    exit 1
fi
if [ "$bad" -ne 0 ]; then
    exit 1
fi
echo "ok: $found orly process(es), all $REMOTE_BIN as root"
REMOTE
    else
        warn "  Skipping restart (--no-restart)."
    fi

    REMOTE_VER=$($SSH "${SSH_USER}@$host" "$REMOTE_BIN version" 2>/dev/null || echo "unknown")
    ok "  Remote version: $REMOTE_VER"
done

ok "Deployment complete."
