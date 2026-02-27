package database

import (
	"bytes"
	"testing"

	"next.orly.dev/pkg/lol/chk"
	"github.com/minio/sha256-simd"
	"next.orly.dev/pkg/database/indexes"
	types2 "next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/utils"
)

func TestGetIndexesForEvent(t *testing.T) {
	t.Run("BasicEvent", testBasicEvent)
	t.Run("EventWithTags", testEventWithTags)
	t.Run("ErrorHandling", testErrorHandling)
}

// Helper function to verify that a specific index is included in the generated
// indexes
func verifyIndexIncluded(t *testing.T, idxs [][]byte, expectedIdx *indexes.T) {
	// Marshal the expected index
	buf := new(bytes.Buffer)
	err := expectedIdx.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("Failed to marshal expected index: %v", err)
	}

	expectedBytes := buf.Bytes()
	found := false

	for _, idx := range idxs {
		if utils.FastEqual(idx, expectedBytes) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected index not found in generated indexes")
		t.Errorf("Expected: %v", expectedBytes)
		t.Errorf("Generated indexes: %d indexes", len(idxs))
	}
}

// Test basic event with minimal fields
func testBasicEvent(t *testing.T) {
	// Create a basic event
	ev := event.New()

	// Set ID
	id := make([]byte, sha256.Size)
	for i := range id {
		id[i] = byte(i)
	}
	ev.ID = id

	// Set Pubkey
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 1)
	}
	ev.Pubkey = pubkey

	// Set CreatedAt
	ev.CreatedAt = 12345

	// Set Kind
	ev.Kind = kind.TextNote.K

	// Set Content
	ev.Content = []byte("Test content")

	// Generate indexes
	serial := uint64(1)
	idxs, err := GetIndexesForEvent(ev, serial)
	if chk.E(err) {
		t.Fatalf("GetIndexesForEvent failed: %v", err)
	}

	// Verify the number of indexes (should be 8 for a basic event without tags: 6 base + 2 word indexes from content)
	if len(idxs) != 8 {
		t.Fatalf("Expected 8 indexes, got %d", len(idxs))
	}

	// Create and verify the expected indexes

	// 1. ID index
	ser := new(types2.Uint40)
	err = ser.Set(serial)
	if chk.E(err) {
		t.Fatalf("Failed to create Uint40: %v", err)
	}

	idHash := new(types2.IdHash)
	err = idHash.FromId(ev.ID)
	if chk.E(err) {
		t.Fatalf("Failed to create IdHash: %v", err)
	}
	idIndex := indexes.IdEnc(idHash, ser)
	verifyIndexIncluded(t, idxs, idIndex)

	// 2. FullIdPubkey index
	fullID := new(types2.Id)
	err = fullID.FromId(ev.ID)
	if chk.E(err) {
		t.Fatalf("Failed to create ID: %v", err)
	}

	pubHash := new(types2.PubHash)
	err = pubHash.FromPubkey(ev.Pubkey)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}

	createdAt := new(types2.Uint64)
	createdAt.Set(uint64(ev.CreatedAt))

	idPubkeyIndex := indexes.FullIdPubkeyEnc(ser, fullID, pubHash, createdAt)
	verifyIndexIncluded(t, idxs, idPubkeyIndex)

	// 3. CreatedAt index
	createdAtIndex := indexes.CreatedAtEnc(createdAt, ser)
	verifyIndexIncluded(t, idxs, createdAtIndex)

	// 4. Pubkey index
	pubkeyIndex := indexes.PubkeyEnc(pubHash, createdAt, ser)
	verifyIndexIncluded(t, idxs, pubkeyIndex)

	// 5. Kind index
	kind := new(types2.Uint16)
	kind.Set(ev.Kind)

	kindIndex := indexes.KindEnc(kind, createdAt, ser)
	verifyIndexIncluded(t, idxs, kindIndex)

	// 6. KindPubkey index
	kindPubkeyIndex := indexes.KindPubkeyEnc(kind, pubHash, createdAt, ser)
	verifyIndexIncluded(t, idxs, kindPubkeyIndex)
}

// Test event with tags
func testEventWithTags(t *testing.T) {
	// Create an event with tags
	ev := event.New()

	// Set ID
	id := make([]byte, sha256.Size)
	for i := range id {
		id[i] = byte(i)
	}
	ev.ID = id

	// Set Pubkey
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 1)
	}
	ev.Pubkey = pubkey

	// Set CreatedAt
	ev.CreatedAt = 12345

	// Set Kind
	ev.Kind = kind.TextNote.K // TextNote kind

	// Set Content
	ev.Content = []byte("Test content with tags")

	// Add tags
	ev.Tags = tag.NewS()

	// Add e tag (event reference)
	eTagKey := "e"
	eTagValue := "abcdef1234567890"
	eTag := tag.NewFromAny(eTagKey, eTagValue)
	*ev.Tags = append(*ev.Tags, eTag)

	// Add p tag (pubkey reference)
	pTagKey := "p"
	pTagValue := "0123456789abcdef"
	pTag := tag.NewFromAny(pTagKey, pTagValue)
	*ev.Tags = append(*ev.Tags, pTag)

	// Generate indexes
	serial := uint64(2)
	idxs, err := GetIndexesForEvent(ev, serial)
	if chk.E(err) {
		t.Fatalf("GetIndexesForEvent failed: %v", err)
	}

	// Verify the number of indexes (should be 19 for an event with 2 tags)
	// 6 basic indexes + 3 word indexes from content ("test", "content", "tags" — "with" is a stopword)
	// + 4 indexes per tag (TagPubkey, Tag, TagKind, TagKindPubkey) = 8
	// + 2 word indexes from tag values ("abcdef1234567890", "0123456789abcdef")
	if len(idxs) != 19 {
		t.Fatalf("Expected 19 indexes, got %d", len(idxs))
	}

	// Create and verify the basic indexes (same as in testBasicEvent)
	ser := new(types2.Uint40)
	err = ser.Set(serial)
	if chk.E(err) {
		t.Fatalf("Failed to create Uint40: %v", err)
	}

	idHash := new(types2.IdHash)
	err = idHash.FromId(ev.ID)
	if chk.E(err) {
		t.Fatalf("Failed to create IdHash: %v", err)
	}

	// Verify one of the tag-related indexes (e tag)
	pubHash := new(types2.PubHash)
	err = pubHash.FromPubkey(ev.Pubkey)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}

	createdAt := new(types2.Uint64)
	createdAt.Set(uint64(ev.CreatedAt))

	// Create tag key and value for e tag
	eKey := new(types2.Letter)
	eKey.Set('e')

	eValueHash := new(types2.Ident)
	eValueHash.FromIdent([]byte("abcdef1234567890"))

	// Verify TagPubkey index for e tag
	pubkeyTagIndex := indexes.TagPubkeyEnc(
		eKey, eValueHash, pubHash, createdAt, ser,
	)
	verifyIndexIncluded(t, idxs, pubkeyTagIndex)

	// Verify Tag index for e tag
	tagIndex := indexes.TagEnc(
		eKey, eValueHash, createdAt, ser,
	)
	verifyIndexIncluded(t, idxs, tagIndex)

	// Verify TagKind index for e tag
	kind := new(types2.Uint16)
	kind.Set(ev.Kind)

	kindTagIndex := indexes.TagKindEnc(eKey, eValueHash, kind, createdAt, ser)
	verifyIndexIncluded(t, idxs, kindTagIndex)

	// Verify TagKindPubkey index for e tag
	kindPubkeyTagIndex := indexes.TagKindPubkeyEnc(
		eKey, eValueHash, kind, pubHash, createdAt, ser,
	)
	verifyIndexIncluded(t, idxs, kindPubkeyTagIndex)
}

// Test error handling
func testErrorHandling(t *testing.T) {
	// Test with invalid serial number (too large for Uint40)
	ev := event.New()

	// Set ID
	id := make([]byte, sha256.Size)
	for i := range id {
		id[i] = byte(i)
	}
	ev.ID = id

	// Set Pubkey
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i + 1)
	}
	ev.Pubkey = pubkey

	// Set CreatedAt
	ev.CreatedAt = 12345

	// Set Kind
	ev.Kind = kind.TextNote.K

	// Set Content
	ev.Content = []byte("Test content")

	// Use an invalid serial number (too large for Uint40)
	invalidSerial := uint64(1) << 40 // 2^40, which is too large for Uint40

	// Generate indexes
	idxs, err := GetIndexesForEvent(ev, invalidSerial)

	// Verify that an error was returned
	if err == nil {
		t.Fatalf("Expected error for invalid serial number, got nil")
	}

	// Verify that idxs is nil when an error occurs
	if idxs != nil {
		t.Fatalf("Expected nil idxs when error occurs, got %v", idxs)
	}

	// Note: We don't test with nil event as it causes a panic
	// The function doesn't have nil checks, which is a potential improvement
}
