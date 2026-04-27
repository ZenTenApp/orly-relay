package types

import (
	"bytes"
	"testing"

	"git.smesh.lol/orly/pkg/lol/chk"
	"github.com/minio/sha256-simd"
	"git.smesh.lol/orly/pkg/utils"
)

func TestFromIdent(t *testing.T) {
	var err error
	// Create a test identity
	testIdent := []byte("test-identity")

	// Calculate the expected hash
	idh := sha256.Sum256(testIdent)
	expected := idh[:IdentLen]

	// Test FromIdent
	i := &Ident{}
	i.FromIdent(testIdent)
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
	}

	// Verify the hash was set correctly
	if !utils.FastEqual(i.Bytes(), expected) {
		t.Errorf(
			"FromIdent did not set the hash correctly: got %v, want %v",
			i.Bytes(), expected,
		)
	}
}

func TestIdent_MarshalWriteUnmarshalRead(t *testing.T) {
	var err error
	// Create a Ident with a known value
	i1 := &Ident{}
	testIdent := []byte("test-identity")
	i1.FromIdent(testIdent)
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
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
	i2 := &Ident{}
	err = i2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if !utils.FastEqual(i2.Bytes(), i1.Bytes()) {
		t.Errorf("UnmarshalRead read %v, want %v", i2.Bytes(), i1.Bytes())
	}
}

func TestIdent_UnmarshalReadWithCorruptedData(t *testing.T) {
	var err error
	// Create a Ident with a known value
	i1 := &Ident{}
	testIdent1 := []byte("test-identity-1")
	i1.FromIdent(testIdent1)
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
	}

	// Create a second Ident with a different value
	i2 := &Ident{}
	testIdent2 := []byte("test-identity-2")
	i2.FromIdent(testIdent2)
	if chk.E(err) {
		t.Fatalf("FromIdent failed: %v", err)
	}

	// Test UnmarshalRead with corrupted data (less than IdentLen bytes)
	corruptedData := make([]byte, IdentLen/2)
	i2.UnmarshalRead(bytes.NewBuffer(corruptedData))

	// The UnmarshalRead method should not have copied the original data to itself
	// before reading, so the value should be partially overwritten
	if utils.FastEqual(i2.Bytes(), i1.Bytes()) {
		t.Errorf("UnmarshalRead did not modify the value as expected")
	}
}
