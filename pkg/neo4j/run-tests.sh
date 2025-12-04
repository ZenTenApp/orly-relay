#!/bin/bash
# Run Neo4j integration tests with Docker
# Usage: ./run-tests.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Starting Neo4j test database..."
docker compose up -d

echo "Waiting for Neo4j to be ready..."
for i in {1..30}; do
    if docker compose exec -T neo4j-test cypher-shell -u neo4j -p testpassword "RETURN 1" > /dev/null 2>&1; then
        echo "Neo4j is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "Timeout waiting for Neo4j"
        docker compose logs
        docker compose down
        exit 1
    fi
    echo "Waiting... ($i/30)"
    sleep 2
done

echo ""
echo "Running tests..."
echo "================="

# Set environment variables for tests
export NEO4J_TEST_URI="bolt://localhost:7687"
export NEO4J_TEST_USER="neo4j"
export NEO4J_TEST_PASSWORD="testpassword"

# Run tests with verbose output
cd ../..
CGO_ENABLED=0 go test -v ./pkg/neo4j/... -count=1
TEST_EXIT_CODE=$?

cd "$SCRIPT_DIR"

echo ""
echo "================="
echo "Stopping Neo4j test database..."
docker compose down

exit $TEST_EXIT_CODE
