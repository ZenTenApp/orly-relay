// Package directory_client provides a client library for the Distributed
// Directory Consensus Protocol (NIP-XX).
//
// This package offers a high-level API for working with directory events,
// managing identity resolution, tracking key delegations, and computing
// trust scores. It builds on the lower-level directory protocol package.
//
// # Basic Usage
//
//	// Create an identity resolver
//	resolver := directory_client.NewIdentityResolver()
//
//	// Parse and track events
//	event := getDirectoryEvent()
//	resolver.ProcessEvent(event)
//
//	// Resolve identity behind a delegate key
//	actualIdentity := resolver.ResolveIdentity(delegateKey)
//
//	// Check if a key is a delegate
//	isDelegate := resolver.IsDelegateKey(pubkey)
//
// # Trust Management
//
//	// Create a trust calculator
//	calculator := directory_client.NewTrustCalculator()
//
//	// Add trust acts
//	trustAct := directory.ParseTrustAct(event)
//	calculator.AddAct(trustAct)
//
//	// Calculate aggregate trust score
//	score := calculator.CalculateTrust(targetPubkey)
//
// # Replication Filtering
//
//	// Create a replication filter
//	filter := directory_client.NewReplicationFilter(50) // min trust score of 50
//
//	// Add trust acts to influence replication decisions
//	filter.AddTrustAct(trustAct)
//
//	// Check if should replicate from a relay
//	if filter.ShouldReplicate(relayPubkey) {
//		// Proceed with replication
//	}
//
// # Event Filtering
//
//	// Filter directory events from a stream
//	events := getEvents()
//	directoryEvents := directory_client.FilterDirectoryEvents(events)
//
//	// Check if an event is a directory event
//	if directory_client.IsDirectoryEvent(event) {
//		// Handle directory event
//	}
package directory_client

import (
	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/protocol/directory"
)

// IsDirectoryEvent checks if an event is a directory consensus event.
func IsDirectoryEvent(ev *event.E) bool {
	if ev == nil {
		return false
	}
	k := uint16(ev.Kind)
	return k == 39100 || k == 39101 || k == 39102 ||
		k == 39103 || k == 39104 || k == 39105
}

// FilterDirectoryEvents filters a slice of events to only directory events.
func FilterDirectoryEvents(events []*event.E) (filtered []*event.E) {
	filtered = make([]*event.E, 0)
	for _, ev := range events {
		if IsDirectoryEvent(ev) {
			filtered = append(filtered, ev)
		}
	}
	return
}

// NormalizeRelayURL ensures a relay URL has the canonical format with trailing slash.
func NormalizeRelayURL(url string) string {
	if url == "" {
		return ""
	}
	if url[len(url)-1] != '/' {
		return url + "/"
	}
	return url
}

// ParseDirectoryEvent parses any directory event based on its kind.
func ParseDirectoryEvent(ev *event.E) (parsed interface{}, err error) {
	if !IsDirectoryEvent(ev) {
		return nil, errorf.E("not a directory event: kind %d", uint16(ev.Kind))
	}

	switch uint16(ev.Kind) {
	case 39100:
		return directory.ParseRelayIdentityAnnouncement(ev)
	case 39101:
		return directory.ParseTrustAct(ev)
	case 39102:
		return directory.ParseGroupTagAct(ev)
	case 39103:
		return directory.ParsePublicKeyAdvertisement(ev)
	case 39104:
		return directory.ParseDirectoryEventReplicationRequest(ev)
	case 39105:
		return directory.ParseDirectoryEventReplicationResponse(ev)
	default:
		return nil, errorf.E("unknown directory event kind: %d", uint16(ev.Kind))
	}
}
