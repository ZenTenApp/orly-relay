//go:build !(js && wasm)

package database

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/crypto/keys"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// Key prefixes for NRC data
const (
	nrcConnectionPrefix = "nrc:conn:" // NRC connections by ID
)

// NRCConnection stores an NRC connection configuration in the database.
type NRCConnection struct {
	ID        string `json:"id"`         // Unique identifier (hex of first 8 bytes of secret)
	Label     string `json:"label"`      // Human-readable label (e.g., "Phone", "Laptop")
	Secret    []byte `json:"secret"`     // 32-byte secret for client authentication
	CreatedAt int64  `json:"created_at"` // Unix timestamp
	LastUsed  int64  `json:"last_used"`  // Unix timestamp of last connection (0 if never)
}

// GetNRCConnection retrieves an NRC connection by ID.
func (d *D) GetNRCConnection(id string) (conn *NRCConnection, err error) {
	key := []byte(nrcConnectionPrefix + id)

	err = d.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			conn = &NRCConnection{}
			return json.Unmarshal(val, conn)
		})
	})
	return
}

// SaveNRCConnection stores an NRC connection in the database.
func (d *D) SaveNRCConnection(conn *NRCConnection) error {
	data, err := json.Marshal(conn)
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}

	key := []byte(nrcConnectionPrefix + conn.ID)

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// DeleteNRCConnection removes an NRC connection from the database.
func (d *D) DeleteNRCConnection(id string) error {
	key := []byte(nrcConnectionPrefix + id)

	return d.DB.Update(func(txn *badger.Txn) error {
		if err := txn.Delete(key); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		return nil
	})
}

// GetAllNRCConnections returns all NRC connections.
func (d *D) GetAllNRCConnections() (conns []*NRCConnection, err error) {
	prefix := []byte(nrcConnectionPrefix)

	err = d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				conn := &NRCConnection{}
				if err := json.Unmarshal(val, conn); err != nil {
					return err
				}
				conns = append(conns, conn)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return
}

// CreateNRCConnection generates a new NRC connection with a random secret.
func (d *D) CreateNRCConnection(label string) (*NRCConnection, error) {
	// Generate random 32-byte secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("failed to generate random secret: %w", err)
	}

	// Use first 8 bytes of secret as ID (hex encoded = 16 chars)
	id := string(hex.Enc(secret[:8]))

	conn := &NRCConnection{
		ID:        id,
		Label:     label,
		Secret:    secret,
		CreatedAt: time.Now().Unix(),
		LastUsed:  0,
	}

	if err := d.SaveNRCConnection(conn); chk.E(err) {
		return nil, err
	}

	log.I.F("created NRC connection: id=%s label=%s", id, label)
	return conn, nil
}

// GetNRCConnectionURI generates the full connection URI for a connection.
// relayPubkey is the relay's public key (32 bytes).
// rendezvousURL is the public relay URL.
func (d *D) GetNRCConnectionURI(conn *NRCConnection, relayPubkey []byte, rendezvousURL string) (string, error) {
	if len(relayPubkey) != 32 {
		return "", fmt.Errorf("invalid relay pubkey length: %d", len(relayPubkey))
	}
	if rendezvousURL == "" {
		return "", fmt.Errorf("rendezvous URL is required")
	}

	relayPubkeyHex := hex.Enc(relayPubkey)
	secretHex := hex.Enc(conn.Secret)

	// Secret-only URI
	uri := fmt.Sprintf("nostr+relayconnect://%s?relay=%s&secret=%s",
		relayPubkeyHex, rendezvousURL, secretHex)

	if conn.Label != "" {
		uri += fmt.Sprintf("&name=%s", conn.Label)
	}

	return uri, nil
}

// GetNRCAuthorizedSecrets returns a map of derived pubkeys to labels for all connections.
// This is used by the NRC bridge to authorize incoming connections.
func (d *D) GetNRCAuthorizedSecrets() (map[string]string, error) {
	conns, err := d.GetAllNRCConnections()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, conn := range conns {
		// Derive pubkey from secret
		pubkey, err := keys.SecretBytesToPubKeyBytes(conn.Secret)
		if err != nil {
			log.W.F("failed to derive pubkey for NRC connection %s: %v", conn.ID, err)
			continue
		}
		pubkeyHex := string(hex.Enc(pubkey))
		result[pubkeyHex] = conn.Label
	}

	return result, nil
}

// UpdateNRCConnectionLastUsed updates the last used timestamp for a connection.
func (d *D) UpdateNRCConnectionLastUsed(id string) error {
	conn, err := d.GetNRCConnection(id)
	if err != nil {
		return err
	}

	conn.LastUsed = time.Now().Unix()
	return d.SaveNRCConnection(conn)
}
