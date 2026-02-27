//go:build !(js && wasm)

package database

import (
	"context"
	"testing"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

func TestTraverseFollows(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a simple follow graph:
	// Alice -> Bob, Carol
	// Bob -> David, Eve
	// Carol -> Eve, Frank
	//
	// Expected depth 1 from Alice: Bob, Carol
	// Expected depth 2 from Alice: David, Eve, Frank (Eve deduplicated)

	alice, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	bob, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	carol, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")
	david, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000004")
	eve, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000005")
	frank, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000006")

	// Create Alice's follow list (kind 3)
	aliceContactID := make([]byte, 32)
	aliceContactID[0] = 0x10
	aliceContactSig := make([]byte, 64)
	aliceContactSig[0] = 0x10
	aliceContact := &event.E{
		ID:        aliceContactID,
		Pubkey:    alice,
		CreatedAt: 1234567890,
		Kind:      3,
		Content:   []byte(""),
		Sig:       aliceContactSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(bob)),
			tag.NewFromAny("p", hex.Enc(carol)),
		),
	}
	_, err = db.SaveEvent(ctx, aliceContact)
	if err != nil {
		t.Fatalf("Failed to save Alice's contact list: %v", err)
	}

	// Create Bob's follow list
	bobContactID := make([]byte, 32)
	bobContactID[0] = 0x20
	bobContactSig := make([]byte, 64)
	bobContactSig[0] = 0x20
	bobContact := &event.E{
		ID:        bobContactID,
		Pubkey:    bob,
		CreatedAt: 1234567891,
		Kind:      3,
		Content:   []byte(""),
		Sig:       bobContactSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(david)),
			tag.NewFromAny("p", hex.Enc(eve)),
		),
	}
	_, err = db.SaveEvent(ctx, bobContact)
	if err != nil {
		t.Fatalf("Failed to save Bob's contact list: %v", err)
	}

	// Create Carol's follow list
	carolContactID := make([]byte, 32)
	carolContactID[0] = 0x30
	carolContactSig := make([]byte, 64)
	carolContactSig[0] = 0x30
	carolContact := &event.E{
		ID:        carolContactID,
		Pubkey:    carol,
		CreatedAt: 1234567892,
		Kind:      3,
		Content:   []byte(""),
		Sig:       carolContactSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(eve)),
			tag.NewFromAny("p", hex.Enc(frank)),
		),
	}
	_, err = db.SaveEvent(ctx, carolContact)
	if err != nil {
		t.Fatalf("Failed to save Carol's contact list: %v", err)
	}

	// Traverse follows from Alice with depth 2
	result, err := db.TraverseFollows(alice, 2)
	if err != nil {
		t.Fatalf("TraverseFollows failed: %v", err)
	}

	// Check depth 1: should have Bob and Carol
	depth1 := result.GetPubkeysAtDepth(1)
	if len(depth1) != 2 {
		t.Errorf("Expected 2 pubkeys at depth 1, got %d", len(depth1))
	}

	depth1Set := make(map[string]bool)
	for _, pk := range depth1 {
		depth1Set[pk] = true
	}
	if !depth1Set[hex.Enc(bob)] {
		t.Error("Bob should be at depth 1")
	}
	if !depth1Set[hex.Enc(carol)] {
		t.Error("Carol should be at depth 1")
	}

	// Check depth 2: should have David, Eve, Frank (Eve deduplicated)
	depth2 := result.GetPubkeysAtDepth(2)
	if len(depth2) != 3 {
		t.Errorf("Expected 3 pubkeys at depth 2, got %d: %v", len(depth2), depth2)
	}

	depth2Set := make(map[string]bool)
	for _, pk := range depth2 {
		depth2Set[pk] = true
	}
	if !depth2Set[hex.Enc(david)] {
		t.Error("David should be at depth 2")
	}
	if !depth2Set[hex.Enc(eve)] {
		t.Error("Eve should be at depth 2")
	}
	if !depth2Set[hex.Enc(frank)] {
		t.Error("Frank should be at depth 2")
	}

	// Verify total count
	if result.TotalPubkeys != 5 {
		t.Errorf("Expected 5 total pubkeys, got %d", result.TotalPubkeys)
	}

	// Verify ToDepthArrays output
	arrays := result.ToDepthArrays()
	if len(arrays) != 2 {
		t.Errorf("Expected 2 depth arrays, got %d", len(arrays))
	}
	if len(arrays[0]) != 2 {
		t.Errorf("Expected 2 pubkeys in depth 1 array, got %d", len(arrays[0]))
	}
	if len(arrays[1]) != 3 {
		t.Errorf("Expected 3 pubkeys in depth 2 array, got %d", len(arrays[1]))
	}
}

func TestTraverseFollowsDepth1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	alice, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	bob, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	carol, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")

	// Create Alice's follow list
	aliceContactID := make([]byte, 32)
	aliceContactID[0] = 0x10
	aliceContactSig := make([]byte, 64)
	aliceContactSig[0] = 0x10
	aliceContact := &event.E{
		ID:        aliceContactID,
		Pubkey:    alice,
		CreatedAt: 1234567890,
		Kind:      3,
		Content:   []byte(""),
		Sig:       aliceContactSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(bob)),
			tag.NewFromAny("p", hex.Enc(carol)),
		),
	}
	_, err = db.SaveEvent(ctx, aliceContact)
	if err != nil {
		t.Fatalf("Failed to save contact list: %v", err)
	}

	// Traverse with depth 1 only
	result, err := db.TraverseFollows(alice, 1)
	if err != nil {
		t.Fatalf("TraverseFollows failed: %v", err)
	}

	if result.TotalPubkeys != 2 {
		t.Errorf("Expected 2 pubkeys, got %d", result.TotalPubkeys)
	}

	arrays := result.ToDepthArrays()
	if len(arrays) != 1 {
		t.Errorf("Expected 1 depth array for depth 1 query, got %d", len(arrays))
	}
}

func TestTraverseFollowersBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create scenario: Bob and Carol follow Alice
	alice, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	bob, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	carol, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")

	// Bob's contact list includes Alice
	bobContactID := make([]byte, 32)
	bobContactID[0] = 0x10
	bobContactSig := make([]byte, 64)
	bobContactSig[0] = 0x10
	bobContact := &event.E{
		ID:        bobContactID,
		Pubkey:    bob,
		CreatedAt: 1234567890,
		Kind:      3,
		Content:   []byte(""),
		Sig:       bobContactSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(alice)),
		),
	}
	_, err = db.SaveEvent(ctx, bobContact)
	if err != nil {
		t.Fatalf("Failed to save Bob's contact list: %v", err)
	}

	// Carol's contact list includes Alice
	carolContactID := make([]byte, 32)
	carolContactID[0] = 0x20
	carolContactSig := make([]byte, 64)
	carolContactSig[0] = 0x20
	carolContact := &event.E{
		ID:        carolContactID,
		Pubkey:    carol,
		CreatedAt: 1234567891,
		Kind:      3,
		Content:   []byte(""),
		Sig:       carolContactSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(alice)),
		),
	}
	_, err = db.SaveEvent(ctx, carolContact)
	if err != nil {
		t.Fatalf("Failed to save Carol's contact list: %v", err)
	}

	// Find Alice's followers
	result, err := db.TraverseFollowers(alice, 1)
	if err != nil {
		t.Fatalf("TraverseFollowers failed: %v", err)
	}

	if result.TotalPubkeys != 2 {
		t.Errorf("Expected 2 followers, got %d", result.TotalPubkeys)
	}

	followers := result.GetPubkeysAtDepth(1)
	followerSet := make(map[string]bool)
	for _, pk := range followers {
		followerSet[pk] = true
	}
	if !followerSet[hex.Enc(bob)] {
		t.Error("Bob should be a follower")
	}
	if !followerSet[hex.Enc(carol)] {
		t.Error("Carol should be a follower")
	}
}

func TestTraverseFollowsNonExistent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Try to traverse from a pubkey that doesn't exist
	nonExistent, _ := hex.Dec("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	result, err := db.TraverseFollows(nonExistent, 2)
	if err != nil {
		t.Fatalf("TraverseFollows should not error for non-existent pubkey: %v", err)
	}

	if result.TotalPubkeys != 0 {
		t.Errorf("Expected 0 pubkeys for non-existent seed, got %d", result.TotalPubkeys)
	}
}
