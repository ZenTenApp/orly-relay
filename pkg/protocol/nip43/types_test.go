package nip43

import (
	"testing"
	"time"

	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

// TestInviteManager_GenerateCode tests invite code generation
func TestInviteManager_GenerateCode(t *testing.T) {
	im := NewInviteManager(24 * time.Hour)

	code, err := im.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	if code == "" {
		t.Fatal("generated code is empty")
	}

	// Verify the code exists in the manager
	im.mu.Lock()
	invite, exists := im.codes[code]
	im.mu.Unlock()

	if !exists {
		t.Fatal("generated code not found in manager")
	}

	if invite.Code != code {
		t.Errorf("code mismatch: got %s, want %s", invite.Code, code)
	}

	if invite.UsedBy != nil {
		t.Error("newly generated code should not be used")
	}

	if time.Until(invite.ExpiresAt) > 24*time.Hour {
		t.Error("expiry time is too far in the future")
	}
}

// TestInviteManager_ValidateAndConsume tests invite code validation
func TestInviteManager_ValidateAndConsume(t *testing.T) {
	im := NewInviteManager(24 * time.Hour)

	// Generate a code
	code, err := im.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	testPubkey := make([]byte, 32)
	for i := range testPubkey {
		testPubkey[i] = byte(i)
	}

	// Test valid code
	valid, reason := im.ValidateAndConsume(code, testPubkey)
	if !valid {
		t.Fatalf("valid code rejected: %s", reason)
	}

	// Test already used code
	valid, reason = im.ValidateAndConsume(code, testPubkey)
	if valid {
		t.Error("already used code was accepted")
	}
	if reason != "invite code already used" {
		t.Errorf("wrong rejection reason: got %s", reason)
	}

	// Test invalid code
	valid, reason = im.ValidateAndConsume("invalid-code", testPubkey)
	if valid {
		t.Error("invalid code was accepted")
	}
	if reason != "invalid invite code" {
		t.Errorf("wrong rejection reason: got %s", reason)
	}
}

// TestInviteManager_ExpiredCode tests expired invite code handling
func TestInviteManager_ExpiredCode(t *testing.T) {
	// Create manager with very short expiry
	im := NewInviteManager(1 * time.Millisecond)

	code, err := im.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	testPubkey := make([]byte, 32)
	valid, reason := im.ValidateAndConsume(code, testPubkey)
	if valid {
		t.Error("expired code was accepted")
	}
	if reason != "invite code expired" {
		t.Errorf("wrong rejection reason: got %s, want 'invite code expired'", reason)
	}

	// Verify code was deleted
	im.mu.Lock()
	_, exists := im.codes[code]
	im.mu.Unlock()

	if exists {
		t.Error("expired code was not deleted")
	}
}

// TestInviteManager_CleanupExpired tests cleanup of expired codes
func TestInviteManager_CleanupExpired(t *testing.T) {
	im := NewInviteManager(1 * time.Millisecond)

	// Generate multiple codes
	codes := make([]string, 5)
	for i := 0; i < 5; i++ {
		code, err := im.GenerateCode()
		if err != nil {
			t.Fatalf("failed to generate code %d: %v", i, err)
		}
		codes[i] = code
	}

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Cleanup
	im.CleanupExpired()

	// Verify all codes were deleted
	im.mu.Lock()
	remaining := len(im.codes)
	im.mu.Unlock()

	if remaining != 0 {
		t.Errorf("cleanup failed: %d codes remaining", remaining)
	}
}

// TestBuildMemberListEvent tests membership list event creation
func TestBuildMemberListEvent(t *testing.T) {
	// Generate a test relay secret
	relaySecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate relay secret: %v", err)
	}

	// Create test member pubkeys
	members := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		members[i] = make([]byte, 32)
		for j := range members[i] {
			members[i][j] = byte(i*10 + j)
		}
	}

	// Build event
	ev, err := BuildMemberListEvent(relaySecret, members)
	if err != nil {
		t.Fatalf("failed to build member list event: %v", err)
	}

	// Verify event kind
	if ev.Kind != KindMemberList {
		t.Errorf("wrong event kind: got %d, want %d", ev.Kind, KindMemberList)
	}

	// Verify NIP-70 tag
	minusTag := ev.Tags.GetFirst([]byte("-"))
	if minusTag == nil {
		t.Error("missing NIP-70 `-` tag")
	}

	// Verify member tags
	memberTags := ev.Tags.GetAll([]byte("member"))
	if len(memberTags) != 3 {
		t.Errorf("wrong number of member tags: got %d, want 3", len(memberTags))
	}

	// Verify signature
	valid, err := ev.Verify()
	if err != nil {
		t.Fatalf("signature verification error: %v", err)
	}
	if !valid {
		t.Error("event signature is invalid")
	}
}

// TestBuildAddUserEvent tests add user event creation
func TestBuildAddUserEvent(t *testing.T) {
	relaySecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate relay secret: %v", err)
	}

	userPubkey := make([]byte, 32)
	for i := range userPubkey {
		userPubkey[i] = byte(i)
	}

	ev, err := BuildAddUserEvent(relaySecret, userPubkey)
	if err != nil {
		t.Fatalf("failed to build add user event: %v", err)
	}

	// Verify event kind
	if ev.Kind != KindAddUser {
		t.Errorf("wrong event kind: got %d, want %d", ev.Kind, KindAddUser)
	}

	// Verify NIP-70 tag
	minusTag := ev.Tags.GetFirst([]byte("-"))
	if minusTag == nil {
		t.Error("missing NIP-70 `-` tag")
	}

	// Verify p tag
	pTag := ev.Tags.GetFirst([]byte("p"))
	if pTag == nil {
		t.Error("missing p tag")
	}

	// Verify signature
	valid, err := ev.Verify()
	if err != nil {
		t.Fatalf("signature verification error: %v", err)
	}
	if !valid {
		t.Error("event signature is invalid")
	}
}

// TestBuildRemoveUserEvent tests remove user event creation
func TestBuildRemoveUserEvent(t *testing.T) {
	relaySecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate relay secret: %v", err)
	}

	userPubkey := make([]byte, 32)
	for i := range userPubkey {
		userPubkey[i] = byte(i)
	}

	ev, err := BuildRemoveUserEvent(relaySecret, userPubkey)
	if err != nil {
		t.Fatalf("failed to build remove user event: %v", err)
	}

	// Verify event kind
	if ev.Kind != KindRemoveUser {
		t.Errorf("wrong event kind: got %d, want %d", ev.Kind, KindRemoveUser)
	}

	// Verify NIP-70 tag
	minusTag := ev.Tags.GetFirst([]byte("-"))
	if minusTag == nil {
		t.Error("missing NIP-70 `-` tag")
	}

	// Verify p tag
	pTag := ev.Tags.GetFirst([]byte("p"))
	if pTag == nil {
		t.Error("missing p tag")
	}

	// Verify signature
	valid, err := ev.Verify()
	if err != nil {
		t.Fatalf("signature verification error: %v", err)
	}
	if !valid {
		t.Error("event signature is invalid")
	}
}

// TestBuildInviteEvent tests invite event creation
func TestBuildInviteEvent(t *testing.T) {
	relaySecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate relay secret: %v", err)
	}

	inviteCode := "test-invite-code-12345"

	ev, err := BuildInviteEvent(relaySecret, inviteCode)
	if err != nil {
		t.Fatalf("failed to build invite event: %v", err)
	}

	// Verify event kind
	if ev.Kind != KindInviteReq {
		t.Errorf("wrong event kind: got %d, want %d", ev.Kind, KindInviteReq)
	}

	// Verify NIP-70 tag
	minusTag := ev.Tags.GetFirst([]byte("-"))
	if minusTag == nil {
		t.Error("missing NIP-70 `-` tag")
	}

	// Verify claim tag
	claimTag := ev.Tags.GetFirst([]byte("claim"))
	if claimTag == nil {
		t.Error("missing claim tag")
	}
	if claimTag.Len() < 2 {
		t.Error("claim tag has no value")
	}
	if string(claimTag.T[1]) != inviteCode {
		t.Errorf("wrong invite code in tag: got %s, want %s", string(claimTag.T[1]), inviteCode)
	}

	// Verify signature
	valid, err := ev.Verify()
	if err != nil {
		t.Fatalf("signature verification error: %v", err)
	}
	if !valid {
		t.Error("event signature is invalid")
	}
}

// TestValidateJoinRequest tests join request validation
func TestValidateJoinRequest(t *testing.T) {
	tests := []struct {
		name          string
		setupEvent    func() *event.E
		expectValid   bool
		expectCode    string
		expectReason  string
	}{
		{
			name: "valid join request",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindJoinRequest
				ev.Tags = tag.NewS()
				ev.Tags.Append(tag.NewFromAny("-"))
				ev.Tags.Append(tag.NewFromAny("claim", "test-code-123"))
				ev.CreatedAt = time.Now().Unix()
				return ev
			},
			expectValid:  true,
			expectCode:   "test-code-123",
			expectReason: "",
		},
		{
			name: "wrong kind",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = 1000
				return ev
			},
			expectValid:  false,
			expectReason: "invalid event kind",
		},
		{
			name: "missing minus tag",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindJoinRequest
				ev.Tags = tag.NewS()
				ev.Tags.Append(tag.NewFromAny("claim", "test-code"))
				ev.CreatedAt = time.Now().Unix()
				return ev
			},
			expectValid:  false,
			expectReason: "missing NIP-70 `-` tag",
		},
		{
			name: "missing claim tag",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindJoinRequest
				ev.Tags = tag.NewS()
				ev.Tags.Append(tag.NewFromAny("-"))
				ev.CreatedAt = time.Now().Unix()
				return ev
			},
			expectValid:  false,
			expectReason: "missing claim tag",
		},
		{
			name: "timestamp too old",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindJoinRequest
				ev.Tags = tag.NewS()
				ev.Tags.Append(tag.NewFromAny("-"))
				ev.Tags.Append(tag.NewFromAny("claim", "test-code"))
				ev.CreatedAt = time.Now().Unix() - 700 // More than 10 minutes ago
				return ev
			},
			expectValid:  false,
			expectCode:   "test-code",
			expectReason: "timestamp out of range",
		},
		{
			name: "timestamp too far in future",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindJoinRequest
				ev.Tags = tag.NewS()
				ev.Tags.Append(tag.NewFromAny("-"))
				ev.Tags.Append(tag.NewFromAny("claim", "test-code"))
				ev.CreatedAt = time.Now().Unix() + 700 // More than 10 minutes ahead
				return ev
			},
			expectValid:  false,
			expectCode:   "test-code",
			expectReason: "timestamp out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := tt.setupEvent()
			code, valid, reason := ValidateJoinRequest(ev)

			if valid != tt.expectValid {
				t.Errorf("valid mismatch: got %v, want %v", valid, tt.expectValid)
			}
			if tt.expectCode != "" && code != tt.expectCode {
				t.Errorf("code mismatch: got %s, want %s", code, tt.expectCode)
			}
			if tt.expectReason != "" && reason != tt.expectReason {
				t.Errorf("reason mismatch: got %s, want %s", reason, tt.expectReason)
			}
		})
	}
}

// TestValidateLeaveRequest tests leave request validation
func TestValidateLeaveRequest(t *testing.T) {
	tests := []struct {
		name         string
		setupEvent   func() *event.E
		expectValid  bool
		expectReason string
	}{
		{
			name: "valid leave request",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindLeaveRequest
				ev.Tags = tag.NewS()
				ev.Tags.Append(tag.NewFromAny("-"))
				ev.CreatedAt = time.Now().Unix()
				return ev
			},
			expectValid:  true,
			expectReason: "",
		},
		{
			name: "wrong kind",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = 1000
				return ev
			},
			expectValid:  false,
			expectReason: "invalid event kind",
		},
		{
			name: "missing minus tag",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindLeaveRequest
				ev.CreatedAt = time.Now().Unix()
				return ev
			},
			expectValid:  false,
			expectReason: "missing NIP-70 `-` tag",
		},
		{
			name: "timestamp out of range",
			setupEvent: func() *event.E {
				ev := event.New()
				ev.Kind = KindLeaveRequest
				ev.Tags = tag.NewS()
				ev.Tags.Append(tag.NewFromAny("-"))
				ev.CreatedAt = time.Now().Unix() - 700
				return ev
			},
			expectValid:  false,
			expectReason: "timestamp out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := tt.setupEvent()
			valid, reason := ValidateLeaveRequest(ev)

			if valid != tt.expectValid {
				t.Errorf("valid mismatch: got %v, want %v", valid, tt.expectValid)
			}
			if tt.expectReason != "" && reason != tt.expectReason {
				t.Errorf("reason mismatch: got %s, want %s", reason, tt.expectReason)
			}
		})
	}
}
