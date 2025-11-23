package dgraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/dgo/v230/protos/api"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// Markers provide metadata key-value storage using Dgraph predicates
// We store markers as special nodes with type "Marker"

// SetMarker sets a metadata marker
func (d *D) SetMarker(key string, value []byte) error {
	// Create or update a marker node
	markerID := "marker_" + key
	valueHex := hex.Enc(value)

	nquads := fmt.Sprintf(`
_:%s <dgraph.type> "Marker" .
_:%s <marker.key> %q .
_:%s <marker.value> %q .
`, markerID, markerID, key, markerID, valueHex)

	mutation := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	if _, err := d.Mutate(context.Background(), mutation); err != nil {
		return fmt.Errorf("failed to set marker: %w", err)
	}

	return nil
}

// GetMarker retrieves a metadata marker
func (d *D) GetMarker(key string) (value []byte, err error) {
	query := fmt.Sprintf(`{
		marker(func: eq(marker.key, %q)) {
			marker.value
		}
	}`, key)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to get marker: %w", err)
	}

	var result struct {
		Marker []struct {
			Value string `json:"marker.value"`
		} `json:"marker"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, fmt.Errorf("failed to parse marker response: %w", err)
	}

	if len(result.Marker) == 0 {
		return nil, fmt.Errorf("marker not found: %s", key)
	}

	// Decode hex value
	value, err = hex.Dec(result.Marker[0].Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode marker value: %w", err)
	}

	return value, nil
}

// HasMarker checks if a marker exists
func (d *D) HasMarker(key string) bool {
	_, err := d.GetMarker(key)
	return err == nil
}

// DeleteMarker removes a metadata marker
func (d *D) DeleteMarker(key string) error {
	// Find the marker's UID
	query := fmt.Sprintf(`{
		marker(func: eq(marker.key, %q)) {
			uid
		}
	}`, key)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to find marker: %w", err)
	}

	var result struct {
		Marker []struct {
			UID string `json:"uid"`
		} `json:"marker"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return fmt.Errorf("failed to parse marker query: %w", err)
	}

	if len(result.Marker) == 0 {
		return nil // Marker doesn't exist
	}

	// Delete the marker node
	mutation := &api.Mutation{
		DelNquads: []byte(fmt.Sprintf("<%s> * * .", result.Marker[0].UID)),
		CommitNow: true,
	}

	if _, err = d.Mutate(context.Background(), mutation); err != nil {
		return fmt.Errorf("failed to delete marker: %w", err)
	}

	return nil
}
