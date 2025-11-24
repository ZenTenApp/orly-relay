#!/bin/bash
# Enable ORLY policy system

set -e

echo "Enabling ORLY policy system..."

# Backup the current service file
sudo cp /etc/systemd/system/orly.service /etc/systemd/system/orly.service.backup

# Add ORLY_POLICY_ENABLED=true to the service file
sudo sed -i '/SyslogIdentifier=orly/a\\n# Policy system\nEnvironment="ORLY_POLICY_ENABLED=true"' /etc/systemd/system/orly.service

# Reload systemd
sudo systemctl daemon-reload

echo "✓ Policy system enabled in systemd service"
echo "✓ Daemon reloaded"
echo ""
echo "Next steps:"
echo "1. Restart the relay: sudo systemctl restart orly"
echo "2. Verify policy is active: journalctl -u orly -f | grep policy"
echo ""
echo "Your policy configuration (~/.config/ORLY/policy.json):"
cat ~/.config/ORLY/policy.json
