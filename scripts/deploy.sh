#!/bin/bash
# deploy.sh — Mirror the GitHub Actions deployment flow locally.
#
# Replicates .github/workflows/deploy.yml step-for-step:
#   1. Build the unified ./cmd/orly binary for linux/amd64 with the same
#      flags the CI runner uses (CGO_ENABLED=0, ldflags "-s -w",
#      GOEXPERIMENT=greenteagc,jsonv2).
#   2. Ship the prebuilt binary to each target host over scp using an explicit
#      SSH key (the same key the CI secret DEPLOY_SSH_KEY provides).
#   3. SSH in, use sudo when needed to back up the previous binary, stop the service, install
#      the new binary, start the service, verify it is active (health check), reap stray
#      orly processes, and confirm exactly one binary runs as the service user.
#   4. With --bootstrap, provision a fresh Ubuntu server with the systemd unit,
#      Nginx reverse proxy, and Let's Encrypt certificate before deploying.
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
#   --user USER        ssh user (default root; fallback DEPLOY_USER)
#   --ip HOST          explicit IP/host to keyscan (per run; fallback DEPLOY_IP)
#   --port N           ssh port (default 22; fallback DEPLOY_PORT)
#   --restart          restart service after install (default)
#   --no-restart       do not restart the service
#   --remote-bin PATH  remote binary path (default /usr/local/bin/orly; fallback REMOTE_BIN)
#   --service NAME     systemd unit name (default orly; fallback SERVICE)
#   --goos OS          build GOOS (default linux)
#   --goarch ARCH      build GOARCH (default amd64)
#   --goexperiment EXP build GOEXPERIMENT (default greenteagc,jsonv2)
#   --cgo 0|1          build CGO_ENABLED (default 0)
#   --version VER      version string to inject (default from pkg/version/version)
#   --commit SHA       git commit sha to inject (default from git rev-parse)
#   --build-date DATE  build timestamp to inject (default now)
#   --bootstrap        provision the systemd service and Nginx/TLS for a fresh server
#   --domain DOMAIN    relay domain for Nginx/TLS; repeatable (required with --bootstrap)
#   --email EMAIL      Let's Encrypt email address (required with --bootstrap)
#   --relay-port N     local relay port behind Nginx (default 7777; fallback RELAY_PORT)
#   --configure-firewall allow ports 80 and 443 with UFW when bootstrapping
#   -h, --help         show this help

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_OUTPUT="$PROJECT_DIR/orly"

# --- Configuration (flags win, env fills in the rest, then defaults) ---
HOSTS=()
DEPLOY_KEY="${SSH_KEY:-}"
SSH_USER="${DEPLOY_USER:-root}"
DEPLOY_IP="${DEPLOY_IP:-}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
RESTART=true
REMOTE_BIN="${REMOTE_BIN:-/usr/local/bin/orly}"
SERVICE="${SERVICE:-orly}"
BOOTSTRAP=false
DOMAINS=()
LETSENCRYPT_EMAIL="${LETSENCRYPT_EMAIL:-}"
RELAY_PORT="${RELAY_PORT:-7777}"
CONFIGURE_FIREWALL=false

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
        --user)        SSH_USER="$2"; shift 2 ;;
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
        --bootstrap)   BOOTSTRAP=true; shift ;;
        --domain)      DOMAINS+=("$2"); shift 2 ;;
        --email)       LETSENCRYPT_EMAIL="$2"; shift 2 ;;
        --relay-port)  RELAY_PORT="$2"; shift 2 ;;
        --configure-firewall) CONFIGURE_FIREWALL=true; shift ;;
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
if [[ "$BOOTSTRAP" == "true" ]] && [[ ${#DOMAINS[@]} -eq 0 || -z "$LETSENCRYPT_EMAIL" ]]; then
    err "--bootstrap requires at least one --domain and --email."
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

# --- Prepare SSH (keyscan hosts) ---
mkdir -p ~/.ssh && chmod 700 ~/.ssh
for host in "${HOSTS[@]}"; do
    ssh-keyscan -p "$DEPLOY_PORT" -H "${DEPLOY_IP:-$host}" >> ~/.ssh/known_hosts 2>/dev/null || true
done
chmod 644 ~/.ssh/known_hosts 2>/dev/null || true

SSH=(ssh -i "$DEPLOY_KEY" -p "$DEPLOY_PORT" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
SCP=(scp -i "$DEPLOY_KEY" -P "$DEPLOY_PORT" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new)

run_remote() {
    local host="$1"
    shift
    "${SSH[@]}" "${SSH_USER}@$host" "$@"
}

run_privileged_remote() {
    local host="$1"
    shift

    if [[ "$SSH_USER" == "root" ]]; then
        run_remote "$host" "$@"
    else
        "${SSH[@]}" -tt "${SSH_USER}@$host" "sudo $*"
    fi
}

write_privileged_remote() {
    local host="$1"
    local destination="$2"

    if [[ "$SSH_USER" == "root" ]]; then
        "${SSH[@]}" "${SSH_USER}@$host" "tee $destination >/dev/null"
    else
        "${SSH[@]}" -tt "${SSH_USER}@$host" "sudo tee $destination >/dev/null"
    fi
}

bootstrap_host() {
    local host="$1"
    local domain_args=()
    local server_names
    local certbot_args=()
    local nginx_config policy_config service_config

    for domain in "${DOMAINS[@]}"; do
        domain_args+=("-d" "$domain")
    done
    server_names="${DOMAINS[*]}"
    certbot_args="${domain_args[*]}"

    nginx_config=$(cat <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name $server_names;

    location / {
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header Host \$host;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_http_version 1.1;
        proxy_pass http://127.0.0.1:$RELAY_PORT;
    }
}
EOF
)
    service_config=$(cat <<EOF
[Unit]
Description=ORLY Nostr Relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=orly
Group=orly
WorkingDirectory=/var/lib/orly
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=ORLY_POLICY_ENABLED=true
Environment=ORLY_POLICY_PATH=/etc/orly/policy.json
Environment=ORLY_AUTH_REQUIRED=true
Environment=ORLY_APP_NAME=ORLY
Environment=ORLY_DATA_DIR=/var/lib/orly
ExecStart=$REMOTE_BIN
Environment=ORLY_LISTEN=127.0.0.1
Environment=ORLY_PORT=$RELAY_PORT
Environment=ORLY_LOG_LEVEL=trace
Environment=ORLY_DB_LOG_LEVEL=error
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
)
    policy_config=$(cat <<'EOF'
{
  "kind": {
    "whitelist": [4]
  },
  "rules": {
    "4": {
      "description": "Nostr DM events",
      "privileged": true,
      "size_limit": 4096,
      "max_age_of_event": 3600,
      "max_age_event_in_future": 60,
      "max_expiry_duration": "P3D",
      "must_have_tags": ["p", "expiration"]
    }
  }
}
EOF
)

    log "  Bootstrapping $host (Nginx, TLS, and $SERVICE.service)..."
    run_privileged_remote "$host" "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y nginx certbot python3-certbot-nginx"
    run_privileged_remote "$host" "id -u orly >/dev/null 2>&1 || useradd --system --user-group --home-dir /var/lib/orly --create-home --shell /usr/sbin/nologin orly"
    run_privileged_remote "$host" "install -d -o orly -g orly -m 0750 /var/lib/orly && install -d -o root -g orly -m 0750 /etc/orly"
    run_privileged_remote "$host" "install -d -m 0755 /etc/nginx/sites-available /etc/nginx/sites-enabled"
    printf '%s' "$nginx_config" | write_privileged_remote "$host" "/etc/nginx/sites-available/$SERVICE"
    run_privileged_remote "$host" "rm -f /etc/nginx/sites-enabled/default && ln -sf /etc/nginx/sites-available/$SERVICE /etc/nginx/sites-enabled/$SERVICE && nginx -t && systemctl enable --now nginx"

    if [[ "$CONFIGURE_FIREWALL" == "true" ]]; then
        run_privileged_remote "$host" "if command -v ufw >/dev/null; then ufw allow 80/tcp && ufw allow 443/tcp; fi"
    fi

    run_privileged_remote "$host" "certbot --nginx --non-interactive --agree-tos --redirect -m '$LETSENCRYPT_EMAIL' $certbot_args"
    printf '%s' "$policy_config" | write_privileged_remote "$host" "/etc/orly/policy.json"
    run_privileged_remote "$host" "chown root:orly /etc/orly/policy.json && chmod 0640 /etc/orly/policy.json"
    printf '%s' "$service_config" | write_privileged_remote "$host" "/etc/systemd/system/$SERVICE.service"
    run_privileged_remote "$host" "systemctl daemon-reload && systemctl enable $SERVICE"
}

# --- Step 2 + 3: Deploy and start/verify on each host ---
for host in "${HOSTS[@]}"; do
    log "==> Deploying to $host"
    REMOTE_STAGING="/tmp/orly-deploy-${SERVICE}-$$"

    if [[ "$BOOTSTRAP" == "true" ]]; then
        bootstrap_host "$host"
    fi

    log "  Backing up current binary..."
    run_privileged_remote "$host" "cp -f $REMOTE_BIN ${REMOTE_BIN}.prev 2>/dev/null || true"

    if [[ "$RESTART" == "true" ]]; then
        log "  Stopping service..."
        run_privileged_remote "$host" "systemctl stop $SERVICE" || true
        # Reap any leftover orly processes so the new install is the only binary.
        run_privileged_remote "$host" "REMOTE_BIN='$REMOTE_BIN' bash -s" <<'REMOTE'
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
    "${SCP[@]}" "$BUILD_OUTPUT" "${SSH_USER}@$host:$REMOTE_STAGING"
    run_privileged_remote "$host" "mkdir -p $(dirname "$REMOTE_BIN") && install -m 0755 $REMOTE_STAGING $REMOTE_BIN && rm -f $REMOTE_STAGING"

    if [[ "$RESTART" == "true" ]]; then
        log "  Starting service..."
        run_privileged_remote "$host" "systemctl start $SERVICE"
        sleep 3
        log "  Checking service status..."
        if run_privileged_remote "$host" "systemctl is-active --quiet $SERVICE"; then
            ok "  ✔ $SERVICE active on $host"
        else
            err "Service failed to start on $host — journalctl -u $SERVICE -n 50"
            err "Rollback: ssh -p $DEPLOY_PORT $SSH_USER@$host 'cp ${REMOTE_BIN}.prev ${REMOTE_BIN} && systemctl restart $SERVICE'"
            exit 1
        fi
        log "  Verifying single $REMOTE_BIN as the service user..."
        run_privileged_remote "$host" "REMOTE_BIN='$REMOTE_BIN' SERVICE='$SERVICE' bash -s" <<'REMOTE'
set -euo pipefail
unit_user=$(systemctl show -p User --value "$SERVICE" 2>/dev/null || true)
[ -n "$unit_user" ] || unit_user=root
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
    if [ "$user" != "$unit_user" ]; then
        echo "ERROR: orly process not running as $unit_user: got $user pid=${pid##*/}" >&2
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
echo "ok: $found orly process(es), all $REMOTE_BIN as $unit_user"
REMOTE
    else
        warn "  Skipping restart (--no-restart)."
    fi

done

ok "Deployment complete."
