//go:build !(js && wasm)

package bbolt

import (
	"context"
	"errors"
	"time"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
)

// This file contains stub implementations for interface methods that will be
// implemented fully in later phases. It allows the code to compile while
// we implement the core functionality.

var errNotImplemented = errors.New("bbolt: not implemented yet")

// QueryEvents queries events matching a filter.
func (b *B) QueryEvents(c context.Context, f *filter.F) (evs event.S, err error) {
	// Get serials matching filter
	var serials types.Uint40s
	if serials, err = b.GetSerialsFromFilter(f); err != nil {
		return
	}

	// Fetch events by serials
	evs = make(event.S, 0, len(serials))
	for _, ser := range serials {
		ev, e := b.FetchEventBySerial(ser)
		if e == nil && ev != nil {
			evs = append(evs, ev)
		}
	}
	return
}

// QueryAllVersions queries all versions of events matching a filter.
func (b *B) QueryAllVersions(c context.Context, f *filter.F) (evs event.S, err error) {
	return b.QueryEvents(c, f)
}

// QueryEventsWithOptions queries events with additional options.
func (b *B) QueryEventsWithOptions(c context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool) (evs event.S, err error) {
	return b.QueryEvents(c, f)
}

// QueryDeleteEventsByTargetId queries delete events targeting a specific event.
func (b *B) QueryDeleteEventsByTargetId(c context.Context, targetEventId []byte) (evs event.S, err error) {
	return nil, errNotImplemented
}

// QueryForSerials queries and returns only the serials.
func (b *B) QueryForSerials(c context.Context, f *filter.F) (serials types.Uint40s, err error) {
	return b.GetSerialsFromFilter(f)
}

// QueryForIds queries and returns event ID/pubkey/timestamp tuples.
func (b *B) QueryForIds(c context.Context, f *filter.F) (idPkTs []*store.IdPkTs, err error) {
	var serials types.Uint40s
	if serials, err = b.GetSerialsFromFilter(f); err != nil {
		return
	}
	return b.GetFullIdPubkeyBySerials(serials)
}

// DeleteEvent deletes an event by ID.
func (b *B) DeleteEvent(c context.Context, eid []byte) error {
	return errNotImplemented
}

// DeleteEventBySerial deletes an event by serial.
func (b *B) DeleteEventBySerial(c context.Context, ser *types.Uint40, ev *event.E) error {
	return errNotImplemented
}

// DeleteExpired deletes expired events.
func (b *B) DeleteExpired() {
	// TODO: Implement
}

// ProcessDelete processes a deletion event.
func (b *B) ProcessDelete(ev *event.E, admins [][]byte) error {
	return errNotImplemented
}

// CheckForDeleted checks if an event has been deleted.
func (b *B) CheckForDeleted(ev *event.E, admins [][]byte) error {
	return nil // Not deleted by default
}

// GetSubscription gets a user's subscription.
func (b *B) GetSubscription(pubkey []byte) (*database.Subscription, error) {
	return nil, errNotImplemented
}

// IsSubscriptionActive checks if a subscription is active.
func (b *B) IsSubscriptionActive(pubkey []byte) (bool, error) {
	return false, errNotImplemented
}

// ExtendSubscription extends a subscription.
func (b *B) ExtendSubscription(pubkey []byte, days int) error {
	return errNotImplemented
}

// RecordPayment records a payment.
func (b *B) RecordPayment(pubkey []byte, amount int64, invoice, preimage string) error {
	return errNotImplemented
}

// GetPaymentHistory gets payment history.
func (b *B) GetPaymentHistory(pubkey []byte) ([]database.Payment, error) {
	return nil, errNotImplemented
}

// ExtendBlossomSubscription extends a Blossom subscription.
func (b *B) ExtendBlossomSubscription(pubkey []byte, tier string, storageMB int64, daysExtended int) error {
	return errNotImplemented
}

// GetBlossomStorageQuota gets Blossom storage quota.
func (b *B) GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error) {
	return 0, errNotImplemented
}

// IsFirstTimeUser checks if this is a first-time user.
func (b *B) IsFirstTimeUser(pubkey []byte) (bool, error) {
	return true, nil
}

// AddNIP43Member adds a NIP-43 member.
func (b *B) AddNIP43Member(pubkey []byte, inviteCode string) error {
	return errNotImplemented
}

// RemoveNIP43Member removes a NIP-43 member.
func (b *B) RemoveNIP43Member(pubkey []byte) error {
	return errNotImplemented
}

// IsNIP43Member checks if pubkey is a NIP-43 member.
func (b *B) IsNIP43Member(pubkey []byte) (isMember bool, err error) {
	return false, errNotImplemented
}

// GetNIP43Membership gets NIP-43 membership details.
func (b *B) GetNIP43Membership(pubkey []byte) (*database.NIP43Membership, error) {
	return nil, errNotImplemented
}

// GetAllNIP43Members gets all NIP-43 members.
func (b *B) GetAllNIP43Members() ([][]byte, error) {
	return nil, errNotImplemented
}

// StoreInviteCode stores an invite code.
func (b *B) StoreInviteCode(code string, expiresAt time.Time) error {
	return errNotImplemented
}

// ValidateInviteCode validates an invite code.
func (b *B) ValidateInviteCode(code string) (valid bool, err error) {
	return false, errNotImplemented
}

// DeleteInviteCode deletes an invite code.
func (b *B) DeleteInviteCode(code string) error {
	return errNotImplemented
}

// PublishNIP43MembershipEvent publishes a NIP-43 membership event.
func (b *B) PublishNIP43MembershipEvent(kind int, pubkey []byte) error {
	return errNotImplemented
}

// RunMigrations runs database migrations.
func (b *B) RunMigrations() {
	// TODO: Implement if needed
}

// GetCachedJSON gets cached JSON for a filter (stub - no caching in bbolt).
func (b *B) GetCachedJSON(f *filter.F) ([][]byte, bool) {
	return nil, false
}

// CacheMarshaledJSON caches JSON for a filter (stub - no caching in bbolt).
func (b *B) CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte) {
	// No-op: BBolt doesn't use query cache to save RAM
}

// GetCachedEvents gets cached events for a filter (stub - no caching in bbolt).
func (b *B) GetCachedEvents(f *filter.F) (event.S, bool) {
	return nil, false
}

// CacheEvents caches events for a filter (stub - no caching in bbolt).
func (b *B) CacheEvents(f *filter.F, events event.S) {
	// No-op: BBolt doesn't use query cache to save RAM
}

// InvalidateQueryCache invalidates the query cache (stub - no caching in bbolt).
func (b *B) InvalidateQueryCache() {
	// No-op
}

// RecordEventAccess records an event access.
func (b *B) RecordEventAccess(serial uint64, connectionID string) error {
	return nil // TODO: Implement if needed
}

// GetEventAccessInfo gets event access information.
func (b *B) GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error) {
	return 0, 0, errNotImplemented
}

// GetLeastAccessedEvents gets least accessed events.
func (b *B) GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error) {
	return nil, errNotImplemented
}

// EventIdsBySerial gets event IDs by serial range.
func (b *B) EventIdsBySerial(start uint64, count int) (evs []uint64, err error) {
	return nil, errNotImplemented
}
