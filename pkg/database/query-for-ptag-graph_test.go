package database

import (
	"context"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

func TestCanUsePTagGraph(t *testing.T) {
	tests := []struct {
		name     string
		filter   *filter.F
		expected bool
	}{
		{
			name: "filter with p-tags only",
			filter: &filter.F{
				Tags: tag.NewS(
					tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000001"),
				),
			},
			expected: true,
		},
		{
			name: "filter with p-tags and kinds",
			filter: &filter.F{
				Kinds: kind.NewS(kind.New(1)),
				Tags: tag.NewS(
					tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000001"),
				),
			},
			expected: true,
		},
		{
			name: "filter with p-tags and authors (should use traditional index)",
			filter: &filter.F{
				Authors: tag.NewFromBytesSlice([]byte("author")),
				Tags: tag.NewS(
					tag.NewFromAny("p", "0000000000000000000000000000000000000000000000000000000000000001"),
				),
			},
			expected: false,
		},
		{
			name: "filter with e-tags only (no p-tags)",
			filter: &filter.F{
				Tags: tag.NewS(
					tag.NewFromAny("e", "someeventid"),
				),
			},
			expected: false,
		},
		{
			name:     "filter with no tags",
			filter:   &filter.F{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanUsePTagGraph(tt.filter)
			if result != tt.expected {
				t.Errorf("CanUsePTagGraph() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQueryPTagGraph(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test events with p-tags
	authorPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	alicePubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	bobPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")

	// Event 1: kind-1 (text note) mentioning Alice
	eventID1 := make([]byte, 32)
	eventID1[0] = 1
	eventSig1 := make([]byte, 64)
	eventSig1[0] = 1

	ev1 := &event.E{
		ID:        eventID1,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("Mentioning Alice"),
		Sig:       eventSig1,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(alicePubkey)),
		),
	}

	// Event 2: kind-6 (repost) mentioning Alice
	eventID2 := make([]byte, 32)
	eventID2[0] = 2
	eventSig2 := make([]byte, 64)
	eventSig2[0] = 2

	ev2 := &event.E{
		ID:        eventID2,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567891,
		Kind:      6,
		Content:   []byte("Reposting Alice"),
		Sig:       eventSig2,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(alicePubkey)),
		),
	}

	// Event 3: kind-1 mentioning Bob
	eventID3 := make([]byte, 32)
	eventID3[0] = 3
	eventSig3 := make([]byte, 64)
	eventSig3[0] = 3

	ev3 := &event.E{
		ID:        eventID3,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567892,
		Kind:      1,
		Content:   []byte("Mentioning Bob"),
		Sig:       eventSig3,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(bobPubkey)),
		),
	}

	// Save all events
	if _, err := db.SaveEvent(ctx, ev1); err != nil {
		t.Fatalf("Failed to save event 1: %v", err)
	}
	if _, err := db.SaveEvent(ctx, ev2); err != nil {
		t.Fatalf("Failed to save event 2: %v", err)
	}
	if _, err := db.SaveEvent(ctx, ev3); err != nil {
		t.Fatalf("Failed to save event 3: %v", err)
	}

	// Test 1: Query for all events mentioning Alice
	t.Run("query for Alice mentions", func(t *testing.T) {
		f := &filter.F{
			Tags: tag.NewS(
				tag.NewFromAny("p", hex.Enc(alicePubkey)),
			),
		}

		sers, err := db.QueryPTagGraph(f)
		if err != nil {
			t.Fatalf("QueryPTagGraph failed: %v", err)
		}

		if len(sers) != 2 {
			t.Errorf("Expected 2 events mentioning Alice, got %d", len(sers))
		}
		t.Logf("Found %d events mentioning Alice", len(sers))
	})

	// Test 2: Query for kind-1 events mentioning Alice
	t.Run("query for kind-1 Alice mentions", func(t *testing.T) {
		f := &filter.F{
			Kinds: kind.NewS(kind.New(1)),
			Tags: tag.NewS(
				tag.NewFromAny("p", hex.Enc(alicePubkey)),
			),
		}

		sers, err := db.QueryPTagGraph(f)
		if err != nil {
			t.Fatalf("QueryPTagGraph failed: %v", err)
		}

		if len(sers) != 1 {
			t.Errorf("Expected 1 kind-1 event mentioning Alice, got %d", len(sers))
		}
		t.Logf("Found %d kind-1 events mentioning Alice", len(sers))
	})

	// Test 3: Query for events mentioning Bob
	t.Run("query for Bob mentions", func(t *testing.T) {
		f := &filter.F{
			Tags: tag.NewS(
				tag.NewFromAny("p", hex.Enc(bobPubkey)),
			),
		}

		sers, err := db.QueryPTagGraph(f)
		if err != nil {
			t.Fatalf("QueryPTagGraph failed: %v", err)
		}

		if len(sers) != 1 {
			t.Errorf("Expected 1 event mentioning Bob, got %d", len(sers))
		}
		t.Logf("Found %d events mentioning Bob", len(sers))
	})

	// Test 4: Query for non-existent pubkey
	t.Run("query for non-existent pubkey", func(t *testing.T) {
		nonExistentPubkey := make([]byte, 32)
		for i := range nonExistentPubkey {
			nonExistentPubkey[i] = 0xFF
		}

		f := &filter.F{
			Tags: tag.NewS(
				tag.NewFromAny("p", hex.Enc(nonExistentPubkey)),
			),
		}

		sers, err := db.QueryPTagGraph(f)
		if err != nil {
			t.Fatalf("QueryPTagGraph failed: %v", err)
		}

		if len(sers) != 0 {
			t.Errorf("Expected 0 events for non-existent pubkey, got %d", len(sers))
		}
		t.Logf("Correctly found 0 events for non-existent pubkey")
	})

	// Test 5: Query for multiple kinds
	t.Run("query for multiple kinds mentioning Alice", func(t *testing.T) {
		f := &filter.F{
			Kinds: kind.NewS(kind.New(1), kind.New(6)),
			Tags: tag.NewS(
				tag.NewFromAny("p", hex.Enc(alicePubkey)),
			),
		}

		sers, err := db.QueryPTagGraph(f)
		if err != nil {
			t.Fatalf("QueryPTagGraph failed: %v", err)
		}

		if len(sers) != 2 {
			t.Errorf("Expected 2 events (kind 1 and 6) mentioning Alice, got %d", len(sers))
		}
		t.Logf("Found %d events (kind 1 and 6) mentioning Alice", len(sers))
	})
}

func TestGetSerialsFromFilterWithPTagOptimization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test event with p-tag
	authorPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	alicePubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")

	eventID := make([]byte, 32)
	eventID[0] = 1
	eventSig := make([]byte, 64)
	eventSig[0] = 1

	ev := &event.E{
		ID:        eventID,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("Mentioning Alice"),
		Sig:       eventSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(alicePubkey)),
		),
	}

	if _, err := db.SaveEvent(ctx, ev); err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Test that GetSerialsFromFilter uses the p-tag graph optimization
	f := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(alicePubkey)),
		),
	}

	sers, err := db.GetSerialsFromFilter(f)
	if err != nil {
		t.Fatalf("GetSerialsFromFilter failed: %v", err)
	}

	if len(sers) != 1 {
		t.Errorf("Expected 1 event, got %d", len(sers))
	}

	t.Logf("GetSerialsFromFilter successfully used p-tag graph optimization, found %d events", len(sers))
}
