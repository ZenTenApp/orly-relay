#!/bin/sh
# deploy.sh — Push assets to VPS and restart.
# --delete removes stale files on the remote that no longer exist locally.
set -e
cd "$(dirname "$0")"

echo "syncing assets..."
rsync -avz --delete --exclude='*.go' app/smesh3/ orly:/home/mleku/sm3sh/

echo "restarting orly..."
ssh orly "systemctl restart orly"

echo "deploy complete"
