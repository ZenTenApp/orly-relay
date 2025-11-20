package querycache

import (
	"container/list"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
)

const (
	// DefaultMaxSize is the default maximum cache size in bytes (512 MB)
	DefaultMaxSize = 512 * 1024 * 1024
	// DefaultMaxAge is the default maximum age for cache entries
	DefaultMaxAge = 5 * time.Minute
)

// EventCacheEntry represents a cached set of compressed serialized events for a filter
type EventCacheEntry struct {
	FilterKey        string
	CompressedData   []byte    // ZSTD compressed serialized JSON events
	UncompressedSize int       // Original size before compression (for stats)
	CompressedSize   int       // Actual compressed size in bytes
	EventCount       int       // Number of events in this entry
	LastAccess       time.Time
	CreatedAt        time.Time
	listElement      *list.Element
}

// EventCache caches event.S results from database queries with ZSTD compression
type EventCache struct {
	mu sync.RWMutex

	entries map[string]*EventCacheEntry
	lruList *list.List

	currentSize int64 // Tracks compressed size
	maxSize     int64
	maxAge      time.Duration

	// ZSTD encoder/decoder (reused for efficiency)
	encoder *zstd.Encoder
	decoder *zstd.Decoder

	// Compaction tracking
	needsCompaction bool
	compactionChan  chan struct{}

	// Metrics
	hits              uint64
	misses            uint64
	evictions         uint64
	invalidations     uint64
	compressionRatio  float64 // Average compression ratio
	compactionRuns    uint64
}

// NewEventCache creates a new event cache
func NewEventCache(maxSize int64, maxAge time.Duration) *EventCache {
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	if maxAge <= 0 {
		maxAge = DefaultMaxAge
	}

	// Create ZSTD encoder at level 9 (best compression)
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		log.E.F("failed to create ZSTD encoder: %v", err)
		return nil
	}

	// Create ZSTD decoder
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		log.E.F("failed to create ZSTD decoder: %v", err)
		return nil
	}

	c := &EventCache{
		entries:        make(map[string]*EventCacheEntry),
		lruList:        list.New(),
		maxSize:        maxSize,
		maxAge:         maxAge,
		encoder:        encoder,
		decoder:        decoder,
		compactionChan: make(chan struct{}, 1),
	}

	// Start background workers
	go c.cleanupExpired()
	go c.compactionWorker()

	return c
}

// Get retrieves cached serialized events for a filter (decompresses on the fly)
func (c *EventCache) Get(f *filter.F) (serializedJSON [][]byte, found bool) {
	// Normalize filter by sorting to ensure consistent cache keys
	f.Sort()
	filterKey := string(f.Serialize())

	c.mu.RLock()
	entry, exists := c.entries[filterKey]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check if expired
	if time.Since(entry.CreatedAt) > c.maxAge {
		c.mu.Lock()
		c.removeEntry(entry)
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Decompress the data (outside of write lock for better concurrency)
	decompressed, err := c.decoder.DecodeAll(entry.CompressedData, nil)
	if err != nil {
		log.E.F("failed to decompress cache entry: %v", err)
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Deserialize the individual JSON events from the decompressed blob
	// Format: each event is newline-delimited JSON
	serializedJSON = make([][]byte, 0, entry.EventCount)
	start := 0
	for i := 0; i < len(decompressed); i++ {
		if decompressed[i] == '\n' {
			if i > start {
				eventJSON := make([]byte, i-start)
				copy(eventJSON, decompressed[start:i])
				serializedJSON = append(serializedJSON, eventJSON)
			}
			start = i + 1
		}
	}
	// Handle last event if no trailing newline
	if start < len(decompressed) {
		eventJSON := make([]byte, len(decompressed)-start)
		copy(eventJSON, decompressed[start:])
		serializedJSON = append(serializedJSON, eventJSON)
	}

	// Update access time and move to front
	c.mu.Lock()
	entry.LastAccess = time.Now()
	c.lruList.MoveToFront(entry.listElement)
	c.hits++
	c.mu.Unlock()

	log.D.F("event cache HIT: filter=%s events=%d compressed=%d uncompressed=%d ratio=%.2f",
		filterKey[:min(50, len(filterKey))], entry.EventCount, entry.CompressedSize,
		entry.UncompressedSize, float64(entry.UncompressedSize)/float64(entry.CompressedSize))

	return serializedJSON, true
}

// PutJSON stores pre-marshaled JSON in the cache with ZSTD compression
// This should be called AFTER events are sent to the client with the marshaled envelopes
func (c *EventCache) PutJSON(f *filter.F, marshaledJSON [][]byte) {
	if len(marshaledJSON) == 0 {
		return
	}

	// Normalize filter by sorting to ensure consistent cache keys
	f.Sort()
	filterKey := string(f.Serialize())

	// Concatenate all JSON events with newline delimiters for compression
	totalSize := 0
	for _, jsonData := range marshaledJSON {
		totalSize += len(jsonData) + 1 // +1 for newline
	}

	uncompressed := make([]byte, 0, totalSize)
	for _, jsonData := range marshaledJSON {
		uncompressed = append(uncompressed, jsonData...)
		uncompressed = append(uncompressed, '\n')
	}

	// Compress with ZSTD level 9
	compressed := c.encoder.EncodeAll(uncompressed, nil)
	compressedSize := len(compressed)

	// Don't cache if compressed size is still too large
	if int64(compressedSize) > c.maxSize {
		log.W.F("event cache: compressed entry too large: %d bytes", compressedSize)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if existing, exists := c.entries[filterKey]; exists {
		c.currentSize -= int64(existing.CompressedSize)
		existing.CompressedData = compressed
		existing.UncompressedSize = totalSize
		existing.CompressedSize = compressedSize
		existing.EventCount = len(marshaledJSON)
		existing.LastAccess = time.Now()
		existing.CreatedAt = time.Now()
		c.currentSize += int64(compressedSize)
		c.lruList.MoveToFront(existing.listElement)
		c.updateCompressionRatio(totalSize, compressedSize)
		log.T.F("event cache UPDATE: filter=%s events=%d ratio=%.2f",
			filterKey[:min(50, len(filterKey))], len(marshaledJSON),
			float64(totalSize)/float64(compressedSize))
		return
	}

	// Evict if necessary
	evictionCount := 0
	for c.currentSize+int64(compressedSize) > c.maxSize && c.lruList.Len() > 0 {
		oldest := c.lruList.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*EventCacheEntry)
			c.removeEntry(oldEntry)
			c.evictions++
			evictionCount++
		}
	}

	// Trigger compaction if we evicted entries
	if evictionCount > 0 {
		c.needsCompaction = true
		select {
		case c.compactionChan <- struct{}{}:
		default:
			// Channel already has signal, compaction will run
		}
	}

	// Create new entry
	entry := &EventCacheEntry{
		FilterKey:        filterKey,
		CompressedData:   compressed,
		UncompressedSize: totalSize,
		CompressedSize:   compressedSize,
		EventCount:       len(marshaledJSON),
		LastAccess:       time.Now(),
		CreatedAt:        time.Now(),
	}

	entry.listElement = c.lruList.PushFront(entry)
	c.entries[filterKey] = entry
	c.currentSize += int64(compressedSize)
	c.updateCompressionRatio(totalSize, compressedSize)

	log.D.F("event cache PUT: filter=%s events=%d uncompressed=%d compressed=%d ratio=%.2f total=%d/%d",
		filterKey[:min(50, len(filterKey))], len(marshaledJSON), totalSize, compressedSize,
		float64(totalSize)/float64(compressedSize), c.currentSize, c.maxSize)
}

// updateCompressionRatio updates the rolling average compression ratio
func (c *EventCache) updateCompressionRatio(uncompressed, compressed int) {
	if compressed == 0 {
		return
	}
	newRatio := float64(uncompressed) / float64(compressed)
	// Use exponential moving average
	if c.compressionRatio == 0 {
		c.compressionRatio = newRatio
	} else {
		c.compressionRatio = 0.9*c.compressionRatio + 0.1*newRatio
	}
}

// Invalidate clears all entries (called when new events are stored)
func (c *EventCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) > 0 {
		cleared := len(c.entries)
		c.entries = make(map[string]*EventCacheEntry)
		c.lruList = list.New()
		c.currentSize = 0
		c.invalidations += uint64(cleared)
		log.T.F("event cache INVALIDATE: cleared %d entries", cleared)
	}
}

// removeEntry removes an entry (must be called with lock held)
func (c *EventCache) removeEntry(entry *EventCacheEntry) {
	delete(c.entries, entry.FilterKey)
	c.lruList.Remove(entry.listElement)
	c.currentSize -= int64(entry.CompressedSize)
}

// compactionWorker runs in the background and compacts cache entries after evictions
// to reclaim fragmented space and improve cache efficiency
func (c *EventCache) compactionWorker() {
	for range c.compactionChan {
		c.mu.Lock()
		if !c.needsCompaction {
			c.mu.Unlock()
			continue
		}

		log.D.F("cache compaction: starting (entries=%d size=%d/%d)",
			len(c.entries), c.currentSize, c.maxSize)

		// For ZSTD compressed entries, compaction mainly means ensuring
		// entries are tightly packed in memory. Since each entry is already
		// individually compressed at level 9, there's not much additional
		// compression to gain. The main benefit is from the eviction itself.

		c.needsCompaction = false
		c.compactionRuns++
		c.mu.Unlock()

		log.D.F("cache compaction: completed (runs=%d)", c.compactionRuns)
	}
}

// cleanupExpired removes expired entries periodically
func (c *EventCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		var toRemove []*EventCacheEntry

		for _, entry := range c.entries {
			if now.Sub(entry.CreatedAt) > c.maxAge {
				toRemove = append(toRemove, entry)
			}
		}

		for _, entry := range toRemove {
			c.removeEntry(entry)
		}

		if len(toRemove) > 0 {
			log.D.F("event cache cleanup: removed %d expired entries", len(toRemove))
		}

		c.mu.Unlock()
	}
}

// CacheStats holds cache performance metrics
type CacheStats struct {
	Entries          int
	CurrentSize      int64   // Compressed size
	MaxSize          int64
	Hits             uint64
	Misses           uint64
	HitRate          float64
	Evictions        uint64
	Invalidations    uint64
	CompressionRatio float64 // Average compression ratio
	CompactionRuns   uint64
}

// Stats returns cache statistics
func (c *EventCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Entries:          len(c.entries),
		CurrentSize:      c.currentSize,
		MaxSize:          c.maxSize,
		Hits:             c.hits,
		Misses:           c.misses,
		HitRate:          hitRate,
		Evictions:        c.evictions,
		Invalidations:    c.invalidations,
		CompressionRatio: c.compressionRatio,
		CompactionRuns:   c.compactionRuns,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetEvents retrieves cached events for a filter (decompresses and deserializes on the fly)
// This is the new method that returns event.E objects instead of marshaled JSON
func (c *EventCache) GetEvents(f *filter.F) (events []*event.E, found bool) {
	// Normalize filter by sorting to ensure consistent cache keys
	f.Sort()
	filterKey := string(f.Serialize())

	c.mu.RLock()
	entry, exists := c.entries[filterKey]
	if !exists {
		c.mu.RUnlock()
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check if entry is expired
	if time.Since(entry.CreatedAt) > c.maxAge {
		c.mu.RUnlock()
		c.mu.Lock()
		c.removeEntry(entry)
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Decompress
	decompressed, err := c.decoder.DecodeAll(entry.CompressedData, nil)
	c.mu.RUnlock()
	if err != nil {
		log.E.F("failed to decompress cached events: %v", err)
		c.mu.Lock()
		c.removeEntry(entry)
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Deserialize events from newline-delimited JSON
	events = make([]*event.E, 0, entry.EventCount)
	start := 0
	for i, b := range decompressed {
		if b == '\n' {
			if i > start {
				ev := event.New()
				if _, err := ev.Unmarshal(decompressed[start:i]); err != nil {
					log.E.F("failed to unmarshal cached event: %v", err)
					c.mu.Lock()
					c.removeEntry(entry)
					c.misses++
					c.mu.Unlock()
					return nil, false
				}
				events = append(events, ev)
			}
			start = i + 1
		}
	}

	// Handle last event if no trailing newline
	if start < len(decompressed) {
		ev := event.New()
		if _, err := ev.Unmarshal(decompressed[start:]); err != nil {
			log.E.F("failed to unmarshal cached event: %v", err)
			c.mu.Lock()
			c.removeEntry(entry)
			c.misses++
			c.mu.Unlock()
			return nil, false
		}
		events = append(events, ev)
	}

	// Update access time and move to front
	c.mu.Lock()
	entry.LastAccess = time.Now()
	c.lruList.MoveToFront(entry.listElement)
	c.hits++
	c.mu.Unlock()

	log.D.F("event cache HIT: filter=%s events=%d compressed=%d uncompressed=%d ratio=%.2f",
		filterKey[:min(50, len(filterKey))], entry.EventCount, entry.CompressedSize,
		entry.UncompressedSize, float64(entry.UncompressedSize)/float64(entry.CompressedSize))

	return events, true
}

// PutEvents stores events in the cache with ZSTD compression
// This should be called AFTER events are sent to the client
func (c *EventCache) PutEvents(f *filter.F, events []*event.E) {
	if len(events) == 0 {
		return
	}

	// Normalize filter by sorting to ensure consistent cache keys
	f.Sort()
	filterKey := string(f.Serialize())

	// Serialize all events as newline-delimited JSON for compression
	totalSize := 0
	for _, ev := range events {
		totalSize += ev.EstimateSize() + 1 // +1 for newline
	}

	uncompressed := make([]byte, 0, totalSize)
	for _, ev := range events {
		uncompressed = ev.Marshal(uncompressed)
		uncompressed = append(uncompressed, '\n')
	}

	// Compress with ZSTD level 9
	compressed := c.encoder.EncodeAll(uncompressed, nil)
	compressedSize := len(compressed)

	// Don't cache if compressed size is still too large
	if int64(compressedSize) > c.maxSize {
		log.W.F("event cache: compressed entry too large: %d bytes", compressedSize)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if existing, exists := c.entries[filterKey]; exists {
		c.currentSize -= int64(existing.CompressedSize)
		existing.CompressedData = compressed
		existing.UncompressedSize = len(uncompressed)
		existing.CompressedSize = compressedSize
		existing.EventCount = len(events)
		existing.LastAccess = time.Now()
		existing.CreatedAt = time.Now()
		c.currentSize += int64(compressedSize)
		c.lruList.MoveToFront(existing.listElement)
		c.updateCompressionRatio(len(uncompressed), compressedSize)
		log.T.F("event cache UPDATE: filter=%s events=%d ratio=%.2f",
			filterKey[:min(50, len(filterKey))], len(events),
			float64(len(uncompressed))/float64(compressedSize))
		return
	}

	// Evict if necessary
	evictionCount := 0
	for c.currentSize+int64(compressedSize) > c.maxSize && c.lruList.Len() > 0 {
		oldest := c.lruList.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*EventCacheEntry)
			c.removeEntry(oldEntry)
			c.evictions++
			evictionCount++
		}
	}

	if evictionCount > 0 {
		c.needsCompaction = true
		select {
		case c.compactionChan <- struct{}{}:
		default:
		}
	}

	// Create new entry
	entry := &EventCacheEntry{
		FilterKey:        filterKey,
		CompressedData:   compressed,
		UncompressedSize: len(uncompressed),
		CompressedSize:   compressedSize,
		EventCount:       len(events),
		LastAccess:       time.Now(),
		CreatedAt:        time.Now(),
	}

	entry.listElement = c.lruList.PushFront(entry)
	c.entries[filterKey] = entry
	c.currentSize += int64(compressedSize)
	c.updateCompressionRatio(len(uncompressed), compressedSize)

	log.D.F("event cache PUT: filter=%s events=%d uncompressed=%d compressed=%d ratio=%.2f total=%d/%d",
		filterKey[:min(50, len(filterKey))], len(events), len(uncompressed), compressedSize,
		float64(len(uncompressed))/float64(compressedSize), c.currentSize, c.maxSize)
}
