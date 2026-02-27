//go:build integration
// +build integration

// NOTE: This file requires updates to match the current nostr library types.
// The filter/tag/kind types have changed since this test was written.

package neo4j

import (
	"context"
	"testing"

	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
)

// Valid test pubkeys and event IDs (64-character lowercase hex)
const (
	validPubkey1 = "0000000000000000000000000000000000000000000000000000000000000001"
	validPubkey2 = "0000000000000000000000000000000000000000000000000000000000000002"
	validPubkey3 = "0000000000000000000000000000000000000000000000000000000000000003"
	validEventID1 = "1111111111111111111111111111111111111111111111111111111111111111"
	validEventID2 = "2222222222222222222222222222222222222222222222222222222222222222"
	validEventID3 = "3333333333333333333333333333333333333333333333333333333333333333"
)

// TestQueryEventsWithNilFilter tests that QueryEvents handles nil filter fields gracefully
// This test covers the nil pointer fix in query-events.go
func TestQueryEventsWithNilFilter(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	// Setup some test events
	setupTestEvent(t, validEventID1, validPubkey1, 1, "[]")
	setupTestEvent(t, validEventID2, validPubkey2, 1, "[]")

	ctx := context.Background()

	// Test 1: Completely empty filter (all nil fields)
	t.Run("EmptyFilter", func(t *testing.T) {
		f := &filter.F{}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents with empty filter should not panic: %v", err)
		}
		if len(events) == 0 {
			t.Error("Expected to find events with empty filter")
		}
	})

	// Test 2: Filter with nil Ids
	t.Run("NilIds", func(t *testing.T) {
		f := &filter.F{
			Ids: nil, // Explicitly nil
		}
		_, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents with nil Ids should not panic: %v", err)
		}
	})

	// Test 3: Filter with nil Authors
	t.Run("NilAuthors", func(t *testing.T) {
		f := &filter.F{
			Authors: nil, // Explicitly nil
		}
		_, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents with nil Authors should not panic: %v", err)
		}
	})

	// Test 4: Filter with nil Kinds
	t.Run("NilKinds", func(t *testing.T) {
		f := &filter.F{
			Kinds: nil, // Explicitly nil
		}
		_, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents with nil Kinds should not panic: %v", err)
		}
	})

	// Test 5: Filter with empty Ids (using tag with empty slice)
	t.Run("EmptyIds", func(t *testing.T) {
		f := &filter.F{
			Ids: &tag.T{T: [][]byte{}},
		}
		_, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents with empty Ids should not panic: %v", err)
		}
	})

	// Test 6: Filter with empty Authors (using tag with empty slice)
	t.Run("EmptyAuthors", func(t *testing.T) {
		f := &filter.F{
			Authors: &tag.T{T: [][]byte{}},
		}
		_, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents with empty Authors should not panic: %v", err)
		}
	})

	// Test 7: Filter with empty Kinds slice
	t.Run("EmptyKinds", func(t *testing.T) {
		f := &filter.F{
			Kinds: kind.NewS(),
		}
		_, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents with empty Kinds should not panic: %v", err)
		}
	})
}

// TestQueryEventsWithValidFilters tests that QueryEvents works correctly with valid filters
func TestQueryEventsWithValidFilters(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	// Setup test events
	setupTestEvent(t, validEventID1, validPubkey1, 1, "[]")
	setupTestEvent(t, validEventID2, validPubkey2, 3, "[]")
	setupTestEvent(t, validEventID3, validPubkey1, 1, "[]")

	ctx := context.Background()

	// Test 1: Filter by ID
	t.Run("FilterByID", func(t *testing.T) {
		f := &filter.F{
			Ids: tag.NewFromBytesSlice([]byte(validEventID1)),
		}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
	})

	// Test 2: Filter by Author
	t.Run("FilterByAuthor", func(t *testing.T) {
		f := &filter.F{
			Authors: tag.NewFromBytesSlice([]byte(validPubkey1)),
		}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("Expected 2 events from pubkey1, got %d", len(events))
		}
	})

	// Test 3: Filter by Kind
	t.Run("FilterByKind", func(t *testing.T) {
		f := &filter.F{
			Kinds: kind.NewS(kind.New(1)),
		}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("Expected 2 kind-1 events, got %d", len(events))
		}
	})

	// Test 4: Combined filters (kind + author)
	t.Run("FilterByKindAndAuthor", func(t *testing.T) {
		f := &filter.F{
			Kinds:   kind.NewS(kind.New(1)),
			Authors: tag.NewFromBytesSlice([]byte(validPubkey1)),
		}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("Expected 2 kind-1 events from pubkey1, got %d", len(events))
		}
	})

	// Test 5: Filter with limit
	t.Run("FilterWithLimit", func(t *testing.T) {
		limit := uint(1)
		f := &filter.F{
			Kinds: kind.NewS(kind.New(1)),
			Limit: &limit,
		}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 event due to limit, got %d", len(events))
		}
	})
}

// TestBuildCypherQueryWithNilFields tests the buildCypherQuery function with nil fields
func TestBuildCypherQueryWithNilFields(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Test that buildCypherQuery doesn't panic with nil fields
	t.Run("AllNilFields", func(t *testing.T) {
		f := &filter.F{
			Ids:     nil,
			Authors: nil,
			Kinds:   nil,
			Since:   nil,
			Until:   nil,
			Tags:    nil,
			Limit:   nil,
		}
		cypher, params := testDB.buildCypherQuery(f, false)
		if cypher == "" {
			t.Error("Expected non-empty Cypher query")
		}
		if params == nil {
			t.Error("Expected non-nil params map")
		}
	})

	// Test with empty slices
	t.Run("EmptySlices", func(t *testing.T) {
		f := &filter.F{
			Ids:     &tag.T{T: [][]byte{}},
			Authors: &tag.T{T: [][]byte{}},
			Kinds:   kind.NewS(),
		}
		cypher, params := testDB.buildCypherQuery(f, false)
		if cypher == "" {
			t.Error("Expected non-empty Cypher query")
		}
		if params == nil {
			t.Error("Expected non-nil params map")
		}
	})

	// Test with time filters
	t.Run("TimeFilters", func(t *testing.T) {
		since := timestamp.Now()
		until := timestamp.Now()
		f := &filter.F{
			Since: since,
			Until: until,
		}
		cypher, params := testDB.buildCypherQuery(f, false)
		if _, ok := params["since"]; !ok {
			t.Error("Expected 'since' param")
		}
		if _, ok := params["until"]; !ok {
			t.Error("Expected 'until' param")
		}
		_ = cypher
	})
}

// TestQueryEventsUppercaseHexNormalization tests that uppercase hex in filters is normalized
func TestQueryEventsUppercaseHexNormalization(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	// Clean up before test
	cleanTestDatabase()

	// Setup test event with lowercase pubkey (as Neo4j stores)
	lowercasePubkey := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	lowercaseEventID := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	setupTestEvent(t, lowercaseEventID, lowercasePubkey, 1, "[]")

	ctx := context.Background()

	// Test query with uppercase pubkey - should be normalized and still match
	t.Run("UppercaseAuthor", func(t *testing.T) {
		uppercasePubkey := "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"
		f := &filter.F{
			Authors: tag.NewFromBytesSlice([]byte(uppercasePubkey)),
		}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected to find 1 event with uppercase pubkey filter, got %d", len(events))
		}
	})

	// Test query with uppercase event ID - should be normalized and still match
	t.Run("UppercaseEventID", func(t *testing.T) {
		uppercaseEventID := "FEDCBA9876543210FEDCBA9876543210FEDCBA9876543210FEDCBA9876543210"
		f := &filter.F{
			Ids: tag.NewFromBytesSlice([]byte(uppercaseEventID)),
		}
		events, err := testDB.QueryEvents(ctx, f)
		if err != nil {
			t.Fatalf("QueryEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected to find 1 event with uppercase ID filter, got %d", len(events))
		}
	})
}
