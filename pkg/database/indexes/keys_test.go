package indexes

import (
	"bytes"
	"io"
	"testing"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/utils"
)

// TestNewPrefix tests the NewPrefix function with and without arguments
func TestNewPrefix(t *testing.T) {
	// Test with no arguments (default prefix)
	defaultPrefix := NewPrefix()
	if len(defaultPrefix.Bytes()) != 3 {
		t.Errorf(
			"Default prefix should be 3 bytes, got %d",
			len(defaultPrefix.Bytes()),
		)
	}

	// Test with a valid prefix index
	validPrefix := NewPrefix(Event)
	if string(validPrefix.Bytes()) != string(EventPrefix) {
		t.Errorf("Expected prefix %q, got %q", EventPrefix, validPrefix.Bytes())
	}

	// Test with an invalid prefix index (should panic)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewPrefix should panic with invalid prefix index")
		}
	}()
	_ = NewPrefix(-1) // This should panic
}

// TestPrefixMethods tests the methods of the P struct
func TestPrefixMethods(t *testing.T) {
	// Create a prefix
	prefix := NewPrefix(Event)

	// Test Bytes method
	if !utils.FastEqual(prefix.Bytes(), []byte(EventPrefix)) {
		t.Errorf(
			"Bytes method returned %v, expected %v", prefix.Bytes(),
			[]byte(EventPrefix),
		)
	}

	// Test MarshalWrite method
	buf := new(bytes.Buffer)
	err := prefix.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}
	if !utils.FastEqual(buf.Bytes(), []byte(EventPrefix)) {
		t.Errorf(
			"MarshalWrite wrote %v, expected %v", buf.Bytes(),
			[]byte(EventPrefix),
		)
	}

	// Test UnmarshalRead method
	newPrefix := &P{}
	err = newPrefix.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}
	if !utils.FastEqual(newPrefix.Bytes(), []byte(EventPrefix)) {
		t.Errorf(
			"UnmarshalRead read %v, expected %v", newPrefix.Bytes(),
			[]byte(EventPrefix),
		)
	}
}

// TestPrefixFunction tests the Prefix function
func TestPrefixFunction(t *testing.T) {
	testCases := []struct {
		name     string
		index    int
		expected I
	}{
		{"Event", Event, EventPrefix},
		{"ID", Id, IdPrefix},
		{"FullIdPubkey", FullIdPubkey, FullIdPubkeyPrefix},
		{"Pubkey", Pubkey, PubkeyPrefix},
		{"CreatedAt", CreatedAt, CreatedAtPrefix},
		{"TagPubkey", TagPubkey, TagPubkeyPrefix},
		{"Tag", Tag, TagPrefix},
		{"Kind", Kind, KindPrefix},
		{"KindPubkey", KindPubkey, KindPubkeyPrefix},
		{"TagKind", TagKind, TagKindPrefix},
		{
			"TagKindPubkey", TagKindPubkey,
			TagKindPubkeyPrefix,
		},
		{"Invalid", -1, ""},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				result := Prefix(tc.index)
				if result != tc.expected {
					t.Errorf(
						"Prefix(%d) = %q, expected %q", tc.index, result,
						tc.expected,
					)
				}
			},
		)
	}
}

// TestIdentify tests the Identify function
func TestIdentify(t *testing.T) {
	testCases := []struct {
		name     string
		prefix   I
		expected int
	}{
		{"Event", EventPrefix, Event},
		{"ID", IdPrefix, Id},
		{"FullIdPubkey", FullIdPubkeyPrefix, FullIdPubkey},
		{"Pubkey", PubkeyPrefix, Pubkey},
		{"CreatedAt", CreatedAtPrefix, CreatedAt},
		{"TagPubkey", TagPubkeyPrefix, TagPubkey},
		{"Tag", TagPrefix, Tag},
		{"Kind", KindPrefix, Kind},
		{"KindPubkey", KindPubkeyPrefix, KindPubkey},
		{"TagKind", TagKindPrefix, TagKind},
		{
			"TagKindPubkey", TagKindPubkeyPrefix,
			TagKindPubkey,
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				result, err := Identify(bytes.NewReader([]byte(tc.prefix)))
				if chk.E(err) {
					t.Fatalf("Identify failed: %v", err)
				}
				if result != tc.expected {
					t.Errorf(
						"Identify(%q) = %d, expected %d", tc.prefix, result,
						tc.expected,
					)
				}
			},
		)
	}

	// Test with invalid data
	t.Run(
		"Invalid", func(t *testing.T) {
			result, err := Identify(bytes.NewReader([]byte("xyz")))
			if chk.E(err) {
				t.Fatalf("Identify failed: %v", err)
			}
			if result != 0 {
				t.Errorf(
					"Identify with invalid prefix should return 0, got %d",
					result,
				)
			}
		},
	)

	// Test with error from reader
	t.Run(
		"ReaderError", func(t *testing.T) {
			errReader := &errorReader{}
			result, err := Identify(errReader)
			if err == nil {
				t.Errorf("Identify should return error with failing reader")
			}
			if result != -1 {
				t.Errorf(
					"Identify with reader error should return -1, got %d",
					result,
				)
			}
		},
	)
}

// errorReader is a mock reader that always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

// TestTStruct tests the T struct and its methods
func TestTStruct(t *testing.T) {
	// Create some test encoders
	prefix := NewPrefix(Event)
	ser := new(types.Uint40)
	ser.Set(12345)

	// Test New function
	enc := New(prefix, ser)
	if len(enc.Encs) != 2 {
		t.Errorf("New should create T with 2 encoders, got %d", len(enc.Encs))
	}

	// Test MarshalWrite
	buf := new(bytes.Buffer)
	err := enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Test UnmarshalRead
	dec := New(NewPrefix(), new(types.Uint40))
	err = dec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	decodedPrefix := dec.Encs[0].(*P)
	decodedSer := dec.Encs[1].(*types.Uint40)
	if !utils.FastEqual(decodedPrefix.Bytes(), prefix.Bytes()) {
		t.Errorf(
			"Decoded prefix %v, expected %v", decodedPrefix.Bytes(),
			prefix.Bytes(),
		)
	}
	if decodedSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", decodedSer.Get(), ser.Get())
	}

	// Test with nil encoder
	encWithNil := New(prefix, nil, ser)
	buf.Reset()
	err = encWithNil.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite with nil encoder failed: %v", err)
	}
}

// TestEventFunctions tests the Event-related functions
func TestEventFunctions(t *testing.T) {
	// Test EventVars
	ser := EventVars()
	if ser == nil {
		t.Fatalf("EventVars should return non-nil *types.Uint40")
	}

	// Set a value
	ser.Set(12345)

	// Test EventEnc
	enc := EventEnc(ser)
	if len(enc.Encs) != 2 {
		t.Errorf(
			"EventEnc should create T with 2 encoders, got %d", len(enc.Encs),
		)
	}

	// Test EventDec
	dec := EventDec(ser)
	if len(dec.Encs) != 2 {
		t.Errorf(
			"EventDec should create T with 2 encoders, got %d", len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err := enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newSer := new(types.Uint40)
	newDec := EventDec(newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded value
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestIdFunctions tests the Id-related functions
func TestIdFunctions(t *testing.T) {
	// Test IdVars
	id, ser := IdVars()
	if id == nil || ser == nil {
		t.Fatalf("IdVars should return non-nil *types.IdHash and *types.Uint40")
	}

	// Set values
	id.Set([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	ser.Set(12345)

	// Test IdEnc
	enc := IdEnc(id, ser)
	if len(enc.Encs) != 3 {
		t.Errorf("IdEnc should create T with 3 encoders, got %d", len(enc.Encs))
	}

	// Test IdDec
	dec := IdDec(id, ser)
	if len(dec.Encs) != 3 {
		t.Errorf("IdDec should create T with 3 encoders, got %d", len(dec.Encs))
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err := enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newId, newSer := IdVars()
	newDec := IdDec(newId, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if !utils.FastEqual(newId.Bytes(), id.Bytes()) {
		t.Errorf("Decoded id %v, expected %v", newId.Bytes(), id.Bytes())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestIdPubkeyFunctions tests the FullIdPubkey-related functions
func TestIdPubkeyFunctions(t *testing.T) {
	// Test FullIdPubkeyVars
	ser, fid, p, ca := FullIdPubkeyVars()
	if ser == nil || fid == nil || p == nil || ca == nil {
		t.Fatalf("FullIdPubkeyVars should return non-nil values")
	}

	// Set values
	ser.Set(12345)
	err := fid.FromId(
		[]byte{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
		},
	)
	if chk.E(err) {
		t.Fatalf("FromId failed: %v", err)
	}
	err = p.FromPubkey(
		[]byte{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
		},
	)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}
	ca.Set(98765)

	// Test FullIdPubkeyEnc
	enc := FullIdPubkeyEnc(ser, fid, p, ca)
	if len(enc.Encs) != 5 {
		t.Errorf(
			"FullIdPubkeyEnc should create T with 5 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test FullIdPubkeyDec
	dec := FullIdPubkeyDec(ser, fid, p, ca)
	if len(dec.Encs) != 5 {
		t.Errorf(
			"FullIdPubkeyDec should create T with 5 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err = enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newSer, newFid, newP, newCa := FullIdPubkeyVars()
	newDec := FullIdPubkeyDec(newSer, newFid, newP, newCa)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
	if !utils.FastEqual(newFid.Bytes(), fid.Bytes()) {
		t.Errorf("Decoded id %v, expected %v", newFid.Bytes(), fid.Bytes())
	}
	if !utils.FastEqual(newP.Bytes(), p.Bytes()) {
		t.Errorf("Decoded pubkey hash %v, expected %v", newP.Bytes(), p.Bytes())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
}

// TestCreatedAtFunctions tests the CreatedAt-related functions
func TestCreatedAtFunctions(t *testing.T) {
	// Test CreatedAtVars
	ca, ser := CreatedAtVars()
	if ca == nil || ser == nil {
		t.Fatalf("CreatedAtVars should return non-nil values")
	}

	// Set values
	ca.Set(98765)
	ser.Set(12345)

	// Test CreatedAtEnc
	enc := CreatedAtEnc(ca, ser)
	if len(enc.Encs) != 3 {
		t.Errorf(
			"CreatedAtEnc should create T with 3 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test CreatedAtDec
	dec := CreatedAtDec(ca, ser)
	if len(dec.Encs) != 3 {
		t.Errorf(
			"CreatedAtDec should create T with 3 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err := enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newCa, newSer := CreatedAtVars()
	newDec := CreatedAtDec(newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestPubkeyFunctions tests the Pubkey-related functions
func TestPubkeyFunctions(t *testing.T) {
	// Test PubkeyVars
	p, ca, ser := PubkeyVars()
	if p == nil || ca == nil || ser == nil {
		t.Fatalf("PubkeyVars should return non-nil values")
	}

	// Set values
	err := p.FromPubkey(
		[]byte{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
		},
	)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}
	ca.Set(98765)
	ser.Set(12345)

	// Test PubkeyEnc
	enc := PubkeyEnc(p, ca, ser)
	if len(enc.Encs) != 4 {
		t.Errorf(
			"PubkeyEnc should create T with 4 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test PubkeyDec
	dec := PubkeyDec(p, ca, ser)
	if len(dec.Encs) != 4 {
		t.Errorf(
			"PubkeyDec should create T with 4 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err = enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newP, newCa, newSer := PubkeyVars()
	newDec := PubkeyDec(newP, newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if !utils.FastEqual(newP.Bytes(), p.Bytes()) {
		t.Errorf("Decoded pubkey hash %v, expected %v", newP.Bytes(), p.Bytes())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestPubkeyTagFunctions tests the TagPubkey-related functions
func TestPubkeyTagFunctions(t *testing.T) {
	// Test TagPubkeyVars
	k, v, p, ca, ser := TagPubkeyVars()
	if p == nil || k == nil || v == nil || ca == nil || ser == nil {
		t.Fatalf("TagPubkeyVars should return non-nil values")
	}

	// Set values
	err := p.FromPubkey(
		[]byte{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
		},
	)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}
	k.Set('e')
	v.FromIdent([]byte("test-value"))
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
	}
	ca.Set(98765)
	ser.Set(12345)

	// Test TagPubkeyEnc
	enc := TagPubkeyEnc(k, v, p, ca, ser)
	if len(enc.Encs) != 6 {
		t.Errorf(
			"TagPubkeyEnc should create T with 6 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test TagPubkeyDec
	dec := TagPubkeyDec(k, v, p, ca, ser)
	if len(dec.Encs) != 6 {
		t.Errorf(
			"TagPubkeyDec should create T with 6 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err = enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newK, newV, newP, newCa, newSer := TagPubkeyVars()
	newDec := TagPubkeyDec(newK, newV, newP, newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if !utils.FastEqual(newP.Bytes(), p.Bytes()) {
		t.Errorf("Decoded pubkey hash %v, expected %v", newP.Bytes(), p.Bytes())
	}
	if newK.Letter() != k.Letter() {
		t.Errorf(
			"Decoded key letter %c, expected %c", newK.Letter(), k.Letter(),
		)
	}
	if !utils.FastEqual(newV.Bytes(), v.Bytes()) {
		t.Errorf("Decoded value hash %v, expected %v", newV.Bytes(), v.Bytes())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestTagFunctions tests the Tag-related functions
func TestTagFunctions(t *testing.T) {
	var err error
	// Test TagVars
	k, v, ca, ser := TagVars()
	if k == nil || v == nil || ca == nil || ser == nil {
		t.Fatalf("TagVars should return non-nil values")
	}

	// Set values
	k.Set('e')
	v.FromIdent([]byte("test-value"))
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
	}
	ca.Set(98765)
	ser.Set(12345)

	// Test TagEnc
	enc := TagEnc(k, v, ca, ser)
	if len(enc.Encs) != 5 {
		t.Errorf(
			"TagEnc should create T with 5 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test TagDec
	dec := TagDec(k, v, ca, ser)
	if len(dec.Encs) != 5 {
		t.Errorf(
			"TagDec should create T with 5 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err = enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newK, newV, newCa, newSer := TagVars()
	newDec := TagDec(newK, newV, newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if newK.Letter() != k.Letter() {
		t.Errorf(
			"Decoded key letter %c, expected %c", newK.Letter(), k.Letter(),
		)
	}
	if !utils.FastEqual(newV.Bytes(), v.Bytes()) {
		t.Errorf("Decoded value hash %v, expected %v", newV.Bytes(), v.Bytes())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestKindFunctions tests the Kind-related functions
func TestKindFunctions(t *testing.T) {
	// Test KindVars
	ki, ca, ser := KindVars()
	if ki == nil || ca == nil || ser == nil {
		t.Fatalf("KindVars should return non-nil values")
	}

	// Set values
	ki.Set(1234)
	ca.Set(98765)
	ser.Set(12345)

	// Test KindEnc
	enc := KindEnc(ki, ca, ser)
	if len(enc.Encs) != 4 {
		t.Errorf(
			"KindEnc should create T with 4 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test KindDec
	dec := KindDec(ki, ca, ser)
	if len(dec.Encs) != 4 {
		t.Errorf(
			"KindDec should create T with 4 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err := enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newKi, newCa, newSer := KindVars()
	newDec := KindDec(newKi, newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if newKi.Get() != ki.Get() {
		t.Errorf("Decoded kind %d, expected %d", newKi.Get(), ki.Get())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestKindTagFunctions tests the TagKind-related functions
func TestKindTagFunctions(t *testing.T) {
	var err error
	// Test TagKindVars
	k, v, ki, ca, ser := TagKindVars()
	if ki == nil || k == nil || v == nil || ca == nil || ser == nil {
		t.Fatalf("TagKindVars should return non-nil values")
	}

	// Set values
	ki.Set(1234)
	k.Set('e')
	v.FromIdent([]byte("test-value"))
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
	}
	ca.Set(98765)
	ser.Set(12345)

	// Test TagKindEnc
	enc := TagKindEnc(k, v, ki, ca, ser)
	if len(enc.Encs) != 6 {
		t.Errorf(
			"TagKindEnc should create T with 6 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test TagKindDec
	dec := TagKindDec(k, v, ki, ca, ser)
	if len(dec.Encs) != 6 {
		t.Errorf(
			"TagKindDec should create T with 6 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err = enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newK, newV, newKi, newCa, newSer := TagKindVars()
	newDec := TagKindDec(newK, newV, newKi, newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if newKi.Get() != ki.Get() {
		t.Errorf("Decoded kind %d, expected %d", newKi.Get(), ki.Get())
	}
	if newK.Letter() != k.Letter() {
		t.Errorf(
			"Decoded key letter %c, expected %c", newK.Letter(), k.Letter(),
		)
	}
	if !utils.FastEqual(newV.Bytes(), v.Bytes()) {
		t.Errorf("Decoded value hash %v, expected %v", newV.Bytes(), v.Bytes())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestKindPubkeyFunctions tests the KindPubkey-related functions
func TestKindPubkeyFunctions(t *testing.T) {
	// Test KindPubkeyVars
	ki, p, ca, ser := KindPubkeyVars()
	if ki == nil || p == nil || ca == nil || ser == nil {
		t.Fatalf("KindPubkeyVars should return non-nil values")
	}

	// Set values
	ki.Set(1234)
	err := p.FromPubkey(
		[]byte{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
		},
	)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}
	ca.Set(98765)
	ser.Set(12345)

	// Test KindPubkeyEnc
	enc := KindPubkeyEnc(ki, p, ca, ser)
	if len(enc.Encs) != 5 {
		t.Errorf(
			"KindPubkeyEnc should create T with 5 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test KindPubkeyDec
	dec := KindPubkeyDec(ki, p, ca, ser)
	if len(dec.Encs) != 5 {
		t.Errorf(
			"KindPubkeyDec should create T with 5 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err = enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newKi, newP, newCa, newSer := KindPubkeyVars()
	newDec := KindPubkeyDec(newKi, newP, newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if newKi.Get() != ki.Get() {
		t.Errorf("Decoded kind %d, expected %d", newKi.Get(), ki.Get())
	}
	if !utils.FastEqual(newP.Bytes(), p.Bytes()) {
		t.Errorf("Decoded pubkey hash %v, expected %v", newP.Bytes(), p.Bytes())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}

// TestKindPubkeyTagFunctions tests the TagKindPubkey-related functions
func TestKindPubkeyTagFunctions(t *testing.T) {
	// Test TagKindPubkeyVars
	k, v, ki, p, ca, ser := TagKindPubkeyVars()
	if ki == nil || p == nil || k == nil || v == nil || ca == nil || ser == nil {
		t.Fatalf("TagKindPubkeyVars should return non-nil values")
	}

	// Set values
	ki.Set(1234)
	err := p.FromPubkey(
		[]byte{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
		},
	)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}
	k.Set('e')
	v.FromIdent([]byte("test-value"))
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
	}
	ca.Set(98765)
	ser.Set(12345)

	// Test TagKindPubkeyEnc
	enc := TagKindPubkeyEnc(k, v, ki, p, ca, ser)
	if len(enc.Encs) != 7 {
		t.Errorf(
			"TagKindPubkeyEnc should create T with 7 encoders, got %d",
			len(enc.Encs),
		)
	}

	// Test TagKindPubkeyDec
	dec := TagKindPubkeyDec(k, v, ki, p, ca, ser)
	if len(dec.Encs) != 7 {
		t.Errorf(
			"TagKindPubkeyDec should create T with 7 encoders, got %d",
			len(dec.Encs),
		)
	}

	// Test marshaling and unmarshaling
	buf := new(bytes.Buffer)
	err = enc.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Create new variables for decoding
	newK, newV, newKi, newP, newCa, newSer := TagKindPubkeyVars()
	newDec := TagKindPubkeyDec(newK, newV, newKi, newP, newCa, newSer)

	err = newDec.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the decoded values
	if newKi.Get() != ki.Get() {
		t.Errorf("Decoded kind %d, expected %d", newKi.Get(), ki.Get())
	}
	if !utils.FastEqual(newP.Bytes(), p.Bytes()) {
		t.Errorf("Decoded pubkey hash %v, expected %v", newP.Bytes(), p.Bytes())
	}
	if newK.Letter() != k.Letter() {
		t.Errorf(
			"Decoded key letter %c, expected %c", newK.Letter(), k.Letter(),
		)
	}
	if !utils.FastEqual(newV.Bytes(), v.Bytes()) {
		t.Errorf("Decoded value hash %v, expected %v", newV.Bytes(), v.Bytes())
	}
	if newCa.Get() != ca.Get() {
		t.Errorf("Decoded created at %d, expected %d", newCa.Get(), ca.Get())
	}
	if newSer.Get() != ser.Get() {
		t.Errorf("Decoded serial %d, expected %d", newSer.Get(), ser.Get())
	}
}
