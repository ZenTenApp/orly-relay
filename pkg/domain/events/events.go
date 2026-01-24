// Package events provides domain event types and a dispatcher for the ORLY relay.
// Domain events represent significant occurrences in the system that other components
// may want to react to, enabling loose coupling between components.
package events

import (
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
)

// DomainEvent is the base interface for all domain events.
type DomainEvent interface {
	// OccurredAt returns when the event occurred.
	OccurredAt() time.Time
	// EventType returns a string identifier for the event type.
	EventType() string
}

// Base provides common fields for all domain events.
type Base struct {
	occurredAt time.Time
	eventType  string
}

// OccurredAt returns when the event occurred.
func (b Base) OccurredAt() time.Time { return b.occurredAt }

// EventType returns the event type identifier.
func (b Base) EventType() string { return b.eventType }

// newBase creates a new Base with the current time.
func newBase(eventType string) Base {
	return Base{
		occurredAt: time.Now(),
		eventType:  eventType,
	}
}

// =============================================================================
// Event Storage Events
// =============================================================================

// EventSavedType is the event type for EventSaved.
const EventSavedType = "event.saved"

// EventSaved is emitted when a Nostr event is successfully saved to the database.
type EventSaved struct {
	Base
	Event   *event.E // The saved event
	Serial  uint64   // The assigned serial number
	IsAdmin bool     // Whether the author is an admin
	IsOwner bool     // Whether the author is an owner
}

// NewEventSaved creates a new EventSaved domain event.
func NewEventSaved(ev *event.E, serial uint64, isAdmin, isOwner bool) *EventSaved {
	return &EventSaved{
		Base:    newBase(EventSavedType),
		Event:   ev,
		Serial:  serial,
		IsAdmin: isAdmin,
		IsOwner: isOwner,
	}
}

// EventDeletedType is the event type for EventDeleted.
const EventDeletedType = "event.deleted"

// EventDeleted is emitted when a Nostr event is deleted.
type EventDeleted struct {
	Base
	EventID   []byte // The deleted event ID
	DeletedBy []byte // Pubkey that requested deletion
	Serial    uint64 // The serial of the deleted event
}

// NewEventDeleted creates a new EventDeleted domain event.
func NewEventDeleted(eventID, deletedBy []byte, serial uint64) *EventDeleted {
	return &EventDeleted{
		Base:      newBase(EventDeletedType),
		EventID:   eventID,
		DeletedBy: deletedBy,
		Serial:    serial,
	}
}

// =============================================================================
// ACL Events
// =============================================================================

// FollowListUpdatedType is the event type for FollowListUpdated.
const FollowListUpdatedType = "acl.followlist.updated"

// FollowListUpdated is emitted when an admin's follow list changes.
type FollowListUpdated struct {
	Base
	AdminPubkey    []byte   // The admin whose follow list changed
	AddedFollows   [][]byte // Pubkeys that were added
	RemovedFollows [][]byte // Pubkeys that were removed
}

// NewFollowListUpdated creates a new FollowListUpdated domain event.
func NewFollowListUpdated(adminPubkey []byte, added, removed [][]byte) *FollowListUpdated {
	return &FollowListUpdated{
		Base:           newBase(FollowListUpdatedType),
		AdminPubkey:    adminPubkey,
		AddedFollows:   added,
		RemovedFollows: removed,
	}
}

// ACLMembershipChangedType is the event type for ACLMembershipChanged.
const ACLMembershipChangedType = "acl.membership.changed"

// ACLMembershipChanged is emitted when a pubkey's access level changes.
type ACLMembershipChanged struct {
	Base
	Pubkey    []byte // The affected pubkey
	PrevLevel string // Previous access level
	NewLevel  string // New access level
	Reason    string // Reason for the change
}

// NewACLMembershipChanged creates a new ACLMembershipChanged domain event.
func NewACLMembershipChanged(pubkey []byte, prevLevel, newLevel, reason string) *ACLMembershipChanged {
	return &ACLMembershipChanged{
		Base:      newBase(ACLMembershipChangedType),
		Pubkey:    pubkey,
		PrevLevel: prevLevel,
		NewLevel:  newLevel,
		Reason:    reason,
	}
}

// =============================================================================
// Policy Events
// =============================================================================

// PolicyConfigUpdatedType is the event type for PolicyConfigUpdated.
const PolicyConfigUpdatedType = "policy.config.updated"

// PolicyConfigUpdated is emitted when the policy configuration changes.
type PolicyConfigUpdated struct {
	Base
	UpdatedBy []byte                 // Pubkey that made the update
	Changes   map[string]interface{} // Changed configuration keys
}

// NewPolicyConfigUpdated creates a new PolicyConfigUpdated domain event.
func NewPolicyConfigUpdated(updatedBy []byte, changes map[string]interface{}) *PolicyConfigUpdated {
	return &PolicyConfigUpdated{
		Base:      newBase(PolicyConfigUpdatedType),
		UpdatedBy: updatedBy,
		Changes:   changes,
	}
}

// PolicyFollowsUpdatedType is the event type for PolicyFollowsUpdated.
const PolicyFollowsUpdatedType = "policy.follows.updated"

// PolicyFollowsUpdated is emitted when policy follow lists are refreshed.
type PolicyFollowsUpdated struct {
	Base
	AdminPubkey []byte   // The admin whose follows were processed
	FollowCount int      // Number of follows in the updated list
	Follows     [][]byte // The follow pubkeys (may be nil for large lists)
}

// NewPolicyFollowsUpdated creates a new PolicyFollowsUpdated domain event.
func NewPolicyFollowsUpdated(adminPubkey []byte, followCount int, follows [][]byte) *PolicyFollowsUpdated {
	return &PolicyFollowsUpdated{
		Base:        newBase(PolicyFollowsUpdatedType),
		AdminPubkey: adminPubkey,
		FollowCount: followCount,
		Follows:     follows,
	}
}

// =============================================================================
// Sync Events
// =============================================================================

// RelayGroupConfigChangedType is the event type for RelayGroupConfigChanged.
const RelayGroupConfigChangedType = "sync.relaygroup.changed"

// RelayGroupConfigChanged is emitted when relay group configuration changes.
type RelayGroupConfigChanged struct {
	Base
	Event *event.E // The kind 39105 event
}

// NewRelayGroupConfigChanged creates a new RelayGroupConfigChanged domain event.
func NewRelayGroupConfigChanged(ev *event.E) *RelayGroupConfigChanged {
	return &RelayGroupConfigChanged{
		Base:  newBase(RelayGroupConfigChangedType),
		Event: ev,
	}
}

// ClusterMembershipChangedType is the event type for ClusterMembershipChanged.
const ClusterMembershipChangedType = "sync.cluster.membership.changed"

// ClusterMembershipChanged is emitted when cluster membership changes.
type ClusterMembershipChanged struct {
	Base
	Event  *event.E // The kind 39108 event
	Action string   // "join" or "leave"
}

// NewClusterMembershipChanged creates a new ClusterMembershipChanged domain event.
func NewClusterMembershipChanged(ev *event.E, action string) *ClusterMembershipChanged {
	return &ClusterMembershipChanged{
		Base:   newBase(ClusterMembershipChangedType),
		Event:  ev,
		Action: action,
	}
}

// SyncSerialUpdatedType is the event type for SyncSerialUpdated.
const SyncSerialUpdatedType = "sync.serial.updated"

// SyncSerialUpdated is emitted when the sync manager's serial is updated.
type SyncSerialUpdated struct {
	Base
	Serial uint64 // The new serial number
}

// NewSyncSerialUpdated creates a new SyncSerialUpdated domain event.
func NewSyncSerialUpdated(serial uint64) *SyncSerialUpdated {
	return &SyncSerialUpdated{
		Base:   newBase(SyncSerialUpdatedType),
		Serial: serial,
	}
}

// =============================================================================
// Authentication Events
// =============================================================================

// UserAuthenticatedType is the event type for UserAuthenticated.
const UserAuthenticatedType = "auth.user.authenticated"

// UserAuthenticated is emitted when a user successfully authenticates.
type UserAuthenticated struct {
	Base
	Pubkey       []byte // The authenticated pubkey
	AccessLevel  string // The granted access level
	IsFirstTime  bool   // Whether this is a first-time user
	ConnectionID string // The connection ID
}

// NewUserAuthenticated creates a new UserAuthenticated domain event.
func NewUserAuthenticated(pubkey []byte, accessLevel string, isFirstTime bool, connID string) *UserAuthenticated {
	return &UserAuthenticated{
		Base:         newBase(UserAuthenticatedType),
		Pubkey:       pubkey,
		AccessLevel:  accessLevel,
		IsFirstTime:  isFirstTime,
		ConnectionID: connID,
	}
}

// =============================================================================
// Connection Events
// =============================================================================

// ConnectionOpenedType is the event type for ConnectionOpened.
const ConnectionOpenedType = "connection.opened"

// ConnectionOpened is emitted when a new WebSocket connection is established.
type ConnectionOpened struct {
	Base
	ConnectionID string // Unique connection identifier
	RemoteAddr   string // Client IP:port
}

// NewConnectionOpened creates a new ConnectionOpened domain event.
func NewConnectionOpened(connID, remoteAddr string) *ConnectionOpened {
	return &ConnectionOpened{
		Base:         newBase(ConnectionOpenedType),
		ConnectionID: connID,
		RemoteAddr:   remoteAddr,
	}
}

// ConnectionClosedType is the event type for ConnectionClosed.
const ConnectionClosedType = "connection.closed"

// ConnectionClosed is emitted when a WebSocket connection is closed.
type ConnectionClosed struct {
	Base
	ConnectionID    string        // Unique connection identifier
	Duration        time.Duration // How long the connection was open
	EventsReceived  int           // Number of events received
	EventsPublished int           // Number of events published
}

// NewConnectionClosed creates a new ConnectionClosed domain event.
func NewConnectionClosed(connID string, duration time.Duration, received, published int) *ConnectionClosed {
	return &ConnectionClosed{
		Base:            newBase(ConnectionClosedType),
		ConnectionID:    connID,
		Duration:        duration,
		EventsReceived:  received,
		EventsPublished: published,
	}
}

// =============================================================================
// Subscription Events
// =============================================================================

// SubscriptionCreatedType is the event type for SubscriptionCreated.
const SubscriptionCreatedType = "subscription.created"

// SubscriptionCreated is emitted when a new REQ subscription is created.
type SubscriptionCreated struct {
	Base
	SubscriptionID string // The subscription ID from REQ
	ConnectionID   string // The connection this subscription belongs to
	FilterCount    int    // Number of filters in the subscription
}

// NewSubscriptionCreated creates a new SubscriptionCreated domain event.
func NewSubscriptionCreated(subID, connID string, filterCount int) *SubscriptionCreated {
	return &SubscriptionCreated{
		Base:           newBase(SubscriptionCreatedType),
		SubscriptionID: subID,
		ConnectionID:   connID,
		FilterCount:    filterCount,
	}
}

// SubscriptionClosedType is the event type for SubscriptionClosed.
const SubscriptionClosedType = "subscription.closed"

// SubscriptionClosed is emitted when a subscription is closed.
type SubscriptionClosed struct {
	Base
	SubscriptionID string // The subscription ID
	ConnectionID   string // The connection this subscription belonged to
	EventsMatched  int    // Number of events that matched this subscription
}

// NewSubscriptionClosed creates a new SubscriptionClosed domain event.
func NewSubscriptionClosed(subID, connID string, eventsMatched int) *SubscriptionClosed {
	return &SubscriptionClosed{
		Base:           newBase(SubscriptionClosedType),
		SubscriptionID: subID,
		ConnectionID:   connID,
		EventsMatched:  eventsMatched,
	}
}

// =============================================================================
// NIP-43 Events
// =============================================================================

// MemberJoinedType is the event type for MemberJoined.
const MemberJoinedType = "nip43.member.joined"

// MemberJoined is emitted when a new member joins via NIP-43.
type MemberJoined struct {
	Base
	Pubkey     []byte // The new member's pubkey
	InviteCode string // The invite code used (if any)
}

// NewMemberJoined creates a new MemberJoined domain event.
func NewMemberJoined(pubkey []byte, inviteCode string) *MemberJoined {
	return &MemberJoined{
		Base:       newBase(MemberJoinedType),
		Pubkey:     pubkey,
		InviteCode: inviteCode,
	}
}

// MemberLeftType is the event type for MemberLeft.
const MemberLeftType = "nip43.member.left"

// MemberLeft is emitted when a member leaves via NIP-43.
type MemberLeft struct {
	Base
	Pubkey []byte // The departed member's pubkey
}

// NewMemberLeft creates a new MemberLeft domain event.
func NewMemberLeft(pubkey []byte) *MemberLeft {
	return &MemberLeft{
		Base:   newBase(MemberLeftType),
		Pubkey: pubkey,
	}
}

// =============================================================================
// Event Type Registry
// =============================================================================

// AllEventTypes returns all registered event type constants.
func AllEventTypes() []string {
	return []string{
		EventSavedType,
		EventDeletedType,
		FollowListUpdatedType,
		ACLMembershipChangedType,
		PolicyConfigUpdatedType,
		PolicyFollowsUpdatedType,
		RelayGroupConfigChangedType,
		ClusterMembershipChangedType,
		SyncSerialUpdatedType,
		UserAuthenticatedType,
		ConnectionOpenedType,
		ConnectionClosedType,
		SubscriptionCreatedType,
		SubscriptionClosedType,
		MemberJoinedType,
		MemberLeftType,
	}
}
