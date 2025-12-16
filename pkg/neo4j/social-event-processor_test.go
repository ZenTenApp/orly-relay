//go:build integration
// +build integration

package neo4j

import (
	"context"
	"fmt"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// TestSocialEventProcessor tests the social event processor with kinds 0, 3, 1984, 10000
// Uses the shared testDB instance from testmain_test.go to avoid auth rate limiting
func TestSocialEventProcessor(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	// Clean database for this test
	cleanTestDatabase()

	// Generate test keypairs
	alice := generateTestKeypair(t, "alice")
	bob := generateTestKeypair(t, "bob")
	charlie := generateTestKeypair(t, "charlie")
	dave := generateTestKeypair(t, "dave")
	eve := generateTestKeypair(t, "eve")

	// Use explicit timestamps to avoid same-second timing issues
	// (Nostr timestamps are in seconds)
	baseTimestamp := timestamp.Now().V

	t.Run("Kind0_ProfileMetadata", func(t *testing.T) {
		testProfileMetadata(t, ctx, testDB, alice, baseTimestamp)
	})

	t.Run("Kind3_ContactList_Initial", func(t *testing.T) {
		testContactListInitial(t, ctx, testDB, alice, bob, charlie, baseTimestamp+1)
	})

	t.Run("Kind3_ContactList_Update_AddFollow", func(t *testing.T) {
		testContactListUpdate(t, ctx, testDB, alice, bob, charlie, dave, baseTimestamp+2)
	})

	t.Run("Kind3_ContactList_Update_RemoveFollow", func(t *testing.T) {
		testContactListRemove(t, ctx, testDB, alice, bob, charlie, dave, baseTimestamp+3)
	})

	t.Run("Kind3_ContactList_OlderEventRejected", func(t *testing.T) {
		// Use timestamp BEFORE the initial contact list to test rejection
		testContactListOlderRejected(t, ctx, testDB, alice, bob, baseTimestamp)
	})

	t.Run("Kind10000_MuteList", func(t *testing.T) {
		testMuteList(t, ctx, testDB, alice, eve)
	})

	t.Run("Kind1984_Reports", func(t *testing.T) {
		testReports(t, ctx, testDB, alice, bob, eve)
	})

	t.Run("VerifyGraphState", func(t *testing.T) {
		verifyFinalGraphState(t, ctx, testDB, alice, bob, charlie, dave, eve)
	})
}

// testProfileMetadata tests kind 0 profile metadata processing
func testProfileMetadata(t *testing.T, ctx context.Context, db *N, user testKeypair, ts int64) {
	// Create profile metadata event
	ev := event.New()
	ev.Pubkey = user.pubkey
	ev.CreatedAt = ts
	ev.Kind = 0
	ev.Content = []byte(`{"name":"Alice","about":"Test user","picture":"https://example.com/alice.jpg"}`)

	// Sign event
	if err := ev.Sign(user.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	// Save event (which triggers social processing)
	exists, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save profile event: %v", err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify NostrUser node was created with profile data
	cypher := `
		MATCH (u:NostrUser {pubkey: $pubkey})
		RETURN u.name AS name, u.about AS about, u.picture AS picture
	`
	params := map[string]any{"pubkey": hex.Enc(user.pubkey[:])}

	result, err := db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query NostrUser: %v", err)
	}

	if !result.Next(ctx) {
		t.Fatal("NostrUser node not found")
	}

	record := result.Record()
	name := record.Values[0].(string)
	about := record.Values[1].(string)
	picture := record.Values[2].(string)

	if name != "Alice" {
		t.Errorf("Expected name 'Alice', got '%s'", name)
	}
	if about != "Test user" {
		t.Errorf("Expected about 'Test user', got '%s'", about)
	}
	if picture != "https://example.com/alice.jpg" {
		t.Errorf("Expected picture URL, got '%s'", picture)
	}

	t.Logf("✓ Profile metadata processed: name=%s", name)
}

// testContactListInitial tests initial contact list creation
func testContactListInitial(t *testing.T, ctx context.Context, db *N, alice, bob, charlie testKeypair, ts int64) {
	// Alice follows Bob and Charlie
	ev := event.New()
	ev.Pubkey = alice.pubkey
	ev.CreatedAt = ts
	ev.Kind = 3
	ev.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(bob.pubkey[:])),
		tag.NewFromAny("p", hex.Enc(charlie.pubkey[:])),
	)

	if err := ev.Sign(alice.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	exists, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save contact list: %v", err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify FOLLOWS relationships were created
	follows := queryFollows(t, ctx, db, alice.pubkey)
	if len(follows) != 2 {
		t.Fatalf("Expected 2 follows, got %d", len(follows))
	}

	expectedFollows := map[string]bool{
		hex.Enc(bob.pubkey[:]): true,
		hex.Enc(charlie.pubkey[:]): true,
	}

	for _, follow := range follows {
		if !expectedFollows[follow] {
			t.Errorf("Unexpected follow: %s", follow)
		}
		delete(expectedFollows, follow)
	}

	if len(expectedFollows) > 0 {
		t.Errorf("Missing follows: %v", expectedFollows)
	}

	t.Logf("✓ Initial contact list created: Alice follows [Bob, Charlie]")
}

// testContactListUpdate tests adding a follow to existing contact list
func testContactListUpdate(t *testing.T, ctx context.Context, db *N, alice, bob, charlie, dave testKeypair, ts int64) {
	// Alice now follows Bob, Charlie, and Dave
	ev := event.New()
	ev.Pubkey = alice.pubkey
	ev.CreatedAt = ts
	ev.Kind = 3
	ev.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(bob.pubkey[:])),
		tag.NewFromAny("p", hex.Enc(charlie.pubkey[:])),
		tag.NewFromAny("p", hex.Enc(dave.pubkey[:])),
	)

	if err := ev.Sign(alice.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	exists, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save contact list: %v", err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify updated FOLLOWS relationships
	follows := queryFollows(t, ctx, db, alice.pubkey)
	if len(follows) != 3 {
		t.Fatalf("Expected 3 follows, got %d", len(follows))
	}

	expectedFollows := map[string]bool{
		hex.Enc(bob.pubkey[:]): true,
		hex.Enc(charlie.pubkey[:]): true,
		hex.Enc(dave.pubkey[:]): true,
	}

	for _, follow := range follows {
		if !expectedFollows[follow] {
			t.Errorf("Unexpected follow: %s", follow)
		}
		delete(expectedFollows, follow)
	}

	if len(expectedFollows) > 0 {
		t.Errorf("Missing follows: %v", expectedFollows)
	}

	t.Logf("✓ Contact list updated: Alice follows [Bob, Charlie, Dave]")
}

// testContactListRemove tests removing a follow from contact list
func testContactListRemove(t *testing.T, ctx context.Context, db *N, alice, bob, charlie, dave testKeypair, ts int64) {
	// Alice unfollows Charlie, keeps Bob and Dave
	ev := event.New()
	ev.Pubkey = alice.pubkey
	ev.CreatedAt = ts
	ev.Kind = 3
	ev.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(bob.pubkey[:])),
		tag.NewFromAny("p", hex.Enc(dave.pubkey[:])),
	)

	if err := ev.Sign(alice.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	exists, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save contact list: %v", err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify Charlie was removed
	follows := queryFollows(t, ctx, db, alice.pubkey)
	if len(follows) != 2 {
		t.Fatalf("Expected 2 follows after removal, got %d", len(follows))
	}

	expectedFollows := map[string]bool{
		hex.Enc(bob.pubkey[:]): true,
		hex.Enc(dave.pubkey[:]): true,
	}

	for _, follow := range follows {
		if !expectedFollows[follow] {
			t.Errorf("Unexpected follow: %s", follow)
		}
		if follow == hex.Enc(charlie.pubkey[:]) {
			t.Error("Charlie should have been unfollowed")
		}
		delete(expectedFollows, follow)
	}

	t.Logf("✓ Contact list updated: Alice unfollowed Charlie")
}

// testContactListOlderRejected tests that older events are rejected
func testContactListOlderRejected(t *testing.T, ctx context.Context, db *N, alice, bob testKeypair, ts int64) {
	// Try to save an old contact list (timestamp is older than the existing one)
	ev := event.New()
	ev.Pubkey = alice.pubkey
	ev.CreatedAt = ts // This is baseTimestamp, which is older than the current contact list
	ev.Kind = 3
	ev.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(bob.pubkey[:])),
	)

	if err := ev.Sign(alice.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	// Save should succeed (base event stored), but social processing should skip it
	_, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}

	// Verify follows list unchanged (should still be Bob and Dave from previous test)
	follows := queryFollows(t, ctx, db, alice.pubkey)
	if len(follows) != 2 {
		t.Fatalf("Expected follows list unchanged, got %d follows", len(follows))
	}

	t.Logf("✓ Older contact list event rejected (follows unchanged)")
}

// testMuteList tests kind 10000 mute list processing
func testMuteList(t *testing.T, ctx context.Context, db *N, alice, eve testKeypair) {
	// Alice mutes Eve
	ev := event.New()
	ev.Pubkey = alice.pubkey
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = 10000
	ev.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(eve.pubkey[:])),
	)

	if err := ev.Sign(alice.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	exists, err := db.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save mute list: %v", err)
	}
	if exists {
		t.Fatal("Event should not exist yet")
	}

	// Verify MUTES relationship was created
	mutes := queryMutes(t, ctx, db, alice.pubkey)
	if len(mutes) != 1 {
		t.Fatalf("Expected 1 mute, got %d", len(mutes))
	}

	if mutes[0] != hex.Enc(eve.pubkey[:]) {
		t.Errorf("Expected to mute Eve, got %s", mutes[0])
	}

	t.Logf("✓ Mute list processed: Alice mutes Eve")
}

// testReports tests kind 1984 report processing
func testReports(t *testing.T, ctx context.Context, db *N, alice, bob, eve testKeypair) {
	// Alice reports Eve for spam
	ev1 := event.New()
	ev1.Pubkey = alice.pubkey
	ev1.CreatedAt = timestamp.Now().V
	ev1.Kind = 1984
	ev1.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(eve.pubkey[:]), "spam"),
	)
	ev1.Content = []byte("Spamming the relay")

	if err := ev1.Sign(alice.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, ev1); err != nil {
		t.Fatalf("Failed to save report: %v", err)
	}

	// Bob also reports Eve for illegal content
	ev2 := event.New()
	ev2.Pubkey = bob.pubkey
	ev2.CreatedAt = timestamp.Now().V
	ev2.Kind = 1984
	ev2.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(eve.pubkey[:]), "illegal"),
	)

	if err := ev2.Sign(bob.signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if _, err := db.SaveEvent(ctx, ev2); err != nil {
		t.Fatalf("Failed to save report: %v", err)
	}

	// Verify REPORTS relationships were created
	reports := queryReports(t, ctx, db, eve.pubkey)
	if len(reports) != 2 {
		t.Fatalf("Expected 2 reports against Eve, got %d", len(reports))
	}

	// Check report types
	reportTypes := make(map[string]int)
	for _, report := range reports {
		reportTypes[report.ReportType]++
	}

	if reportTypes["spam"] != 1 {
		t.Errorf("Expected 1 spam report, got %d", reportTypes["spam"])
	}
	if reportTypes["illegal"] != 1 {
		t.Errorf("Expected 1 illegal report, got %d", reportTypes["illegal"])
	}

	t.Logf("✓ Reports processed: Eve reported by Alice (spam) and Bob (illegal)")
}

// verifyFinalGraphState verifies the complete graph state
func verifyFinalGraphState(t *testing.T, ctx context.Context, db *N, alice, bob, charlie, dave, eve testKeypair) {
	t.Log("Verifying final graph state...")

	// Verify Alice's follows: Bob and Dave (Charlie removed)
	follows := queryFollows(t, ctx, db, alice.pubkey)
	if len(follows) != 2 {
		t.Errorf("Expected Alice to follow 2 users, got %d", len(follows))
	}

	// Verify Alice's mutes: Eve
	mutes := queryMutes(t, ctx, db, alice.pubkey)
	if len(mutes) != 1 {
		t.Errorf("Expected Alice to mute 1 user, got %d", len(mutes))
	}

	// Verify reports against Eve
	reports := queryReports(t, ctx, db, eve.pubkey)
	if len(reports) != 2 {
		t.Errorf("Expected 2 reports against Eve, got %d", len(reports))
	}

	// Verify event traceability - all relationships should have created_by_event
	cypher := `
		MATCH ()-[r:FOLLOWS|MUTES|REPORTS]->()
		WHERE r.created_by_event IS NULL
		RETURN count(r) AS count
	`
	result, err := db.ExecuteRead(ctx, cypher, nil)
	if err != nil {
		t.Fatalf("Failed to check traceability: %v", err)
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count > 0 {
			t.Errorf("Found %d relationships without created_by_event", count)
		}
	}

	t.Log("✓ Final graph state verified")
	t.Logf("  - Alice follows: %v", follows)
	t.Logf("  - Alice mutes: %v", mutes)
	t.Logf("  - Reports against Eve: %d", len(reports))
}

// Helper types and functions

type testKeypair struct {
	pubkey []byte
	signer *p8k.Signer
}

type reportInfo struct {
	Reporter   string
	ReportType string
}

func generateTestKeypair(t *testing.T, name string) testKeypair {
	t.Helper()

	signer, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer for %s: %v", name, err)
	}

	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair for %s: %v", name, err)
	}

	return testKeypair{
		pubkey: signer.Pub(),
		signer: signer,
	}
}

func queryFollows(t *testing.T, ctx context.Context, db *N, pubkey []byte) []string {
	t.Helper()

	cypher := `
		MATCH (user:NostrUser {pubkey: $pubkey})-[f:FOLLOWS]->(followed:NostrUser)
		WHERE NOT EXISTS {
			MATCH (old:ProcessedSocialEvent {event_id: f.created_by_event})
			WHERE old.superseded_by IS NOT NULL
		}
		RETURN followed.pubkey AS pubkey
	`
	params := map[string]any{"pubkey": hex.Enc(pubkey)}

	result, err := db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query follows: %v", err)
	}

	var follows []string
	for result.Next(ctx) {
		follows = append(follows, result.Record().Values[0].(string))
	}

	return follows
}

func queryMutes(t *testing.T, ctx context.Context, db *N, pubkey []byte) []string {
	t.Helper()

	cypher := `
		MATCH (user:NostrUser {pubkey: $pubkey})-[m:MUTES]->(muted:NostrUser)
		WHERE NOT EXISTS {
			MATCH (old:ProcessedSocialEvent {event_id: m.created_by_event})
			WHERE old.superseded_by IS NOT NULL
		}
		RETURN muted.pubkey AS pubkey
	`
	params := map[string]any{"pubkey": hex.Enc(pubkey)}

	result, err := db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query mutes: %v", err)
	}

	var mutes []string
	for result.Next(ctx) {
		mutes = append(mutes, result.Record().Values[0].(string))
	}

	return mutes
}

func queryReports(t *testing.T, ctx context.Context, db *N, pubkey []byte) []reportInfo {
	t.Helper()

	cypher := `
		MATCH (reporter:NostrUser)-[r:REPORTS]->(reported:NostrUser {pubkey: $pubkey})
		RETURN reporter.pubkey AS reporter, r.report_type AS report_type
	`
	params := map[string]any{"pubkey": hex.Enc(pubkey)}

	result, err := db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		t.Fatalf("Failed to query reports: %v", err)
	}

	var reports []reportInfo
	for result.Next(ctx) {
		record := result.Record()
		reports = append(reports, reportInfo{
			Reporter:   record.Values[0].(string),
			ReportType: record.Values[1].(string),
		})
	}

	return reports
}

// TestDiffComputation tests the diff computation helper function
func TestDiffComputation(t *testing.T) {
	tests := []struct {
		name           string
		old            []string
		new            []string
		expectAdded    []string
		expectRemoved  []string
	}{
		{
			name:          "Empty to non-empty",
			old:           []string{},
			new:           []string{"a", "b", "c"},
			expectAdded:   []string{"a", "b", "c"},
			expectRemoved: []string{},
		},
		{
			name:          "Non-empty to empty",
			old:           []string{"a", "b", "c"},
			new:           []string{},
			expectAdded:   []string{},
			expectRemoved: []string{"a", "b", "c"},
		},
		{
			name:          "No changes",
			old:           []string{"a", "b", "c"},
			new:           []string{"a", "b", "c"},
			expectAdded:   []string{},
			expectRemoved: []string{},
		},
		{
			name:          "Add some, remove some",
			old:           []string{"a", "b", "c"},
			new:           []string{"b", "c", "d", "e"},
			expectAdded:   []string{"d", "e"},
			expectRemoved: []string{"a"},
		},
		{
			name:          "All different",
			old:           []string{"a", "b", "c"},
			new:           []string{"d", "e", "f"},
			expectAdded:   []string{"d", "e", "f"},
			expectRemoved: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := diffStringSlices(tt.old, tt.new)

			if !slicesEqual(added, tt.expectAdded) {
				t.Errorf("Added mismatch:\n  got:      %v\n  expected: %v", added, tt.expectAdded)
			}

			if !slicesEqual(removed, tt.expectRemoved) {
				t.Errorf("Removed mismatch:\n  got:      %v\n  expected: %v", removed, tt.expectRemoved)
			}
		})
	}
}

// slicesEqual checks if two string slices contain the same elements (order doesn't matter)
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]int)
	for _, s := range a {
		aMap[s]++
	}

	bMap := make(map[string]int)
	for _, s := range b {
		bMap[s]++
	}

	for k, v := range aMap {
		if bMap[k] != v {
			return false
		}
	}

	return true
}

// TestExtractPTags tests the p-tag extraction helper function
func TestExtractPTags(t *testing.T) {
	// Valid 64-character hex pubkeys for testing
	pk1 := "0000000000000000000000000000000000000000000000000000000000000001"
	pk2 := "0000000000000000000000000000000000000000000000000000000000000002"
	pk3 := "0000000000000000000000000000000000000000000000000000000000000003"

	tests := []struct {
		name     string
		tags     *tag.S
		expected []string
	}{
		{
			name:     "No tags",
			tags:     &tag.S{},
			expected: []string{},
		},
		{
			name: "Only p-tags",
			tags: tag.NewS(
				tag.NewFromAny("p", pk1),
				tag.NewFromAny("p", pk2),
				tag.NewFromAny("p", pk3),
			),
			expected: []string{pk1, pk2, pk3},
		},
		{
			name: "Mixed tags",
			tags: tag.NewS(
				tag.NewFromAny("p", pk1),
				tag.NewFromAny("e", "event1"),
				tag.NewFromAny("p", pk2),
				tag.NewFromAny("t", "hashtag"),
			),
			expected: []string{pk1, pk2},
		},
		{
			name: "Duplicate p-tags",
			tags: tag.NewS(
				tag.NewFromAny("p", pk1),
				tag.NewFromAny("p", pk1),
				tag.NewFromAny("p", pk2),
			),
			expected: []string{pk1, pk2},
		},
		{
			name: "Invalid p-tags (too short)",
			tags: tag.NewS(
				tag.NewFromAny("p"),
				tag.NewFromAny("p", "tooshort"),
			),
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := event.New()
			ev.Tags = tt.tags

			result := extractPTags(ev)

			if !slicesEqual(result, tt.expected) {
				t.Errorf("Extracted p-tags mismatch:\n  got:      %v\n  expected: %v", result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkDiffComputation(b *testing.B) {
	old := make([]string, 1000)
	new := make([]string, 1000)

	for i := 0; i < 800; i++ {
		old[i] = fmt.Sprintf("pubkey%d", i)
		new[i] = fmt.Sprintf("pubkey%d", i)
	}

	// 200 removed from old
	for i := 800; i < 1000; i++ {
		old[i] = fmt.Sprintf("oldpubkey%d", i)
	}

	// 200 added to new
	for i := 800; i < 1000; i++ {
		new[i] = fmt.Sprintf("newpubkey%d", i)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = diffStringSlices(old, new)
	}
}

// TestReportDeduplication tests that duplicate REPORTS are deduplicated
func TestReportDeduplication(t *testing.T) {
	if testDB == nil {
		t.Skip("Neo4j not available")
	}

	ctx := context.Background()

	t.Run("DeduplicateSameType", func(t *testing.T) {
		// Clean database for this subtest
		cleanTestDatabase()

		reporter := generateTestKeypair(t, "reporter")
		reported := generateTestKeypair(t, "reported")

		reporterPubkey := hex.Enc(reporter.pubkey[:])
		reportedPubkey := hex.Enc(reported.pubkey[:])

		// Create first report (older timestamp)
		ev1 := event.New()
		ev1.Pubkey = reporter.pubkey
		ev1.CreatedAt = 1000
		ev1.Kind = 1984
		ev1.Tags = tag.NewS(
			tag.NewFromAny("p", reportedPubkey, "impersonation"),
		)
		ev1.Content = []byte("First report")

		if err := ev1.Sign(reporter.signer); err != nil {
			t.Fatalf("Failed to sign first event: %v", err)
		}

		if _, err := testDB.SaveEvent(ctx, ev1); err != nil {
			t.Fatalf("Failed to save first report: %v", err)
		}

		// Create second report (newer timestamp, same type)
		ev2 := event.New()
		ev2.Pubkey = reporter.pubkey
		ev2.CreatedAt = 2000 // Newer timestamp
		ev2.Kind = 1984
		ev2.Tags = tag.NewS(
			tag.NewFromAny("p", reportedPubkey, "impersonation"),
		)
		ev2.Content = []byte("Second report")

		if err := ev2.Sign(reporter.signer); err != nil {
			t.Fatalf("Failed to sign second event: %v", err)
		}

		if _, err := testDB.SaveEvent(ctx, ev2); err != nil {
			t.Fatalf("Failed to save second report: %v", err)
		}

		// Verify only ONE REPORTS relationship exists
		cypher := `
			MATCH (r:NostrUser {pubkey: $reporter})-[rel:REPORTS]->(d:NostrUser {pubkey: $reported})
			RETURN count(rel) AS count, rel.created_at AS created_at, rel.created_by_event AS event_id
		`
		params := map[string]any{
			"reporter": reporterPubkey,
			"reported": reportedPubkey,
		}

		result, err := testDB.ExecuteRead(ctx, cypher, params)
		if err != nil {
			t.Fatalf("Failed to query REPORTS: %v", err)
		}

		if !result.Next(ctx) {
			t.Fatal("No REPORTS relationship found")
		}

		record := result.Record()
		count := record.Values[0].(int64)
		createdAt := record.Values[1].(int64)
		eventID := record.Values[2].(string)

		if count != 1 {
			t.Errorf("Expected 1 REPORTS relationship, got %d", count)
		}

		// Verify the relationship has the newer event's data
		if createdAt != 2000 {
			t.Errorf("Expected created_at=2000 (newer), got %d", createdAt)
		}

		ev2ID := hex.Enc(ev2.ID[:])
		if eventID != ev2ID {
			t.Errorf("Expected event_id=%s, got %s", ev2ID, eventID)
		}

		t.Log("✓ Duplicate reports correctly deduplicated to single relationship with newest data")
	})

	t.Run("DifferentTypesAllowed", func(t *testing.T) {
		// Clean database for this subtest
		cleanTestDatabase()

		reporter := generateTestKeypair(t, "reporter2")
		reported := generateTestKeypair(t, "reported2")

		reporterPubkey := hex.Enc(reporter.pubkey[:])
		reportedPubkey := hex.Enc(reported.pubkey[:])

		// Report for impersonation
		ev1 := event.New()
		ev1.Pubkey = reporter.pubkey
		ev1.CreatedAt = 1000
		ev1.Kind = 1984
		ev1.Tags = tag.NewS(
			tag.NewFromAny("p", reportedPubkey, "impersonation"),
		)

		if err := ev1.Sign(reporter.signer); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := testDB.SaveEvent(ctx, ev1); err != nil {
			t.Fatalf("Failed to save report: %v", err)
		}

		// Report for spam (different type)
		ev2 := event.New()
		ev2.Pubkey = reporter.pubkey
		ev2.CreatedAt = 2000
		ev2.Kind = 1984
		ev2.Tags = tag.NewS(
			tag.NewFromAny("p", reportedPubkey, "spam"),
		)

		if err := ev2.Sign(reporter.signer); err != nil {
			t.Fatalf("Failed to sign event: %v", err)
		}

		if _, err := testDB.SaveEvent(ctx, ev2); err != nil {
			t.Fatalf("Failed to save report: %v", err)
		}

		// Verify TWO REPORTS relationships exist (different types)
		cypher := `
			MATCH (r:NostrUser {pubkey: $reporter})-[rel:REPORTS]->(d:NostrUser {pubkey: $reported})
			RETURN rel.report_type AS type ORDER BY type
		`
		params := map[string]any{
			"reporter": reporterPubkey,
			"reported": reportedPubkey,
		}

		result, err := testDB.ExecuteRead(ctx, cypher, params)
		if err != nil {
			t.Fatalf("Failed to query REPORTS: %v", err)
		}

		var types []string
		for result.Next(ctx) {
			types = append(types, result.Record().Values[0].(string))
		}

		if len(types) != 2 {
			t.Errorf("Expected 2 REPORTS relationships, got %d", len(types))
		}

		if len(types) >= 2 && (types[0] != "impersonation" || types[1] != "spam") {
			t.Errorf("Expected [impersonation, spam], got %v", types)
		}

		t.Log("✓ Different report types correctly create separate relationships")
	})

	t.Run("SupersededEventTracking", func(t *testing.T) {
		// Clean database for this subtest
		cleanTestDatabase()

		reporter := generateTestKeypair(t, "reporter3")
		reported := generateTestKeypair(t, "reported3")

		reporterPubkey := hex.Enc(reporter.pubkey[:])
		reportedPubkey := hex.Enc(reported.pubkey[:])

		// Create first report
		ev1 := event.New()
		ev1.Pubkey = reporter.pubkey
		ev1.CreatedAt = 1000
		ev1.Kind = 1984
		ev1.Tags = tag.NewS(
			tag.NewFromAny("p", reportedPubkey, "spam"),
		)

		if err := ev1.Sign(reporter.signer); err != nil {
			t.Fatalf("Failed to sign first event: %v", err)
		}

		if _, err := testDB.SaveEvent(ctx, ev1); err != nil {
			t.Fatalf("Failed to save first report: %v", err)
		}

		ev1ID := hex.Enc(ev1.ID[:])

		// Create second report (supersedes first)
		ev2 := event.New()
		ev2.Pubkey = reporter.pubkey
		ev2.CreatedAt = 2000
		ev2.Kind = 1984
		ev2.Tags = tag.NewS(
			tag.NewFromAny("p", reportedPubkey, "spam"),
		)

		if err := ev2.Sign(reporter.signer); err != nil {
			t.Fatalf("Failed to sign second event: %v", err)
		}

		if _, err := testDB.SaveEvent(ctx, ev2); err != nil {
			t.Fatalf("Failed to save second report: %v", err)
		}

		ev2ID := hex.Enc(ev2.ID[:])

		// Verify first ProcessedSocialEvent is superseded
		cypher := `
			MATCH (evt:ProcessedSocialEvent {event_id: $event_id, event_kind: 1984})
			RETURN evt.superseded_by AS superseded_by
		`
		params := map[string]any{"event_id": ev1ID}

		result, err := testDB.ExecuteRead(ctx, cypher, params)
		if err != nil {
			t.Fatalf("Failed to query ProcessedSocialEvent: %v", err)
		}

		if !result.Next(ctx) {
			t.Fatal("First ProcessedSocialEvent not found")
		}

		supersededBy := result.Record().Values[0]
		if supersededBy == nil {
			t.Error("Expected first event to be superseded, but superseded_by is null")
		} else if supersededBy.(string) != ev2ID {
			t.Errorf("Expected superseded_by=%s, got %v", ev2ID, supersededBy)
		}

		// Verify second ProcessedSocialEvent is NOT superseded
		params = map[string]any{"event_id": ev2ID}
		result, err = testDB.ExecuteRead(ctx, cypher, params)
		if err != nil {
			t.Fatalf("Failed to query second ProcessedSocialEvent: %v", err)
		}

		if !result.Next(ctx) {
			t.Fatal("Second ProcessedSocialEvent not found")
		}

		supersededBy = result.Record().Values[0]
		if supersededBy != nil {
			t.Errorf("Expected second event not to be superseded, but superseded_by=%v", supersededBy)
		}

		t.Log("✓ ProcessedSocialEvent correctly tracks superseded events")
	})
}
