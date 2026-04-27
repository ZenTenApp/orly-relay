#!/bin/bash
# Enable archive relay features on mleku.dev

set -e

SERVICE_FILE="/etc/systemd/system/orly.service"

echo "Updating orly.service to enable archive features..."

sudo tee "$SERVICE_FILE" > /dev/null << 'EOF'
[Unit]
Description=ORLY Nostr Relay
After=network.target
Wants=network.target

[Service]
Type=simple
User=mleku
Group=mleku
WorkingDirectory=/home/mleku/src/git.smesh.lol/orly
ExecStart=/home/mleku/.local/bin/orly
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=orly

# Archive relay query augmentation
Environment=ORLY_ARCHIVE_ENABLED=true
Environment=ORLY_ARCHIVE_RELAYS=wss://archive.orly.dev/

# Network settings
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF

echo "Reloading systemd..."
sudo systemctl daemon-reload

echo "Building and installing new version..."
cd /home/mleku/src/git.smesh.lol/orly
CGO_ENABLED=0 go build -o orly
sudo setcap 'cap_net_bind_service=+ep' ./orly
cp ./orly ~/.local/bin/

echo "Restarting orly service..."
sudo systemctl restart orly

echo "Done! Archive features are now enabled."
echo ""
echo "Check status with: sudo systemctl status orly"
echo "View logs with: sudo journalctl -u orly -f"
