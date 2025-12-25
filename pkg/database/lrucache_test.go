//go:build !(js && wasm)

package database

import (
	"sync"
	"testing"
)

func TestLRUCache_BasicOperations(t *testing.T) {
	c := NewLRUCache[string, int](10)

	// Test Put and Get
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("Get('a') = %d, %v; want 1, true", v, ok)
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("Get('b') = %d, %v; want 2, true", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("Get('c') = %d, %v; want 3, true", v, ok)
	}

	// Test non-existent key
	if _, ok := c.Get("d"); ok {
		t.Error("Get('d') should return false for non-existent key")
	}

	// Test Len
	if c.Len() != 3 {
		t.Errorf("Len() = %d; want 3", c.Len())
	}
}

func TestLRUCache_Update(t *testing.T) {
	c := NewLRUCache[string, int](10)

	c.Put("a", 1)
	c.Put("a", 2) // Update

	if v, ok := c.Get("a"); !ok || v != 2 {
		t.Errorf("Get('a') = %d, %v; want 2, true", v, ok)
	}
	if c.Len() != 1 {
		t.Errorf("Len() = %d; want 1 (update should not add new entry)", c.Len())
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	c := NewLRUCache[int, string](3)

	// Fill cache
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")

	// All should be present
	if c.Len() != 3 {
		t.Errorf("Len() = %d; want 3", c.Len())
	}

	// Add one more - should evict "1" (oldest)
	c.Put(4, "four")

	if c.Len() != 3 {
		t.Errorf("Len() = %d; want 3 after eviction", c.Len())
	}

	// "1" should be evicted
	if _, ok := c.Get(1); ok {
		t.Error("Key 1 should have been evicted")
	}

	// Others should still be present
	if _, ok := c.Get(2); !ok {
		t.Error("Key 2 should still be present")
	}
	if _, ok := c.Get(3); !ok {
		t.Error("Key 3 should still be present")
	}
	if _, ok := c.Get(4); !ok {
		t.Error("Key 4 should be present")
	}
}

func TestLRUCache_LRUOrder(t *testing.T) {
	c := NewLRUCache[int, string](3)

	// Fill cache
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")

	// Access "1" - makes it most recent
	c.Get(1)

	// Add "4" - should evict "2" (now oldest)
	c.Put(4, "four")

	// "1" should still be present (was accessed recently)
	if _, ok := c.Get(1); !ok {
		t.Error("Key 1 should still be present after being accessed")
	}

	// "2" should be evicted
	if _, ok := c.Get(2); ok {
		t.Error("Key 2 should have been evicted (oldest)")
	}
}

func TestLRUCache_Delete(t *testing.T) {
	c := NewLRUCache[string, int](10)

	c.Put("a", 1)
	c.Put("b", 2)

	c.Delete("a")

	if _, ok := c.Get("a"); ok {
		t.Error("Key 'a' should be deleted")
	}
	if c.Len() != 1 {
		t.Errorf("Len() = %d; want 1", c.Len())
	}

	// Delete non-existent key should not panic
	c.Delete("nonexistent")
}

func TestLRUCache_Clear(t *testing.T) {
	c := NewLRUCache[int, int](10)

	for i := 0; i < 5; i++ {
		c.Put(i, i*10)
	}

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("Len() = %d; want 0 after Clear()", c.Len())
	}

	// Should be able to add after clear
	c.Put(100, 1000)
	if v, ok := c.Get(100); !ok || v != 1000 {
		t.Errorf("Get(100) = %d, %v; want 1000, true", v, ok)
	}
}

func TestLRUCache_Contains(t *testing.T) {
	c := NewLRUCache[string, int](10)

	c.Put("a", 1)

	if !c.Contains("a") {
		t.Error("Contains('a') should return true")
	}
	if c.Contains("b") {
		t.Error("Contains('b') should return false")
	}
}

func TestLRUCache_ByteArrayKey(t *testing.T) {
	// Test with [32]byte keys (like pubkeys/event IDs)
	c := NewLRUCache[[32]byte, uint64](100)

	var key1, key2 [32]byte
	key1[0] = 1
	key2[0] = 2

	c.Put(key1, 100)
	c.Put(key2, 200)

	if v, ok := c.Get(key1); !ok || v != 100 {
		t.Errorf("Get(key1) = %d, %v; want 100, true", v, ok)
	}
	if v, ok := c.Get(key2); !ok || v != 200 {
		t.Errorf("Get(key2) = %d, %v; want 200, true", v, ok)
	}
}

func TestLRUCache_Concurrent(t *testing.T) {
	c := NewLRUCache[int, int](1000)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Put(base*100+j, j)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Get(base*100 + j)
			}
		}(i)
	}

	wg.Wait()

	// Cache should not exceed max size
	if c.Len() > c.MaxSize() {
		t.Errorf("Len() = %d exceeds MaxSize() = %d", c.Len(), c.MaxSize())
	}
}

func BenchmarkLRUCache_Put(b *testing.B) {
	c := NewLRUCache[uint64, []byte](10000)
	value := make([]byte, 32)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.Put(uint64(i%10000), value)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	c := NewLRUCache[uint64, []byte](10000)
	value := make([]byte, 32)

	// Pre-fill cache
	for i := 0; i < 10000; i++ {
		c.Put(uint64(i), value)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.Get(uint64(i % 10000))
	}
}

func BenchmarkLRUCache_PutGet(b *testing.B) {
	c := NewLRUCache[uint64, []byte](10000)
	value := make([]byte, 32)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := uint64(i % 10000)
		c.Put(key, value)
		c.Get(key)
	}
}
