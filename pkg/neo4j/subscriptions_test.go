package neo4j

import (
	"context"
	"os"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func TestSubscriptions_AddAndRemove(t *testing.T) {
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

	// Create a subscription
	subID := "test-sub-123"
	f := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	}

	// Add subscription
	db.AddSubscription(subID, f)

	// Get subscription count (should be 1)
	count := db.GetSubscriptionCount()
	if count != 1 {
		t.Fatalf("Expected 1 subscription, got %d", count)
	}

	// Remove subscription
	db.RemoveSubscription(subID)

	// Get subscription count (should be 0)
	count = db.GetSubscriptionCount()
	if count != 0 {
		t.Fatalf("Expected 0 subscriptions after removal, got %d", count)
	}

	t.Logf("✓ Subscription add/remove works correctly")
}

func TestSubscriptions_MultipleSubscriptions(t *testing.T) {
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

	// Add multiple subscriptions
	for i := 0; i < 5; i++ {
		subID := string(rune('A' + i))
		f := &filter.F{
			Kinds: kind.NewS(kind.New(uint16(i + 1))),
		}
		db.AddSubscription(subID, f)
	}

	// Get subscription count
	count := db.GetSubscriptionCount()
	if count != 5 {
		t.Fatalf("Expected 5 subscriptions, got %d", count)
	}

	// Remove some subscriptions
	db.RemoveSubscription("A")
	db.RemoveSubscription("C")

	count = db.GetSubscriptionCount()
	if count != 3 {
		t.Fatalf("Expected 3 subscriptions after removal, got %d", count)
	}

	// Clear all subscriptions
	db.ClearSubscriptions()

	count = db.GetSubscriptionCount()
	if count != 0 {
		t.Fatalf("Expected 0 subscriptions after clear, got %d", count)
	}

	t.Logf("✓ Multiple subscriptions managed correctly")
}

func TestSubscriptions_DuplicateID(t *testing.T) {
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

	subID := "duplicate-test"

	// Add first subscription
	f1 := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
	}
	db.AddSubscription(subID, f1)

	// Add subscription with same ID (should replace)
	f2 := &filter.F{
		Kinds: kind.NewS(kind.New(7)),
	}
	db.AddSubscription(subID, f2)

	// Should still have only 1 subscription
	count := db.GetSubscriptionCount()
	if count != 1 {
		t.Fatalf("Expected 1 subscription (duplicate replaced), got %d", count)
	}

	t.Logf("✓ Duplicate subscription ID handling works correctly")
}

func TestSubscriptions_RemoveNonExistent(t *testing.T) {
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

	// Try to remove non-existent subscription (should not panic)
	db.RemoveSubscription("non-existent")

	// Should still have 0 subscriptions
	count := db.GetSubscriptionCount()
	if count != 0 {
		t.Fatalf("Expected 0 subscriptions, got %d", count)
	}

	t.Logf("✓ Removing non-existent subscription handled gracefully")
}

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

	// Get identity (creates if not exists)
	signer := db.Identity()
	if signer == nil {
		t.Fatal("Expected non-nil signer from Identity()")
	}

	// Get identity again (should return same one)
	signer2 := db.Identity()
	if signer2 == nil {
		t.Fatal("Expected non-nil signer from second Identity() call")
	}

	// Public keys should match
	pub1 := signer.Pub()
	pub2 := signer2.Pub()
	for i := range pub1 {
		if pub1[i] != pub2[i] {
			t.Fatal("Identity pubkeys don't match across calls")
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
