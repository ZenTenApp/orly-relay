package store

import "context"

// CypherExecutor is an optional interface for database backends that support
// direct Cypher query execution (currently only Neo4j). The gRPC client also
// implements this to proxy queries in split IPC mode.
type CypherExecutor interface {
	ExecuteCypherRead(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)
}
