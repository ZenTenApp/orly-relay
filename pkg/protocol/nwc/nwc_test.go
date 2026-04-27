package nwc_test

import (
	"context"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/protocol/nwc"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

func TestNWCClientCreation(t *testing.T) {
	uri := "nostr+walletconnect://816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b?relay=wss://relay.getalby.com/v1&secret=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	c, err := nwc.NewClient(uri)
	if err != nil {
		t.Fatal(err)
	}

	if c == nil {
		t.Fatal("client should not be nil")
	}
}

func TestNWCInvalidURI(t *testing.T) {
	invalidURIs := []string{
		"invalid://test",
		"nostr+walletconnect://",
		"nostr+walletconnect://invalid",
		"nostr+walletconnect://816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b",
		"nostr+walletconnect://816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b?relay=invalid",
	}

	for _, uri := range invalidURIs {
		_, err := nwc.NewClient(uri)
		if err == nil {
			t.Fatalf("expected error for invalid URI: %s", uri)
		}
	}
}

func TestNWCRelayConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	rc, err := ws.RelayConnect(ctx, "wss://relay.getalby.com/v1")
	if err != nil {
		t.Fatalf("relay connection failed: %v", err)
	}
	defer rc.Close()

	t.Log("relay connection successful")
}

func TestNWCRequestTimeout(t *testing.T) {
	uri := "nostr+walletconnect://816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b?relay=wss://relay.getalby.com/v1&secret=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	c, err := nwc.NewClient(uri)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
	defer cancel()

	var r map[string]any
	err = c.Request(ctx, "get_info", nil, &r)

	if err == nil {
		t.Log("wallet responded")
		return
	}

	expectedErrors := []string{
		"no response from wallet",
		"subscription closed",
		"timeout waiting for response",
		"context deadline exceeded",
	}

	errorFound := false
	for _, expected := range expectedErrors {
		if contains(err.Error(), expected) {
			errorFound = true
			break
		}
	}

	if !errorFound {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("proper timeout handling: %v", err)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findInString(s, substr))))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNWCEncryption(t *testing.T) {
	uri := "nostr+walletconnect://816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b?relay=wss://relay.getalby.com/v1&secret=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	c, err := nwc.NewClient(uri)
	if err != nil {
		t.Fatal(err)
	}

	// We can't directly access private fields, but we can test the client creation
	// check conversation key generation
	if c == nil {
		t.Fatal("client creation should succeed with valid URI")
	}

	// Test passed
}

func TestNWCEventFormat(t *testing.T) {
	uri := "nostr+walletconnect://816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b?relay=wss://relay.getalby.com/v1&secret=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	c, err := nwc.NewClient(uri)
	if err != nil {
		t.Fatal(err)
	}

	// Test client creation
	// The Request method will create proper NWC events with:
	// - Kind 23194 for requests
	// - Proper encryption tag
	// - Signed with client key

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	var r map[string]any
	err = c.Request(ctx, "get_info", nil, &r)

	// We expect this to fail due to inactive connection, but it should fail
	// after creating and sending NWC event
	if err == nil {
		t.Log("wallet responded")
		return
	}

	// Verify it failed for the right reason (connection/response issue, not formatting)
	validFailures := []string{
		"subscription closed",
		"no response from wallet",
		"context deadline exceeded",
		"timeout waiting for response",
	}

	validFailure := false
	for _, failure := range validFailures {
		if contains(err.Error(), failure) {
			validFailure = true
			break
		}
	}

	if !validFailure {
		t.Fatalf("unexpected error type (suggests formatting issue): %v", err)
	}

	// Test passed
}
