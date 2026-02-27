//go:build !(js && wasm)

package database

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

const relayIdentitySecretKey = "relay:identity:sk"

// GetRelayIdentitySecret returns the relay identity secret key bytes if present.
// If the key is not found, returns (nil, badger.ErrKeyNotFound).
func (d *D) GetRelayIdentitySecret() (skb []byte, err error) {
	err = d.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(relayIdentitySecretKey))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			// value stored as hex string
			b, err := hex.Dec(string(val))
			if err != nil {
				return err
			}
			skb = make([]byte, len(b))
			copy(skb, b)
			return nil
		})
	})
	return
}

// SetRelayIdentitySecret stores the relay identity secret key bytes (expects 32 bytes).
func (d *D) SetRelayIdentitySecret(skb []byte) (err error) {
	if len(skb) != 32 {
		return fmt.Errorf("invalid secret key length: %d", len(skb))
	}
	val := []byte(hex.Enc(skb))
	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(relayIdentitySecretKey), val)
	})
}

// GetOrCreateRelayIdentitySecret retrieves the existing relay identity secret
// key or creates and stores a new one if none exists.
func (d *D) GetOrCreateRelayIdentitySecret() (skb []byte, err error) {
	// Try get fast path
	if skb, err = d.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		return skb, nil
	}
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return nil, err
	}

	// Create new key and store atomically
	var gen []byte
	if gen, err = keys.GenerateSecretKey(); chk.E(err) {
		return nil, err
	}
	if err = d.SetRelayIdentitySecret(gen); chk.E(err) {
		return nil, err
	}
	log.I.F("generated new relay identity key (pub=%s)", mustPub(gen))
	return gen, nil
}

func mustPub(skb []byte) string {
	pk, err := keys.SecretBytesToPubKeyHex(skb)
	if err != nil {
		return ""
	}
	return pk
}
