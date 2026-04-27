// Package directory implements the distributed directory consensus protocol
// as defined in NIP-XX for Nostr relay operators.
//
// # Overview
//
// This package provides complete message encoding, validation, and helper
// functions for implementing the distributed directory consensus protocol.
// The protocol enables Nostr relay operators to form trusted consortiums
// that automatically synchronize essential identity-related events while
// maintaining decentralization and Byzantine fault tolerance.
//
// # Event Kinds
//
// The protocol defines six new event kinds:
//
//   - 39100: Relay Identity Announcement - Announces relay participation
//   - 39101: Trust Act - Creates trust relationships between relays
//   - 39102: Group Tag Act - Attests to arbitrary string values
//   - 39103: Public Key Advertisement - Advertises HD-derived keys
//   - 39104: Directory Event Replication Request - Requests event replication
//   - 39105: Directory Event Replication Response - Responds to replication requests
//
// # Directory Events
//
// The following existing event kinds are considered "directory events" and
// are automatically replicated among consortium members:
//
//   - Kind 0: User Metadata
//   - Kind 3: Follow Lists
//   - Kind 5: Event Deletion Requests
//   - Kind 1984: Reporting
//   - Kind 10002: Relay List Metadata
//   - Kind 10000: Mute Lists
//   - Kind 10050: DM Relay Lists
//
// # Basic Usage
//
// ## Creating a Relay Identity Announcement
//
//	pubkey := []byte{...} // 32-byte relay identity key
//	announcement, err := directory.NewRelayIdentityAnnouncement(
//		pubkey,
//		"relay.example.com",           // name
//		"A community relay",           // description
//		"admin@example.com",           // contact
//		"wss://relay.example.com",     // relay URL
//		"abc123...",                   // signing key (hex)
//		"def456...",                   // encryption key (hex)
//		"1",                           // version
//		"https://relay.example.com/.well-known/nostr.json", // NIP-11 URL
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// ## Creating a Trust Act
//
//	act, err := directory.NewTrustAct(
//		pubkey,
//		"target_relay_pubkey_hex",     // target relay
//		directory.TrustLevelHigh,      // trust level
//		"wss://target.relay.com",      // target URL
//		nil,                           // no expiry
//		directory.TrustReasonManual,   // manual trust
//		[]uint16{1, 6, 7},            // additional kinds to replicate
//		nil,                           // no identity tag
//	)
//
// ## Creating a Public Key Advertisement
//
//	validFrom := time.Now()
//	validUntil := validFrom.Add(30 * 24 * time.Hour) // 30 days
//
//	keyAd, err := directory.NewPublicKeyAdvertisement(
//		pubkey,
//		"signing-key-001",                    // key ID
//		"fedcba9876543210...",               // public key (hex)
//		directory.KeyPurposeSigning,         // purpose
//		validFrom,                           // valid from
//		validUntil,                          // valid until
//		"secp256k1",                         // algorithm
//		"m/39103'/1237'/0'/0/1",            // derivation path
//		1,                                   // key index
//		nil,                                 // no identity tag
//	)
//
// # Identity Tags
//
// Identity tags (I tags) provide npub-encoded identities with proof-of-control
// signatures. They bind an identity to a specific delegate key, preventing
// unauthorized use.
//
// ## Creating Identity Tags
//
//	// Create identity tag builder with private key
//	identityPrivkey := []byte{...} // 32-byte private key
//	builder, err := directory.NewIdentityTagBuilder(identityPrivkey)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create signed identity tag for delegate key
//	delegatePubkey := []byte{...} // 32-byte delegate public key
//	identityTag, err := builder.CreateIdentityTag(delegatePubkey)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Use in trust act
//	act, err := directory.NewTrustAct(
//		pubkey,
//		"target_relay_pubkey_hex",
//		directory.TrustLevelHigh,
//		"wss://target.relay.com",
//		nil,
//		directory.TrustReasonManual,
//		[]uint16{1, 6, 7},
//		identityTag, // Include identity tag
//	)
//
// # Validation
//
// All message types include comprehensive validation:
//
//	// Validate a parsed event
//	if err := announcement.Validate(); err != nil {
//		log.Printf("Invalid announcement: %v", err)
//		return
//	}
//
//	// Validate any consortium event
//	if err := directory.ValidateConsortiumEvent(event); err != nil {
//		log.Printf("Invalid consortium event: %v", err)
//		return
//	}
//
//	// Verify NIP-11 binding
//	valid, err := directory.ValidateRelayIdentityBinding(
//		announcement,
//		nip11Pubkey,
//		nip11Nonce,
//		nip11Sig,
//		relayAddress,
//	)
//	if err != nil || !valid {
//		log.Printf("Invalid relay identity binding")
//		return
//	}
//
// # Trust Calculation
//
// The package provides utilities for calculating trust relationships:
//
//	// Create trust calculator
//	calculator := directory.NewTrustCalculator()
//
//	// Add trust acts
//	calculator.AddAct(act1)
//	calculator.AddAct(act2)
//
//	// Get direct trust level
//	level := calculator.GetTrustLevel("relay_pubkey_hex")
//
//	// Calculate inherited trust
//	inheritedLevel := calculator.CalculateInheritedTrust(
//		"from_relay_pubkey",
//		"to_relay_pubkey",
//	)
//
// # Replication Filtering
//
// Determine which events should be replicated to which relays:
//
//	// Create replication filter
//	filter := directory.NewReplicationFilter(calculator)
//	filter.AddTrustAct(act)
//
//	// Check if event should be replicated
//	shouldReplicate := filter.ShouldReplicate(event, "target_relay_pubkey")
//
//	// Get all replication targets for an event
//	targets := filter.GetReplicationTargets(event)
//
// # Event Batching
//
// Batch events for efficient replication:
//
//	// Create event batcher
//	batcher := directory.NewEventBatcher(100) // max 100 events per batch
//
//	// Add events to batches
//	batcher.AddEvent("wss://relay1.com", event1)
//	batcher.AddEvent("wss://relay1.com", event2)
//	batcher.AddEvent("wss://relay2.com", event3)
//
//	// Check if batch is full
//	if batcher.IsBatchFull("wss://relay1.com") {
//		batch := batcher.FlushBatch("wss://relay1.com")
//		// Send batch for replication
//	}
//
// # Replication Requests and Responses
//
// ## Creating Replication Requests
//
//	requestID, err := directory.GenerateRequestID()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	request, err := directory.NewDirectoryEventReplicationRequest(
//		pubkey,
//		requestID,
//		"wss://target.relay.com",
//		[]*event.E{event1, event2, event3},
//	)
//
// ## Creating Replication Responses
//
//	// Success response
//	results := []*directory.EventResult{
//		directory.CreateEventResult("event_id_1", true, ""),
//		directory.CreateEventResult("event_id_2", false, "duplicate event"),
//	}
//
//	response, err := directory.CreateSuccessResponse(
//		pubkey,
//		requestID,
//		"wss://source.relay.com",
//		results,
//	)
//
//	// Error response
//	errorResponse, err := directory.CreateErrorResponse(
//		pubkey,
//		requestID,
//		"wss://source.relay.com",
//		"relay temporarily unavailable",
//	)
//
// # Key Management
//
// The protocol uses BIP32 HD key derivation for deterministic key generation:
//
//	// Create key pool manager
//	masterSeed := []byte{...} // BIP39 seed
//	manager := directory.NewKeyPoolManager(masterSeed, 0) // identity index 0
//
//	// Generate derivation paths
//	signingPath := manager.GenerateDerivationPath(directory.KeyPurposeSigning, 5)
//	// Returns: "m/39103'/1237'/0'/0/5"
//
//	encryptionPath := manager.GenerateDerivationPath(directory.KeyPurposeEncryption, 3)
//	// Returns: "m/39103'/1237'/0'/1/3"
//
//	// Track key usage
//	nextIndex := manager.GetNextKeyIndex(directory.KeyPurposeSigning)
//	manager.SetKeyIndex(directory.KeyPurposeSigning, 10) // Skip to index 10
//
// # Error Handling
//
// All functions return detailed errors using the errorf package:
//
//	announcement, err := directory.ParseRelayIdentityAnnouncement(event)
//	if err != nil {
//		// Handle specific error types
//		switch {
//		case strings.Contains(err.Error(), "invalid event kind"):
//			log.Printf("Wrong event kind: %v", err)
//		case strings.Contains(err.Error(), "missing"):
//			log.Printf("Missing required field: %v", err)
//		default:
//			log.Printf("Parse error: %v", err)
//		}
//		return
//	}
//
// # Security Considerations
//
// The package implements several security measures:
//
//   - All events must have valid signatures
//   - Identity tags prevent unauthorized identity use
//   - NIP-11 binding prevents relay impersonation
//   - Timestamp validation prevents replay attacks
//   - Content size limits prevent DoS attacks
//   - Nonce validation ensures cryptographic security
//
// # Protocol Constants
//
// Important protocol constants:
//
//   - MaxKeyDelegations: 512 unused key delegations per identity
//   - KeyExpirationDays: 30 days for unused key delegations
//   - MinNonceSize: 16 bytes minimum for nonces
//   - MaxContentLength: 65536 bytes maximum for event content
//
// # Integration Example
//
// Complete example of implementing consortium membership:
//
//	package main
//
//	import (
//		"log"
//		"time"
//
//		"git.smesh.lol/orly/pkg/protocol/directory"
//	)
//
//	func main() {
//		// Generate relay identity key
//		relayPrivkey := []byte{...} // 32 bytes
//		relayPubkey := schnorr.PubkeyFromSeckey(relayPrivkey)
//
//		// Create relay identity announcement
//		announcement, err := directory.NewRelayIdentityAnnouncement(
//			relayPubkey,
//			"my-relay.com",
//			"My Community Relay",
//			"admin@my-relay.com",
//			"wss://my-relay.com",
//			hex.EncodeToString(signingPubkey),
//			hex.EncodeToString(encryptionPubkey),
//			"1",
//			"https://my-relay.com/.well-known/nostr.json",
//		)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// Sign and publish announcement
//		if err := announcement.Event.Sign(relayPrivkey); err != nil {
//			log.Fatal(err)
//		}
//
//		// Create trust act for another relay
//		act, err := directory.NewTrustAct(
//			relayPubkey,
//			"trusted_relay_pubkey_hex",
//			directory.TrustLevelHigh,
//			"wss://trusted-relay.com",
//			nil, // no expiry
//			directory.TrustReasonManual,
//			[]uint16{1, 6, 7}, // replicate text notes, reposts, reactions
//			nil, // no identity tag
//		)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// Sign and publish act
//		if err := act.Event.Sign(relayPrivkey); err != nil {
//			log.Fatal(err)
//		}
//
//		// Set up replication filter
//		calculator := directory.NewTrustCalculator()
//		calculator.AddAct(act)
//
//		filter := directory.NewReplicationFilter(calculator)
//		filter.AddTrustAct(act)
//
//		// When receiving events, check if they should be replicated
//		for event := range eventChannel {
//			targets := filter.GetReplicationTargets(event)
//			for _, target := range targets {
//				// Replicate event to target relay
//				replicateEvent(event, target)
//			}
//		}
//	}
//
// For more detailed examples and advanced usage patterns, see the test files
// and the reference implementation in the main relay codebase.
package directory
