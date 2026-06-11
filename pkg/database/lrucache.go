//go:build !(js && wasm)

package database

import (
	"container/list"

	"git.smesh.lol/actor"
)

type lruGetResp[V any] struct {
	value V
	found bool
}

type lruPutArgs[K comparable, V any] struct {
	key   K
	value V
}

// lruEntry holds a key-value pair for the LRU list.
type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// LRUCache provides a thread-safe LRU cache with configurable max size.
// All mutable state is owned by the actor goroutine.
type LRUCache[K comparable, V any] struct {
	get      actor.Func[K, lruGetResp[V]]
	put      actor.Proc[lruPutArgs[K, V]]
	del      actor.Proc[K]
	length   actor.Query[int]
	clear    actor.Signal
	contains actor.Func[K, bool]
	actor.Lifecycle
	maxSize int
}

// NewLRUCache creates a new LRU cache with the given maximum size.
func NewLRUCache[K comparable, V any](maxSize int) *LRUCache[K, V] {
	if maxSize <= 0 {
		maxSize = 1000
	}
	c := &LRUCache[K, V]{
		get:       actor.NewFunc[K, lruGetResp[V]](),
		put:       actor.NewProc[lruPutArgs[K, V]](),
		del:       actor.NewProc[K](),
		length:    actor.NewQuery[int](),
		clear:     actor.NewSignal(),
		contains:  actor.NewFunc[K, bool](),
		Lifecycle: actor.NewLifecycle(),
		maxSize:   maxSize,
	}
	actor.Go(c.Lifecycle, c.actorLoop)
	return c
}

func (c *LRUCache[K, V]) actorLoop() {
	items := make(map[K]*list.Element)
	order := list.New()

	for {
		select {
		case <-c.Stopping():
			return
		case msg := <-c.get.Recv():
			if elem, ok := items[msg.Req]; ok {
				order.MoveToFront(elem)
				entry := elem.Value.(*lruEntry[K, V])
				msg.Reply(lruGetResp[V]{value: entry.value, found: true})
			} else {
				msg.Reply(lruGetResp[V]{})
			}
		case msg := <-c.put.Recv():
			if elem, ok := items[msg.Req.key]; ok {
				order.MoveToFront(elem)
				elem.Value.(*lruEntry[K, V]).value = msg.Req.value
			} else {
				if len(items) >= c.maxSize {
					oldest := order.Back()
					if oldest != nil {
						entry := oldest.Value.(*lruEntry[K, V])
						delete(items, entry.key)
						order.Remove(oldest)
					}
				}
				entry := &lruEntry[K, V]{key: msg.Req.key, value: msg.Req.value}
				elem := order.PushFront(entry)
				items[msg.Req.key] = elem
			}
			msg.Done()
		case msg := <-c.del.Recv():
			if elem, ok := items[msg.Req]; ok {
				delete(items, msg.Req)
				order.Remove(elem)
			}
			msg.Done()
		case msg := <-c.length.Recv():
			msg.Reply(len(items))
		case msg := <-c.clear.Recv():
			items = make(map[K]*list.Element)
			order.Init()
			msg.Done()
		case msg := <-c.contains.Recv():
			_, ok := items[msg.Req]
			msg.Reply(ok)
		}
	}
}

// Shutdown stops the actor goroutine.
func (c *LRUCache[K, V]) Shutdown() {
	c.Stop()
}

// Get retrieves a value by key and marks it as recently used.
func (c *LRUCache[K, V]) Get(key K) (value V, found bool) {
	r := c.get.Call(key)
	return r.value, r.found
}

// Put adds or updates a value, evicting the LRU entry if at capacity.
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.put.Call(lruPutArgs[K, V]{key: key, value: value})
}

// Delete removes an entry from the cache.
func (c *LRUCache[K, V]) Delete(key K) {
	c.del.Call(key)
}

// Len returns the current number of entries in the cache.
func (c *LRUCache[K, V]) Len() int {
	return c.length.Call()
}

// MaxSize returns the maximum capacity of the cache.
func (c *LRUCache[K, V]) MaxSize() int {
	return c.maxSize
}

// Clear removes all entries from the cache.
func (c *LRUCache[K, V]) Clear() {
	c.clear.Call()
}

// Contains returns true if the key exists in the cache without updating LRU order.
func (c *LRUCache[K, V]) Contains(key K) bool {
	return c.contains.Call(key)
}
