package nrc

import (
	"errors"
	"net/url"

	"git.mleku.dev/mleku/nostr/crypto/encryption"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"lol.mleku.dev/chk"
)

// AuthMode defines the authentication mode for NRC connections.
type AuthMode int

const (
	// AuthModeSecret uses a shared secret for authentication.
	AuthModeSecret AuthMode = iota
	// AuthModeCAT uses Cashu Access Tokens for authentication.
	AuthModeCAT
)

// ConnectionURI represents a parsed nostr+relayconnect:// URI.
type ConnectionURI struct {
	// RelayPubkey is the public key of the private relay (32 bytes).
	RelayPubkey []byte
	// RendezvousRelay is the WebSocket URL of the public relay.
	RendezvousRelay string
	// AuthMode indicates whether to use secret or CAT authentication.
	AuthMode AuthMode
	// DeviceName is an optional human-readable device identifier.
	DeviceName string

	// Secret-based authentication fields
	clientSecretKey signer.I
	conversationKey []byte

	// CAT-based authentication fields
	MintURL string
}

// GetClientSigner returns the signer derived from the secret (secret-based auth only).
func (c *ConnectionURI) GetClientSigner() signer.I {
	return c.clientSecretKey
}

// GetConversationKey returns the NIP-44 conversation key (secret-based auth only).
func (c *ConnectionURI) GetConversationKey() []byte {
	return c.conversationKey
}

// ParseConnectionURI parses a nostr+relayconnect:// URI.
//
// Secret-based URI format:
//
//	nostr+relayconnect://<relay-pubkey>?relay=<rendezvous-relay>&secret=<client-secret>[&name=<device-name>]
//
// CAT-based URI format:
//
//	nostr+relayconnect://<relay-pubkey>?relay=<rendezvous-relay>&auth=cat&mint=<mint-url>
func ParseConnectionURI(nrcURI string) (conn *ConnectionURI, err error) {
	var p *url.URL
	if p, err = url.Parse(nrcURI); chk.E(err) {
		return
	}
	if p == nil {
		err = errors.New("invalid uri")
		return
	}

	conn = &ConnectionURI{}

	// Validate scheme
	if p.Scheme != "nostr+relayconnect" {
		err = errors.New("incorrect scheme: expected nostr+relayconnect")
		return
	}

	// Parse relay pubkey from host
	if conn.RelayPubkey, err = hex.Dec(p.Host); chk.E(err) {
		err = errors.New("invalid relay public key")
		return
	}
	if len(conn.RelayPubkey) != 32 {
		err = errors.New("relay public key must be 32 bytes")
		return
	}

	query := p.Query()

	// Parse rendezvous relay URL (required)
	relayParam := query.Get("relay")
	if relayParam == "" {
		err = errors.New("missing relay parameter")
		return
	}
	conn.RendezvousRelay = relayParam

	// Parse optional device name
	conn.DeviceName = query.Get("name")

	// Determine auth mode
	authParam := query.Get("auth")
	if authParam == "cat" {
		conn.AuthMode = AuthModeCAT
		// Parse mint URL for CAT auth
		conn.MintURL = query.Get("mint")
		if conn.MintURL == "" {
			err = errors.New("missing mint parameter for CAT auth")
			return
		}
	} else {
		conn.AuthMode = AuthModeSecret
		// Parse secret for secret-based auth
		secret := query.Get("secret")
		if secret == "" {
			err = errors.New("missing secret parameter")
			return
		}

		var secretBytes []byte
		if secretBytes, err = hex.Dec(secret); chk.E(err) {
			err = errors.New("invalid secret: must be hex-encoded")
			return
		}
		if len(secretBytes) != 32 {
			err = errors.New("secret must be 32 bytes")
			return
		}

		// Create signer from secret
		var clientKey *p8k.Signer
		if clientKey, err = p8k.New(); chk.E(err) {
			return
		}
		if err = clientKey.InitSec(secretBytes); chk.E(err) {
			return
		}
		conn.clientSecretKey = clientKey

		// Generate conversation key using NIP-44 key derivation
		if conn.conversationKey, err = encryption.GenerateConversationKey(
			clientKey.Sec(),
			conn.RelayPubkey,
		); chk.E(err) {
			return
		}
	}

	return
}

// GenerateConnectionURI creates a new NRC connection URI with a random secret.
func GenerateConnectionURI(relayPubkey []byte, rendezvousRelay string, deviceName string) (uri string, secret []byte, err error) {
	if len(relayPubkey) != 32 {
		err = errors.New("relay public key must be 32 bytes")
		return
	}

	// Generate random 32-byte secret
	var clientKey *p8k.Signer
	if clientKey, err = p8k.New(); chk.E(err) {
		return
	}
	if err = clientKey.Generate(); chk.E(err) {
		return
	}
	secret = clientKey.Sec()

	// Build URI
	u := &url.URL{
		Scheme: "nostr+relayconnect",
		Host:   string(hex.Enc(relayPubkey)),
	}
	q := u.Query()
	q.Set("relay", rendezvousRelay)
	q.Set("secret", string(hex.Enc(secret)))
	if deviceName != "" {
		q.Set("name", deviceName)
	}
	u.RawQuery = q.Encode()
	uri = u.String()
	return
}

// GenerateCATConnectionURI creates a new NRC connection URI for CAT authentication.
func GenerateCATConnectionURI(relayPubkey []byte, rendezvousRelay string, mintURL string) (uri string, err error) {
	if len(relayPubkey) != 32 {
		err = errors.New("relay public key must be 32 bytes")
		return
	}

	// Build URI
	u := &url.URL{
		Scheme: "nostr+relayconnect",
		Host:   string(hex.Enc(relayPubkey)),
	}
	q := u.Query()
	q.Set("relay", rendezvousRelay)
	q.Set("auth", "cat")
	q.Set("mint", mintURL)
	u.RawQuery = q.Encode()
	uri = u.String()
	return
}
