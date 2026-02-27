package event

import (
	"bytes"
	"testing"

	"git.mleku.dev/mleku/nostr/types"
)

func TestIDFixed(t *testing.T) {
	ev := New()
	ev.ID = make([]byte, 32)
	for i := range ev.ID {
		ev.ID[i] = byte(i)
	}

	// Get fixed ID
	id := ev.IDFixed()

	// Verify contents match
	if !bytes.Equal(id[:], ev.ID) {
		t.Errorf("IDFixed: contents don't match")
	}

	// Verify it's a copy (modification doesn't affect original)
	id[0] = 0xFF
	if ev.ID[0] == 0xFF {
		t.Error("IDFixed: should return a copy, not a reference")
	}
}

func TestSetIDFixed(t *testing.T) {
	ev := New()

	// Create fixed ID
	var id types.EventID
	for i := range id {
		id[i] = byte(i + 100)
	}

	// Set it
	ev.SetIDFixed(id)

	// Verify slice was updated
	if !bytes.Equal(ev.ID, id[:]) {
		t.Errorf("SetIDFixed: slice not updated correctly")
	}

	// Verify the slice is independent
	id[0] = 0xFF
	if ev.ID[0] == 0xFF {
		t.Error("SetIDFixed: slice should be independent of input")
	}
}

func TestPubkeyFixed(t *testing.T) {
	ev := New()
	ev.Pubkey = make([]byte, 32)
	for i := range ev.Pubkey {
		ev.Pubkey[i] = byte(i + 32)
	}

	pk := ev.PubkeyFixed()

	if !bytes.Equal(pk[:], ev.Pubkey) {
		t.Errorf("PubkeyFixed: contents don't match")
	}

	// Verify it's a copy
	pk[0] = 0xFF
	if ev.Pubkey[0] == 0xFF {
		t.Error("PubkeyFixed: should return a copy")
	}
}

func TestSetPubkeyFixed(t *testing.T) {
	ev := New()

	var pk types.Pubkey
	for i := range pk {
		pk[i] = byte(i + 50)
	}

	ev.SetPubkeyFixed(pk)

	if !bytes.Equal(ev.Pubkey, pk[:]) {
		t.Errorf("SetPubkeyFixed: slice not updated correctly")
	}
}

func TestSigFixed(t *testing.T) {
	ev := New()
	ev.Sig = make([]byte, 64)
	for i := range ev.Sig {
		ev.Sig[i] = byte(i)
	}

	sig := ev.SigFixed()

	if !bytes.Equal(sig[:], ev.Sig) {
		t.Errorf("SigFixed: contents don't match")
	}

	// Verify it's a copy
	sig[0] = 0xFF
	if ev.Sig[0] == 0xFF {
		t.Error("SigFixed: should return a copy")
	}
}

func TestHexMethods(t *testing.T) {
	ev := New()
	ev.ID = make([]byte, 32)
	ev.Pubkey = make([]byte, 32)
	ev.Sig = make([]byte, 64)

	for i := 0; i < 32; i++ {
		ev.ID[i] = byte(i)
		ev.Pubkey[i] = byte(i + 32)
	}
	for i := 0; i < 64; i++ {
		ev.Sig[i] = byte(i)
	}

	// Test hex output
	idHex := ev.IDHex()
	if len(idHex) != 64 {
		t.Errorf("IDHex: expected 64 chars, got %d", len(idHex))
	}

	pkHex := ev.PubkeyHex()
	if len(pkHex) != 64 {
		t.Errorf("PubkeyHex: expected 64 chars, got %d", len(pkHex))
	}

	sigHex := ev.SigHex()
	if len(sigHex) != 128 {
		t.Errorf("SigHex: expected 128 chars, got %d", len(sigHex))
	}

	// Verify lowercase
	expectedIDHex := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	if idHex != expectedIDHex {
		t.Errorf("IDHex: got %s, want %s", idHex, expectedIDHex)
	}
}

func TestCopyOnAssignment(t *testing.T) {
	ev := New()
	ev.ID = make([]byte, 32)
	for i := range ev.ID {
		ev.ID[i] = byte(i)
	}

	// Get fixed ID
	id1 := ev.IDFixed()
	id2 := id1 // This should copy the array

	// Modify id2
	id2[0] = 0xFF

	// id1 should be unchanged (arrays copy on assignment)
	if id1[0] == 0xFF {
		t.Error("Array assignment should create a copy")
	}

	// Original event should be unchanged
	if ev.ID[0] == 0xFF {
		t.Error("Original event should be unchanged")
	}
}

func BenchmarkIDFixed(b *testing.B) {
	ev := New()
	ev.ID = make([]byte, 32)
	for i := range ev.ID {
		ev.ID[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ev.IDFixed()
		_ = id
	}
}

func BenchmarkIDSliceCopy(b *testing.B) {
	ev := New()
	ev.ID = make([]byte, 32)
	for i := range ev.ID {
		ev.ID[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := make([]byte, 32)
		copy(id, ev.ID)
		_ = id
	}
}
