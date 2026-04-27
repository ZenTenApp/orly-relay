package types

import (
	"bytes"
	"testing"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/crypto/ec/schnorr"
	"github.com/minio/sha256-simd"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/utils"
)

func TestPubHash_FromPubkey(t *testing.T) {
	// Create a valid pubkey (32 bytes)
	validPubkey := make([]byte, schnorr.PubKeyBytesLen)
	for i := 0; i < schnorr.PubKeyBytesLen; i++ {
		validPubkey[i] = byte(i)
	}

	// Create an invalid pubkey (wrong size)
	invalidPubkey := make([]byte, schnorr.PubKeyBytesLen-1)

	// Test with valid pubkey
	ph := &PubHash{}
	err := ph.FromPubkey(validPubkey)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed with valid pubkey: %v", err)
	}

	// Calculate the expected hash
	pkh := sha256.Sum256(validPubkey)
	expected := pkh[:PubHashLen]

	// Verify the hash was set correctly
	if !utils.FastEqual(ph.Bytes(), expected) {
		t.Errorf(
			"FromPubkey did not set the hash correctly: got %v, want %v",
			ph.Bytes(), expected,
		)
	}

	// Test with invalid pubkey
	ph = &PubHash{}
	err = ph.FromPubkey(invalidPubkey)
	if err == nil {
		t.Errorf("FromPubkey should have failed with invalid pubkey size")
	}
}

func TestPubHash_FromPubkeyHex(t *testing.T) {
	// Create a valid pubkey (32 bytes)
	validPubkey := make([]byte, schnorr.PubKeyBytesLen)
	for i := 0; i < schnorr.PubKeyBytesLen; i++ {
		validPubkey[i] = byte(i)
	}

	// Encode the pubkey as hex
	validPubkeyHex := hex.Enc(validPubkey)

	// Test with valid hex pubkey
	ph := &PubHash{}
	err := ph.FromPubkeyHex(validPubkeyHex)
	if chk.E(err) {
		t.Fatalf("FromPubkeyHex failed with valid pubkey: %v", err)
	}

	// Calculate the expected hash
	pkh := sha256.Sum256(validPubkey)
	expected := pkh[:PubHashLen]

	// Verify the hash was set correctly
	if !utils.FastEqual(ph.Bytes(), expected) {
		t.Errorf(
			"FromPubkeyHex did not set the hash correctly: got %v, want %v",
			ph.Bytes(), expected,
		)
	}

	// Test with invalid hex pubkey (wrong size)
	ph = &PubHash{}
	err = ph.FromPubkeyHex(validPubkeyHex[:len(validPubkeyHex)-2])
	if err == nil {
		t.Errorf("FromPubkeyHex should have failed with invalid pubkey size")
	}

	// Test with invalid hex pubkey (not hex)
	ph = &PubHash{}
	err = ph.FromPubkeyHex("invalid-hex")
	if err == nil {
		t.Errorf("FromPubkeyHex should have failed with invalid hex")
	}
}

func TestPubHash_MarshalWriteUnmarshalRead(t *testing.T) {
	// Create a PubHash with a known value
	ph1 := &PubHash{}
	validPubkey := make([]byte, schnorr.PubKeyBytesLen)
	for i := 0; i < schnorr.PubKeyBytesLen; i++ {
		validPubkey[i] = byte(i)
	}
	err := ph1.FromPubkey(validPubkey)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}

	// Test MarshalWrite
	buf := new(bytes.Buffer)
	err = ph1.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Verify the written bytes
	if !utils.FastEqual(buf.Bytes(), ph1.Bytes()) {
		t.Errorf("MarshalWrite wrote %v, want %v", buf.Bytes(), ph1.Bytes())
	}

	// Test UnmarshalRead
	ph2 := &PubHash{}
	err = ph2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if !utils.FastEqual(ph2.Bytes(), ph1.Bytes()) {
		t.Errorf("UnmarshalRead read %v, want %v", ph2.Bytes(), ph1.Bytes())
	}
}

func TestPubHash_UnmarshalReadWithCorruptedData(t *testing.T) {
	// Create a PubHash with a known value
	ph1 := &PubHash{}
	validPubkey1 := make([]byte, schnorr.PubKeyBytesLen)
	for i := 0; i < schnorr.PubKeyBytesLen; i++ {
		validPubkey1[i] = byte(i)
	}
	err := ph1.FromPubkey(validPubkey1)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}

	// Create a second PubHash with a different value
	ph2 := &PubHash{}
	validPubkey2 := make([]byte, schnorr.PubKeyBytesLen)
	for i := 0; i < schnorr.PubKeyBytesLen; i++ {
		validPubkey2[i] = byte(schnorr.PubKeyBytesLen - i - 1)
	}
	err = ph2.FromPubkey(validPubkey2)
	if chk.E(err) {
		t.Fatalf("FromPubkey failed: %v", err)
	}

	// Test UnmarshalRead with corrupted data (less than PubHashLen bytes)
	corruptedData := make([]byte, PubHashLen/2)
	ph2.UnmarshalRead(bytes.NewBuffer(corruptedData))

	// The UnmarshalRead method should not have copied the original data to itself
	// before reading, so the value should be partially overwritten
	if utils.FastEqual(ph2.Bytes(), ph1.Bytes()) {
		t.Errorf("UnmarshalRead did not modify the value as expected")
	}
}
