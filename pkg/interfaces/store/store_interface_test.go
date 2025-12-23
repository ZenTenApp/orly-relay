package store

import (
	"bytes"
	"testing"

	ntypes "git.mleku.dev/mleku/nostr/types"
)

func TestIdPkTsFixedMethods(t *testing.T) {
	// Create an IdPkTs with sample data
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 32)
	}

	ipk := &IdPkTs{
		Id:  id,
		Pub: pub,
		Ts:  1234567890,
		Ser: 42,
	}

	// Test IDFixed returns correct data
	idFixed := ipk.IDFixed()
	if !bytes.Equal(idFixed[:], id) {
		t.Errorf("IDFixed: got %x, want %x", idFixed[:], id)
	}

	// Test IDFixed returns a copy
	idFixed[0] = 0xFF
	if ipk.Id[0] == 0xFF {
		t.Error("IDFixed should return a copy, not a reference")
	}

	// Test PubFixed returns correct data
	pubFixed := ipk.PubFixed()
	if !bytes.Equal(pubFixed[:], pub) {
		t.Errorf("PubFixed: got %x, want %x", pubFixed[:], pub)
	}

	// Test hex methods
	idHex := ipk.IDHex()
	expectedIDHex := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	if idHex != expectedIDHex {
		t.Errorf("IDHex: got %s, want %s", idHex, expectedIDHex)
	}
}

func TestEventRef(t *testing.T) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	// Create EventRef
	ref := NewEventRef(id, pub, 1234567890, 42)

	// Test accessors - need to get addressable values for slicing
	refID := ref.ID()
	refPub := ref.Pub()
	if !bytes.Equal(refID[:], id) {
		t.Error("ID() mismatch")
	}
	if !bytes.Equal(refPub[:], pub) {
		t.Error("Pub() mismatch")
	}
	if ref.Ts() != 1234567890 {
		t.Error("Ts() mismatch")
	}
	if ref.Ser() != 42 {
		t.Error("Ser() mismatch")
	}

	// Test copy-on-assignment
	ref2 := ref
	testID := ref.ID()
	testID[0] = 0xFF
	ref2ID := ref2.ID()
	if ref2ID[0] == 0xFF {
		t.Error("EventRef should copy on assignment")
	}

	// Test hex methods
	if len(ref.IDHex()) != 64 {
		t.Errorf("IDHex length: got %d, want 64", len(ref.IDHex()))
	}
}

func TestEventRefToIdPkTs(t *testing.T) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ref := NewEventRef(id, pub, 1234567890, 42)
	ipk := ref.ToIdPkTs()

	// Verify conversion
	if !bytes.Equal(ipk.Id, id) {
		t.Error("ToIdPkTs: Id mismatch")
	}
	if !bytes.Equal(ipk.Pub, pub) {
		t.Error("ToIdPkTs: Pub mismatch")
	}
	if ipk.Ts != 1234567890 {
		t.Error("ToIdPkTs: Ts mismatch")
	}
	if ipk.Ser != 42 {
		t.Error("ToIdPkTs: Ser mismatch")
	}

	// Verify independence (modifications don't affect original)
	ipk.Id[0] = 0xFF
	if ref.ID()[0] == 0xFF {
		t.Error("ToIdPkTs should create independent copy")
	}
}

func TestIdPkTsToEventRef(t *testing.T) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ipk := &IdPkTs{
		Id:  id,
		Pub: pub,
		Ts:  1234567890,
		Ser: 42,
	}

	ref := ipk.ToEventRef()

	// Verify conversion - need addressable values for slicing
	refID := ref.ID()
	refPub := ref.Pub()
	if !bytes.Equal(refID[:], id) {
		t.Error("ToEventRef: ID mismatch")
	}
	if !bytes.Equal(refPub[:], pub) {
		t.Error("ToEventRef: Pub mismatch")
	}
	if ref.Ts() != 1234567890 {
		t.Error("ToEventRef: Ts mismatch")
	}
	if ref.Ser() != 42 {
		t.Error("ToEventRef: Ser mismatch")
	}
}

func BenchmarkEventRefCopy(b *testing.B) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ref := NewEventRef(id, pub, 1234567890, 42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ref2 := ref // Copy (should stay on stack)
		_ = ref2
	}
}

func BenchmarkIdPkTsToEventRef(b *testing.B) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ipk := &IdPkTs{
		Id:  id,
		Pub: pub,
		Ts:  1234567890,
		Ser: 42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ref := ipk.ToEventRef()
		_ = ref
	}
}

func BenchmarkEventRefAccess(b *testing.B) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ref := NewEventRef(id, pub, 1234567890, 42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idCopy := ref.ID()
		pubCopy := ref.Pub()
		_ = idCopy
		_ = pubCopy
	}
}

func BenchmarkIdPkTsFixedAccess(b *testing.B) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ipk := &IdPkTs{
		Id:  id,
		Pub: pub,
		Ts:  1234567890,
		Ser: 42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idCopy := ipk.IDFixed()
		pubCopy := ipk.PubFixed()
		_ = idCopy
		_ = pubCopy
	}
}

// Ensure EventRef implements expected interface at compile time
var _ interface {
	ID() ntypes.EventID
	Pub() ntypes.Pubkey
	Ts() int64
	Ser() uint64
} = EventRef{}
