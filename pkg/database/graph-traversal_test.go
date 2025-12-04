//go:build !(js && wasm)

package database

import (
	"context"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

func TestGetPTagsFromEventSerial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create an author pubkey
	authorPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")

	// Create p-tag target pubkeys
	target1, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	target2, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")

	// Create event with p-tags
	eventID := make([]byte, 32)
	eventID[0] = 0x10
	eventSig := make([]byte, 64)
	eventSig[0] = 0x10

	ev := &event.E{
		ID:        eventID,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("Test event with p-tags"),
		Sig:       eventSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(target1)),
			tag.NewFromAny("p", hex.Enc(target2)),
		),
	}

	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Get the event serial
	eventSerial, err := db.GetSerialById(eventID)
	if err != nil {
		t.Fatalf("Failed to get event serial: %v", err)
	}

	// Get p-tags from event serial
	ptagSerials, err := db.GetPTagsFromEventSerial(eventSerial)
	if err != nil {
		t.Fatalf("GetPTagsFromEventSerial failed: %v", err)
	}

	// Should have 2 p-tags
	if len(ptagSerials) != 2 {
		t.Errorf("Expected 2 p-tag serials, got %d", len(ptagSerials))
	}

	// Verify the pubkeys
	for _, serial := range ptagSerials {
		pubkey, err := db.GetPubkeyBySerial(serial)
		if err != nil {
			t.Errorf("Failed to get pubkey for serial: %v", err)
			continue
		}
		pubkeyHex := hex.Enc(pubkey)
		if pubkeyHex != hex.Enc(target1) && pubkeyHex != hex.Enc(target2) {
			t.Errorf("Unexpected pubkey: %s", pubkeyHex)
		}
	}
}

func TestGetETagsFromEventSerial(t *testing.T) {
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
		t.Fatalf("Failed to save parent event: %v", err)
	}

	// Create a reply event with e-tag
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
		Content:   []byte("Reply"),
		Sig:       replySig,
		Tags: tag.NewS(
			tag.NewFromAny("e", hex.Enc(parentID)),
		),
	}
	_, err = db.SaveEvent(ctx, replyEvent)
	if err != nil {
		t.Fatalf("Failed to save reply event: %v", err)
	}

	// Get e-tags from reply
	replySerial, _ := db.GetSerialById(replyID)
	etagSerials, err := db.GetETagsFromEventSerial(replySerial)
	if err != nil {
		t.Fatalf("GetETagsFromEventSerial failed: %v", err)
	}

	if len(etagSerials) != 1 {
		t.Errorf("Expected 1 e-tag serial, got %d", len(etagSerials))
	}

	// Verify the target event
	if len(etagSerials) > 0 {
		targetEventID, err := db.GetEventIdBySerial(etagSerials[0])
		if err != nil {
			t.Fatalf("Failed to get event ID from serial: %v", err)
		}
		if hex.Enc(targetEventID) != hex.Enc(parentID) {
			t.Errorf("Expected parent ID, got %s", hex.Enc(targetEventID))
		}
	}
}

func TestGetReferencingEvents(t *testing.T) {
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
		t.Fatalf("Failed to save parent event: %v", err)
	}

	// Create multiple replies and reactions
	for i := 0; i < 3; i++ {
		replyPubkey := make([]byte, 32)
		replyPubkey[0] = byte(0x20 + i)
		replyID := make([]byte, 32)
		replyID[0] = byte(0x30 + i)
		replySig := make([]byte, 64)
		replySig[0] = byte(0x30 + i)

		var evKind uint16 = 1 // Reply
		if i == 2 {
			evKind = 7 // Reaction
		}

		replyEvent := &event.E{
			ID:        replyID,
			Pubkey:    replyPubkey,
			CreatedAt: int64(1234567891 + i),
			Kind:      evKind,
			Content:   []byte("Response"),
			Sig:       replySig,
			Tags: tag.NewS(
				tag.NewFromAny("e", hex.Enc(parentID)),
			),
		}
		_, err = db.SaveEvent(ctx, replyEvent)
		if err != nil {
			t.Fatalf("Failed to save reply %d: %v", i, err)
		}
	}

	// Get parent serial
	parentSerial, _ := db.GetSerialById(parentID)

	// Test without kind filter
	refs, err := db.GetReferencingEvents(parentSerial, nil)
	if err != nil {
		t.Fatalf("GetReferencingEvents failed: %v", err)
	}
	if len(refs) != 3 {
		t.Errorf("Expected 3 referencing events, got %d", len(refs))
	}

	// Test with kind filter (only replies)
	refs, err = db.GetReferencingEvents(parentSerial, []uint16{1})
	if err != nil {
		t.Fatalf("GetReferencingEvents with kind filter failed: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("Expected 2 kind-1 referencing events, got %d", len(refs))
	}

	// Test with kind filter (only reactions)
	refs, err = db.GetReferencingEvents(parentSerial, []uint16{7})
	if err != nil {
		t.Fatalf("GetReferencingEvents with kind 7 filter failed: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("Expected 1 kind-7 referencing event, got %d", len(refs))
	}
}

func TestGetFollowsFromPubkeySerial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create author and their follows
	authorPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	follow1, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	follow2, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")
	follow3, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000004")

	// Create kind-3 contact list
	eventID := make([]byte, 32)
	eventID[0] = 0x10
	eventSig := make([]byte, 64)
	eventSig[0] = 0x10

	contactList := &event.E{
		ID:        eventID,
		Pubkey:    authorPubkey,
		CreatedAt: 1234567890,
		Kind:      3,
		Content:   []byte(""),
		Sig:       eventSig,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(follow1)),
			tag.NewFromAny("p", hex.Enc(follow2)),
			tag.NewFromAny("p", hex.Enc(follow3)),
		),
	}
	_, err = db.SaveEvent(ctx, contactList)
	if err != nil {
		t.Fatalf("Failed to save contact list: %v", err)
	}

	// Get author serial
	authorSerial, err := db.GetPubkeySerial(authorPubkey)
	if err != nil {
		t.Fatalf("Failed to get author serial: %v", err)
	}

	// Get follows
	follows, err := db.GetFollowsFromPubkeySerial(authorSerial)
	if err != nil {
		t.Fatalf("GetFollowsFromPubkeySerial failed: %v", err)
	}

	if len(follows) != 3 {
		t.Errorf("Expected 3 follows, got %d", len(follows))
	}

	// Verify the follows are correct
	expectedFollows := map[string]bool{
		hex.Enc(follow1): false,
		hex.Enc(follow2): false,
		hex.Enc(follow3): false,
	}
	for _, serial := range follows {
		pubkey, err := db.GetPubkeyBySerial(serial)
		if err != nil {
			t.Errorf("Failed to get pubkey from serial: %v", err)
			continue
		}
		pkHex := hex.Enc(pubkey)
		if _, exists := expectedFollows[pkHex]; exists {
			expectedFollows[pkHex] = true
		} else {
			t.Errorf("Unexpected follow: %s", pkHex)
		}
	}
	for pk, found := range expectedFollows {
		if !found {
			t.Errorf("Expected follow not found: %s", pk)
		}
	}
}

func TestGraphResult(t *testing.T) {
	result := NewGraphResult()

	// Add pubkeys at different depths
	result.AddPubkeyAtDepth("pubkey1", 1)
	result.AddPubkeyAtDepth("pubkey2", 1)
	result.AddPubkeyAtDepth("pubkey3", 2)
	result.AddPubkeyAtDepth("pubkey4", 2)
	result.AddPubkeyAtDepth("pubkey5", 3)

	// Try to add duplicate
	added := result.AddPubkeyAtDepth("pubkey1", 2)
	if added {
		t.Error("Should not add duplicate pubkey")
	}

	// Verify counts
	if result.TotalPubkeys != 5 {
		t.Errorf("Expected 5 total pubkeys, got %d", result.TotalPubkeys)
	}

	// Verify depth tracking
	if result.GetPubkeyDepth("pubkey1") != 1 {
		t.Errorf("pubkey1 should be at depth 1")
	}
	if result.GetPubkeyDepth("pubkey3") != 2 {
		t.Errorf("pubkey3 should be at depth 2")
	}

	// Verify HasPubkey
	if !result.HasPubkey("pubkey1") {
		t.Error("Should have pubkey1")
	}
	if result.HasPubkey("nonexistent") {
		t.Error("Should not have nonexistent pubkey")
	}

	// Verify ToDepthArrays
	arrays := result.ToDepthArrays()
	if len(arrays) != 3 {
		t.Errorf("Expected 3 depth arrays, got %d", len(arrays))
	}
	if len(arrays[0]) != 2 {
		t.Errorf("Expected 2 pubkeys at depth 1, got %d", len(arrays[0]))
	}
	if len(arrays[1]) != 2 {
		t.Errorf("Expected 2 pubkeys at depth 2, got %d", len(arrays[1]))
	}
	if len(arrays[2]) != 1 {
		t.Errorf("Expected 1 pubkey at depth 3, got %d", len(arrays[2]))
	}
}

func TestGraphResultRefs(t *testing.T) {
	result := NewGraphResult()

	// Add some pubkeys
	result.AddPubkeyAtDepth("pubkey1", 1)
	result.AddEventAtDepth("event1", 1)

	// Add inbound refs (kind 7 reactions)
	result.AddInboundRef(7, "event1", "reaction1")
	result.AddInboundRef(7, "event1", "reaction2")
	result.AddInboundRef(7, "event1", "reaction3")

	// Get sorted refs
	refs := result.GetInboundRefsSorted(7)
	if len(refs) != 1 {
		t.Fatalf("Expected 1 aggregation, got %d", len(refs))
	}
	if refs[0].RefCount != 3 {
		t.Errorf("Expected 3 refs, got %d", refs[0].RefCount)
	}
	if refs[0].TargetEventID != "event1" {
		t.Errorf("Expected event1, got %s", refs[0].TargetEventID)
	}
}

func TestGetFollowersOfPubkeySerial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create target pubkey (the one being followed)
	targetPubkey, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")

	// Create followers
	follower1, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	follower2, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")

	// Create kind-3 contact lists for followers
	for i, followerPubkey := range [][]byte{follower1, follower2} {
		eventID := make([]byte, 32)
		eventID[0] = byte(0x10 + i)
		eventSig := make([]byte, 64)
		eventSig[0] = byte(0x10 + i)

		contactList := &event.E{
			ID:        eventID,
			Pubkey:    followerPubkey,
			CreatedAt: int64(1234567890 + i),
			Kind:      3,
			Content:   []byte(""),
			Sig:       eventSig,
			Tags: tag.NewS(
				tag.NewFromAny("p", hex.Enc(targetPubkey)),
			),
		}
		_, err = db.SaveEvent(ctx, contactList)
		if err != nil {
			t.Fatalf("Failed to save contact list %d: %v", i, err)
		}
	}

	// Get target serial
	targetSerial, err := db.GetPubkeySerial(targetPubkey)
	if err != nil {
		t.Fatalf("Failed to get target serial: %v", err)
	}

	// Get followers
	followers, err := db.GetFollowersOfPubkeySerial(targetSerial)
	if err != nil {
		t.Fatalf("GetFollowersOfPubkeySerial failed: %v", err)
	}

	if len(followers) != 2 {
		t.Errorf("Expected 2 followers, got %d", len(followers))
	}

	// Verify the followers
	expectedFollowers := map[string]bool{
		hex.Enc(follower1): false,
		hex.Enc(follower2): false,
	}
	for _, serial := range followers {
		pubkey, err := db.GetPubkeyBySerial(serial)
		if err != nil {
			t.Errorf("Failed to get pubkey from serial: %v", err)
			continue
		}
		pkHex := hex.Enc(pubkey)
		if _, exists := expectedFollowers[pkHex]; exists {
			expectedFollowers[pkHex] = true
		} else {
			t.Errorf("Unexpected follower: %s", pkHex)
		}
	}
	for pk, found := range expectedFollowers {
		if !found {
			t.Errorf("Expected follower not found: %s", pk)
		}
	}
}

func TestPubkeyHexToSerial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := New(ctx, cancel, t.TempDir(), "info")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a pubkey by saving an event
	pubkeyBytes, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	eventID := make([]byte, 32)
	eventID[0] = 0x10
	eventSig := make([]byte, 64)
	eventSig[0] = 0x10

	ev := &event.E{
		ID:        eventID,
		Pubkey:    pubkeyBytes,
		CreatedAt: 1234567890,
		Kind:      1,
		Content:   []byte("Test"),
		Sig:       eventSig,
		Tags:      &tag.S{},
	}
	_, err = db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Convert hex to serial
	pubkeyHex := hex.Enc(pubkeyBytes)
	serial, err := db.PubkeyHexToSerial(pubkeyHex)
	if err != nil {
		t.Fatalf("PubkeyHexToSerial failed: %v", err)
	}
	if serial == nil {
		t.Fatal("Expected non-nil serial")
	}

	// Convert back and verify
	backToHex, err := db.GetPubkeyHexFromSerial(serial)
	if err != nil {
		t.Fatalf("GetPubkeyHexFromSerial failed: %v", err)
	}
	if backToHex != pubkeyHex {
		t.Errorf("Round-trip failed: %s != %s", backToHex, pubkeyHex)
	}
}
