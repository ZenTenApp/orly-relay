#!/usr/bin/env bash
#
# Bootstrap script for ORLY relay
#
# This script clones the ORLY repository and runs the deployment script.
# It can be executed directly via curl:
#
#   curl -sSL https://git.nostrdev.com/mleku/git.smesh.lol/orly/raw/branch/main/scripts/bootstrap.sh | bash
#
# Or downloaded and executed:
#
#   curl -o bootstrap.sh https://git.nostrdev.com/mleku/git.smesh.lol/orly/raw/branch/main/scripts/bootstrap.sh
#   chmod +x bootstrap.sh
#   ./bootstrap.sh

set -e  # Exit on error
set -u  # Exit on undefined variable
set -o pipefail  # Exit on pipe failure

# Configuration
REPO_URL="https://git.nostrdev.com/mleku/git.smesh.lol/orly.git"
REPO_NAME="git.smesh.lol/orly"
CLONE_DIR="${HOME}/src/${REPO_NAME}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print functions
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Error handler
error_exit() {
    print_error "$1"
    exit 1
}

# Check if git is installed
check_git() {
    if ! command -v git &> /dev/null; then
        error_exit "git is not installed. Please install git and try again."
    fi
    print_success "git is installed"
}

# Clone or update repository
clone_or_update_repo() {
    if [ -d "${CLONE_DIR}/.git" ]; then
        print_info "Repository already exists at ${CLONE_DIR}"
        print_info "Updating repository..."

        cd "${CLONE_DIR}" || error_exit "Failed to change to directory ${CLONE_DIR}"

        # Stash any local changes
        if ! git diff-index --quiet HEAD --; then
            print_warning "Local changes detected. Stashing them..."
            git stash || error_exit "Failed to stash changes"
        fi

        # Pull latest changes
        git pull origin main || error_exit "Failed to update repository"
        print_success "Repository updated successfully"
    else
        print_info "Cloning repository from ${REPO_URL}..."

        # Create parent directory if it doesn't exist
        mkdir -p "$(dirname "${CLONE_DIR}")" || error_exit "Failed to create directory $(dirname "${CLONE_DIR}")"

        # Clone the repository
        git clone "${REPO_URL}" "${CLONE_DIR}" || error_exit "Failed to clone repository"
        print_success "Repository cloned successfully to ${CLONE_DIR}"

        cd "${CLONE_DIR}" || error_exit "Failed to change to directory ${CLONE_DIR}"
    fi
}

# Run deployment script
run_deployment() {
    print_info "Running deployment script..."

    if [ ! -f "${CLONE_DIR}/scripts/deploy.sh" ]; then
        error_exit "Deployment script not found at ${CLONE_DIR}/scripts/deploy.sh"
    fi

    chmod +x "${CLONE_DIR}/scripts/deploy.sh" || error_exit "Failed to make deployment script executable"

    "${CLONE_DIR}/scripts/deploy.sh" || error_exit "Deployment failed"

    print_success "Deployment completed successfully!"
}

# Main execution
main() {
    echo ""
    print_info "ORLY Relay Bootstrap Script"
    print_info "=============================="
    echo ""

    check_git
    clone_or_update_repo
    run_deployment

    echo ""
    print_success "Bootstrap process completed successfully!"
    echo ""
    print_info "The ORLY relay has been deployed."
    print_info "Repository location: ${CLONE_DIR}"
    echo ""
    print_info "To start the relay service:"
    echo "  sudo systemctl start orly"
    echo ""
    print_info "To check the relay status:"
    echo "  sudo systemctl status orly"
    echo ""
    print_info "To view relay logs:"
    echo "  sudo journalctl -u orly -f"
    echo ""
}

# Run main function
main
