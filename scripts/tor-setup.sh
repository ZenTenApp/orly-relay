#!/bin/bash
# tor-setup.sh - Production Tor hidden service setup for ORLY relay
#
# This script installs Tor and configures a hidden service for the relay.
# The .onion address will be automatically detected by ORLY.
#
# Usage: sudo ./scripts/tor-setup.sh [port]
#   port: internal port ORLY listens on for Tor traffic (default: 3336)
#
# Requirements:
#   - Root privileges (for installing packages and configuring Tor)
#   - Systemd-based Linux distribution
#
# After running this script:
#   1. Start ORLY with: ORLY_TOR_ENABLED=true ORLY_TOR_HS_DIR=/var/lib/tor/orly-relay ./orly
#   2. The .onion address will appear in logs and NIP-11

set -e

# Configuration
TOR_PORT="${1:-3336}"
HS_NAME="orly-relay"
HS_DIR="/var/lib/tor/${HS_NAME}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    error "Please run as root: sudo $0"
fi

# Detect package manager and install Tor
install_tor() {
    info "Installing Tor..."

    if command -v apt-get &> /dev/null; then
        # Debian/Ubuntu
        apt-get update
        apt-get install -y tor
    elif command -v dnf &> /dev/null; then
        # Fedora/RHEL
        dnf install -y tor
    elif command -v pacman &> /dev/null; then
        # Arch Linux
        pacman -Sy --noconfirm tor
    elif command -v apk &> /dev/null; then
        # Alpine
        apk add tor
    elif command -v brew &> /dev/null; then
        # macOS (run as user, not root)
        brew install tor
    else
        error "Unsupported package manager. Please install Tor manually."
    fi

    info "Tor installed successfully"
}

# Configure hidden service
configure_tor() {
    info "Configuring Tor hidden service..."

    TORRC="/etc/tor/torrc"

    # Check if hidden service already configured
    if grep -q "HiddenServiceDir ${HS_DIR}" "$TORRC" 2>/dev/null; then
        warn "Hidden service already configured in ${TORRC}"
        return 0
    fi

    # Backup original torrc
    if [ -f "$TORRC" ]; then
        cp "$TORRC" "${TORRC}.backup.$(date +%Y%m%d%H%M%S)"
        info "Backed up original torrc"
    fi

    # Add hidden service configuration
    cat >> "$TORRC" << EOF

# ORLY Relay Hidden Service
# Added by tor-setup.sh on $(date)
HiddenServiceDir ${HS_DIR}/
HiddenServicePort 80 127.0.0.1:${TOR_PORT}
EOF

    info "Hidden service configured: ${HS_DIR}"
}

# Set permissions
set_permissions() {
    info "Setting directory permissions..."

    # Create hidden service directory if it doesn't exist
    mkdir -p "$HS_DIR"

    # Set correct ownership (debian-tor on Debian/Ubuntu, tor on others)
    if id "debian-tor" &>/dev/null; then
        chown -R debian-tor:debian-tor "$HS_DIR"
    elif id "tor" &>/dev/null; then
        chown -R tor:tor "$HS_DIR"
    fi

    chmod 700 "$HS_DIR"

    info "Permissions set"
}

# Restart Tor service
restart_tor() {
    info "Restarting Tor service..."

    if command -v systemctl &> /dev/null; then
        systemctl enable tor
        systemctl restart tor
    elif command -v service &> /dev/null; then
        service tor restart
    else
        warn "Could not restart Tor. Please restart manually."
        return 1
    fi

    # Wait for Tor to create the hostname file
    info "Waiting for hidden service to initialize..."
    for i in {1..30}; do
        if [ -f "${HS_DIR}/hostname" ]; then
            break
        fi
        sleep 1
    done

    if [ -f "${HS_DIR}/hostname" ]; then
        ONION_ADDR=$(cat "${HS_DIR}/hostname")
        info "Tor service started successfully"
        echo ""
        echo -e "${GREEN}======================================${NC}"
        echo -e "${GREEN}Hidden Service Address:${NC}"
        echo -e "${YELLOW}${ONION_ADDR}${NC}"
        echo -e "${GREEN}======================================${NC}"
        echo ""
    else
        warn "Tor started but hostname file not yet created"
        warn "Check: ls -la ${HS_DIR}/"
    fi
}

# Print usage instructions
print_instructions() {
    echo ""
    info "Setup complete! To enable Tor in ORLY:"
    echo ""
    echo "  Option 1 - Environment variables:"
    echo "    export ORLY_TOR_ENABLED=true"
    echo "    export ORLY_TOR_HS_DIR=${HS_DIR}"
    echo "    export ORLY_TOR_PORT=${TOR_PORT}"
    echo "    ./orly"
    echo ""
    echo "  Option 2 - Command line:"
    echo "    ORLY_TOR_ENABLED=true ORLY_TOR_HS_DIR=${HS_DIR} ./orly"
    echo ""
    echo "  The .onion address will automatically appear in NIP-11 relay info."
    echo ""
    echo "  To view the .onion address:"
    echo "    cat ${HS_DIR}/hostname"
    echo ""
    echo "  To check Tor status:"
    echo "    systemctl status tor"
    echo ""
}

# Main
main() {
    info "ORLY Tor Hidden Service Setup"
    info "Internal port: ${TOR_PORT}"
    echo ""

    # Check if Tor is already installed
    if ! command -v tor &> /dev/null; then
        install_tor
    else
        info "Tor is already installed"
    fi

    configure_tor
    set_permissions
    restart_tor
    print_instructions
}

main
