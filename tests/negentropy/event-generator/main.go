// event-generator generates properly signed Nostr events for negentropy testing.
// Creates events of various kinds with realistic content for sync testing.
// Sends events via a single WebSocket connection using gorilla/websocket.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

// Test key pairs (deterministic for reproducible tests)
var testKeys = []struct {
	Name    string
	PrivKey string
}{
	{
		Name:    "alice",
		PrivKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	},
	{
		Name:    "bob",
		PrivKey: "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
	},
	{
		Name:    "carol",
		PrivKey: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	},
}

// Pre-create signers so we don't recreate them per event
var signers []*p8k.Signer

func init() {
	signers = make([]*p8k.Signer, len(testKeys))
	for i, key := range testKeys {
		s, err := p8k.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create signer for %s: %v\n", key.Name, err)
			os.Exit(1)
		}
		secretKey, err := hex.Dec(key.PrivKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to decode key for %s: %v\n", key.Name, err)
			os.Exit(1)
		}
		if err := s.InitSec(secretKey); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to init signer for %s: %v\n", key.Name, err)
			os.Exit(1)
		}
		signers[i] = s
	}
}

// EventKind represents a Nostr event kind with sample content
type EventKind struct {
	Kind    *kind.K
	Name    string
	Content func(author, index int) string
}

var eventKinds = []EventKind{
	{
		Kind: kind.ProfileMetadata,
		Name: "metadata",
		Content: func(author, index int) string {
			metadata := map[string]string{
				"name":        fmt.Sprintf("TestUser%d_%d", author, index),
				"about":       fmt.Sprintf("Test user %d, event %d for negentropy testing", author, index),
				"picture":     fmt.Sprintf("https://example.com/avatar%d.png", index),
				"nip05":       fmt.Sprintf("user%d@example.com", index),
				"displayName": fmt.Sprintf("Test Display %d", index),
			}
			b, _ := json.Marshal(metadata)
			return string(b)
		},
	},
	{
		Kind: kind.TextNote,
		Name: "short_text_note",
		Content: func(author, index int) string {
			messages := []string{
				"Testing negentropy sync between relays!",
				"This is event number %d in the test suite.",
				"Nostr protocol testing for relay synchronization.",
				"Event %d: checking if sync works correctly.",
				"Negentropy is an efficient set reconciliation protocol.",
				"Testing with kind 1 text notes.",
				"Relay sync test message %d.",
				"Making sure events propagate correctly between relays.",
				"Test event for bidirectional sync testing.",
				"NIP-77 negentropy implementation test.",
			}
			msg := messages[index%len(messages)]
			if index%2 == 0 {
				return fmt.Sprintf(msg, index)
			}
			return msg
		},
	},
	{
		Kind: kind.FollowList,
		Name: "contacts",
		Content: func(author, index int) string {
			return fmt.Sprintf("Contact list update %d for test user %d", index, author)
		},
	},
	{
		Kind: kind.Reporting,
		Name: "report",
		Content: func(author, index int) string {
			return fmt.Sprintf("Report content %d: testing moderation event sync", index)
		},
	},
	{
		Kind: kind.MuteList,
		Name: "mute_list",
		Content: func(author, index int) string {
			return fmt.Sprintf("Mute list update %d", index)
		},
	},
	{
		Kind: kind.PinList,
		Name: "pin_list",
		Content: func(author, index int) string {
			return fmt.Sprintf("Pinned events list %d", index)
		},
	},
	{
		Kind: kind.LongFormContent,
		Name: "long_form",
		Content: func(author, index int) string {
			return fmt.Sprintf("# Long Form Article %d\n\nThis is a test long-form article for kind 30023. Testing negentropy sync with larger content payloads. Article number %d written by test author %d.", index, index, author)
		},
	},
	{
		Kind: kind.ApplicationSpecificData,
		Name: "application_specific",
		Content: func(author, index int) string {
			appData := map[string]interface{}{
				"app":     "test-suite",
				"version": "1.0.0",
				"test_id": index,
				"data": map[string]string{
					"key1": fmt.Sprintf("value%d", index),
					"key2": fmt.Sprintf("data%d", index*2),
				},
			}
			b, _ := json.Marshal(appData)
			return string(b)
		},
	},
}

type Config struct {
	Count      int
	OutputFile string
	RelayURL   string
	BatchSize  int
}

func main() {
	var cfg Config
	flag.IntVar(&cfg.Count, "count", 1000, "Number of events to generate")
	flag.StringVar(&cfg.OutputFile, "output", "", "Output file (JSON array)")
	flag.StringVar(&cfg.RelayURL, "relay", "", "Send directly to relay WebSocket URL")
	flag.IntVar(&cfg.BatchSize, "batch", 100, "Batch size for sending")
	flag.Parse()

	// Generate events
	fmt.Fprintf(os.Stderr, "Generating %d events...\n", cfg.Count)
	events := generateEvents(cfg.Count)

	// Handle output
	if cfg.RelayURL != "" {
		if err := sendToRelay(events, cfg.RelayURL, cfg.BatchSize); err != nil {
			fmt.Fprintf(os.Stderr, "Error sending to relay: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Sent %d events to %s\n", len(events), cfg.RelayURL)
	} else if cfg.OutputFile != "" {
		if err := writeToFile(events, cfg.OutputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Wrote %d events to %s\n", len(events), cfg.OutputFile)
	} else {
		// Print to stdout as JSON array
		output := map[string]interface{}{
			"events": events,
			"count":  len(events),
		}
		jsonBytes, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonBytes))
	}
}

func generateEvents(count int) []*event.E {
	events := make([]*event.E, 0, count)
	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < count; i++ {
		authorIdx := i % len(testKeys)

		kindIdx := getWeightedKindIndex(i)
		kindDef := eventKinds[kindIdx]

		createdAt := baseTime.Add(time.Duration(i) * time.Second).Unix()

		ev, err := createEvent(authorIdx, kindDef.Kind, kindDef.Content(authorIdx, i), createdAt, i)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create event %d: %v\n", i, err)
			continue
		}
		events = append(events, ev)
	}

	return events
}

// kindPattern distributes event kinds in a repeating 20-event pattern.
// This ensures variety even for small event counts while maintaining
// approximate target proportions over larger samples.
//
//	metadata (kind 0):    2/20 = 10%
//	text notes (kind 1): 12/20 = 60%
//	contacts (kind 3):    2/20 = 10%
//	reporting (kind 1984): 1/20 = 5%
//	mute list (kind 10000): 1/20 = 5%
//	pin list (kind 10001): 1/20 = 5%
//	long form (kind 30023): 1/20 = 5%
var kindPattern = []int{
	1, 0, 1, 2, 1, 1, 3, 1, 1, 4,
	1, 5, 1, 6, 1, 1, 0, 1, 2, 1,
}

func getWeightedKindIndex(seed int) int {
	return kindPattern[seed%len(kindPattern)]
}

func createEvent(authorIdx int, kindDef *kind.K, content string, createdAt int64, index int) (*event.E, error) {
	ev := event.New()
	ev.CreatedAt = createdAt
	ev.Kind = kindDef.K
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	signer := signers[authorIdx]

	// Add tags based on kind
	switch kindDef.K {
	case kind.FollowList.K:
		// Add p-tags with hex pubkeys of other test users
		for j := 0; j < 3; j++ {
			targetIdx := (index + j + 1) % len(testKeys)
			targetPub := signers[targetIdx].Pub()
			targetHex := hex.Enc(targetPub)
			ev.Tags.Append(tag.NewFromBytesSlice([]byte("p"), []byte(targetHex)))
		}

	case kind.MuteList.K, kind.PinList.K:
		// Replaceable list events need a d-tag
		ev.Tags.Append(tag.NewFromBytesSlice([]byte("d"), []byte("")))

	case kind.LongFormContent.K:
		// Addressable events MUST have a d-tag
		ev.Tags.Append(tag.NewFromBytesSlice([]byte("d"), []byte(fmt.Sprintf("article-%d", index))))
		ev.Tags.Append(tag.NewFromBytesSlice([]byte("title"), []byte(fmt.Sprintf("Article %d", index))))
		ev.Tags.Append(tag.NewFromBytesSlice([]byte("published_at"), []byte(fmt.Sprintf("%d", createdAt))))

	case kind.ApplicationSpecificData.K:
		// Addressable events MUST have a d-tag
		ev.Tags.Append(tag.NewFromBytesSlice([]byte("d"), []byte(fmt.Sprintf("test-data-%d", index))))

	case kind.Reporting.K:
		targetIdx := (index + 1) % len(testKeys)
		targetPub := signers[targetIdx].Pub()
		targetHex := hex.Enc(targetPub)
		ev.Tags.Append(tag.NewFromBytesSlice([]byte("p"), []byte(targetHex), []byte("other"), []byte("spam")))
	}

	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	return ev, nil
}

// sendToRelay sends events to a relay via a single WebSocket connection.
func sendToRelay(events []*event.E, relayURL string, batchSize int) error {
	u, err := url.Parse(relayURL)
	if err != nil {
		return fmt.Errorf("invalid relay URL: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Connecting to %s...\n", u.String())

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}
	defer conn.Close()

	sent := 0
	rejected := 0

	for i, ev := range events {
		eventJSON, err := ev.MarshalJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to marshal event %d: %v\n", i, err)
			continue
		}

		msg := fmt.Sprintf(`["EVENT",%s]`, string(eventJSON))
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return fmt.Errorf("failed to send event %d: %w", i, err)
		}

		// Read the OK response
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, response, err := conn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: no response for event %d: %v\n", i, err)
		} else {
			// Check if the response indicates success
			respStr := string(response)
			if len(respStr) > 10 {
				// Parse ["OK","id",true/false,"message"]
				var okResp []interface{}
				if json.Unmarshal(response, &okResp) == nil && len(okResp) >= 3 {
					if accepted, ok := okResp[2].(bool); ok && accepted {
						sent++
					} else {
						rejected++
						if rejected <= 5 {
							fmt.Fprintf(os.Stderr, "Rejected: %s\n", respStr)
						}
					}
				}
			}
		}

		// Log progress periodically
		if (i+1)%batchSize == 0 || i == len(events)-1 {
			fmt.Fprintf(os.Stderr, "Progress: %d/%d sent, %d rejected\n", sent, i+1, rejected)
		}
	}

	fmt.Fprintf(os.Stderr, "Total: %d sent, %d rejected out of %d\n", sent, rejected, len(events))
	return nil
}

func writeToFile(events []*event.E, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	output := map[string]interface{}{
		"events": events,
		"count":  len(events),
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
