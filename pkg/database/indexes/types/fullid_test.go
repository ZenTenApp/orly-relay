package types

import (
	"bytes"
	"testing"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/utils"

	"github.com/minio/sha256-simd"
)

func TestFromId(t *testing.T) {
	// Create a valid ID (32 bytes)
	validId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		validId[i] = byte(i)
	}

	// Create an invalid ID (wrong size)
	invalidId := make([]byte, sha256.Size-1)

	// Test with valid ID
	fi := &Id{}
	err := fi.FromId(validId)
	if chk.E(err) {
		t.Fatalf("FromId failed with valid ID: %v", err)
	}

	// Verify the ID was set correctly
	if !utils.FastEqual(fi.Bytes(), validId) {
		t.Errorf(
			"FromId did not set the ID correctly: got %v, want %v", fi.Bytes(),
			validId,
		)
	}

	// Test with invalid ID
	fi = &Id{}
	err = fi.FromId(invalidId)
	if err == nil {
		t.Errorf("FromId should have failed with invalid ID size")
	}
}

func TestIdMarshalWriteUnmarshalRead(t *testing.T) {
	// Create a ID with a known value
	fi1 := &Id{}
	validId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		validId[i] = byte(i)
	}
	err := fi1.FromId(validId)
	if chk.E(err) {
		t.Fatalf("FromId failed: %v", err)
	}

	// Test MarshalWrite
	buf := new(bytes.Buffer)
	err = fi1.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Verify the written bytes
	if !utils.FastEqual(buf.Bytes(), validId) {
		t.Errorf("MarshalWrite wrote %v, want %v", buf.Bytes(), validId)
	}

	// Test UnmarshalRead
	fi2 := &Id{}
	err = fi2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if !utils.FastEqual(fi2.Bytes(), validId) {
		t.Errorf("UnmarshalRead read %v, want %v", fi2.Bytes(), validId)
	}
}

func TestIdUnmarshalReadWithCorruptedData(t *testing.T) {
	// Create a ID with a known value
	fi1 := &Id{}
	validId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		validId[i] = byte(i)
	}
	err := fi1.FromId(validId)
	if chk.E(err) {
		t.Fatalf("FromId failed: %v", err)
	}

	// Create a second ID with a different value
	fi2 := &Id{}
	differentId := make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		differentId[i] = byte(sha256.Size - i - 1)
	}
	err = fi2.FromId(differentId)
	if chk.E(err) {
		t.Fatalf("FromId failed: %v", err)
	}

	// Test UnmarshalRead with corrupted data (less than Len bytes)
	corruptedData := make([]byte, sha256.Size/2)
	fi2.UnmarshalRead(bytes.NewBuffer(corruptedData))

	// The UnmarshalRead method should not have copied the original data to itself
	// before reading, so the value should be partially overwritten
	if utils.FastEqual(fi2.Bytes(), differentId) {
		t.Errorf("UnmarshalRead did not modify the value as expected")
	}
}
