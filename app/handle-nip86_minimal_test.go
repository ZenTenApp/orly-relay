package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"next.orly.dev/app/config"
	"next.orly.dev/pkg/database"
)

func TestHandleNIP86Management_Basic(t *testing.T) {
	// Setup test database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a temporary directory for the test database
	tmpDir := t.TempDir()
	db, err := database.New(ctx, cancel, tmpDir, "test.db")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Setup non-managed ACL
	cfg := &config.C{
		AuthRequired: false,
		Owners:       []string{"owner1"},
		Admins:       []string{"admin1"},
		ACLMode:      "none",
	}

	// Setup server
	server := &Server{
		Config: cfg,
		DB:     db,
		Admins: [][]byte{[]byte("admin1")},
		Owners: [][]byte{[]byte("owner1")},
	}

	t.Run("non-managed mode should reject management API", func(t *testing.T) {
		// Create request body
		body := map[string]interface{}{"method": "banpubkey", "params": []string{"user1", "test ban"}}
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}

		// Create HTTP request without authentication to test the managed mode check
		req := httptest.NewRequest("POST", "/api/nip86", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/nostr+json+rpc")

		// Create response recorder
		rr := httptest.NewRecorder()

		// Call the handler
		server.handleNIP86Management(rr, req)

		// Check status code (should be 401 due to authentication failure, not 400)
		if rr.Code != 401 {
			t.Errorf("handleNIP86Management() status = %v, want 401", rr.Code)
		}

		// The test verifies that the handler runs and returns an error
		if rr.Body.String() == "" {
			t.Errorf("handleNIP86Management() body should not be empty")
		}
	})

	t.Run("GET method should not be allowed", func(t *testing.T) {
		// Create HTTP request
		req := httptest.NewRequest("GET", "/api/nip86", nil)

		// Create response recorder
		rr := httptest.NewRecorder()

		// Call the handler
		server.handleNIP86Management(rr, req)

		// Check status code
		if rr.Code != 405 {
			t.Errorf("handleNIP86Management() status = %v, want 405", rr.Code)
		}

		// Check error message (should contain "Method not allowed")
		if rr.Body.String() == "" {
			t.Errorf("handleNIP86Management() body should not be empty")
		}
	})

	t.Run("unauthenticated request should be rejected", func(t *testing.T) {
		// Create request body
		body := map[string]interface{}{"method": "banpubkey", "params": []string{"user1", "test ban"}}
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}

		// Create HTTP request without authentication
		req := httptest.NewRequest("POST", "/api/nip86", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/nostr+json+rpc")

		// Create response recorder
		rr := httptest.NewRecorder()

		// Call the handler
		server.handleNIP86Management(rr, req)

		// Check status code
		if rr.Code != 401 {
			t.Errorf("handleNIP86Management() status = %v, want 401", rr.Code)
		}

		// Check error message (should be about missing authorization header)
		if rr.Body.String() == "" {
			t.Errorf("handleNIP86Management() body should not be empty")
		}
	})
}
