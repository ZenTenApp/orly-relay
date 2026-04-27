//go:build !(js && wasm)

package database

import (
	"context"
	"sort"
	"testing"

	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// kindFilterTestFixture holds all pubkeys and their hex strings for the test graph:
//
//	pkA --kind3--> pkB, pkE   (pkA follows pkB and pkE)
//	pkA --kind10000--> pkC    (pkA mutes pkC)
//	pkA --kind1984--> pkD     (pkA reports pkD)
//	pkF --kind3--> pkB        (pkF also follows pkB)
type kindFilterTestFixture struct {
	db     *D
	ctx    context.Context
	pkAHex string
	pkBHex string
	pkCHex string
	pkDHex string
	pkEHex string
	pkFHex string
}

// resolveSerials converts a slice of pubkey serials to a set of hex strings.
func resolveSerials(t *testing.T, db *D, serials []*types.Uint40) map[string]bool {
	t.Helper()
	m := make(map[string]bool, len(serials))
	for _, s := range serials {
		h, err := db.GetPubkeyHexFromSerial(s)
		if err != nil {
			t.Errorf("GetPubkeyHexFromSerial: %v", err)
			continue
		}
		m[h] = true
	}
	return m
}

func setupKindFilterFixture(t *testing.T) *kindFilterTestFixture {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	db, err := New(ctx, cancel, t.TempDir(), "off")
	if err != nil {
		t.Fatalf("New db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	pkA, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000001")
	pkB, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000002")
	pkC, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000003")
	pkD, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000004")
	pkE, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000005")
	pkF, _ := hex.Dec("0000000000000000000000000000000000000000000000000000000000000006")

	makeEvent := func(idByte byte, author []byte, kind uint16, targets ...[]byte) *event.E {
		id := make([]byte, 32)
		id[0] = idByte
		sig := make([]byte, 64)
		sig[0] = idByte
		ptags := make([]*tag.T, 0, len(targets))
		for _, tgt := range targets {
			ptags = append(ptags, tag.NewFromAny("p", hex.Enc(tgt)))
		}
		return &event.E{
			ID:        id,
			Pubkey:    author,
			CreatedAt: int64(1000000 + int(idByte)),
			Kind:      kind,
			Content:   []byte(""),
			Sig:       sig,
			Tags:      tag.NewS(ptags...),
		}
	}

	for _, ev := range []*event.E{
		makeEvent(0x01, pkA, 3, pkB, pkE),  // pkA kind-3 follows pkB, pkE
		makeEvent(0x02, pkA, 10000, pkC),   // pkA kind-10000 mutes pkC
		makeEvent(0x03, pkA, 1984, pkD),    // pkA kind-1984 reports pkD
		makeEvent(0x04, pkF, 3, pkB),       // pkF kind-3 follows pkB
	} {
		if _, err := db.SaveEvent(ctx, ev); err != nil {
			t.Fatalf("SaveEvent kind=%d: %v", ev.Kind, err)
		}
	}

	return &kindFilterTestFixture{
		db:     db,
		ctx:    ctx,
		pkAHex: hex.Enc(pkA),
		pkBHex: hex.Enc(pkB),
		pkCHex: hex.Enc(pkC),
		pkDHex: hex.Enc(pkD),
		pkEHex: hex.Enc(pkE),
		pkFHex: hex.Enc(pkF),
	}
}

// TestGetFollowsByKindViaPPG verifies the forward kind-filtered PPG scan.
func TestGetFollowsByKindViaPPG(t *testing.T) {
	f := setupKindFilterFixture(t)
	db := f.db

	pkASerial, err := db.PubkeyHexToSerial(f.pkAHex)
	if err != nil {
		t.Fatalf("PubkeyHexToSerial(A): %v", err)
	}

	t.Run("GetFollowsViaPPG returns all kinds", func(t *testing.T) {
		serials, err := db.GetFollowsViaPPG(pkASerial)
		if err != nil {
			t.Fatalf("GetFollowsViaPPG: %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				t.Errorf("GetPubkeyHexFromSerial: %v", err)
				continue
			}
			got[h] = true
		}
		want := map[string]bool{
			f.pkBHex: true, // kind-3
			f.pkEHex: true, // kind-3
			f.pkCHex: true, // kind-10000
			f.pkDHex: true, // kind-1984
		}
		if len(got) != len(want) {
			t.Errorf("GetFollowsViaPPG: got %d results, want %d; got=%v want=%v", len(got), len(want), got, want)
			return
		}
		for pk := range want {
			if !got[pk] {
				t.Errorf("GetFollowsViaPPG: missing expected pubkey %s", pk[:8])
			}
		}
	})

	t.Run("GetFollowsByKindViaPPG kind-3 returns only follows", func(t *testing.T) {
		serials, err := db.GetFollowsByKindViaPPG(pkASerial, 3)
		if err != nil {
			t.Fatalf("GetFollowsByKindViaPPG(3): %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				t.Errorf("GetPubkeyHexFromSerial: %v", err)
				continue
			}
			got[h] = true
		}
		want := map[string]bool{f.pkBHex: true, f.pkEHex: true}
		if len(got) != len(want) {
			t.Errorf("GetFollowsByKindViaPPG(3): got %d, want 2; got=%v", len(got), got)
			return
		}
		for pk := range want {
			if !got[pk] {
				t.Errorf("GetFollowsByKindViaPPG(3): missing %s", pk[:8])
			}
		}
		// Must NOT contain muted or reported pubkeys
		if got[f.pkCHex] {
			t.Errorf("GetFollowsByKindViaPPG(3): should NOT return muted pubkey C")
		}
		if got[f.pkDHex] {
			t.Errorf("GetFollowsByKindViaPPG(3): should NOT return reported pubkey D")
		}
	})

	t.Run("GetFollowsByKindViaPPG kind-10000 returns only mutes", func(t *testing.T) {
		serials, err := db.GetFollowsByKindViaPPG(pkASerial, 10000)
		if err != nil {
			t.Fatalf("GetFollowsByKindViaPPG(10000): %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				continue
			}
			got[h] = true
		}
		if len(got) != 1 || !got[f.pkCHex] {
			t.Errorf("GetFollowsByKindViaPPG(10000): want [C], got %v", got)
		}
	})

	t.Run("GetFollowsByKindViaPPG kind-1984 returns only reports", func(t *testing.T) {
		serials, err := db.GetFollowsByKindViaPPG(pkASerial, 1984)
		if err != nil {
			t.Fatalf("GetFollowsByKindViaPPG(1984): %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				continue
			}
			got[h] = true
		}
		if len(got) != 1 || !got[f.pkDHex] {
			t.Errorf("GetFollowsByKindViaPPG(1984): want [D], got %v", got)
		}
	})

	t.Run("GetFollowsByKindViaPPG unknown kind returns empty", func(t *testing.T) {
		serials, err := db.GetFollowsByKindViaPPG(pkASerial, 9999)
		if err != nil {
			t.Fatalf("GetFollowsByKindViaPPG(9999): %v", err)
		}
		if len(serials) != 0 {
			t.Errorf("GetFollowsByKindViaPPG(9999): want empty, got %d results", len(serials))
		}
	})
}

// TestGetFollowersByKindViaGPP verifies the reverse kind-filtered GPP scan.
func TestGetFollowersByKindViaGPP(t *testing.T) {
	f := setupKindFilterFixture(t)
	db := f.db

	pkBSerial, err := db.PubkeyHexToSerial(f.pkBHex)
	if err != nil {
		t.Fatalf("PubkeyHexToSerial(B): %v", err)
	}
	pkCSerial, err := db.PubkeyHexToSerial(f.pkCHex)
	if err != nil {
		t.Fatalf("PubkeyHexToSerial(C): %v", err)
	}
	pkDSerial, err := db.PubkeyHexToSerial(f.pkDHex)
	if err != nil {
		t.Fatalf("PubkeyHexToSerial(D): %v", err)
	}

	t.Run("GetFollowersViaGPP(B) returns all kinds including A and F", func(t *testing.T) {
		serials, err := db.GetFollowersViaGPP(pkBSerial)
		if err != nil {
			t.Fatalf("GetFollowersViaGPP(B): %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				continue
			}
			got[h] = true
		}
		if !got[f.pkAHex] {
			t.Errorf("GetFollowersViaGPP(B): missing A")
		}
		if !got[f.pkFHex] {
			t.Errorf("GetFollowersViaGPP(B): missing F")
		}
	})

	t.Run("GetFollowersByKindViaGPP(B, 3) returns A and F", func(t *testing.T) {
		serials, err := db.GetFollowersByKindViaGPP(pkBSerial, 3)
		if err != nil {
			t.Fatalf("GetFollowersByKindViaGPP(B,3): %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				continue
			}
			got[h] = true
		}
		if len(got) != 2 {
			t.Errorf("GetFollowersByKindViaGPP(B,3): want 2, got %d: %v", len(got), got)
			return
		}
		if !got[f.pkAHex] {
			t.Errorf("GetFollowersByKindViaGPP(B,3): missing A")
		}
		if !got[f.pkFHex] {
			t.Errorf("GetFollowersByKindViaGPP(B,3): missing F")
		}
	})

	t.Run("GetFollowersByKindViaGPP(B, 10000) returns empty — nobody mutes B", func(t *testing.T) {
		serials, err := db.GetFollowersByKindViaGPP(pkBSerial, 10000)
		if err != nil {
			t.Fatalf("GetFollowersByKindViaGPP(B,10000): %v", err)
		}
		if len(serials) != 0 {
			t.Errorf("GetFollowersByKindViaGPP(B,10000): want empty, got %d", len(serials))
		}
	})

	t.Run("GetFollowersByKindViaGPP(C, 10000) returns A — A mutes C", func(t *testing.T) {
		serials, err := db.GetFollowersByKindViaGPP(pkCSerial, 10000)
		if err != nil {
			t.Fatalf("GetFollowersByKindViaGPP(C,10000): %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				continue
			}
			got[h] = true
		}
		if len(got) != 1 || !got[f.pkAHex] {
			t.Errorf("GetFollowersByKindViaGPP(C,10000): want [A], got %v", got)
		}
	})

	t.Run("GetFollowersByKindViaGPP(C, 3) returns empty — A only mutes C, does not follow", func(t *testing.T) {
		serials, err := db.GetFollowersByKindViaGPP(pkCSerial, 3)
		if err != nil {
			t.Fatalf("GetFollowersByKindViaGPP(C,3): %v", err)
		}
		if len(serials) != 0 {
			got := make([]string, 0, len(serials))
			for _, s := range serials {
				h, _ := db.GetPubkeyHexFromSerial(s)
				got = append(got, h[:8])
			}
			t.Errorf("GetFollowersByKindViaGPP(C,3): want empty (A mutes C but does not kind-3 follow C), got %v", got)
		}
	})

	t.Run("GetFollowersByKindViaGPP(D, 1984) returns A — A reports D", func(t *testing.T) {
		serials, err := db.GetFollowersByKindViaGPP(pkDSerial, 1984)
		if err != nil {
			t.Fatalf("GetFollowersByKindViaGPP(D,1984): %v", err)
		}
		got := make(map[string]bool, len(serials))
		for _, s := range serials {
			h, err := db.GetPubkeyHexFromSerial(s)
			if err != nil {
				continue
			}
			got[h] = true
		}
		if len(got) != 1 || !got[f.pkAHex] {
			t.Errorf("GetFollowersByKindViaGPP(D,1984): want [A], got %v", got)
		}
	})

	t.Run("GetFollowersByKindViaGPP(D, 3) returns empty — A only reports D", func(t *testing.T) {
		serials, err := db.GetFollowersByKindViaGPP(pkDSerial, 3)
		if err != nil {
			t.Fatalf("GetFollowersByKindViaGPP(D,3): %v", err)
		}
		if len(serials) != 0 {
			t.Errorf("GetFollowersByKindViaGPP(D,3): want empty, got %d", len(serials))
		}
	})
}

// TestKindFilterIsolation is the key regression test for the grapevine zero-influence bug.
//
// The bug: GetFollowersViaGPP (all kinds) fed getFollowers() in the engine.
// When pkA both follows AND mutes pkX (different targets here, but same mechanism),
// a node that is only muted by pkA should NOT appear as a kind-3 follower.
// This test proves the kind-3 filtered path is correctly isolated from mutes/reports.
func TestKindFilterIsolation(t *testing.T) {
	f := setupKindFilterFixture(t)
	db := f.db

	pkBSerial, _ := db.PubkeyHexToSerial(f.pkBHex)
	pkCSerial, _ := db.PubkeyHexToSerial(f.pkCHex)
	pkDSerial, _ := db.PubkeyHexToSerial(f.pkDHex)

	// Followers of B via kind-3 should be {A, F} — not empty
	followersOfB, err := db.GetFollowersByKindViaGPP(pkBSerial, 3)
	if err != nil {
		t.Fatalf("GetFollowersByKindViaGPP(B,3): %v", err)
	}
	if len(followersOfB) != 2 {
		t.Errorf("B should have 2 kind-3 followers (A and F), got %d", len(followersOfB))
	}

	// C is only muted, not followed — kind-3 followers of C must be empty
	kind3FollowersOfC, err := db.GetFollowersByKindViaGPP(pkCSerial, 3)
	if err != nil {
		t.Fatalf("GetFollowersByKindViaGPP(C,3): %v", err)
	}
	if len(kind3FollowersOfC) != 0 {
		t.Errorf("C has no kind-3 followers (A only mutes C), but got %d results — this is the double-count bug", len(kind3FollowersOfC))
	}

	// Muters of C must be {A}
	mutersOfC, err := db.GetFollowersByKindViaGPP(pkCSerial, 10000)
	if err != nil {
		t.Fatalf("GetFollowersByKindViaGPP(C,10000): %v", err)
	}
	if len(mutersOfC) != 1 {
		t.Errorf("C should have 1 muter (A), got %d", len(mutersOfC))
	}

	// D is only reported, not followed
	kind3FollowersOfD, err := db.GetFollowersByKindViaGPP(pkDSerial, 3)
	if err != nil {
		t.Fatalf("GetFollowersByKindViaGPP(D,3): %v", err)
	}
	if len(kind3FollowersOfD) != 0 {
		t.Errorf("D has no kind-3 followers (A only reports D), but got %d results", len(kind3FollowersOfD))
	}

	// kind-3 follows of A via PPG: only B and E — not C (muted) or D (reported)
	pkASerial, _ := db.PubkeyHexToSerial(f.pkAHex)
	kind3FollowsOfA, err := db.GetFollowsByKindViaPPG(pkASerial, 3)
	if err != nil {
		t.Fatalf("GetFollowsByKindViaPPG(A,3): %v", err)
	}
	if len(kind3FollowsOfA) != 2 {
		got := make([]string, 0, len(kind3FollowsOfA))
		for _, s := range kind3FollowsOfA {
			h, _ := db.GetPubkeyHexFromSerial(s)
			got = append(got, h[:8])
		}
		sort.Strings(got)
		t.Errorf("A has 2 kind-3 follows (B,E), got %d: %v — mutes/reports must not bleed into kind-3", len(kind3FollowsOfA), got)
	}
}
