package blossom

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
)

// TestHTTPGetBlob tests GET /<sha256> endpoint
func TestHTTPGetBlob(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	// Upload a blob first
	testData := []byte("test blob content")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	sha256Hex := hex.Enc(sha256Hash)

	// Test GET request
	req := httptest.NewRequest("GET", "/"+sha256Hex, nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.Bytes()
	if !bytes.Equal(body, testData) {
		t.Error("Response body mismatch")
	}

	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", w.Header().Get("Content-Type"))
	}
}

// TestHTTPHeadBlob tests HEAD /<sha256> endpoint
func TestHTTPHeadBlob(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	testData := []byte("test blob content")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	sha256Hex := hex.Enc(sha256Hash)

	req := httptest.NewRequest("HEAD", "/"+sha256Hex, nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.Len() != 0 {
		t.Error("HEAD request should not return body")
	}

	if w.Header().Get("Content-Length") != "17" {
		t.Errorf("Expected Content-Length 17, got %s", w.Header().Get("Content-Length"))
	}
}

// TestHTTPUpload tests PUT /upload endpoint
func TestHTTPUpload(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)

	testData := []byte("test upload data")
	sha256Hash := CalculateSHA256(testData)

	// Create auth event
	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	// Create request
	req := httptest.NewRequest("PUT", "/upload", bytes.NewReader(testData))
	req.Header.Set("Authorization", createAuthHeader(authEv))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var desc BlobDescriptor
	if err := json.Unmarshal(w.Body.Bytes(), &desc); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if desc.SHA256 != hex.Enc(sha256Hash) {
		t.Errorf("SHA256 mismatch: expected %s, got %s", hex.Enc(sha256Hash), desc.SHA256)
	}

	if desc.Size != int64(len(testData)) {
		t.Errorf("Size mismatch: expected %d, got %d", len(testData), desc.Size)
	}

	// Verify blob was saved
	exists, err := server.storage.HasBlob(sha256Hash)
	if err != nil {
		t.Fatalf("Failed to check blob: %v", err)
	}
	if !exists {
		t.Error("Blob should exist after upload")
	}
}

// TestHTTPUploadRequirements tests HEAD /upload endpoint
func TestHTTPUploadRequirements(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	testData := []byte("test data")
	sha256Hash := CalculateSHA256(testData)

	req := httptest.NewRequest("HEAD", "/upload", nil)
	req.Header.Set("X-SHA-256", hex.Enc(sha256Hash))
	req.Header.Set("X-Content-Length", "9")
	req.Header.Set("X-Content-Type", "text/plain")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Header().Get("X-Reason"))
	}
}

// TestHTTPUploadTooLarge tests upload size limit
func TestHTTPUploadTooLarge(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	// Create request with size exceeding limit
	req := httptest.NewRequest("HEAD", "/upload", nil)
	req.Header.Set("X-SHA-256", hex.Enc(CalculateSHA256([]byte("test"))))
	req.Header.Set("X-Content-Length", "200000000") // 200MB
	req.Header.Set("X-Content-Type", "application/octet-stream")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", w.Code)
	}
}

// TestHTTPListBlobs tests GET /list/<pubkey> endpoint
func TestHTTPListBlobs(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()
	pubkeyHex := hex.Enc(pubkey)

	// Upload multiple blobs
	for i := 0; i < 3; i++ {
		testData := []byte("test data " + string(rune('A'+i)))
		sha256Hash := CalculateSHA256(testData)
		err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
		if err != nil {
			t.Fatalf("Failed to save blob: %v", err)
		}
	}

	// Create auth event
	authEv := createAuthEvent(t, signer, "list", nil, 3600)

	req := httptest.NewRequest("GET", "/list/"+pubkeyHex, nil)
	req.Header.Set("Authorization", createAuthHeader(authEv))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var descriptors []BlobDescriptor
	if err := json.Unmarshal(w.Body.Bytes(), &descriptors); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(descriptors) != 3 {
		t.Errorf("Expected 3 blobs, got %d", len(descriptors))
	}
}

// TestHTTPDeleteBlob tests DELETE /<sha256> endpoint
func TestHTTPDeleteBlob(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()

	testData := []byte("test delete data")
	sha256Hash := CalculateSHA256(testData)

	// Upload blob first
	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	// Create auth event
	authEv := createAuthEvent(t, signer, "delete", sha256Hash, 3600)

	sha256Hex := hex.Enc(sha256Hash)
	req := httptest.NewRequest("DELETE", "/"+sha256Hex, nil)
	req.Header.Set("Authorization", createAuthHeader(authEv))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify blob was deleted
	exists, err := server.storage.HasBlob(sha256Hash)
	if err != nil {
		t.Fatalf("Failed to check blob: %v", err)
	}
	if exists {
		t.Error("Blob should not exist after delete")
	}
}

// TestHTTPMirror tests PUT /mirror endpoint
func TestHTTPMirror(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)

	// Create a mock remote server
	testData := []byte("mirrored blob data")
	sha256Hash := CalculateSHA256(testData)
	sha256Hex := hex.Enc(sha256Hash)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(testData)
	}))
	defer mockServer.Close()

	// Create mirror request
	mirrorReq := map[string]string{
		"url": mockServer.URL + "/" + sha256Hex,
	}
	reqBody, _ := json.Marshal(mirrorReq)

	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	req := httptest.NewRequest("PUT", "/mirror", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", createAuthHeader(authEv))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify blob was saved
	exists, err := server.storage.HasBlob(sha256Hash)
	if err != nil {
		t.Fatalf("Failed to check blob: %v", err)
	}
	if !exists {
		t.Error("Blob should exist after mirror")
	}
}

// TestHTTPMediaUpload tests PUT /media endpoint
func TestHTTPMediaUpload(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)

	testData := []byte("test media data")
	sha256Hash := CalculateSHA256(testData)

	authEv := createAuthEvent(t, signer, "media", sha256Hash, 3600)

	req := httptest.NewRequest("PUT", "/media", bytes.NewReader(testData))
	req.Header.Set("Authorization", createAuthHeader(authEv))
	req.Header.Set("Content-Type", "image/png")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var desc BlobDescriptor
	if err := json.Unmarshal(w.Body.Bytes(), &desc); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if desc.SHA256 == "" {
		t.Error("Expected SHA256 in response")
	}
}

// TestHTTPReport tests PUT /report endpoint
func TestHTTPReport(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()

	// Upload a blob first
	testData := []byte("test blob")
	sha256Hash := CalculateSHA256(testData)

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	// Create report event (kind 1984)
	reportEv := &event.E{
		CreatedAt: timestamp.Now().V,
		Kind:      1984,
		Tags:      tag.NewS(tag.NewFromAny("x", hex.Enc(sha256Hash))),
		Content:   []byte("This blob violates policy"),
		Pubkey:    pubkey,
	}

	if err := reportEv.Sign(signer); err != nil {
		t.Fatalf("Failed to sign report: %v", err)
	}

	reqBody := reportEv.Serialize()
	req := httptest.NewRequest("PUT", "/report", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHTTPRangeRequest tests range request support
func TestHTTPRangeRequest(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	testData := []byte("0123456789abcdef")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	sha256Hex := hex.Enc(sha256Hash)

	// Test range request
	req := httptest.NewRequest("GET", "/"+sha256Hex, nil)
	req.Header.Set("Range", "bytes=4-9")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206, got %d", w.Code)
	}

	body := w.Body.Bytes()
	expected := testData[4:10]
	if !bytes.Equal(body, expected) {
		t.Errorf("Range response mismatch: expected %s, got %s", string(expected), string(body))
	}

	if w.Header().Get("Content-Range") == "" {
		t.Error("Missing Content-Range header")
	}
}

// TestHTTPNotFound tests 404 handling
func TestHTTPNotFound(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestHTTPServerIntegration tests full server integration
func TestHTTPServerIntegration(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	// Start HTTP server
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	_, signer := createTestKeypair(t)

	// Upload blob via HTTP
	testData := []byte("integration test data")
	sha256Hash := CalculateSHA256(testData)
	sha256Hex := hex.Enc(sha256Hash)

	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	uploadReq, _ := http.NewRequest("PUT", httpServer.URL+"/upload", bytes.NewReader(testData))
	uploadReq.Header.Set("Authorization", createAuthHeader(authEv))
	uploadReq.Header.Set("Content-Type", "text/plain")

	client := &http.Client{}
	resp, err := client.Do(uploadReq)
	if err != nil {
		t.Fatalf("Failed to upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Upload failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Retrieve blob via HTTP
	getReq, _ := http.NewRequest("GET", httpServer.URL+"/"+sha256Hex, nil)
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("Failed to get blob: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Get failed: status %d", getResp.StatusCode)
	}

	body, _ := io.ReadAll(getResp.Body)
	if !bytes.Equal(body, testData) {
		t.Error("Retrieved blob data mismatch")
	}
}

// TestCORSHeaders tests CORS header handling
func TestCORSHeaders(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Missing CORS header")
	}
}

// TestAuthorizationRequired tests authorization requirement
func TestAuthorizationRequired(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	// Configure server to require auth
	server.requireAuth = true

	testData := []byte("test")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	sha256Hex := hex.Enc(sha256Hash)

	// Request without auth should fail
	req := httptest.NewRequest("GET", "/"+sha256Hex, nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

// TestACLIntegration tests ACL integration
func TestACLIntegration(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	// Note: This test assumes ACL is configured
	// In a real scenario, you'd set up a proper ACL instance

	_, signer := createTestKeypair(t)
	testData := []byte("test")
	sha256Hash := CalculateSHA256(testData)

	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	req := httptest.NewRequest("PUT", "/upload", bytes.NewReader(testData))
	req.Header.Set("Authorization", createAuthHeader(authEv))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	// Should succeed if ACL allows, or fail if not
	// The exact behavior depends on ACL configuration
	if w.Code != http.StatusOK && w.Code != http.StatusForbidden {
		t.Errorf("Unexpected status: %d", w.Code)
	}
}

// TestMimeTypeDetection tests MIME type detection from various sources
func TestMimeTypeDetection(t *testing.T) {
	tests := []struct {
		contentType string
		ext         string
		expected    string
	}{
		{"image/png", "", "image/png"},
		{"", ".png", "image/png"},
		{"", ".pdf", "application/pdf"},
		{"application/pdf", ".txt", "application/pdf"},
		{"", ".unknown", "application/octet-stream"},
		{"", "", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := DetectMimeType(tt.contentType, tt.ext)
		if result != tt.expected {
			t.Errorf("DetectMimeType(%q, %q) = %q, want %q",
				tt.contentType, tt.ext, result, tt.expected)
		}
	}
}

// TestSHA256Validation tests SHA256 validation
func TestSHA256Validation(t *testing.T) {
	validHashes := []string{
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"abc123def456789012345678901234567890123456789012345678901234abcd",
	}

	invalidHashes := []string{
		"",
		"abc",
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855x",
		"12345",
	}

	for _, hash := range validHashes {
		if !ValidateSHA256Hex(hash) {
			t.Errorf("Hash %s should be valid", hash)
		}
	}

	for _, hash := range invalidHashes {
		if ValidateSHA256Hex(hash) {
			t.Errorf("Hash %s should be invalid", hash)
		}
	}
}

// TestBlobURLBuilding tests URL building
func TestBlobURLBuilding(t *testing.T) {
	baseURL := "https://example.com"
	sha256Hex := "abc123def456"
	ext := ".pdf"

	url := BuildBlobURL(baseURL, sha256Hex, ext)
	expected := baseURL + "/" + sha256Hex + ext

	if url != expected {
		t.Errorf("Expected %s, got %s", expected, url)
	}

	// Test without extension
	url2 := BuildBlobURL(baseURL, sha256Hex, "")
	expected2 := baseURL + "/" + sha256Hex

	if url2 != expected2 {
		t.Errorf("Expected %s, got %s", expected2, url2)
	}
}

// TestErrorResponses tests error response formatting
func TestErrorResponses(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	w := httptest.NewRecorder()

	server.setErrorResponse(w, http.StatusBadRequest, "Invalid request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if w.Header().Get("X-Reason") == "" {
		t.Error("Missing X-Reason header")
	}
}

// TestExtractSHA256FromURL tests URL hash extraction
func TestExtractSHA256FromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
		hasError bool
	}{
		{"https://example.com/abc123def456789012345678901234567890123456789012345678901234abcd", "abc123def456789012345678901234567890123456789012345678901234abcd", false},
		{"https://example.com/user/path/abc123def456789012345678901234567890123456789012345678901234abcd.pdf", "abc123def456789012345678901234567890123456789012345678901234abcd", false},
		{"https://example.com/", "", true},
		{"no hash here", "", true},
	}

	for _, tt := range tests {
		hash, err := ExtractSHA256FromURL(tt.url)
		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for URL %s", tt.url)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for URL %s: %v", tt.url, err)
			}
			if hash != tt.expected {
				t.Errorf("Expected %s, got %s for URL %s", tt.expected, hash, tt.url)
			}
		}
	}
}

// TestStorageReport tests report storage
func TestStorageReport(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	sha256Hash := CalculateSHA256([]byte("test"))
	reportData := []byte("report data")

	err := server.storage.SaveReport(sha256Hash, reportData)
	if err != nil {
		t.Fatalf("Failed to save report: %v", err)
	}

	// Reports are stored but not retrieved in current implementation
	// This test verifies the operation doesn't fail
}

// BenchmarkStorageOperations benchmarks storage operations
func BenchmarkStorageOperations(b *testing.B) {
	server, cleanup := testSetup(&testing.T{})
	defer cleanup()

	testData := []byte("benchmark test data")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
		_, _, _ = server.storage.GetBlob(sha256Hash)
		_ = server.storage.DeleteBlob(sha256Hash, pubkey)
	}
}

// TestConcurrentUploads tests concurrent uploads
func TestConcurrentUploads(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	// Prepare all auth events sequentially to avoid race in p256k1.TaggedHash
	// (the package-level SHA256 hasher is not safe for concurrent Sign() calls).
	// The test is about concurrent HTTP uploads, not concurrent signing.
	const numUploads = 10
	type uploadPrep struct {
		data    []byte
		authHdr string
	}
	preps := make([]uploadPrep, numUploads)
	for i := range preps {
		_, signer := createTestKeypair(t)
		data := []byte("concurrent test " + string(rune('A'+i)))
		sha256Hash := CalculateSHA256(data)
		authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)
		preps[i] = uploadPrep{data: data, authHdr: createAuthHeader(authEv)}
	}

	done := make(chan error, numUploads)

	for i := 0; i < numUploads; i++ {
		go func(id int) {
			req := httptest.NewRequest("PUT", "/upload", bytes.NewReader(preps[id].data))
			req.Header.Set("Authorization", preps[id].authHdr)

			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				done <- &testError{code: w.Code, body: w.Body.String()}
				return
			}
			done <- nil
		}(i)
	}

	for i := 0; i < numUploads; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent upload failed: %v", err)
		}
	}
}

type testError struct {
	code int
	body string
}

func (e *testError) Error() string {
	return strings.Join([]string{"HTTP", string(rune(e.code)), e.body}, " ")
}
