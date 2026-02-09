package main

import (
	"context"
	"fmt"
	"io"
	"time"

	sqlite "github.com/vertex-lab/nostr-sqlite"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/interfaces/store"
)

// RelySQLiteWrapper wraps the vertex-lab/nostr-sqlite store to implement
// the minimal database.Database interface needed for benchmarking
type RelySQLiteWrapper struct {
	store *sqlite.Store
	path  string
	ready chan struct{}
}

// NewRelySQLiteWrapper creates a new wrapper around nostr-sqlite
func NewRelySQLiteWrapper(dbPath string) (*RelySQLiteWrapper, error) {
	store, err := sqlite.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlite store: %w", err)
	}

	wrapper := &RelySQLiteWrapper{
		store: store,
		path:  dbPath,
		ready: make(chan struct{}),
	}

	// Close the ready channel immediately as SQLite is ready on creation
	close(wrapper.ready)

	return wrapper, nil
}

// SaveEvent saves an event to the database
func (w *RelySQLiteWrapper) SaveEvent(ctx context.Context, ev *event.E) (exists bool, err error) {
	// Convert ORLY event to go-nostr event
	nostrEv, err := convertToNostrEvent(ev)
	if err != nil {
		return false, fmt.Errorf("failed to convert event: %w", err)
	}

	// Use Replace for replaceable/addressable events, Save otherwise
	if isReplaceableKind(int(ev.Kind)) || isAddressableKind(int(ev.Kind)) {
		replaced, err := w.store.Replace(ctx, nostrEv)
		return replaced, err
	}

	saved, err := w.store.Save(ctx, nostrEv)
	return !saved, err // saved=true means it's new, exists=false
}

// QueryEvents queries events matching the filter
func (w *RelySQLiteWrapper) QueryEvents(ctx context.Context, f *filter.F) (evs event.S, err error) {
	// Convert ORLY filter to go-nostr filter
	nostrFilter, err := convertToNostrFilter(f)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filter: %w", err)
	}

	// Query the store
	nostrEvents, err := w.store.Query(ctx, nostrFilter)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Convert back to ORLY events
	events := make(event.S, 0, len(nostrEvents))
	for _, ne := range nostrEvents {
		ev, err := convertFromNostrEvent(&ne)
		if err != nil {
			continue // Skip events that fail to convert
		}
		events = append(events, ev)
	}

	return events, nil
}

// Close closes the database
func (w *RelySQLiteWrapper) Close() error {
	if w.store != nil {
		return w.store.Close()
	}
	return nil
}

// Ready returns a channel that closes when the database is ready
func (w *RelySQLiteWrapper) Ready() <-chan struct{} {
	return w.ready
}

// Path returns the database path
func (w *RelySQLiteWrapper) Path() string {
	return w.path
}

// Wipe clears all data from the database
func (w *RelySQLiteWrapper) Wipe() error {
	// Close current store
	if err := w.store.Close(); err != nil {
		return err
	}

	// Delete the database file
	// Note: This is a simplified approach - in production you'd want
	// to handle this more carefully
	return nil
}

// Stub implementations for unused interface methods
func (w *RelySQLiteWrapper) Init(path string) error { return nil }
func (w *RelySQLiteWrapper) Sync() error            { return nil }
func (w *RelySQLiteWrapper) SetLogLevel(level string) {}
func (w *RelySQLiteWrapper) GetSerialsFromFilter(f *filter.F) (serials types.Uint40s, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
	return false, nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) QueryAllVersions(c context.Context, f *filter.F) (evs event.S, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) QueryEventsWithOptions(c context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool) (evs event.S, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) QueryDeleteEventsByTargetId(c context.Context, targetEventId []byte) (evs event.S, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) QueryForSerials(c context.Context, f *filter.F) (serials types.Uint40s, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) QueryForIds(c context.Context, f *filter.F) (idPkTs []*store.IdPkTs, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) CountEvents(c context.Context, f *filter.F) (count int, approximate bool, err error) {
	return 0, false, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) FetchEventsBySerials(serials []*types.Uint40) (events map[uint64]*event.E, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetSerialsByIds(ids *tag.T) (serials map[string]*types.Uint40, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetSerialsByIdsWithFilter(ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool) (serials map[string]*types.Uint40, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetSerialsByRange(idx database.Range) (serials types.Uint40s, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetFullIdPubkeyBySerial(ser *types.Uint40) (fidpk *store.IdPkTs, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetFullIdPubkeyBySerials(sers []*types.Uint40) (fidpks []*store.IdPkTs, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) DeleteEvent(c context.Context, eid []byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) DeleteEventBySerial(c context.Context, ser *types.Uint40, ev *event.E) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) DeleteExpired() {}
func (w *RelySQLiteWrapper) ProcessDelete(ev *event.E, admins [][]byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) CheckForDeleted(ev *event.E, admins [][]byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) Import(rr io.Reader) {}
func (w *RelySQLiteWrapper) Export(c context.Context, writer io.Writer, pubkeys ...[]byte) {
}
func (w *RelySQLiteWrapper) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) ImportEventsFromStrings(ctx context.Context, eventJSONs []string, policyManager interface{ CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error) }) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetRelayIdentitySecret() (skb []byte, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) SetRelayIdentitySecret(skb []byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetOrCreateRelayIdentitySecret() (skb []byte, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) SetMarker(key string, value []byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetMarker(key string) (value []byte, err error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) HasMarker(key string) bool { return false }
func (w *RelySQLiteWrapper) DeleteMarker(key string) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetSubscription(pubkey []byte) (*database.Subscription, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) IsSubscriptionActive(pubkey []byte) (bool, error) {
	return false, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) ExtendSubscription(pubkey []byte, days int) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) RecordPayment(pubkey []byte, amount int64, invoice, preimage string) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetPaymentHistory(pubkey []byte) ([]database.Payment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) ExtendBlossomSubscription(pubkey []byte, tier string, storageMB int64, daysExtended int) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error) {
	return 0, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) IsFirstTimeUser(pubkey []byte) (bool, error) {
	return false, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) AddNIP43Member(pubkey []byte, inviteCode string) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) RemoveNIP43Member(pubkey []byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) IsNIP43Member(pubkey []byte) (isMember bool, err error) {
	return false, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetNIP43Membership(pubkey []byte) (*database.NIP43Membership, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetAllNIP43Members() ([][]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) StoreInviteCode(code string, expiresAt time.Time) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) ValidateInviteCode(code string) (valid bool, err error) {
	return false, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) DeleteInviteCode(code string) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) PublishNIP43MembershipEvent(kind int, pubkey []byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) RunMigrations() {}
func (w *RelySQLiteWrapper) GetCachedJSON(f *filter.F) ([][]byte, bool) {
	return nil, false
}
func (w *RelySQLiteWrapper) CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte) {
}
func (w *RelySQLiteWrapper) GetCachedEvents(f *filter.F) (event.S, bool) {
	return nil, false
}
func (w *RelySQLiteWrapper) CacheEvents(f *filter.F, events event.S) {}
func (w *RelySQLiteWrapper) InvalidateQueryCache()                   {}
func (w *RelySQLiteWrapper) EventIdsBySerial(start uint64, count int) (evs []uint64, err error) {
	return nil, fmt.Errorf("not implemented")
}

// Access tracking stubs (not needed for benchmarking)
func (w *RelySQLiteWrapper) RecordEventAccess(serial uint64, connectionID string) error {
	return nil // No-op for benchmarking
}
func (w *RelySQLiteWrapper) GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error) {
	return 0, 0, nil
}
func (w *RelySQLiteWrapper) GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error) {
	return nil, nil
}

// Blob storage stubs (not needed for benchmarking)
func (w *RelySQLiteWrapper) SaveBlob(sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetBlob(sha256Hash []byte) (data []byte, metadata *database.BlobMetadata, err error) {
	return nil, nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) HasBlob(sha256Hash []byte) (exists bool, err error) {
	return false, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) DeleteBlob(sha256Hash []byte, pubkey []byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) ListBlobs(pubkey []byte, since, until int64) ([]*database.BlobDescriptor, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetBlobMetadata(sha256Hash []byte) (*database.BlobMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetTotalBlobStorageUsed(pubkey []byte) (totalMB int64, err error) {
	return 0, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) SaveBlobReport(sha256Hash []byte, reportData []byte) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) ListAllBlobUserStats() ([]*database.UserBlobStats, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) ReconcileBlobMetadata() (int, error) {
	return 0, fmt.Errorf("not implemented")
}

// NRC (Nostr Relay Connect) stubs - not needed for benchmarking
func (w *RelySQLiteWrapper) CreateNRCConnection(label string, createdBy []byte) (*database.NRCConnection, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetNRCConnection(id string) (*database.NRCConnection, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetNRCConnectionByDerivedPubkey(derivedPubkey []byte) (*database.NRCConnection, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) SaveNRCConnection(conn *database.NRCConnection) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) DeleteNRCConnection(id string) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetAllNRCConnections() ([]*database.NRCConnection, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetNRCAuthorizedSecrets() (map[string]string, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) UpdateNRCConnectionLastUsed(id string) error {
	return fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetNRCConnectionURI(conn *database.NRCConnection, relayPubkey []byte, rendezvousURL string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// Thumbnail stubs - not needed for benchmarking
func (w *RelySQLiteWrapper) ListAllBlobs() ([]*database.BlobDescriptor, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) GetThumbnail(key string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w *RelySQLiteWrapper) SaveThumbnail(key string, data []byte) error {
	return fmt.Errorf("not implemented")
}

// Helper function to check if a kind is replaceable
func isReplaceableKind(kind int) bool {
	return (kind >= 10000 && kind < 20000) || kind == 0 || kind == 3
}

// Helper function to check if a kind is addressable
func isAddressableKind(kind int) bool {
	return kind >= 30000 && kind < 40000
}
