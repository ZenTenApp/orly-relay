package neo4j

import (
	"fmt"

	"git.mleku.dev/mleku/nostr/crypto/keys"
)

// Relay identity methods
// We use the marker system to store the relay's private key

const relayIdentityMarkerKey = "relay_identity_secret"

// GetRelayIdentitySecret retrieves the relay's identity secret key
func (n *N) GetRelayIdentitySecret() (skb []byte, err error) {
	return n.GetMarker(relayIdentityMarkerKey)
}

// SetRelayIdentitySecret sets the relay's identity secret key
func (n *N) SetRelayIdentitySecret(skb []byte) error {
	return n.SetMarker(relayIdentityMarkerKey, skb)
}

// GetOrCreateRelayIdentitySecret retrieves or creates the relay identity
func (n *N) GetOrCreateRelayIdentitySecret() (skb []byte, err error) {
	skb, err = n.GetRelayIdentitySecret()
	if err == nil {
		return skb, nil
	}

	// Generate new identity
	skb, err = keys.GenerateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity: %w", err)
	}

	// Store it
	if err = n.SetRelayIdentitySecret(skb); err != nil {
		return nil, fmt.Errorf("failed to store identity: %w", err)
	}

	n.Logger.Infof("generated new relay identity")
	return skb, nil
}
