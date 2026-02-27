// Package marmot implements the Marmot protocol (MLS-based E2E encrypted
// messaging) for Nostr. It wraps github.com/emersion/go-mls to provide
// 1:1 DM groups using NIP-EE event kinds.
//
// Event kinds:
//   - 443:  Key packages (MIP-00)
//   - 445:  Group messages (MIP-03)
//   - 1059: Gift-wrapped welcome events (MIP-02)
package marmot

// Nostr event kinds used by the Marmot protocol.
const (
	// KindKeyPackage is published by users to advertise their MLS key
	// material. Other users fetch this to create DM groups.
	KindKeyPackage uint16 = 443

	// KindGroupMessage carries MLS-encrypted application messages within
	// an established group.
	KindGroupMessage uint16 = 445

	// KindGiftWrap wraps a Welcome message so only the intended recipient
	// can decrypt it. Uses NIP-44 encryption to the recipient's pubkey.
	KindGiftWrap uint16 = 1059
)
