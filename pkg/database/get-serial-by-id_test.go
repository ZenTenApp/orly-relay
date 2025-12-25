package database

import (
	"testing"
)

func TestGetSerialById(t *testing.T) {
	// Use shared database (skips in short mode)
	db, _ := GetSharedDB(t)
	savedEvents := GetSharedEvents(t)

	if len(savedEvents) < 4 {
		t.Fatalf("Need at least 4 saved events, got %d", len(savedEvents))
	}
	testEvent := savedEvents[3]

	// Get the serial by ID
	serial, err := db.GetSerialById(testEvent.ID)
	if err != nil {
		t.Fatalf("Failed to get serial by ID: %v", err)
	}

	// Verify the serial is not nil
	if serial == nil {
		t.Fatal("Expected serial to be non-nil, but got nil")
	}

	// Test with a non-existent ID
	nonExistentId := make([]byte, len(testEvent.ID))
	// Ensure it's different from any real ID
	for i := range nonExistentId {
		nonExistentId[i] = ^testEvent.ID[i]
	}

	serial, err = db.GetSerialById(nonExistentId)
	if err == nil {
		t.Fatalf("Expected error for non-existent ID, but got none")
	}

	// For non-existent Ids, the function should return nil serial and an error
	if serial != nil {
		t.Fatalf("Expected nil serial for non-existent ID, but got: %v", serial)
	}
}
