package blossom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
)

// TestFullServerIntegration tests a complete workflow with a real HTTP server
func TestFullServerIntegration(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	// Start real HTTP server
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	baseURL := httpServer.URL
	client := &http.Client{Timeout: 10 * time.Second}

	// Create test keypair
	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()
	pubkeyHex := hex.Enc(pubkey)

	// Step 1: Upload a blob
	testData := []byte("integration test blob content")
	sha256Hash := CalculateSHA256(testData)
	sha256Hex := hex.Enc(sha256Hash)

	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	uploadReq, err := http.NewRequest("PUT", baseURL+"/upload", bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}
	uploadReq.Header.Set("Authorization", createAuthHeader(authEv))
	uploadReq.Header.Set("Content-Type", "text/plain")

	uploadResp, err := client.Do(uploadReq)
	if err != nil {
		t.Fatalf("Failed to upload: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(uploadResp.Body)
		t.Fatalf("Upload failed: status %d, body: %s", uploadResp.StatusCode, string(body))
	}

	var uploadDesc BlobDescriptor
	if err := json.NewDecoder(uploadResp.Body).Decode(&uploadDesc); err != nil {
		t.Fatalf("Failed to parse upload response: %v", err)
	}

	if uploadDesc.SHA256 != sha256Hex {
		t.Errorf("SHA256 mismatch: expected %s, got %s", sha256Hex, uploadDesc.SHA256)
	}

	// Step 2: Retrieve the blob
	getReq, err := http.NewRequest("GET", baseURL+"/"+sha256Hex, nil)
	if err != nil {
		t.Fatalf("Failed to create GET request: %v", err)
	}

	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("Failed to get blob: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Get failed: status %d", getResp.StatusCode)
	}

	retrievedData, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if !bytes.Equal(retrievedData, testData) {
		t.Error("Retrieved blob data mismatch")
	}

	// Step 3: List blobs
	listAuthEv := createAuthEvent(t, signer, "list", nil, 3600)
	listReq, err := http.NewRequest("GET", baseURL+"/list/"+pubkeyHex, nil)
	if err != nil {
		t.Fatalf("Failed to create list request: %v", err)
	}
	listReq.Header.Set("Authorization", createAuthHeader(listAuthEv))

	listResp, err := client.Do(listReq)
	if err != nil {
		t.Fatalf("Failed to list blobs: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("List failed: status %d", listResp.StatusCode)
	}

	var descriptors []BlobDescriptor
	if err := json.NewDecoder(listResp.Body).Decode(&descriptors); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	if len(descriptors) == 0 {
		t.Error("Expected at least one blob in list")
	}

	// Step 4: Delete the blob
	deleteAuthEv := createAuthEvent(t, signer, "delete", sha256Hash, 3600)
	deleteReq, err := http.NewRequest("DELETE", baseURL+"/"+sha256Hex, nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}
	deleteReq.Header.Set("Authorization", createAuthHeader(deleteAuthEv))

	deleteResp, err := client.Do(deleteReq)
	if err != nil {
		t.Fatalf("Failed to delete blob: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("Delete failed: status %d", deleteResp.StatusCode)
	}

	// Step 5: Verify blob is gone
	getResp2, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("Failed to get blob: %v", err)
	}
	defer getResp2.Body.Close()

	if getResp2.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", getResp2.StatusCode)
	}
}

// TestServerWithMultipleBlobs tests multiple blob operations
func TestServerWithMultipleBlobs(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()
	pubkeyHex := hex.Enc(pubkey)

	// Upload multiple blobs
	const numBlobs = 5
	var hashes []string
	var data []byte

	for i := 0; i < numBlobs; i++ {
		testData := []byte(fmt.Sprintf("blob %d content", i))
		sha256Hash := CalculateSHA256(testData)
		sha256Hex := hex.Enc(sha256Hash)
		hashes = append(hashes, sha256Hex)
		data = append(data, testData...)

		authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

		req, _ := http.NewRequest("PUT", httpServer.URL+"/upload", bytes.NewReader(testData))
		req.Header.Set("Authorization", createAuthHeader(authEv))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to upload blob %d: %v", i, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Upload %d failed: status %d", i, resp.StatusCode)
		}
	}

	// List all blobs
	authEv := createAuthEvent(t, signer, "list", nil, 3600)
	req, _ := http.NewRequest("GET", httpServer.URL+"/list/"+pubkeyHex, nil)
	req.Header.Set("Authorization", createAuthHeader(authEv))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to list blobs: %v", err)
	}
	defer resp.Body.Close()

	var descriptors []BlobDescriptor
	json.NewDecoder(resp.Body).Decode(&descriptors)

	if len(descriptors) != numBlobs {
		t.Errorf("Expected %d blobs, got %d", numBlobs, len(descriptors))
	}
}

// TestServerCORS tests CORS headers on all endpoints
func TestServerCORS(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/test123456789012345678901234567890123456789012345678901234567890"},
		{"HEAD", "/test123456789012345678901234567890123456789012345678901234567890"},
		{"PUT", "/upload"},
		{"HEAD", "/upload"},
		{"GET", "/list/test123456789012345678901234567890123456789012345678901234567890"},
		{"PUT", "/media"},
		{"HEAD", "/media"},
		{"PUT", "/mirror"},
		{"PUT", "/report"},
		{"DELETE", "/test123456789012345678901234567890123456789012345678901234567890"},
		{"OPTIONS", "/"},
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, httpServer.URL+ep.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("Failed to test %s %s: %v", ep.method, ep.path, err)
			continue
		}
		resp.Body.Close()

		corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
		if corsHeader != "*" {
			t.Errorf("Missing CORS header on %s %s", ep.method, ep.path)
		}
	}
}

// TestServerRangeRequests tests range request handling
func TestServerRangeRequests(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	// Upload a blob
	testData := []byte("0123456789abcdefghij")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	sha256Hex := hex.Enc(sha256Hash)

	// Test various range requests
	tests := []struct {
		rangeHeader string
		expected    string
		status      int
	}{
		{"bytes=0-4", "01234", http.StatusPartialContent},
		{"bytes=5-9", "56789", http.StatusPartialContent},
		{"bytes=10-", "abcdefghij", http.StatusPartialContent},
		{"bytes=-5", "fghij", http.StatusPartialContent},
		{"bytes=0-0", "0", http.StatusPartialContent},
		{"bytes=100-200", "", http.StatusRequestedRangeNotSatisfiable},
	}

	for _, tt := range tests {
		req, _ := http.NewRequest("GET", httpServer.URL+"/"+sha256Hex, nil)
		req.Header.Set("Range", tt.rangeHeader)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("Failed to request range %s: %v", tt.rangeHeader, err)
			continue
		}

		if resp.StatusCode != tt.status {
			t.Errorf("Range %s: expected status %d, got %d", tt.rangeHeader, tt.status, resp.StatusCode)
			resp.Body.Close()
			continue
		}

		if tt.status == http.StatusPartialContent {
			body, _ := io.ReadAll(resp.Body)
			if string(body) != tt.expected {
				t.Errorf("Range %s: expected %q, got %q", tt.rangeHeader, tt.expected, string(body))
			}

			if resp.Header.Get("Content-Range") == "" {
				t.Errorf("Range %s: missing Content-Range header", tt.rangeHeader)
			}
		}

		resp.Body.Close()
	}
}

// TestServerAuthorizationFlow tests complete authorization flow
func TestServerAuthorizationFlow(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)

	testData := []byte("authorized blob")
	sha256Hash := CalculateSHA256(testData)

	// Test with valid authorization
	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	req := httptest.NewRequest("PUT", "/upload", bytes.NewReader(testData))
	req.Header.Set("Authorization", createAuthHeader(authEv))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Valid auth failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Test with expired authorization
	expiredAuthEv := createAuthEvent(t, signer, "upload", sha256Hash, -3600)

	req2 := httptest.NewRequest("PUT", "/upload", bytes.NewReader(testData))
	req2.Header.Set("Authorization", createAuthHeader(expiredAuthEv))

	w2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("Expired auth should fail: status %d", w2.Code)
	}

	// Test with wrong verb
	wrongVerbAuthEv := createAuthEvent(t, signer, "delete", sha256Hash, 3600)

	req3 := httptest.NewRequest("PUT", "/upload", bytes.NewReader(testData))
	req3.Header.Set("Authorization", createAuthHeader(wrongVerbAuthEv))

	w3 := httptest.NewRecorder()
	server.Handler().ServeHTTP(w3, req3)

	if w3.Code != http.StatusUnauthorized {
		t.Errorf("Wrong verb auth should fail: status %d", w3.Code)
	}
}

// TestServerUploadRequirementsFlow tests upload requirements check flow
func TestServerUploadRequirementsFlow(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	testData := []byte("test")
	sha256Hash := CalculateSHA256(testData)

	// Test HEAD /upload with valid requirements
	req := httptest.NewRequest("HEAD", "/upload", nil)
	req.Header.Set("X-SHA-256", hex.Enc(sha256Hash))
	req.Header.Set("X-Content-Length", "4")
	req.Header.Set("X-Content-Type", "text/plain")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Upload requirements check failed: status %d", w.Code)
	}

	// Test HEAD /upload with missing header
	req2 := httptest.NewRequest("HEAD", "/upload", nil)
	w2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("Expected BadRequest for missing header, got %d", w2.Code)
	}

	// Test HEAD /upload with invalid hash
	req3 := httptest.NewRequest("HEAD", "/upload", nil)
	req3.Header.Set("X-SHA-256", "invalid")
	req3.Header.Set("X-Content-Length", "4")

	w3 := httptest.NewRecorder()
	server.Handler().ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Errorf("Expected BadRequest for invalid hash, got %d", w3.Code)
	}
}

// TestServerMirrorFlow tests mirror endpoint flow
func TestServerMirrorFlow(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)

	// Create mock remote server
	remoteData := []byte("remote blob data")
	sha256Hash := CalculateSHA256(remoteData)
	sha256Hex := hex.Enc(sha256Hash)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(remoteData)))
		w.Write(remoteData)
	}))
	defer mockServer.Close()

	// Mirror the blob
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
		t.Errorf("Mirror failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify blob was stored
	exists, err := server.storage.HasBlob(sha256Hash)
	if err != nil {
		t.Fatalf("Failed to check blob: %v", err)
	}
	if !exists {
		t.Error("Blob should exist after mirror")
	}
}

// TestServerReportFlow tests report endpoint flow
func TestServerReportFlow(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()

	// Upload a blob first
	testData := []byte("reportable blob")
	sha256Hash := CalculateSHA256(testData)

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	// Create report event
	reportEv := &event.E{
		CreatedAt: timestamp.Now().V,
		Kind:      1984,
		Tags:      tag.NewS(tag.NewFromAny("x", hex.Enc(sha256Hash))),
		Content:   []byte("This blob should be reported"),
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
		t.Errorf("Report failed: status %d, body: %s", w.Code, w.Body.String())
	}
}

// TestServerErrorHandling tests various error scenarios
func TestServerErrorHandling(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	tests := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		body       []byte
		statusCode int
	}{
		{
			name:       "Invalid path",
			method:     "GET",
			path:       "/invalid",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "Non-existent blob",
			method:     "GET",
			path:       "/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "Missing auth header",
			method:     "PUT",
			path:       "/upload",
			body:       []byte("test"),
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "Invalid JSON in mirror",
			method:     "PUT",
			path:       "/mirror",
			body:       []byte("invalid json"),
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid JSON in report",
			method:     "PUT",
			path:       "/report",
			body:       []byte("invalid json"),
			statusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != nil {
				body = bytes.NewReader(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d: %s", tt.statusCode, w.Code, w.Body.String())
			}
		})
	}
}

// TestServerMediaOptimization tests media optimization endpoint
func TestServerMediaOptimization(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)

	testData := []byte("test media for optimization")
	sha256Hash := CalculateSHA256(testData)

	authEv := createAuthEvent(t, signer, "media", sha256Hash, 3600)

	req := httptest.NewRequest("PUT", "/media", bytes.NewReader(testData))
	req.Header.Set("Authorization", createAuthHeader(authEv))
	req.Header.Set("Content-Type", "image/png")

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Media upload failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var desc BlobDescriptor
	if err := json.Unmarshal(w.Body.Bytes(), &desc); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if desc.SHA256 == "" {
		t.Error("Expected SHA256 in response")
	}

	// Test HEAD /media
	req2 := httptest.NewRequest("HEAD", "/media", nil)
	w2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("HEAD /media failed: status %d", w2.Code)
	}
}

// TestServerListWithQueryParams tests list endpoint with query parameters
func TestServerListWithQueryParams(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()
	pubkeyHex := hex.Enc(pubkey)

	// Upload blobs at different times
	now := time.Now().Unix()
	blobs := []struct {
		data      []byte
		timestamp int64
	}{
		{[]byte("blob 1"), now - 1000},
		{[]byte("blob 2"), now - 500},
		{[]byte("blob 3"), now},
	}

	for _, b := range blobs {
		sha256Hash := CalculateSHA256(b.data)
		// Manually set uploaded timestamp
		err := server.storage.SaveBlob(sha256Hash, b.data, pubkey, "text/plain", "")
		if err != nil {
			t.Fatalf("Failed to save blob: %v", err)
		}
	}

	// List with since parameter (future timestamp - should return no blobs)
	authEv := createAuthEvent(t, signer, "list", nil, 3600)
	futureTime := time.Now().Unix() + 3600 // 1 hour in the future
	req := httptest.NewRequest("GET", "/list/"+pubkeyHex+"?since="+fmt.Sprintf("%d", futureTime), nil)
	req.Header.Set("Authorization", createAuthHeader(authEv))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("List failed: status %d", w.Code)
	}

	var descriptors []BlobDescriptor
	if err := json.NewDecoder(w.Body).Decode(&descriptors); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should get no blobs since they were all uploaded before the future timestamp
	if len(descriptors) != 0 {
		t.Errorf("Expected 0 blobs, got %d", len(descriptors))
	}

	// Test without since parameter - should get all blobs
	req2 := httptest.NewRequest("GET", "/list/"+pubkeyHex, nil)
	req2.Header.Set("Authorization", createAuthHeader(authEv))

	w2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("List failed: status %d", w2.Code)
	}

	var allDescriptors []BlobDescriptor
	if err := json.NewDecoder(w2.Body).Decode(&allDescriptors); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should get all 3 blobs when no filter is applied
	if len(allDescriptors) != 3 {
		t.Errorf("Expected 3 blobs, got %d", len(allDescriptors))
	}
}

// TestServerConcurrentOperations tests concurrent operations on server
func TestServerConcurrentOperations(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	_, signer := createTestKeypair(t)

	const numOps = 20
	done := make(chan error, numOps)

	for i := 0; i < numOps; i++ {
		go func(id int) {
			testData := []byte(fmt.Sprintf("concurrent op %d", id))
			sha256Hash := CalculateSHA256(testData)
			sha256Hex := hex.Enc(sha256Hash)

			// Upload
			authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)
			req, _ := http.NewRequest("PUT", httpServer.URL+"/upload", bytes.NewReader(testData))
			req.Header.Set("Authorization", createAuthHeader(authEv))

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				done <- err
				return
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				done <- fmt.Errorf("upload failed: %d", resp.StatusCode)
				return
			}

			// Get
			req2, _ := http.NewRequest("GET", httpServer.URL+"/"+sha256Hex, nil)
			resp2, err := http.DefaultClient.Do(req2)
			if err != nil {
				done <- err
				return
			}
			resp2.Body.Close()

			if resp2.StatusCode != http.StatusOK {
				done <- fmt.Errorf("get failed: %d", resp2.StatusCode)
				return
			}

			done <- nil
		}(i)
	}

	for i := 0; i < numOps; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}

// TestServerBlobExtensionHandling tests blob retrieval with file extensions
func TestServerBlobExtensionHandling(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	testData := []byte("test PDF content")
	sha256Hash := CalculateSHA256(testData)
	pubkey := []byte("testpubkey123456789012345678901234")

	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "application/pdf", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	sha256Hex := hex.Enc(sha256Hash)

	// Test GET with extension
	req := httptest.NewRequest("GET", "/"+sha256Hex+".pdf", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET with extension failed: status %d", w.Code)
	}

	// Should still return correct MIME type
	if w.Header().Get("Content-Type") != "application/pdf" {
		t.Errorf("Expected application/pdf, got %s", w.Header().Get("Content-Type"))
	}
}

// TestServerBlobAlreadyExists tests uploading existing blob
func TestServerBlobAlreadyExists(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)
	pubkey := signer.Pub()

	testData := []byte("existing blob")
	sha256Hash := CalculateSHA256(testData)

	// Upload blob first time
	err := server.storage.SaveBlob(sha256Hash, testData, pubkey, "text/plain", "")
	if err != nil {
		t.Fatalf("Failed to save blob: %v", err)
	}

	// Try to upload same blob again
	authEv := createAuthEvent(t, signer, "upload", sha256Hash, 3600)

	req := httptest.NewRequest("PUT", "/upload", bytes.NewReader(testData))
	req.Header.Set("Authorization", createAuthHeader(authEv))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	// Should succeed and return existing blob descriptor
	if w.Code != http.StatusOK {
		t.Errorf("Re-upload should succeed: status %d", w.Code)
	}
}

// TestServerInvalidAuthorization tests various invalid authorization scenarios
func TestServerInvalidAuthorization(t *testing.T) {
	server, cleanup := testSetup(t)
	defer cleanup()

	_, signer := createTestKeypair(t)

	testData := []byte("test")
	sha256Hash := CalculateSHA256(testData)

	tests := []struct {
		name      string
		modifyEv  func(*event.E)
		expectErr bool
	}{
		{
			name: "Missing expiration",
			modifyEv: func(ev *event.E) {
				ev.Tags = tag.NewS(tag.NewFromAny("t", "upload"))
			},
			expectErr: true,
		},
		{
			name: "Wrong kind",
			modifyEv: func(ev *event.E) {
				ev.Kind = 1
			},
			expectErr: true,
		},
		{
			name: "Wrong verb",
			modifyEv: func(ev *event.E) {
				ev.Tags = tag.NewS(
					tag.NewFromAny("t", "delete"),
					tag.NewFromAny("expiration", timestamp.FromUnix(time.Now().Unix()+3600).String()),
				)
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createAuthEvent(t, signer, "upload", sha256Hash, 3600)
			tt.modifyEv(ev)

			req := httptest.NewRequest("PUT", "/upload", bytes.NewReader(testData))
			req.Header.Set("Authorization", createAuthHeader(ev))

			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, req)

			if tt.expectErr {
				if w.Code == http.StatusOK {
					t.Error("Expected error but got success")
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("Expected success but got error: status %d", w.Code)
				}
			}
		})
	}
}

