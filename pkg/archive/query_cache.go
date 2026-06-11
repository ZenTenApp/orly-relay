package archive

import (
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
)

// queryHasQueriedReq is a request to check if a filter was recently queried.
type queryHasQueriedReq struct {
	fingerprint string
	resp        chan bool
}

// queryMarkQueriedReq is a request to mark a filter as queried.
type queryMarkQueriedReq struct {
	fingerprint string
}

// queryLenReq is a request for the current cache size.
type queryLenReq struct {
	resp chan int
}

// queryClearReq is a request to clear the cache.
type queryClearReq struct{}

// QueryCache tracks which filters have been queried recently to avoid
// repeated requests to archive relays for the same filter.
type QueryCache struct {
	hasQueried chan queryHasQueriedReq
	markQueried chan queryMarkQueriedReq
	lenReq      chan queryLenReq
	clearReq    chan queryClearReq
	stop        chan struct{}
	done        chan struct{}

	maxSize int
	ttl     time.Duration
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

	qc := &QueryCache{
		hasQueried:  make(chan queryHasQueriedReq),
		markQueried: make(chan queryMarkQueriedReq, 16),
		lenReq:      make(chan queryLenReq),
		clearReq:    make(chan queryClearReq),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		maxSize:     maxSize,
		ttl:         ttl,
	}

	go qc.actor()
	return qc
}

// actor owns the cache state and processes requests sequentially.
func (qc *QueryCache) actor() {
	defer close(qc.done)

	entries := make(map[string]*list.Element)
	order := list.New()

	for {
		select {
		case <-qc.stop:
			return

		case req := <-qc.hasQueried:
			elem, exists := entries[req.fingerprint]
			if !exists {
				req.resp <- false
				continue
			}
			entry := elem.Value.(*queryCacheEntry)
			if time.Since(entry.queriedAt) > qc.ttl {
				// Expired - remove it
				delete(entries, req.fingerprint)
				order.Remove(elem)
				req.resp <- false
				continue
			}
			req.resp <- true

		case req := <-qc.markQueried:
			// Update existing entry
			if elem, exists := entries[req.fingerprint]; exists {
				order.MoveToFront(elem)
				elem.Value.(*queryCacheEntry).queriedAt = time.Now()
				continue
			}
			// Evict oldest if at capacity
			if len(entries) >= qc.maxSize {
				oldest := order.Back()
				if oldest != nil {
					entry := oldest.Value.(*queryCacheEntry)
					delete(entries, entry.fingerprint)
					order.Remove(oldest)
				}
			}
			// Add new entry
			entry := &queryCacheEntry{
				fingerprint: req.fingerprint,
				queriedAt:   time.Now(),
			}
			elem := order.PushFront(entry)
			entries[req.fingerprint] = elem

		case req := <-qc.lenReq:
			req.resp <- len(entries)

		case <-qc.clearReq:
			entries = make(map[string]*list.Element)
			order.Init()
		}
	}
}

// HasQueried returns true if the filter was queried within the TTL.
func (qc *QueryCache) HasQueried(f *filter.F) bool {
	fingerprint := qc.normalizeAndHash(f)
	req := queryHasQueriedReq{
		fingerprint: fingerprint,
		resp:        make(chan bool, 1),
	}
	select {
	case qc.hasQueried <- req:
		return <-req.resp
	case <-qc.stop:
		return false
	}
}

// MarkQueried marks a filter as having been queried.
func (qc *QueryCache) MarkQueried(f *filter.F) {
	fingerprint := qc.normalizeAndHash(f)
	select {
	case qc.markQueried <- queryMarkQueriedReq{fingerprint: fingerprint}:
	case <-qc.stop:
	}
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
	req := queryLenReq{resp: make(chan int, 1)}
	select {
	case qc.lenReq <- req:
		return <-req.resp
	case <-qc.stop:
		return 0
	}
}

// MaxSize returns the maximum cache size.
func (qc *QueryCache) MaxSize() int {
	return qc.maxSize
}

// Clear removes all entries from the cache.
func (qc *QueryCache) Clear() {
	select {
	case qc.clearReq <- queryClearReq{}:
	case <-qc.stop:
	}
}

// Stop stops the query cache actor.
func (qc *QueryCache) Stop() {
	close(qc.stop)
	<-qc.done
}
