//go:build !(js && wasm)

package database

import (
	"container/list"
)

// --- Actor request/response types ---

type lruGetReq[K comparable, V any] struct {
	key  K
	resp chan lruGetResp[V]
}

type lruGetResp[V any] struct {
	value V
	found bool
}

type lruPutReq[K comparable, V any] struct {
	key   K
	value V
	done  chan struct{}
}

type lruDeleteReq[K comparable] struct {
	key  K
	done chan struct{}
}

type lruLenReq struct {
	resp chan int
}

type lruClearReq struct {
	done chan struct{}
}

type lruContainsReq[K comparable] struct {
	key  K
	resp chan bool
}

// lruEntry holds a key-value pair for the LRU list.
type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// LRUCache provides a thread-safe LRU cache with configurable max size.
// All mutable state is owned by the actor goroutine.
type LRUCache[K comparable, V any] struct {
	getCh      chan lruGetReq[K, V]
	putCh      chan lruPutReq[K, V]
	deleteCh   chan lruDeleteReq[K]
	lenCh      chan lruLenReq
	clearCh    chan lruClearReq
	containsCh chan lruContainsReq[K]
	stop       chan struct{}
	done       chan struct{}
	maxSize    int
}

// NewLRUCache creates a new LRU cache with the given maximum size.
func NewLRUCache[K comparable, V any](maxSize int) *LRUCache[K, V] {
	if maxSize <= 0 {
		maxSize = 1000
	}
	c := &LRUCache[K, V]{
		getCh:      make(chan lruGetReq[K, V]),
		putCh:      make(chan lruPutReq[K, V]),
		deleteCh:   make(chan lruDeleteReq[K]),
		lenCh:      make(chan lruLenReq),
		clearCh:    make(chan lruClearReq),
		containsCh: make(chan lruContainsReq[K]),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		maxSize:    maxSize,
	}
	go c.actorLoop()
	return c
}

func (c *LRUCache[K, V]) actorLoop() {
	defer close(c.done)

	items := make(map[K]*list.Element)
	order := list.New()

	for {
		select {
		case <-c.stop:
			return
		case req := <-c.getCh:
			if elem, ok := items[req.key]; ok {
				order.MoveToFront(elem)
				entry := elem.Value.(*lruEntry[K, V])
				req.resp <- lruGetResp[V]{value: entry.value, found: true}
			} else {
				req.resp <- lruGetResp[V]{}
			}
		case req := <-c.putCh:
			if elem, ok := items[req.key]; ok {
				order.MoveToFront(elem)
				elem.Value.(*lruEntry[K, V]).value = req.value
			} else {
				if len(items) >= c.maxSize {
					oldest := order.Back()
					if oldest != nil {
						entry := oldest.Value.(*lruEntry[K, V])
						delete(items, entry.key)
						order.Remove(oldest)
					}
				}
				entry := &lruEntry[K, V]{key: req.key, value: req.value}
				elem := order.PushFront(entry)
				items[req.key] = elem
			}
			close(req.done)
		case req := <-c.deleteCh:
			if elem, ok := items[req.key]; ok {
				delete(items, req.key)
				order.Remove(elem)
			}
			close(req.done)
		case req := <-c.lenCh:
			req.resp <- len(items)
		case req := <-c.clearCh:
			items = make(map[K]*list.Element)
			order.Init()
			close(req.done)
		case req := <-c.containsCh:
			_, ok := items[req.key]
			req.resp <- ok
		}
	}
}

// Shutdown stops the actor goroutine.
func (c *LRUCache[K, V]) Shutdown() {
	close(c.stop)
	<-c.done
}

// Get retrieves a value by key and marks it as recently used.
func (c *LRUCache[K, V]) Get(key K) (value V, found bool) {
	req := lruGetReq[K, V]{key: key, resp: make(chan lruGetResp[V], 1)}
	c.getCh <- req
	r := <-req.resp
	return r.value, r.found
}

// Put adds or updates a value, evicting the LRU entry if at capacity.
func (c *LRUCache[K, V]) Put(key K, value V) {
	done := make(chan struct{})
	c.putCh <- lruPutReq[K, V]{key: key, value: value, done: done}
	<-done
}

// Delete removes an entry from the cache.
func (c *LRUCache[K, V]) Delete(key K) {
	done := make(chan struct{})
	c.deleteCh <- lruDeleteReq[K]{key: key, done: done}
	<-done
}

// Len returns the current number of entries in the cache.
func (c *LRUCache[K, V]) Len() int {
	req := lruLenReq{resp: make(chan int, 1)}
	c.lenCh <- req
	return <-req.resp
}

// MaxSize returns the maximum capacity of the cache.
func (c *LRUCache[K, V]) MaxSize() int {
	return c.maxSize
}

// Clear removes all entries from the cache.
func (c *LRUCache[K, V]) Clear() {
	done := make(chan struct{})
	c.clearCh <- lruClearReq{done: done}
	<-done
}

// Contains returns true if the key exists in the cache without updating LRU order.
func (c *LRUCache[K, V]) Contains(key K) bool {
	req := lruContainsReq[K]{key: key, resp: make(chan bool, 1)}
	c.containsCh <- req
	return <-req.resp
}
