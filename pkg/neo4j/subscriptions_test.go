//go:build integration
// +build integration

package neo4j

import (
	"testing"

	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// Note: WebSocket subscription management (AddSubscription, GetSubscriptionCount,
// RemoveSubscription, ClearSubscriptions) is handled at the app layer, not the
// database layer. Tests for those methods have been removed.

// All tests in this file use the shared testDB instance from testmain_test.go
// to avoid Neo4j authentication rate limiting from too many connections.

func TestMarkers_SetGetDelete(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	// Set a marker
	key := "test-marker"
	value := []byte("test-value-123")
	if err := testDB.SetMarker(key, value); err != nil {
		t.Fatalf("Failed to set marker: %v", err)
	}

	// Get the marker
	retrieved, err := testDB.GetMarker(key)
	if err != nil {
		t.Fatalf("Failed to get marker: %v", err)
	}
	if string(retrieved) != string(value) {
		t.Fatalf("Marker value mismatch: got %s, expected %s", string(retrieved), string(value))
	}

	// Update the marker
	newValue := []byte("updated-value")
	if err := testDB.SetMarker(key, newValue); err != nil {
		t.Fatalf("Failed to update marker: %v", err)
	}

	retrieved, err = testDB.GetMarker(key)
	if err != nil {
		t.Fatalf("Failed to get updated marker: %v", err)
	}
	if string(retrieved) != string(newValue) {
		t.Fatalf("Updated marker value mismatch")
	}

	// Delete the marker
	if err := testDB.DeleteMarker(key); err != nil {
		t.Fatalf("Failed to delete marker: %v", err)
	}

	// Verify marker is deleted
	_, err = testDB.GetMarker(key)
	if err == nil {
		t.Fatal("Expected error when getting deleted marker")
	}

	t.Logf("✓ Markers set/get/delete works correctly")
}

func TestMarkers_GetNonExistent(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Try to get non-existent marker (don't wipe - just test non-existent key)
	_, err := testDB.GetMarker("non-existent-marker-unique-12345")
	if err == nil {
		t.Fatal("Expected error when getting non-existent marker")
	}

	t.Logf("✓ Getting non-existent marker returns error as expected")
}

func TestSerial_GetNextSerial(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Get first serial
	serial1, err := testDB.getNextSerial()
	if err != nil {
		t.Fatalf("Failed to get first serial: %v", err)
	}

	// Get second serial
	serial2, err := testDB.getNextSerial()
	if err != nil {
		t.Fatalf("Failed to get second serial: %v", err)
	}

	// Serial should increment
	if serial2 <= serial1 {
		t.Fatalf("Expected serial to increment: serial1=%d, serial2=%d", serial1, serial2)
	}

	// Get multiple more serials and verify they're all unique and increasing
	var serials []uint64
	for i := 0; i < 10; i++ {
		s, err := testDB.getNextSerial()
		if err != nil {
			t.Fatalf("Failed to get serial %d: %v", i, err)
		}
		serials = append(serials, s)
	}

	for i := 1; i < len(serials); i++ {
		if serials[i] <= serials[i-1] {
			t.Fatalf("Serials not increasing: %d <= %d", serials[i], serials[i-1])
		}
	}

	t.Logf("✓ Serial generation works correctly (generated %d unique serials)", len(serials)+2)
}

func TestDatabaseReady(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Database should already be ready (testDB is initialized in TestMain)
	select {
	case <-testDB.Ready():
		t.Logf("✓ Database ready signal works correctly")
	default:
		t.Fatal("Expected database to be ready")
	}
}

func TestIdentity(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	// Get identity (creates if not exists)
	secret1, err := testDB.GetOrCreateRelayIdentitySecret()
	if err != nil {
		t.Fatalf("Failed to get identity: %v", err)
	}
	if secret1 == nil {
		t.Fatal("Expected non-nil secret from GetOrCreateRelayIdentitySecret()")
	}

	// Get identity again (should return same one)
	secret2, err := testDB.GetOrCreateRelayIdentitySecret()
	if err != nil {
		t.Fatalf("Failed to get identity second time: %v", err)
	}
	if secret2 == nil {
		t.Fatal("Expected non-nil secret from second GetOrCreateRelayIdentitySecret() call")
	}

	// Secrets should match
	if len(secret1) != len(secret2) {
		t.Fatalf("Secret lengths don't match: %d vs %d", len(secret1), len(secret2))
	}
	for i := range secret1 {
		if secret1[i] != secret2[i] {
			t.Fatal("Identity secrets don't match across calls")
		}
	}

	t.Logf("✓ Identity persistence works correctly")
}

func TestWipe(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	signer, _ := p8k.New()
	signer.Generate()

	// Add some data
	if err := testDB.AddNIP43Member(signer.Pub(), "test"); err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// Wipe the database
	if err := testDB.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Verify data is gone
	isMember, _ := testDB.IsNIP43Member(signer.Pub())
	if isMember {
		t.Fatal("Expected data to be wiped")
	}

	t.Logf("✓ Wipe clears database correctly")
}
