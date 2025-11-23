// Package sync provides relay group configuration management
package sync

import (
	"context"
	"github.com/minio/sha256-simd"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// RelayGroupConfig represents a relay group configuration event
type RelayGroupConfig struct {
	Relays []string `json:"relays"`
}

// RelayGroupManager handles relay group configuration
type RelayGroupManager struct {
	db           *database.D
	authorizedPubkeys [][]byte
}

// NewRelayGroupManager creates a new relay group manager
func NewRelayGroupManager(db *database.D, adminNpubs []string) *RelayGroupManager {
	var pubkeys [][]byte
	for _, npub := range adminNpubs {
		if pk, err := bech32encoding.NpubOrHexToPublicKeyBinary(npub); err == nil {
			pubkeys = append(pubkeys, pk)
		}
	}

	return &RelayGroupManager{
		db:                db,
		authorizedPubkeys: pubkeys,
	}
}

// FindAuthoritativeConfig finds the authoritative relay group configuration
// by selecting the latest event by timestamp, with hash tie-breaking
func (rgm *RelayGroupManager) FindAuthoritativeConfig(ctx context.Context) (*RelayGroupConfig, error) {
	if len(rgm.authorizedPubkeys) == 0 {
		return nil, nil
	}

	// Query for all relay group config events from authorized pubkeys
	f := &filter.F{
		Kinds:   kind.NewS(kind.RelayGroupConfig),
		Authors: tag.NewFromBytesSlice(rgm.authorizedPubkeys...),
	}

	events, err := rgm.db.QueryEvents(ctx, f)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	// Find the authoritative event
	authEvent := rgm.selectAuthoritativeEvent(events)
	if authEvent == nil {
		return nil, nil
	}

	// Parse the configuration from the event content
	var config RelayGroupConfig
	if err := json.Unmarshal([]byte(authEvent.Content), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// selectAuthoritativeEvent selects the authoritative event using the specified criteria
func (rgm *RelayGroupManager) selectAuthoritativeEvent(events []*event.E) *event.E {
	if len(events) == 0 {
		return nil
	}

	// Sort events by timestamp (newest first), then by hash (smallest first)
	sort.Slice(events, func(i, j int) bool {
		// First compare timestamps (newest first)
		if events[i].CreatedAt != events[j].CreatedAt {
			return events[i].CreatedAt > events[j].CreatedAt
		}

		// If timestamps are equal, compare hashes (smallest first)
		hashI := sha256.Sum256([]byte(events[i].ID))
		hashJ := sha256.Sum256([]byte(events[j].ID))
		return strings.Compare(hex.EncodeToString(hashI[:]), hex.EncodeToString(hashJ[:])) < 0
	})

	return events[0]
}

// IsAuthorizedPublisher checks if a pubkey is authorized to publish relay group configs
func (rgm *RelayGroupManager) IsAuthorizedPublisher(pubkey []byte) bool {
	for _, authPK := range rgm.authorizedPubkeys {
		if string(authPK) == string(pubkey) {
			return true
		}
	}
	return false
}

// ValidateRelayGroupEvent validates a relay group configuration event
func (rgm *RelayGroupManager) ValidateRelayGroupEvent(ev *event.E) error {
	// Check if it's the right kind
	if ev.Kind != kind.RelayGroupConfig.K {
		return nil // Not our concern
	}

	// Check if publisher is authorized
	if !rgm.IsAuthorizedPublisher(ev.Pubkey) {
		return nil // Not our concern, but won't be considered authoritative
	}

	// Try to parse the content
	var config RelayGroupConfig
	if err := json.Unmarshal([]byte(ev.Content), &config); err != nil {
		return err
	}

	// Basic validation - at least one relay should be specified
	if len(config.Relays) == 0 {
		return nil // Empty config is allowed, just won't be selected
	}

	return nil
}

// HandleRelayGroupEvent processes a relay group configuration event and updates peer lists
func (rgm *RelayGroupManager) HandleRelayGroupEvent(ev *event.E, syncManager *Manager) {
	if ev.Kind != kind.RelayGroupConfig.K {
		return
	}

	// Check if this event is the new authoritative configuration
	authConfig, err := rgm.FindAuthoritativeConfig(context.Background())
	if err != nil {
		log.E.F("failed to find authoritative config: %v", err)
		return
	}

	if authConfig != nil {
		// Update the sync manager's peer list
		syncManager.UpdatePeers(authConfig.Relays)
	}
}
