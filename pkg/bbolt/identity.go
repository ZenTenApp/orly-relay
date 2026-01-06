//go:build !(js && wasm)

package bbolt

import (
	"crypto/rand"
	"errors"

	bolt "go.etcd.io/bbolt"
)

const identityKey = "relay_identity_secret"

// GetRelayIdentitySecret gets the relay's identity secret key.
func (b *B) GetRelayIdentitySecret() (skb []byte, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return errors.New("bbolt: meta bucket not found")
		}
		data := bucket.Get([]byte(identityKey))
		if data == nil {
			return errors.New("bbolt: relay identity not set")
		}
		skb = make([]byte, len(data))
		copy(skb, data)
		return nil
	})
	return
}

// SetRelayIdentitySecret sets the relay's identity secret key.
func (b *B) SetRelayIdentitySecret(skb []byte) error {
	if len(skb) != 32 {
		return errors.New("bbolt: invalid secret key length (must be 32 bytes)")
	}
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return errors.New("bbolt: meta bucket not found")
		}
		return bucket.Put([]byte(identityKey), skb)
	})
}

// GetOrCreateRelayIdentitySecret gets or creates the relay's identity secret.
func (b *B) GetOrCreateRelayIdentitySecret() (skb []byte, err error) {
	// Try to get existing secret
	skb, err = b.GetRelayIdentitySecret()
	if err == nil && len(skb) == 32 {
		return skb, nil
	}

	// Generate new secret
	skb = make([]byte, 32)
	if _, err = rand.Read(skb); err != nil {
		return nil, err
	}

	// Store it
	if err = b.SetRelayIdentitySecret(skb); err != nil {
		return nil, err
	}

	return skb, nil
}
