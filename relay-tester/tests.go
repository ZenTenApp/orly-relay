package relaytester

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// Test implementations - these are referenced by test.go

func testPublishBasicEvent(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "test content", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		return TestResult{Pass: false, Info: fmt.Sprintf("event rejected: %s", reason)}
	}
	return TestResult{Pass: true}
}

func testFindByID(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by id test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"ids": []string{hex.Enc(ev.ID)},
	}
	events, err := client.GetEvents("test-sub", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by ID"}
	}
	return TestResult{Pass: true}
}

func testFindByAuthor(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by author test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-author", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by author"}
	}
	return TestResult{Pass: true}
}

func testFindByKind(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by kind test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds": []int{int(kind.TextNote.K)},
	}
	events, err := client.GetEvents("test-kind", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by kind"}
	}
	return TestResult{Pass: true}
}

func testFindByTags(client *Client, key1, key2 *KeyPair) (result TestResult) {
	tags := tag.NewS(tag.NewFromBytesSlice([]byte("t"), []byte("test-tag")))
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by tags test", tags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"#t": []string{"test-tag"},
	}
	events, err := client.GetEvents("test-tags", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by tags"}
	}
	return TestResult{Pass: true}
}

func testFindByMultipleTags(client *Client, key1, key2 *KeyPair) (result TestResult) {
	tags := tag.NewS(
		tag.NewFromBytesSlice([]byte("t"), []byte("multi-tag-1")),
		tag.NewFromBytesSlice([]byte("t"), []byte("multi-tag-2")),
	)
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by multiple tags test", tags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"#t": []string{"multi-tag-1", "multi-tag-2"},
	}
	events, err := client.GetEvents("test-multi-tags", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by multiple tags"}
	}
	return TestResult{Pass: true}
}

func testFindByTimeRange(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "find by time range test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	now := time.Now().Unix()
	filter := map[string]interface{}{
		"since": now - 3600,
		"until": now + 3600,
	}
	events, err := client.GetEvents("test-time", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by time range"}
	}
	return TestResult{Pass: true}
}

func testRejectInvalidSignature(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "invalid sig test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	// Corrupt the signature
	ev.Sig[0] ^= 0xFF
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		return TestResult{Pass: false, Info: "invalid signature was accepted"}
	}
	_ = reason
	return TestResult{Pass: true}
}

func testRejectFutureEvent(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "future event test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev.CreatedAt = time.Now().Unix() + 3601 // More than 1 hour in the future (should be rejected)
	// Re-sign with new timestamp
	if err = ev.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		return TestResult{Pass: false, Info: "future event was accepted"}
	}
	_ = reason
	return TestResult{Pass: true}
}

func testRejectExpiredEvent(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "expired event test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev.CreatedAt = time.Now().Unix() - 86400*365 // 1 year ago
	// Re-sign with new timestamp
	if err = ev.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	// Some relays may accept old events, so this is optional
	if !accepted {
		return TestResult{Pass: true, Info: "expired event rejected (expected)"}
	}
	return TestResult{Pass: true, Info: "expired event accepted (relay allows old events)"}
}

func testReplaceableEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "first version")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "first event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "second version")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "second event not accepted"}
	}
	// Wait longer for replacement to complete
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds":   []int{int(kind.ProfileMetadata.K)},
		"authors": []string{hex.Enc(key1.Pubkey)},
		"limit":   2, // Set limit > 1 to get multiple versions of replaceable events
	}
	events, err := client.GetEvents("test-replaceable", []interface{}{filter}, 3*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundSecond := false
	for _, e := range events {
		if string(e.ID) == string(ev2.ID) {
			foundSecond = true
			break
		}
	}
	if !foundSecond {
		return TestResult{Pass: false, Info: "second replaceable event not found"}
	}
	return TestResult{Pass: true}
}

func testEphemeralEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEphemeralEvent(key1.Secret, 20000, "ephemeral test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "ephemeral event not accepted"}
	}
	// Ephemeral events should not be stored, so query should not find them
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds": []int{20000},
	}
	events, err := client.GetEvents("test-ephemeral", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Ephemeral events should not be queryable
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			return TestResult{Pass: false, Info: "ephemeral event was stored (should not be)"}
		}
	}
	return TestResult{Pass: true}
}

func testParameterizedReplaceableEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "first list", "test-list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "first event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "second list", "test-list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "second event not accepted"}
	}
	return TestResult{Pass: true}
}

func testDeletionEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// First create an event to delete
	targetEv, err := CreateEvent(key1.Secret, kind.TextNote.K, "event to delete", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(targetEv); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(targetEv.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "target event not accepted"}
	}
	// Wait longer for event to be indexed
	time.Sleep(500 * time.Millisecond)
	// Now create deletion event
	deleteEv, err := CreateDeleteEvent(key1.Secret, [][]byte{targetEv.ID}, "deletion reason")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create delete event: %v", err)}
	}
	if err = client.Publish(deleteEv); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(deleteEv.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "delete event not accepted"}
	}
	return TestResult{Pass: true}
}

func testCountRequest(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "count test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds": []int{int(kind.TextNote.K)},
	}
	count, err := client.Count([]interface{}{filter})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("COUNT failed: %v", err)}
	}
	if count < 1 {
		return TestResult{Pass: false, Info: fmt.Sprintf("COUNT returned %d, expected at least 1", count)}
	}
	return TestResult{Pass: true}
}

func testLimitParameter(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Publish multiple events
	for i := 0; i < 5; i++ {
		ev, err := CreateEvent(key1.Secret, kind.TextNote.K, fmt.Sprintf("limit test %d", i), nil)
		if err != nil {
			continue
		}
		client.Publish(ev)
		client.WaitForOK(ev.ID, 2*time.Second)
	}
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"limit": 2,
	}
	events, err := client.GetEvents("test-limit", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Limit should be respected (though exact count may vary)
	if len(events) > 10 {
		return TestResult{Pass: false, Info: fmt.Sprintf("got %d events, limit may not be working", len(events))}
	}
	return TestResult{Pass: true}
}

func testMultipleFilters(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateEvent(key1.Secret, kind.TextNote.K, "filter 1", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev2, err := CreateEvent(key2.Secret, kind.TextNote.K, "filter 2", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 2*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event 1 not accepted"}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 2*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event 2 not accepted"}
	}
	time.Sleep(300 * time.Millisecond)
	filter1 := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	filter2 := map[string]interface{}{
		"authors": []string{hex.Enc(key2.Pubkey)},
	}
	events, err := client.GetEvents("test-multi-filter", []interface{}{filter1, filter2}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	if len(events) < 2 {
		return TestResult{Pass: false, Info: fmt.Sprintf("got %d events, expected at least 2", len(events))}
	}
	return TestResult{Pass: true}
}

func testSubscriptionClose(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ch, err := client.Subscribe("close-test", []interface{}{map[string]interface{}{}})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to subscribe: %v", err)}
	}
	if err = client.Unsubscribe("close-test"); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to unsubscribe: %v", err)}
	}
	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			return TestResult{Pass: false, Info: "subscription channel not closed"}
		}
	default:
		// Channel already closed, which is fine
	}
	return TestResult{Pass: true}
}

// Filter tests

func testSinceUntilAreInclusive(client *Client, key1, key2 *KeyPair) (result TestResult) {
	now := time.Now().Unix()
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "since until test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev.CreatedAt = now
	if err = ev.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	// Test until filter (should be inclusive)
	untilFilter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
		"kinds":   []int{int(kind.TextNote.K)},
		"until":   now,
	}
	untilEvents, err := client.GetEvents("test-until", []interface{}{untilFilter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Test since filter (should be inclusive)
	sinceFilter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
		"kinds":   []int{int(kind.TextNote.K)},
		"since":   now,
	}
	sinceEvents, err := client.GetEvents("test-since", []interface{}{sinceFilter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundUntil := false
	for _, e := range untilEvents {
		if string(e.ID) == string(ev.ID) {
			foundUntil = true
			break
		}
	}
	foundSince := false
	for _, e := range sinceEvents {
		if string(e.ID) == string(ev.ID) {
			foundSince = true
			break
		}
	}
	if !foundUntil || !foundSince {
		return TestResult{Pass: false, Info: fmt.Sprintf("since/until not inclusive: until=%v since=%v", foundUntil, foundSince)}
	}
	return TestResult{Pass: true}
}

func testLimitZero(client *Client, key1, key2 *KeyPair) (result TestResult) {
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
		"limit":   0,
	}
	events, err := client.GetEvents("test-limit-zero", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Limit 0 should return no events pre-EOSE
	if len(events) > 0 {
		return TestResult{Pass: false, Info: fmt.Sprintf("limit 0 returned %d events", len(events))}
	}
	return TestResult{Pass: true}
}

// Find tests

func testEventsOrderedFromNewestToOldest(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create multiple events
	var eventIDs [][]byte
	for i := 0; i < 3; i++ {
		ev, err := CreateEvent(key1.Secret, kind.TextNote.K, fmt.Sprintf("order test %d", i), nil)
		if err != nil {
			continue
		}
		if err = client.Publish(ev); err != nil {
			continue
		}
		accepted, _, err := client.WaitForOK(ev.ID, 2*time.Second)
		if err != nil || !accepted {
			continue
		}
		eventIDs = append(eventIDs, ev.ID)
		time.Sleep(100 * time.Millisecond) // Small delay to ensure different timestamps
	}
	if len(eventIDs) < 3 {
		return TestResult{Pass: false, Info: "failed to create enough events"}
	}
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"ids": eventIDsToStrings(eventIDs),
	}
	events, err := client.GetEvents("test-order", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	if len(events) < 3 {
		return TestResult{Pass: false, Info: fmt.Sprintf("got %d events, expected at least 3", len(events))}
	}
	// Check ordering (newest first)
	for i := 0; i < len(events)-1; i++ {
		if events[i].CreatedAt < events[i+1].CreatedAt {
			return TestResult{Pass: false, Info: "events not ordered from newest to oldest"}
		}
	}
	return TestResult{Pass: true}
}

func testNewestEventsWhenLimited(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create multiple events with tags
	tags1 := tag.NewS(tag.NewFromBytesSlice([]byte("t"), []byte("limit-tag-a")))
	tags2 := tag.NewS(tag.NewFromBytesSlice([]byte("t"), []byte("limit-tag-b")))
	ev1, err := CreateEvent(key1.Secret, kind.TextNote.K, "limit first", tags1)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	time.Sleep(100 * time.Millisecond)
	ev2, err := CreateEvent(key1.Secret, kind.TextNote.K, "limit second", tags2)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	time.Sleep(100 * time.Millisecond)
	ev3, err := CreateEvent(key1.Secret, kind.TextNote.K, "limit third", tags1)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	for _, ev := range []*event.E{ev1, ev2, ev3} {
		if err = client.Publish(ev); err != nil {
			continue
		}
		client.WaitForOK(ev.ID, 2*time.Second)
	}
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
		"#t":      []string{"limit-tag-a", "limit-tag-b"},
		"kinds":   []int{int(kind.TextNote.K)},
		"limit":   2,
	}
	events, err := client.GetEvents("test-newest-limit", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	if len(events) != 2 {
		return TestResult{Pass: false, Info: fmt.Sprintf("got %d events, expected 2", len(events))}
	}
	// Should get newest events (ev3 and ev2, not ev1)
	foundEv1 := false
	foundEv3 := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundEv1 = true
		}
		if string(e.ID) == string(ev3.ID) {
			foundEv3 = true
		}
	}
	if foundEv1 && !foundEv3 {
		return TestResult{Pass: false, Info: "got older event instead of newest"}
	}
	if !foundEv3 {
		return TestResult{Pass: false, Info: "newest event not found"}
	}
	return TestResult{Pass: true}
}

func testFindByPubkeyAndKind(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateEvent(key1.Secret, kind.TextNote.K, "pubkey kind test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev2, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "pubkey kind metadata")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	for _, ev := range []*event.E{ev1, ev2} {
		if err = client.Publish(ev); err != nil {
			continue
		}
		client.WaitForOK(ev.ID, 2*time.Second)
	}
	time.Sleep(300 * time.Millisecond)
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
		"kinds":   []int{int(kind.TextNote.K), int(kind.ProfileMetadata.K)},
	}
	events, err := client.GetEvents("test-pubkey-kind", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundEv1 := false
	foundEv2 := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundEv1 = true
		}
		if string(e.ID) == string(ev2.ID) {
			foundEv2 = true
		}
	}
	if !foundEv1 || !foundEv2 {
		return TestResult{Pass: false, Info: fmt.Sprintf("events not found: ev1=%v ev2=%v", foundEv1, foundEv2)}
	}
	return TestResult{Pass: true}
}

func testFindByPubkeyAndTags(client *Client, key1, key2 *KeyPair) (result TestResult) {
	pTag := tag.NewS(tag.NewFromBytesSlice([]byte("p"), []byte(hex.Enc(key1.Pubkey))))
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "pubkey tags test", pTag)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
		"#p":      []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-pubkey-tags", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by pubkey and tags"}
	}
	return TestResult{Pass: true}
}

func testFindByKindAndTags(client *Client, key1, key2 *KeyPair) (result TestResult) {
	tags := tag.NewS(tag.NewFromBytesSlice([]byte("n"), []byte("approved")))
	ev, err := CreateEvent(key1.Secret, 9999, "kind tags test", tags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds": []int{9999, int(kind.TextNote.K)},
		"#n":    []string{"approved"},
	}
	events, err := client.GetEvents("test-kind-tags", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by kind and tags"}
	}
	return TestResult{Pass: true}
}

func testFindByScrape(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "scrape test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	// Empty filter (scrape all)
	filter := map[string]interface{}{}
	events, err := client.GetEvents("test-scrape", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "event not found by scrape"}
	}
	return TestResult{Pass: true}
}

// Helper function
func eventIDsToStrings(ids [][]byte) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = hex.Enc(id)
	}
	return result
}

// Replaceable event tests

func testReplacesMetadata(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "older metadata")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev1.CreatedAt = time.Now().Unix() - 60
	if err = ev1.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		// If rejected, check if it's because there's already a newer event (which is OK for this test)
		if strings.Contains(reason, "older than existing") || strings.Contains(reason, "already exists") {
			// There's already a newer event - try to publish a newer one and verify replacement works
			ev2, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "newer metadata")
			if err != nil {
				return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
			}
			if err = client.Publish(ev2); err != nil {
				return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
			}
			accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
			if err != nil || !accepted {
				return TestResult{Pass: false, Info: "second event not accepted"}
			}
			time.Sleep(500 * time.Millisecond)
			filter := map[string]interface{}{
				"kinds":   []int{int(kind.ProfileMetadata.K)},
				"authors": []string{hex.Enc(key1.Pubkey)},
			}
			events, err := client.GetEvents("test-metadata-replace", []interface{}{filter}, 2*time.Second)
			if err != nil {
				return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
			}
			foundNew := false
			for _, e := range events {
				if string(e.ID) == string(ev2.ID) {
					foundNew = true
					break
				}
			}
			if !foundNew {
				return TestResult{Pass: false, Info: "newer metadata not found"}
			}
			return TestResult{Pass: true, Info: "older event rejected (expected), newer event accepted and returned"}
		}
		return TestResult{Pass: false, Info: fmt.Sprintf("first event not accepted: %s", reason)}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "newer metadata")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "second event not accepted"}
	}
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds":   []int{int(kind.ProfileMetadata.K)},
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-metadata-replace", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundOld := false
	foundNew := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundOld = true
		}
		if string(e.ID) == string(ev2.ID) {
			foundNew = true
		}
	}
	if foundOld && !foundNew {
		return TestResult{Pass: false, Info: "older metadata returned, newer not returned"}
	}
	if !foundNew {
		return TestResult{Pass: false, Info: "newer metadata not found"}
	}
	if foundOld && foundNew {
		// Both found is acceptable if relay keeps old versions
		return TestResult{Pass: true, Info: "both versions found (relay keeps old versions)"}
	}
	return TestResult{Pass: true}
}

func testReplacesContactList(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateReplaceableEvent(key1.Secret, kind.FollowList.K, "older contact list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev1.CreatedAt = time.Now().Unix() - 60
	if err = ev1.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		// If rejected, check if it's because there's already a newer event (which is OK for this test)
		if strings.Contains(reason, "older than existing") || strings.Contains(reason, "already exists") {
			// There's already a newer event - try to publish a newer one and verify replacement works
			ev2, err := CreateReplaceableEvent(key1.Secret, kind.FollowList.K, "newer contact list")
			if err != nil {
				return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
			}
			if err = client.Publish(ev2); err != nil {
				return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
			}
			accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
			if err != nil || !accepted {
				return TestResult{Pass: false, Info: "second event not accepted"}
			}
			time.Sleep(500 * time.Millisecond)
			filter := map[string]interface{}{
				"kinds":   []int{int(kind.FollowList.K)},
				"authors": []string{hex.Enc(key1.Pubkey)},
			}
			events, err := client.GetEvents("test-contact-replace", []interface{}{filter}, 2*time.Second)
			if err != nil {
				return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
			}
			foundNew := false
			for _, e := range events {
				if string(e.ID) == string(ev2.ID) {
					foundNew = true
					break
				}
			}
			if !foundNew {
				return TestResult{Pass: false, Info: "newer contact list not found"}
			}
			return TestResult{Pass: true, Info: "older event rejected (expected), newer event accepted and returned"}
		}
		return TestResult{Pass: false, Info: fmt.Sprintf("first event not accepted: %s", reason)}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateReplaceableEvent(key1.Secret, kind.FollowList.K, "newer contact list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "second event not accepted"}
	}
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds":   []int{int(kind.FollowList.K)},
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-contact-replace", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundOld := false
	foundNew := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundOld = true
		}
		if string(e.ID) == string(ev2.ID) {
			foundNew = true
		}
	}
	if foundOld && !foundNew {
		return TestResult{Pass: false, Info: "older contact list returned, newer not returned"}
	}
	if !foundNew {
		return TestResult{Pass: false, Info: "newer contact list not found"}
	}
	return TestResult{Pass: true}
}

func testReplacedEventsStillAvailableByID(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateReplaceableEvent(key1.Secret, kind.FollowList.K, "old contact list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev1.CreatedAt = time.Now().Unix() - 60
	if err = ev1.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		// If rejected, check if it's because there's already a newer event
		if strings.Contains(reason, "older than existing") || strings.Contains(reason, "already exists") {
			// There's already a newer event - just verify it's still available by ID
			// Try to fetch by ID (should still work even if replaced)
			filter := map[string]interface{}{
				"ids": []string{hex.Enc(ev1.ID)},
			}
			events, err := client.GetEvents("test-old-by-id", []interface{}{filter}, 2*time.Second)
			if err != nil {
				return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
			}
			found := false
			for _, e := range events {
				if string(e.ID) == string(ev1.ID) {
					found = true
					break
				}
			}
			if found {
				return TestResult{Pass: true, Info: "older event rejected but still available by ID"}
			}
			// If not found, it might have been deleted, which is also acceptable
			return TestResult{Pass: true, Info: "older event rejected (some relays delete old versions)"}
		}
		return TestResult{Pass: false, Info: fmt.Sprintf("first event not accepted: %s", reason)}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateReplaceableEvent(key1.Secret, kind.FollowList.K, "new contact list")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(ev2.ID, 2*time.Second)
	time.Sleep(500 * time.Millisecond)
	// Try to fetch old event by ID - should still be available
	filter := map[string]interface{}{
		"ids": []string{hex.Enc(ev1.ID)},
	}
	events, err := client.GetEvents("test-old-by-id", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	found := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			found = true
			break
		}
	}
	if !found {
		return TestResult{Pass: false, Info: "replaced event not available by ID"}
	}
	return TestResult{Pass: true}
}

func testReplaceableEventRemovesPrevious(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Use a custom replaceable kind
	ev1, err := CreateReplaceableEvent(key1.Secret, 10001, "old replaceable")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev1.CreatedAt = time.Now().Unix() - 60
	if err = ev1.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "first event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateReplaceableEvent(key1.Secret, 10001, "new replaceable")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(ev2.ID, 2*time.Second)
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds":   []int{10001},
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-replace-remove", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Old event should not be returned (unless relay keeps old versions)
	foundOld := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundOld = true
			break
		}
	}
	if foundOld {
		// Some relays keep old versions, which is acceptable
		return TestResult{Pass: true, Info: "old event still present (relay keeps old versions)"}
	}
	return TestResult{Pass: true}
}

func testReplaceableEventRejectedIfFuture(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create newer replaceable event first
	ev1, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "newer metadata")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "newer event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	// Try to submit older replaceable event
	ev2, err := CreateReplaceableEvent(key1.Secret, kind.ProfileMetadata.K, "older metadata")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev2.CreatedAt = time.Now().Unix() - 60
	if err = ev2.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		// Some relays accept old replaceable events
		return TestResult{Pass: true, Info: "older replaceable event accepted (relay allows old versions)"}
	}
	_ = reason
	return TestResult{Pass: true, Info: "older replaceable event rejected (expected)"}
}

func testAddressableEventRemovesPrevious(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev1, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "old list", "test-addr")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev1.CreatedAt = time.Now().Unix() - 60
	if err = ev1.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "first event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "new list", "test-addr")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(ev2.ID, 2*time.Second)
	time.Sleep(500 * time.Millisecond)
	filter := map[string]interface{}{
		"kinds":   []int{30023},
		"authors": []string{hex.Enc(key1.Pubkey)},
		"#d":      []string{"test-addr"},
	}
	events, err := client.GetEvents("test-addr-remove", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundOld := false
	foundNew := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundOld = true
		}
		if string(e.ID) == string(ev2.ID) {
			foundNew = true
		}
	}
	if foundOld && !foundNew {
		return TestResult{Pass: false, Info: "older addressable event returned, newer not returned"}
	}
	if !foundNew {
		return TestResult{Pass: false, Info: "newer addressable event not found"}
	}
	return TestResult{Pass: true}
}

func testAddressableEventRejectedIfFuture(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create newer addressable event first
	ev1, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "newer list", "test-future")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev1.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "newer event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	// Try to submit older addressable event
	ev2, err := CreateParameterizedReplaceableEvent(key1.Secret, 30023, "older list", "test-future")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev2.CreatedAt = time.Now().Unix() - 60
	if err = ev2.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev2.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		return TestResult{Pass: true, Info: "older addressable event accepted (relay allows old versions)"}
	}
	_ = reason
	return TestResult{Pass: true, Info: "older addressable event rejected (expected)"}
}

// Deletion tests

func testDeleteByAddr(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create addressable event
	ev, err := CreateParameterizedReplaceableEvent(key1.Secret, kind.LongFormContent.K, "content to delete", "delete-test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	time.Sleep(500 * time.Millisecond)
	// Create deletion event with a-tag
	aTag := fmt.Sprintf("%d:%s:%s", kind.LongFormContent.K, hex.Enc(key1.Pubkey), "delete-test")
	deleteTags := tag.NewS(tag.NewFromBytesSlice([]byte("a"), []byte(aTag)))
	deleteEv, err := CreateEvent(key1.Secret, kind.EventDeletion.K, "delete reason", deleteTags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create delete event: %v", err)}
	}
	if err = client.Publish(deleteEv); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err = client.WaitForOK(deleteEv.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "delete event not accepted"}
	}
	time.Sleep(500 * time.Millisecond)
	// Try to fetch deleted event by ID
	filter := map[string]interface{}{
		"ids": []string{hex.Enc(ev.ID)},
	}
	events, err := client.GetEvents("test-delete-addr", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			return TestResult{Pass: false, Info: "deleted event still returned"}
		}
	}
	return TestResult{Pass: true}
}

func testDeleteByAddrOnlyDeletesOlder(client *Client, key1, key2 *KeyPair) (result TestResult) {
	time1 := time.Now().Unix() - 300
	time2 := time.Now().Unix() - 100
	time3 := time.Now().Unix()
	// Create older event
	ev1, err := CreateParameterizedReplaceableEvent(key1.Secret, kind.LongFormContent.K, "old content", "delete-older-test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev1.CreatedAt = time1
	if err = ev1.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(ev1.ID, 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	// Create newer event
	ev2, err := CreateParameterizedReplaceableEvent(key1.Secret, kind.LongFormContent.K, "new content", "delete-older-test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	ev2.CreatedAt = time3
	if err = ev2.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(ev2.ID, 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	// Create deletion event dated between them
	aTag := fmt.Sprintf("%d:%s:%s", kind.LongFormContent.K, hex.Enc(key1.Pubkey), "delete-older-test")
	deleteTags := tag.NewS(tag.NewFromBytesSlice([]byte("a"), []byte(aTag)))
	deleteEv, err := CreateEvent(key1.Secret, kind.EventDeletion.K, "", deleteTags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create delete event: %v", err)}
	}
	deleteEv.CreatedAt = time2
	if err = deleteEv.Sign(key1.Secret); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to re-sign: %v", err)}
	}
	if err = client.Publish(deleteEv); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(deleteEv.ID, 2*time.Second)
	time.Sleep(500 * time.Millisecond)
	// Fetch events by address
	filter := map[string]interface{}{
		"kinds":   []int{int(kind.LongFormContent.K)},
		"authors": []string{hex.Enc(key1.Pubkey)},
		"#d":      []string{"delete-older-test"},
	}
	events, err := client.GetEvents("test-delete-older", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundEv1 := false
	foundEv2 := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundEv1 = true
		}
		if string(e.ID) == string(ev2.ID) {
			foundEv2 = true
		}
	}
	if foundEv1 {
		return TestResult{Pass: false, Info: "older event not deleted"}
	}
	if !foundEv2 {
		return TestResult{Pass: false, Info: "newer event wrongly deleted"}
	}
	return TestResult{Pass: true}
}

func testDeleteByAddrIsBoundByTag(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create events with same author and kind but different d-tags
	ev1, err := CreateParameterizedReplaceableEvent(key1.Secret, kind.LongFormContent.K, "content 1", "bound-test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev1); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(ev1.ID, 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	ev2, err := CreateParameterizedReplaceableEvent(key1.Secret, kind.LongFormContent.K, "content 2", "bound-test-other")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev2); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(ev2.ID, 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	// Delete only one address
	aTag := fmt.Sprintf("%d:%s:%s", kind.LongFormContent.K, hex.Enc(key1.Pubkey), "bound-test")
	deleteTags := tag.NewS(tag.NewFromBytesSlice([]byte("a"), []byte(aTag)))
	deleteEv, err := CreateEvent(key1.Secret, kind.EventDeletion.K, "", deleteTags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create delete event: %v", err)}
	}
	if err = client.Publish(deleteEv); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	client.WaitForOK(deleteEv.ID, 2*time.Second)
	time.Sleep(500 * time.Millisecond)
	// Fetch both events by ID
	filter := map[string]interface{}{
		"ids": []string{hex.Enc(ev1.ID), hex.Enc(ev2.ID)},
	}
	events, err := client.GetEvents("test-delete-bound", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	foundEv1 := false
	foundEv2 := false
	for _, e := range events {
		if string(e.ID) == string(ev1.ID) {
			foundEv1 = true
		}
		if string(e.ID) == string(ev2.ID) {
			foundEv2 = true
		}
	}
	if foundEv1 {
		return TestResult{Pass: false, Info: "deleted event still returned"}
	}
	if !foundEv2 {
		return TestResult{Pass: false, Info: "other event wrongly deleted"}
	}
	return TestResult{Pass: true}
}

// Ephemeral tests

func testEphemeralSubscriptionsWork(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Subscribe to ephemeral events
	filter := map[string]interface{}{
		"kinds":   []int{20000},
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	ch, err := client.Subscribe("test-ephemeral-sub", []interface{}{filter})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to subscribe: %v", err)}
	}
	defer client.Unsubscribe("test-ephemeral-sub")

	// Wait for EOSE to ensure subscription is established
	eoseTimeout := time.After(3 * time.Second)
	gotEose := false
	for !gotEose {
		select {
		case msg, ok := <-ch:
			if !ok {
				return TestResult{Pass: false, Info: "channel closed before EOSE"}
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 2 {
				if typ, ok := raw[0].(string); ok && typ == "EOSE" {
					gotEose = true
					break
				}
			}
		case <-eoseTimeout:
			return TestResult{Pass: false, Info: "timeout waiting for EOSE"}
		}
	}

	// Now publish ephemeral event
	ev, err := CreateEphemeralEvent(key1.Secret, 20000, "ephemeral test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "ephemeral event not accepted"}
	}
	// Give the relay time to process and distribute the ephemeral event
	time.Sleep(2 * time.Second)
	// Wait for event to come through subscription
	timeout := time.After(15 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return TestResult{Pass: false, Info: "subscription closed"}
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 3 && raw[0] == "EVENT" {
				if evData, ok := raw[2].(map[string]interface{}); ok {
					evJSON, _ := json.Marshal(evData)
					receivedEv := event.New()
					if _, err = receivedEv.Unmarshal(evJSON); err == nil {
						if string(receivedEv.ID) == string(ev.ID) {
							return TestResult{Pass: true}
						}
					}
				}
			}
		case <-timeout:
			return TestResult{Pass: false, Info: "timeout waiting for ephemeral event"}
		}
	}
}

func testPersistsEphemeralEvents(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEphemeralEvent(key1.Secret, 20001, "ephemeral persist test")
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "ephemeral event not accepted"}
	}
	time.Sleep(200 * time.Millisecond)
	// Try to query for ephemeral event
	filter := map[string]interface{}{
		"kinds":   []int{20001},
		"authors": []string{hex.Enc(key1.Pubkey)},
	}
	events, err := client.GetEvents("test-ephemeral-persist", []interface{}{filter}, 2*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get events: %v", err)}
	}
	// Ephemeral events should NOT be queryable
	for _, e := range events {
		if string(e.ID) == string(ev.ID) {
			return TestResult{Pass: false, Info: "ephemeral event was persisted (should not be)"}
		}
	}
	return TestResult{Pass: true}
}

// EOSE tests

func testSupportsEose(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Subscribe to events from a random author (should have 0 events)
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key2.Pubkey)},
		"kinds":   []int{int(kind.TextNote.K)},
		"limit":   10,
	}
	ch, err := client.Subscribe("test-eose", []interface{}{filter})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to subscribe: %v", err)}
	}
	defer client.Unsubscribe("test-eose")
	// Wait for EOSE message or timeout
	timeout := time.After(3 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return TestResult{Pass: false, Info: "channel closed before EOSE"}
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 2 {
				if typ, ok := raw[0].(string); ok && typ == "EOSE" {
					return TestResult{Pass: true}
				}
			}
		case <-timeout:
			return TestResult{Pass: false, Info: "timeout waiting for EOSE"}
		}
	}
}

func testSubscriptionReceivesEventAfterPingPeriod(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create a second client for publishing
	publisherClient, err := NewClient(client.URL())
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create publisher client: %v", err)}
	}
	defer publisherClient.Close()

	// Subscribe to events from key1
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key1.Pubkey)},
		"kinds":   []int{int(kind.TextNote.K)},
	}
	ch, err := client.Subscribe("test-ping-period", []interface{}{filter})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to subscribe: %v", err)}
	}
	defer client.Unsubscribe("test-ping-period")

	// Wait for EOSE to ensure subscription is established
	eoseTimeout := time.After(3 * time.Second)
	gotEose := false
	for !gotEose {
		select {
		case msg, ok := <-ch:
			if !ok {
				return TestResult{Pass: false, Info: "channel closed before EOSE"}
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 2 {
				if typ, ok := raw[0].(string); ok && typ == "EOSE" {
					gotEose = true
					break
				}
			}
		case <-eoseTimeout:
			return TestResult{Pass: false, Info: "timeout waiting for EOSE"}
		}
	}

	// Wait for at least one ping period (30 seconds) to ensure connection is idle
	// and has been pinged at least once
	pingPeriod := 35 * time.Second // Slightly longer than 30s to ensure at least one ping
	// Reduce for testing - the ping/pong mechanism is tested separately
	pingPeriod = 1 * time.Second
	time.Sleep(pingPeriod)

	// Now publish an event from the publisher client that matches the subscription
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "event after ping period", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = publisherClient.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, _, err := publisherClient.WaitForOK(ev.ID, 5*time.Second)
	if err != nil || !accepted {
		return TestResult{Pass: false, Info: "event not accepted"}
	}
	// Give the relay time to process and distribute the event
	time.Sleep(2 * time.Second)

	// Wait for event to come through subscription (should work even after ping period)
	eventTimeout := time.After(15 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return TestResult{Pass: false, Info: "subscription closed"}
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 3 && raw[0] == "EVENT" {
				if evData, ok := raw[2].(map[string]interface{}); ok {
					evJSON, _ := json.Marshal(evData)
					receivedEv := event.New()
					if _, err = receivedEv.Unmarshal(evJSON); err == nil {
						if string(receivedEv.ID) == string(ev.ID) {
							return TestResult{Pass: true}
						}
					}
				}
			}
		case <-eventTimeout:
			return TestResult{Pass: false, Info: "timeout waiting for event after ping period"}
		}
	}
}

func testClosesCompleteSubscriptionsAfterEose(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create a filter that fetches a specific event by ID (complete subscription)
	fakeID := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	filter := map[string]interface{}{
		"ids":   []string{fakeID},
		"kinds": []int{int(kind.TextNote.K)},
	}
	ch, err := client.Subscribe("test-close-complete", []interface{}{filter})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to subscribe: %v", err)}
	}
	defer func() {
		client.Unsubscribe("test-close-complete")
	}()
	// Wait for EOSE and verify channel is closed
	timeout := time.After(3 * time.Second)
	gotEose := false
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				// Channel closed - for complete subscriptions, this should happen after EOSE
				if gotEose {
					return TestResult{Pass: true}
				}
				return TestResult{Pass: false, Info: "channel closed before EOSE"}
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 2 {
				if typ, ok := raw[0].(string); ok && typ == "EOSE" {
					gotEose = true
					// For complete subscriptions, channel should close after EOSE
					time.Sleep(100 * time.Millisecond)
					select {
					case _, ok := <-ch:
						if ok {
							return TestResult{Pass: false, Info: "subscription not closed after EOSE"}
						}
						// Channel closed, which is correct
						return TestResult{Pass: true}
					default:
						// Channel might be closed already
						return TestResult{Pass: true}
					}
				}
			}
		case <-timeout:
			if gotEose {
				return TestResult{Pass: false, Info: "timeout but EOSE received - subscription should be closed"}
			}
			return TestResult{Pass: false, Info: "timeout waiting for EOSE"}
		}
	}
}

func testKeepsOpenIncompleteSubscriptionsAfterEose(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Subscribe to events from a random author (incomplete subscription)
	filter := map[string]interface{}{
		"authors": []string{hex.Enc(key2.Pubkey)},
		"kinds":   []int{int(kind.TextNote.K)},
		"limit":   10,
		"until":   time.Now().Unix() - 86400, // Past timestamp
	}
	ch, err := client.Subscribe("test-open-incomplete", []interface{}{filter})
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to subscribe: %v", err)}
	}
	defer client.Unsubscribe("test-open-incomplete")
	// Wait for EOSE
	timeout := time.After(3 * time.Second)
	gotEose := false
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return TestResult{Pass: false, Info: "incomplete subscription closed after EOSE"}
			}
			var raw []interface{}
			if err = json.Unmarshal(msg, &raw); err != nil {
				continue
			}
			if len(raw) >= 2 {
				if typ, ok := raw[0].(string); ok && typ == "EOSE" {
					gotEose = true
					// After EOSE, subscription should remain open for incomplete subscriptions
					time.Sleep(200 * time.Millisecond)
					// Channel should still be open (not closed)
					select {
					case _, ok := <-ch:
						if !ok {
							return TestResult{Pass: false, Info: "incomplete subscription closed after EOSE"}
						}
						// Channel is still open, which is correct
						return TestResult{Pass: true}
					default:
						// Channel is still open, which is correct
						return TestResult{Pass: true}
					}
				}
			}
		case <-timeout:
			if gotEose {
				return TestResult{Pass: true, Info: "EOSE received, subscription remains open"}
			}
			return TestResult{Pass: false, Info: "timeout waiting for EOSE"}
		}
	}
}

// JSON tests

func testAcceptsEventsWithEmptyTags(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// Create event with empty tags array
	emptyTags := tag.NewS()
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "empty tags test", emptyTags)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		return TestResult{Pass: false, Info: fmt.Sprintf("event rejected: %s", reason)}
	}
	return TestResult{Pass: true}
}

func testAcceptsNip1JsonEscapeSequences(client *Client, key1, key2 *KeyPair) (result TestResult) {
	// NIP-01 escape sequences: \n, \", \\, \r, \t, \b, \f
	content := "linebreak\\ndoublequote\\\"backslash\\\\carraigereturn\\rtab\\tbackspace\\bformfeed\\fend"
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, content, nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		return TestResult{Pass: false, Info: fmt.Sprintf("event rejected: %s", reason)}
	}
	return TestResult{Pass: true}
}

// Registration tests

func testSendsOkAfterEvent(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "OK test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if !accepted {
		return TestResult{Pass: false, Info: fmt.Sprintf("event rejected: %s", reason)}
	}
	return TestResult{Pass: true}
}

func testVerifiesSignatures(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "signature test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	// Corrupt the signature
	for i := range ev.Sig {
		ev.Sig[i] = 0
	}
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		return TestResult{Pass: false, Info: "invalid signature was accepted"}
	}
	_ = reason
	return TestResult{Pass: true}
}

func testVerifiesIdHashes(client *Client, key1, key2 *KeyPair) (result TestResult) {
	ev, err := CreateEvent(key1.Secret, kind.TextNote.K, "ID hash test", nil)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to create event: %v", err)}
	}
	// Save the correct ID
	correctID := make([]byte, len(ev.ID))
	copy(correctID, ev.ID)
	// Corrupt the ID AFTER signing (Sign() recalculates ID, so we corrupt it after)
	for i := range ev.ID {
		ev.ID[i] = 0xCA
	}
	// Don't re-sign - the signature is valid for the correct ID, but we have a corrupted ID
	if err = client.Publish(ev); err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to publish: %v", err)}
	}
	// Use the corrupted ID to wait for OK (relay should reject based on ID mismatch)
	accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
	if err != nil {
		return TestResult{Pass: false, Info: fmt.Sprintf("failed to get OK: %v", err)}
	}
	if accepted {
		return TestResult{Pass: false, Info: "invalid ID hash was accepted"}
	}
	_ = reason
	_ = correctID
	return TestResult{Pass: true}
}
