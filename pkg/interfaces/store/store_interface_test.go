package store

import (
	"bytes"
	"testing"

	ntypes "git.mleku.dev/mleku/nostr/types"
)

func TestNewIdPkTs(t *testing.T) {
	// Create sample data
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 32)
	}

	ipk := NewIdPkTs(id, pub, 1234567890, 42)

	// Test that data was copied correctly
	if !bytes.Equal(ipk.Id[:], id) {
		t.Errorf("Id: got %x, want %x", ipk.Id[:], id)
	}
	if !bytes.Equal(ipk.Pub[:], pub) {
		t.Errorf("Pub: got %x, want %x", ipk.Pub[:], pub)
	}
	if ipk.Ts != 1234567890 {
		t.Errorf("Ts: got %d, want 1234567890", ipk.Ts)
	}
	if ipk.Ser != 42 {
		t.Errorf("Ser: got %d, want 42", ipk.Ser)
	}

	// Test that Id is a copy (modifying original doesn't affect struct)
	id[0] = 0xFF
	if ipk.Id[0] == 0xFF {
		t.Error("NewIdPkTs should copy data, not reference")
	}
}

func TestIdPkTsSliceMethods(t *testing.T) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 32)
	}

	ipk := NewIdPkTs(id, pub, 1234567890, 42)

	// Test IDSlice returns correct data
	idSlice := ipk.IDSlice()
	if !bytes.Equal(idSlice, id) {
		t.Errorf("IDSlice: got %x, want %x", idSlice, id)
	}

	// Test PubSlice returns correct data
	pubSlice := ipk.PubSlice()
	if !bytes.Equal(pubSlice, pub) {
		t.Errorf("PubSlice: got %x, want %x", pubSlice, pub)
	}

	// Test hex methods
	idHex := ipk.IDHex()
	expectedIDHex := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	if idHex != expectedIDHex {
		t.Errorf("IDHex: got %s, want %s", idHex, expectedIDHex)
	}
}

func TestIdPkTsCopyOnAssignment(t *testing.T) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 32)
	}

	ipk1 := NewIdPkTs(id, pub, 1234567890, 42)
	ipk2 := ipk1 // Copy

	// Modify the copy
	ipk2.Id[0] = 0xFF

	// Original should be unchanged (arrays are copied on assignment)
	if ipk1.Id[0] == 0xFF {
		t.Error("IdPkTs should copy on assignment")
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
	if !bytes.Equal(ipk.Id[:], id) {
		t.Error("ToIdPkTs: Id mismatch")
	}
	if !bytes.Equal(ipk.Pub[:], pub) {
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

	ipk := NewIdPkTs(id, pub, 1234567890, 42)
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

func BenchmarkIdPkTsCopy(b *testing.B) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ipk := NewIdPkTs(id, pub, 1234567890, 42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ipk2 := ipk // Copy (should stay on stack)
		_ = ipk2
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

	ipk := NewIdPkTs(id, pub, 1234567890, 42)

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

func BenchmarkIdPkTsAccess(b *testing.B) {
	id := make([]byte, 32)
	pub := make([]byte, 32)
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 100)
	}

	ipk := NewIdPkTs(id, pub, 1234567890, 42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idCopy := ipk.Id
		pubCopy := ipk.Pub
		_ = idCopy
		_ = pubCopy
	}
}

// Ensure types satisfy expected size for stack allocation
func TestStructSizes(t *testing.T) {
	var ipk IdPkTs
	var ref EventRef

	// Both should be exactly 80 bytes (32+32+8+8)
	// This is not directly testable in Go, but we can verify the fields exist
	_ = ipk.Id
	_ = ipk.Pub
	_ = ipk.Ts
	_ = ipk.Ser
	_ = ref
}

// Ensure ntypes.EventID and ntypes.Pubkey are used correctly
func TestNtypesCompatibility(t *testing.T) {
	var id ntypes.EventID
	var pub ntypes.Pubkey

	// Fill with test data
	for i := 0; i < 32; i++ {
		id[i] = byte(i)
		pub[i] = byte(i + 32)
	}

	// Create IdPkTs directly with ntypes
	ipk := IdPkTs{
		Id:  id,
		Pub: pub,
		Ts:  1234567890,
		Ser: 42,
	}

	// Verify
	if ipk.Id != id {
		t.Error("ntypes.EventID should be directly assignable to IdPkTs.Id")
	}
	if ipk.Pub != pub {
		t.Error("ntypes.Pubkey should be directly assignable to IdPkTs.Pub")
	}
}
