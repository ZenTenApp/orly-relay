package database

import (
	"bufio"
	"bytes"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"lol.mleku.dev/chk"
)

// TestExport tests the Export function by:
// 1. Using the shared database with events from examples.Cache
// 2. Checking that events can be exported
// 3. Verifying the exported events can be parsed
func TestExport(t *testing.T) {
	// Use shared database (skips in short mode)
	db, ctx := GetSharedDB(t)
	savedEvents := GetSharedEvents(t)

	t.Logf("Shared database has %d events", len(savedEvents))

	// Test 1: Export all events and verify they can be parsed
	var exportBuffer bytes.Buffer
	db.Export(ctx, &exportBuffer)

	// Parse the exported events and count them
	exportedIDs := make(map[string]bool)
	exportScanner := bufio.NewScanner(&exportBuffer)
	exportScanner.Buffer(make([]byte, 0, 1_000_000_000), 1_000_000_000)
	exportCount := 0
	for exportScanner.Scan() {
		b := exportScanner.Bytes()
		ev := event.New()
		if _, err := ev.Unmarshal(b); chk.E(err) {
			t.Fatal(err)
		}
		exportedIDs[string(ev.ID)] = true
		exportCount++
	}
	// Check for scanner errors
	if err := exportScanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	t.Logf("Found %d events in the export", exportCount)

	// Verify we exported a reasonable number of events
	if exportCount == 0 {
		t.Fatal("Export returned no events")
	}
}
