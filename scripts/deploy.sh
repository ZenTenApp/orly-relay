#!/bin/bash

# ORLY Relay Deployment Script
# This script installs Go, builds the relay, and sets up systemd service

set -e

# Configuration
GO_VERSION="1.25.3"
GOROOT="$HOME/go"
GOPATH="$HOME"
GOBIN="$HOME/.local/bin"
GOENV_FILE="$HOME/.goenv"
BASHRC_FILE="$HOME/.bashrc"
SERVICE_NAME="orly"
BINARY_NAME="orly"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root for certain operations
check_root() {
    if [[ $EUID -eq 0 ]]; then
        return 0
    else
        return 1
    fi
}

# Check if bun is installed
check_bun_installation() {
    if command -v bun >/dev/null 2>&1; then
        local installed_version=$(bun --version)
        log_success "Bun $installed_version is already installed"
        return 0
    else
        log_info "Bun is not installed"
        return 1
    fi
}

# Install bun
install_bun() {
    log_info "Installing Bun..."
    
    # Install bun using official installer
    curl -fsSL https://bun.com/install | bash
    
    # Source bashrc to pick up bun in current session
    if [[ -f "$HOME/.bashrc" ]]; then
        source "$HOME/.bashrc"
    fi
    
    # Verify installation
    if command -v bun >/dev/null 2>&1; then
        log_success "Bun installed successfully"
    else
        log_error "Failed to install Bun"
        exit 1
    fi
}

# Check if Go is installed and get version
check_go_installation() {
    if command -v go >/dev/null 2>&1; then
        local installed_version=$(go version | grep -o 'go[0-9]\+\.[0-9]\+\.[0-9]\+' | sed 's/go//')
        local required_version=$(echo $GO_VERSION | sed 's/go//')
        
        if [[ "$installed_version" == "$required_version" ]]; then
            log_success "Go $installed_version is already installed"
            return 0
        else
            log_warning "Go $installed_version is installed, but version $required_version is required"
            return 1
        fi
    else
        log_info "Go is not installed"
        return 1
    fi
}

# Install Go
install_go() {
    log_info "Installing Go $GO_VERSION..."
    
    # Save original directory
    local original_dir=$(pwd)
    
    # Determine architecture
    local arch=$(uname -m)
    case $arch in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7l) arch="armv6l" ;;
        *) log_error "Unsupported architecture: $arch"; exit 1 ;;
    esac
    
    local go_archive="go${GO_VERSION}.linux-${arch}.tar.gz"
    local download_url="https://golang.org/dl/${go_archive}"
    
    # Remove existing installation if present (before download to save space/time)
    if [[ -d "$GOROOT" ]]; then
        log_info "Removing existing Go installation..."
        # Make it writable in case it's read-only
        chmod -R u+w "$GOROOT" 2>/dev/null || true
        rm -rf "$GOROOT"
    fi
    
    # Create directories
    mkdir -p "$GOBIN"
    
    # Change to home directory and download Go
    log_info "Downloading Go from $download_url..."
    cd ~
    wget -q "$download_url" || {
        log_error "Failed to download Go"
        exit 1
    }
    
    # Extract Go to a temporary location first, then move to final destination
    log_info "Extracting Go..."
    tar -xf "$go_archive" -C /tmp
    mv /tmp/go "$GOROOT"

    # Clean up
    rm -f "$go_archive"
    
    # Return to original directory
    cd "$original_dir"
    
    log_success "Go $GO_VERSION installed successfully"
}

# Setup Go environment
setup_go_environment() {
    log_info "Setting up Go environment..."
    
    # Create .goenv file
    cat > "$GOENV_FILE" << EOF
# Go environment configuration
export GOROOT="$GOROOT"
export GOPATH="$GOPATH"
export GOBIN="$GOBIN"
export PATH="\$GOBIN:\$GOROOT/bin:\$PATH"
EOF
    
    # Source the environment for current session
    source "$GOENV_FILE"
    
    # Add to .bashrc if not already present
    if ! grep -q "source $GOENV_FILE" "$BASHRC_FILE" 2>/dev/null; then
        log_info "Adding Go environment to $BASHRC_FILE..."
        echo "" >> "$BASHRC_FILE"
        echo "# Go environment" >> "$BASHRC_FILE"
        echo "if [[ -f \"$GOENV_FILE\" ]]; then" >> "$BASHRC_FILE"
        echo "    source \"$GOENV_FILE\"" >> "$BASHRC_FILE"
        echo "fi" >> "$BASHRC_FILE"
        log_success "Go environment added to $BASHRC_FILE"
    else
        log_info "Go environment already configured in $BASHRC_FILE"
    fi
}

# Build the application
build_application() {
    log_info "Building ORLY relay..."
    
    # Source Go environment
    source "$GOENV_FILE"
    
    # Update embedded web assets
    log_info "Updating embedded web assets..."
    ./scripts/update-embedded-web.sh
    
    # Build the binary in the current directory
    log_info "Building binary in current directory (pure Go + purego)..."
    CGO_ENABLED=0 go build -o "$BINARY_NAME"
    
    # Verify libsecp256k1.so exists in repo (used by purego for runtime crypto)
    if [[ -f "./libsecp256k1.so" ]]; then
        chmod +x libsecp256k1.so
        log_success "Found libsecp256k1.so in repository"
    else
        log_warning "libsecp256k1.so not found in repo - relay will still work but may have slower crypto"
    fi
    
    if [[ -f "./$BINARY_NAME" ]]; then
        log_success "ORLY relay built successfully"
    else
        log_error "Failed to build ORLY relay"
        exit 1
    fi
}

# Set capabilities for port 443 binding
set_capabilities() {
    log_info "Setting capabilities for port 443 binding..."
    
    if check_root; then
        setcap 'cap_net_bind_service=+ep' "./$BINARY_NAME"
    else
        sudo setcap 'cap_net_bind_service=+ep' "./$BINARY_NAME"
    fi
    
    log_success "Capabilities set for port 443 binding"
}

# Install binary
install_binary() {
    log_info "Installing binary to $GOBIN..."
    
    # Ensure GOBIN directory exists
    mkdir -p "$GOBIN"
    
    # Copy binary and library
    cp "./$BINARY_NAME" "$GOBIN/"
    chmod +x "$GOBIN/$BINARY_NAME"
    
    # Copy library if it exists
    if [[ -f "./libsecp256k1.so" ]]; then
        cp "./libsecp256k1.so" "$GOBIN/"
        log_info "Copied libsecp256k1.so to $GOBIN/"
    fi
    
    log_success "Binary installed to $GOBIN/$BINARY_NAME"
}

# Create systemd service
create_systemd_service() {
    local port="$1"
    log_info "Creating systemd service (port: $port)..."

    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    local working_dir=$(pwd)

    # Create service file content
    local service_content="[Unit]
Description=ORLY Nostr Relay
After=network.target
Wants=network.target

[Service]
Type=simple
User=$USER
Group=$USER
WorkingDirectory=$working_dir
Environment=ORLY_PORT=$port
ExecStart=$GOBIN/$BINARY_NAME
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

# Network settings
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target"

    # Write service file
    if check_root; then
        echo "$service_content" > "$service_file"
    else
        echo "$service_content" | sudo tee "$service_file" > /dev/null
    fi
    
    # Reload systemd and enable service
    if check_root; then
        systemctl daemon-reload
        systemctl enable "$SERVICE_NAME"
    else
        sudo systemctl daemon-reload
        sudo systemctl enable "$SERVICE_NAME"
    fi
    
    log_success "Systemd service created and enabled"
}

# Main deployment function
main() {
    local port="${1:-3334}"
    log_info "Starting ORLY relay deployment (port: $port)..."

    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]] || ! grep -q "next.orly.dev" go.mod; then
        log_error "This script must be run from the next.orly.dev project root directory"
        exit 1
    fi

    # Check and install Bun if needed
    if ! check_bun_installation; then
        install_bun
    fi

    # Check and install Go if needed
    if ! check_go_installation; then
        install_go
        setup_go_environment
    fi

    # Build application
    build_application

    # Set capabilities
    set_capabilities

    # Install binary
    install_binary

    # Create systemd service
    create_systemd_service "$port"
    
    log_success "ORLY relay deployment completed successfully!"
    echo ""
    log_info "Next steps:"
    echo "  1. Reload your terminal environment: source ~/.bashrc"
    echo "  2. Configure your relay by setting environment variables"
    echo "  3. Start the service: sudo systemctl start $SERVICE_NAME"
    echo "  4. Check service status: sudo systemctl status $SERVICE_NAME"
    echo "  5. View logs: sudo journalctl -u $SERVICE_NAME -f"
    echo ""
    log_info "Service management commands:"
    echo "  Start:   sudo systemctl start $SERVICE_NAME"
    echo "  Stop:    sudo systemctl stop $SERVICE_NAME"
    echo "  Restart: sudo systemctl restart $SERVICE_NAME"
    echo "  Enable:  sudo systemctl enable $SERVICE_NAME --now"
    echo "  Disable: sudo systemctl disable $SERVICE_NAME --now"
    echo "  Status:  sudo systemctl status $SERVICE_NAME"
    echo "  Logs:    sudo journalctl -u $SERVICE_NAME -f"
}

# Handle command line arguments
PORT=""
case "${1:-}" in
    --help|-h)
        echo "ORLY Relay Deployment Script"
        echo ""
        echo "Usage: $0 [options] [port]"
        echo ""
        echo "Arguments:"
        echo "  port         Port number for the relay to listen on (default: 3334)"
        echo ""
        echo "Options:"
        echo "  --help, -h   Show this help message"
        echo ""
        echo "This script will:"
        echo "  1. Install Bun if not present"
        echo "  2. Install Go $GO_VERSION if not present"
        echo "  3. Set up Go environment in ~/.goenv"
        echo "  4. Install build dependencies (requires sudo)"
        echo "  5. Build the ORLY relay"
        echo "  6. Set capabilities for port 443 binding"
        echo "  7. Install the binary to ~/.local/bin"
        echo "  8. Create and enable systemd service"
        echo ""
        echo "Examples:"
        echo "  $0              # Deploy with default port 3334"
        echo "  $0 8080         # Deploy with port 8080"
        exit 0
        ;;
    [0-9]*)
        # First argument is a number, treat it as port
        PORT="$1"
        shift
        main "$PORT" "$@"
        ;;
    *)
        main "$@"
        ;;
esac
