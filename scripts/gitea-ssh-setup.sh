#!/usr/bin/env bash
set -euo pipefail

# Gitea SSH Configuration Script
# Configures Gitea to use the system SSH server on port 22

GITEA_BASE_DIR="/home/mleku/gitea"
GITEA_USER="mleku"
SSH_DIR="/home/${GITEA_USER}/.ssh"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== Gitea SSH Configuration Script ===${NC}"
echo "Configuring Gitea to use system SSH on port 22"
echo ""

# Check if running as the correct user
if [ "$(whoami)" != "$GITEA_USER" ]; then
    echo -e "${RED}Error: This script must be run as user '${GITEA_USER}'${NC}"
    echo "Run: sudo -u ${GITEA_USER} $0"
    exit 1
fi

# Ensure SSH directory exists
echo -e "${YELLOW}Setting up SSH directory...${NC}"
mkdir -p "${SSH_DIR}"
chmod 700 "${SSH_DIR}"

# Create SSH key if it doesn't exist
if [ ! -f "${SSH_DIR}/id_ed25519" ]; then
    echo -e "${YELLOW}Generating SSH key for Gitea...${NC}"
    ssh-keygen -t ed25519 -C "gitea@$(hostname)" -f "${SSH_DIR}/id_ed25519" -N ""
    echo -e "${GREEN}✓ SSH key generated${NC}"
else
    echo -e "${GREEN}✓ SSH key already exists${NC}"
fi

# Update Gitea configuration
echo -e "${YELLOW}Updating Gitea configuration...${NC}"
GITEA_CONFIG="${GITEA_BASE_DIR}/custom/conf/app.ini"

if [ ! -f "$GITEA_CONFIG" ]; then
    echo -e "${RED}Error: Gitea configuration not found at ${GITEA_CONFIG}${NC}"
    exit 1
fi

# Backup existing config
cp "${GITEA_CONFIG}" "${GITEA_CONFIG}.backup.$(date +%Y%m%d_%H%M%S)"

# Update SSH settings in app.ini
# We'll use sed to update or add the SSH settings
if grep -q "^\[server\]" "$GITEA_CONFIG"; then
    # Section exists, update settings
    sed -i '/^\[server\]/,/^\[/ {
        /^DISABLE_SSH/d
        /^SSH_DOMAIN/d
        /^SSH_PORT/d
        /^SSH_LISTEN_HOST/d
        /^SSH_LISTEN_PORT/d
        /^START_SSH_SERVER/d
    }' "$GITEA_CONFIG"

    # Add updated settings after [server] section
    sed -i '/^\[server\]/a\
START_SSH_SERVER = false\
SSH_DOMAIN       = localhost\
SSH_PORT         = 22\
DISABLE_SSH      = false' "$GITEA_CONFIG"
else
    echo -e "${RED}Error: [server] section not found in config${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Gitea configuration updated${NC}"

# Print next steps
echo ""
echo -e "${GREEN}=== Configuration Complete ===${NC}"
echo ""
echo "Gitea has been configured to use system SSH on port 22."
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo ""
echo "1. Restart Gitea to apply changes:"
echo "   sudo systemctl restart gitea"
echo ""
echo "2. Add your SSH public key to Gitea:"
echo "   - Log in to Gitea web interface"
echo "   - Go to Settings → SSH/GPG Keys"
echo "   - Click 'Add Key'"
echo "   - Paste your public key (from ~/.ssh/id_ed25519.pub or id_rsa.pub)"
echo ""
echo "3. Test SSH access:"
echo "   ssh -T git@localhost -p 22"
echo "   (You should see: 'Hi there! You've successfully authenticated...')"
echo ""
echo "4. Clone repositories using SSH:"
echo "   git clone git@your-server:mleku/repo-name.git"
echo ""
echo -e "${BLUE}Configuration backup saved to:${NC}"
echo "   ${GITEA_CONFIG}.backup.$(date +%Y%m%d_%H%M%S)"
echo ""
