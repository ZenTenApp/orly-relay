# Directory Client Library

High-level Go client library for the Distributed Directory Consensus Protocol (NIP-XX).

## Overview

This package provides a convenient API for working with directory events, managing identity resolution, tracking key delegations, and computing trust scores. It builds on the lower-level `directory` protocol package.

## Features

- **Identity Resolution**: Track and resolve delegate keys to their primary identities
- **Trust Management**: Calculate aggregate trust scores from multiple trust acts
- **Replication Filtering**: Determine which relays to trust for event replication
- **Event Collection**: Convenient utilities for extracting specific event types
- **Trust Graph**: Build and analyze trust relationship networks
- **Thread-Safe**: All components support concurrent access

## Installation

```go
import "git.smesh.lol/orly/pkg/protocol/directory-client"
```

## Quick Start

### Identity Resolution

```go
// Create an identity resolver
resolver := directory_client.NewIdentityResolver()

// Process events to build identity mappings
for _, event := range events {
    resolver.ProcessEvent(event)
}

// Resolve identity behind a delegate key
actualIdentity := resolver.ResolveIdentity(delegateKey)

// Check if a key is a delegate
if resolver.IsDelegateKey(pubkey) {
    tag, _ := resolver.GetIdentityTag(pubkey)
    fmt.Printf("Delegate belongs to: %s\n", tag.Identity)
}

// Get all delegates for an identity
delegates := resolver.GetDelegatesForIdentity(identityPubkey)

// Filter events by identity (including delegates)
filteredEvents := resolver.FilterEventsByIdentity(events, identityPubkey)
```

### Trust Management

```go
// Create a trust calculator
calculator := directory_client.NewTrustCalculator()

// Add trust acts
for _, event := range trustEvents {
    if act, err := directory.ParseTrustAct(event); err == nil {
        calculator.AddAct(act)
    }
}

// Calculate aggregate trust score (0-100)
score := calculator.CalculateTrust(targetPubkey)

// Get active (non-expired) trust acts
activeActs := calculator.GetActiveTrustActs(targetPubkey)
```

### Replication Filtering

```go
// Create filter with minimum trust score threshold
filter := directory_client.NewReplicationFilter(50)

// Add trust acts
for _, act := range trustActs {
    filter.AddTrustAct(act)
}

// Check if should replicate from a relay
if filter.ShouldReplicate(relayPubkey) {
    // Proceed with replication
}

// Get all trusted relays
trustedRelays := filter.GetTrustedRelays()

// Filter events to only trusted sources
trustedEvents := filter.FilterEvents(events)
```

### Event Collection

```go
// Create event collector
collector := directory_client.NewEventCollector(events)

// Extract specific event types
identities := collector.RelayIdentities()
trustActs := collector.TrustActs()
groupTagActs := collector.GroupTagActs()
keyAds := collector.PublicKeyAdvertisements()
requests := collector.ReplicationRequests()
responses := collector.ReplicationResponses()

// Find specific events
identity, found := directory_client.FindRelayIdentity(events, "wss://relay.example.com/")
trustActs := directory_client.FindTrustActsForRelay(events, targetPubkey)
groupActs := directory_client.FindGroupTagActsByGroup(events, "premium")
```

### Trust Graph Analysis

```go
// Build trust graph from events
graph := directory_client.BuildTrustGraph(events)

// Find who trusts a relay
trustedBy := graph.GetTrustedBy(targetPubkey)
fmt.Printf("Trusted by %d relays\n", len(trustedBy))

// Find who a relay trusts
targets := graph.GetTrustTargets(sourcePubkey)
fmt.Printf("Trusts %d relays\n", len(targets))

// Get all trust acts from a source
acts := graph.GetTrustActs(sourcePubkey)
```

## API Reference

### IdentityResolver

Manages identity resolution and key delegation tracking.

**Methods:**
- `NewIdentityResolver() *IdentityResolver` - Create new instance
- `ProcessEvent(ev *event.E)` - Process event to extract identity info
- `ResolveIdentity(pubkey string) string` - Resolve delegate to identity
- `ResolveEventIdentity(ev *event.E) string` - Resolve event's identity
- `IsDelegateKey(pubkey string) bool` - Check if key is a delegate
- `IsIdentityKey(pubkey string) bool` - Check if key has delegates
- `GetDelegatesForIdentity(identity string) []string` - Get all delegates
- `GetIdentityTag(delegate string) (*IdentityTag, error)` - Get identity tag
- `GetPublicKeyAdvertisements(identity string) []*PublicKeyAdvertisement` - Get key ads
- `FilterEventsByIdentity(events []*event.E, identity string) []*event.E` - Filter events
- `ClearCache()` - Clear all cached mappings
- `GetStats() Stats` - Get statistics

### TrustCalculator

Computes aggregate trust scores from multiple trust acts.

**Methods:**
- `NewTrustCalculator() *TrustCalculator` - Create new instance
- `AddAct(act *TrustAct)` - Add a trust act
- `CalculateTrust(pubkey string) float64` - Calculate trust score (0-100)
- `GetActs(pubkey string) []*TrustAct` - Get all acts for pubkey
- `GetActiveTrustActs(pubkey string) []*TrustAct` - Get non-expired acts
- `Clear()` - Remove all acts
- `GetAllPubkeys() []string` - Get all tracked pubkeys

**Trust Score Weights:**
- High: 100
- Medium: 50
- Low: 25

### ReplicationFilter

Manages replication decisions based on trust scores.

**Methods:**
- `NewReplicationFilter(minTrustScore float64) *ReplicationFilter` - Create new instance
- `AddTrustAct(act *TrustAct)` - Add trust act and update trusted relays
- `ShouldReplicate(pubkey string) bool` - Check if relay is trusted
- `GetTrustedRelays() []string` - Get all trusted relay pubkeys
- `GetTrustScore(pubkey string) float64` - Get trust score for relay
- `SetMinTrustScore(minScore float64)` - Update threshold
- `GetMinTrustScore() float64` - Get current threshold
- `FilterEvents(events []*event.E) []*event.E` - Filter to trusted events

### EventCollector

Utility for collecting specific types of directory events.

**Methods:**
- `NewEventCollector(events []*event.E) *EventCollector` - Create new instance
- `RelayIdentities() []*RelayIdentity` - Get all relay identities
- `TrustActs() []*TrustAct` - Get all trust acts
- `GroupTagActs() []*GroupTagAct` - Get all group tag acts
- `PublicKeyAdvertisements() []*PublicKeyAdvertisement` - Get all key ads
- `ReplicationRequests() []*ReplicationRequest` - Get all requests
- `ReplicationResponses() []*ReplicationResponse` - Get all responses

### Helper Functions

**Event Filtering:**
- `IsDirectoryEvent(ev *event.E) bool` - Check if event is directory event
- `FilterDirectoryEvents(events []*event.E) []*event.E` - Filter to directory events
- `ParseDirectoryEvent(ev *event.E) (interface{}, error)` - Parse any directory event

**Event Finding:**
- `FindRelayIdentity(events, relayURL) (*RelayIdentity, bool)` - Find identity by URL
- `FindTrustActsForRelay(events, targetPubkey) []*TrustAct` - Find trust acts
- `FindGroupTagActsForRelay(events, targetPubkey) []*GroupTagAct` - Find group acts
- `FindGroupTagActsByGroup(events, groupTag) []*GroupTagAct` - Find by group

**URL Utilities:**
- `NormalizeRelayURL(url string) string` - Ensure trailing slash

**Trust Graph:**
- `NewTrustGraph() *TrustGraph` - Create new graph
- `BuildTrustGraph(events []*event.E) *TrustGraph` - Build from events
- `AddTrustAct(act *TrustAct)` - Add trust act to graph
- `GetTrustActs(source string) []*TrustAct` - Get acts from source
- `GetTrustedBy(target string) []string` - Get who trusts target
- `GetTrustTargets(source string) []string` - Get who source trusts

## Thread Safety

All components are thread-safe and can be used concurrently from multiple goroutines. Internal state is protected by read-write mutexes (`sync.RWMutex`).

## Performance Considerations

- **IdentityResolver**: Maintains in-memory caches. Use `ClearCache()` if needed
- **TrustCalculator**: Stores all trust acts in memory. Consider periodic cleanup of expired acts
- **ReplicationFilter**: Minimal memory overhead, recalculates trust on each act addition

## Integration

This package works with:

- **directory protocol package**: For parsing and creating events
- **event package**: For event structures
- **EventStore**: Can be integrated with any event storage system

## Testing

The directory-client package includes comprehensive tests to ensure reliability and correctness:

### Running Tests

```bash
# Run directory-client tests
go test ./pkg/protocol/directory-client

# Run all tests including directory-client
go test ./...

# Run with verbose output
go test -v ./pkg/protocol/directory-client

# Run with race detection
go test -race ./pkg/protocol/directory-client

# Run with coverage
go test -cover ./pkg/protocol/directory-client
```

### Integration Testing

The directory-client is tested as part of the project's integration test suite:

```bash
# Run the full test suite
./scripts/test.sh

# Run specific package tests
go test ./pkg/protocol/...
```

### Test Coverage

The test suite covers:

- **Identity Resolution**: Delegate key mapping, identity resolution, caching
- **Trust Calculation**: Trust score computation, act aggregation, expiration handling
- **Replication Filtering**: Trust threshold filtering, relay selection
- **Event Collection**: Event parsing, filtering by type, collection utilities
- **Trust Graph**: Graph construction, relationship analysis
- **Thread Safety**: Concurrent access patterns, race condition prevention
- **Error Handling**: Invalid events, malformed data, edge cases

### Example Test Usage

```bash
# Test identity resolution functionality
go test -v ./pkg/protocol/directory-client -run TestIdentityResolver

# Test trust calculation
go test -v ./pkg/protocol/directory-client -run TestTrustCalculator

# Test thread safety
go test -race -v ./pkg/protocol/directory-client
```

## Development

### Building and Usage

```bash
# Build the directory-client package
go build ./pkg/protocol/directory-client

# Run example usage
go run -tags=example ./pkg/protocol/directory-client
```

### Code Quality

The directory-client follows Go best practices:

- Comprehensive error handling with custom error types
- Thread-safe concurrent access
- Memory-efficient data structures
- Extensive documentation and examples
- Full test coverage with race detection

### Adding New Features

When adding new functionality:

1. Add unit tests for new components
2. Update existing tests if behavior changes
3. Ensure thread safety for concurrent access
4. Add documentation and examples
5. Update the API reference section

## Related Documentation

- [NIP-XX Cluster Replication](../../docs/NIP-XX-Cluster-Replication.md)
- [Directory Protocol Package](../directory/)

## License

See [LICENSE](../../LICENSE) file.
