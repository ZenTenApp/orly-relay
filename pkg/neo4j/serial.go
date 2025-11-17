package neo4j

import (
	"context"
	"fmt"
	"sync"
)

// Serial number management
// We use a special Marker node in Neo4j to track the next available serial number

const serialCounterKey = "serial_counter"

var (
	serialMutex sync.Mutex
)

// getNextSerial atomically increments and returns the next serial number
func (n *N) getNextSerial() (uint64, error) {
	serialMutex.Lock()
	defer serialMutex.Unlock()

	ctx := context.Background()

	// Query current serial value
	cypher := "MATCH (m:Marker {key: $key}) RETURN m.value AS value"
	params := map[string]any{"key": serialCounterKey}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return 0, fmt.Errorf("failed to query serial counter: %w", err)
	}

	neo4jResult, ok := result.(interface {
		Next(context.Context) bool
		Record() *interface{}
		Err() error
	})
	if !ok {
		return 1, nil
	}

	var currentSerial uint64 = 1
	if neo4jResult.Next(ctx) {
		record := neo4jResult.Record()
		if record != nil {
			recordMap, ok := (*record).(map[string]any)
			if ok {
				if value, ok := recordMap["value"].(int64); ok {
					currentSerial = uint64(value)
				}
			}
		}
	}

	// Increment serial
	nextSerial := currentSerial + 1

	// Update counter
	updateCypher := `
MERGE (m:Marker {key: $key})
SET m.value = $value`
	updateParams := map[string]any{
		"key":   serialCounterKey,
		"value": int64(nextSerial),
	}

	_, err = n.ExecuteWrite(ctx, updateCypher, updateParams)
	if err != nil {
		return 0, fmt.Errorf("failed to update serial counter: %w", err)
	}

	return currentSerial, nil
}

// initSerialCounter initializes the serial counter if it doesn't exist
func (n *N) initSerialCounter() error {
	ctx := context.Background()

	// Check if counter exists
	cypher := "MATCH (m:Marker {key: $key}) RETURN m.value AS value"
	params := map[string]any{"key": serialCounterKey}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to check serial counter: %w", err)
	}

	neo4jResult, ok := result.(interface {
		Next(context.Context) bool
		Record() *interface{}
		Err() error
	})
	if ok && neo4jResult.Next(ctx) {
		// Counter already exists
		return nil
	}

	// Initialize counter at 1
	initCypher := "CREATE (m:Marker {key: $key, value: $value})"
	initParams := map[string]any{
		"key":   serialCounterKey,
		"value": int64(1),
	}

	_, err = n.ExecuteWrite(ctx, initCypher, initParams)
	if err != nil {
		return fmt.Errorf("failed to initialize serial counter: %w", err)
	}

	n.Logger.Infof("initialized serial counter")
	return nil
}
