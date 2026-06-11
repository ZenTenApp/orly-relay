package nrc

import (
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// Test keys - generated from known secrets for reproducibility
var (
	// From secret: 0000000000000000000000000000000000000000000000000000000000000001
	testRelaySecret = "0000000000000000000000000000000000000000000000000000000000000001"
	// From secret: 0000000000000000000000000000000000000000000000000000000000000002
	testClientSecret = "0000000000000000000000000000000000000000000000000000000000000002"
)

// getTestRelayPubkey returns the pubkey derived from testRelaySecret
func getTestRelayPubkey(t *testing.T) []byte {
	secretBytes, err := hex.Dec(testRelaySecret)
	if err != nil {
		t.Fatalf("failed to decode test secret: %v", err)
	}
	pubkey, err := keys.SecretBytesToPubKeyBytes(secretBytes)
	if err != nil {
		t.Fatalf("failed to derive pubkey: %v", err)
	}
	return pubkey
}

// getTestRelayPubkeyHex returns the hex-encoded pubkey
func getTestRelayPubkeyHex(t *testing.T) string {
	return string(hex.Enc(getTestRelayPubkey(t)))
}

func TestParseConnectionURI(t *testing.T) {
	// Get valid pubkey for tests
	relayPubkeyHex := getTestRelayPubkeyHex(t)

	tests := []struct {
		name    string
		uri     string
		wantErr bool
		check   func(*testing.T, *ConnectionURI)
	}{
		{
			name:    "invalid scheme",
			uri:     "nostr+wallet://abc123",
			wantErr: true,
		},
		{
			name:    "missing relay parameter",
			uri:     "nostr+relayconnect://" + relayPubkeyHex,
			wantErr: true,
		},
		{
			name:    "missing secret parameter",
			uri:     "nostr+relayconnect://" + relayPubkeyHex + "?relay=wss://relay.example.com",
			wantErr: true,
		},
		{
			name:    "invalid relay pubkey",
			uri:     "nostr+relayconnect://invalid?relay=wss://relay.example.com&secret=" + testClientSecret,
			wantErr: true,
		},
		{
			name: "valid secret-based URI",
			uri:  "nostr+relayconnect://" + relayPubkeyHex + "?relay=wss://relay.example.com&secret=" + testClientSecret,
			check: func(t *testing.T, conn *ConnectionURI) {
				if conn.AuthMode != AuthModeSecret {
					t.Errorf("expected AuthModeSecret, got %d", conn.AuthMode)
				}
				if conn.RendezvousRelay != "wss://relay.example.com" {
					t.Errorf("expected wss://relay.example.com, got %s", conn.RendezvousRelay)
				}
				if conn.GetClientSigner() == nil {
					t.Error("expected client signer to be set")
				}
				if len(conn.GetConversationKey()) != 32 {
					t.Errorf("expected 32-byte conversation key, got %d", len(conn.GetConversationKey()))
				}
			},
		},
		{
			name: "valid URI with device name",
			uri:  "nostr+relayconnect://" + relayPubkeyHex + "?relay=wss://relay.example.com&secret=" + testClientSecret + "&name=phone",
			check: func(t *testing.T, conn *ConnectionURI) {
				if conn.DeviceName != "phone" {
					t.Errorf("expected device name 'phone', got '%s'", conn.DeviceName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := ParseConnectionURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConnectionURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && err == nil {
				tt.check(t, conn)
			}
		})
	}
}

func TestGenerateConnectionURI(t *testing.T) {
	relayPubkey := getTestRelayPubkey(t)
	rendezvousRelay := "wss://relay.example.com"

	uri, secret, err := GenerateConnectionURI(relayPubkey, rendezvousRelay, "test-device")
	if err != nil {
		t.Fatalf("GenerateConnectionURI() error = %v", err)
	}

	if len(secret) != 32 {
		t.Errorf("expected 32-byte secret, got %d", len(secret))
	}

	// Parse the generated URI to verify it's valid
	conn, err := ParseConnectionURI(uri)
	if err != nil {
		t.Fatalf("failed to parse generated URI: %v", err)
	}

	if conn.DeviceName != "test-device" {
		t.Errorf("expected device name 'test-device', got '%s'", conn.DeviceName)
	}

	if conn.RendezvousRelay != rendezvousRelay {
		t.Errorf("expected rendezvous relay %s, got %s", rendezvousRelay, conn.RendezvousRelay)
	}
}

func TestSession(t *testing.T) {
	clientPubkey := make([]byte, 32)
	conversationKey := make([]byte, 32)

	session := NewSession("test-session", clientPubkey, conversationKey, AuthModeSecret, "test-device")
	if session == nil {
		t.Fatal("NewSession returned nil")
	}

	// Test basic properties
	if session.ID != "test-session" {
		t.Errorf("expected ID 'test-session', got '%s'", session.ID)
	}
	if session.DeviceName != "test-device" {
		t.Errorf("expected device name 'test-device', got '%s'", session.DeviceName)
	}
	if session.AuthMode != AuthModeSecret {
		t.Errorf("expected AuthModeSecret, got %d", session.AuthMode)
	}

	// Test subscription management
	if err := session.AddSubscription("sub1"); err != nil {
		t.Errorf("AddSubscription() error = %v", err)
	}
	if !session.HasSubscription("sub1") {
		t.Error("expected subscription 'sub1' to exist")
	}
	if session.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription, got %d", session.SubscriptionCount())
	}

	session.RemoveSubscription("sub1")
	if session.HasSubscription("sub1") {
		t.Error("expected subscription 'sub1' to be removed")
	}

	// Test expiry
	if session.IsExpired(time.Hour) {
		t.Error("session should not be expired")
	}

	// Test close
	session.Close()
	select {
	case <-session.Context().Done():
		// Expected
	default:
		t.Error("expected session context to be cancelled after Close()")
	}
}

func TestSessionManager(t *testing.T) {
	manager := NewSessionManager(time.Minute)
	if manager == nil {
		t.Fatal("NewSessionManager returned nil")
	}

	clientPubkey := make([]byte, 32)
	conversationKey := make([]byte, 32)

	// Test GetOrCreate
	session := manager.GetOrCreate("session1", clientPubkey, conversationKey, AuthModeSecret, "device1")
	if session == nil {
		t.Fatal("GetOrCreate returned nil")
	}

	// Get same session again
	session2 := manager.GetOrCreate("session1", clientPubkey, conversationKey, AuthModeSecret, "device1")
	if session2 != session {
		t.Error("expected GetOrCreate to return same session")
	}

	// Test Get
	retrieved := manager.Get("session1")
	if retrieved != session {
		t.Error("expected Get to return the session")
	}

	// Test Count
	if manager.Count() != 1 {
		t.Errorf("expected count 1, got %d", manager.Count())
	}

	// Test Remove
	manager.Remove("session1")
	if manager.Get("session1") != nil {
		t.Error("expected session to be removed")
	}
	if manager.Count() != 0 {
		t.Errorf("expected count 0 after removal, got %d", manager.Count())
	}

	// Test Close - verifies it doesn't panic or hang
	manager.GetOrCreate("session2", clientPubkey, conversationKey, AuthModeSecret, "device2")
	manager.Close()
}

func TestParseRequestContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		check   func(*testing.T, *RequestMessage)
	}{
		{
			name:    "empty content",
			content: "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			content: "not json",
			wantErr: true,
		},
		{
			name:    "missing type",
			content: `{"payload": []}`,
			wantErr: true,
		},
		{
			name:    "valid EVENT request",
			content: `{"type": "EVENT", "payload": ["EVENT", {}]}`,
			check: func(t *testing.T, msg *RequestMessage) {
				if msg.Type != "EVENT" {
					t.Errorf("expected type EVENT, got %s", msg.Type)
				}
			},
		},
		{
			name:    "valid REQ request",
			content: `{"type": "REQ", "payload": ["REQ", "sub1", {}]}`,
			check: func(t *testing.T, msg *RequestMessage) {
				if msg.Type != "REQ" {
					t.Errorf("expected type REQ, got %s", msg.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseRequestContent([]byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequestContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && err == nil {
				tt.check(t, msg)
			}
		})
	}
}

func TestMarshalResponseContent(t *testing.T) {
	resp := &ResponseMessage{
		Type:    "OK",
		Payload: []any{"OK", "eventid123", true, ""},
	}

	content, err := MarshalResponseContent(resp)
	if err != nil {
		t.Fatalf("MarshalResponseContent() error = %v", err)
	}

	// Verify it's valid JSON that can be parsed back
	parsed, err := ParseRequestContent(content)
	if err != nil {
		t.Fatalf("failed to parse marshaled response: %v", err)
	}

	if parsed.Type != "OK" {
		t.Errorf("expected type OK, got %s", parsed.Type)
	}
}

func TestBridgeConfig(t *testing.T) {
	config := &BridgeConfig{
		RendezvousURL:     "wss://relay.example.com",
		LocalRelayURL:     "ws://localhost:3334",
		AuthorizedSecrets: map[string]string{"pubkey1": "device1"},
		SessionTimeout:    time.Minute,
	}

	bridge := NewBridge(config)
	if bridge == nil {
		t.Fatal("NewBridge returned nil")
	}

	// Bridge shouldn't start without a valid rendezvous relay
	// but we can verify it was created
	bridge.Stop()
}

func TestSubscriptionTooMany(t *testing.T) {
	clientPubkey := make([]byte, 32)
	conversationKey := make([]byte, 32)

	session := NewSession("test-session", clientPubkey, conversationKey, AuthModeSecret, "test-device")

	// Add max subscriptions
	for i := 0; i < DefaultMaxSubscriptions; i++ {
		if err := session.AddSubscription(string(rune(i))); err != nil {
			t.Fatalf("AddSubscription() error = %v at iteration %d", err, i)
		}
	}

	// Next one should fail
	err := session.AddSubscription("overflow")
	if err != ErrTooManySubscriptions {
		t.Errorf("expected ErrTooManySubscriptions, got %v", err)
	}

	session.Close()
}
