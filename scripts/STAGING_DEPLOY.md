# ORLY Staging Relay Deployment Procedure

Manual steps for deploying to relay.orly.dev. These match the automated
`.gitea/workflows/staging-deploy.yml` workflow.

## Prerequisites

- SSH access to `root@relay.orly.dev` (IP: 69.164.249.71)
- Go 1.25+ installed locally
- Bun installed locally (for web UI build)
- `rsync` available

## Quick Deploy (Automated Script)

The fastest way — uses the existing upgrade script:

```bash
cd /path/to/next.orly.dev

# Edit the version
vim pkg/version/version    # e.g. v0.60.4

# Deploy (builds + deploys + restarts)
./scripts/upgrade.sh

# Or dry-run first
./scripts/upgrade.sh --dry-run
```

## Full Manual Procedure

### Step 1: Check the current deployed version

Query the relay's NIP-11 endpoint to see what's currently running:

```bash
curl -s -H 'Accept: application/nostr+json' https://relay.orly.dev/ | jq '.version'
```

This returns the version without the `v` prefix (e.g. `"0.59.0"`).

### Step 2: Read the target version

```bash
cat pkg/version/version
# Output: v0.60.3
```

Compare with Step 1. If they match (ignoring the `v` prefix), no deployment
is needed.

### Step 3: Build the web UI

```bash
cd app/web
bun install
bun run build
cd ../..
```

### Step 4: Build the unified binary

Target architecture is **amd64** (relay.orly.dev is x86_64):

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w" -o orly ./cmd/orly
```

Verify the architecture:

```bash
file orly
# Should show: ELF 64-bit LSB executable, x86-64
```

### Step 5: Run tests (optional but recommended)

```bash
CGO_ENABLED=0 go test ./...
```

### Step 6: Deploy

SSH details:
- **Host**: relay.orly.dev (69.164.249.71)
- **User**: root
- **SSH Key**: ~/.ssh/id_ed25519
- **Remote binary path**: /home/mleku/.local/bin/orly
- **Service name**: orly

```bash
SSH_OPTS="-i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes"
DEPLOY_IP="69.164.249.71"
REMOTE_BIN="/home/mleku/.local/bin/orly"

# 6a. Stop the service
ssh $SSH_OPTS root@$DEPLOY_IP "systemctl stop orly"

# 6b. Backup the current binary
ssh $SSH_OPTS root@$DEPLOY_IP "cp -f $REMOTE_BIN ${REMOTE_BIN}.prev"

# 6c. Copy the new binary
rsync -avz --compress -e "ssh $SSH_OPTS" \
  orly root@$DEPLOY_IP:$REMOTE_BIN

# 6d. Fix ownership (deploy as root, binary should be owned by mleku)
ssh $SSH_OPTS root@$DEPLOY_IP "chown mleku:mleku $REMOTE_BIN && chmod +x $REMOTE_BIN"

# 6e. Start the service
ssh $SSH_OPTS root@$DEPLOY_IP "systemctl start orly"
```

### Step 7: Verify

```bash
# Check service is active
ssh $SSH_OPTS root@$DEPLOY_IP "systemctl is-active orly"

# Check version via NIP-11 (wait a few seconds for startup)
sleep 5
curl -s -H 'Accept: application/nostr+json' https://relay.orly.dev/ | jq '.version'

# Check logs for errors
ssh $SSH_OPTS root@$DEPLOY_IP "journalctl -u orly -n 20 --no-pager"
```

### Rollback

If something goes wrong, roll back to the previous binary:

```bash
ssh $SSH_OPTS root@$DEPLOY_IP \
  "cp /home/mleku/.local/bin/orly.prev /home/mleku/.local/bin/orly && systemctl restart orly"
```

## Automated Workflow

The `.gitea/workflows/staging-deploy.yml` workflow automates all of the above.
It triggers when `pkg/version/version` is modified in a push to `main`.

### Workflow logic

1. Read version from `pkg/version/version`
2. Query NIP-11 at `https://relay.orly.dev/` for deployed version
3. If versions differ: build web UI → build binary → run tests → deploy → verify
4. If versions match: skip (no-op)

### Required Gitea secrets

Configure these in the repo settings at
`https://git.nostrdev.com/mleku/next.orly.dev/settings/actions/secrets`:

| Secret | Description |
|--------|-------------|
| `DEPLOY_SSH_KEY` | Private SSH key with root access to relay.orly.dev |
| `GITEATOKEN` | Gitea API token (already configured for release workflow) |

### Triggering a deployment

```bash
# 1. Bump the version
echo "v0.60.4" > pkg/version/version

# 2. Commit and push to main
git add pkg/version/version
git commit -m "release: v0.60.4"
git push origin main

# 3. The workflow triggers automatically.
#    Monitor at: https://git.nostrdev.com/mleku/next.orly.dev/actions
```

### Modifying the workflow

Key sections you might want to change:

- **Target host**: Change `DEPLOY_HOST`, `DEPLOY_IP` in the "Deploy" step
- **Go version**: Update the `wget` URL in "Set up Go"
- **Skip tests**: Remove the "Run tests" step for faster deploys
- **Add a staging-specific relay**: Duplicate the deploy step with different host/IP
