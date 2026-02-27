// Package sync provides backward compatibility facade for sync services
// New code should import the specific subpackages directly:
//   - next.orly.dev/pkg/sync/distributed
//   - next.orly.dev/pkg/sync/cluster
//   - next.orly.dev/pkg/sync/relaygroup
//   - next.orly.dev/pkg/sync/negentropy
package sync

import (
	"context"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/sync/cluster"
	"next.orly.dev/pkg/sync/common"
	"next.orly.dev/pkg/sync/distributed"
	"next.orly.dev/pkg/sync/relaygroup"
)

// Re-export types for backward compatibility

// Manager is the distributed sync manager
type Manager = distributed.Manager

// ClusterManager is the cluster replication manager
type ClusterManager = cluster.Manager

// RelayGroupManager is the relay group configuration manager
type RelayGroupManager = relaygroup.Manager

// RelayGroupConfig is the relay group configuration
type RelayGroupConfig = relaygroup.Config

// NIP11Cache is the NIP-11 relay info cache
type NIP11Cache = common.NIP11Cache

// NewNIP11Cache creates a new NIP-11 cache
func NewNIP11Cache(ttl time.Duration) *NIP11Cache {
	return common.NewNIP11Cache(ttl)
}

// NewManager creates a new distributed sync manager with backward compatible signature
func NewManager(ctx context.Context, db *database.D, nodeID, relayURL string, peers []string, relayGroupMgr *RelayGroupManager, policyManager interface{ CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error) }) *Manager {
	cfg := &distributed.Config{
		NodeID:        nodeID,
		RelayURL:      relayURL,
		Peers:         peers,
		SyncInterval:  5 * time.Second,
		NIP11CacheTTL: 30 * time.Minute,
	}
	return distributed.NewManager(ctx, db, cfg, policyManager)
}

// NewClusterManager creates a new cluster manager with backward compatible signature
func NewClusterManager(ctx context.Context, db *database.D, adminNpubs []string, propagatePrivilegedEvents bool, publisher interface{ Deliver(*event.E) }) *ClusterManager {
	cfg := &cluster.Config{
		AdminNpubs:                adminNpubs,
		PropagatePrivilegedEvents: propagatePrivilegedEvents,
		PollInterval:              5 * time.Second,
		NIP11CacheTTL:             30 * time.Minute,
	}
	return cluster.NewManager(ctx, db, cfg, publisher)
}

// NewRelayGroupManager creates a new relay group manager with backward compatible signature
func NewRelayGroupManager(db *database.D, adminNpubs []string) *RelayGroupManager {
	cfg := &relaygroup.ManagerConfig{
		AdminNpubs: adminNpubs,
	}
	return relaygroup.NewManager(db, cfg)
}
