package database

import (
	"context"
	"os"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"lol.mleku.dev/chk"
)

// TestBinaryTagFilterRegression tests that queries with #e and #p tags work correctly
// even when the event's tags are stored in binary format but filter values come as hex strings.
//
// This is a regression test for the bug where:
// - Events with e/p tags are stored with binary-encoded values (32 bytes + null terminator)
// - Filters from clients use hex strings (64 characters)
// - The mismatch caused queries with #e or #p filter tags to fail
//
// See: https://github.com/mleku/orly/issues/XXX
func TestBinaryTagFilterRegression(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-db-binary-tag-*")
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

	// Create signers for the test
	authorSign := p8k.MustNew()
	if err := authorSign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	referencedPubkeySign := p8k.MustNew()
	if err := referencedPubkeySign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	// Create a referenced event (to generate a valid event ID for e-tag)
	referencedEvent := event.New()
	referencedEvent.Kind = kind.TextNote.K
	referencedEvent.Pubkey = referencedPubkeySign.Pub()
	referencedEvent.CreatedAt = timestamp.Now().V - 7200 // 2 hours ago
	referencedEvent.Content = []byte("Referenced event")
	referencedEvent.Tags = tag.NewS()
	referencedEvent.Sign(referencedPubkeySign)

	// Save the referenced event
	if _, err := db.SaveEvent(ctx, referencedEvent); err != nil {
		t.Fatalf("Failed to save referenced event: %v", err)
	}

	// Get hex representations of the IDs we'll use in tags
	referencedEventIdHex := hex.Enc(referencedEvent.ID)
	referencedPubkeyHex := hex.Enc(referencedPubkeySign.Pub())

	// Create a test event similar to the problematic case:
	// - Kind 30520 (addressable)
	// - Has d, p, e, u, t tags
	testEvent := event.New()
	testEvent.Kind = 30520 // Addressable event kind
	testEvent.Pubkey = authorSign.Pub()
	testEvent.CreatedAt = timestamp.Now().V
	testEvent.Content = []byte("Test content with binary tags")
	testEvent.Tags = tag.NewS(
		tag.NewFromAny("d", "test-d-tag-value"),
		tag.NewFromAny("p", string(referencedPubkeyHex)),  // p-tag with hex pubkey
		tag.NewFromAny("e", string(referencedEventIdHex)), // e-tag with hex event ID
		tag.NewFromAny("u", "test.app"),
		tag.NewFromAny("t", "test-topic"),
	)
	testEvent.Sign(authorSign)

	// Save the test event
	if _, err := db.SaveEvent(ctx, testEvent); err != nil {
		t.Fatalf("Failed to save test event: %v", err)
	}

	authorPubkeyHex := hex.Enc(authorSign.Pub())
	testEventIdHex := hex.Enc(testEvent.ID)

	// Test case 1: Query WITHOUT e/p tags (should work - baseline)
	t.Run("QueryWithoutEPTags", func(t *testing.T) {
		f := &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(authorSign.Pub()),
			Tags: tag.NewS(
				tag.NewFromAny("#d", "test-d-tag-value"),
				tag.NewFromAny("#u", "test.app"),
			),
		}

		results, err := db.QueryForIds(ctx, f)
		if err != nil {
			t.Fatalf("Query without e/p tags failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected to find event with d/u tags filter, got 0 results")
		}

		// Verify we got the correct event
		found := false
		for _, r := range results {
			if r.IDHex() == testEventIdHex {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event ID %s not found in results", testEventIdHex)
		}
	})

	// Test case 2: Query WITH #p tag (this was the failing case)
	t.Run("QueryWithPTag", func(t *testing.T) {
		f := &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(authorSign.Pub()),
			Tags: tag.NewS(
				tag.NewFromAny("#d", "test-d-tag-value"),
				tag.NewFromAny("#p", string(referencedPubkeyHex)),
				tag.NewFromAny("#u", "test.app"),
			),
		}

		results, err := db.QueryForIds(ctx, f)
		if err != nil {
			t.Fatalf("Query with #p tag failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatalf("REGRESSION: Expected to find event with #p tag filter, got 0 results. "+
				"This suggests the binary tag encoding fix is not working. "+
				"Author: %s, #p: %s", authorPubkeyHex, referencedPubkeyHex)
		}

		// Verify we got the correct event
		found := false
		for _, r := range results {
			if r.IDHex() == testEventIdHex {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event ID %s not found in results", testEventIdHex)
		}
	})

	// Test case 3: Query WITH #e tag (this was also the failing case)
	t.Run("QueryWithETag", func(t *testing.T) {
		f := &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(authorSign.Pub()),
			Tags: tag.NewS(
				tag.NewFromAny("#d", "test-d-tag-value"),
				tag.NewFromAny("#e", string(referencedEventIdHex)),
				tag.NewFromAny("#u", "test.app"),
			),
		}

		results, err := db.QueryForIds(ctx, f)
		if err != nil {
			t.Fatalf("Query with #e tag failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatalf("REGRESSION: Expected to find event with #e tag filter, got 0 results. "+
				"This suggests the binary tag encoding fix is not working. "+
				"Author: %s, #e: %s", authorPubkeyHex, referencedEventIdHex)
		}

		// Verify we got the correct event
		found := false
		for _, r := range results {
			if r.IDHex() == testEventIdHex {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event ID %s not found in results", testEventIdHex)
		}
	})

	// Test case 4: Query WITH BOTH #e AND #p tags (the most complete failing case)
	t.Run("QueryWithBothEAndPTags", func(t *testing.T) {
		f := &filter.F{
			Kinds:   kind.NewS(kind.New(30520)),
			Authors: tag.NewFromBytesSlice(authorSign.Pub()),
			Tags: tag.NewS(
				tag.NewFromAny("#d", "test-d-tag-value"),
				tag.NewFromAny("#e", string(referencedEventIdHex)),
				tag.NewFromAny("#p", string(referencedPubkeyHex)),
				tag.NewFromAny("#u", "test.app"),
			),
		}

		results, err := db.QueryForIds(ctx, f)
		if err != nil {
			t.Fatalf("Query with both #e and #p tags failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatalf("REGRESSION: Expected to find event with #e and #p tag filters, got 0 results. "+
				"This is the exact regression case from the bug report. "+
				"Author: %s, #e: %s, #p: %s", authorPubkeyHex, referencedEventIdHex, referencedPubkeyHex)
		}

		// Verify we got the correct event
		found := false
		for _, r := range results {
			if r.IDHex() == testEventIdHex {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event ID %s not found in results", testEventIdHex)
		}
	})

	// Test case 5: Query with kinds + #p tag (no authors)
	// Note: Queries with only kinds+tags may use different index paths
	t.Run("QueryWithKindAndPTag", func(t *testing.T) {
		f := &filter.F{
			Kinds: kind.NewS(kind.New(30520)),
			Tags: tag.NewS(
				tag.NewFromAny("#p", string(referencedPubkeyHex)),
			),
		}

		results, err := db.QueryForIds(ctx, f)
		if err != nil {
			t.Fatalf("Query with kind+#p tag failed: %v", err)
		}

		// This query should find results using the TagKindEnc index
		t.Logf("Query with kind+#p tag returned %d results", len(results))
	})

	// Test case 6: Query with kinds + #e tag (no authors)
	t.Run("QueryWithKindAndETag", func(t *testing.T) {
		f := &filter.F{
			Kinds: kind.NewS(kind.New(30520)),
			Tags: tag.NewS(
				tag.NewFromAny("#e", string(referencedEventIdHex)),
			),
		}

		results, err := db.QueryForIds(ctx, f)
		if err != nil {
			t.Fatalf("Query with kind+#e tag failed: %v", err)
		}

		// This query should find results using the TagKindEnc index
		t.Logf("Query with kind+#e tag returned %d results", len(results))
	})
}

// TestFilterNormalization tests the filter normalization utilities
func TestFilterNormalization(t *testing.T) {
	// Test hex pubkey value (64 chars)
	hexPubkey := []byte("8b1180c2e03cbf83ab048068a7f7d6959ff0331761aba867aaecdc793045c1bc")

	// Test IsBinaryOptimizedTag
	if !IsBinaryOptimizedTag('e') {
		t.Error("Expected 'e' to be a binary-optimized tag")
	}
	if !IsBinaryOptimizedTag('p') {
		t.Error("Expected 'p' to be a binary-optimized tag")
	}
	if IsBinaryOptimizedTag('d') {
		t.Error("Expected 'd' NOT to be a binary-optimized tag")
	}
	if IsBinaryOptimizedTag('t') {
		t.Error("Expected 't' NOT to be a binary-optimized tag")
	}

	// Test IsValidHexValue
	if !IsValidHexValue(hexPubkey) {
		t.Error("Expected valid hex pubkey to pass IsValidHexValue")
	}
	if IsValidHexValue([]byte("not-hex")) {
		t.Error("Expected invalid hex to fail IsValidHexValue")
	}
	if IsValidHexValue([]byte("abc123")) { // Too short
		t.Error("Expected short hex to fail IsValidHexValue")
	}

	// Test HexToBinary conversion
	binary := HexToBinary(hexPubkey)
	if binary == nil {
		t.Fatal("HexToBinary returned nil for valid hex")
	}
	if len(binary) != BinaryEncodedLen {
		t.Errorf("Expected binary length %d, got %d", BinaryEncodedLen, len(binary))
	}
	if binary[HashLen] != 0 {
		t.Error("Expected null terminator at position 32")
	}

	// Test IsBinaryEncoded
	if !IsBinaryEncoded(binary) {
		t.Error("Expected converted binary to pass IsBinaryEncoded")
	}
	if IsBinaryEncoded(hexPubkey) {
		t.Error("Expected hex to fail IsBinaryEncoded")
	}

	// Test BinaryToHex (round-trip)
	hexBack := BinaryToHex(binary)
	if hexBack == nil {
		t.Fatal("BinaryToHex returned nil")
	}
	if string(hexBack) != string(hexPubkey) {
		t.Errorf("Round-trip failed: expected %s, got %s", hexPubkey, hexBack)
	}

	// Test NormalizeTagValue for p-tag (should convert hex to binary)
	normalized := NormalizeTagValue('p', hexPubkey)
	if !IsBinaryEncoded(normalized) {
		t.Error("Expected NormalizeTagValue to convert hex to binary for p-tag")
	}

	// Test NormalizeTagValue for d-tag (should NOT convert)
	dTagValue := []byte("some-d-tag-value")
	normalizedD := NormalizeTagValue('d', dTagValue)
	if string(normalizedD) != string(dTagValue) {
		t.Error("Expected NormalizeTagValue to leave d-tag unchanged")
	}

	// Test TagValuesMatch with different encodings
	if !TagValuesMatch('p', binary, hexPubkey) {
		t.Error("Expected binary and hex values to match for p-tag")
	}
	if !TagValuesMatch('p', hexPubkey, binary) {
		t.Error("Expected hex and binary values to match for p-tag (reverse)")
	}
	if !TagValuesMatch('p', binary, binary) {
		t.Error("Expected identical binary values to match")
	}
	if !TagValuesMatch('p', hexPubkey, hexPubkey) {
		t.Error("Expected identical hex values to match")
	}

	// Test non-matching values
	otherHex := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if TagValuesMatch('p', hexPubkey, otherHex) {
		t.Error("Expected different hex values NOT to match")
	}
}

// TestNormalizeFilterTag tests the NormalizeFilterTag function
func TestNormalizeFilterTag(t *testing.T) {
	hexPubkey := "8b1180c2e03cbf83ab048068a7f7d6959ff0331761aba867aaecdc793045c1bc"

	// Test with #p style tag (filter format)
	pTag := tag.NewFromAny("#p", hexPubkey)
	normalized := NormalizeFilterTag(pTag)

	if normalized == nil {
		t.Fatal("NormalizeFilterTag returned nil")
	}

	// Check that the normalized value is binary
	normalizedValue := normalized.T[1]
	if !IsBinaryEncoded(normalizedValue) {
		t.Errorf("Expected normalized #p tag value to be binary, got length %d", len(normalizedValue))
	}

	// Test with e style tag (event format - single letter key)
	hexEventId := "34ccd22f852544a0b7a310b50cc76189130fd3d121d1f4dd77d759862a7b7261"
	eTag := tag.NewFromAny("e", hexEventId)
	normalizedE := NormalizeFilterTag(eTag)

	normalizedEValue := normalizedE.T[1]
	if !IsBinaryEncoded(normalizedEValue) {
		t.Errorf("Expected normalized e tag value to be binary, got length %d", len(normalizedEValue))
	}

	// Test with non-optimized tag (should remain unchanged)
	dTag := tag.NewFromAny("#d", "some-value")
	normalizedD := NormalizeFilterTag(dTag)

	normalizedDValue := normalizedD.T[1]
	if string(normalizedDValue) != "some-value" {
		t.Errorf("Expected #d tag value to remain unchanged, got %s", normalizedDValue)
	}
}

// TestNormalizeFilter tests the full filter normalization
func TestNormalizeFilter(t *testing.T) {
	hexPubkey := "8b1180c2e03cbf83ab048068a7f7d6959ff0331761aba867aaecdc793045c1bc"
	hexEventId := "34ccd22f852544a0b7a310b50cc76189130fd3d121d1f4dd77d759862a7b7261"

	f := &filter.F{
		Kinds: kind.NewS(kind.New(30520)),
		Tags: tag.NewS(
			tag.NewFromAny("#d", "test-value"),
			tag.NewFromAny("#e", hexEventId),
			tag.NewFromAny("#p", hexPubkey),
			tag.NewFromAny("#u", "test.app"),
		),
	}

	normalized := NormalizeFilter(f)

	// Verify non-tag fields are preserved
	if normalized.Kinds == nil || normalized.Kinds.Len() != 1 {
		t.Error("Filter Kinds should be preserved")
	}

	// Verify tags are normalized
	if normalized.Tags == nil {
		t.Fatal("Normalized filter Tags is nil")
	}

	// Check that #e and #p tags have binary values
	for _, tg := range *normalized.Tags {
		key := tg.Key()
		if len(key) == 2 && key[0] == '#' {
			switch key[1] {
			case 'e', 'p':
				// These should have binary values
				val := tg.T[1]
				if !IsBinaryEncoded(val) {
					t.Errorf("Expected #%c tag to have binary value after normalization", key[1])
				}
			case 'd', 'u':
				// These should NOT have binary values
				val := tg.T[1]
				if IsBinaryEncoded(val) {
					t.Errorf("Expected #%c tag NOT to have binary value", key[1])
				}
			}
		}
	}
}
