//go:build integration
// +build integration

package neo4j

import (
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// All tests in this file use the shared testDB instance from testmain_test.go
// to avoid Neo4j authentication rate limiting from too many connections.

func TestNIP43_AddAndRemoveMember(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	signer, _ := p8k.New()
	signer.Generate()
	pubkey := signer.Pub()

	// Add member
	inviteCode := "test-invite-123"
	if err := testDB.AddNIP43Member(pubkey, inviteCode); err != nil {
		t.Fatalf("Failed to add NIP-43 member: %v", err)
	}

	// Check membership
	isMember, err := testDB.IsNIP43Member(pubkey)
	if err != nil {
		t.Fatalf("Failed to check membership: %v", err)
	}
	if !isMember {
		t.Fatal("Expected pubkey to be a member")
	}

	// Get membership details
	membership, err := testDB.GetNIP43Membership(pubkey)
	if err != nil {
		t.Fatalf("Failed to get membership: %v", err)
	}
	if membership.InviteCode != inviteCode {
		t.Fatalf("Invite code mismatch: got %s, expected %s", membership.InviteCode, inviteCode)
	}

	// Remove member
	if err := testDB.RemoveNIP43Member(pubkey); err != nil {
		t.Fatalf("Failed to remove member: %v", err)
	}

	// Verify no longer a member
	isMember, _ = testDB.IsNIP43Member(pubkey)
	if isMember {
		t.Fatal("Expected pubkey to not be a member after removal")
	}

	t.Logf("✓ NIP-43 add and remove member works correctly")
}

func TestNIP43_GetAllMembers(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	// Add multiple members
	var pubkeys [][]byte
	for i := 0; i < 3; i++ {
		signer, _ := p8k.New()
		signer.Generate()
		pubkey := signer.Pub()
		pubkeys = append(pubkeys, pubkey)

		if err := testDB.AddNIP43Member(pubkey, "invite"+string(rune('A'+i))); err != nil {
			t.Fatalf("Failed to add member %d: %v", i, err)
		}
	}

	// Get all members
	members, err := testDB.GetAllNIP43Members()
	if err != nil {
		t.Fatalf("Failed to get all members: %v", err)
	}

	if len(members) != 3 {
		t.Fatalf("Expected 3 members, got %d", len(members))
	}

	// Verify all added pubkeys are in the members list
	memberMap := make(map[string]bool)
	for _, m := range members {
		memberMap[hex.Enc(m)] = true
	}

	for i, pk := range pubkeys {
		if !memberMap[hex.Enc(pk)] {
			t.Fatalf("Member %d not found in list", i)
		}
	}

	t.Logf("✓ GetAllNIP43Members returned %d members", len(members))
}

func TestNIP43_InviteCode(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	// Store valid invite code (expires in 1 hour)
	validCode := "valid-code-123"
	expiresAt := time.Now().Add(1 * time.Hour)
	if err := testDB.StoreInviteCode(validCode, expiresAt); err != nil {
		t.Fatalf("Failed to store invite code: %v", err)
	}

	// Validate the code
	isValid, err := testDB.ValidateInviteCode(validCode)
	if err != nil {
		t.Fatalf("Failed to validate invite code: %v", err)
	}
	if !isValid {
		t.Fatal("Expected valid invite code to be valid")
	}

	// Test non-existent code
	isValid, err = testDB.ValidateInviteCode("non-existent-code")
	if err != nil {
		t.Fatalf("Failed to validate non-existent code: %v", err)
	}
	if isValid {
		t.Fatal("Expected non-existent code to be invalid")
	}

	// Delete the invite code
	if err := testDB.DeleteInviteCode(validCode); err != nil {
		t.Fatalf("Failed to delete invite code: %v", err)
	}

	// Verify code is no longer valid
	isValid, _ = testDB.ValidateInviteCode(validCode)
	if isValid {
		t.Fatal("Expected deleted code to be invalid")
	}

	t.Logf("✓ NIP-43 invite code operations work correctly")
}

func TestNIP43_ExpiredInviteCode(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	// Store expired invite code (expired 1 hour ago)
	expiredCode := "expired-code-123"
	expiresAt := time.Now().Add(-1 * time.Hour)
	if err := testDB.StoreInviteCode(expiredCode, expiresAt); err != nil {
		t.Fatalf("Failed to store expired invite code: %v", err)
	}

	// Validate should return false for expired code
	isValid, err := testDB.ValidateInviteCode(expiredCode)
	if err != nil {
		t.Fatalf("Failed to validate expired code: %v", err)
	}
	if isValid {
		t.Fatal("Expected expired code to be invalid")
	}

	t.Logf("✓ Expired invite code correctly detected as invalid")
}

func TestNIP43_DuplicateMember(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	signer, _ := p8k.New()
	signer.Generate()
	pubkey := signer.Pub()

	// Add member first time
	if err := testDB.AddNIP43Member(pubkey, "invite1"); err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// Add same member again (should not error, just update)
	if err := testDB.AddNIP43Member(pubkey, "invite2"); err != nil {
		t.Fatalf("Failed to re-add member: %v", err)
	}

	// Check membership still exists
	isMember, _ := testDB.IsNIP43Member(pubkey)
	if !isMember {
		t.Fatal("Expected pubkey to still be a member")
	}

	// Get all members should have only 1 entry
	members, _ := testDB.GetAllNIP43Members()
	if len(members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(members))
	}

	t.Logf("✓ Duplicate member handling works correctly")
}

func TestNIP43_MembershipPersistence(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	cleanTestDatabase()

	signer, _ := p8k.New()
	signer.Generate()
	pubkey := signer.Pub()

	// Add member
	inviteCode := "persistence-test"
	if err := testDB.AddNIP43Member(pubkey, inviteCode); err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// Get membership and verify all fields
	membership, err := testDB.GetNIP43Membership(pubkey)
	if err != nil {
		t.Fatalf("Failed to get membership: %v", err)
	}

	if membership.InviteCode != inviteCode {
		t.Fatalf("InviteCode mismatch")
	}

	if membership.AddedAt.IsZero() {
		t.Fatal("AddedAt should not be zero")
	}

	// Verify the pubkey in membership matches
	if hex.Enc(membership.Pubkey[:]) != hex.Enc(pubkey) {
		t.Fatal("Pubkey mismatch in membership")
	}

	t.Logf("✓ NIP-43 membership persistence verified")
}
