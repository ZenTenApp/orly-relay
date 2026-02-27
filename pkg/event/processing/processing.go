// Package processing provides event processing services for the ORLY relay.
// It handles event persistence, delivery to subscribers, and post-save hooks.
package processing

import (
	"context"
	"strings"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"

	"next.orly.dev/pkg/domain/events"
)

// Result contains the outcome of event processing.
type Result struct {
	Saved     bool
	Duplicate bool
	Blocked   bool
	BlockMsg  string
	Error     error
}

// OK returns a successful processing result.
func OK() Result {
	return Result{Saved: true}
}

// Blocked returns a blocked processing result.
func Blocked(msg string) Result {
	return Result{Blocked: true, BlockMsg: msg}
}

// Failed returns an error processing result.
func Failed(err error) Result {
	return Result{Error: err}
}

// Database abstracts database operations for event processing.
type Database interface {
	// SaveEvent saves an event to the database.
	SaveEvent(ctx context.Context, ev *event.E) (exists bool, err error)
	// CheckForDeleted checks if an event has been deleted.
	CheckForDeleted(ev *event.E, adminOwners [][]byte) error
}

// Publisher abstracts event delivery to subscribers.
type Publisher interface {
	// Deliver sends an event to all matching subscribers.
	Deliver(ev *event.E)
}

// RateLimiter abstracts rate limiting for write operations.
type RateLimiter interface {
	// IsEnabled returns whether rate limiting is enabled.
	IsEnabled() bool
	// Wait blocks until the rate limit allows the operation.
	Wait(ctx context.Context, opType int) error
}

// SyncManager abstracts sync manager for serial updates.
type SyncManager interface {
	// UpdateSerial updates the serial number after saving an event.
	UpdateSerial()
}

// ACLRegistry abstracts ACL registry for reconfiguration.
type ACLRegistry interface {
	// Configure reconfigures the ACL system.
	Configure(cfg ...any) error
	// Active returns the active ACL mode.
	Active() string
}

// RelayGroupManager handles relay group configuration events.
type RelayGroupManager interface {
	// ValidateRelayGroupEvent validates a relay group config event.
	ValidateRelayGroupEvent(ev *event.E) error
	// HandleRelayGroupEvent processes a relay group event.
	HandleRelayGroupEvent(ev *event.E, syncMgr any)
}

// ClusterManager handles cluster membership events.
type ClusterManager interface {
	// HandleMembershipEvent processes a cluster membership event.
	HandleMembershipEvent(ev *event.E) error
}

// DomainEventDispatcher abstracts domain event publishing.
type DomainEventDispatcher interface {
	// PublishAsync queues an event for asynchronous processing.
	PublishAsync(event events.DomainEvent) bool
}

// Config holds configuration for the processing service.
type Config struct {
	Admins       [][]byte
	Owners       [][]byte
	WriteTimeout time.Duration
}

// DefaultConfig returns the default processing configuration.
func DefaultConfig() *Config {
	return &Config{
		WriteTimeout: 30 * time.Second,
	}
}

// Service implements event processing.
type Service struct {
	cfg             *Config
	db              Database
	publisher       Publisher
	rateLimiter     RateLimiter
	syncManager     SyncManager
	aclRegistry     ACLRegistry
	relayGroupMgr   RelayGroupManager
	clusterManager  ClusterManager
	eventDispatcher DomainEventDispatcher
}

// New creates a new processing service.
func New(cfg *Config, db Database, publisher Publisher) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{
		cfg:       cfg,
		db:        db,
		publisher: publisher,
	}
}

// SetRateLimiter sets the rate limiter.
func (s *Service) SetRateLimiter(rl RateLimiter) {
	s.rateLimiter = rl
}

// SetSyncManager sets the sync manager.
func (s *Service) SetSyncManager(sm SyncManager) {
	s.syncManager = sm
}

// SetACLRegistry sets the ACL registry.
func (s *Service) SetACLRegistry(acl ACLRegistry) {
	s.aclRegistry = acl
}

// SetRelayGroupManager sets the relay group manager.
func (s *Service) SetRelayGroupManager(rgm RelayGroupManager) {
	s.relayGroupMgr = rgm
}

// SetClusterManager sets the cluster manager.
func (s *Service) SetClusterManager(cm ClusterManager) {
	s.clusterManager = cm
}

// SetEventDispatcher sets the domain event dispatcher.
func (s *Service) SetEventDispatcher(d DomainEventDispatcher) {
	s.eventDispatcher = d
}

// Process saves an event and triggers delivery.
func (s *Service) Process(ctx context.Context, ev *event.E) Result {
	// Check if event was previously deleted (skip for "none" ACL mode and delete events)
	// Delete events (kind 5) shouldn't be blocked by existing deletes
	if ev.Kind != kind.EventDeletion.K && s.aclRegistry != nil && s.aclRegistry.Active() != "none" {
		adminOwners := append(s.cfg.Admins, s.cfg.Owners...)
		if err := s.db.CheckForDeleted(ev, adminOwners); err != nil {
			if strings.HasPrefix(err.Error(), "blocked:") {
				errStr := err.Error()[len("blocked: "):]
				return Blocked(errStr)
			}
		}
	}

	// Save the event
	result := s.saveEvent(ctx, ev)
	if !result.Saved {
		return result
	}

	// Run post-save hooks
	s.runPostSaveHooks(ev)

	// Deliver the event to subscribers
	s.deliver(ev)

	return OK()
}

// saveEvent handles rate limiting and database persistence.
func (s *Service) saveEvent(ctx context.Context, ev *event.E) Result {
	// Create timeout context
	saveCtx, cancel := context.WithTimeout(ctx, s.cfg.WriteTimeout)
	defer cancel()

	// Apply rate limiting (skip for NIP-46 bunker events which need realtime priority)
	const kindNIP46 = 24133
	if s.rateLimiter != nil && s.rateLimiter.IsEnabled() && ev.Kind != uint16(kindNIP46) {
		const writeOpType = 1 // ratelimit.Write
		s.rateLimiter.Wait(saveCtx, writeOpType)
	}

	// Save to database
	_, err := s.db.SaveEvent(saveCtx, ev)
	if err != nil {
		if strings.HasPrefix(err.Error(), "blocked:") {
			errStr := err.Error()[len("blocked: "):]
			return Blocked(errStr)
		}
		return Failed(err)
	}

	return OK()
}

// deliver sends event to subscribers.
func (s *Service) deliver(ev *event.E) {
	cloned := ev.Clone()
	go s.publisher.Deliver(cloned)
}

// runPostSaveHooks handles side effects after event persistence.
func (s *Service) runPostSaveHooks(ev *event.E) {
	isAdmin := s.isAdminPubkey(ev.Pubkey)
	isOwner := s.isOwnerPubkey(ev.Pubkey)

	// Dispatch domain event for saved event
	if s.eventDispatcher != nil {
		s.eventDispatcher.PublishAsync(events.NewEventSaved(ev, 0, isAdmin, isOwner))
	}

	// Handle relay group configuration events
	if s.relayGroupMgr != nil {
		if err := s.relayGroupMgr.ValidateRelayGroupEvent(ev); err == nil {
			if s.syncManager != nil {
				s.relayGroupMgr.HandleRelayGroupEvent(ev, s.syncManager)
			}
		}
	}

	// Handle cluster membership events (Kind 39108)
	if ev.Kind == 39108 && s.clusterManager != nil {
		s.clusterManager.HandleMembershipEvent(ev)
	}

	// Update serial for distributed synchronization
	if s.syncManager != nil {
		s.syncManager.UpdateSerial()
	}

	// ACL reconfiguration for admin events
	if isAdmin || isOwner {
		if ev.Kind == kind.FollowList.K || ev.Kind == kind.RelayListMetadata.K {
			if s.aclRegistry != nil {
				go s.aclRegistry.Configure()
			}
		}
	}
}

// isAdminPubkey checks if pubkey is an admin.
func (s *Service) isAdminPubkey(pubkey []byte) bool {
	for _, admin := range s.cfg.Admins {
		if fastEqual(admin, pubkey) {
			return true
		}
	}
	return false
}

// isOwnerPubkey checks if pubkey is an owner.
func (s *Service) isOwnerPubkey(pubkey []byte) bool {
	for _, owner := range s.cfg.Owners {
		if fastEqual(owner, pubkey) {
			return true
		}
	}
	return false
}

// fastEqual compares two byte slices for equality.
func fastEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
