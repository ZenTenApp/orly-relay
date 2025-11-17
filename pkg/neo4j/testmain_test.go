package neo4j

import (
	"os"
	"testing"
)

// skipIfNeo4jNotAvailable skips the test if Neo4j is not available
func skipIfNeo4jNotAvailable(t *testing.T) {
	// Check if Neo4j connection details are provided
	uri := os.Getenv("ORLY_NEO4J_URI")
	if uri == "" {
		t.Skip("Neo4j not available (set ORLY_NEO4J_URI to enable tests)")
	}
}
