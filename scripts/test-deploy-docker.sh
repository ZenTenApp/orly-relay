#!/bin/bash

# Test the deployment script using Docker
# This script builds a Docker image and runs the deployment tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== ORLY Deployment Script Docker Test ===${NC}"
echo ""

# Check if Docker is available
if ! command -v docker >/dev/null 2>&1; then
    echo -e "${RED}ERROR: Docker is not installed or not in PATH${NC}"
    echo "Please install Docker to run this test."
    echo ""
    echo "Alternative: Run the local test instead:"
    echo "  ./test-deploy-local.sh"
    exit 1
fi

# Check if Docker is accessible
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}ERROR: Cannot access Docker daemon${NC}"
    echo "This usually means:"
    echo "  1. Docker daemon is not running"
    echo "  2. Current user is not in the 'docker' group"
    echo "  3. Need to run with sudo"
    echo ""
    echo "Try one of these solutions:"
    echo "  sudo ./test-deploy-docker.sh"
    echo "  sudo usermod -aG docker \$USER && newgrp docker"
    echo ""
    echo "Alternative: Run the local test instead:"
    echo "  ./test-deploy-local.sh"
    exit 1
fi

# Check if we're in the right directory
if [[ ! -f "go.mod" ]] || ! grep -q "git.smesh.lol/orly" go.mod; then
    echo -e "${RED}ERROR: This script must be run from the git.smesh.lol/orly project root${NC}"
    exit 1
fi

echo -e "${YELLOW}Building Docker test image...${NC}"
docker build -f scripts/Dockerfile.deploy-test -t orly-deploy-test . || {
    echo -e "${RED}ERROR: Failed to build Docker test image${NC}"
    exit 1
}

echo ""
echo -e "${YELLOW}Running deployment tests...${NC}"
echo ""

# Run the container and capture the exit code
if docker run --rm orly-deploy-test; then
    echo ""
    echo -e "${GREEN}✅ All deployment tests passed successfully!${NC}"
    echo ""
    echo -e "${BLUE}The deployment script is ready for use.${NC}"
    echo ""
    echo "To deploy ORLY on a server:"
    echo "  1. Clone the repository"
    echo "  2. Run: ./scripts/deploy.sh"
    echo "  3. Configure environment variables"
    echo "  4. Start the service: sudo systemctl start orly"
    echo ""
else
    echo ""
    echo -e "${RED}❌ Deployment tests failed!${NC}"
    echo ""
    echo "Please check the output above for specific errors."
    exit 1
fi

# Clean up the test image
echo -e "${YELLOW}Cleaning up test image...${NC}"
docker rmi orly-deploy-test >/dev/null 2>&1 || true

echo -e "${GREEN}Test completed successfully!${NC}"
