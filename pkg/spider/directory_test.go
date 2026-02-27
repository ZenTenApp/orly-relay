package spider

import (
	"context"
	"testing"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRelaysFromEvents(t *testing.T) {
	ds := &DirectorySpider{}

	tests := []struct {
		name     string
		events   []*event.E
		expected []string
	}{
		{
			name:     "empty events",
			events:   []*event.E{},
			expected: []string{},
		},
		{
			name: "single event with relays",
			events: []*event.E{
				{
					Kind: kind.RelayListMetadata.K,
					Tags: &tag.S{
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://relay1.example.com")),
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://relay2.example.com")),
					},
				},
			},
			expected: []string{"wss://relay1.example.com", "wss://relay2.example.com"},
		},
		{
			name: "multiple events with duplicate relays",
			events: []*event.E{
				{
					Kind: kind.RelayListMetadata.K,
					Tags: &tag.S{
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://relay1.example.com")),
					},
				},
				{
					Kind: kind.RelayListMetadata.K,
					Tags: &tag.S{
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://relay1.example.com")),
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://relay3.example.com")),
					},
				},
			},
			expected: []string{"wss://relay1.example.com", "wss://relay3.example.com"},
		},
		{
			name: "event with empty r tags",
			events: []*event.E{
				{
					Kind: kind.RelayListMetadata.K,
					Tags: &tag.S{
						tag.NewFromBytesSlice([]byte("r")), // empty value
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://valid.relay.com")),
					},
				},
			},
			expected: []string{"wss://valid.relay.com"},
		},
		{
			name: "normalizes relay URLs",
			events: []*event.E{
				{
					Kind: kind.RelayListMetadata.K,
					Tags: &tag.S{
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://relay.example.com")),
						tag.NewFromBytesSlice([]byte("r"), []byte("wss://relay.example.com/")), // duplicate with trailing slash
					},
				},
			},
			expected: []string{"wss://relay.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ds.extractRelaysFromEvents(tt.events)

			// For empty case, check length
			if len(tt.expected) == 0 {
				assert.Empty(t, result)
				return
			}

			// Check that all expected relays are present (order may vary)
			assert.Len(t, result, len(tt.expected))
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestDirectorySpiderLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create spider without database (will return error)
	_, err := NewDirectorySpider(ctx, nil, nil, 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database cannot be nil")
}

func TestDirectorySpiderDefaults(t *testing.T) {
	// Test that defaults are applied correctly
	assert.Equal(t, 24*time.Hour, DirectorySpiderDefaultInterval)
	assert.Equal(t, 3, DirectorySpiderDefaultMaxHops)
	assert.Equal(t, 30*time.Second, DirectorySpiderRelayTimeout)
	assert.Equal(t, 60*time.Second, DirectorySpiderQueryTimeout)
	assert.Equal(t, 500*time.Millisecond, DirectorySpiderRelayDelay)
	assert.Equal(t, 5000, DirectorySpiderMaxEventsPerQuery)
}

func TestTriggerNow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ds := &DirectorySpider{
		ctx:         ctx,
		triggerChan: make(chan struct{}, 1),
	}

	// First trigger should succeed
	ds.TriggerNow()

	// Verify trigger was sent
	select {
	case <-ds.triggerChan:
		// Expected
	default:
		t.Error("trigger was not sent")
	}

	// Second trigger while channel is empty should also succeed
	ds.TriggerNow()

	// But if we trigger again without draining, it should not block
	ds.TriggerNow() // Should not block due to select default case
}

func TestLastRun(t *testing.T) {
	ds := &DirectorySpider{}

	// Initially should be zero
	assert.True(t, ds.LastRun().IsZero())

	// Set a time
	now := time.Now()
	ds.lastRun = now

	// Should return the set time
	assert.Equal(t, now, ds.LastRun())
}
