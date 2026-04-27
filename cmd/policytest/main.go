package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

func main() {
	var err error
	url := flag.String("url", "ws://127.0.0.1:3334", "relay websocket URL")
	timeout := flag.Duration("timeout", 20*time.Second, "operation timeout")
	testType := flag.String("type", "event", "test type: 'event' for write control, 'req' for read control, 'both' for both, 'publish-and-query' for full test")
	eventKind := flag.Int("kind", 4678, "event kind to test")
	numEvents := flag.Int("count", 2, "number of events to publish (for publish-and-query)")
	flag.Parse()

	// Connect to relay
	var rl *ws.Client
	if rl, err = ws.RelayConnect(context.Background(), *url); chk.E(err) {
		log.E.F("connect error: %v", err)
		return
	}
	defer rl.Close()

	// Create signer
	var signer *p8k.Signer
	if signer, err = p8k.New(); chk.E(err) {
		log.E.F("signer create error: %v", err)
		return
	}
	if err = signer.Generate(); chk.E(err) {
		log.E.F("signer generate error: %v", err)
		return
	}

	// Perform tests based on type
	switch *testType {
	case "event":
		testEventWrite(rl, signer, *eventKind, *timeout)
	case "req":
		testReqRead(rl, signer, *eventKind, *timeout)
	case "both":
		log.I.Ln("Testing EVENT (write control)...")
		testEventWrite(rl, signer, *eventKind, *timeout)
		log.I.Ln("\nTesting REQ (read control)...")
		testReqRead(rl, signer, *eventKind, *timeout)
	case "publish-and-query":
		testPublishAndQuery(rl, signer, *eventKind, *numEvents, *timeout)
	default:
		log.E.F("invalid test type: %s (must be 'event', 'req', 'both', or 'publish-and-query')", *testType)
	}
}

func testEventWrite(rl *ws.Client, signer *p8k.Signer, eventKind int, timeout time.Duration) {
	ev := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      uint16(eventKind),
		Tags:      tag.NewS(),
		Content:   []byte("policy test: expect rejection for write"),
	}
	if err := ev.Sign(signer); chk.E(err) {
		log.E.F("sign error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := rl.Publish(ctx, ev); err != nil {
		// Expected path if policy rejects: client returns error with reason (from OK false)
		fmt.Println("EVENT policy reject:", err)
		return
	}

	log.I.Ln("EVENT publish result: accepted")
	fmt.Println("EVENT ACCEPT")
}

func testReqRead(rl *ws.Client, signer *p8k.Signer, eventKind int, timeout time.Duration) {
	// First, publish a test event to the relay that we'll try to query
	testEvent := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      uint16(eventKind),
		Tags:      tag.NewS(),
		Content:   []byte("policy test: event for read control test"),
	}
	if err := testEvent.Sign(signer); chk.E(err) {
		log.E.F("sign error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Try to publish the test event first (ignore errors if policy rejects)
	_ = rl.Publish(ctx, testEvent)
	log.I.F("published test event kind %d for read testing", eventKind)

	// Now try to query for events of this kind
	limit := uint(10)
	f := &filter.F{
		Kinds: kind.FromIntSlice([]int{eventKind}),
		Limit: &limit,
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()

	events, err := rl.QuerySync(ctx2, f)
	if chk.E(err) {
		log.E.F("query error: %v", err)
		fmt.Println("REQ query error:", err)
		return
	}

	// Check if we got the expected events
	if len(events) == 0 {
		// Could mean policy filtered it out, or it wasn't stored
		fmt.Println("REQ policy reject: no events returned (filtered by read policy)")
		log.I.F("REQ result: no events of kind %d returned (policy filtered or not stored)", eventKind)
		return
	}

	// Events were returned - read access allowed
	fmt.Printf("REQ ACCEPT: %d events returned\n", len(events))
	log.I.F("REQ result: %d events of kind %d returned", len(events), eventKind)
}

func testPublishAndQuery(rl *ws.Client, signer *p8k.Signer, eventKind int, numEvents int, timeout time.Duration) {
	log.I.F("Publishing %d events of kind %d...", numEvents, eventKind)

	publishedIDs := make([][]byte, 0, numEvents)
	acceptedCount := 0
	rejectedCount := 0

	// Publish multiple events
	for i := 0; i < numEvents; i++ {
		ev := &event.E{
			CreatedAt: time.Now().Unix() + int64(i), // Slightly different timestamps
			Kind:      uint16(eventKind),
			Tags:      tag.NewS(),
			Content:   []byte(fmt.Sprintf("policy test event %d/%d", i+1, numEvents)),
		}
		if err := ev.Sign(signer); chk.E(err) {
			log.E.F("sign error for event %d: %v", i+1, err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := rl.Publish(ctx, ev)
		cancel()

		if err != nil {
			log.W.F("Event %d/%d rejected: %v", i+1, numEvents, err)
			rejectedCount++
		} else {
			log.I.F("Event %d/%d published successfully (id: %x...)", i+1, numEvents, ev.ID[:8])
			publishedIDs = append(publishedIDs, ev.ID)
			acceptedCount++
		}
	}

	fmt.Printf("PUBLISH: %d accepted, %d rejected out of %d total\n", acceptedCount, rejectedCount, numEvents)

	if acceptedCount == 0 {
		fmt.Println("No events were accepted, skipping query test")
		return
	}

	// Wait a moment for events to be stored
	time.Sleep(500 * time.Millisecond)

	// Now query for events of this kind
	log.I.F("Querying for events of kind %d...", eventKind)

	limit := uint(100)
	f := &filter.F{
		Kinds: kind.FromIntSlice([]int{eventKind}),
		Limit: &limit,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	events, err := rl.QuerySync(ctx, f)
	if chk.E(err) {
		log.E.F("query error: %v", err)
		fmt.Println("QUERY ERROR:", err)
		return
	}

	log.I.F("Query returned %d events", len(events))

	// Check if we got our published events back
	foundCount := 0
	for _, pubID := range publishedIDs {
		found := false
		for _, ev := range events {
			if string(ev.ID) == string(pubID) {
				found = true
				break
			}
		}
		if found {
			foundCount++
		}
	}

	fmt.Printf("QUERY: found %d/%d published events (total returned: %d)\n", foundCount, len(publishedIDs), len(events))

	if foundCount == len(publishedIDs) {
		fmt.Println("SUCCESS: All published events were retrieved")
	} else if foundCount > 0 {
		fmt.Printf("PARTIAL: Only %d/%d events retrieved (some filtered by read policy?)\n", foundCount, len(publishedIDs))
	} else {
		fmt.Println("FAILURE: None of the published events were retrieved (read policy blocked?)")
	}
}
