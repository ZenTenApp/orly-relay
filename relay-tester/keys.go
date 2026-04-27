package relaytester

import (
	"crypto/rand"
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// KeyPair represents a test keypair.
type KeyPair struct {
	Secret *p8k.Signer
	Pubkey []byte
	Nsec   string
	Npub   string
}

// GenerateKeyPair generates a new keypair for testing.
func GenerateKeyPair() (kp *KeyPair, err error) {
	kp = &KeyPair{}
	if kp.Secret, err = p8k.New(); chk.E(err) {
		return
	}
	if err = kp.Secret.Generate(); chk.E(err) {
		return
	}
	kp.Pubkey = kp.Secret.Pub()
	nsecBytes, err := bech32encoding.BinToNsec(kp.Secret.Sec())
	if chk.E(err) {
		return
	}
	kp.Nsec = string(nsecBytes)
	npubBytes, err := bech32encoding.BinToNpub(kp.Pubkey)
	if chk.E(err) {
		return
	}
	kp.Npub = string(npubBytes)
	return
}

// CreateEvent creates a signed event with the given parameters.
func CreateEvent(signer *p8k.Signer, kindNum uint16, content string, tags *tag.S) (ev *event.E, err error) {
	ev = event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kindNum
	ev.Content = []byte(content)
	if tags != nil {
		ev.Tags = tags
	} else {
		ev.Tags = tag.NewS()
	}
	if err = ev.Sign(signer); chk.E(err) {
		return
	}
	return
}

// CreateEventWithTags creates an event with specific tags.
func CreateEventWithTags(signer *p8k.Signer, kindNum uint16, content string, tagPairs [][]string) (ev *event.E, err error) {
	tags := tag.NewS()
	for _, pair := range tagPairs {
		if len(pair) >= 2 {
			// Build tag fields as []byte variadic arguments
			tagFields := make([][]byte, len(pair))
			tagFields[0] = []byte(pair[0])
			for i := 1; i < len(pair); i++ {
				tagFields[i] = []byte(pair[i])
			}
			tags.Append(tag.NewFromBytesSlice(tagFields...))
		}
	}
	return CreateEvent(signer, kindNum, content, tags)
}

// CreateReplaceableEvent creates a replaceable event (kind 0-3, 10000-19999).
func CreateReplaceableEvent(signer *p8k.Signer, kindNum uint16, content string) (ev *event.E, err error) {
	return CreateEvent(signer, kindNum, content, nil)
}

// CreateEphemeralEvent creates an ephemeral event (kind 20000-29999).
func CreateEphemeralEvent(signer *p8k.Signer, kindNum uint16, content string) (ev *event.E, err error) {
	return CreateEvent(signer, kindNum, content, nil)
}

// CreateDeleteEvent creates a deletion event (kind 5).
func CreateDeleteEvent(signer *p8k.Signer, eventIDs [][]byte, reason string) (ev *event.E, err error) {
	tags := tag.NewS()
	for _, id := range eventIDs {
		// e tags must contain hex-encoded event IDs
		tags.Append(tag.NewFromBytesSlice([]byte("e"), []byte(hex.Enc(id))))
	}
	if reason != "" {
		tags.Append(tag.NewFromBytesSlice([]byte("content"), []byte(reason)))
	}
	return CreateEvent(signer, kind.EventDeletion.K, reason, tags)
}

// CreateParameterizedReplaceableEvent creates a parameterized replaceable event (kind 30000-39999).
func CreateParameterizedReplaceableEvent(signer *p8k.Signer, kindNum uint16, content string, dTag string) (ev *event.E, err error) {
	tags := tag.NewS()
	tags.Append(tag.NewFromBytesSlice([]byte("d"), []byte(dTag)))
	return CreateEvent(signer, kindNum, content, tags)
}

// RandomID generates a random 32-byte ID.
func RandomID() (id []byte, err error) {
	id = make([]byte, 32)
	if _, err = rand.Read(id); err != nil {
		return nil, fmt.Errorf("failed to generate random ID: %w", err)
	}
	return
}

// MustHex decodes a hex string or panics.
func MustHex(s string) []byte {
	b, err := hex.Dec(s)
	if err != nil {
		panic(fmt.Sprintf("invalid hex: %s", s))
	}
	return b
}

// HexID returns the hex-encoded event ID.
func HexID(ev *event.E) string {
	return hex.Enc(ev.ID)
}
