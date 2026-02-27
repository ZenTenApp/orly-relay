package types

import (
	"bytes"
	"encoding/base64"
	"testing"

	"next.orly.dev/pkg/lol/chk"
	"github.com/minio/sha256-simd"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/utils"
)

func TestFromIdHash(t *testing.T) {
	// Create a valid ID (32 bytes)
	validId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		validId[i] = byte(i)
	}

	// Create an invalid ID (wrong size)
	invalidId := make([]byte, sha256.Size-1)

	// Test with valid ID
	i := new(IdHash)
	err := i.FromId(validId)
	if chk.E(err) {
		t.Fatalf("FromId failed with valid ID: %v", err)
	}

	// Calculate the expected hash
	idh := sha256.Sum256(validId)
	expected := idh[:IdHashLen]

	// Verify the hash was set correctly
	if !utils.FastEqual(i.Bytes(), expected) {
		t.Errorf(
			"FromId did not set the hash correctly: got %v, want %v", i.Bytes(),
			expected,
		)
	}

	// Test with invalid ID
	i = new(IdHash)
	err = i.FromId(invalidId)
	if err == nil {
		t.Errorf("FromId should have failed with invalid ID size")
	}
}

func TestFromIdBase64(t *testing.T) {
	// Create a valid ID (32 bytes)
	validId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		validId[i] = byte(i)
	}

	// Encode the ID as base64
	validIdBase64 := base64.RawURLEncoding.EncodeToString(validId)

	// Test with valid base64 ID
	i := new(IdHash)
	err := i.FromIdBase64(validIdBase64)
	if chk.E(err) {
		t.Fatalf("FromIdBase64 failed with valid ID: %v", err)
	}

	// Calculate the expected hash
	idh := sha256.Sum256(validId)
	expected := idh[:IdHashLen]

	// Verify the hash was set correctly
	if !utils.FastEqual(i.Bytes(), expected) {
		t.Errorf(
			"FromIdBase64 did not set the hash correctly: got %v, want %v",
			i.Bytes(), expected,
		)
	}

	// Test with invalid base64 ID
	i = new(IdHash)
	err = i.FromIdBase64("invalid-base64")
	if err == nil {
		t.Errorf("FromIdBase64 should have failed with invalid base64")
	}
}

func TestFromIdHex(t *testing.T) {
	// Create a valid ID (32 bytes)
	validId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		validId[i] = byte(i)
	}

	// Encode the ID as hex
	validIdHex := hex.Enc(validId)

	// Test with valid hex ID
	i := new(IdHash)
	err := i.FromIdHex(validIdHex)
	if chk.E(err) {
		t.Fatalf("FromIdHex failed with valid ID: %v", err)
	}

	// Calculate the expected hash
	idh := sha256.Sum256(validId)
	expected := idh[:IdHashLen]

	// Verify the hash was set correctly
	if !utils.FastEqual(i.Bytes(), expected) {
		t.Errorf(
			"FromIdHex did not set the hash correctly: got %v, want %v",
			i.Bytes(), expected,
		)
	}

	// Test with invalid hex ID (wrong size)
	i = new(IdHash)
	err = i.FromIdHex(validIdHex[:len(validIdHex)-2])
	if err == nil {
		t.Errorf("FromIdHex should have failed with invalid ID size")
	}

	// Test with invalid hex ID (not hex)
	i = new(IdHash)
	err = i.FromIdHex("invalid-hex")
	if err == nil {
		t.Errorf("FromIdHex should have failed with invalid hex")
	}
}

func TestIdHashMarshalWriteUnmarshalRead(t *testing.T) {
	// Create a IdHash with a known value
	i1 := new(IdHash)
	validId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		validId[i] = byte(i)
	}
	err := i1.FromId(validId)
	if chk.E(err) {
		t.Fatalf("FromId failed: %v", err)
	}

	// Test MarshalWrite
	buf := new(bytes.Buffer)
	err = i1.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Verify the written bytes
	if !utils.FastEqual(buf.Bytes(), i1.Bytes()) {
		t.Errorf("MarshalWrite wrote %v, want %v", buf.Bytes(), i1.Bytes())
	}

	// Test UnmarshalRead
	i2 := new(IdHash)
	err = i2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if !utils.FastEqual(i2.Bytes(), i1.Bytes()) {
		t.Errorf("UnmarshalRead read %v, want %v", i2.Bytes(), i1.Bytes())
	}
}

func TestUnmarshalReadWithEmptyVal(t *testing.T) {
	// Create a IdHash with an empty val
	i := new(IdHash)

	// Create some test data
	testData := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	// Test UnmarshalRead
	err := i.UnmarshalRead(bytes.NewBuffer(testData))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if !utils.FastEqual(i.Bytes(), testData) {
		t.Errorf("UnmarshalRead read %v, want %v", i.Bytes(), testData)
	}
}
