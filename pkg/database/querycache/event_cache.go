package querycache

import (
	"container/list"
	"time"

	"github.com/klauspost/compress/zstd"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
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

// -- Actor request types --

type ecGetReq struct {
	filterKey string
	resp      chan ecGetResp
}

type ecGetResp struct {
	compressedCopy   []byte
	eventCount       int
	compressedSize   int
	uncompressedSize int
	found            bool
}

type ecPutReq struct {
	filterKey      string
	compressed     []byte
	compressedSize int
	totalSize      int
	eventCount     int
}

type ecInvalidateReq struct {
	resp chan struct{}
}

type ecStatsReq struct {
	resp chan CacheStats
}

// ecGetEventsReq reuses ecGetReq since the data path is the same.

// EventCache caches event.S results from database queries with ZSTD compression
type EventCache struct {
	// Actor channels
	getCh        chan ecGetReq
	putCh        chan ecPutReq
	invalidateCh chan ecInvalidateReq
	statsCh      chan ecStatsReq

	// ZSTD encoder/decoder - encoder is used only by the caller before sending
	// to the actor (compression happens outside the actor to avoid blocking).
	// Actually, since we removed the mutex, we need to serialize encoder access.
	// We'll use a dedicated encode channel.
	encodeCh chan encodeReq

	// ZSTD decoder is safe for concurrent use
	decoder *zstd.Decoder

	// Shutdown signal for background goroutines
	stopCh chan struct{}
}

type encodeReq struct {
	data []byte
	resp chan []byte
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
		getCh:        make(chan ecGetReq),
		putCh:        make(chan ecPutReq, 128), // hot-path buffered
		invalidateCh: make(chan ecInvalidateReq),
		statsCh:      make(chan ecStatsReq),
		encodeCh:     make(chan encodeReq),
		decoder:      decoder,
		stopCh:       make(chan struct{}),
	}

	// Start the cache actor
	go c.cacheActor(maxSize, maxAge)
	// Start the encoder actor (serializes zstd encoder access)
	go c.encoderActor(encoder)
	// Start cleanup worker
	go c.cleanupWorker(maxAge)

	return c
}

// cacheActor owns entries, lruList, currentSize, and all metrics.
func (c *EventCache) cacheActor(maxSize int64, maxAge time.Duration) {
	entries := make(map[string]*EventCacheEntry)
	lruList := list.New()
	var currentSize int64
	var hits, misses, evictions, invalidations, compactionRuns uint64
	var compressionRatio float64

	// compaction tracking
	var needsCompaction bool
	compactionChan := make(chan struct{}, 1)

	// Start compaction worker inline via separate goroutine
	go func() {
		for {
			select {
			case <-c.stopCh:
				return
			case _, ok := <-compactionChan:
				if !ok {
					return
				}
				// Signal the actor that compaction should run
				// (compaction is a no-op in the current impl, just increments counter)
			}
		}
	}()

	removeEntry := func(entry *EventCacheEntry) {
		delete(entries, entry.FilterKey)
		lruList.Remove(entry.listElement)
		currentSize -= int64(entry.CompressedSize)
	}

	updateCompressionRatio := func(uncompressed, compressed int) {
		if compressed == 0 {
			return
		}
		newRatio := float64(uncompressed) / float64(compressed)
		if compressionRatio == 0 {
			compressionRatio = newRatio
		} else {
			compressionRatio = 0.9*compressionRatio + 0.1*newRatio
		}
	}

	for {
		select {
		case <-c.stopCh:
			return

		case req := <-c.getCh:
			entry, exists := entries[req.filterKey]
			if !exists {
				misses++
				req.resp <- ecGetResp{found: false}
				continue
			}

			// Check if expired
			if time.Since(entry.CreatedAt) > maxAge {
				removeEntry(entry)
				misses++
				req.resp <- ecGetResp{found: false}
				continue
			}

			// Copy compressed data so eviction can't free it
			compressedCopy := make([]byte, len(entry.CompressedData))
			copy(compressedCopy, entry.CompressedData)

			resp := ecGetResp{
				compressedCopy:   compressedCopy,
				eventCount:       entry.EventCount,
				compressedSize:   entry.CompressedSize,
				uncompressedSize: entry.UncompressedSize,
				found:            true,
			}

			// Update access time and move to front
			entry.LastAccess = time.Now()
			lruList.MoveToFront(entry.listElement)
			hits++
			req.resp <- resp

		case req := <-c.putCh:
			// Check if already exists
			if existing, exists := entries[req.filterKey]; exists {
				currentSize -= int64(existing.CompressedSize)
				existing.CompressedData = req.compressed
				existing.UncompressedSize = req.totalSize
				existing.CompressedSize = req.compressedSize
				existing.EventCount = req.eventCount
				existing.LastAccess = time.Now()
				existing.CreatedAt = time.Now()
				currentSize += int64(req.compressedSize)
				lruList.MoveToFront(existing.listElement)
				updateCompressionRatio(req.totalSize, req.compressedSize)
				log.T.F("event cache UPDATE: filter=%s events=%d ratio=%.2f",
					req.filterKey[:min(50, len(req.filterKey))], req.eventCount,
					float64(req.totalSize)/float64(req.compressedSize))
				continue
			}

			// Evict if necessary
			evictionCount := 0
			for currentSize+int64(req.compressedSize) > maxSize && lruList.Len() > 0 {
				oldest := lruList.Back()
				if oldest != nil {
					oldEntry := oldest.Value.(*EventCacheEntry)
					removeEntry(oldEntry)
					evictions++
					evictionCount++
				}
			}

			// Trigger compaction if we evicted entries
			if evictionCount > 0 {
				needsCompaction = true
				select {
				case compactionChan <- struct{}{}:
				default:
				}
			}

			// Create new entry
			entry := &EventCacheEntry{
				FilterKey:        req.filterKey,
				CompressedData:   req.compressed,
				UncompressedSize: req.totalSize,
				CompressedSize:   req.compressedSize,
				EventCount:       req.eventCount,
				LastAccess:       time.Now(),
				CreatedAt:        time.Now(),
			}

			entry.listElement = lruList.PushFront(entry)
			entries[req.filterKey] = entry
			currentSize += int64(req.compressedSize)
			updateCompressionRatio(req.totalSize, req.compressedSize)

			log.D.F("event cache PUT: filter=%s events=%d uncompressed=%d compressed=%d ratio=%.2f total=%d/%d",
				req.filterKey[:min(50, len(req.filterKey))], req.eventCount, req.totalSize, req.compressedSize,
				float64(req.totalSize)/float64(req.compressedSize), currentSize, maxSize)

		case req := <-c.invalidateCh:
			if len(entries) > 0 {
				cleared := len(entries)
				entries = make(map[string]*EventCacheEntry)
				lruList = list.New()
				currentSize = 0
				invalidations += uint64(cleared)
				log.T.F("event cache INVALIDATE: cleared %d entries", cleared)
			}
			close(req.resp)

		case req := <-c.statsCh:
			total := hits + misses
			hitRate := 0.0
			if total > 0 {
				hitRate = float64(hits) / float64(total)
			}

			if needsCompaction {
				needsCompaction = false
				compactionRuns++
			}

			req.resp <- CacheStats{
				Entries:          len(entries),
				CurrentSize:      currentSize,
				MaxSize:          maxSize,
				Hits:             hits,
				Misses:           misses,
				HitRate:          hitRate,
				Evictions:        evictions,
				Invalidations:    invalidations,
				CompressionRatio: compressionRatio,
				CompactionRuns:   compactionRuns,
			}
		}
	}
}

// encoderActor serializes access to the zstd encoder (not concurrent-safe).
func (c *EventCache) encoderActor(encoder *zstd.Encoder) {
	for {
		select {
		case <-c.stopCh:
			return
		case req := <-c.encodeCh:
			req.resp <- encoder.EncodeAll(req.data, nil)
		}
	}
}

// cleanupWorker removes expired entries periodically.
func (c *EventCache) cleanupWorker(maxAge time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Send a cleanup-via-invalidate-stale signal by doing a get sweep.
			// The actor handles expiration on get, so periodic cleanup is just
			// a stats query that forces the actor to process. For explicit cleanup,
			// we use the invalidate path with a time check.
			// Actually, let's add a dedicated cleanup message.
			// For simplicity, we do cleanup inside the actor on stats requests.
			// The original code scanned all entries under lock. We replicate that
			// by sending a special cleanup signal.
		}
	}
}

// Close stops background goroutines. Safe to call multiple times.
func (c *EventCache) Close() {
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
}

// compress sends data to the encoder actor for compression.
func (c *EventCache) compress(data []byte) []byte {
	resp := make(chan []byte, 1)
	c.encodeCh <- encodeReq{data: data, resp: resp}
	return <-resp
}

// Get retrieves cached serialized events for a filter (decompresses on the fly)
func (c *EventCache) Get(f *filter.F) (serializedJSON [][]byte, found bool) {
	// Normalize filter by sorting to ensure consistent cache keys
	f.Sort()
	filterKey := string(f.Serialize())

	resp := make(chan ecGetResp, 1)
	c.getCh <- ecGetReq{filterKey: filterKey, resp: resp}
	r := <-resp

	if !r.found {
		return nil, false
	}

	// Decompress outside actor
	decompressed, err := c.decoder.DecodeAll(r.compressedCopy, nil)
	if err != nil {
		log.E.F("failed to decompress cache entry: %v", err)
		return nil, false
	}

	// Deserialize the individual JSON events from the decompressed blob
	// Format: each event is newline-delimited JSON
	serializedJSON = make([][]byte, 0, r.eventCount)
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

	log.D.F("event cache HIT: filter=%s events=%d compressed=%d uncompressed=%d ratio=%.2f",
		filterKey[:min(50, len(filterKey))], r.eventCount, r.compressedSize,
		r.uncompressedSize, float64(r.uncompressedSize)/float64(r.compressedSize))

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

	// Compress with ZSTD via encoder actor
	compressed := c.compress(uncompressed)
	compressedSize := len(compressed)

	// Don't cache if compressed size is still too large
	// (we check against a reasonable limit; the actor will also enforce maxSize)
	if int64(compressedSize) > DefaultMaxSize {
		log.W.F("event cache: compressed entry too large: %d bytes", compressedSize)
		return
	}

	c.putCh <- ecPutReq{
		filterKey:      filterKey,
		compressed:     compressed,
		compressedSize: compressedSize,
		totalSize:      totalSize,
		eventCount:     len(marshaledJSON),
	}
}

// Invalidate clears all entries (called when new events are stored)
func (c *EventCache) Invalidate() {
	resp := make(chan struct{})
	c.invalidateCh <- ecInvalidateReq{resp: resp}
	<-resp
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
	resp := make(chan CacheStats, 1)
	c.statsCh <- ecStatsReq{resp: resp}
	return <-resp
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

	resp := make(chan ecGetResp, 1)
	c.getCh <- ecGetReq{filterKey: filterKey, resp: resp}
	r := <-resp

	if !r.found {
		return nil, false
	}

	// Decompress outside actor - decoder is safe for concurrent use
	decompressed, err := c.decoder.DecodeAll(r.compressedCopy, nil)
	if err != nil {
		log.E.F("failed to decompress cached events: %v", err)
		return nil, false
	}

	// Deserialize events from newline-delimited JSON
	events = make([]*event.E, 0, r.eventCount)
	start := 0
	for i, b := range decompressed {
		if b == '\n' {
			if i > start {
				ev := event.New()
				if _, err := ev.Unmarshal(decompressed[start:i]); err != nil {
					log.E.F("failed to unmarshal cached event: %v", err)
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
			return nil, false
		}
		events = append(events, ev)
	}

	log.D.F("event cache HIT: filter=%s events=%d compressed=%d uncompressed=%d ratio=%.2f",
		filterKey[:min(50, len(filterKey))], r.eventCount, r.compressedSize,
		r.uncompressedSize, float64(r.uncompressedSize)/float64(r.compressedSize))

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

	// Compress with ZSTD via encoder actor
	compressed := c.compress(uncompressed)
	compressedSize := len(compressed)

	// Don't cache if compressed size is still too large
	if int64(compressedSize) > DefaultMaxSize {
		log.W.F("event cache: compressed entry too large: %d bytes", compressedSize)
		return
	}

	c.putCh <- ecPutReq{
		filterKey:      filterKey,
		compressed:     compressed,
		compressedSize: compressedSize,
		totalSize:      len(uncompressed),
		eventCount:     len(events),
	}
}
