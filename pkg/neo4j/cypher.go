package neo4j

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

// writeKeywords matches Cypher write operations. This is a safety net for the
// owner-only endpoint, not a security boundary.
var writeKeywords = regexp.MustCompile(`(?i)\b(CREATE|MERGE|SET|DELETE|REMOVE|DETACH|DROP|CALL)\b`)

// stripCypherComments removes single-line (//) and block (/* */) comments
// from a Cypher query string so they don't trigger false write-keyword matches.
func stripCypherComments(q string) string {
	// Block comments
	for {
		start := strings.Index(q, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(q[start+2:], "*/")
		if end < 0 {
			q = q[:start]
			break
		}
		q = q[:start] + q[start+2+end+2:]
	}
	// Single-line comments
	lines := strings.Split(q, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

// validateReadOnly checks that a Cypher query contains no write operations.
func validateReadOnly(cypher string) error {
	stripped := stripCypherComments(cypher)
	if writeKeywords.MatchString(stripped) {
		return fmt.Errorf("query contains write operations; only read-only queries are allowed")
	}
	return nil
}

// ExecuteCypherRead executes a read-only Cypher query and returns the results
// as a slice of maps. Each map represents one record with column names as keys.
// This implements the store.CypherExecutor interface.
func (n *N) ExecuteCypherRead(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error) {
	if err := validateReadOnly(cypher); err != nil {
		return nil, err
	}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("cypher read failed: %w", err)
	}

	records := make([]map[string]any, 0, result.Len())
	for result.Next(ctx) {
		rec := result.Record()
		row := make(map[string]any, len(rec.Keys))
		for _, key := range rec.Keys {
			val, _ := rec.Get(key)
			row[key] = convertNeo4jValue(val)
		}
		records = append(records, row)
	}
	return records, nil
}

// convertNeo4jValue converts Neo4j driver types (Node, Relationship, Path)
// into JSON-serializable plain maps and slices.
func convertNeo4jValue(v any) any {
	switch val := v.(type) {
	case dbtype.Node:
		m := map[string]any{
			"_type":   "node",
			"_id":     val.ElementId,
			"_labels": val.Labels,
		}
		for k, pv := range val.Props {
			m[k] = convertNeo4jValue(pv)
		}
		return m
	case dbtype.Relationship:
		return map[string]any{
			"_type":     "relationship",
			"_id":       val.ElementId,
			"_relType":  val.Type,
			"_startId":  val.StartElementId,
			"_endId":    val.EndElementId,
			"_props":    convertNeo4jProps(val.Props),
		}
	case dbtype.Path:
		nodes := make([]any, len(val.Nodes))
		for i, node := range val.Nodes {
			nodes[i] = convertNeo4jValue(node)
		}
		rels := make([]any, len(val.Relationships))
		for i, rel := range val.Relationships {
			rels[i] = convertNeo4jValue(rel)
		}
		return map[string]any{
			"_type":          "path",
			"_nodes":         nodes,
			"_relationships": rels,
		}
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = convertNeo4jValue(item)
		}
		return out
	case map[string]any:
		return convertNeo4jProps(val)
	default:
		return v
	}
}

func convertNeo4jProps(props map[string]any) map[string]any {
	out := make(map[string]any, len(props))
	for k, v := range props {
		out[k] = convertNeo4jValue(v)
	}
	return out
}

// Ensure N implements CypherExecutor at compile time.
var _ interface {
	ExecuteCypherRead(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)
} = (*N)(nil)
