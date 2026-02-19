package smtp

import (
	"crypto/rand"
	"crypto/rsa"
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
