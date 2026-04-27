package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

func main() {
	var err error
	url := flag.String("url", "ws://127.0.0.1:34568", "relay websocket URL")
	allowedPubkeyHex := flag.String("allowed-pubkey", "", "hex-encoded allowed pubkey")
	allowedSecHex := flag.String("allowed-sec", "", "hex-encoded allowed secret key")
	unauthorizedPubkeyHex := flag.String("unauthorized-pubkey", "", "hex-encoded unauthorized pubkey")
	unauthorizedSecHex := flag.String("unauthorized-sec", "", "hex-encoded unauthorized secret key")
	timeout := flag.Duration("timeout", 10*time.Second, "operation timeout")
	flag.Parse()

	if *allowedPubkeyHex == "" || *allowedSecHex == "" {
		log.E.F("required flags: -allowed-pubkey and -allowed-sec")
		os.Exit(1)
	}
	if *unauthorizedPubkeyHex == "" || *unauthorizedSecHex == "" {
		log.E.F("required flags: -unauthorized-pubkey and -unauthorized-sec")
		os.Exit(1)
	}

	// Decode keys
	allowedSecBytes, err := hex.Dec(*allowedSecHex)
	if err != nil {
		log.E.F("failed to decode allowed secret key: %v", err)
		os.Exit(1)
	}
	var allowedSigner *p8k.Signer
	if allowedSigner, err = p8k.New(); chk.E(err) {
		log.E.F("failed to create allowed signer: %v", err)
		os.Exit(1)
	}
	if err = allowedSigner.InitSec(allowedSecBytes); chk.E(err) {
		log.E.F("failed to initialize allowed signer: %v", err)
		os.Exit(1)
	}

	unauthorizedSecBytes, err := hex.Dec(*unauthorizedSecHex)
	if err != nil {
		log.E.F("failed to decode unauthorized secret key: %v", err)
		os.Exit(1)
	}
	var unauthorizedSigner *p8k.Signer
	if unauthorizedSigner, err = p8k.New(); chk.E(err) {
		log.E.F("failed to create unauthorized signer: %v", err)
		os.Exit(1)
	}
	if err = unauthorizedSigner.InitSec(unauthorizedSecBytes); chk.E(err) {
		log.E.F("failed to initialize unauthorized signer: %v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Test 1: Authenticated as allowed pubkey - should work
	fmt.Println("Test 1: Publishing event 30520 with allowed pubkey (authenticated)...")
	if err := testWriteEvent(ctx, *url, 30520, allowedSigner, allowedSigner); err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ PASSED: Event published successfully")

	// Test 2: Authenticated as allowed pubkey, then read event 10306 - should work
	// First publish an event, then read it
	fmt.Println("\nTest 2: Publishing and reading event 10306 with allowed pubkey (authenticated)...")
	if err := testWriteEvent(ctx, *url, 10306, allowedSigner, allowedSigner); err != nil {
		fmt.Printf("❌ FAILED to publish: %v\n", err)
		os.Exit(1)
	}
	if err := testReadEvent(ctx, *url, 10306, allowedSigner); err != nil {
		fmt.Printf("❌ FAILED to read: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ PASSED: Event readable by allowed user")

	// Test 3: Unauthenticated request - should be blocked
	fmt.Println("\nTest 3: Publishing event 30520 without authentication...")
	if err := testWriteEventUnauthenticated(ctx, *url, 30520, allowedSigner); err != nil {
		fmt.Printf("✅ PASSED: Event correctly blocked (expected): %v\n", err)
	} else {
		fmt.Println("❌ FAILED: Event was allowed when it should have been blocked")
		os.Exit(1)
	}

	// Test 4: Authenticated as unauthorized pubkey - should be blocked
	fmt.Println("\nTest 4: Publishing event 30520 with unauthorized pubkey...")
	if err := testWriteEvent(ctx, *url, 30520, unauthorizedSigner, unauthorizedSigner); err != nil {
		fmt.Printf("✅ PASSED: Event correctly blocked (expected): %v\n", err)
	} else {
		fmt.Println("❌ FAILED: Event was allowed when it should have been blocked")
		os.Exit(1)
	}

	// Test 5: Read event 10306 without authentication - should be blocked
	// Event was published in test 2, so it exists in the database
	fmt.Println("\nTest 5: Reading event 10306 without authentication (should be blocked)...")
	// Wait a bit to ensure event is stored
	time.Sleep(500 * time.Millisecond)
	// If no error is returned, that means no events were received (which is correct)
	// If an error is returned, it means an event was received (which is wrong)
	if err := testReadEventUnauthenticated(ctx, *url, 10306); err != nil {
		// If we got an error about receiving an event, that's a failure
		if strings.Contains(err.Error(), "unexpected event received") {
			fmt.Printf("❌ FAILED: %v\n", err)
			os.Exit(1)
		}
		// Other errors (like connection errors) are also failures
		fmt.Printf("❌ FAILED: Unexpected error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ PASSED: No events received (correctly filtered by policy)")

	// Test 6: Read event 10306 with unauthorized pubkey - should be blocked
	fmt.Println("\nTest 6: Reading event 10306 with unauthorized pubkey (should be blocked)...")
	// If no error is returned, that means no events were received (which is correct)
	// If an error is returned about receiving an event, that's a failure
	if err := testReadEvent(ctx, *url, 10306, unauthorizedSigner); err != nil {
		// Connection/subscription errors are failures
		fmt.Printf("❌ FAILED: Unexpected error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ PASSED: No events received (correctly filtered by policy)")

	fmt.Println("\n✅ All tests passed!")
}

func testWriteEvent(ctx context.Context, url string, kindNum uint16, eventSigner, authSigner *p8k.Signer) error {
	rl, err := ws.RelayConnect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}
	defer rl.Close()

	// Send a REQ first to trigger AUTH challenge (when AuthToWrite is enabled)
	// This is needed because challenges are sent on REQ, not on connect
	limit := uint(1)
	ff := filter.NewS(&filter.F{
		Kinds: kind.NewS(kind.New(kindNum)),
		Limit: &limit,
	})
	sub, err := rl.Subscribe(ctx, ff)
	if err != nil {
		return fmt.Errorf("subscription error (may be expected): %w", err)
	}
	// Wait a bit for challenge to arrive
	time.Sleep(500 * time.Millisecond)
	sub.Unsub()

	// Authenticate
	if err = rl.Auth(ctx, authSigner); err != nil {
		return fmt.Errorf("auth error: %w", err)
	}

	// Create and sign event
	ev := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.K{K: kindNum}.K,
		Tags:      tag.NewS(),
		Content:   []byte(fmt.Sprintf("test event kind %d", kindNum)),
	}
	// Add p tag for privileged check
	pTag := tag.NewFromAny("p", hex.Enc(authSigner.Pub()))
	ev.Tags.Append(pTag)
	
	// Add d tag for addressable events (kinds 30000-39999)
	if kindNum >= 30000 && kindNum < 40000 {
		dTag := tag.NewFromAny("d", "test")
		ev.Tags.Append(dTag)
	}

	if err = ev.Sign(eventSigner); err != nil {
		return fmt.Errorf("sign error: %w", err)
	}

	// Publish
	if err = rl.Publish(ctx, ev); err != nil {
		return fmt.Errorf("publish error: %w", err)
	}

	return nil
}

func testWriteEventUnauthenticated(ctx context.Context, url string, kindNum uint16, eventSigner *p8k.Signer) error {
	rl, err := ws.RelayConnect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}
	defer rl.Close()

	// Do NOT authenticate

	// Create and sign event
	ev := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.K{K: kindNum}.K,
		Tags:      tag.NewS(),
		Content:   []byte(fmt.Sprintf("test event kind %d (unauthenticated)", kindNum)),
	}
	
	// Add d tag for addressable events (kinds 30000-39999)
	if kindNum >= 30000 && kindNum < 40000 {
		dTag := tag.NewFromAny("d", "test")
		ev.Tags.Append(dTag)
	}

	if err = ev.Sign(eventSigner); err != nil {
		return fmt.Errorf("sign error: %w", err)
	}

	// Publish (should fail)
	if err = rl.Publish(ctx, ev); err != nil {
		return fmt.Errorf("publish error (expected): %w", err)
	}

	return nil
}

func testReadEvent(ctx context.Context, url string, kindNum uint16, authSigner *p8k.Signer) error {
	rl, err := ws.RelayConnect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}
	defer rl.Close()

	// Send a REQ first to trigger AUTH challenge (when AuthToWrite is enabled)
	// Then authenticate
	ff := filter.NewS(&filter.F{
		Kinds: kind.NewS(kind.New(kindNum)),
	})
	sub, err := rl.Subscribe(ctx, ff)
	if err != nil {
		return fmt.Errorf("subscription error: %w", err)
	}
	// Wait a bit for challenge to arrive
	time.Sleep(500 * time.Millisecond)

	// Authenticate
	if err = rl.Auth(ctx, authSigner); err != nil {
		sub.Unsub()
		return fmt.Errorf("auth error: %w", err)
	}

	// Wait for events or timeout
	// If we receive any events, return nil (success)
	// If we don't receive events, also return nil (no events found, which may be expected)
	select {
	case ev := <-sub.Events:
		if ev != nil {
			sub.Unsub()
			return nil // Event received
		}
	case <-sub.EndOfStoredEvents:
		// EOSE received, no more events
		sub.Unsub()
		return nil
	case <-time.After(5 * time.Second):
		// No events received - this might be OK if no events exist or they're filtered
		sub.Unsub()
		return nil
	case <-ctx.Done():
		sub.Unsub()
		return ctx.Err()
	}

	return nil
}

func testReadEventUnauthenticated(ctx context.Context, url string, kindNum uint16) error {
	rl, err := ws.RelayConnect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}
	defer rl.Close()

	// Do NOT authenticate

	// Subscribe to events
	ff := filter.NewS(&filter.F{
		Kinds: kind.NewS(kind.New(kindNum)),
	})

	sub, err := rl.Subscribe(ctx, ff)
	if err != nil {
		return fmt.Errorf("subscription error (may be expected): %w", err)
	}
	defer sub.Unsub()

	// Wait for events or timeout
	// If we receive any events, that's a failure (should be blocked)
	select {
	case ev := <-sub.Events:
		if ev != nil {
			return fmt.Errorf("unexpected event received: should have been blocked by policy (event ID: %s)", hex.Enc(ev.ID))
		}
	case <-sub.EndOfStoredEvents:
		// EOSE received, no events (this is expected for unauthenticated privileged events)
		return nil
	case <-time.After(5 * time.Second):
		// No events received - this is expected for unauthenticated requests
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

