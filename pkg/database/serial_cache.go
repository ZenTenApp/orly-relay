//go:build !(js && wasm)

package database

import (
	"errors"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/bufpool"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
)

// SerialCache provides LRU caching for pubkey and event ID serial lookups.
// This is critical for compact event decoding performance since every event
// requires looking up the author pubkey and potentially multiple tag references.
type SerialCache struct {
	// Pubkey serial -> full pubkey (for decoding)
	pubkeyBySerial     map[uint64][]byte
	pubkeyBySerialLock sync.RWMutex

	// Pubkey hash -> serial (for encoding)
	serialByPubkeyHash     map[string]uint64
	serialByPubkeyHashLock sync.RWMutex

	// Event serial -> full event ID (for decoding)
	eventIdBySerial     map[uint64][]byte
	eventIdBySerialLock sync.RWMutex

	// Event ID hash -> serial (for encoding)
	serialByEventIdHash     map[string]uint64
	serialByEventIdHashLock sync.RWMutex

	// Maximum cache sizes
	maxPubkeys  int
	maxEventIds int
}

// NewSerialCache creates a new serial cache with the specified sizes.
func NewSerialCache(maxPubkeys, maxEventIds int) *SerialCache {
	if maxPubkeys <= 0 {
		maxPubkeys = 100000 // Default 100k pubkeys (~3.2MB)
	}
	if maxEventIds <= 0 {
		maxEventIds = 500000 // Default 500k event IDs (~16MB)
	}
	return &SerialCache{
		pubkeyBySerial:      make(map[uint64][]byte, maxPubkeys),
		serialByPubkeyHash:  make(map[string]uint64, maxPubkeys),
		eventIdBySerial:     make(map[uint64][]byte, maxEventIds),
		serialByEventIdHash: make(map[string]uint64, maxEventIds),
		maxPubkeys:          maxPubkeys,
		maxEventIds:         maxEventIds,
	}
}

// CachePubkey adds a pubkey to the cache.
func (c *SerialCache) CachePubkey(serial uint64, pubkey []byte) {
	if len(pubkey) != 32 {
		return
	}

	// Cache serial -> pubkey
	c.pubkeyBySerialLock.Lock()
	if len(c.pubkeyBySerial) >= c.maxPubkeys {
		// Simple eviction: clear half the cache
		// A proper LRU would be better but this is simpler
		count := 0
		for k := range c.pubkeyBySerial {
			delete(c.pubkeyBySerial, k)
			count++
			if count >= c.maxPubkeys/2 {
				break
			}
		}
	}
	pk := make([]byte, 32)
	copy(pk, pubkey)
	c.pubkeyBySerial[serial] = pk
	c.pubkeyBySerialLock.Unlock()

	// Cache pubkey hash -> serial
	c.serialByPubkeyHashLock.Lock()
	if len(c.serialByPubkeyHash) >= c.maxPubkeys {
		count := 0
		for k := range c.serialByPubkeyHash {
			delete(c.serialByPubkeyHash, k)
			count++
			if count >= c.maxPubkeys/2 {
				break
			}
		}
	}
	c.serialByPubkeyHash[string(pubkey)] = serial
	c.serialByPubkeyHashLock.Unlock()
}

// GetPubkeyBySerial returns the pubkey for a serial from cache.
func (c *SerialCache) GetPubkeyBySerial(serial uint64) (pubkey []byte, found bool) {
	c.pubkeyBySerialLock.RLock()
	pubkey, found = c.pubkeyBySerial[serial]
	c.pubkeyBySerialLock.RUnlock()
	return
}

// GetSerialByPubkey returns the serial for a pubkey from cache.
func (c *SerialCache) GetSerialByPubkey(pubkey []byte) (serial uint64, found bool) {
	c.serialByPubkeyHashLock.RLock()
	serial, found = c.serialByPubkeyHash[string(pubkey)]
	c.serialByPubkeyHashLock.RUnlock()
	return
}

// CacheEventId adds an event ID to the cache.
func (c *SerialCache) CacheEventId(serial uint64, eventId []byte) {
	if len(eventId) != 32 {
		return
	}

	// Cache serial -> event ID
	c.eventIdBySerialLock.Lock()
	if len(c.eventIdBySerial) >= c.maxEventIds {
		count := 0
		for k := range c.eventIdBySerial {
			delete(c.eventIdBySerial, k)
			count++
			if count >= c.maxEventIds/2 {
				break
			}
		}
	}
	eid := make([]byte, 32)
	copy(eid, eventId)
	c.eventIdBySerial[serial] = eid
	c.eventIdBySerialLock.Unlock()

	// Cache event ID hash -> serial
	c.serialByEventIdHashLock.Lock()
	if len(c.serialByEventIdHash) >= c.maxEventIds {
		count := 0
		for k := range c.serialByEventIdHash {
			delete(c.serialByEventIdHash, k)
			count++
			if count >= c.maxEventIds/2 {
				break
			}
		}
	}
	c.serialByEventIdHash[string(eventId)] = serial
	c.serialByEventIdHashLock.Unlock()
}

// GetEventIdBySerial returns the event ID for a serial from cache.
func (c *SerialCache) GetEventIdBySerial(serial uint64) (eventId []byte, found bool) {
	c.eventIdBySerialLock.RLock()
	eventId, found = c.eventIdBySerial[serial]
	c.eventIdBySerialLock.RUnlock()
	return
}

// GetSerialByEventId returns the serial for an event ID from cache.
func (c *SerialCache) GetSerialByEventId(eventId []byte) (serial uint64, found bool) {
	c.serialByEventIdHashLock.RLock()
	serial, found = c.serialByEventIdHash[string(eventId)]
	c.serialByEventIdHashLock.RUnlock()
	return
}

// DatabaseSerialResolver implements SerialResolver using the database and cache.
type DatabaseSerialResolver struct {
	db    *D
	cache *SerialCache
}

// NewDatabaseSerialResolver creates a new resolver.
func NewDatabaseSerialResolver(db *D, cache *SerialCache) *DatabaseSerialResolver {
	return &DatabaseSerialResolver{db: db, cache: cache}
}

// GetOrCreatePubkeySerial implements SerialResolver.
func (r *DatabaseSerialResolver) GetOrCreatePubkeySerial(pubkey []byte) (serial uint64, err error) {
	if len(pubkey) != 32 {
		return 0, errors.New("pubkey must be 32 bytes")
	}

	// Check cache first
	if s, found := r.cache.GetSerialByPubkey(pubkey); found {
		return s, nil
	}

	// Use existing function which handles creation
	ser, err := r.db.GetOrCreatePubkeySerial(pubkey)
	if err != nil {
		return 0, err
	}

	serial = ser.Get()

	// Cache it
	r.cache.CachePubkey(serial, pubkey)

	return serial, nil
}

// GetPubkeyBySerial implements SerialResolver.
func (r *DatabaseSerialResolver) GetPubkeyBySerial(serial uint64) (pubkey []byte, err error) {
	// Check cache first
	if pk, found := r.cache.GetPubkeyBySerial(serial); found {
		return pk, nil
	}

	// Look up in database
	ser := new(types.Uint40)
	if err = ser.Set(serial); err != nil {
		return nil, err
	}

	pubkey, err = r.db.GetPubkeyBySerial(ser)
	if err != nil {
		return nil, err
	}

	// Cache it
	r.cache.CachePubkey(serial, pubkey)

	return pubkey, nil
}

// GetEventSerialById implements SerialResolver.
func (r *DatabaseSerialResolver) GetEventSerialById(eventId []byte) (serial uint64, found bool, err error) {
	if len(eventId) != 32 {
		return 0, false, errors.New("event ID must be 32 bytes")
	}

	// Check cache first
	if s, ok := r.cache.GetSerialByEventId(eventId); ok {
		return s, true, nil
	}

	// Look up in database using existing GetSerialById
	ser, err := r.db.GetSerialById(eventId)
	if err != nil {
		// Not found is not an error - just return found=false
		return 0, false, nil
	}

	serial = ser.Get()

	// Cache it
	r.cache.CacheEventId(serial, eventId)

	return serial, true, nil
}

// GetEventIdBySerial implements SerialResolver.
func (r *DatabaseSerialResolver) GetEventIdBySerial(serial uint64) (eventId []byte, err error) {
	// Check cache first
	if eid, found := r.cache.GetEventIdBySerial(serial); found {
		return eid, nil
	}

	// Look up in database - use SerialEventId index
	ser := new(types.Uint40)
	if err = ser.Set(serial); err != nil {
		return nil, err
	}

	eventId, err = r.db.GetEventIdBySerial(ser)
	if err != nil {
		return nil, err
	}

	// Cache it
	r.cache.CacheEventId(serial, eventId)

	return eventId, nil
}

// GetEventIdBySerial looks up an event ID by its serial number.
// Uses the SerialEventId index (sei prefix).
func (d *D) GetEventIdBySerial(ser *types.Uint40) (eventId []byte, err error) {
	keyBuf := bufpool.GetSmall()
	defer bufpool.PutSmall(keyBuf)
	if err = indexes.SerialEventIdEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
		return nil, err
	}

	err = d.View(func(txn *badger.Txn) error {
		item, gerr := txn.Get(keyBuf.Bytes())
		if chk.E(gerr) {
			return gerr
		}

		return item.Value(func(val []byte) error {
			eventId = make([]byte, len(val))
			copy(eventId, val)
			return nil
		})
	})

	if err != nil {
		return nil, errors.New("event ID not found for serial")
	}

	return eventId, nil
}

// StoreEventIdSerial stores the mapping from event serial to full event ID.
// This is called during event save to enable later reconstruction.
func (d *D) StoreEventIdSerial(txn *badger.Txn, serial uint64, eventId []byte) error {
	if len(eventId) != 32 {
		return errors.New("event ID must be 32 bytes")
	}

	ser := new(types.Uint40)
	if err := ser.Set(serial); err != nil {
		return err
	}

	keyBuf := bufpool.GetSmall()
	defer bufpool.PutSmall(keyBuf)
	if err := indexes.SerialEventIdEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
		return err
	}

	return txn.Set(bufpool.CopyBytes(keyBuf), eventId)
}

// SerialCacheStats holds statistics about the serial cache.
type SerialCacheStats struct {
	PubkeysCached       int // Number of pubkeys currently cached
	PubkeysMaxSize      int // Maximum pubkey cache size
	EventIdsCached      int // Number of event IDs currently cached
	EventIdsMaxSize     int // Maximum event ID cache size
	PubkeyMemoryBytes   int // Estimated memory usage for pubkey cache
	EventIdMemoryBytes  int // Estimated memory usage for event ID cache
	TotalMemoryBytes    int // Total estimated memory usage
}

// Stats returns statistics about the serial cache.
func (c *SerialCache) Stats() SerialCacheStats {
	c.pubkeyBySerialLock.RLock()
	pubkeysCached := len(c.pubkeyBySerial)
	c.pubkeyBySerialLock.RUnlock()

	c.eventIdBySerialLock.RLock()
	eventIdsCached := len(c.eventIdBySerial)
	c.eventIdBySerialLock.RUnlock()

	// Memory estimation:
	// - Each pubkey entry: 8 bytes (uint64 key) + 32 bytes (pubkey value) = 40 bytes
	// - Each event ID entry: 8 bytes (uint64 key) + 32 bytes (event ID value) = 40 bytes
	// - Map overhead is roughly 2x the entry size for buckets
	pubkeyMemory := pubkeysCached * 40 * 2
	eventIdMemory := eventIdsCached * 40 * 2

	return SerialCacheStats{
		PubkeysCached:       pubkeysCached,
		PubkeysMaxSize:      c.maxPubkeys,
		EventIdsCached:      eventIdsCached,
		EventIdsMaxSize:     c.maxEventIds,
		PubkeyMemoryBytes:   pubkeyMemory,
		EventIdMemoryBytes:  eventIdMemory,
		TotalMemoryBytes:    pubkeyMemory + eventIdMemory,
	}
}

// SerialCacheStats returns statistics about the serial cache.
func (d *D) SerialCacheStats() SerialCacheStats {
	if d.serialCache == nil {
		return SerialCacheStats{}
	}
	return d.serialCache.Stats()
}
