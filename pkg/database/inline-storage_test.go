package database

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// TestInlineSmallEventStorage tests the Reiser4-inspired inline storage optimization
// for small events (<=1024 bytes by default).
func TestInlineSmallEventStorage(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-inline-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a signer
	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Test Case 1: Small event (should use inline storage)
	t.Run("SmallEventInlineStorage", func(t *testing.T) {
		smallEvent := event.New()
		smallEvent.Kind = kind.TextNote.K
		smallEvent.CreatedAt = timestamp.Now().V
		smallEvent.Content = []byte("Hello Nostr!") // Small content
		smallEvent.Pubkey = sign.Pub()
		smallEvent.Tags = tag.NewS()

		// Sign the event
		if err := smallEvent.Sign(sign); err != nil {
			t.Fatalf("Failed to sign small event: %v", err)
		}

		// Save the event
		if _, err := db.SaveEvent(ctx, smallEvent); err != nil {
			t.Fatalf("Failed to save small event: %v", err)
		}

		// Verify it was stored with sev prefix
		serial, err := db.GetSerialById(smallEvent.ID)
		if err != nil {
			t.Fatalf("Failed to get serial for small event: %v", err)
		}

		// Check that sev key exists
		sevKeyExists := false
		db.View(func(txn *badger.Txn) error {
			smallBuf := new(bytes.Buffer)
			indexes.SmallEventEnc(serial).MarshalWrite(smallBuf)

			opts := badger.DefaultIteratorOptions
			opts.Prefix = smallBuf.Bytes()
			it := txn.NewIterator(opts)
			defer it.Close()

			it.Rewind()
			if it.Valid() {
				sevKeyExists = true
			}
			return nil
		})

		if !sevKeyExists {
			t.Errorf("Small event was not stored with sev prefix")
		}

		// Verify evt key does NOT exist for small event
		evtKeyExists := false
		db.View(func(txn *badger.Txn) error {
			buf := new(bytes.Buffer)
			indexes.EventEnc(serial).MarshalWrite(buf)

			_, err := txn.Get(buf.Bytes())
			if err == nil {
				evtKeyExists = true
			}
			return nil
		})

		if evtKeyExists {
			t.Errorf("Small event should not have evt key (should only use sev)")
		}

		// Fetch and verify the event
		fetchedEvent, err := db.FetchEventBySerial(serial)
		if err != nil {
			t.Fatalf("Failed to fetch small event: %v", err)
		}

		if !bytes.Equal(fetchedEvent.ID, smallEvent.ID) {
			t.Errorf("Fetched event ID mismatch: got %x, want %x", fetchedEvent.ID, smallEvent.ID)
		}
		if !bytes.Equal(fetchedEvent.Content, smallEvent.Content) {
			t.Errorf("Fetched event content mismatch: got %q, want %q", fetchedEvent.Content, smallEvent.Content)
		}
	})

	// Test Case 2: Large event (should use traditional storage)
	t.Run("LargeEventTraditionalStorage", func(t *testing.T) {
		largeEvent := event.New()
		largeEvent.Kind = kind.TextNote.K
		largeEvent.CreatedAt = timestamp.Now().V
		// Create content larger than 1024 bytes (the default inline storage threshold)
		largeContent := make([]byte, 1500)
		for i := range largeContent {
			largeContent[i] = 'x'
		}
		largeEvent.Content = largeContent
		largeEvent.Pubkey = sign.Pub()
		largeEvent.Tags = tag.NewS()

		// Sign the event
		if err := largeEvent.Sign(sign); err != nil {
			t.Fatalf("Failed to sign large event: %v", err)
		}

		// Save the event
		if _, err := db.SaveEvent(ctx, largeEvent); err != nil {
			t.Fatalf("Failed to save large event: %v", err)
		}

		// Verify it was stored with evt prefix
		serial, err := db.GetSerialById(largeEvent.ID)
		if err != nil {
			t.Fatalf("Failed to get serial for large event: %v", err)
		}

		// Check that evt key exists
		evtKeyExists := false
		db.View(func(txn *badger.Txn) error {
			buf := new(bytes.Buffer)
			indexes.EventEnc(serial).MarshalWrite(buf)

			_, err := txn.Get(buf.Bytes())
			if err == nil {
				evtKeyExists = true
			}
			return nil
		})

		if !evtKeyExists {
			t.Errorf("Large event was not stored with evt prefix")
		}

		// Fetch and verify the event
		fetchedEvent, err := db.FetchEventBySerial(serial)
		if err != nil {
			t.Fatalf("Failed to fetch large event: %v", err)
		}

		if !bytes.Equal(fetchedEvent.ID, largeEvent.ID) {
			t.Errorf("Fetched event ID mismatch: got %x, want %x", fetchedEvent.ID, largeEvent.ID)
		}
	})

	// Test Case 3: Batch fetch with mixed small and large events
	t.Run("BatchFetchMixedEvents", func(t *testing.T) {
		var serials []*types.Uint40
		expectedIDs := make(map[uint64][]byte)

		// Create 10 small events and 10 large events
		for i := 0; i < 20; i++ {
			ev := event.New()
			ev.Kind = kind.TextNote.K
			ev.CreatedAt = timestamp.Now().V + int64(i)
			ev.Pubkey = sign.Pub()
			ev.Tags = tag.NewS()

			// Alternate between small and large
			if i%2 == 0 {
				ev.Content = []byte("Small event")
			} else {
				largeContent := make([]byte, 500)
				for j := range largeContent {
					largeContent[j] = 'x'
				}
				ev.Content = largeContent
			}

			if err := ev.Sign(sign); err != nil {
				t.Fatalf("Failed to sign event %d: %v", i, err)
			}

			if _, err := db.SaveEvent(ctx, ev); err != nil {
				t.Fatalf("Failed to save event %d: %v", i, err)
			}

			serial, err := db.GetSerialById(ev.ID)
			if err != nil {
				t.Fatalf("Failed to get serial for event %d: %v", i, err)
			}

			serials = append(serials, serial)
			expectedIDs[serial.Get()] = ev.ID
		}

		// Batch fetch all events
		events, err := db.FetchEventsBySerials(serials)
		if err != nil {
			t.Fatalf("Failed to batch fetch events: %v", err)
		}

		if len(events) != 20 {
			t.Errorf("Expected 20 events, got %d", len(events))
		}

		// Verify all events were fetched correctly
		for serialValue, ev := range events {
			expectedID := expectedIDs[serialValue]
			if !bytes.Equal(ev.ID, expectedID) {
				t.Errorf("Event ID mismatch for serial %d: got %x, want %x",
					serialValue, ev.ID, expectedID)
			}
		}
	})

	// Test Case 4: Edge case - event near 384 byte threshold
	t.Run("ThresholdEvent", func(t *testing.T) {
		ev := event.New()
		ev.Kind = kind.TextNote.K
		ev.CreatedAt = timestamp.Now().V
		ev.Pubkey = sign.Pub()
		ev.Tags = tag.NewS()

		// Create content near the threshold
		testContent := make([]byte, 250)
		for i := range testContent {
			testContent[i] = 'x'
		}
		ev.Content = testContent

		if err := ev.Sign(sign); err != nil {
			t.Fatalf("Failed to sign threshold event: %v", err)
		}

		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("Failed to save threshold event: %v", err)
		}

		serial, err := db.GetSerialById(ev.ID)
		if err != nil {
			t.Fatalf("Failed to get serial: %v", err)
		}

		// Fetch and verify
		fetchedEvent, err := db.FetchEventBySerial(serial)
		if err != nil {
			t.Fatalf("Failed to fetch threshold event: %v", err)
		}

		if !bytes.Equal(fetchedEvent.ID, ev.ID) {
			t.Errorf("Fetched event ID mismatch")
		}
	})
}

// TestInlineStorageMigration tests the migration from traditional to inline storage
func TestInlineStorageMigration(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-migration-db-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create a signer
	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Manually set database version to 3 (before inline storage migration)
	db.writeVersionTag(3)

	// Create and save some small events the old way (manually)
	var testEvents []*event.E
	for i := 0; i < 5; i++ {
		ev := event.New()
		ev.Kind = kind.TextNote.K
		ev.CreatedAt = timestamp.Now().V + int64(i)
		ev.Content = []byte("Test event")
		ev.Pubkey = sign.Pub()
		ev.Tags = tag.NewS()

		if err := ev.Sign(sign); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		// Get next serial
		serial, err := db.seq.Next()
		if err != nil {
			t.Fatalf("Failed to get serial: %v", err)
		}

		// Generate indexes
		idxs, err := GetIndexesForEvent(ev, serial)
		if err != nil {
			t.Fatalf("Failed to generate indexes: %v", err)
		}

		// Serialize event
		eventDataBuf := new(bytes.Buffer)
		ev.MarshalBinary(eventDataBuf)
		eventData := eventDataBuf.Bytes()

		// Save the old way (evt prefix with value)
		db.Update(func(txn *badger.Txn) error {
			ser := new(types.Uint40)
			ser.Set(serial)

			// Save indexes
			for _, key := range idxs {
				txn.Set(key, nil)
			}

			// Save event the old way
			keyBuf := new(bytes.Buffer)
			indexes.EventEnc(ser).MarshalWrite(keyBuf)
			txn.Set(keyBuf.Bytes(), eventData)

			return nil
		})

		testEvents = append(testEvents, ev)
	}

	t.Logf("Created %d test events with old storage format", len(testEvents))

	// Close and reopen database to trigger migration
	db.Close()

	db, err = New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db.Close()

	// Give migration time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify all events can still be fetched
	for i, ev := range testEvents {
		serial, err := db.GetSerialById(ev.ID)
		if err != nil {
			t.Fatalf("Failed to get serial for event %d after migration: %v", i, err)
		}

		fetchedEvent, err := db.FetchEventBySerial(serial)
		if err != nil {
			t.Fatalf("Failed to fetch event %d after migration: %v", i, err)
		}

		if !bytes.Equal(fetchedEvent.ID, ev.ID) {
			t.Errorf("Event %d ID mismatch after migration: got %x, want %x",
				i, fetchedEvent.ID, ev.ID)
		}

		if !bytes.Equal(fetchedEvent.Content, ev.Content) {
			t.Errorf("Event %d content mismatch after migration: got %q, want %q",
				i, fetchedEvent.Content, ev.Content)
		}

		// Verify it's now using inline storage
		sevKeyExists := false
		db.View(func(txn *badger.Txn) error {
			smallBuf := new(bytes.Buffer)
			indexes.SmallEventEnc(serial).MarshalWrite(smallBuf)

			opts := badger.DefaultIteratorOptions
			opts.Prefix = smallBuf.Bytes()
			it := txn.NewIterator(opts)
			defer it.Close()

			it.Rewind()
			if it.Valid() {
				sevKeyExists = true
				t.Logf("Event %d (%s) successfully migrated to inline storage",
					i, hex.Enc(ev.ID[:8]))
			}
			return nil
		})

		if !sevKeyExists {
			t.Errorf("Event %d was not migrated to inline storage", i)
		}
	}
}

// BenchmarkInlineVsTraditionalStorage compares performance of inline vs traditional storage
func BenchmarkInlineVsTraditionalStorage(b *testing.B) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "bench-inline-db-*")
	if err != nil {
		b.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a signer
	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		b.Fatal(err)
	}

	// Pre-populate database with mix of small and large events
	var smallSerials []*types.Uint40
	var largeSerials []*types.Uint40

	for i := 0; i < 100; i++ {
		// Small event
		smallEv := event.New()
		smallEv.Kind = kind.TextNote.K
		smallEv.CreatedAt = timestamp.Now().V + int64(i)*2
		smallEv.Content = []byte("Small test event")
		smallEv.Pubkey = sign.Pub()
		smallEv.Tags = tag.NewS()
		smallEv.Sign(sign)

		db.SaveEvent(ctx, smallEv)
		if serial, err := db.GetSerialById(smallEv.ID); err == nil {
			smallSerials = append(smallSerials, serial)
		}

		// Large event
		largeEv := event.New()
		largeEv.Kind = kind.TextNote.K
		largeEv.CreatedAt = timestamp.Now().V + int64(i)*2 + 1
		largeContent := make([]byte, 500)
		for j := range largeContent {
			largeContent[j] = 'x'
		}
		largeEv.Content = largeContent
		largeEv.Pubkey = sign.Pub()
		largeEv.Tags = tag.NewS()
		largeEv.Sign(sign)

		db.SaveEvent(ctx, largeEv)
		if serial, err := db.GetSerialById(largeEv.ID); err == nil {
			largeSerials = append(largeSerials, serial)
		}
	}

	b.Run("FetchSmallEventsInline", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			idx := i % len(smallSerials)
			db.FetchEventBySerial(smallSerials[idx])
		}
	})

	b.Run("FetchLargeEventsTraditional", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			idx := i % len(largeSerials)
			db.FetchEventBySerial(largeSerials[idx])
		}
	})

	b.Run("BatchFetchSmallEvents", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			db.FetchEventsBySerials(smallSerials[:10])
		}
	})

	b.Run("BatchFetchLargeEvents", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			db.FetchEventsBySerials(largeSerials[:10])
		}
	})
}
