package directory

import (
	"github.com/minio/sha256-simd"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/nostr/crypto/ec/schnorr"
	"next.orly.dev/pkg/nostr/crypto/ec/secp256k1"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/encoders/event"
)

// IdentityTagBuilder helps construct identity tags with proper signatures.
type IdentityTagBuilder struct {
	identityPrivkey []byte
	identityPubkey  []byte
	npubIdentity    string
}

// NewIdentityTagBuilder creates a new identity tag builder with the given
// identity private key.
func NewIdentityTagBuilder(identityPrivkey []byte) (builder *IdentityTagBuilder, err error) {
	if len(identityPrivkey) != 32 {
		return nil, errorf.E("identity private key must be 32 bytes")
	}

	// Derive public key from secret key using p8k signer
	var signer *p8k.Signer
	if signer, err = p8k.New(); chk.E(err) {
		return nil, errorf.E("failed to create signer: %w", err)
	}
	if err = signer.InitSec(identityPrivkey); chk.E(err) {
		return nil, errorf.E("failed to initialize signer: %w", err)
	}
	identityPubkeyBytes := signer.Pub()

	// Parse public key for npub encoding
	var identityPubkey *secp256k1.PublicKey
	if identityPubkey, err = schnorr.ParsePubKey(identityPubkeyBytes); chk.E(err) {
		return nil, errorf.E("failed to parse public key: %w", err)
	}

	// Encode as npub
	var npubIdentity []byte
	if npubIdentity, err = bech32encoding.PublicKeyToNpub(identityPubkey); chk.E(err) {
		return nil, errorf.E("failed to encode npub: %w", err)
	}

	return &IdentityTagBuilder{
		identityPrivkey: identityPrivkey,
		identityPubkey:  identityPubkeyBytes,
		npubIdentity:    string(npubIdentity),
	}, nil
}

// CreateIdentityTag creates a signed identity tag for the given delegate pubkey.
func (builder *IdentityTagBuilder) CreateIdentityTag(delegatePubkey []byte) (identityTag *IdentityTag, err error) {
	if len(delegatePubkey) != 32 {
		return nil, errorf.E("delegate pubkey must be 32 bytes")
	}

	// Generate nonce
	var nonceHex string
	if nonceHex, err = GenerateNonceHex(16); chk.E(err) {
		return nil, errorf.E("failed to generate nonce: %w", err)
	}

	// Create message: nonce + delegate_pubkey_hex + identity_pubkey_hex
	delegatePubkeyHex := hex.EncodeToString(delegatePubkey)
	identityPubkeyHex := hex.EncodeToString(builder.identityPubkey)
	message := nonceHex + delegatePubkeyHex + identityPubkeyHex

	// Hash and sign using p8k signer
	hash := sha256.Sum256([]byte(message))
	var signer *p8k.Signer
	if signer, err = p8k.New(); chk.E(err) {
		return nil, errorf.E("failed to create signer: %w", err)
	}
	if err = signer.InitSec(builder.identityPrivkey); chk.E(err) {
		return nil, errorf.E("failed to initialize signer: %w", err)
	}
	var signature []byte
	if signature, err = signer.Sign(hash[:]); chk.E(err) {
		return nil, errorf.E("failed to sign identity tag: %w", err)
	}

	identityTag = &IdentityTag{
		NPubIdentity: builder.npubIdentity,
		Nonce:        nonceHex,
		Signature:    hex.EncodeToString(signature),
	}

	return
}

// GetNPubIdentity returns the npub-encoded identity.
func (builder *IdentityTagBuilder) GetNPubIdentity() string {
	return builder.npubIdentity
}

// GetIdentityPubkey returns the raw identity public key.
func (builder *IdentityTagBuilder) GetIdentityPubkey() []byte {
	return builder.identityPubkey
}

// KeyPoolManager helps manage HD key derivation and advertisement.
type KeyPoolManager struct {
	masterSeed     []byte
	identityIndex  uint32
	currentIndices map[KeyPurpose]int
}

// NewKeyPoolManager creates a new key pool manager with the given master seed.
func NewKeyPoolManager(masterSeed []byte, identityIndex uint32) *KeyPoolManager {
	return &KeyPoolManager{
		masterSeed:     masterSeed,
		identityIndex:  identityIndex,
		currentIndices: make(map[KeyPurpose]int),
	}
}

// GenerateDerivationPath creates a BIP32 derivation path for the given purpose and index.
func (kpm *KeyPoolManager) GenerateDerivationPath(purpose KeyPurpose, index int) string {
	var usageIndex int
	switch purpose {
	case KeyPurposeSigning:
		usageIndex = 0
	case KeyPurposeEncryption:
		usageIndex = 1
	case KeyPurposeDelegation:
		usageIndex = 2
	default:
		usageIndex = 0
	}

	return fmt.Sprintf("m/39103'/1237'/%d'/%d/%d", kpm.identityIndex, usageIndex, index)
}

// GetNextKeyIndex returns the next available key index for the given purpose.
func (kpm *KeyPoolManager) GetNextKeyIndex(purpose KeyPurpose) int {
	current := kpm.currentIndices[purpose]
	kpm.currentIndices[purpose] = current + 1
	return current
}

// SetKeyIndex sets the current key index for the given purpose.
func (kpm *KeyPoolManager) SetKeyIndex(purpose KeyPurpose, index int) {
	kpm.currentIndices[purpose] = index
}

// GetCurrentKeyIndex returns the current key index for the given purpose.
func (kpm *KeyPoolManager) GetCurrentKeyIndex(purpose KeyPurpose) int {
	return kpm.currentIndices[purpose]
}

// TrustCalculator helps calculate trust scores and inheritance.
type TrustCalculator struct {
	acts map[string]*TrustAct
}

// NewTrustCalculator creates a new trust calculator.
func NewTrustCalculator() *TrustCalculator {
	return &TrustCalculator{
		acts: make(map[string]*TrustAct),
	}
}

// AddAct adds a trust act to the calculator.
func (tc *TrustCalculator) AddAct(act *TrustAct) {
	key := act.GetTargetPubkey()
	tc.acts[key] = act
}

// GetTrustLevel returns the trust level for a given pubkey.
func (tc *TrustCalculator) GetTrustLevel(pubkey string) TrustLevel {
	if act, exists := tc.acts[pubkey]; exists {
		if !act.IsExpired() {
			return act.GetTrustLevel()
		}
	}
	return TrustLevelNone // Return 0 for no trust
}

// CalculateInheritedTrust calculates inherited trust through the web of trust.
// With numeric trust levels, inherited trust is calculated by multiplying
// the trust percentages at each hop, reducing trust over distance.
func (tc *TrustCalculator) CalculateInheritedTrust(
	fromPubkey, toPubkey string,
) TrustLevel {
	// Direct trust
	if directTrust := tc.GetTrustLevel(toPubkey); directTrust > 0 {
		return directTrust
	}

	// Look for inherited trust through intermediate nodes
	var maxInheritedTrust TrustLevel = 0
	for intermediatePubkey, act := range tc.acts {
		if act.IsExpired() {
			continue
		}

		// Check if we trust the intermediate node
		intermediateLevel := tc.GetTrustLevel(intermediatePubkey)
		if intermediateLevel == 0 {
			continue
		}

		// Check if intermediate node trusts the target
		targetLevel := tc.GetTrustLevel(toPubkey)
		if targetLevel == 0 {
			continue
		}

		// Calculate inherited trust level (multiply percentages)
		inheritedLevel := tc.combinesTrustLevels(intermediateLevel, targetLevel)
		if inheritedLevel > maxInheritedTrust {
			maxInheritedTrust = inheritedLevel
		}
	}

	return maxInheritedTrust
}

// combinesTrustLevels combines two trust levels to calculate inherited trust.
// With numeric trust levels (0-100), inherited trust is calculated by
// multiplying the two percentages: (level1 * level2) / 100
// This naturally reduces trust over distance.
func (tc *TrustCalculator) combinesTrustLevels(level1, level2 TrustLevel) TrustLevel {
	// Multiply percentages: (level1% * level2%) = (level1 * level2) / 100
	// Example: 75% trust * 50% trust = 37.5% inherited trust
	combined := (uint16(level1) * uint16(level2)) / 100
	return TrustLevel(combined)
}

// ReplicationFilter helps determine which events should be replicated.
type ReplicationFilter struct {
	trustCalculator *TrustCalculator
	acts            map[string]*TrustAct
}

// NewReplicationFilter creates a new replication filter.
func NewReplicationFilter(trustCalculator *TrustCalculator) *ReplicationFilter {
	return &ReplicationFilter{
		trustCalculator: trustCalculator,
		acts:            make(map[string]*TrustAct),
	}
}

// AddTrustAct adds a trust act to the filter.
func (rf *ReplicationFilter) AddTrustAct(act *TrustAct) {
	rf.acts[act.GetTargetPubkey()] = act
}

// ShouldReplicate determines if an event should be replicated to a target relay.
func (rf *ReplicationFilter) ShouldReplicate(ev *event.E, targetPubkey string) bool {
	act, exists := rf.acts[targetPubkey]
	if !exists || act.IsExpired() {
		return false
	}

	return act.ShouldReplicate(ev.Kind)
}

// GetReplicationTargets returns all target relays that should receive an event.
func (rf *ReplicationFilter) GetReplicationTargets(ev *event.E) []string {
	var targets []string

	for pubkey, act := range rf.acts {
		if !act.IsExpired() && act.ShouldReplicate(ev.Kind) {
			targets = append(targets, pubkey)
		}
	}

	return targets
}

// EventBatcher helps batch events for efficient replication.
type EventBatcher struct {
	maxBatchSize int
	batches      map[string][]*event.E
}

// NewEventBatcher creates a new event batcher.
func NewEventBatcher(maxBatchSize int) *EventBatcher {
	if maxBatchSize <= 0 {
		maxBatchSize = 100 // Default batch size
	}

	return &EventBatcher{
		maxBatchSize: maxBatchSize,
		batches:      make(map[string][]*event.E),
	}
}

// AddEvent adds an event to the batch for a target relay.
func (eb *EventBatcher) AddEvent(targetRelay string, ev *event.E) {
	eb.batches[targetRelay] = append(eb.batches[targetRelay], ev)
}

// GetBatch returns the current batch for a target relay.
func (eb *EventBatcher) GetBatch(targetRelay string) []*event.E {
	return eb.batches[targetRelay]
}

// IsBatchFull returns true if the batch for a target relay is full.
func (eb *EventBatcher) IsBatchFull(targetRelay string) bool {
	return len(eb.batches[targetRelay]) >= eb.maxBatchSize
}

// FlushBatch returns and clears the batch for a target relay.
func (eb *EventBatcher) FlushBatch(targetRelay string) []*event.E {
	batch := eb.batches[targetRelay]
	eb.batches[targetRelay] = nil
	return batch
}

// GetAllBatches returns all current batches.
func (eb *EventBatcher) GetAllBatches() map[string][]*event.E {
	result := make(map[string][]*event.E)
	for relay, batch := range eb.batches {
		if len(batch) > 0 {
			result[relay] = batch
		}
	}
	return result
}

// FlushAllBatches returns and clears all batches.
func (eb *EventBatcher) FlushAllBatches() map[string][]*event.E {
	result := eb.GetAllBatches()
	eb.batches = make(map[string][]*event.E)
	return result
}

// Utility functions

// ParseKindsList parses a comma-separated list of event kinds.
func ParseKindsList(kindsStr string) (kinds []uint16, err error) {
	if kindsStr == "" {
		return nil, nil
	}

	kindStrings := strings.Split(kindsStr, ",")
	for _, kindStr := range kindStrings {
		kindStr = strings.TrimSpace(kindStr)
		if kindStr == "" {
			continue
		}

		var kind uint64
		if kind, err = strconv.ParseUint(kindStr, 10, 16); chk.E(err) {
			return nil, errorf.E("invalid kind: %s", kindStr)
		}

		kinds = append(kinds, uint16(kind))
	}

	return
}

// FormatKindsList formats a list of event kinds as a comma-separated string.
func FormatKindsList(kinds []uint16) string {
	if len(kinds) == 0 {
		return ""
	}

	var kindStrings []string
	for _, kind := range kinds {
		kindStrings = append(kindStrings, strconv.FormatUint(uint64(kind), 10))
	}

	return strings.Join(kindStrings, ",")
}

// GenerateRequestID generates a unique request ID for replication requests.
func GenerateRequestID() (requestID string, err error) {
	// Use timestamp + random nonce for uniqueness
	timestamp := time.Now().Unix()
	var nonce string
	if nonce, err = GenerateNonceHex(8); chk.E(err) {
		return
	}

	requestID = fmt.Sprintf("%d-%s", timestamp, nonce)
	return
}

// CreateSuccessResponse creates a successful replication response.
func CreateSuccessResponse(
	pubkey []byte,
	requestID, sourceRelay string,
	eventResults []*EventResult,
) (response *DirectoryEventReplicationResponse, err error) {
	return NewDirectoryEventReplicationResponse(
		pubkey,
		requestID,
		ReplicationStatusSuccess,
		"",
		sourceRelay,
		eventResults,
	)
}

// CreateErrorResponse creates an error replication response.
func CreateErrorResponse(
	pubkey []byte,
	requestID, sourceRelay, errorMsg string,
) (response *DirectoryEventReplicationResponse, err error) {
	return NewDirectoryEventReplicationResponse(
		pubkey,
		requestID,
		ReplicationStatusError,
		errorMsg,
		sourceRelay,
		nil,
	)
}

// CreateEventResult creates an event result for a replication response.
func CreateEventResult(eventID string, success bool, errorMsg string) *EventResult {
	status := ReplicationStatusSuccess
	if !success {
		status = ReplicationStatusError
	}

	return &EventResult{
		EventID: eventID,
		Status:  status,
		Error:   errorMsg,
	}
}
