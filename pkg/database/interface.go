package database

import (
	"context"
	"io"
	"time"

	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/interfaces/store"
)

// Database defines the interface that all database implementations must satisfy.
// This allows switching between different storage backends (badger, neo4j, etc.)
type Database interface {
	// Core lifecycle methods
	Path() string
	Init(path string) error
	Sync() error
	Close() error
	Wipe() error
	SetLogLevel(level string)
	Ready() <-chan struct{} // Returns a channel that closes when database is ready to serve requests

	// Event storage and retrieval
	SaveEvent(c context.Context, ev *event.E) (exists bool, err error)
	GetSerialsFromFilter(f *filter.F) (serials types.Uint40s, err error)
	WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error)

	QueryEvents(c context.Context, f *filter.F) (evs event.S, err error)
	QueryAllVersions(c context.Context, f *filter.F) (evs event.S, err error)
	QueryEventsWithOptions(c context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool) (evs event.S, err error)
	QueryDeleteEventsByTargetId(c context.Context, targetEventId []byte) (evs event.S, err error)
	QueryForSerials(c context.Context, f *filter.F) (serials types.Uint40s, err error)
	QueryForIds(c context.Context, f *filter.F) (idPkTs []*store.IdPkTs, err error)

	CountEvents(c context.Context, f *filter.F) (count int, approximate bool, err error)

	FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error)
	FetchEventsBySerials(serials []*types.Uint40) (events map[uint64]*event.E, err error)

	GetSerialById(id []byte) (ser *types.Uint40, err error)
	GetSerialsByIds(ids *tag.T) (serials map[string]*types.Uint40, err error)
	GetSerialsByIdsWithFilter(ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool) (serials map[string]*types.Uint40, err error)
	GetSerialsByRange(idx Range) (serials types.Uint40s, err error)

	GetFullIdPubkeyBySerial(ser *types.Uint40) (fidpk *store.IdPkTs, err error)
	GetFullIdPubkeyBySerials(sers []*types.Uint40) (fidpks []*store.IdPkTs, err error)

	// Event deletion
	DeleteEvent(c context.Context, eid []byte) error
	DeleteEventBySerial(c context.Context, ser *types.Uint40, ev *event.E) error
	DeleteExpired()
	ProcessDelete(ev *event.E, admins [][]byte) error
	CheckForDeleted(ev *event.E, admins [][]byte) error

	// Import/Export
	Import(rr io.Reader)
	Export(c context.Context, w io.Writer, pubkeys ...[]byte)
	ImportEventsFromReader(ctx context.Context, rr io.Reader) error
	ImportEventsFromStrings(ctx context.Context, eventJSONs []string, policyManager interface {
		CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (allowed bool, reason string, err error)
	}) error

	// Relay identity
	GetRelayIdentitySecret() (skb []byte, err error)
	SetRelayIdentitySecret(skb []byte) error
	GetOrCreateRelayIdentitySecret() (skb []byte, err error)

	// Markers (metadata key-value storage)
	SetMarker(key string, value []byte) error
	GetMarker(key string) (value []byte, err error)
	HasMarker(key string) bool
	DeleteMarker(key string) error

	// Subscriptions (payment-based access control)
	GetSubscription(pubkey []byte) (*Subscription, error)
	IsSubscriptionActive(pubkey []byte) (bool, error)
	ExtendSubscription(pubkey []byte, days int) error
	RecordPayment(pubkey []byte, amount int64, invoice, preimage string) error
	GetPaymentHistory(pubkey []byte) ([]Payment, error)
	ExtendBlossomSubscription(pubkey []byte, tier string, storageMB int64, daysExtended int) error
	GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error)
	IsFirstTimeUser(pubkey []byte) (bool, error)

	// Paid ACL (Lightning payment-gated access)
	SavePaidSubscription(sub *PaidSubscription) error
	GetPaidSubscription(pubkeyHex string) (*PaidSubscription, error)
	DeletePaidSubscription(pubkeyHex string) error
	ListPaidSubscriptions() ([]*PaidSubscription, error)
	ClaimAlias(alias, pubkeyHex string) error
	GetAliasByPubkey(pubkeyHex string) (string, error)
	GetAliasesByPubkey(pubkeyHex string) ([]string, error)
	GetPubkeyByAlias(alias string) (string, error)
	IsAliasTaken(alias string) (bool, error)

	// NIP-43 Invite-based ACL
	AddNIP43Member(pubkey []byte, inviteCode string) error
	RemoveNIP43Member(pubkey []byte) error
	IsNIP43Member(pubkey []byte) (isMember bool, err error)
	GetNIP43Membership(pubkey []byte) (*NIP43Membership, error)
	GetAllNIP43Members() ([][]byte, error)
	StoreInviteCode(code string, expiresAt time.Time) error
	ValidateInviteCode(code string) (valid bool, err error)
	DeleteInviteCode(code string) error
	PublishNIP43MembershipEvent(kind int, pubkey []byte) error

	// Migrations (version tracking for schema updates)
	RunMigrations()

	// Query cache methods
	GetCachedJSON(f *filter.F) ([][]byte, bool)
	CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte)
	GetCachedEvents(f *filter.F) (event.S, bool)
	CacheEvents(f *filter.F, events event.S)
	InvalidateQueryCache()

	// Access tracking for storage management (garbage collection based on access patterns)
	// RecordEventAccess records an access to an event by a connection.
	// The connectionID is used to deduplicate accesses from the same connection.
	RecordEventAccess(serial uint64, connectionID string) error
	// GetEventAccessInfo returns the last access time and access count for an event.
	GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error)
	// GetLeastAccessedEvents returns event serials sorted by coldness (oldest/lowest access).
	// limit: max events to return, minAgeSec: minimum age in seconds since last access.
	GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error)

	// Utility methods
	EventIdsBySerial(start uint64, count int) (evs []uint64, err error)

	// Blob storage (Blossom)
	SaveBlob(sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string) error
	SaveBlobMetadata(sha256Hash []byte, size int64, pubkey []byte, mimeType string, extension string) error
	GetBlob(sha256Hash []byte) (data []byte, metadata *BlobMetadata, err error)
	HasBlob(sha256Hash []byte) (exists bool, err error)
	DeleteBlob(sha256Hash []byte, pubkey []byte) error
	ListBlobs(pubkey []byte, since, until int64) ([]*BlobDescriptor, error)
	ListAllBlobs() ([]*BlobDescriptor, error)
	GetBlobMetadata(sha256Hash []byte) (*BlobMetadata, error)
	GetTotalBlobStorageUsed(pubkey []byte) (totalMB int64, err error)
	SaveBlobReport(sha256Hash []byte, reportData []byte) error
	ListAllBlobUserStats() ([]*UserBlobStats, error)
	ReconcileBlobMetadata() (reconciled int, err error)

	// Thumbnail caching
	GetThumbnail(key string) (data []byte, err error)
	SaveThumbnail(key string, data []byte) error

	// NRC (Nostr Relay Connect) client management
	CreateNRCConnection(label string, createdBy []byte) (*NRCConnection, error)
	GetNRCConnection(id string) (*NRCConnection, error)
	GetNRCConnectionByDerivedPubkey(derivedPubkey []byte) (*NRCConnection, error)
	SaveNRCConnection(conn *NRCConnection) error
	DeleteNRCConnection(id string) error
	GetAllNRCConnections() ([]*NRCConnection, error)
	GetNRCAuthorizedSecrets() (map[string]string, error)
	UpdateNRCConnectionLastUsed(id string) error
	GetNRCConnectionURI(conn *NRCConnection, relayPubkey []byte, rendezvousURL string) (string, error)
}
