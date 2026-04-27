//go:build !(js && wasm)

package database

import (
	"errors"

	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/database/bufpool"
	"git.smesh.lol/orly/pkg/database/indexes"
	"git.smesh.lol/orly/pkg/database/indexes/types"
)

// SerialCache provides LRU caching for pubkey and event ID serial lookups.
// This is critical for compact event decoding performance since every event
// requires looking up the author pubkey and potentially multiple tag references.
//
// The cache uses LRU eviction and starts empty, growing on demand up to the
// configured limits. This provides better memory efficiency than pre-allocation
// and better hit rates than random eviction.
type SerialCache struct {
	// Pubkey serial -> full pubkey (for decoding)
	pubkeyBySerial *LRUCache[uint64, []byte]

	// Pubkey bytes -> serial (for encoding)
	// Uses [32]byte as key since []byte isn't comparable
	serialByPubkey *LRUCache[[32]byte, uint64]

	// Event serial -> full event ID (for decoding)
	eventIdBySerial *LRUCache[uint64, []byte]

	// Event ID bytes -> serial (for encoding)
	serialByEventId *LRUCache[[32]byte, uint64]

	// Limits (for stats reporting)
	maxPubkeys  int
	maxEventIds int
}

// NewSerialCache creates a new serial cache with the specified maximum sizes.
// The cache starts empty and grows on demand up to these limits.
func NewSerialCache(maxPubkeys, maxEventIds int) *SerialCache {
	if maxPubkeys <= 0 {
		maxPubkeys = 100000 // Default 100k pubkeys
	}
	if maxEventIds <= 0 {
		maxEventIds = 500000 // Default 500k event IDs
	}
	return &SerialCache{
		pubkeyBySerial:  NewLRUCache[uint64, []byte](maxPubkeys),
		serialByPubkey:  NewLRUCache[[32]byte, uint64](maxPubkeys),
		eventIdBySerial: NewLRUCache[uint64, []byte](maxEventIds),
		serialByEventId: NewLRUCache[[32]byte, uint64](maxEventIds),
		maxPubkeys:      maxPubkeys,
		maxEventIds:     maxEventIds,
	}
}

// CachePubkey adds a pubkey to the cache in both directions.
func (c *SerialCache) CachePubkey(serial uint64, pubkey []byte) {
	if len(pubkey) != 32 {
		return
	}

	// Copy pubkey to avoid referencing external slice
	pk := make([]byte, 32)
	copy(pk, pubkey)

	// Cache serial -> pubkey (for decoding)
	c.pubkeyBySerial.Put(serial, pk)

	// Cache pubkey -> serial (for encoding)
	var key [32]byte
	copy(key[:], pubkey)
	c.serialByPubkey.Put(key, serial)
}

// GetPubkeyBySerial returns the pubkey for a serial from cache.
func (c *SerialCache) GetPubkeyBySerial(serial uint64) (pubkey []byte, found bool) {
	return c.pubkeyBySerial.Get(serial)
}

// GetSerialByPubkey returns the serial for a pubkey from cache.
func (c *SerialCache) GetSerialByPubkey(pubkey []byte) (serial uint64, found bool) {
	if len(pubkey) != 32 {
		return 0, false
	}
	var key [32]byte
	copy(key[:], pubkey)
	return c.serialByPubkey.Get(key)
}

// CacheEventId adds an event ID to the cache in both directions.
func (c *SerialCache) CacheEventId(serial uint64, eventId []byte) {
	if len(eventId) != 32 {
		return
	}

	// Copy event ID to avoid referencing external slice
	eid := make([]byte, 32)
	copy(eid, eventId)

	// Cache serial -> event ID (for decoding)
	c.eventIdBySerial.Put(serial, eid)

	// Cache event ID -> serial (for encoding)
	var key [32]byte
	copy(key[:], eventId)
	c.serialByEventId.Put(key, serial)
}

// GetEventIdBySerial returns the event ID for a serial from cache.
func (c *SerialCache) GetEventIdBySerial(serial uint64) (eventId []byte, found bool) {
	return c.eventIdBySerial.Get(serial)
}

// GetSerialByEventId returns the serial for an event ID from cache.
func (c *SerialCache) GetSerialByEventId(eventId []byte) (serial uint64, found bool) {
	if len(eventId) != 32 {
		return 0, false
	}
	var key [32]byte
	copy(key[:], eventId)
	return c.serialByEventId.Get(key)
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
		if gerr != nil {
			// Don't log ErrKeyNotFound - it's expected for legacy events
			// that don't have SerialEventId mappings
			return gerr
		}

		return item.Value(func(val []byte) error {
			// Validate that the stored value is exactly 32 bytes
			if len(val) != 32 {
				return errors.New("corrupted event ID: expected 32 bytes")
			}
			eventId = make([]byte, 32)
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
	PubkeysCached      int // Number of pubkeys currently cached
	PubkeysMaxSize     int // Maximum pubkey cache size
	EventIdsCached     int // Number of event IDs currently cached
	EventIdsMaxSize    int // Maximum event ID cache size
	PubkeyMemoryBytes  int // Estimated memory usage for pubkey cache
	EventIdMemoryBytes int // Estimated memory usage for event ID cache
	TotalMemoryBytes   int // Total estimated memory usage
}

// Stats returns statistics about the serial cache.
func (c *SerialCache) Stats() SerialCacheStats {
	pubkeysCached := c.pubkeyBySerial.Len()
	eventIdsCached := c.eventIdBySerial.Len()

	// Memory estimation:
	// Each entry has: key + value + list.Element overhead + map entry overhead
	// - Pubkey by serial: 8 (key) + 32 (value) + ~80 (list) + ~16 (map) ≈ 136 bytes
	// - Serial by pubkey: 32 (key) + 8 (value) + ~80 (list) + ~16 (map) ≈ 136 bytes
	// Total per pubkey (both directions): ~272 bytes
	// Similarly for event IDs: ~272 bytes per entry (both directions)
	pubkeyMemory := pubkeysCached * 272
	eventIdMemory := eventIdsCached * 272

	return SerialCacheStats{
		PubkeysCached:      pubkeysCached,
		PubkeysMaxSize:     c.maxPubkeys,
		EventIdsCached:     eventIdsCached,
		EventIdsMaxSize:    c.maxEventIds,
		PubkeyMemoryBytes:  pubkeyMemory,
		EventIdMemoryBytes: eventIdMemory,
		TotalMemoryBytes:   pubkeyMemory + eventIdMemory,
	}
}

// SerialCacheStats returns statistics about the serial cache.
func (d *D) SerialCacheStats() SerialCacheStats {
	if d.serialCache == nil {
		return SerialCacheStats{}
	}
	return d.serialCache.Stats()
}
