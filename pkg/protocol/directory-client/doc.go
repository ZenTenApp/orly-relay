// Package directory_client provides a high-level client API for the
// Distributed Directory Consensus Protocol (NIP-XX).
//
// # Overview
//
// This package builds on top of the lower-level directory protocol package
// to provide convenient utilities for:
//
//   - Identity resolution and key delegation tracking
//   - Trust score calculation and aggregation
//   - Replication filtering based on trust relationships
//   - Event collection and filtering
//   - Trust graph construction and analysis
//
// # Architecture
//
// The client library consists of several main components:
//
// **IdentityResolver**: Tracks mappings between delegate keys and primary
// identities, enabling resolution of the actual identity behind any signing
// key. It processes Identity Tags (I tags) from events and maintains a cache
// of delegation relationships.
//
// **TrustCalculator**: Computes aggregate trust scores from multiple trust
// acts using a weighted average approach. Trust levels (high/medium/low) are
// mapped to numeric weights and non-expired acts are combined to produce
// an overall trust score.
//
// **ReplicationFilter**: Uses trust scores to determine which relays should
// be trusted for event replication. It maintains a set of trusted relays
// based on a configurable minimum trust score threshold.
//
// **EventCollector**: Provides convenient methods to extract and parse
// specific types of directory events from a collection.
//
// **TrustGraph**: Represents trust relationships as a directed graph,
// enabling analysis of trust networks and transitive trust paths.
//
// # Thread Safety
//
// All components are thread-safe and can be used concurrently from multiple
// goroutines. Internal state is protected by read-write mutexes.
//
// # Example: Identity Resolution
//
//	resolver := directory_client.NewIdentityResolver()
//
//	// Process events to build identity mappings
//	for _, event := range events {
//		resolver.ProcessEvent(event)
//	}
//
//	// Resolve identity behind a delegate key
//	actualIdentity := resolver.ResolveIdentity(delegateKey)
//
//	// Check delegation status
//	if resolver.IsDelegateKey(pubkey) {
//		tag, _ := resolver.GetIdentityTag(pubkey)
//		fmt.Printf("Delegate %s belongs to identity %s\n",
//			pubkey, tag.Identity)
//	}
//
// # Example: Trust Management
//
//	calculator := directory_client.NewTrustCalculator()
//
//	// Add trust acts
//	for _, event := range trustEvents {
//		if act, err := directory.ParseTrustAct(event); err == nil {
//			calculator.AddAct(act)
//		}
//	}
//
//	// Calculate trust score
//	score := calculator.CalculateTrust(targetPubkey)
//	fmt.Printf("Trust score: %.1f\n", score)
//
// # Example: Replication Filtering
//
//	// Create filter with minimum trust score of 50
//	filter := directory_client.NewReplicationFilter(50)
//
//	// Add trust acts to influence decisions
//	for _, act := range trustActs {
//		filter.AddTrustAct(act)
//	}
//
//	// Check if should replicate from a relay
//	if filter.ShouldReplicate(relayPubkey) {
//		// Proceed with replication
//		events := fetchEventsFromRelay(relayPubkey)
//		// ...
//	}
//
// # Example: Event Collection
//
//	collector := directory_client.NewEventCollector(events)
//
//	// Extract specific event types
//	identities := collector.RelayIdentities()
//	trustActs := collector.TrustActs()
//	keyAds := collector.PublicKeyAdvertisements()
//
//	// Find specific events
//	if identity, found := directory_client.FindRelayIdentity(
//		events, "wss://relay.example.com/"); found {
//		fmt.Printf("Found relay identity: %s\n", identity.RelayURL)
//	}
//
// # Example: Trust Graph Analysis
//
//	graph := directory_client.BuildTrustGraph(events)
//
//	// Find who trusts a specific relay
//	trustedBy := graph.GetTrustedBy(targetPubkey)
//	fmt.Printf("Trusted by %d relays\n", len(trustedBy))
//
//	// Find who a relay trusts
//	targets := graph.GetTrustTargets(sourcePubkey)
//	fmt.Printf("Trusts %d relays\n", len(targets))
//
// # Integration with Directory Protocol
//
// This package is designed to work seamlessly with the lower-level
// directory protocol package (git.smesh.lol/orly/pkg/protocol/directory).
// Use the protocol package for:
//
//   - Parsing individual directory events
//   - Creating new directory events
//   - Validating event structure
//   - Working with event content and tags
//
// Use the client package for:
//
//   - Managing collections of events
//   - Tracking identity relationships
//   - Computing trust metrics
//   - Filtering events for replication
//   - Analyzing trust networks
//
// # Performance Considerations
//
// The IdentityResolver maintains in-memory caches of identity mappings.
// For large numbers of identities and delegates, memory usage will grow
// proportionally. Use ClearCache() if you need to free memory.
//
// The TrustCalculator stores all trust acts in memory. For long-running
// applications processing many trust acts, consider periodically clearing
// expired acts or implementing a sliding window approach.
//
// # Related Documentation
//
// See the NIP-XX specification for details on the protocol:
//
//	docs/NIP-XX-distributed-directory-consensus.md
//
// See the directory protocol package for low-level event handling:
//
//	pkg/protocol/directory/
package directory_client
