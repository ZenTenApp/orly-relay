package neo4j

import (
	"context"
	"fmt"

	"next.orly.dev/pkg/encoders/hex"
)

// Markers provide metadata key-value storage using Neo4j Marker nodes
// We store markers as special nodes with label "Marker"

// SetMarker sets a metadata marker
func (n *N) SetMarker(key string, value []byte) error {
	valueHex := hex.Enc(value)

	cypher := `
MERGE (m:Marker {key: $key})
SET m.value = $value`

	params := map[string]any{
		"key":   key,
		"value": valueHex,
	}

	_, err := n.ExecuteWrite(context.Background(), cypher, params)
	if err != nil {
		return fmt.Errorf("failed to set marker: %w", err)
	}

	return nil
}

// GetMarker retrieves a metadata marker
func (n *N) GetMarker(key string) (value []byte, err error) {
	cypher := "MATCH (m:Marker {key: $key}) RETURN m.value AS value"
	params := map[string]any{"key": key}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get marker: %w", err)
	}

	ctx := context.Background()

	if result.Next(ctx) {
		record := result.Record()
		if record != nil {
			valueRaw, found := record.Get("value")
			if found {
				if valueStr, ok := valueRaw.(string); ok {
					// Decode hex value
					value, err = hex.Dec(valueStr)
					if err != nil {
						return nil, fmt.Errorf("failed to decode marker value: %w", err)
					}
					return value, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("marker not found: %s", key)
}

// HasMarker checks if a marker exists
func (n *N) HasMarker(key string) bool {
	_, err := n.GetMarker(key)
	return err == nil
}

// DeleteMarker removes a metadata marker
func (n *N) DeleteMarker(key string) error {
	cypher := "MATCH (m:Marker {key: $key}) DELETE m"
	params := map[string]any{"key": key}

	_, err := n.ExecuteWrite(context.Background(), cypher, params)
	if err != nil {
		return fmt.Errorf("failed to delete marker: %w", err)
	}

	return nil
}
