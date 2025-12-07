#!/bin/bash
# Neo4j Integration Test Runner
#
# This script runs the Neo4j integration tests by:
# 1. Checking if Docker/Docker Compose are available
# 2. Starting a Neo4j container
# 3. Running the integration tests
# 4. Stopping the container
#
# Usage:
#   ./scripts/test-neo4j-integration.sh
#
# Environment variables:
#   SKIP_DOCKER_INSTALL=1  - Skip Docker installation check
#   KEEP_CONTAINER=1       - Don't stop container after tests
#   NEO4J_TEST_REQUIRED=1  - Fail if Docker/Neo4j not available (for local testing)
#
# Exit codes:
#   0 - Tests passed OR Docker/Neo4j not available (soft fail for CI)
#   1 - Tests failed (only when Neo4j is available)
#   2 - Tests required but Docker/Neo4j not available (when NEO4J_TEST_REQUIRED=1)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/pkg/neo4j/docker-compose.yaml"
CONTAINER_NAME="neo4j-test"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_skip() {
    echo -e "${BLUE}[SKIP]${NC} $1"
}

# Soft fail - exit 0 for CI compatibility unless NEO4J_TEST_REQUIRED is set
soft_fail() {
    local message="$1"
    if [ "$NEO4J_TEST_REQUIRED" = "1" ]; then
        log_error "$message"
        log_error "NEO4J_TEST_REQUIRED=1 is set, failing"
        exit 2
    else
        log_skip "$message"
        log_skip "Neo4j integration tests skipped (set NEO4J_TEST_REQUIRED=1 to require)"
        exit 0
    fi
}

# Check if Docker is installed and running
check_docker() {
    if ! command -v docker &> /dev/null; then
        soft_fail "Docker is not installed"
        return 1
    fi

    if ! docker info &> /dev/null 2>&1; then
        soft_fail "Docker daemon is not running or permission denied"
        return 1
    fi

    log_info "Docker is available"
    return 0
}

# Check if Docker Compose is installed
check_docker_compose() {
    # Try docker compose (v2) first, then docker-compose (v1)
    if docker compose version &> /dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
        log_info "Using Docker Compose v2"
        return 0
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
        log_info "Using Docker Compose v1"
        return 0
    else
        soft_fail "Docker Compose is not installed"
        return 1
    fi
}

# Start Neo4j container
start_neo4j() {
    log_info "Starting Neo4j container..."

    cd "$PROJECT_ROOT"

    # Try to start container, soft fail if it doesn't work
    if ! $COMPOSE_CMD -f "$COMPOSE_FILE" up -d 2>&1; then
        soft_fail "Failed to start Neo4j container"
        return 1
    fi

    log_info "Waiting for Neo4j to become healthy..."

    # Wait for container to be healthy (up to 2 minutes)
    local timeout=120
    local elapsed=0

    while [ $elapsed -lt $timeout ]; do
        local health=$(docker inspect --format='{{.State.Health.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo "not_found")

        if [ "$health" = "healthy" ]; then
            log_info "Neo4j is healthy and ready"
            return 0
        elif [ "$health" = "not_found" ]; then
            log_warn "Container $CONTAINER_NAME not found, retrying..."
        fi

        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done

    echo ""
    log_warn "Neo4j failed to become healthy within $timeout seconds"
    log_info "Container logs:"
    docker logs "$CONTAINER_NAME" --tail 20 2>/dev/null || true

    # Clean up failed container
    $COMPOSE_CMD -f "$COMPOSE_FILE" down -v 2>/dev/null || true

    soft_fail "Neo4j container failed to start properly"
    return 1
}

# Stop Neo4j container
stop_neo4j() {
    if [ "$KEEP_CONTAINER" = "1" ]; then
        log_info "KEEP_CONTAINER=1, leaving Neo4j running"
        return 0
    fi

    log_info "Stopping Neo4j container..."
    cd "$PROJECT_ROOT"
    $COMPOSE_CMD -f "$COMPOSE_FILE" down -v 2>/dev/null || true
}

# Run integration tests
run_tests() {
    log_info "Running Neo4j integration tests..."

    cd "$PROJECT_ROOT"

    # Set environment variables for tests
    # Note: Tests use ORLY_NEO4J_* prefix (consistent with app config)
    export ORLY_NEO4J_URI="bolt://localhost:7687"
    export ORLY_NEO4J_USER="neo4j"
    export ORLY_NEO4J_PASSWORD="testpassword"
    # Also set NEO4J_TEST_URI for testmain_test.go compatibility
    export NEO4J_TEST_URI="bolt://localhost:7687"

    # Run tests with integration tag
    if go test -tags=integration ./pkg/neo4j/... -v -timeout 5m; then
        log_info "All integration tests passed!"
        return 0
    else
        log_error "Some integration tests failed"
        return 1
    fi
}

# Main execution
main() {
    log_info "Neo4j Integration Test Runner"
    log_info "=============================="

    if [ "$NEO4J_TEST_REQUIRED" = "1" ]; then
        log_info "NEO4J_TEST_REQUIRED=1 - tests will fail if Neo4j unavailable"
    else
        log_info "NEO4J_TEST_REQUIRED not set - tests will skip if Neo4j unavailable"
    fi

    # Check prerequisites (these will soft_fail if not available)
    check_docker || exit $?
    check_docker_compose || exit $?

    # Check if compose file exists
    if [ ! -f "$COMPOSE_FILE" ]; then
        soft_fail "Docker Compose file not found: $COMPOSE_FILE"
    fi

    # Track if we need to stop the container
    local need_cleanup=0

    # Check if container is already running
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${CONTAINER_NAME}$"; then
        log_info "Neo4j container is already running"
    else
        start_neo4j || exit $?
        need_cleanup=1
    fi

    # Run tests
    local test_result=0
    run_tests || test_result=1

    # Cleanup
    if [ $need_cleanup -eq 1 ]; then
        stop_neo4j
    fi

    if [ $test_result -eq 0 ]; then
        log_info "Integration tests completed successfully"
    else
        log_error "Integration tests failed"
    fi

    exit $test_result
}

# Handle cleanup on script exit
cleanup() {
    if [ "$KEEP_CONTAINER" != "1" ] && docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${CONTAINER_NAME}$"; then
        log_warn "Cleaning up after interrupt..."
        stop_neo4j
    fi
}

trap cleanup EXIT INT TERM

main "$@"
