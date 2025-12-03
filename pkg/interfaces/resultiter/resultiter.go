// Package resultiter defines interfaces for iterating over database query results.
package resultiter

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jResultIterator defines the interface for iterating over Neo4j query results.
// This is implemented by both neo4j.Result and CollectedResult types.
type Neo4jResultIterator interface {
	Next(context.Context) bool
	Record() *neo4j.Record
	Err() error
}
