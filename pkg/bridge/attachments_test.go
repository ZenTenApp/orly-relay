package bridge

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptAttachment(t *testing.T) {
	original := []byte("This is a test attachment with some content.")

	ciphertext, keyHex, err := EncryptAttachment(original)
	if err != nil {
		t.Fatalf("EncryptAttachment: %v", err)
	}

	if len(keyHex) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("key hex length = %d, want 64", len(keyHex))
	}

	if bytes.Equal(ciphertext, original) {
		t.Error("ciphertext should differ from plaintext")
	}

	// Decrypt
	decrypted, err := DecryptAttachment(ciphertext, keyHex)
	if err != nil {
		t.Fatalf("DecryptAttachment: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Error("decrypted content doesn't match original")
	}
}

func TestEncryptDecryptAttachment_LargeData(t *testing.T) {
	// 1MB of test data
	original := make([]byte, 1024*1024)
	for i := range original {
		original[i] = byte(i % 256)
	}

	ciphertext, keyHex, err := EncryptAttachment(original)
	if err != nil {
		t.Fatalf("EncryptAttachment: %v", err)
	}

	decrypted, err := DecryptAttachment(ciphertext, keyHex)
	if err != nil {
		t.Fatalf("DecryptAttachment: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Error("large data round-trip failed")
	}
}

func TestEncryptDecryptAttachment_EmptyData(t *testing.T) {
	original := []byte{}

	ciphertext, keyHex, err := EncryptAttachment(original)
	if err != nil {
		t.Fatalf("EncryptAttachment: %v", err)
	}

	decrypted, err := DecryptAttachment(ciphertext, keyHex)
	if err != nil {
		t.Fatalf("DecryptAttachment: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Error("empty data round-trip failed")
	}
}

func TestDecryptAttachment_WrongKey(t *testing.T) {
	original := []byte("secret data")

	ciphertext, _, err := EncryptAttachment(original)
	if err != nil {
		t.Fatalf("EncryptAttachment: %v", err)
	}

	// Use a different key
	wrongKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err = DecryptAttachment(ciphertext, wrongKey)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestDecryptAttachment_InvalidKeyHex(t *testing.T) {
	_, err := DecryptAttachment([]byte("data"), "not-hex")
	if err == nil {
		t.Error("expected error for invalid hex key")
	}
}

func TestDecryptAttachment_ShortCiphertext(t *testing.T) {
	// Ciphertext shorter than nonce
	_, err := DecryptAttachment([]byte("short"), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err == nil {
		t.Error("expected error for short ciphertext")
	}
}

func TestEncryptAttachment_DifferentKeys(t *testing.T) {
	original := []byte("same data")

	_, key1, _ := EncryptAttachment(original)
	_, key2, _ := EncryptAttachment(original)

	if key1 == key2 {
		t.Error("encrypting same data twice should produce different keys")
	}
}
