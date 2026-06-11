package neo4j

import (
	"context"
	"fmt"
)

// Serial number management
// We use a special Marker node in Neo4j to track the next available serial number

const serialCounterKey = "serial_counter"

// --- Actor request/response types ---

type serialNextReq struct {
	n    *N
	resp chan serialNextResp
}

type serialNextResp struct {
	serial uint64
	err    error
}

// serialActor serializes serial number generation.
// All serial requests go through this channel.
var serialCh = make(chan serialNextReq)

func init() {
	go serialActorLoop()
}

func serialActorLoop() {
	for req := range serialCh {
		serial, err := req.n.getNextSerialInternal()
		req.resp <- serialNextResp{serial: serial, err: err}
	}
}

// getNextSerial atomically increments and returns the next serial number
func (n *N) getNextSerial() (uint64, error) {
	req := serialNextReq{n: n, resp: make(chan serialNextResp, 1)}
	serialCh <- req
	r := <-req.resp
	return r.serial, r.err
}

// getNextSerialInternal performs the actual serial generation.
// Must only be called from the serialActorLoop.
func (n *N) getNextSerialInternal() (uint64, error) {
	ctx := context.Background()

	// Query current serial value
	cypher := "MATCH (m:Marker {key: $key}) RETURN m.value AS value"
	params := map[string]any{"key": serialCounterKey}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return 0, fmt.Errorf("failed to query serial counter: %w", err)
	}

	var currentSerial uint64 = 1
	if result.Next(ctx) {
		record := result.Record()
		if record != nil {
			valueRaw, found := record.Get("value")
			if found {
				if value, ok := valueRaw.(int64); ok {
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

	initCypher := `
MERGE (m:Marker {key: $key})
ON CREATE SET m.value = $value`
	initParams := map[string]any{
		"key":   serialCounterKey,
		"value": int64(1),
	}

	_, err := n.ExecuteWrite(ctx, initCypher, initParams)
	if err != nil {
		return fmt.Errorf("failed to initialize serial counter: %w", err)
	}

	n.Logger.Debugf("serial counter initialized/verified")
	return nil
}
