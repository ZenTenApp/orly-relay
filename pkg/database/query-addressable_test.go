//go:build !(js && wasm)

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/lol/chk"
)

func TestIsAddressableEventQuery(t *testing.T) {
	// Generate a test keypair
	signer := p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatal(err)
	}
	pub := signer.Pub()

	tests := []struct {
		name     string
		filter   *filter.F
		expected bool
	}{
		{
			name: "valid NIP-33 query - kind 30000",
			filter: &filter.F{
				Kinds:   kind.NewS(kind.New(30000)),
				Authors: tag.NewFromBytesSlice(pub),
				Tags:    tag.NewS(tag.NewFromAny("#d", []byte("test-d-tag"))),
			},
			expected: true,
		},
		{
			name: "valid NIP-33 query - kind 30382",
			filter: &filter.F{
				Kinds:   kind.NewS(kind.New(30382)),
				Authors: tag.NewFromBytesSlice(pub),
				Tags:    tag.NewS(tag.NewFromAny("#d", []byte("some-identifier"))),
			},
			expected: true,
		},
		{
			name: "invalid - kind 1 (not parameterized replaceable)",
			filter: &filter.F{
				Kinds:   kind.NewS(kind.New(1)),
				Authors: tag.NewFromBytesSlice(pub),
				Tags:    tag.NewS(tag.NewFromAny("#d", []byte("test"))),
			},
			expected: false,
		},
		{
			name: "invalid - kind 10000 (replaceable, not parameterized)",
			filter: &filter.F{
				Kinds:   kind.NewS(kind.New(10000)),
				Authors: tag.NewFromBytesSlice(pub),
				Tags:    tag.NewS(tag.NewFromAny("#d", []byte("test"))),
			},
			expected: false,
		},
		{
			name: "invalid - missing d-tag",
			filter: &filter.F{
				Kinds:   kind.NewS(kind.New(30000)),
				Authors: tag.NewFromBytesSlice(pub),
			},
			expected: false,
		},
		{
			name: "invalid - multiple kinds",
			filter: &filter.F{
				Kinds:   kind.NewS(kind.New(30000), kind.New(30001)),
				Authors: tag.NewFromBytesSlice(pub),
				Tags:    tag.NewS(tag.NewFromAny("#d", []byte("test"))),
			},
			expected: false,
		},
		{
			name: "invalid - no authors",
			filter: &filter.F{
				Kinds: kind.NewS(kind.New(30000)),
				Tags:  tag.NewS(tag.NewFromAny("#d", []byte("test"))),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAddressableEventQuery(tt.filter)
			if result != tt.expected {
				t.Errorf("IsAddressableEventQuery() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQueryForAddressableEvent(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "test-addressable-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Generate a test keypair
	signer := p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatal(err)
	}
	pub := signer.Pub()

	// Create and save a parameterized replaceable event (kind 30382)
	dTagValue := []byte("test-identifier-12345")
	ev := &event.E{
		Kind:      30382,
		Pubkey:    pub,
		CreatedAt: time.Now().Unix(),
		Content:   []byte("Test content for addressable event"),
		Tags:      tag.NewS(tag.NewFromAny("d", dTagValue)),
	}

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Save the event
	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("failed to save event: %v", err)
	}

	// Query using the fast path
	queryFilter := &filter.F{
		Kinds:   kind.NewS(kind.New(30382)),
		Authors: tag.NewFromBytesSlice(pub),
		Tags:    tag.NewS(tag.NewFromAny("#d", dTagValue)),
	}

	// Test IsAddressableEventQuery
	if !IsAddressableEventQuery(queryFilter) {
		t.Errorf("Expected IsAddressableEventQuery to return true for valid NIP-33 filter")
	}

	// Test QueryForAddressableEvent
	serial, err := db.QueryForAddressableEvent(queryFilter)
	if err != nil {
		t.Fatalf("QueryForAddressableEvent failed: %v", err)
	}
	if serial == nil {
		t.Fatalf("QueryForAddressableEvent returned nil serial, expected to find event")
	}

	// Fetch the event and verify it matches
	fetchedEv, err := db.FetchEventBySerial(serial)
	if err != nil {
		t.Fatalf("FetchEventBySerial failed: %v", err)
	}
	if fetchedEv == nil {
		t.Fatalf("FetchEventBySerial returned nil event")
	}

	// Verify it's the same event
	if string(fetchedEv.ID[:]) != string(ev.ID[:]) {
		t.Errorf("Fetched event ID doesn't match: got %x, want %x", fetchedEv.ID, ev.ID)
	}

	// Test that QueryEvents also uses the fast path
	evs, err := db.QueryEvents(ctx, queryFilter)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("QueryEvents returned %d events, expected 1", len(evs))
	}
	if string(evs[0].ID[:]) != string(ev.ID[:]) {
		t.Errorf("QueryEvents returned wrong event: got %x, want %x", evs[0].ID, ev.ID)
	}

	t.Logf("Successfully queried addressable event via fast path: kind=%d, d=%s", ev.Kind, string(dTagValue))
}

func TestQueryForAddressableEventNotFound(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "test-addressable-notfound-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Generate a test keypair
	signer := p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatal(err)
	}
	pub := signer.Pub()

	// Query for non-existent event
	queryFilter := &filter.F{
		Kinds:   kind.NewS(kind.New(30000)),
		Authors: tag.NewFromBytesSlice(pub),
		Tags:    tag.NewS(tag.NewFromAny("#d", []byte("non-existent-d-tag"))),
	}

	serial, err := db.QueryForAddressableEvent(queryFilter)
	if err != nil {
		t.Fatalf("QueryForAddressableEvent failed: %v", err)
	}
	if serial != nil {
		t.Errorf("Expected nil serial for non-existent event, got %d", serial.Get())
	}
}

func TestAddressableEventReplacement(t *testing.T) {
	// Create temporary database
	tempDir, err := os.MkdirTemp("", "test-addressable-replace-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Generate a test keypair
	signer := p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatal(err)
	}
	pub := signer.Pub()

	dTagValue := []byte("replaceable-event")
	baseTime := time.Now().Unix()

	// Create and save the first event
	ev1 := &event.E{
		Kind:      30000,
		Pubkey:    pub,
		CreatedAt: baseTime,
		Content:   []byte("First version"),
		Tags:      tag.NewS(tag.NewFromAny("d", dTagValue)),
	}
	if err := ev1.Sign(signer); err != nil {
		t.Fatalf("failed to sign event 1: %v", err)
	}
	if _, err := db.SaveEvent(ctx, ev1); err != nil {
		t.Fatalf("failed to save event 1: %v", err)
	}

	// Create and save a newer replacement event
	ev2 := &event.E{
		Kind:      30000,
		Pubkey:    pub,
		CreatedAt: baseTime + 1000, // Newer
		Content:   []byte("Second version - replacement"),
		Tags:      tag.NewS(tag.NewFromAny("d", dTagValue)),
	}
	if err := ev2.Sign(signer); err != nil {
		t.Fatalf("failed to sign event 2: %v", err)
	}
	if _, err := db.SaveEvent(ctx, ev2); err != nil {
		t.Fatalf("failed to save event 2: %v", err)
	}

	// Query and verify we get the newer event via fast path
	queryFilter := &filter.F{
		Kinds:   kind.NewS(kind.New(30000)),
		Authors: tag.NewFromBytesSlice(pub),
		Tags:    tag.NewS(tag.NewFromAny("#d", dTagValue)),
	}

	serial, err := db.QueryForAddressableEvent(queryFilter)
	if err != nil {
		t.Fatalf("QueryForAddressableEvent failed: %v", err)
	}
	if serial == nil {
		t.Fatalf("QueryForAddressableEvent returned nil serial")
	}

	fetchedEv, err := db.FetchEventBySerial(serial)
	if err != nil {
		t.Fatalf("FetchEventBySerial failed: %v", err)
	}

	// Should be the second (newer) event
	if string(fetchedEv.ID[:]) != string(ev2.ID[:]) {
		t.Errorf("Expected to get newer event (ev2), got different event")
	}
	if string(fetchedEv.Content) != "Second version - replacement" {
		t.Errorf("Expected content 'Second version - replacement', got '%s'", string(fetchedEv.Content))
	}

	t.Logf("Replacement event correctly indexed: %x", ev2.ID)
}
