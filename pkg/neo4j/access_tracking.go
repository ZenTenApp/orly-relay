package neo4j

import (
	"context"
	"fmt"
	"time"
)

// RecordEventAccess updates access tracking for an event in Neo4j.
// This creates or updates an AccessTrack node for the event.
func (n *N) RecordEventAccess(serial uint64, connectionID string) error {
	cypher := `
MERGE (a:AccessTrack {serial: $serial})
ON CREATE SET a.lastAccess = $now, a.count = 1
ON MATCH SET a.lastAccess = $now, a.count = a.count + 1`

	params := map[string]any{
		"serial": int64(serial), // Neo4j uses int64
		"now":    time.Now().Unix(),
	}

	_, err := n.ExecuteWrite(context.Background(), cypher, params)
	if err != nil {
		return fmt.Errorf("failed to record event access: %w", err)
	}

	return nil
}

// GetEventAccessInfo returns access information for an event.
func (n *N) GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error) {
	cypher := "MATCH (a:AccessTrack {serial: $serial}) RETURN a.lastAccess AS lastAccess, a.count AS count"
	params := map[string]any{"serial": int64(serial)}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get event access info: %w", err)
	}

	ctx := context.Background()
	if result.Next(ctx) {
		record := result.Record()
		if record != nil {
			if la, found := record.Get("lastAccess"); found {
				if v, ok := la.(int64); ok {
					lastAccess = v
				}
			}
			if c, found := record.Get("count"); found {
				if v, ok := c.(int64); ok {
					accessCount = uint32(v)
				}
			}
		}
	}

	return lastAccess, accessCount, nil
}

// GetLeastAccessedEvents returns event serials sorted by coldness.
func (n *N) GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error) {
	cutoffTime := time.Now().Unix() - minAgeSec

	cypher := `
MATCH (a:AccessTrack)
WHERE a.lastAccess < $cutoff
RETURN a.serial AS serial, a.lastAccess AS lastAccess, a.count AS count
ORDER BY (a.lastAccess + a.count * 3600) ASC
LIMIT $limit`

	params := map[string]any{
		"cutoff": cutoffTime,
		"limit":  limit,
	}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get least accessed events: %w", err)
	}

	ctx := context.Background()
	for result.Next(ctx) {
		record := result.Record()
		if record != nil {
			if s, found := record.Get("serial"); found {
				if v, ok := s.(int64); ok {
					serials = append(serials, uint64(v))
				}
			}
		}
	}

	return serials, nil
}
