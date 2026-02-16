package neo4j

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// TestConcurrentDriverConnections verifies that three independent Neo4j driver
// instances can execute Cypher queries concurrently without conflict.
// This confirms that multiple processes in the same environment can share
// the bolt://localhost:7687 endpoint.
func TestConcurrentDriverConnections(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	const numDrivers = 3

	// Create independent drivers using the same connection URI as the test DB
	drivers := make([]neo4j.DriverWithContext, numDrivers)
	for i := range drivers {
		driver, err := neo4j.NewDriverWithContext(
			testDB.neo4jURI,
			neo4j.BasicAuth(testDB.neo4jUser, testDB.neo4jPassword, ""),
			func(config *neo4j.Config) {
				config.MaxConnectionPoolSize = 5
			},
		)
		if err != nil {
			t.Fatalf("failed to create driver %d: %v", i, err)
		}
		defer driver.Close(context.Background())

		if err := driver.VerifyConnectivity(context.Background()); err != nil {
			t.Fatalf("driver %d failed connectivity check: %v", i, err)
		}
		drivers[i] = driver
	}

	// Run concurrent read and write queries across all three drivers
	var wg sync.WaitGroup
	errs := make(chan error, numDrivers)

	for i, driver := range drivers {
		wg.Add(1)
		go func(id int, d neo4j.DriverWithContext) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Write: create a uniquely-labeled node
			label := fmt.Sprintf("ConcurrentTest_%d", id)
			writeSession := d.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
			_, err := writeSession.Run(ctx,
				fmt.Sprintf("CREATE (n:%s {driver: $id, ts: timestamp()}) RETURN n", label),
				map[string]any{"id": id},
			)
			writeSession.Close(ctx)
			if err != nil {
				errs <- fmt.Errorf("driver %d write failed: %w", id, err)
				return
			}

			// Read: count nodes with our label
			readSession := d.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
			result, err := readSession.Run(ctx,
				fmt.Sprintf("MATCH (n:%s) RETURN count(n) AS cnt", label),
				nil,
			)
			if err != nil {
				readSession.Close(ctx)
				errs <- fmt.Errorf("driver %d read failed: %w", id, err)
				return
			}

			if !result.Next(ctx) {
				readSession.Close(ctx)
				errs <- fmt.Errorf("driver %d read returned no results", id)
				return
			}
			cnt, ok := result.Record().Values[0].(int64)
			readSession.Close(ctx)
			if !ok || cnt < 1 {
				errs <- fmt.Errorf("driver %d expected count >= 1, got %v", id, cnt)
				return
			}

			// Cleanup
			cleanupSession := d.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
			_, _ = cleanupSession.Run(ctx,
				fmt.Sprintf("MATCH (n:%s) DETACH DELETE n", label),
				nil,
			)
			cleanupSession.Close(ctx)
		}(i, driver)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}
