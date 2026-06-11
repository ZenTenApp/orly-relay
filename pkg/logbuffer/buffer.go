package logbuffer

import (
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

// --- Actor request/response types ---

type bufAddReq struct {
	entry LogEntry
}

type bufGetReq struct {
	offset int
	limit  int
	resp   chan []LogEntry
}

type bufClearReq struct{}

type bufCountReq struct {
	resp chan int
}

// Buffer is a ring buffer for log entries.
// All mutable state is owned by the actor goroutine.
type Buffer struct {
	addCh   chan bufAddReq
	getCh   chan bufGetReq
	clearCh chan bufClearReq
	countCh chan bufCountReq
	stop    chan struct{}
	done    chan struct{}
	size    int
}

// NewBuffer creates a new ring buffer with the specified size
func NewBuffer(size int) *Buffer {
	if size <= 0 {
		size = 10000
	}
	b := &Buffer{
		addCh:   make(chan bufAddReq, 128),
		getCh:   make(chan bufGetReq),
		clearCh: make(chan bufClearReq, 1),
		countCh: make(chan bufCountReq),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		size:    size,
	}
	go b.actorLoop()
	return b
}

func (b *Buffer) actorLoop() {
	defer close(b.done)

	entries := make([]LogEntry, b.size)
	head := 0
	count := 0
	var nextID int64

	for {
		select {
		case <-b.stop:
			return
		case req := <-b.addCh:
			nextID++
			req.entry.ID = nextID
			entries[head] = req.entry
			head = (head + 1) % b.size
			if count < b.size {
				count++
			}
		case req := <-b.getCh:
			if count == 0 || req.offset >= count {
				req.resp <- []LogEntry{}
				continue
			}
			limit := req.limit
			if limit <= 0 {
				limit = 100
			}
			available := count - req.offset
			if limit > available {
				limit = available
			}
			result := make([]LogEntry, limit)
			for i := 0; i < limit; i++ {
				idx := (head - 1 - req.offset - i + b.size*2) % b.size
				result[i] = entries[idx]
			}
			req.resp <- result
		case <-b.clearCh:
			head = 0
			count = 0
		case req := <-b.countCh:
			req.resp <- count
		}
	}
}

// Shutdown stops the actor goroutine.
func (b *Buffer) Shutdown() {
	close(b.stop)
	<-b.done
}

// Add adds a log entry to the buffer
func (b *Buffer) Add(entry LogEntry) {
	b.addCh <- bufAddReq{entry: entry}
}

// Get returns log entries, newest first
func (b *Buffer) Get(offset, limit int) []LogEntry {
	req := bufGetReq{offset: offset, limit: limit, resp: make(chan []LogEntry, 1)}
	b.getCh <- req
	return <-req.resp
}

// Clear removes all entries from the buffer
func (b *Buffer) Clear() {
	b.clearCh <- bufClearReq{}
}

// Count returns the number of entries in the buffer
func (b *Buffer) Count() int {
	req := bufCountReq{resp: make(chan int, 1)}
	b.countCh <- req
	return <-req.resp
}

// Global buffer instance
var GlobalBuffer *Buffer

// Init initializes the global log buffer
func Init(size int) {
	GlobalBuffer = NewBuffer(size)
}
