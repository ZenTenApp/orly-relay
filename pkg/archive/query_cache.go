package archive

import (
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
)

// QueryCache tracks which filters have been queried recently to avoid
// repeated requests to archive relays for the same filter.
type QueryCache struct {
	mu       sync.RWMutex
	entries  map[string]*list.Element
	order    *list.List
	maxSize  int
	ttl      time.Duration
}

// queryCacheEntry holds a cached query fingerprint and timestamp.
type queryCacheEntry struct {
	fingerprint string
	queriedAt   time.Time
}

// NewQueryCache creates a new query cache.
func NewQueryCache(ttl time.Duration, maxSize int) *QueryCache {
	if maxSize <= 0 {
		maxSize = 100000
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	return &QueryCache{
		entries: make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// HasQueried returns true if the filter was queried within the TTL.
func (qc *QueryCache) HasQueried(f *filter.F) bool {
	fingerprint := qc.normalizeAndHash(f)

	qc.mu.RLock()
	elem, exists := qc.entries[fingerprint]
	qc.mu.RUnlock()

	if !exists {
		return false
	}

	entry := elem.Value.(*queryCacheEntry)

	// Check if still within TTL
	if time.Since(entry.queriedAt) > qc.ttl {
		// Expired - remove it
		qc.mu.Lock()
		if elem, exists := qc.entries[fingerprint]; exists {
			delete(qc.entries, fingerprint)
			qc.order.Remove(elem)
		}
		qc.mu.Unlock()
		return false
	}

	return true
}

// MarkQueried marks a filter as having been queried.
func (qc *QueryCache) MarkQueried(f *filter.F) {
	fingerprint := qc.normalizeAndHash(f)

	qc.mu.Lock()
	defer qc.mu.Unlock()

	// Update existing entry
	if elem, exists := qc.entries[fingerprint]; exists {
		qc.order.MoveToFront(elem)
		elem.Value.(*queryCacheEntry).queriedAt = time.Now()
		return
	}

	// Evict oldest if at capacity
	if len(qc.entries) >= qc.maxSize {
		oldest := qc.order.Back()
		if oldest != nil {
			entry := oldest.Value.(*queryCacheEntry)
			delete(qc.entries, entry.fingerprint)
			qc.order.Remove(oldest)
		}
	}

	// Add new entry
	entry := &queryCacheEntry{
		fingerprint: fingerprint,
		queriedAt:   time.Now(),
	}
	elem := qc.order.PushFront(entry)
	qc.entries[fingerprint] = elem
}

// normalizeAndHash creates a canonical fingerprint for a filter.
// This ensures that differently-ordered filters with the same content
// produce identical fingerprints.
func (qc *QueryCache) normalizeAndHash(f *filter.F) string {
	h := sha256.New()

	// Normalize and hash IDs (sorted)
	if f.Ids != nil && f.Ids.Len() > 0 {
		ids := make([]string, 0, f.Ids.Len())
		for _, id := range f.Ids.T {
			ids = append(ids, string(id))
		}
		sort.Strings(ids)
		h.Write([]byte("ids:"))
		for _, id := range ids {
			h.Write([]byte(id))
		}
	}

	// Normalize and hash Authors (sorted)
	if f.Authors != nil && f.Authors.Len() > 0 {
		authors := make([]string, 0, f.Authors.Len())
		for _, author := range f.Authors.T {
			authors = append(authors, string(author))
		}
		sort.Strings(authors)
		h.Write([]byte("authors:"))
		for _, a := range authors {
			h.Write([]byte(a))
		}
	}

	// Normalize and hash Kinds (sorted)
	if f.Kinds != nil && f.Kinds.Len() > 0 {
		kinds := f.Kinds.ToUint16()
		sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
		h.Write([]byte("kinds:"))
		for _, k := range kinds {
			var buf [2]byte
			binary.BigEndian.PutUint16(buf[:], k)
			h.Write(buf[:])
		}
	}

	// Normalize and hash Tags (sorted by key, then values)
	if f.Tags != nil && f.Tags.Len() > 0 {
		// Collect all tag keys and sort them
		tagMap := make(map[string][]string)
		for _, t := range *f.Tags {
			if t.Len() > 0 {
				key := string(t.Key())
				values := make([]string, 0, t.Len()-1)
				for j := 1; j < t.Len(); j++ {
					values = append(values, string(t.T[j]))
				}
				sort.Strings(values)
				tagMap[key] = values
			}
		}

		// Sort keys and hash
		keys := make([]string, 0, len(tagMap))
		for k := range tagMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		h.Write([]byte("tags:"))
		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte(":"))
			for _, v := range tagMap[k] {
				h.Write([]byte(v))
			}
		}
	}

	// Hash Since timestamp
	if f.Since != nil {
		h.Write([]byte("since:"))
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(f.Since.V))
		h.Write(buf[:])
	}

	// Hash Until timestamp
	if f.Until != nil {
		h.Write([]byte("until:"))
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(f.Until.V))
		h.Write(buf[:])
	}

	// Hash Limit
	if f.Limit != nil && *f.Limit > 0 {
		h.Write([]byte("limit:"))
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(*f.Limit))
		h.Write(buf[:])
	}

	// Hash Search (NIP-50)
	if len(f.Search) > 0 {
		h.Write([]byte("search:"))
		h.Write(f.Search)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// Len returns the number of cached queries.
func (qc *QueryCache) Len() int {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return len(qc.entries)
}

// MaxSize returns the maximum cache size.
func (qc *QueryCache) MaxSize() int {
	return qc.maxSize
}

// Clear removes all entries from the cache.
func (qc *QueryCache) Clear() {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.entries = make(map[string]*list.Element)
	qc.order.Init()
}
