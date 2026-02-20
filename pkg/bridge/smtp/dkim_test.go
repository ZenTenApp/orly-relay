package smtp

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDKIMSigner_SignMessage(t *testing.T) {
	// Generate a test key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	signer := NewDKIMSignerFromKey("example.com", "test", key)

	msg := "From: user@example.com\r\nTo: recipient@other.com\r\nSubject: Test\r\nDate: Mon, 01 Jan 2024 00:00:00 +0000\r\nMessage-Id: <test@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nHello world"

	signed, err := signer.Sign([]byte(msg))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Signed message should contain DKIM-Signature header
	if !strings.Contains(string(signed), "DKIM-Signature") {
		t.Error("signed message missing DKIM-Signature header")
	}

	// Should still contain original content
	if !strings.Contains(string(signed), "Hello world") {
		t.Error("signed message missing original body")
	}
	if !strings.Contains(string(signed), "Subject: Test") {
		t.Error("signed message missing Subject header")
	}
}

func TestNewDKIMSigner_FromFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "dkim.key")

	// Generate and save a key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	privPEM, _, err := GenerateDKIMKeyPair()
	if err != nil {
		t.Fatalf("GenerateDKIMKeyPair: %v", err)
	}

	if err := os.WriteFile(keyPath, privPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	signer, err := NewDKIMSigner("example.com", "marmot", keyPath)
	if err != nil {
		t.Fatalf("NewDKIMSigner: %v", err)
	}

	// Sign a test message
	msg := "From: user@example.com\r\nTo: recipient@other.com\r\nSubject: File Key Test\r\nDate: Mon, 01 Jan 2024 00:00:00 +0000\r\nMessage-Id: <test2@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nBody"

	signed, err := signer.Sign([]byte(msg))
	if err != nil {
		t.Fatalf("Sign from file key: %v", err)
	}

	if !strings.Contains(string(signed), "DKIM-Signature") {
		t.Error("signed message missing DKIM-Signature")
	}

	_ = key // suppress unused warning
}

func TestNewDKIMSigner_InvalidFile(t *testing.T) {
	_, err := NewDKIMSigner("example.com", "test", "/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestNewDKIMSigner_InvalidPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.key")
	os.WriteFile(path, []byte("not a PEM file"), 0600)

	_, err := NewDKIMSigner("example.com", "test", path)
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestGenerateDKIMKeyPair(t *testing.T) {
	privPEM, dns, err := GenerateDKIMKeyPair()
	if err != nil {
		t.Fatalf("GenerateDKIMKeyPair: %v", err)
	}

	if len(privPEM) == 0 {
		t.Error("private key PEM is empty")
	}
	if !strings.Contains(string(privPEM), "RSA PRIVATE KEY") {
		t.Error("private key PEM missing RSA header")
	}

	if dns == "" {
		t.Error("DNS record is empty")
	}
	if !strings.HasPrefix(dns, "v=DKIM1; k=rsa; p=") {
		t.Errorf("DNS record has wrong format: %s", dns[:50])
	}
}

func TestNewDKIMSigner_UnsupportedPEMType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.key")
	// Write a properly-formed PEM block with an unsupported type
	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: []byte("fake key data"),
	})
	os.WriteFile(path, pemData, 0600)

	_, err := NewDKIMSigner("example.com", "test", path)
	if err == nil {
		t.Error("expected error for unsupported PEM type")
	}
	if !strings.Contains(err.Error(), "unsupported PEM type") {
		t.Errorf("expected unsupported PEM type error, got: %v", err)
	}
}

func TestNewDKIMSignerFromPEM_InvalidPEM(t *testing.T) {
	_, err := NewDKIMSignerFromPEM("example.com", "test", []byte("not a PEM"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestNewDKIMSignerFromPEM_ValidKey(t *testing.T) {
	privPEM, _, err := GenerateDKIMKeyPair()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	signer, err := NewDKIMSignerFromPEM("example.com", "test", privPEM)
	if err != nil {
		t.Fatalf("NewDKIMSignerFromPEM: %v", err)
	}

	msg := "From: user@example.com\r\nTo: bob@other.com\r\nSubject: Test\r\nDate: Mon, 01 Jan 2024 00:00:00 +0000\r\nMessage-Id: <test@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nBody"
	signed, err := signer.Sign([]byte(msg))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !strings.Contains(string(signed), "DKIM-Signature") {
		t.Error("missing DKIM-Signature")
	}
}

func TestNewDKIMSignerFromPEM_PKCS8Key(t *testing.T) {
	// Generate key and marshal as PKCS8 for NewDKIMSignerFromPEM (different code path)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	signer, err := NewDKIMSignerFromPEM("example.com", "test", pemBlock)
	if err != nil {
		t.Fatalf("NewDKIMSignerFromPEM PKCS8: %v", err)
	}

	msg := "From: user@example.com\r\nTo: bob@other.com\r\nSubject: PKCS8 FromPEM\r\nDate: Mon, 01 Jan 2024 00:00:00 +0000\r\nMessage-Id: <test@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nBody"
	signed, err := signer.Sign([]byte(msg))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !strings.Contains(string(signed), "DKIM-Signature") {
		t.Error("missing DKIM-Signature from PKCS8 key via FromPEM")
	}
}

func TestNewDKIMSignerFromPEM_NonRSAPKCS8(t *testing.T) {
	// PKCS8 key that isn't RSA — should fail with "not RSA" error
	// We'll use an EC key marshaled as PKCS8
	ecKey, err := rsa.GenerateKey(rand.Reader, 2048) // We need a real PKCS8 block
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Create a valid PKCS8 PEM but with wrong type label to force the PKCS1 parse to fail
	// then PKCS8 parse to succeed but it IS RSA, so we need a different approach.
	// Instead, create a PEM that fails PKCS1 but also fails PKCS8
	_ = ecKey

	// Construct a PEM block that decodes but fails both PKCS1 and PKCS8 parsing
	badPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY", // PKCS1 type but garbage bytes
		Bytes: []byte("not a valid key structure at all"),
	})

	_, err = NewDKIMSignerFromPEM("example.com", "test", badPEM)
	if err == nil {
		t.Error("expected error for invalid key bytes")
	}
}

func TestNewDKIMSigner_PKCS8NonRSA(t *testing.T) {
	// Test the "PKCS8 key is not RSA" error path in NewDKIMSigner (file-based)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "non-rsa.key")

	// Create a valid PKCS8 PEM with an Ed25519 key (not RSA)
	_, edKey, err := ed25519Keys()
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(edKey)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})
	os.WriteFile(keyPath, pemBlock, 0600)

	_, err = NewDKIMSigner("example.com", "test", keyPath)
	if err == nil {
		t.Error("expected error for non-RSA PKCS8 key")
	}
	if !strings.Contains(err.Error(), "not RSA") {
		t.Errorf("expected 'not RSA' error, got: %v", err)
	}
}

func ed25519Keys() ([]byte, any, error) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	key := ed25519.NewKeyFromSeed(seed)
	return seed, key, nil
}

func TestNewDKIMSigner_CorruptRSAPKCS1(t *testing.T) {
	// RSA PRIVATE KEY block but corrupt bytes — ParsePKCS1 fails
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "corrupt-rsa.key")
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("corrupt key bytes"),
	})
	os.WriteFile(keyPath, pemBlock, 0600)

	_, err := NewDKIMSigner("example.com", "test", keyPath)
	if err == nil {
		t.Error("expected error for corrupt RSA PKCS1 key")
	}
	if !strings.Contains(err.Error(), "parse RSA key") {
		t.Errorf("expected 'parse RSA key' error, got: %v", err)
	}
}

func TestNewDKIMSigner_CorruptPKCS8(t *testing.T) {
	// PRIVATE KEY block but corrupt bytes — ParsePKCS8 fails
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "corrupt-pkcs8.key")
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("corrupt pkcs8 bytes"),
	})
	os.WriteFile(keyPath, pemBlock, 0600)

	_, err := NewDKIMSigner("example.com", "test", keyPath)
	if err == nil {
		t.Error("expected error for corrupt PKCS8 key")
	}
	if !strings.Contains(err.Error(), "parse PKCS8 key") {
		t.Errorf("expected 'parse PKCS8 key' error, got: %v", err)
	}
}

func TestNewDKIMSignerFromPEM_PKCS8NonRSA(t *testing.T) {
	// Valid PKCS8 Ed25519 key — should fail with "not RSA" in FromPEM path
	_, edKey, err := ed25519Keys()
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(edKey)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	_, err = NewDKIMSignerFromPEM("example.com", "test", pemBlock)
	if err == nil {
		t.Error("expected error for non-RSA PKCS8 key in FromPEM")
	}
	if !strings.Contains(err.Error(), "not RSA") {
		t.Errorf("expected 'not RSA' error, got: %v", err)
	}
}

func TestNewDKIMSigner_PKCS8Key(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "pkcs8.key")

	// Generate key and marshal as PKCS8
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	if err := os.WriteFile(keyPath, pemBlock, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	signer, err := NewDKIMSigner("example.com", "test", keyPath)
	if err != nil {
		t.Fatalf("NewDKIMSigner PKCS8: %v", err)
	}

	msg := "From: user@example.com\r\nTo: bob@other.com\r\nSubject: PKCS8\r\nDate: Mon, 01 Jan 2024 00:00:00 +0000\r\nMessage-Id: <test@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nBody"
	signed, err := signer.Sign([]byte(msg))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !strings.Contains(string(signed), "DKIM-Signature") {
		t.Error("missing DKIM-Signature from PKCS8 key")
	}
}
