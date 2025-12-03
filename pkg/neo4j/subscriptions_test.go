package neo4j

import (
	"context"
	"os"
	"testing"

	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// Note: WebSocket subscription management (AddSubscription, GetSubscriptionCount,
// RemoveSubscription, ClearSubscriptions) is handled at the app layer, not the
// database layer. Tests for those methods have been removed.

func TestMarkers_SetGetDelete(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Set a marker
	key := "test-marker"
	value := []byte("test-value-123")
	if err := db.SetMarker(key, value); err != nil {
		t.Fatalf("Failed to set marker: %v", err)
	}

	// Get the marker
	retrieved, err := db.GetMarker(key)
	if err != nil {
		t.Fatalf("Failed to get marker: %v", err)
	}
	if string(retrieved) != string(value) {
		t.Fatalf("Marker value mismatch: got %s, expected %s", string(retrieved), string(value))
	}

	// Update the marker
	newValue := []byte("updated-value")
	if err := db.SetMarker(key, newValue); err != nil {
		t.Fatalf("Failed to update marker: %v", err)
	}

	retrieved, err = db.GetMarker(key)
	if err != nil {
		t.Fatalf("Failed to get updated marker: %v", err)
	}
	if string(retrieved) != string(newValue) {
		t.Fatalf("Updated marker value mismatch")
	}

	// Delete the marker
	if err := db.DeleteMarker(key); err != nil {
		t.Fatalf("Failed to delete marker: %v", err)
	}

	// Verify marker is deleted
	_, err = db.GetMarker(key)
	if err == nil {
		t.Fatal("Expected error when getting deleted marker")
	}

	t.Logf("✓ Markers set/get/delete works correctly")
}

func TestMarkers_GetNonExistent(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Try to get non-existent marker
	_, err = db.GetMarker("non-existent-marker")
	if err == nil {
		t.Fatal("Expected error when getting non-existent marker")
	}

	t.Logf("✓ Getting non-existent marker returns error as expected")
}

func TestSerial_GetNextSerial(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Get first serial
	serial1, err := db.getNextSerial()
	if err != nil {
		t.Fatalf("Failed to get first serial: %v", err)
	}

	// Get second serial
	serial2, err := db.getNextSerial()
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
		s, err := db.getNextSerial()
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
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Wait for ready
	<-db.Ready()

	// Database should be ready now
	t.Logf("✓ Database ready signal works correctly")
}

func TestIdentity(t *testing.T) {
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	// Wipe to ensure clean state
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Get identity (creates if not exists)
	secret1, err := db.GetOrCreateRelayIdentitySecret()
	if err != nil {
		t.Fatalf("Failed to get identity: %v", err)
	}
	if secret1 == nil {
		t.Fatal("Expected non-nil secret from GetOrCreateRelayIdentitySecret()")
	}

	// Get identity again (should return same one)
	secret2, err := db.GetOrCreateRelayIdentitySecret()
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
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		t.Skip("Skipping Neo4j test: ORLY_NEO4J_URI not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempDir := t.TempDir()
	db, err := New(ctx, cancel, tempDir, "debug")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	<-db.Ready()

	signer, _ := p8k.New()
	signer.Generate()

	// Add some data
	if err := db.AddNIP43Member(signer.Pub(), "test"); err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// Wipe the database
	if err := db.Wipe(); err != nil {
		t.Fatalf("Failed to wipe database: %v", err)
	}

	// Verify data is gone
	isMember, _ := db.IsNIP43Member(signer.Pub())
	if isMember {
		t.Fatal("Expected data to be wiped")
	}

	t.Logf("✓ Wipe clears database correctly")
}
