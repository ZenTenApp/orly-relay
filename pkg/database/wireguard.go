//go:build !(js && wasm)

package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/wireguard"
)

// Key prefixes for WireGuard data
const (
	wgServerKeyPrefix  = "wg:server:key"  // Server's WireGuard private key
	wgSubnetSeedPrefix = "wg:subnet:seed" // Seed for deterministic subnet generation
	wgPeerPrefix       = "wg:peer:"       // Peer data by Nostr pubkey hex
	wgSequenceKey      = "wg:seq"         // Badger sequence key for subnet allocation
	wgRevokedPrefix    = "wg:revoked:"    // Revoked keypairs by Nostr pubkey hex
	wgAccessLogPrefix  = "wg:accesslog:"  // Access log for obsolete addresses
)

// WireGuardPeer stores WireGuard peer information in the database.
type WireGuardPeer struct {
	NostrPubkey  []byte `json:"nostr_pubkey"`   // User's Nostr pubkey (32 bytes)
	WGPrivateKey []byte `json:"wg_private_key"` // WireGuard private key (32 bytes)
	WGPublicKey  []byte `json:"wg_public_key"`  // WireGuard public key (32 bytes)
	Sequence     uint32 `json:"sequence"`       // Sequence number for subnet derivation
	CreatedAt    int64  `json:"created_at"`     // Unix timestamp
}

// WireGuardRevokedKey stores a revoked/old WireGuard keypair for audit purposes.
type WireGuardRevokedKey struct {
	NostrPubkey  []byte `json:"nostr_pubkey"`   // User's Nostr pubkey (32 bytes)
	WGPublicKey  []byte `json:"wg_public_key"`  // Revoked WireGuard public key (32 bytes)
	Sequence     uint32 `json:"sequence"`       // Sequence number (subnet)
	CreatedAt    int64  `json:"created_at"`     // When the key was originally created
	RevokedAt    int64  `json:"revoked_at"`     // When the key was revoked
	AccessCount  int    `json:"access_count"`   // Number of access attempts since revocation
	LastAccessAt int64  `json:"last_access_at"` // Last access attempt timestamp (0 if never)
}

// WireGuardAccessLog records an access attempt to an obsolete address.
type WireGuardAccessLog struct {
	NostrPubkey []byte `json:"nostr_pubkey"`  // User's Nostr pubkey
	WGPublicKey []byte `json:"wg_public_key"` // The obsolete public key used
	Sequence    uint32 `json:"sequence"`      // Subnet sequence
	Timestamp   int64  `json:"timestamp"`     // When the access occurred
	RemoteAddr  string `json:"remote_addr"`   // Remote IP address
}

// ServerIP returns the derived server IP for this peer's subnet.
func (p *WireGuardPeer) ServerIP(pool *wireguard.SubnetPool) string {
	subnet := pool.SubnetForSequence(p.Sequence)
	return subnet.ServerIP.String()
}

// ClientIP returns the derived client IP for this peer's subnet.
func (p *WireGuardPeer) ClientIP(pool *wireguard.SubnetPool) string {
	subnet := pool.SubnetForSequence(p.Sequence)
	return subnet.ClientIP.String()
}

// GetWireGuardServerKey retrieves the WireGuard server private key.
func (d *D) GetWireGuardServerKey() (key []byte, err error) {
	err = d.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(wgServerKeyPrefix))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			key = make([]byte, len(val))
			copy(key, val)
			return nil
		})
	})
	return
}

// SetWireGuardServerKey stores the WireGuard server private key.
func (d *D) SetWireGuardServerKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("invalid key length: %d (expected 32)", len(key))
	}
	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(wgServerKeyPrefix), key)
	})
}

// GetOrCreateWireGuardServerKey retrieves or creates the WireGuard server key.
func (d *D) GetOrCreateWireGuardServerKey() (key []byte, err error) {
	// Try to get existing key
	if key, err = d.GetWireGuardServerKey(); err == nil && len(key) == 32 {
		return key, nil
	}
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return nil, err
	}

	// Generate new keypair
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate WireGuard keypair: %w", err)
	}

	// Store the private key
	if err = d.SetWireGuardServerKey(privateKey); chk.E(err) {
		return nil, err
	}

	log.I.F("generated new WireGuard server key (pubkey=%s...)", hex.Enc(publicKey[:8]))
	return privateKey, nil
}

// GetSubnetSeed retrieves the subnet pool seed.
func (d *D) GetSubnetSeed() (seed []byte, err error) {
	err = d.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(wgSubnetSeedPrefix))
		if errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			seed = make([]byte, len(val))
			copy(seed, val)
			return nil
		})
	})
	return
}

// SetSubnetSeed stores the subnet pool seed.
func (d *D) SetSubnetSeed(seed []byte) error {
	if len(seed) != 32 {
		return fmt.Errorf("invalid seed length: %d (expected 32)", len(seed))
	}
	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(wgSubnetSeedPrefix), seed)
	})
}

// GetOrCreateSubnetPool creates or restores a subnet pool from the database.
func (d *D) GetOrCreateSubnetPool(baseNetwork string) (*wireguard.SubnetPool, error) {
	// Try to get existing seed
	seed, err := d.GetSubnetSeed()
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return nil, err
	}

	var pool *wireguard.SubnetPool

	if len(seed) == 32 {
		// Restore pool with existing seed
		pool, err = wireguard.NewSubnetPoolWithSeed(baseNetwork, seed)
		if err != nil {
			return nil, err
		}
		log.D.F("restored subnet pool with existing seed")
	} else {
		// Create new pool with random seed
		pool, err = wireguard.NewSubnetPool(baseNetwork)
		if err != nil {
			return nil, err
		}

		// Store the new seed
		if err = d.SetSubnetSeed(pool.Seed()); err != nil {
			return nil, fmt.Errorf("failed to store subnet seed: %w", err)
		}
		log.I.F("generated new subnet pool seed")
	}

	// Restore existing allocations from database
	peers, err := d.GetAllWireGuardPeers()
	if err != nil {
		return nil, fmt.Errorf("failed to load existing peers: %w", err)
	}

	for _, peer := range peers {
		pool.RestoreAllocation(hex.Enc(peer.NostrPubkey), peer.Sequence)
	}

	if len(peers) > 0 {
		log.D.F("restored %d subnet allocations", len(peers))
	}

	return pool, nil
}

// GetWireGuardPeer retrieves a WireGuard peer by Nostr pubkey.
func (d *D) GetWireGuardPeer(nostrPubkey []byte) (peer *WireGuardPeer, err error) {
	key := append([]byte(wgPeerPrefix), []byte(hex.Enc(nostrPubkey))...)

	err = d.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			peer = &WireGuardPeer{}
			return json.Unmarshal(val, peer)
		})
	})
	return
}

// GetOrCreateWireGuardPeer retrieves or creates a WireGuard peer.
// The pool is used for subnet derivation from the sequence number.
func (d *D) GetOrCreateWireGuardPeer(nostrPubkey []byte, pool *wireguard.SubnetPool) (peer *WireGuardPeer, err error) {
	// Try to get existing peer
	if peer, err = d.GetWireGuardPeer(nostrPubkey); err == nil {
		return peer, nil
	}
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return nil, err
	}

	// Generate new WireGuard keypair
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate WireGuard keypair: %w", err)
	}

	// Get next sequence number from Badger's sequence
	seq64, err := d.GetNextWGSequence()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate sequence: %w", err)
	}
	seq := uint32(seq64)

	// Register allocation with pool for in-memory tracking
	pubkeyHex := hex.Enc(nostrPubkey)
	pool.RestoreAllocation(pubkeyHex, seq)

	peer = &WireGuardPeer{
		NostrPubkey:  nostrPubkey,
		WGPrivateKey: privateKey,
		WGPublicKey:  publicKey,
		Sequence:     seq,
		CreatedAt:    time.Now().Unix(),
	}

	// Store peer data
	if err = d.setWireGuardPeer(peer); err != nil {
		return nil, err
	}

	subnet := pool.SubnetForSequence(seq)
	log.I.F("created WireGuard peer: nostr=%s... -> subnet %s/%s (seq=%d)",
		hex.Enc(nostrPubkey[:8]), subnet.ServerIP, subnet.ClientIP, seq)

	return peer, nil
}

// RegenerateWireGuardPeer generates a new keypair for an existing peer.
// The sequence number (and thus subnet) is preserved.
// The old keypair is archived for audit purposes.
func (d *D) RegenerateWireGuardPeer(nostrPubkey []byte, pool *wireguard.SubnetPool) (peer *WireGuardPeer, err error) {
	// Get existing peer to preserve sequence
	existing, err := d.GetWireGuardPeer(nostrPubkey)
	if err != nil {
		return nil, err
	}

	// Archive the old keypair for audit purposes
	if err = d.ArchiveRevokedKey(existing); err != nil {
		log.W.F("failed to archive revoked key: %v", err)
		// Continue anyway - this is audit logging, not critical
	}

	// Generate new WireGuard keypair
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate WireGuard keypair: %w", err)
	}

	peer = &WireGuardPeer{
		NostrPubkey:  nostrPubkey,
		WGPrivateKey: privateKey,
		WGPublicKey:  publicKey,
		Sequence:     existing.Sequence, // Keep same sequence (same subnet)
		CreatedAt:    time.Now().Unix(),
	}

	// Store updated peer data
	if err = d.setWireGuardPeer(peer); err != nil {
		return nil, err
	}

	subnet := pool.SubnetForSequence(peer.Sequence)
	log.I.F("regenerated WireGuard peer: nostr=%s... -> subnet %s/%s (old key archived)",
		hex.Enc(nostrPubkey[:8]), subnet.ServerIP, subnet.ClientIP)

	return peer, nil
}

// DeleteWireGuardPeer removes a WireGuard peer from the database.
// Note: The sequence number is not recycled to prevent subnet reuse.
func (d *D) DeleteWireGuardPeer(nostrPubkey []byte) error {
	peerKey := append([]byte(wgPeerPrefix), []byte(hex.Enc(nostrPubkey))...)

	return d.DB.Update(func(txn *badger.Txn) error {
		if err := txn.Delete(peerKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}
		return nil
	})
}

// GetAllWireGuardPeers returns all WireGuard peers.
func (d *D) GetAllWireGuardPeers() (peers []*WireGuardPeer, err error) {
	prefix := []byte(wgPeerPrefix)

	err = d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				peer := &WireGuardPeer{}
				if err := json.Unmarshal(val, peer); err != nil {
					return err
				}
				peers = append(peers, peer)
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

// setWireGuardPeer stores a WireGuard peer in the database.
func (d *D) setWireGuardPeer(peer *WireGuardPeer) error {
	data, err := json.Marshal(peer)
	if err != nil {
		return fmt.Errorf("failed to marshal peer: %w", err)
	}

	peerKey := append([]byte(wgPeerPrefix), []byte(hex.Enc(peer.NostrPubkey))...)

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(peerKey, data)
	})
}

// GetNextWGSequence retrieves and increments the sequence counter using Badger's Sequence.
func (d *D) GetNextWGSequence() (seq uint64, err error) {
	// Get a sequence with bandwidth 1 (allocate 1 number at a time)
	badgerSeq, err := d.DB.GetSequence([]byte(wgSequenceKey), 1)
	if err != nil {
		return 0, fmt.Errorf("failed to get sequence: %w", err)
	}
	defer badgerSeq.Release()

	seq, err = badgerSeq.Next()
	if err != nil {
		return 0, fmt.Errorf("failed to get next sequence number: %w", err)
	}
	return seq, nil
}

// ArchiveRevokedKey stores a revoked keypair for audit purposes.
func (d *D) ArchiveRevokedKey(peer *WireGuardPeer) error {
	revoked := &WireGuardRevokedKey{
		NostrPubkey: peer.NostrPubkey,
		WGPublicKey: peer.WGPublicKey,
		Sequence:    peer.Sequence,
		CreatedAt:   peer.CreatedAt,
		RevokedAt:   time.Now().Unix(),
		AccessCount: 0,
		LastAccessAt: 0,
	}

	data, err := json.Marshal(revoked)
	if err != nil {
		return fmt.Errorf("failed to marshal revoked key: %w", err)
	}

	// Key: wg:revoked:<pubkey-hex>:<revoked-timestamp>
	keyStr := fmt.Sprintf("%s%s:%d", wgRevokedPrefix, hex.Enc(peer.NostrPubkey), revoked.RevokedAt)

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyStr), data)
	})
}

// GetRevokedKeys returns all revoked keys for a user.
func (d *D) GetRevokedKeys(nostrPubkey []byte) (keys []*WireGuardRevokedKey, err error) {
	prefix := []byte(wgRevokedPrefix + hex.Enc(nostrPubkey) + ":")

	err = d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				key := &WireGuardRevokedKey{}
				if err := json.Unmarshal(val, key); err != nil {
					return err
				}
				keys = append(keys, key)
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

// GetAllRevokedKeys returns all revoked keys across all users (admin view).
func (d *D) GetAllRevokedKeys() (keys []*WireGuardRevokedKey, err error) {
	prefix := []byte(wgRevokedPrefix)

	err = d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				key := &WireGuardRevokedKey{}
				if err := json.Unmarshal(val, key); err != nil {
					return err
				}
				keys = append(keys, key)
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

// LogObsoleteAccess records an access attempt to an obsolete WireGuard address.
func (d *D) LogObsoleteAccess(nostrPubkey, wgPubkey []byte, sequence uint32, remoteAddr string) error {
	now := time.Now().Unix()

	logEntry := &WireGuardAccessLog{
		NostrPubkey: nostrPubkey,
		WGPublicKey: wgPubkey,
		Sequence:    sequence,
		Timestamp:   now,
		RemoteAddr:  remoteAddr,
	}

	data, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal access log: %w", err)
	}

	// Key: wg:accesslog:<pubkey-hex>:<timestamp>
	keyStr := fmt.Sprintf("%s%s:%d", wgAccessLogPrefix, hex.Enc(nostrPubkey), now)

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyStr), data)
	})
}

// GetAccessLogs returns access logs for a user.
func (d *D) GetAccessLogs(nostrPubkey []byte) (logs []*WireGuardAccessLog, err error) {
	prefix := []byte(wgAccessLogPrefix + hex.Enc(nostrPubkey) + ":")

	err = d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				logEntry := &WireGuardAccessLog{}
				if err := json.Unmarshal(val, logEntry); err != nil {
					return err
				}
				logs = append(logs, logEntry)
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

// GetAllAccessLogs returns all access logs (admin view).
func (d *D) GetAllAccessLogs() (logs []*WireGuardAccessLog, err error) {
	prefix := []byte(wgAccessLogPrefix)

	err = d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				logEntry := &WireGuardAccessLog{}
				if err := json.Unmarshal(val, logEntry); err != nil {
					return err
				}
				logs = append(logs, logEntry)
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

// IncrementRevokedKeyAccess updates the access count for a revoked key.
func (d *D) IncrementRevokedKeyAccess(nostrPubkey, wgPubkey []byte) error {
	// Find and update the matching revoked key
	prefix := []byte(wgRevokedPrefix + hex.Enc(nostrPubkey) + ":")
	wgPubkeyHex := hex.Enc(wgPubkey)
	now := time.Now().Unix()

	return d.DB.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			err := item.Value(func(val []byte) error {
				revoked := &WireGuardRevokedKey{}
				if err := json.Unmarshal(val, revoked); err != nil {
					return err
				}

				// Check if this is the matching revoked key
				if hex.Enc(revoked.WGPublicKey) == wgPubkeyHex {
					revoked.AccessCount++
					revoked.LastAccessAt = now

					data, err := json.Marshal(revoked)
					if err != nil {
						return err
					}
					return txn.Set(key, data)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}
