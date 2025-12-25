//go:build !(js && wasm)

package database

import (
	"container/list"
	"sync"
)

// LRUCache provides a thread-safe LRU cache with configurable max size.
// It starts empty and grows on demand up to maxSize. When at capacity,
// the least recently used entry is evicted to make room for new entries.
type LRUCache[K comparable, V any] struct {
	mu      sync.Mutex
	items   map[K]*list.Element
	order   *list.List // Front = most recent, Back = least recent
	maxSize int
}

// lruEntry holds a key-value pair for the LRU list.
type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// NewLRUCache creates a new LRU cache with the given maximum size.
// The cache starts empty and grows on demand.
func NewLRUCache[K comparable, V any](maxSize int) *LRUCache[K, V] {
	if maxSize <= 0 {
		maxSize = 1000 // Default minimum
	}
	return &LRUCache[K, V]{
		items:   make(map[K]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
	}
}

// Get retrieves a value by key and marks it as recently used.
// Returns the value and true if found, zero value and false otherwise.
func (c *LRUCache[K, V]) Get(key K) (value V, found bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		entry := elem.Value.(*lruEntry[K, V])
		return entry.value, true
	}
	var zero V
	return zero, false
}

// Put adds or updates a value, evicting the LRU entry if at capacity.
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*lruEntry[K, V]).value = value
		return
	}

	// Evict LRU if at capacity
	if len(c.items) >= c.maxSize {
		oldest := c.order.Back()
		if oldest != nil {
			entry := oldest.Value.(*lruEntry[K, V])
			delete(c.items, entry.key)
			c.order.Remove(oldest)
		}
	}

	// Add new entry
	entry := &lruEntry[K, V]{key: key, value: value}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

// Delete removes an entry from the cache.
func (c *LRUCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		delete(c.items, key)
		c.order.Remove(elem)
	}
}

// Len returns the current number of entries in the cache.
func (c *LRUCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// MaxSize returns the maximum capacity of the cache.
func (c *LRUCache[K, V]) MaxSize() int {
	return c.maxSize
}

// Clear removes all entries from the cache.
func (c *LRUCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*list.Element)
	c.order.Init()
}

// Contains returns true if the key exists in the cache without updating LRU order.
func (c *LRUCache[K, V]) Contains(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[key]
	return ok
}
