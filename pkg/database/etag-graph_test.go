//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

func TestETagGraphEdgeCreation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a parent event (the post being replied to)
	parentPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	parentID := make([]byte, 32)
	parentID[0] = 0x10
	parentSig := make([]byte, 64)
	parentSig[0] = 0x10

	parentEvent := &event.E{
		ID:        parentID,
		Pubkey:    parentPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("This is the parent post"),
		Sig:       parentSig,
		Tags:      &tag.S{},
	}
	_, err = db.SaveEvent(ctx, parentEvent)
	if err != nil {
		t.Fatalf("Failed to save parent event: %v", err)
	}

	// Create a reply event with e-tag pointing to parent
	replyPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	replyID := make([]byte, 32)
	replyID[0] = 0x20
	replySig := make([]byte, 64)
	replySig[0] = 0x20

	replyEvent := &event.E{
		ID:        replyID,
		Pubkey:    replyPubkey,
		CreatedAt: 1234567891,
		Kind:      1,
		Content:   []byte("This is a reply"),
		Sig:       replySig,
		Tags: tag.NewS(
			tag.NewFromAny("e", hex.Enc(parentID)),
		),
	}
	_, err = db.SaveEvent(ctx, replyEvent)
	if err != nil {
		t.Fatalf("Failed to save reply event: %v", err)
	}

	// Get serials for both events
	parentSerial, err := db.GetSerialById(parentID)
	if err != nil {
		t.Fatalf("Failed to get parent serial: %v", err)
	}
	replySerial, err := db.GetSerialById(replyID)
	if err != nil {
		t.Fatalf("Failed to get reply serial: %v", err)
	}

	t.Logf("Parent serial: %d, Reply serial: %d", parentSerial.Get(), replySerial.Get())

	// Verify forward edge exists (reply -> parent)
	forwardFound := false
	prefix := []byte(indexes.EventEventGraphPrefix)

	err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			// Decode the key
			srcSer, tgtSer, kind, direction := indexes.EventEventGraphVars()
			keyReader := bytes.NewReader(key)
			if err := indexes.EventEventGraphDec(srcSer, tgtSer, kind, direction).UnmarshalRead(keyReader); err != nil {
				t.Logf("Failed to decode key: %v", err)
				continue
			}

			// Check if this is our edge
			if srcSer.Get() == replySerial.Get() && tgtSer.Get() == parentSerial.Get() {
				forwardFound = true
				if direction.Letter() != types.EdgeDirectionETagOut {
					t.Errorf("Expected direction %d, got %d", types.EdgeDirectionETagOut, direction.Letter())
				}
				if kind.Get() != 1 {
					t.Errorf("Expected kind 1, got %d", kind.Get())
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}
	if !forwardFound {
		t.Error("Forward edge (reply -> parent) should exist")
	}

	// Verify reverse edge exists (parent <- reply)
	reverseFound := false
	prefix = []byte(indexes.GraphEventEventPrefix)

	err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			// Decode the key
			tgtSer, kind, direction, srcSer := indexes.GraphEventEventVars()
			keyReader := bytes.NewReader(key)
			if err := indexes.GraphEventEventDec(tgtSer, kind, direction, srcSer).UnmarshalRead(keyReader); err != nil {
				t.Logf("Failed to decode key: %v", err)
				continue
			}

			t.Logf("Found gee edge: tgt=%d kind=%d dir=%d src=%d",
				tgtSer.Get(), kind.Get(), direction.Letter(), srcSer.Get())

			// Check if this is our edge
			if tgtSer.Get() == parentSerial.Get() && srcSer.Get() == replySerial.Get() {
				reverseFound = true
				if direction.Letter() != types.EdgeDirectionETagIn {
					t.Errorf("Expected direction %d, got %d", types.EdgeDirectionETagIn, direction.Letter())
				}
				if kind.Get() != 1 {
					t.Errorf("Expected kind 1, got %d", kind.Get())
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}
	if !reverseFound {
		t.Error("Reverse edge (parent <- reply) should exist")
	}
}

func TestETagGraphMultipleReplies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a parent event
	parentPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	parentID := make([]byte, 32)
	parentID[0] = 0x10
	parentSig := make([]byte, 64)
	parentSig[0] = 0x10

	parentEvent := &event.E{
		ID:        parentID,
		Pubkey:    parentPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("Parent post"),
		Sig:       parentSig,
		Tags:      &tag.S{},
	}
	_, err = db.SaveEvent(ctx, parentEvent)
	if err != nil {
		t.Fatalf("Failed to save parent: %v", err)
	}

	// Create multiple replies
	numReplies := 5
	for i := 0; i < numReplies; i++ {
		replyPubkey := make([]byte, 32)
		replyPubkey[0] = byte(i + 0x20)
		replyID := make([]byte, 32)
		replyID[0] = byte(i + 0x30)
		replySig := make([]byte, 64)
		replySig[0] = byte(i + 0x30)

		replyEvent := &event.E{
			ID:        replyID,
			Pubkey:    replyPubkey,
			CreatedAt: int64(1234567891 + i),
			Kind:      1,
			Content:   []byte("Reply"),
			Sig:       replySig,
			Tags: tag.NewS(
				tag.NewFromAny("e", hex.Enc(parentID)),
			),
		}
		_, err := db.SaveEvent(ctx, replyEvent)
		if err != nil {
			t.Fatalf("Failed to save reply %d: %v", i, err)
		}
	}

	// Count inbound edges to parent
	parentSerial, err := db.GetSerialById(parentID)
	if err != nil {
		t.Fatalf("Failed to get parent serial: %v", err)
	}

	inboundCount := 0
	prefix := []byte(indexes.GraphEventEventPrefix)

	err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			tgtSer, kind, direction, srcSer := indexes.GraphEventEventVars()
			keyReader := bytes.NewReader(key)
			if err := indexes.GraphEventEventDec(tgtSer, kind, direction, srcSer).UnmarshalRead(keyReader); err != nil {
				continue
			}

			if tgtSer.Get() == parentSerial.Get() {
				inboundCount++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}

	if inboundCount != numReplies {
		t.Errorf("Expected %d inbound edges, got %d", numReplies, inboundCount)
	}
}

func TestETagGraphDifferentKinds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a parent event (kind 1 - note)
	parentPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	parentID := make([]byte, 32)
	parentID[0] = 0x10
	parentSig := make([]byte, 64)
	parentSig[0] = 0x10

	parentEvent := &event.E{
		ID:        parentID,
		Pubkey:    parentPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("A note"),
		Sig:       parentSig,
		Tags:      &tag.S{},
	}
	_, err = db.SaveEvent(ctx, parentEvent)
	if err != nil {
		t.Fatalf("Failed to save parent: %v", err)
	}

	// Create a reaction (kind 7)
	reactionPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	reactionID := make([]byte, 32)
	reactionID[0] = 0x20
	reactionSig := make([]byte, 64)
	reactionSig[0] = 0x20

	reactionEvent := &event.E{
		ID:        reactionID,
		Pubkey:    reactionPubkey,
		CreatedAt: 1234567891,
		Kind:      7,
		Content:   []byte("+"),
		Sig:       reactionSig,
		Tags: tag.NewS(
			tag.NewFromAny("e", hex.Enc(parentID)),
		),
	}
	_, err = db.SaveEvent(ctx, reactionEvent)
	if err != nil {
		t.Fatalf("Failed to save reaction: %v", err)
	}

	// Create a repost (kind 6)
	repostPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")
	repostID := make([]byte, 32)
	repostID[0] = 0x30
	repostSig := make([]byte, 64)
	repostSig[0] = 0x30

	repostEvent := &event.E{
		ID:        repostID,
		Pubkey:    repostPubkey,
		CreatedAt: 1234567892,
		Kind:      6,
		Content:   []byte(""),
		Sig:       repostSig,
		Tags: tag.NewS(
			tag.NewFromAny("e", hex.Enc(parentID)),
		),
	}
	_, err = db.SaveEvent(ctx, repostEvent)
	if err != nil {
		t.Fatalf("Failed to save repost: %v", err)
	}

	// Query inbound edges by kind
	parentSerial, err := db.GetSerialById(parentID)
	if err != nil {
		t.Fatalf("Failed to get parent serial: %v", err)
	}

	kindCounts := make(map[uint16]int)
	prefix := []byte(indexes.GraphEventEventPrefix)

	err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			tgtSer, kind, direction, srcSer := indexes.GraphEventEventVars()
			keyReader := bytes.NewReader(key)
			if err := indexes.GraphEventEventDec(tgtSer, kind, direction, srcSer).UnmarshalRead(keyReader); err != nil {
				continue
			}

			if tgtSer.Get() == parentSerial.Get() {
				kindCounts[kind.Get()]++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}

	// Verify we have edges for each kind
	if kindCounts[7] != 1 {
		t.Errorf("Expected 1 kind-7 (reaction) edge, got %d", kindCounts[7])
	}
	if kindCounts[6] != 1 {
		t.Errorf("Expected 1 kind-6 (repost) edge, got %d", kindCounts[6])
	}
}

func TestETagGraphUnknownTarget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create an event with e-tag pointing to non-existent event
	unknownID := make([]byte, 32)
	unknownID[0] = 0xFF
	unknownID[31] = 0xFF

	replyPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	replyID := make([]byte, 32)
	replyID[0] = 0x10
	replySig := make([]byte, 64)
	replySig[0] = 0x10

	replyEvent := &event.E{
		ID:        replyID,
		Pubkey:    replyPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("Reply to unknown"),
		Sig:       replySig,
		Tags: tag.NewS(
			tag.NewFromAny("e", hex.Enc(unknownID)),
		),
	}
	_, err = db.SaveEvent(ctx, replyEvent)
	if err != nil {
		t.Fatalf("Failed to save reply: %v", err)
	}

	// Verify event was saved
	replySerial, err := db.GetSerialById(replyID)
	if err != nil {
		t.Fatalf("Failed to get reply serial: %v", err)
	}
	if replySerial == nil {
		t.Fatal("Reply serial should exist")
	}

	// Verify no forward edge was created (since target doesn't exist)
	edgeCount := 0
	prefix := []byte(indexes.EventEventGraphPrefix)

	err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			srcSer, _, _, _ := indexes.EventEventGraphVars()
			keyReader := bytes.NewReader(key)
			if err := indexes.EventEventGraphDec(srcSer, new(types.Uint40), new(types.Uint16), new(types.Letter)).UnmarshalRead(keyReader); err != nil {
				continue
			}

			if srcSer.Get() == replySerial.Get() {
				edgeCount++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("View failed: %v", err)
	}

	if edgeCount != 0 {
		t.Errorf("Expected no edges for unknown target, got %d", edgeCount)
	}
}
