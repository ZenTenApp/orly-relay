// Package relaygroup provides relay group configuration management
package relaygroup

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/minio/sha256-simd"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
)

// PeerUpdater is an interface for updating peer lists
type PeerUpdater interface {
	UpdatePeers(peers []string)
}

// Config represents a relay group configuration
type Config struct {
	Relays []string `json:"relays"`
}

// Manager handles relay group configuration
type Manager struct {
	db                *database.D
	authorizedPubkeys [][]byte
}

// ManagerConfig holds configuration for the relay group manager
type ManagerConfig struct {
	AdminNpubs []string
}

// NewManager creates a new relay group manager
func NewManager(db *database.D, cfg *ManagerConfig) *Manager {
	var pubkeys [][]byte
	if cfg != nil {
		for _, npub := range cfg.AdminNpubs {
			if pk, err := bech32encoding.NpubOrHexToPublicKeyBinary(npub); err == nil {
				pubkeys = append(pubkeys, pk)
			}
		}
	}

	return &Manager{
		db:                db,
		authorizedPubkeys: pubkeys,
	}
}

// FindAuthoritativeConfig finds the authoritative relay group configuration
// by selecting the latest event by timestamp, with hash tie-breaking
func (rgm *Manager) FindAuthoritativeConfig(ctx context.Context) (*Config, error) {
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
	var config Config
	if err := json.Unmarshal([]byte(authEvent.Content), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// FindAuthoritativeRelays returns just the relay URLs from the authoritative config
func (rgm *Manager) FindAuthoritativeRelays(ctx context.Context) ([]string, error) {
	config, err := rgm.FindAuthoritativeConfig(ctx)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, nil
	}
	return config.Relays, nil
}

// selectAuthoritativeEvent selects the authoritative event using the specified criteria
func (rgm *Manager) selectAuthoritativeEvent(events []*event.E) *event.E {
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
func (rgm *Manager) IsAuthorizedPublisher(pubkey []byte) bool {
	for _, authPK := range rgm.authorizedPubkeys {
		if string(authPK) == string(pubkey) {
			return true
		}
	}
	return false
}

// GetAuthorizedPubkeys returns all authorized pubkeys
func (rgm *Manager) GetAuthorizedPubkeys() [][]byte {
	result := make([][]byte, len(rgm.authorizedPubkeys))
	for i, pk := range rgm.authorizedPubkeys {
		pkCopy := make([]byte, len(pk))
		copy(pkCopy, pk)
		result[i] = pkCopy
	}
	return result
}

// ValidateRelayGroupEvent validates a relay group configuration event
func (rgm *Manager) ValidateRelayGroupEvent(ev *event.E) error {
	// Check if it's the right kind
	if ev.Kind != kind.RelayGroupConfig.K {
		return nil // Not our concern
	}

	// Check if publisher is authorized
	if !rgm.IsAuthorizedPublisher(ev.Pubkey) {
		return nil // Not our concern, but won't be considered authoritative
	}

	// Try to parse the content
	var config Config
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
func (rgm *Manager) HandleRelayGroupEvent(ev *event.E, peerUpdater PeerUpdater) {
	if ev.Kind != kind.RelayGroupConfig.K {
		return
	}

	// Check if this event is the new authoritative configuration
	authConfig, err := rgm.FindAuthoritativeConfig(context.Background())
	if err != nil {
		log.E.F("failed to find authoritative config: %v", err)
		return
	}

	if authConfig != nil && peerUpdater != nil {
		// Update the sync manager's peer list
		peerUpdater.UpdatePeers(authConfig.Relays)
	}
}
