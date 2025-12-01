package database

import (
	"bytes"
	"encoding/json"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"github.com/stretchr/testify/assert"
)

// TestKind3TagRoundTrip tests that kind 3 events with p tags survive
// JSON -> binary -> JSON round trip
func TestKind3TagRoundTrip(t *testing.T) {
	// Sample kind 3 event JSON with p tags
	kind3JSON := `{
		"id": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		"pubkey": "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
		"created_at": 1234567890,
		"kind": 3,
		"tags": [
			["p", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],
			["p", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"],
			["p", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"]
		],
		"content": "",
		"sig": "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	}`

	// 1. Unmarshal from JSON (simulates receiving from WebSocket)
	ev1 := event.New()
	err := json.Unmarshal([]byte(kind3JSON), ev1)
	assert.NoError(t, err)
	assert.NotNil(t, ev1.Tags)
	assert.Equal(t, 3, ev1.Tags.Len(), "Should have 3 tags")

	// Verify all tags have key "p"
	pTagCount := 0
	for _, tag := range *ev1.Tags {
		if tag != nil && tag.Len() >= 2 {
			key := tag.Key()
			if len(key) == 1 && key[0] == 'p' {
				pTagCount++
				t.Logf("Found p tag with value length: %d bytes", len(tag.Value()))
			}
		}
	}
	assert.Equal(t, 3, pTagCount, "Should have 3 p tags after JSON unmarshal")

	// 2. Marshal to binary (simulates database storage)
	buf := new(bytes.Buffer)
	ev1.MarshalBinary(buf)
	binaryData := buf.Bytes()
	t.Logf("Binary encoding size: %d bytes", len(binaryData))

	// 3. Unmarshal from binary (simulates database retrieval)
	ev2 := event.New()
	err = ev2.UnmarshalBinary(bytes.NewBuffer(binaryData))
	assert.NoError(t, err)
	assert.NotNil(t, ev2.Tags)
	assert.Equal(t, 3, ev2.Tags.Len(), "Should have 3 tags after binary round-trip")

	// Verify all tags still have key "p"
	pTagCount2 := 0
	for _, tag := range *ev2.Tags {
		if tag != nil && tag.Len() >= 2 {
			key := tag.Key()
			if len(key) == 1 && key[0] == 'p' {
				pTagCount2++
				t.Logf("Found p tag after round-trip with value length: %d bytes", len(tag.Value()))
			}
		}
	}
	assert.Equal(t, 3, pTagCount2, "Should have 3 p tags after binary round-trip")

	// 4. Marshal back to JSON to verify tags are still there
	jsonData2, err := json.Marshal(ev2)
	assert.NoError(t, err)
	t.Logf("JSON after round-trip: %s", string(jsonData2))

	// Parse the JSON and count p tags
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonData2, &jsonMap)
	assert.NoError(t, err)

	tags, ok := jsonMap["tags"].([]interface{})
	assert.True(t, ok, "tags should be an array")
	assert.Equal(t, 3, len(tags), "Should have 3 tags in final JSON")

	for i, tag := range tags {
		tagArray, ok := tag.([]interface{})
		assert.True(t, ok, "tag should be an array")
		assert.GreaterOrEqual(t, len(tagArray), 2, "tag should have at least 2 elements")
		assert.Equal(t, "p", tagArray[0], "tag %d should have key 'p'", i)
		t.Logf("Tag %d: %v", i, tagArray)
	}
}
