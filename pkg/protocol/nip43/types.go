package nip43

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// Event kinds defined by NIP-43
const (
	KindMemberList   = 13534 // Membership list published by relay
	KindAddUser      = 8000  // Add user event published by relay
	KindRemoveUser   = 8001  // Remove user event published by relay
	KindJoinRequest  = 28934 // Join request sent by user
	KindInviteReq    = 28935 // Invite request (ephemeral)
	KindLeaveRequest = 28936 // Leave request sent by user
)

// InviteCode represents a claim/invite code for relay access
type InviteCode struct {
	Code      string
	ExpiresAt time.Time
	UsedBy    []byte // pubkey that used this code, nil if unused
	CreatedAt time.Time
}

// InviteManager manages invite codes for NIP-43
type InviteManager struct {
	mu     sync.RWMutex
	codes  map[string]*InviteCode
	expiry time.Duration
}

// NewInviteManager creates a new invite code manager
func NewInviteManager(expiryDuration time.Duration) *InviteManager {
	if expiryDuration == 0 {
		expiryDuration = 24 * time.Hour // Default: 24 hours
	}
	return &InviteManager{
		codes:  make(map[string]*InviteCode),
		expiry: expiryDuration,
	}
}

// GenerateCode creates a new invite code
func (im *InviteManager) GenerateCode() (code string, err error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	code = base64.URLEncoding.EncodeToString(b)

	im.mu.Lock()
	defer im.mu.Unlock()

	im.codes[code] = &InviteCode{
		Code:      code,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(im.expiry),
	}

	return code, nil
}

// ValidateAndConsume validates an invite code and marks it as used by the given pubkey
func (im *InviteManager) ValidateAndConsume(code string, pubkey []byte) (valid bool, reason string) {
	im.mu.Lock()
	defer im.mu.Unlock()

	invite, exists := im.codes[code]
	if !exists {
		return false, "invalid invite code"
	}

	if time.Now().After(invite.ExpiresAt) {
		delete(im.codes, code)
		return false, "invite code expired"
	}

	if invite.UsedBy != nil {
		return false, "invite code already used"
	}

	// Mark as used
	invite.UsedBy = make([]byte, len(pubkey))
	copy(invite.UsedBy, pubkey)

	return true, ""
}

// CleanupExpired removes expired invite codes
func (im *InviteManager) CleanupExpired() {
	im.mu.Lock()
	defer im.mu.Unlock()

	now := time.Now()
	for code, invite := range im.codes {
		if now.After(invite.ExpiresAt) {
			delete(im.codes, code)
		}
	}
}

// BuildMemberListEvent creates a kind 13534 membership list event
// relaySecretKey: the relay's identity secret key (32 bytes)
// members: list of member pubkeys (32 bytes each)
func BuildMemberListEvent(relaySecretKey []byte, members [][]byte) (*event.E, error) {
	// Create signer
	signer, err := p8k.New()
	if err != nil {
		return nil, err
	}
	if err = signer.InitSec(relaySecretKey); err != nil {
		return nil, err
	}

	ev := event.New()
	ev.Kind = KindMemberList
	copy(ev.Pubkey, signer.Pub())

	// Initialize tags
	ev.Tags = tag.NewS()

	// Add NIP-70 `-` tag
	ev.Tags.Append(tag.NewFromAny("-"))

	// Add member tags
	for _, member := range members {
		if len(member) == 32 {
			ev.Tags.Append(tag.NewFromAny("member", hex.Enc(member)))
		}
	}

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// BuildAddUserEvent creates a kind 8000 add user event
func BuildAddUserEvent(relaySecretKey []byte, userPubkey []byte) (*event.E, error) {
	// Create signer
	signer, err := p8k.New()
	if err != nil {
		return nil, err
	}
	if err = signer.InitSec(relaySecretKey); err != nil {
		return nil, err
	}

	ev := event.New()
	ev.Kind = KindAddUser
	copy(ev.Pubkey, signer.Pub())

	// Initialize tags
	ev.Tags = tag.NewS()

	// Add NIP-70 `-` tag
	ev.Tags.Append(tag.NewFromAny("-"))

	// Add p tag for the user
	if len(userPubkey) == 32 {
		ev.Tags.Append(tag.NewFromAny("p", hex.Enc(userPubkey)))
	}

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// BuildRemoveUserEvent creates a kind 8001 remove user event
func BuildRemoveUserEvent(relaySecretKey []byte, userPubkey []byte) (*event.E, error) {
	// Create signer
	signer, err := p8k.New()
	if err != nil {
		return nil, err
	}
	if err = signer.InitSec(relaySecretKey); err != nil {
		return nil, err
	}

	ev := event.New()
	ev.Kind = KindRemoveUser
	copy(ev.Pubkey, signer.Pub())

	// Initialize tags
	ev.Tags = tag.NewS()

	// Add NIP-70 `-` tag
	ev.Tags.Append(tag.NewFromAny("-"))

	// Add p tag for the user
	if len(userPubkey) == 32 {
		ev.Tags.Append(tag.NewFromAny("p", hex.Enc(userPubkey)))
	}

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// BuildInviteEvent creates a kind 28935 invite event (ephemeral)
func BuildInviteEvent(relaySecretKey []byte, inviteCode string) (*event.E, error) {
	// Create signer
	signer, err := p8k.New()
	if err != nil {
		return nil, err
	}
	if err = signer.InitSec(relaySecretKey); err != nil {
		return nil, err
	}

	ev := event.New()
	ev.Kind = KindInviteReq
	copy(ev.Pubkey, signer.Pub())

	// Initialize tags
	ev.Tags = tag.NewS()

	// Add NIP-70 `-` tag
	ev.Tags.Append(tag.NewFromAny("-"))

	// Add claim tag
	ev.Tags.Append(tag.NewFromAny("claim", inviteCode))

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// ValidateJoinRequest validates a kind 28934 join request event
func ValidateJoinRequest(ev *event.E) (inviteCode string, valid bool, reason string) {
	// Must be kind 28934
	if ev.Kind != KindJoinRequest {
		return "", false, "invalid event kind"
	}

	// Must have NIP-70 `-` tag
	hasMinusTag := ev.Tags.GetFirst([]byte("-")) != nil
	if !hasMinusTag {
		return "", false, "missing NIP-70 `-` tag"
	}

	// Must have claim tag
	claimTag := ev.Tags.GetFirst([]byte("claim"))
	if claimTag != nil && claimTag.Len() >= 2 {
		inviteCode = string(claimTag.T[1])
	}
	if inviteCode == "" {
		return "", false, "missing claim tag"
	}

	// Check timestamp (must be recent, within +/- 10 minutes)
	now := time.Now().Unix()
	if ev.CreatedAt < now-600 || ev.CreatedAt > now+600 {
		return inviteCode, false, "timestamp out of range"
	}

	return inviteCode, true, ""
}

// ValidateLeaveRequest validates a kind 28936 leave request event
func ValidateLeaveRequest(ev *event.E) (valid bool, reason string) {
	// Must be kind 28936
	if ev.Kind != KindLeaveRequest {
		return false, "invalid event kind"
	}

	// Must have NIP-70 `-` tag
	hasMinusTag := ev.Tags.GetFirst([]byte("-")) != nil
	if !hasMinusTag {
		return false, "missing NIP-70 `-` tag"
	}

	// Check timestamp (must be recent, within +/- 10 minutes)
	now := time.Now().Unix()
	if ev.CreatedAt < now-600 || ev.CreatedAt > now+600 {
		return false, "timestamp out of range"
	}

	return true, ""
}
