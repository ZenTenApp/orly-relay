package database

import (
	"bytes"
	"math"
	"testing"

	"next.orly.dev/pkg/lol/chk"
	"github.com/minio/sha256-simd"
	"next.orly.dev/pkg/database/indexes"
	types2 "next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/utils"
)

// TestGetIndexesFromFilter tests the GetIndexesFromFilter function
func TestGetIndexesFromFilter(t *testing.T) {
	t.Run("ID", testIdFilter)
	t.Run("Pubkey", testPubkeyFilter)
	t.Run("CreatedAt", testCreatedAtFilter)
	t.Run("CreatedAtUntil", testCreatedAtUntilFilter)
	t.Run("TagPubkey", testPubkeyTagFilter)
	t.Run("Tag", testTagFilter)
	t.Run("Kind", testKindFilter)
	t.Run("KindPubkey", testKindPubkeyFilter)
	t.Run("MultipleKindPubkey", testMultipleKindPubkeyFilter)
	t.Run("TagKind", testKindTagFilter)
	t.Run("TagKindPubkey", testKindPubkeyTagFilter)
}

// Helper function to verify that the generated index matches the expected indexes
func verifyIndex(
	t *testing.T, idxs []Range, expectedStartIdx, expectedEndIdx *indexes.T,
) {
	if len(idxs) != 1 {
		t.Fatalf("Expected 1 index, got %d", len(idxs))
	}

	// Marshal the expected start index
	startBuf := new(bytes.Buffer)
	err := expectedStartIdx.MarshalWrite(startBuf)
	if chk.E(err) {
		t.Fatalf("Failed to marshal expected start index: %v", err)
	}

	// Compare the generated start index with the expected start index
	if !utils.FastEqual(idxs[0].Start, startBuf.Bytes()) {
		t.Errorf("Generated start index does not match expected start index")
		t.Errorf("Generated: %v", idxs[0].Start)
		t.Errorf("Expected: %v", startBuf.Bytes())
	}

	// If expectedEndIdx is nil, use expectedStartIdx
	endIdx := expectedEndIdx
	if endIdx == nil {
		endIdx = expectedStartIdx
	}

	// Marshal the expected end index
	endBuf := new(bytes.Buffer)
	err = endIdx.MarshalWrite(endBuf)
	if chk.E(err) {
		t.Fatalf("Failed to marshal expected End index: %v", err)
	}

	// Compare the generated end index with the expected end index
	if !utils.FastEqual(idxs[0].End, endBuf.Bytes()) {
		t.Errorf("Generated End index does not match expected End index")
		t.Errorf("Generated: %v", idxs[0].End)
		t.Errorf("Expected: %v", endBuf.Bytes())
	}
}

// Test ID filter
func testIdFilter(t *testing.T) {
	// Create a filter with an ID
	f := filter.New()
	id := make([]byte, sha256.Size)
	for i := range id {
		id[i] = byte(i)
	}
	f.Ids.T = append(f.Ids.T, id)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected index
	idHash := new(types2.IdHash)
	err = idHash.FromId(id)
	if chk.E(err) {
		t.Fatalf("Failed to create IdHash: %v", err)
	}
	expectedIdx := indexes.IdEnc(idHash, nil)

	// Verify the generated index
	// For ID filter, both start and end indexes are the same
	verifyIndex(t, idxs, expectedIdx, expectedIdx)
}

// Test Pubkey filter
func testPubkeyFilter(t *testing.T) {
	// Create a filter with an Author, Since, and Until
	f := filter.New()
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}
	f.Authors.T = append(f.Authors.T, pubkey)
	f.Since = timestamp.FromUnix(12345)
	f.Until = timestamp.FromUnix(67890) // Added Until field

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected indexes
	p := new(types2.PubHash)
	err = p.FromPubkey(pubkey)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.PubkeyEnc(p, caStart, nil)

	// End index uses Until
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(f.Until.V))
	expectedEndIdx := indexes.PubkeyEnc(p, caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test CreatedAt filter
func testCreatedAtFilter(t *testing.T) {
	// Create a filter with Since
	f := filter.New()
	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected start index (using Since)
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.CreatedAtEnc(caStart, nil)

	// Create the expected end index (using math.MaxInt64 since Until is not specified)
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))
	expectedEndIdx := indexes.CreatedAtEnc(caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test CreatedAt filter with Until
func testCreatedAtUntilFilter(t *testing.T) {
	// Create a filter with Until
	f := filter.New()
	f.Until = timestamp.FromUnix(67890)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected start index (using 0 since Since is not specified)
	caStart := new(types2.Uint64)
	caStart.Set(uint64(0))
	expectedStartIdx := indexes.CreatedAtEnc(caStart, nil)

	// Create the expected end index (using Until)
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(f.Until.V))
	expectedEndIdx := indexes.CreatedAtEnc(caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test TagPubkey filter
func testPubkeyTagFilter(t *testing.T) {
	// Create a filter with an Author, a Tag, and Since
	f := filter.New()
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}
	f.Authors.T = append(f.Authors.T, pubkey)
	// Create a tag
	tagKey := "e"
	tagValue := "test-value"
	tagT := tag.NewFromAny(tagKey, tagValue)
	*f.Tags = append(*f.Tags, tagT)

	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected indexes
	p := new(types2.PubHash)
	err = p.FromPubkey(pubkey)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}
	key := new(types2.Letter)
	key.Set(tagKey[0])
	valueHash := new(types2.Ident)
	valueHash.FromIdent([]byte(tagValue))

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.TagPubkeyEnc(key, valueHash, p, caStart, nil)

	// End index uses math.MaxInt64 since Until is not specified
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))
	expectedEndIdx := indexes.TagPubkeyEnc(key, valueHash, p, caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test Tag filter
func testTagFilter(t *testing.T) {
	// Create a filter with a Tag and Since
	f := filter.New()

	// Create a tag
	tagKey := "e"
	tagValue := "test-value"
	tagT := tag.NewFromAny(tagKey, tagValue)
	*f.Tags = append(*f.Tags, tagT)

	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected indexes
	key := new(types2.Letter)
	key.Set(tagKey[0])
	valueHash := new(types2.Ident)
	valueHash.FromIdent([]byte(tagValue))

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.TagEnc(key, valueHash, caStart, nil)

	// End index uses math.MaxInt64 since Until is not specified
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))
	expectedEndIdx := indexes.TagEnc(key, valueHash, caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test Kind filter
func testKindFilter(t *testing.T) {
	// Create a filter with a Kind and Since
	f := filter.New()
	f.Kinds = kind.NewS(kind.TextNote)
	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected indexes
	k := new(types2.Uint16)
	k.Set(1)

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.KindEnc(k, caStart, nil)

	// End index uses math.MaxInt64 since Until is not specified
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))
	expectedEndIdx := indexes.KindEnc(k, caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test KindPubkey filter
func testKindPubkeyFilter(t *testing.T) {
	// Create a filter with a Kind, an Author, and Since
	f := filter.New()
	f.Kinds = kind.NewS(kind.TextNote)
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}
	f.Authors.T = append(f.Authors.T, pubkey)
	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected indexes
	k := new(types2.Uint16)
	k.Set(1)
	p := new(types2.PubHash)
	err = p.FromPubkey(pubkey)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.KindPubkeyEnc(k, p, caStart, nil)

	// End index uses math.MaxInt64 since Until is not specified
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))
	expectedEndIdx := indexes.KindPubkeyEnc(k, p, caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test TagKind filter
func testKindTagFilter(t *testing.T) {
	// Create a filter with a Kind, a Tag, and Since
	f := filter.New()
	f.Kinds = kind.NewS(kind.TextNote)

	// Create a tag
	tagKey := "e"
	tagValue := "test-value"
	tagT := tag.NewFromAny(tagKey, tagValue)
	*f.Tags = append(*f.Tags, tagT)

	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected indexes
	k := new(types2.Uint16)
	k.Set(1)
	key := new(types2.Letter)
	key.Set(tagKey[0])
	valueHash := new(types2.Ident)
	valueHash.FromIdent([]byte(tagValue))

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.TagKindEnc(key, valueHash, k, caStart, nil)

	// End index uses math.MaxInt64 since Until is not specified
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))
	expectedEndIdx := indexes.TagKindEnc(key, valueHash, k, caEnd, nil)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}

// Test Multiple KindPubkey filter
func testMultipleKindPubkeyFilter(t *testing.T) {
	// Create a filter with multiple Kinds and multiple Authors
	f := filter.New()
	f.Kinds = kind.NewS(kind.New(1), kind.New(2))

	// Create two pubkeys
	pubkey1 := make([]byte, 32)
	pubkey2 := make([]byte, 32)
	for i := range pubkey1 {
		pubkey1[i] = byte(i)
		pubkey2[i] = byte(i + 100)
	}
	f.Authors.T = append(f.Authors.T, pubkey1)
	f.Authors.T = append(f.Authors.T, pubkey2)
	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// We should have 4 indexes (2 kinds * 2 pubkeys)
	if len(idxs) != 4 {
		t.Fatalf("Expected 4 indexes, got %d", len(idxs))
	}

	// Create the expected indexes
	k1 := new(types2.Uint16)
	k1.Set(1)
	k2 := new(types2.Uint16)
	k2.Set(2)

	p1 := new(types2.PubHash)
	err = p1.FromPubkey(pubkey1)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}

	p2 := new(types2.PubHash)
	err = p2.FromPubkey(pubkey2)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))

	// End index uses math.MaxInt64 since Until is not specified
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))

	// Create all expected combinations
	expectedIdxs := make([][]byte, 8) // 4 combinations * 2 (start/end)

	// Kind 1, Pubkey 1
	startBuf1 := new(bytes.Buffer)
	idxS1 := indexes.KindPubkeyEnc(k1, p1, caStart, nil)
	if err = idxS1.MarshalWrite(startBuf1); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[0] = startBuf1.Bytes()

	endBuf1 := new(bytes.Buffer)
	idxE1 := indexes.KindPubkeyEnc(k1, p1, caEnd, nil)
	if err = idxE1.MarshalWrite(endBuf1); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[1] = endBuf1.Bytes()

	// Kind 1, Pubkey 2
	startBuf2 := new(bytes.Buffer)
	idxS2 := indexes.KindPubkeyEnc(k1, p2, caStart, nil)
	if err = idxS2.MarshalWrite(startBuf2); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[2] = startBuf2.Bytes()

	endBuf2 := new(bytes.Buffer)
	idxE2 := indexes.KindPubkeyEnc(k1, p2, caEnd, nil)
	if err = idxE2.MarshalWrite(endBuf2); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[3] = endBuf2.Bytes()

	// Kind 2, Pubkey 1
	startBuf3 := new(bytes.Buffer)
	idxS3 := indexes.KindPubkeyEnc(k2, p1, caStart, nil)
	if err = idxS3.MarshalWrite(startBuf3); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[4] = startBuf3.Bytes()

	endBuf3 := new(bytes.Buffer)
	idxE3 := indexes.KindPubkeyEnc(k2, p1, caEnd, nil)
	if err = idxE3.MarshalWrite(endBuf3); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[5] = endBuf3.Bytes()

	// Kind 2, Pubkey 2
	startBuf4 := new(bytes.Buffer)
	idxS4 := indexes.KindPubkeyEnc(k2, p2, caStart, nil)
	if err = idxS4.MarshalWrite(startBuf4); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[6] = startBuf4.Bytes()

	endBuf4 := new(bytes.Buffer)
	idxE4 := indexes.KindPubkeyEnc(k2, p2, caEnd, nil)
	if err = idxE4.MarshalWrite(endBuf4); chk.E(err) {
		t.Fatalf("Failed to marshal index: %v", err)
	}
	expectedIdxs[7] = endBuf4.Bytes()

	// Verify that all expected combinations are present
	foundCombinations := 0
	for _, idx := range idxs {
		for i := 0; i < len(expectedIdxs); i += 2 {
			if utils.FastEqual(idx.Start, expectedIdxs[i]) && utils.FastEqual(
				idx.End, expectedIdxs[i+1],
			) {
				foundCombinations++
				break
			}
		}
	}

	if foundCombinations != 4 {
		t.Fatalf("Expected to find 4 combinations, found %d", foundCombinations)
	}
}

// Test TagKindPubkey filter
func testKindPubkeyTagFilter(t *testing.T) {
	// Create a filter with a Kind, an Author, a Tag, and Since
	f := filter.New()
	f.Kinds = kind.NewS(kind.New(1))
	pubkey := make([]byte, 32)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}
	f.Authors.T = append(f.Authors.T, pubkey)

	// Create a tag
	tagKey := "e"
	tagValue := "test-value"
	tagT := tag.NewFromAny(tagKey, tagValue)
	*f.Tags = append(*f.Tags, tagT)

	f.Since = timestamp.FromUnix(12345)

	// Generate indexes
	idxs, err := GetIndexesFromFilter(f)
	if chk.E(err) {
		t.Fatalf("GetIndexesFromFilter failed: %v", err)
	}

	// Create the expected indexes
	k := new(types2.Uint16)
	k.Set(1)
	p := new(types2.PubHash)
	err = p.FromPubkey(pubkey)
	if chk.E(err) {
		t.Fatalf("Failed to create PubHash: %v", err)
	}
	key := new(types2.Letter)
	key.Set(tagKey[0])
	valueHash := new(types2.Ident)
	valueHash.FromIdent([]byte(tagValue))

	// Start index uses Since
	caStart := new(types2.Uint64)
	caStart.Set(uint64(f.Since.V))
	expectedStartIdx := indexes.TagKindPubkeyEnc(
		key, valueHash, k, p, caStart, nil,
	)

	// End index uses math.MaxInt64 since Until is not specified
	caEnd := new(types2.Uint64)
	caEnd.Set(uint64(math.MaxInt64))
	expectedEndIdx := indexes.TagKindPubkeyEnc(
		key, valueHash, k, p, caEnd, nil,
	)

	// Verify the generated index
	verifyIndex(t, idxs, expectedStartIdx, expectedEndIdx)
}
