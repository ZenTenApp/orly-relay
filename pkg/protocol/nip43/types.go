package nip43

import (
	"crypto/rand"
	"encoding/base64"
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

// --- actor request/response types ---

type imGenerateCodeReq struct {
	resp chan imGenerateCodeResp
}

type imGenerateCodeResp struct {
	code string
	err  error
}

type imValidateAndConsumeReq struct {
	code   string
	pubkey []byte
	resp   chan imValidateAndConsumeResp
}

type imValidateAndConsumeResp struct {
	valid  bool
	reason string
}

type imCleanupExpiredReq struct {
	resp chan struct{}
}

// InviteManager manages invite codes for NIP-43
type InviteManager struct {
	codes  map[string]*InviteCode
	expiry time.Duration

	generateCodeCh       chan imGenerateCodeReq
	validateAndConsumeCh chan imValidateAndConsumeReq
	cleanupExpiredCh     chan imCleanupExpiredReq

	stop chan struct{}
	done chan struct{}
}

// NewInviteManager creates a new invite code manager
func NewInviteManager(expiryDuration time.Duration) *InviteManager {
	if expiryDuration == 0 {
		expiryDuration = 24 * time.Hour // Default: 24 hours
	}
	im := &InviteManager{
		codes:  make(map[string]*InviteCode),
		expiry: expiryDuration,

		generateCodeCh:       make(chan imGenerateCodeReq),
		validateAndConsumeCh: make(chan imValidateAndConsumeReq),
		cleanupExpiredCh:     make(chan imCleanupExpiredReq),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go im.run()
	return im
}

// Stop shuts down the actor goroutine.
func (im *InviteManager) Stop() {
	close(im.stop)
	<-im.done
}

func (im *InviteManager) run() {
	defer close(im.done)
	for {
		select {
		case <-im.stop:
			return
		case req := <-im.generateCodeCh:
			code, err := im.doGenerateCode()
			req.resp <- imGenerateCodeResp{code: code, err: err}
		case req := <-im.validateAndConsumeCh:
			valid, reason := im.doValidateAndConsume(req.code, req.pubkey)
			req.resp <- imValidateAndConsumeResp{valid: valid, reason: reason}
		case req := <-im.cleanupExpiredCh:
			im.doCleanupExpired()
			req.resp <- struct{}{}
		}
	}
}

func (im *InviteManager) doGenerateCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := base64.URLEncoding.EncodeToString(b)

	im.codes[code] = &InviteCode{
		Code:      code,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(im.expiry),
	}

	return code, nil
}

func (im *InviteManager) doValidateAndConsume(code string, pubkey []byte) (bool, string) {
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

	invite.UsedBy = make([]byte, len(pubkey))
	copy(invite.UsedBy, pubkey)

	return true, ""
}

func (im *InviteManager) doCleanupExpired() {
	now := time.Now()
	for code, invite := range im.codes {
		if now.After(invite.ExpiresAt) {
			delete(im.codes, code)
		}
	}
}

// GenerateCode creates a new invite code
func (im *InviteManager) GenerateCode() (string, error) {
	resp := make(chan imGenerateCodeResp, 1)
	select {
	case im.generateCodeCh <- imGenerateCodeReq{resp: resp}:
		r := <-resp
		return r.code, r.err
	case <-im.stop:
		return "", nil
	}
}

// ValidateAndConsume validates an invite code and marks it as used by the given pubkey
func (im *InviteManager) ValidateAndConsume(code string, pubkey []byte) (bool, string) {
	resp := make(chan imValidateAndConsumeResp, 1)
	select {
	case im.validateAndConsumeCh <- imValidateAndConsumeReq{code: code, pubkey: pubkey, resp: resp}:
		r := <-resp
		return r.valid, r.reason
	case <-im.stop:
		return false, "invite manager stopped"
	}
}

// CleanupExpired removes expired invite codes
func (im *InviteManager) CleanupExpired() {
	resp := make(chan struct{}, 1)
	select {
	case im.cleanupExpiredCh <- imCleanupExpiredReq{resp: resp}:
		<-resp
	case <-im.stop:
	}
}

// BuildMemberListEvent creates a kind 13534 membership list event
// relaySecretKey: the relay's identity secret key (32 bytes)
// members: list of member pubkeys (32 bytes each)
func BuildMemberListEvent(relaySecretKey []byte, members [][]byte) (*event.E, error) {
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

	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))

	for _, member := range members {
		if len(member) == 32 {
			ev.Tags.Append(tag.NewFromAny("member", hex.Enc(member)))
		}
	}

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// BuildAddUserEvent creates a kind 8000 add user event
func BuildAddUserEvent(relaySecretKey []byte, userPubkey []byte) (*event.E, error) {
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

	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))

	if len(userPubkey) == 32 {
		ev.Tags.Append(tag.NewFromAny("p", hex.Enc(userPubkey)))
	}

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// BuildRemoveUserEvent creates a kind 8001 remove user event
func BuildRemoveUserEvent(relaySecretKey []byte, userPubkey []byte) (*event.E, error) {
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

	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))

	if len(userPubkey) == 32 {
		ev.Tags.Append(tag.NewFromAny("p", hex.Enc(userPubkey)))
	}

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// BuildInviteEvent creates a kind 28935 invite event (ephemeral)
func BuildInviteEvent(relaySecretKey []byte, inviteCode string) (*event.E, error) {
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

	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))
	ev.Tags.Append(tag.NewFromAny("claim", inviteCode))

	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err := ev.Sign(signer); err != nil {
		return nil, err
	}

	return ev, nil
}

// ValidateJoinRequest validates a kind 28934 join request event
func ValidateJoinRequest(ev *event.E) (inviteCode string, valid bool, reason string) {
	if ev.Kind != KindJoinRequest {
		return "", false, "invalid event kind"
	}

	hasMinusTag := ev.Tags.GetFirst([]byte("-")) != nil
	if !hasMinusTag {
		return "", false, "missing NIP-70 `-` tag"
	}

	claimTag := ev.Tags.GetFirst([]byte("claim"))
	if claimTag != nil && claimTag.Len() >= 2 {
		inviteCode = string(claimTag.T[1])
	}
	if inviteCode == "" {
		return "", false, "missing claim tag"
	}

	now := time.Now().Unix()
	if ev.CreatedAt < now-600 || ev.CreatedAt > now+600 {
		return inviteCode, false, "timestamp out of range"
	}

	return inviteCode, true, ""
}

// ValidateLeaveRequest validates a kind 28936 leave request event
func ValidateLeaveRequest(ev *event.E) (valid bool, reason string) {
	if ev.Kind != KindLeaveRequest {
		return false, "invalid event kind"
	}

	hasMinusTag := ev.Tags.GetFirst([]byte("-")) != nil
	if !hasMinusTag {
		return false, "missing NIP-70 `-` tag"
	}

	now := time.Now().Unix()
	if ev.CreatedAt < now-600 || ev.CreatedAt > now+600 {
		return false, "timestamp out of range"
	}

	return true, ""
}
