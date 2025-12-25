package logbuffer

import (
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	File      string    `json:"file,omitempty"`
	Line      int       `json:"line,omitempty"`
}

// Buffer is a thread-safe ring buffer for log entries
type Buffer struct {
	entries []LogEntry
	size    int
	head    int   // next write position
	count   int   // number of entries
	nextID  int64 // monotonic ID counter
	mu      sync.RWMutex
}

// NewBuffer creates a new ring buffer with the specified size
func NewBuffer(size int) *Buffer {
	if size <= 0 {
		size = 10000
	}
	return &Buffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

// Add adds a log entry to the buffer
func (b *Buffer) Add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	entry.ID = b.nextID

	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.size

	if b.count < b.size {
		b.count++
	}
}

// Get returns log entries, newest first
// offset is the number of entries to skip from the newest
// limit is the maximum number of entries to return
func (b *Buffer) Get(offset, limit int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 || offset >= b.count {
		return []LogEntry{}
	}

	if limit <= 0 {
		limit = 100
	}

	available := b.count - offset
	if limit > available {
		limit = available
	}

	result := make([]LogEntry, limit)

	// Start from the newest entry (head - 1) and go backwards
	for i := 0; i < limit; i++ {
		// Calculate index: newest is at (head - 1), skip offset entries
		idx := (b.head - 1 - offset - i + b.size*2) % b.size
		result[i] = b.entries[idx]
	}

	return result
}

// Clear removes all entries from the buffer
func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.head = 0
	b.count = 0
	// Note: we don't reset nextID to maintain monotonic IDs
}

// Count returns the number of entries in the buffer
func (b *Buffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Global buffer instance
var GlobalBuffer *Buffer

// Init initializes the global log buffer
func Init(size int) {
	GlobalBuffer = NewBuffer(size)
}
