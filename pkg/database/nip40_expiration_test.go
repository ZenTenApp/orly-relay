package database

import (
	"bytes"
	"strconv"
	"testing"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// mustSignExpiringEvent builds a signed text-note carrying a NIP-40
// expiration tag: ["expiration", "<unix-seconds>"].
func mustSignExpiringEvent(t *testing.T, sign *p8k.Signer, content string, expiration int64) *event.E {
	t.Helper()
	ev := event.New()
	ev.Kind = kind.TextNote.K
	ev.CreatedAt = timestamp.Now().V
	ev.Content = []byte(content)
	ev.Pubkey = sign.Pub()
	ev.Tags = tag.NewS(tag.NewFromAny("expiration", strconv.FormatInt(expiration, 10)))
	if err := ev.Sign(sign); err != nil {
		t.Fatalf("failed to sign expiring event: %v", err)
	}
	return ev
}

// TestNIP40_ExpiredEventHiddenAndDeleted verifies the full NIP-40 path:
// expired events are hidden from query results AND physically removed by the
// background DeleteExpired sweeper.
func TestNIP40_ExpiredEventHiddenAndDeleted(t *testing.T) {
	db, ctx, cleanup := setupFreshTestDB(t)
	defer cleanup()
	defer db.Close()

	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatal(err)
	}

	now := timestamp.Now().V

	// Event whose expiration is already in the past (should be expired).
	expired := mustSignExpiringEvent(t, sign, "expired note", now-100)
	// Event whose expiration is far in the future (should remain valid).
	valid := mustSignExpiringEvent(t, sign, "valid note", now+3600)

	if _, err := db.SaveEvent(ctx, expired); err != nil {
		t.Fatalf("failed to save expired event: %v", err)
	}
	if _, err := db.SaveEvent(ctx, valid); err != nil {
		t.Fatalf("failed to save valid event: %v", err)
	}

	serExpired, err := db.GetSerialById(expired.ID)
	if err != nil {
		t.Fatalf("failed to get serial for expired event: %v", err)
	}

	// 1) Author query: expired event must be hidden, valid event must be returned.
	byAuthor := &filter.F{Authors: tag.NewFromBytesSlice(sign.Pub())}
	evs, err := db.QueryEvents(ctx, byAuthor)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	gotValid, gotExpired := false, false
	for _, ev := range evs {
		if bytes.Equal(ev.ID, valid.ID) {
			gotValid = true
		}
		if bytes.Equal(ev.ID, expired.ID) {
			gotExpired = true
		}
	}
	if !gotValid {
		t.Fatalf("expected valid (non-expired) event to be returned by query")
	}
	if gotExpired {
		t.Fatalf("expected expired event to be HIDDEN from query results")
	}

	// 2) Explicit-ID query: NIP-40 still applies and must also hide it.
	byID := &filter.F{Ids: tag.NewFromBytesSlice(expired.ID)}
	evsByID, err := db.QueryEvents(ctx, byID)
	if err != nil {
		t.Fatalf("QueryEvents(by id) failed: %v", err)
	}
	if len(evsByID) != 0 {
		t.Fatalf("expected expired event to be hidden from explicit-id query, got %d events", len(evsByID))
	}

	// 3) Cleanup path: the sweeper must physically delete the expired event.
	db.DeleteExpired()

	if _, err := db.FetchEventBySerial(serExpired); err == nil {
		t.Fatalf("expected expired event to be physically deleted by DeleteExpired")
	}

	serValid, err := db.GetSerialById(valid.ID)
	if err != nil {
		t.Fatalf("valid event unexpectedly missing: %v", err)
	}
	if fetched, err := db.FetchEventBySerial(serValid); err != nil || fetched == nil {
		t.Fatalf("expected valid event to survive DeleteExpired")
	}
}
