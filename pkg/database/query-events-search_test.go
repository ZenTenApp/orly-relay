package database

import (
	"context"
	"os"
	"testing"
	"time"

	"lol.mleku.dev/chk"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
)

// helper to create a fresh DB
func newTestDB(t *testing.T) (*D, context.Context, context.CancelFunc, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "search-db-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	db, err := New(ctx, cancel, tempDir, "error")
	if err != nil {
		cancel()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to init DB: %v", err)
	}
	return db, ctx, cancel, tempDir
}

// TestQueryEventsBySearchTerms creates a small set of events with content and tags,
// saves them, then queries using filter.Search to ensure the word index works.
func TestQueryEventsBySearchTerms(t *testing.T) {
	db, ctx, cancel, tempDir := newTestDB(t)
	defer func() {
		// cancel context first to stop background routines cleanly
		cancel()
		db.Close()
		os.RemoveAll(tempDir)
	}()

	// signer for all events
	sign := p8k.MustNew()
	if err := sign.Generate(); chk.E(err) {
		t.Fatalf("signer generate: %v", err)
	}

	now := timestamp.Now().V

	// Events to cover tokenizer rules:
	// - regular words
	// - URLs ignored
	// - 64-char hex ignored
	// - nostr: URIs ignored
	// - #[n] mentions ignored
	// - tag fields included in search

	// 1. Contains words: "alpha beta", plus URL and hex (ignored)
	ev1 := event.New()
	ev1.Kind = kind.TextNote.K
	ev1.Pubkey = sign.Pub()
	ev1.CreatedAt = now - 5
	ev1.Content = []byte("Alpha beta visit https://example.com deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	ev1.Tags = tag.NewS()
	ev1.Sign(sign)
	if _, err := db.SaveEvent(ctx, ev1); err != nil {
		t.Fatalf("save ev1: %v", err)
	}

	// 2. Contains overlap word "beta" and unique "gamma" and nostr: URI ignored
	ev2 := event.New()
	ev2.Kind = kind.TextNote.K
	ev2.Pubkey = sign.Pub()
	ev2.CreatedAt = now - 4
	ev2.Content = []byte("beta and GAMMA with nostr:nevent1qqqqq")
	ev2.Tags = tag.NewS()
	ev2.Sign(sign)
	if _, err := db.SaveEvent(ctx, ev2); err != nil {
		t.Fatalf("save ev2: %v", err)
	}

	// 3. Contains only a URL (should not create word tokens) and mention #[1] (ignored)
	ev3 := event.New()
	ev3.Kind = kind.TextNote.K
	ev3.Pubkey = sign.Pub()
	ev3.CreatedAt = now - 3
	ev3.Content = []byte("see www.example.org #[1]")
	ev3.Tags = tag.NewS()
	ev3.Sign(sign)
	if _, err := db.SaveEvent(ctx, ev3); err != nil {
		t.Fatalf("save ev3: %v", err)
	}

	// 4. No content words, but tag value has searchable words: "delta epsilon"
	ev4 := event.New()
	ev4.Kind = kind.TextNote.K
	ev4.Pubkey = sign.Pub()
	ev4.CreatedAt = now - 2
	ev4.Content = []byte("")
	ev4.Tags = tag.NewS()
	*ev4.Tags = append(*ev4.Tags, tag.NewFromAny("t", "delta epsilon"))
	ev4.Sign(sign)
	if _, err := db.SaveEvent(ctx, ev4); err != nil {
		t.Fatalf("save ev4: %v", err)
	}

	// 5. Another event with both content and tag tokens for ordering checks
	ev5 := event.New()
	ev5.Kind = kind.TextNote.K
	ev5.Pubkey = sign.Pub()
	ev5.CreatedAt = now - 1
	ev5.Content = []byte("alpha DELTA mixed-case and link http://foo.bar")
	ev5.Tags = tag.NewS()
	*ev5.Tags = append(*ev5.Tags, tag.NewFromAny("t", "zeta"))
	ev5.Sign(sign)
	if _, err := db.SaveEvent(ctx, ev5); err != nil {
		t.Fatalf("save ev5: %v", err)
	}

	// Small sleep to ensure created_at ordering is the only factor
	time.Sleep(5 * time.Millisecond)

	// Helper to run a search and return IDs
	run := func(q string) ([]*event.E, error) {
		f := &filter.F{Search: []byte(q)}
		return db.QueryEvents(ctx, f)
	}

	// Single-term search: alpha -> should match ev1 and ev5 ordered by created_at desc (ev5 newer)
	if evs, err := run("alpha"); err != nil {
		t.Fatalf("search alpha: %v", err)
	} else {
		if len(evs) != 2 {
			t.Fatalf("alpha expected 2 results, got %d", len(evs))
		}
		if !(evs[0].CreatedAt >= evs[1].CreatedAt) {
			t.Fatalf("results not ordered by created_at desc")
		}
	}

	// Overlap term beta -> ev1 and ev2
	if evs, err := run("beta"); err != nil {
		t.Fatalf("search beta: %v", err)
	} else if len(evs) != 2 {
		t.Fatalf("beta expected 2 results, got %d", len(evs))
	}

	// Unique term gamma -> only ev2
	if evs, err := run("gamma"); err != nil {
		t.Fatalf("search gamma: %v", err)
	} else if len(evs) != 1 {
		t.Fatalf("gamma expected 1 result, got %d", len(evs))
	}

	// URL terms should be ignored: example -> appears only as URL in ev1/ev3/ev5; tokenizer ignores URLs so expect 0
	if evs, err := run("example"); err != nil {
		t.Fatalf("search example: %v", err)
	} else if len(evs) != 0 {
		t.Fatalf(
			"example expected 0 results (URL tokens ignored), got %d", len(evs),
		)
	}

	// Tag words searchable: delta should match ev4 and ev5 (delta in tag for ev4, in content for ev5)
	if evs, err := run("delta"); err != nil {
		t.Fatalf("search delta: %v", err)
	} else if len(evs) != 2 {
		t.Fatalf("delta expected 2 results, got %d", len(evs))
	}

	// Very short token ignored: single-letter should yield 0
	if evs, err := run("a"); err != nil {
		t.Fatalf("search short token: %v", err)
	} else if len(evs) != 0 {
		t.Fatalf("single-letter expected 0 results, got %d", len(evs))
	}

	// 64-char hex should be ignored
	hex64 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if evs, err := run(hex64); err != nil {
		t.Fatalf("search hex64: %v", err)
	} else if len(evs) != 0 {
		t.Fatalf("hex64 expected 0 results, got %d", len(evs))
	}

	// nostr: scheme ignored
	if evs, err := run("nostr:nevent1qqqqq"); err != nil {
		t.Fatalf("search nostr: %v", err)
	} else if len(evs) != 0 {
		t.Fatalf("nostr: expected 0 results, got %d", len(evs))
	}
}
