package blossom

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// testSetup creates a test database, ACL, and server
func testSetup(t *testing.T) (*Server, func()) {
	// Create temporary directory for database
	tempDir, err := os.MkdirTemp("", "blossom-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create database
	db, err := database.New(ctx, cancel, tempDir, "error")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create ACL registry and set to "none" mode for tests
	aclRegistry := acl.Registry
	aclRegistry.SetMode("none") // Allow all access for tests

	// Create server
	cfg := &Config{
		BaseURL:          "http://localhost:8080",
		MaxBlobSize:      100 * 1024 * 1024, // 100MB
		AllowedMimeTypes: nil,
		RequireAuth:      false,
	}

	server := NewServer(db, aclRegistry, cfg)

	cleanup := func() {
		cancel()
		db.Close()
		os.RemoveAll(tempDir)
	}

	return server, cleanup
}

// createTestKeypair creates a test keypair for signing events
func createTestKeypair(t *testing.T) ([]byte, *p8k.Signer) {
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	pubkey := signer.Pub()
	return pubkey, signer
}

// createAuthEvent creates a valid kind 24242 authorization event
func createAuthEvent(
	t *testing.T, signer *p8k.Signer, verb string,
	sha256Hash []byte, expiresIn int64,
) *event.E {
	now := time.Now().Unix()
	expires := now + expiresIn

	tags := tag.NewS()
	tags.Append(tag.NewFromAny("t", verb))
	tags.Append(tag.NewFromAny("expiration", timestamp.FromUnix(expires).String()))

	if sha256Hash != nil {
		tags.Append(tag.NewFromAny("x", hex.Enc(sha256Hash)))
	}

	ev := &event.E{
		CreatedAt: now,
		Kind:      BlossomAuthKind,
		Tags:      tags,
		Content:   []byte("Test authorization"),
		Pubkey:    signer.Pub(),
	}

	// Sign event
	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	return ev
}

// createAuthHeader creates an Authorization header from an event
func createAuthHeader(ev *event.E) string {
	eventJSON := ev.Serialize()
	b64 := base64.StdEncoding.EncodeToString(eventJSON)
	return "Nostr " + b64
}

// makeRequest creates an HTTP request with optional authorization
func makeRequest(
	t *testing.T, method, path string, body []byte, authEv *event.E,
) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if body != nil {
		req.Body = httptest.NewRequest(method, path, nil).Body
		req.ContentLength = int64(len(body))
	}

	if authEv != nil {
		req.Header.Set("Authorization", createAuthHeader(authEv))
	}

	return req
}

// TestBlobDescriptor tests BlobDescriptor creation and serialization
func TestBlobDescriptor(t *testing.T) {
	desc := NewBlobDescriptor(
		"https://example.com/blob.pdf",
		"abc123",
		1024,
		"application/pdf",
		1234567890,
	)

	if desc.URL != "https://example.com/blob.pdf" {
		t.Errorf("Expected URL %s, got %s", "https://example.com/blob.pdf", desc.URL)
	}
	if desc.SHA256 != "abc123" {
		t.Errorf("Expected SHA256 %s, got %s", "abc123", desc.SHA256)
	}
	if desc.Size != 1024 {
		t.Errorf("Expected Size %d, got %d", 1024, desc.Size)
	}
	if desc.Type != "application/pdf" {
		t.Errorf("Expected Type %s, got %s", "application/pdf", desc.Type)
	}

	// Test default MIME type
	desc2 := NewBlobDescriptor("url", "hash", 0, "", 0)
	if desc2.Type != "application/octet-stream" {
		t.Errorf("Expected default MIME type, got %s", desc2.Type)
	}
}

// TestBlobMetadata tests BlobMetadata serialization
func TestBlobMetadata(t *testing.T) {
	pubkey := []byte("testpubkey123456789012345678901234")
	meta := NewBlobMetadata(pubkey, "image/png", 2048)

	if meta.Size != 2048 {
		t.Errorf("Expected Size %d, got %d", 2048, meta.Size)
	}
	if meta.MimeType != "image/png" {
		t.Errorf("Expected MIME type %s, got %s", "image/png", meta.MimeType)
	}

	// Test serialization
	data, err := meta.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize metadata: %v", err)
	}

	// Test deserialization
	meta2, err := DeserializeBlobMetadata(data)
	if err != nil {
		t.Fatalf("Failed to deserialize metadata: %v", err)
	}

	if meta2.Size != meta.Size {
		t.Errorf("Size mismatch after deserialize")
	}
	if meta2.MimeType != meta.MimeType {
		t.Errorf("MIME type mismatch after deserialize")
	}
}

// TestUtils tests utility functions
func TestUtils(t *testing.T) {
	data := []byte("test data")
	hash := CalculateSHA256(data)
	if len(hash) != 32 {
		t.Errorf("Expected hash length 32, got %d", len(hash))
	}

	hashHex := CalculateSHA256Hex(data)
	if len(hashHex) != 64 {
		t.Errorf("Expected hex hash length 64, got %d", len(hashHex))
	}

	// Test ExtractSHA256FromPath
	testHash := "abc123def456789012345678901234567890123456789012345678901234abcd"
	sha256Hex, ext, err := ExtractSHA256FromPath(testHash)
	if err != nil {
		t.Fatalf("Failed to extract SHA256: %v", err)
	}
	if sha256Hex != testHash {
		t.Errorf("Expected %s, got %s", testHash, sha256Hex)
	}
	if ext != "" {
		t.Errorf("Expected empty ext, got %s", ext)
	}

	sha256Hex, ext, err = ExtractSHA256FromPath(testHash + ".pdf")
	if err != nil {
		t.Fatalf("Failed to extract SHA256: %v", err)
	}
	if sha256Hex != testHash {
		t.Errorf("Expected %s, got %s", testHash, sha256Hex)
	}
	if ext != ".pdf" {
		t.Errorf("Expected .pdf, got %s", ext)
	}

	// Test MIME type detection
	mime := GetMimeTypeFromExtension(".pdf")
	if mime != "application/pdf" {
		t.Errorf("Expected application/pdf, got %s", mime)
	}

	mime = DetectMimeType("image/png", ".png")
	if mime != "image/png" {
		t.Errorf("Expected image/png, got %s", mime)
	}

	mime = DetectMimeType("", ".jpg")
	if mime != "image/jpeg" {
		t.Errorf("Expected image/jpeg, got %s", mime)
	}
}

// TestStorage tests storage operations
func TestStorage(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	storage := server.storage

	// Create test data
	testData := []byte("test blob data")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	// Test SaveBlob
	err := storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	// Test HasBlob
	exists, err := storage.HasBlob(sha256Hash)
	if err != nil {
		t.Fatalf("Failed to check blob existence: %v", err)
	}
	if !exists {
		t.Error("Blob should exist after save")
	}

	// Test GetBlob
	blobData, metadata, err := storage.GetBlob(sha256Hash)
	if err != nil {
		t.Fatalf("Failed to get blob: %v", err)
	}
	if string(blobData) != string(testData) {
		t.Error("Blob data mismatch")
	}
	if metadata.Size != int64(len(testData)) {
		t.Errorf("Size mismatch: expected %d, got %d", len(testData), metadata.Size)
	}

	// Test ListBlobs
	descriptors, err := storage.ListBlobs(pubkey, 0, 0)
	if err != nil {
		t.Fatalf("Failed to list blobs: %v", err)
	}
	if len(descriptors) != 1 {
		t.Errorf("Expected 1 blob, got %d", len(descriptors))
	}

	// Test DeleteBlob
	err = storage.DeleteBlob(sha256Hash, pubkey)
	if err != nil {
		t.Fatalf("Failed to delete blob: %v", err)
	}

	exists, err = storage.HasBlob(sha256Hash)
	if err != nil {
		t.Fatalf("Failed to check blob existence: %v", err)
	}
	if exists {
		t.Error("Blob should not exist after delete")
	}
}

// TestAuthEvent tests authorization event validation
func TestAuthEvent(t *testing.T) {
	pubkey, signer := createTestKeypair(t)
	sha256Hash := CalculateSHA256([]byte("test"))

	// Create valid auth event
	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	// Create HTTP request
	req := httptest.NewRequest("PUT", "/upload", nil)
	req.Header.Set("Authorization", createAuthHeader(authEv))

	// Extract and validate
	ev, err := ExtractAuthEvent(req)
	if err != nil {
		t.Fatalf("Failed to extract auth event: %v", err)
	}

	if ev.Kind != BlossomAuthKind {
		t.Errorf("Expected kind %d, got %d", BlossomAuthKind, ev.Kind)
	}

	// Validate auth event
	authEv2, err := ValidateAuthEvent(req, "upload", sha256Hash)
	if err != nil {
		t.Fatalf("Failed to validate auth event: %v", err)
	}

	if authEv2.Verb != "upload" {
		t.Errorf("Expected verb 'upload', got '%s'", authEv2.Verb)
	}

	// Verify pubkey matches
	if !bytes.Equal(authEv2.Pubkey, pubkey) {
		t.Error("Pubkey mismatch")
	}
}

// TestAuthEventExpired tests expired authorization events
func TestAuthEventExpired(t *testing.T) {
	_, signer := createTestKeypair(t)
	sha256Hash := CalculateSHA256([]byte("test"))

	// Create expired auth event
	authEv := createAuthEvent(t, signer, "upload", sha256Hash, -3600)

	req := httptest.NewRequest("PUT", "/upload", nil)
	req.Header.Set("Authorization", createAuthHeader(authEv))

	_, err := ValidateAuthEvent(req, "upload", sha256Hash)
	if err == nil {
		t.Error("Expected error for expired auth event")
	}
}

// TestServerHandler tests the server handler routing
func TestServerHandler(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	handler := server.Handler()

	// Test OPTIONS request (CORS preflight)
	req := httptest.NewRequest("OPTIONS", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Missing CORS header")
	}
}
