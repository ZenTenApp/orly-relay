package neo4j

import (
	"context"
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// createTestSigner creates a new signer for testing.
func createTestSigner(t *testing.T) *p8k.Signer {
	t.Helper()
	s, err := p8k.New()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := s.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	return s
}

// saveTestEvent creates, signs, and saves a test event, returning it.
func saveTestEvent(t *testing.T, signer *p8k.Signer, kindVal uint16, content string, tags *tag.S) *event.E {
	t.Helper()
	ctx := context.Background()

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = timestamp.Now().V
	ev.Kind = kindVal
	ev.Content = []byte(content)
	ev.Tags = tags

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	exists, err := testDB.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("Failed to save event: %v", err)
	}
	if exists {
		t.Fatalf("Event already exists")
	}

	return ev
}

// uintPtr returns a pointer to a uint value.
func uintPtr(v uint) *uint { return &v }

// TestSaveEvent_WordTokens verifies that saving an event creates Word nodes
// and HAS_WORD relationships for content words.
func TestSaveEvent_WordTokens(t *testing.T) {
	cleanTestDatabase()
	ctx := context.Background()
	signer := createTestSigner(t)

	// Save event with known content
	saveTestEvent(t, signer, 1, "Bitcoin lightning network", nil)

	// Verify Word nodes were created
	wordCount := countNodes(t, "Word")
	if wordCount == 0 {
		t.Fatal("Expected Word nodes to be created, got 0")
	}

	// Verify specific words exist (bitcoin, lightning, network)
	for _, word := range []string{"bitcoin", "lightning", "network"} {
		cypher := `MATCH (w:Word) WHERE w.text = $text RETURN w.hash AS hash`
		result, err := testDB.ExecuteRead(ctx, cypher, map[string]any{"text": word})
		if err != nil {
			t.Fatalf("Failed to query Word node for %q: %v", word, err)
		}
		if !result.Next(ctx) {
			t.Errorf("Expected Word node for %q, not found", word)
		}
	}

	// Verify HAS_WORD relationships exist
	cypher := `MATCH (e:Event)-[:HAS_WORD]->(w:Word) RETURN count(w) AS count`
	result, err := testDB.ExecuteRead(ctx, cypher, nil)
	if err != nil {
		t.Fatalf("Failed to count HAS_WORD relationships: %v", err)
	}
	if result.Next(ctx) {
		count, _ := result.Record().Values[0].(int64)
		if count < 3 {
			t.Errorf("Expected at least 3 HAS_WORD relationships, got %d", count)
		}
	}
}

// TestSaveEvent_WordTokensFromTags verifies that tag field values are also tokenized.
func TestSaveEvent_WordTokensFromTags(t *testing.T) {
	cleanTestDatabase()
	ctx := context.Background()
	signer := createTestSigner(t)

	// Save event with content in tags but minimal body
	tags := tag.NewS(
		tag.NewFromAny("t", "cryptocurrency"),
		tag.NewFromAny("subject", "decentralized finance"),
	)
	saveTestEvent(t, signer, 1, "hi", tags)

	// Verify tag-sourced words exist
	for _, word := range []string{"cryptocurrency", "decentralized", "finance"} {
		cypher := `MATCH (w:Word) WHERE w.text = $text RETURN w.hash AS hash`
		result, err := testDB.ExecuteRead(ctx, cypher, map[string]any{"text": word})
		if err != nil {
			t.Fatalf("Failed to query Word node for %q: %v", word, err)
		}
		if !result.Next(ctx) {
			t.Errorf("Expected Word node for %q from tags, not found", word)
		}
	}
}

// TestSearchQuery_SingleTerm verifies basic single-term search returns matching events.
func TestSearchQuery_SingleTerm(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	// Save events with distinct content
	saveTestEvent(t, signer, 1, "Bitcoin is digital gold", nil)
	saveTestEvent(t, signer, 1, "Ethereum smart contracts", nil)
	saveTestEvent(t, signer, 1, "Hello world greeting", nil)

	// Search for "bitcoin"
	f := &filter.F{
		Search: []byte("bitcoin"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 result for 'bitcoin', got %d", len(evs))
	}

	if string(evs[0].Content) != "Bitcoin is digital gold" {
		t.Errorf("Expected bitcoin event, got: %s", string(evs[0].Content))
	}
}

// TestSearchQuery_MultiTerm verifies that events matching more search terms rank higher.
func TestSearchQuery_MultiTerm(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	// Event matching one word
	saveTestEvent(t, signer, 1, "Bitcoin price today", nil)
	// Event matching two words
	saveTestEvent(t, signer, 1, "Bitcoin lightning network payment", nil)
	// Event matching no words (should not appear)
	saveTestEvent(t, signer, 1, "Hello world greeting", nil)

	// Search for "bitcoin lightning"
	f := &filter.F{
		Search: []byte("bitcoin lightning"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 results for 'bitcoin lightning', got %d", len(evs))
	}

	// The event matching both words should rank first
	if string(evs[0].Content) != "Bitcoin lightning network payment" {
		t.Errorf("Expected 2-match event first, got: %s", string(evs[0].Content))
	}
}

// TestSearchQuery_WithKindFilter verifies search combined with kinds filter.
func TestSearchQuery_WithKindFilter(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	// Kind 1 (text note) with bitcoin
	saveTestEvent(t, signer, 1, "Bitcoin price analysis", nil)
	// Kind 30023 (long-form) with bitcoin
	dTag := tag.NewS(tag.NewFromAny("d", "article-1"))
	saveTestEvent(t, signer, 30023, "Bitcoin deep dive article", dTag)

	// Search for "bitcoin" but only kind 1
	f := &filter.F{
		Search: []byte("bitcoin"),
		Kinds:  kind.NewS(kind.New(1)),
		Limit:  uintPtr(10),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 result for 'bitcoin' kind 1, got %d", len(evs))
	}

	if evs[0].Kind != 1 {
		t.Errorf("Expected kind 1, got kind %d", evs[0].Kind)
	}
}

// TestSearchQuery_WithAuthorFilter verifies search combined with authors filter.
func TestSearchQuery_WithAuthorFilter(t *testing.T) {
	cleanTestDatabase()

	alice := createTestSigner(t)
	bob := createTestSigner(t)

	// Both authors post about bitcoin
	saveTestEvent(t, alice, 1, "Bitcoin from Alice perspective", nil)
	saveTestEvent(t, bob, 1, "Bitcoin from Bob perspective", nil)

	// Search for "bitcoin" but only from Alice
	f := &filter.F{
		Search:  []byte("bitcoin"),
		Authors: tag.NewFromBytesSlice(alice.Pub()),
		Limit:   uintPtr(10),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 result for alice's bitcoin, got %d", len(evs))
	}

	alicePub := hex.Enc(evs[0].Pubkey)
	expectedPub := hex.Enc(alice.Pub())
	if alicePub != expectedPub {
		t.Errorf("Expected alice's pubkey, got different author")
	}
}

// TestSearchQuery_URLsIgnored verifies that URLs in content are not indexed.
func TestSearchQuery_URLsIgnored(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	// Content with URL — "example" should not be indexed from the URL
	saveTestEvent(t, signer, 1, "Check https://example.com for details", nil)

	// Search for "example" — should not match the URL
	f := &filter.F{
		Search: []byte("example"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// "example" only appears in the URL which should be skipped
	if len(evs) != 0 {
		t.Errorf("Expected 0 results (URL words should be ignored), got %d", len(evs))
	}
}

// TestSearchQuery_CaseInsensitive verifies that search is case-insensitive.
func TestSearchQuery_CaseInsensitive(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	saveTestEvent(t, signer, 1, "BITCOIN IS GREAT", nil)

	// Search with lowercase
	f := &filter.F{
		Search: []byte("bitcoin"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Errorf("Expected 1 result for case-insensitive 'bitcoin', got %d", len(evs))
	}
}

// TestSearchQuery_NoResults verifies empty result for unmatched terms.
func TestSearchQuery_NoResults(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	saveTestEvent(t, signer, 1, "Hello world", nil)

	// Search for a term that doesn't exist
	f := &filter.F{
		Search: []byte("xyznonexistent"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 0 {
		t.Errorf("Expected 0 results for non-existent term, got %d", len(evs))
	}
}

// TestSearchQuery_UnicodeNormalization verifies that decorative unicode
// (small caps, fraktur) in event content matches plain ASCII search terms.
func TestSearchQuery_UnicodeNormalization(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	// Save event with small caps content
	saveTestEvent(t, signer, 1, "ᴅᴇᴀᴛʜ comes for everyone", nil)

	// Search for the ASCII equivalent
	f := &filter.F{
		Search: []byte("death"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 result for 'death' matching small caps, got %d", len(evs))
	}
}

// TestSearchQuery_EmptyContentTagWords verifies that events with no content
// but with searchable words in tags are found by search.
func TestSearchQuery_EmptyContentTagWords(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	// Event with empty content but words in tag values
	tags := tag.NewS(
		tag.NewFromAny("subject", "bitcoin trading strategies"),
	)
	saveTestEvent(t, signer, 1, "", tags)

	// Search for a word that appears only in the tag value
	f := &filter.F{
		Search: []byte("strategies"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("Expected 1 result for tag-only word 'strategies', got %d", len(evs))
	}
}

// TestSearchQuery_MultiTermScoringRecency verifies that recency affects
// ranking when match counts are equal.
func TestSearchQuery_MultiTermScoringRecency(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	// Save events with identical content but different timestamps.
	// We set created_at manually via saveTestEvent ordering.
	// ev1: old event matching "bitcoin lightning"
	ev1 := event.New()
	ev1.Pubkey = signer.Pub()
	ev1.CreatedAt = 1000000
	ev1.Kind = 1
	ev1.Content = []byte("bitcoin lightning old post")
	ev1.Tags = tag.NewS()
	if err := ev1.Sign(signer); err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}
	if _, err := testDB.SaveEvent(context.Background(), ev1); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// ev2: recent event matching "bitcoin lightning"
	ev2 := event.New()
	ev2.Pubkey = signer.Pub()
	ev2.CreatedAt = 2000000
	ev2.Kind = 1
	ev2.Content = []byte("bitcoin lightning recent post")
	ev2.Tags = tag.NewS()
	if err := ev2.Sign(signer); err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}
	if _, err := testDB.SaveEvent(context.Background(), ev2); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	f := &filter.F{
		Search: []byte("bitcoin lightning"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(evs))
	}

	// Both match 2 terms, so recency should break the tie — recent event first
	if evs[0].CreatedAt < evs[1].CreatedAt {
		t.Errorf("Expected recent event first (created_at %d), got older event first (created_at %d)",
			evs[1].CreatedAt, evs[0].CreatedAt)
	}
}

// TestSearchQuery_LimitInteraction verifies that limit=1 returns exactly 1 result.
func TestSearchQuery_LimitInteraction(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	saveTestEvent(t, signer, 1, "bitcoin price today", nil)
	saveTestEvent(t, signer, 1, "bitcoin market analysis", nil)
	saveTestEvent(t, signer, 1, "bitcoin lightning network", nil)

	f := &filter.F{
		Search: []byte("bitcoin"),
		Limit:  uintPtr(1),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Errorf("Expected exactly 1 result with limit=1, got %d", len(evs))
	}
}

// TestSearchQuery_StopWordsNotIndexed verifies that searching for a stop word
// returns no results because stop words are not indexed.
func TestSearchQuery_StopWordsNotIndexed(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	saveTestEvent(t, signer, 1, "the quick brown fox", nil)

	// "the" is a stop word — should not be indexed
	f := &filter.F{
		Search: []byte("the"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 0 {
		t.Errorf("Expected 0 results for stop word 'the', got %d", len(evs))
	}
}

// TestSearchQuery_TagValueOnly verifies that words appearing only in tag values
// (not in content) are searchable.
func TestSearchQuery_TagValueOnly(t *testing.T) {
	cleanTestDatabase()
	signer := createTestSigner(t)

	tags := tag.NewS(
		tag.NewFromAny("t", "cryptocurrency"),
	)
	saveTestEvent(t, signer, 1, "hello world", tags)

	f := &filter.F{
		Search: []byte("cryptocurrency"),
	}

	evs, err := testDB.QueryEvents(context.Background(), f)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(evs) != 1 {
		t.Errorf("Expected 1 result for tag-value word 'cryptocurrency', got %d", len(evs))
	}
}
