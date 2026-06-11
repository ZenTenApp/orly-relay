package logbuffer

import (
	"time"

	"git.smesh.lol/actor"
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

type bufGetArgs struct {
	Offset, Limit int
}

// Buffer is a ring buffer for log entries.
// All mutable state is owned by the actor goroutine.
type Buffer struct {
	add   actor.Inbox[LogEntry]
	get   actor.Func[bufGetArgs, []LogEntry]
	clear actor.Inbox[struct{}]
	count actor.Query[int]
	actor.Lifecycle
	size int
}

// NewBuffer creates a new ring buffer with the specified size
func NewBuffer(size int) *Buffer {
	if size <= 0 {
		size = 10000
	}
	b := &Buffer{
		add:       actor.NewInbox[LogEntry](128),
		get:       actor.NewFunc[bufGetArgs, []LogEntry](),
		clear:     actor.NewInbox[struct{}](1),
		count:     actor.NewQuery[int](),
		Lifecycle: actor.NewLifecycle(),
		size:      size,
	}
	actor.Go(b.Lifecycle, b.actorLoop)
	return b
}

func (b *Buffer) actorLoop() {
	entries := make([]LogEntry, b.size)
	head := 0
	count := 0
	var nextID int64

	for {
		select {
		case <-b.Stopping():
			return
		case entry := <-b.add.Recv():
			nextID++
			entry.ID = nextID
			entries[head] = entry
			head = (head + 1) % b.size
			if count < b.size {
				count++
			}
		case msg := <-b.get.Recv():
			if count == 0 || msg.Req.Offset >= count {
				msg.Reply([]LogEntry{})
				continue
			}
			limit := msg.Req.Limit
			if limit <= 0 {
				limit = 100
			}
			available := count - msg.Req.Offset
			if limit > available {
				limit = available
			}
			result := make([]LogEntry, limit)
			for i := 0; i < limit; i++ {
				idx := (head - 1 - msg.Req.Offset - i + b.size*2) % b.size
				result[i] = entries[idx]
			}
			msg.Reply(result)
		case <-b.clear.Recv():
			head = 0
			count = 0
		case msg := <-b.count.Recv():
			msg.Reply(count)
		}
	}
}

// Shutdown stops the actor goroutine.
func (b *Buffer) Shutdown() {
	b.Stop()
}

// Add adds a log entry to the buffer
func (b *Buffer) Add(entry LogEntry) {
	b.add.Send(entry)
}

// Get returns log entries, newest first
func (b *Buffer) Get(offset, limit int) []LogEntry {
	return b.get.Call(bufGetArgs{Offset: offset, Limit: limit})
}

// Clear removes all entries from the buffer
func (b *Buffer) Clear() {
	b.clear.Send(struct{}{})
}

// Count returns the number of entries in the buffer
func (b *Buffer) Count() int {
	return b.count.Call()
}

// Global buffer instance
var GlobalBuffer *Buffer

// Init initializes the global log buffer
func Init(size int) {
	GlobalBuffer = NewBuffer(size)
}
