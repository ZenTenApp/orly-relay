package database

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

func TestDualStorageForReplaceableEvents(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-dual-db-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	require.NoError(t, err)
	defer db.Close()

	// Create a signing key
	sign := p8k.MustNew()
	require.NoError(t, sign.Generate())

	t.Run("SmallReplaceableEvent", func(t *testing.T) {
		// Create a small replaceable event (kind 0 - profile metadata)
		ev := event.New()
		ev.Pubkey = sign.Pub()
		ev.CreatedAt = timestamp.Now().V
		ev.Kind = kind.ProfileMetadata.K
		ev.Tags = tag.NewS()
		ev.Content = []byte(`{"name":"Alice","about":"Test user"}`)

		require.NoError(t, ev.Sign(sign))

		// Save the event
		replaced, err := db.SaveEvent(ctx, ev)
		require.NoError(t, err)
		assert.False(t, replaced)

		// Fetch by serial - should work via sev key
		ser, err := db.GetSerialById(ev.ID)
		require.NoError(t, err)
		require.NotNil(t, ser)

		fetched, err := db.FetchEventBySerial(ser)
		require.NoError(t, err)
		require.NotNil(t, fetched)

		// Verify event contents
		assert.Equal(t, ev.ID, fetched.ID)
		assert.Equal(t, ev.Pubkey, fetched.Pubkey)
		assert.Equal(t, ev.Kind, fetched.Kind)
		assert.Equal(t, ev.Content, fetched.Content)
	})

	t.Run("LargeReplaceableEvent", func(t *testing.T) {
		// Create a large replaceable event (> 384 bytes)
		largeContent := make([]byte, 500)
		for i := range largeContent {
			largeContent[i] = 'x'
		}

		ev := event.New()
		ev.Pubkey = sign.Pub()
		ev.CreatedAt = timestamp.Now().V + 1
		ev.Kind = kind.ProfileMetadata.K
		ev.Tags = tag.NewS()
		ev.Content = largeContent

		require.NoError(t, ev.Sign(sign))

		// Save the event
		replaced, err := db.SaveEvent(ctx, ev)
		require.NoError(t, err)
		assert.True(t, replaced) // Should replace the previous profile

		// Fetch by serial - should work via evt key
		ser, err := db.GetSerialById(ev.ID)
		require.NoError(t, err)
		require.NotNil(t, ser)

		fetched, err := db.FetchEventBySerial(ser)
		require.NoError(t, err)
		require.NotNil(t, fetched)

		// Verify event contents
		assert.Equal(t, ev.ID, fetched.ID)
		assert.Equal(t, ev.Content, fetched.Content)
	})
}

func TestDualStorageForAddressableEvents(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-addressable-db-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	require.NoError(t, err)
	defer db.Close()

	// Create a signing key
	sign := p8k.MustNew()
	require.NoError(t, sign.Generate())

	t.Run("SmallAddressableEvent", func(t *testing.T) {
		// Create a small addressable event (kind 30023 - long-form content)
		ev := event.New()
		ev.Pubkey = sign.Pub()
		ev.CreatedAt = timestamp.Now().V
		ev.Kind = 30023
		ev.Tags = tag.NewS(
			tag.NewFromAny("d", []byte("my-article")),
			tag.NewFromAny("title", []byte("Test Article")),
		)
		ev.Content = []byte("This is a short article.")

		require.NoError(t, ev.Sign(sign))

		// Save the event
		replaced, err := db.SaveEvent(ctx, ev)
		require.NoError(t, err)
		assert.False(t, replaced)

		// Fetch by serial - should work via sev key
		ser, err := db.GetSerialById(ev.ID)
		require.NoError(t, err)
		require.NotNil(t, ser)

		fetched, err := db.FetchEventBySerial(ser)
		require.NoError(t, err)
		require.NotNil(t, fetched)

		// Verify event contents
		assert.Equal(t, ev.ID, fetched.ID)
		assert.Equal(t, ev.Pubkey, fetched.Pubkey)
		assert.Equal(t, ev.Kind, fetched.Kind)
		assert.Equal(t, ev.Content, fetched.Content)

		// Verify d tag
		dTag := fetched.Tags.GetFirst([]byte("d"))
		require.NotNil(t, dTag)
		assert.Equal(t, []byte("my-article"), dTag.Value())
	})

	t.Run("AddressableEventWithoutDTag", func(t *testing.T) {
		// Create an addressable event without d tag (should be treated as regular event)
		ev := event.New()
		ev.Pubkey = sign.Pub()
		ev.CreatedAt = timestamp.Now().V + 1
		ev.Kind = 30023
		ev.Tags = tag.NewS()
		ev.Content = []byte("Article without d tag")

		require.NoError(t, ev.Sign(sign))

		// Save should fail with missing d tag error
		_, err := db.SaveEvent(ctx, ev)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing a d tag")
	})

	t.Run("ReplaceAddressableEvent", func(t *testing.T) {
		// Create first version
		ev1 := event.New()
		ev1.Pubkey = sign.Pub()
		ev1.CreatedAt = timestamp.Now().V
		ev1.Kind = 30023
		ev1.Tags = tag.NewS(
			tag.NewFromAny("d", []byte("replaceable-article")),
		)
		ev1.Content = []byte("Version 1")

		require.NoError(t, ev1.Sign(sign))

		replaced, err := db.SaveEvent(ctx, ev1)
		require.NoError(t, err)
		assert.False(t, replaced)

		// Create second version (newer)
		ev2 := event.New()
		ev2.Pubkey = sign.Pub()
		ev2.CreatedAt = ev1.CreatedAt + 10
		ev2.Kind = 30023
		ev2.Tags = tag.NewS(
			tag.NewFromAny("d", []byte("replaceable-article")),
		)
		ev2.Content = []byte("Version 2")

		require.NoError(t, ev2.Sign(sign))

		replaced, err = db.SaveEvent(ctx, ev2)
		require.NoError(t, err)
		assert.True(t, replaced)

		// Try to save older version (should fail)
		ev0 := event.New()
		ev0.Pubkey = sign.Pub()
		ev0.CreatedAt = ev1.CreatedAt - 10
		ev0.Kind = 30023
		ev0.Tags = tag.NewS(
			tag.NewFromAny("d", []byte("replaceable-article")),
		)
		ev0.Content = []byte("Version 0 (old)")

		require.NoError(t, ev0.Sign(sign))

		replaced, err = db.SaveEvent(ctx, ev0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "older than existing")
	})
}

func TestDualStorageRegularEvents(t *testing.T) {
	// Create a temporary directory for the database
	tempDir, err := os.MkdirTemp("", "test-regular-db-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a context and cancel function for the database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the database
	db, err := New(ctx, cancel, tempDir, "info")
	require.NoError(t, err)
	defer db.Close()

	// Create a signing key
	sign := p8k.MustNew()
	require.NoError(t, sign.Generate())

	t.Run("SmallRegularEvent", func(t *testing.T) {
		// Create a small regular event (kind 1 - note)
		ev := event.New()
		ev.Pubkey = sign.Pub()
		ev.CreatedAt = timestamp.Now().V
		ev.Kind = kind.TextNote.K
		ev.Tags = tag.NewS()
		ev.Content = []byte("Hello, Nostr!")

		require.NoError(t, ev.Sign(sign))

		// Save the event
		replaced, err := db.SaveEvent(ctx, ev)
		require.NoError(t, err)
		assert.False(t, replaced)

		// Fetch by serial - should work via sev key
		ser, err := db.GetSerialById(ev.ID)
		require.NoError(t, err)
		require.NotNil(t, ser)

		fetched, err := db.FetchEventBySerial(ser)
		require.NoError(t, err)
		require.NotNil(t, fetched)

		// Verify event contents
		assert.Equal(t, ev.ID, fetched.ID)
		assert.Equal(t, ev.Content, fetched.Content)
	})
}
