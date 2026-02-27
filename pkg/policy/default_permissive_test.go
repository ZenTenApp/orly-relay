package policy

import (
	"testing"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/lol/chk"
)

// =============================================================================
// Default-Permissive Access Control Tests
// =============================================================================

// TestDefaultPermissiveRead tests that read access is allowed by default
// when no read restrictions are configured.
func TestDefaultPermissiveRead(t *testing.T) {
	// No read restrictions configured
	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"1": {
				"description": "No read restrictions"
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	authorSigner, authorPubkey := generateTestKeypair(t)
	_, readerPubkey := generateTestKeypair(t)
	_, randomPubkey := generateTestKeypair(t)

	ev := createTestEvent(t, authorSigner, "test content", 1)

	tests := []struct {
		name        string
		pubkey      []byte
		expectAllow bool
	}{
		{
			name:        "author can read (default permissive)",
			pubkey:      authorPubkey,
			expectAllow: true,
		},
		{
			name:        "reader can read (default permissive)",
			pubkey:      readerPubkey,
			expectAllow: true,
		},
		{
			name:        "random user can read (default permissive)",
			pubkey:      randomPubkey,
			expectAllow: true,
		},
		{
			name:        "nil pubkey can read (default permissive)",
			pubkey:      nil,
			expectAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := policy.CheckPolicy("read", ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}
			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// TestDefaultPermissiveWrite tests that write access is allowed by default
// when no write restrictions are configured.
func TestDefaultPermissiveWrite(t *testing.T) {
	// No write restrictions configured
	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"1": {
				"description": "No write restrictions"
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	writerSigner, writerPubkey := generateTestKeypair(t)
	_, randomPubkey := generateTestKeypair(t)

	tests := []struct {
		name        string
		signer      *p8k.Signer
		pubkey      []byte
		expectAllow bool
	}{
		{
			name:        "writer can write (default permissive)",
			signer:      writerSigner,
			pubkey:      writerPubkey,
			expectAllow: true,
		},
		{
			name:        "random user can write (default permissive)",
			signer:      writerSigner,
			pubkey:      randomPubkey,
			expectAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createTestEvent(t, tt.signer, "test content", 1)
			allowed, err := policy.CheckPolicy("write", ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}
			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// TestReadFollowsWhitelist tests the read_follows_whitelist field.
func TestReadFollowsWhitelist(t *testing.T) {
	_, curatorPubkey := generateTestKeypair(t)
	_, followedPubkey := generateTestKeypair(t)
	_, unfollowedPubkey := generateTestKeypair(t)
	authorSigner, authorPubkey := generateTestKeypair(t)

	curatorHex := hex.Enc(curatorPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"1": {
				"description": "Only curator follows can read",
				"read_follows_whitelist": ["` + curatorHex + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Simulate loading curator's follows (includes followed user and curator themselves)
	policy.UpdateRuleReadFollowsWhitelist(1, [][]byte{followedPubkey})

	ev := createTestEvent(t, authorSigner, "test content", 1)

	tests := []struct {
		name        string
		pubkey      []byte
		expectAllow bool
	}{
		{
			name:        "curator can read (is in whitelist pubkeys)",
			pubkey:      curatorPubkey,
			expectAllow: true,
		},
		{
			name:        "followed user can read",
			pubkey:      followedPubkey,
			expectAllow: true,
		},
		{
			name:        "unfollowed user denied",
			pubkey:      unfollowedPubkey,
			expectAllow: false,
		},
		{
			name:        "author cannot read (not in follows)",
			pubkey:      authorPubkey,
			expectAllow: false,
		},
		{
			name:        "nil pubkey denied",
			pubkey:      nil,
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := policy.CheckPolicy("read", ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}
			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}

	// Verify write is still default-permissive (no write restriction)
	t.Run("write is still default permissive", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("write", ev, unfollowedPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if !allowed {
			t.Error("Expected write to be allowed (no write restriction)")
		}
	})
}

// TestWriteFollowsWhitelist tests the write_follows_whitelist field.
func TestWriteFollowsWhitelist(t *testing.T) {
	moderatorSigner, moderatorPubkey := generateTestKeypair(t)
	followedSigner, followedPubkey := generateTestKeypair(t)
	unfollowedSigner, unfollowedPubkey := generateTestKeypair(t)

	moderatorHex := hex.Enc(moderatorPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"1": {
				"description": "Only moderator follows can write",
				"write_follows_whitelist": ["` + moderatorHex + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Simulate loading moderator's follows
	policy.UpdateRuleWriteFollowsWhitelist(1, [][]byte{followedPubkey})

	tests := []struct {
		name        string
		signer      *p8k.Signer
		pubkey      []byte
		expectAllow bool
	}{
		{
			name:        "moderator can write (is in whitelist pubkeys)",
			signer:      moderatorSigner,
			pubkey:      moderatorPubkey,
			expectAllow: true,
		},
		{
			name:        "followed user can write",
			signer:      followedSigner,
			pubkey:      followedPubkey,
			expectAllow: true,
		},
		{
			name:        "unfollowed user denied",
			signer:      unfollowedSigner,
			pubkey:      unfollowedPubkey,
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createTestEvent(t, tt.signer, "test content", 1)
			allowed, err := policy.CheckPolicy("write", ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}
			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}

	// Verify read is still default-permissive (no read restriction)
	t.Run("read is still default permissive", func(t *testing.T) {
		ev := createTestEvent(t, unfollowedSigner, "test content", 1)
		allowed, err := policy.CheckPolicy("read", ev, unfollowedPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if !allowed {
			t.Error("Expected read to be allowed (no read restriction)")
		}
	})
}

// TestGlobalReadFollowsWhitelist tests read_follows_whitelist in global rule.
func TestGlobalReadFollowsWhitelist(t *testing.T) {
	_, curatorPubkey := generateTestKeypair(t)
	_, followedPubkey := generateTestKeypair(t)
	_, unfollowedPubkey := generateTestKeypair(t)
	authorSigner, _ := generateTestKeypair(t)

	curatorHex := hex.Enc(curatorPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"global": {
			"description": "Global read follows whitelist",
			"read_follows_whitelist": ["` + curatorHex + `"]
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Update global read follows whitelist
	policy.UpdateGlobalReadFollowsWhitelist([][]byte{followedPubkey})

	// Test with kind 1
	t.Run("kind 1", func(t *testing.T) {
		ev := createTestEvent(t, authorSigner, "test content", 1)

		// Followed user can read
		allowed, err := policy.CheckPolicy("read", ev, followedPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if !allowed {
			t.Error("Expected followed user to be allowed to read")
		}

		// Unfollowed user denied
		allowed, err = policy.CheckPolicy("read", ev, unfollowedPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if allowed {
			t.Error("Expected unfollowed user to be denied")
		}
	})
}

// TestGlobalWriteFollowsWhitelist tests write_follows_whitelist in global rule.
func TestGlobalWriteFollowsWhitelist(t *testing.T) {
	_, moderatorPubkey := generateTestKeypair(t)
	followedSigner, followedPubkey := generateTestKeypair(t)
	unfollowedSigner, unfollowedPubkey := generateTestKeypair(t)

	moderatorHex := hex.Enc(moderatorPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"global": {
			"description": "Global write follows whitelist",
			"write_follows_whitelist": ["` + moderatorHex + `"]
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Update global write follows whitelist
	policy.UpdateGlobalWriteFollowsWhitelist([][]byte{followedPubkey})

	// Test with kind 1
	t.Run("kind 1", func(t *testing.T) {
		// Followed user can write
		ev := createTestEvent(t, followedSigner, "test content", 1)
		allowed, err := policy.CheckPolicy("write", ev, followedPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if !allowed {
			t.Error("Expected followed user to be allowed to write")
		}

		// Unfollowed user denied
		ev = createTestEvent(t, unfollowedSigner, "test content", 1)
		allowed, err = policy.CheckPolicy("write", ev, unfollowedPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if allowed {
			t.Error("Expected unfollowed user to be denied")
		}
	})
}

// TestPrivilegedOnlyAppliesToReadDP tests that privileged only affects read access.
func TestPrivilegedOnlyAppliesToReadDP(t *testing.T) {
	authorSigner, authorPubkey := generateTestKeypair(t)
	_, recipientPubkey := generateTestKeypair(t)
	thirdPartySigner, thirdPartyPubkey := generateTestKeypair(t)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"4": {
				"description": "Encrypted DMs - privileged",
				"privileged": true
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Create event with p-tag for recipient
	ev := event.New()
	ev.Kind = 4
	ev.Content = []byte("encrypted content")
	ev.CreatedAt = time.Now().Unix()
	ev.Tags = tag.NewS()
	pTag := tag.NewFromAny("p", hex.Enc(recipientPubkey))
	ev.Tags.Append(pTag)
	if err := ev.Sign(authorSigner); chk.E(err) {
		t.Fatalf("Failed to sign event: %v", err)
	}

	// READ tests
	t.Run("author can read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, authorPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if !allowed {
			t.Error("Expected author to be allowed to read")
		}
	})

	t.Run("recipient can read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, recipientPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if !allowed {
			t.Error("Expected recipient to be allowed to read")
		}
	})

	t.Run("third party cannot read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, thirdPartyPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if allowed {
			t.Error("Expected third party to be denied read access")
		}
	})

	// WRITE tests - privileged should NOT affect write
	t.Run("third party CAN write (privileged doesn't affect write)", func(t *testing.T) {
		ev := createTestEvent(t, thirdPartySigner, "test content", 4)
		allowed, err := policy.CheckPolicy("write", ev, thirdPartyPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("CheckPolicy error: %v", err)
		}
		if !allowed {
			t.Error("Expected third party to be allowed to write (privileged doesn't restrict write)")
		}
	})
}

// TestCombinedReadWriteFollowsWhitelists tests using both whitelists on same rule.
func TestCombinedReadWriteFollowsWhitelists(t *testing.T) {
	_, curatorPubkey := generateTestKeypair(t)
	_, moderatorPubkey := generateTestKeypair(t)
	readerSigner, readerPubkey := generateTestKeypair(t)
	writerSigner, writerPubkey := generateTestKeypair(t)
	_, outsiderPubkey := generateTestKeypair(t)

	curatorHex := hex.Enc(curatorPubkey)
	moderatorHex := hex.Enc(moderatorPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"30023": {
				"description": "Articles - different read/write follows",
				"read_follows_whitelist": ["` + curatorHex + `"],
				"write_follows_whitelist": ["` + moderatorHex + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Curator follows reader, moderator follows writer
	policy.UpdateRuleReadFollowsWhitelist(30023, [][]byte{readerPubkey})
	policy.UpdateRuleWriteFollowsWhitelist(30023, [][]byte{writerPubkey})

	tests := []struct {
		name        string
		access      string
		signer      *p8k.Signer
		pubkey      []byte
		expectAllow bool
	}{
		// Read tests
		{
			name:        "reader can read",
			access:      "read",
			signer:      readerSigner,
			pubkey:      readerPubkey,
			expectAllow: true,
		},
		{
			name:        "writer cannot read (not in read follows)",
			access:      "read",
			signer:      writerSigner,
			pubkey:      writerPubkey,
			expectAllow: false,
		},
		{
			name:        "outsider cannot read",
			access:      "read",
			signer:      readerSigner,
			pubkey:      outsiderPubkey,
			expectAllow: false,
		},
		// Write tests
		{
			name:        "writer can write",
			access:      "write",
			signer:      writerSigner,
			pubkey:      writerPubkey,
			expectAllow: true,
		},
		{
			name:        "reader cannot write (not in write follows)",
			access:      "write",
			signer:      readerSigner,
			pubkey:      readerPubkey,
			expectAllow: false,
		},
		{
			name:        "outsider cannot write",
			access:      "write",
			signer:      readerSigner,
			pubkey:      outsiderPubkey,
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createTestEvent(t, tt.signer, "test content", 30023)
			allowed, err := policy.CheckPolicy(tt.access, ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}
			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// TestReadAllowWithReadFollowsWhitelist tests combining read_allow and read_follows_whitelist.
func TestReadAllowWithReadFollowsWhitelist(t *testing.T) {
	_, curatorPubkey := generateTestKeypair(t)
	_, followedPubkey := generateTestKeypair(t)
	_, explicitPubkey := generateTestKeypair(t)
	_, outsiderPubkey := generateTestKeypair(t)
	authorSigner, _ := generateTestKeypair(t)

	curatorHex := hex.Enc(curatorPubkey)
	explicitHex := hex.Enc(explicitPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"1": {
				"description": "Read via follows OR explicit allow",
				"read_follows_whitelist": ["` + curatorHex + `"],
				"read_allow": ["` + explicitHex + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	policy.UpdateRuleReadFollowsWhitelist(1, [][]byte{followedPubkey})

	ev := createTestEvent(t, authorSigner, "test content", 1)

	tests := []struct {
		name        string
		pubkey      []byte
		expectAllow bool
	}{
		{
			name:        "followed user can read",
			pubkey:      followedPubkey,
			expectAllow: true,
		},
		{
			name:        "explicit allow user can read",
			pubkey:      explicitPubkey,
			expectAllow: true,
		},
		{
			name:        "curator can read (is whitelist pubkey)",
			pubkey:      curatorPubkey,
			expectAllow: true,
		},
		{
			name:        "outsider denied",
			pubkey:      outsiderPubkey,
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := policy.CheckPolicy("read", ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}
			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// TestGetAllFollowsWhitelistPubkeysDP tests the combined pubkey retrieval.
func TestGetAllFollowsWhitelistPubkeysDP(t *testing.T) {
	read1 := "1111111111111111111111111111111111111111111111111111111111111111"
	read2 := "2222222222222222222222222222222222222222222222222222222222222222"
	write1 := "3333333333333333333333333333333333333333333333333333333333333333"
	legacy := "4444444444444444444444444444444444444444444444444444444444444444"

	policyJSON := []byte(`{
		"default_policy": "allow",
		"global": {
			"read_follows_whitelist": ["` + read1 + `"],
			"write_follows_whitelist": ["` + write1 + `"]
		},
		"rules": {
			"1": {
				"read_follows_whitelist": ["` + read2 + `"],
				"follows_whitelist_admins": ["` + legacy + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	allPubkeys := policy.GetAllFollowsWhitelistPubkeys()
	if len(allPubkeys) != 4 {
		t.Errorf("Expected 4 unique pubkeys, got %d", len(allPubkeys))
	}

	// Check each is present
	pubkeySet := make(map[string]bool)
	for _, pk := range allPubkeys {
		pubkeySet[pk] = true
	}

	expected := []string{read1, read2, write1, legacy}
	for _, exp := range expected {
		if !pubkeySet[exp] {
			t.Errorf("Expected pubkey %s not found", exp)
		}
	}
}
