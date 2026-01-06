//go:build !(js && wasm)

package bbolt

import (
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"lol.mleku.dev/chk"
)

// BatchedWrite represents a single write operation
type BatchedWrite struct {
	BucketName []byte
	Key        []byte
	Value      []byte
	IsDelete   bool
}

// EventBatch represents a complete event with all its indexes and graph updates
type EventBatch struct {
	Serial       uint64
	EventData    []byte         // Serialized compact event data
	Indexes      []BatchedWrite // Index entries
	EventVertex  *EventVertex   // Graph vertex for this event
	PubkeyUpdate *PubkeyVertexUpdate // Update to author's pubkey vertex
	MentionUpdates []*PubkeyVertexUpdate // Updates to mentioned pubkeys
	EdgeKeys     []EdgeKey      // Edge keys for bloom filter
}

// PubkeyVertexUpdate represents an update to a pubkey's vertex
type PubkeyVertexUpdate struct {
	PubkeySerial uint64
	AddAuthored  uint64 // Event serial to add to authored (0 if none)
	AddMention   uint64 // Event serial to add to mentions (0 if none)
}

// WriteBatcher accumulates writes and flushes them in batches.
// Optimized for HDD with large batches and periodic flushes.
type WriteBatcher struct {
	db        *bolt.DB
	bloom     *EdgeBloomFilter
	logger    *Logger

	mu          sync.Mutex
	pending     []*EventBatch
	pendingSize int64
	stopped     bool

	// Configuration
	maxEvents   int
	maxBytes    int64
	flushPeriod time.Duration

	// Channels for coordination
	flushCh    chan struct{}
	shutdownCh chan struct{}
	doneCh     chan struct{}

	// Stats
	stats BatcherStats
}

// BatcherStats contains batcher statistics
type BatcherStats struct {
	TotalBatches      uint64
	TotalEvents       uint64
	TotalBytes        uint64
	AverageLatencyMs  float64
	LastFlushTime     time.Time
	LastFlushDuration time.Duration
}

// NewWriteBatcher creates a new write batcher
func NewWriteBatcher(db *bolt.DB, bloom *EdgeBloomFilter, cfg *BboltConfig, logger *Logger) *WriteBatcher {
	wb := &WriteBatcher{
		db:          db,
		bloom:       bloom,
		logger:      logger,
		maxEvents:   cfg.BatchMaxEvents,
		maxBytes:    cfg.BatchMaxBytes,
		flushPeriod: cfg.BatchFlushTimeout,
		pending:     make([]*EventBatch, 0, cfg.BatchMaxEvents),
		flushCh:     make(chan struct{}, 1),
		shutdownCh:  make(chan struct{}),
		doneCh:      make(chan struct{}),
	}

	go wb.flushLoop()
	return wb
}

// Add adds an event batch to the pending writes
func (wb *WriteBatcher) Add(batch *EventBatch) error {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.stopped {
		return ErrBatcherStopped
	}

	wb.pending = append(wb.pending, batch)
	wb.pendingSize += int64(len(batch.EventData))
	for _, idx := range batch.Indexes {
		wb.pendingSize += int64(len(idx.Key) + len(idx.Value))
	}

	// Check thresholds
	if len(wb.pending) >= wb.maxEvents || wb.pendingSize >= wb.maxBytes {
		wb.triggerFlush()
	}

	return nil
}

// triggerFlush signals the flush loop to flush (must be called with lock held)
func (wb *WriteBatcher) triggerFlush() {
	select {
	case wb.flushCh <- struct{}{}:
	default:
		// Already a flush pending
	}
}

// flushLoop runs the background flush timer
func (wb *WriteBatcher) flushLoop() {
	defer close(wb.doneCh)

	timer := time.NewTimer(wb.flushPeriod)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			if err := wb.Flush(); err != nil {
				wb.logger.Errorf("bbolt: flush error: %v", err)
			}
			timer.Reset(wb.flushPeriod)

		case <-wb.flushCh:
			if err := wb.Flush(); err != nil {
				wb.logger.Errorf("bbolt: flush error: %v", err)
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(wb.flushPeriod)

		case <-wb.shutdownCh:
			// Final flush
			if err := wb.Flush(); err != nil {
				wb.logger.Errorf("bbolt: final flush error: %v", err)
			}
			return
		}
	}
}

// Flush writes all pending batches to BBolt
func (wb *WriteBatcher) Flush() error {
	wb.mu.Lock()
	if len(wb.pending) == 0 {
		wb.mu.Unlock()
		return nil
	}

	// Swap out pending slice
	toFlush := wb.pending
	toFlushSize := wb.pendingSize
	wb.pending = make([]*EventBatch, 0, wb.maxEvents)
	wb.pendingSize = 0
	wb.mu.Unlock()

	startTime := time.Now()

	// Collect all edge keys for bloom filter update
	var allEdgeKeys []EdgeKey
	for _, batch := range toFlush {
		allEdgeKeys = append(allEdgeKeys, batch.EdgeKeys...)
	}

	// Update bloom filter first (memory only)
	if len(allEdgeKeys) > 0 {
		wb.bloom.AddBatch(allEdgeKeys)
	}

	// Write all batches in a single transaction
	err := wb.db.Update(func(tx *bolt.Tx) error {
		for _, batch := range toFlush {
			// Write compact event data
			cmpBucket := tx.Bucket(bucketCmp)
			if cmpBucket != nil {
				key := makeSerialKey(batch.Serial)
				if err := cmpBucket.Put(key, batch.EventData); err != nil {
					return err
				}
			}

			// Write all indexes
			for _, idx := range batch.Indexes {
				bucket := tx.Bucket(idx.BucketName)
				if bucket == nil {
					continue
				}
				if idx.IsDelete {
					if err := bucket.Delete(idx.Key); err != nil {
						return err
					}
				} else {
					if err := bucket.Put(idx.Key, idx.Value); err != nil {
						return err
					}
				}
			}

			// Write event vertex
			if batch.EventVertex != nil {
				evBucket := tx.Bucket(bucketEv)
				if evBucket != nil {
					key := makeSerialKey(batch.Serial)
					if err := evBucket.Put(key, batch.EventVertex.Encode()); err != nil {
						return err
					}
				}
			}

			// Update pubkey vertices
			if err := wb.updatePubkeyVertex(tx, batch.PubkeyUpdate); err != nil {
				return err
			}
			for _, mentionUpdate := range batch.MentionUpdates {
				if err := wb.updatePubkeyVertex(tx, mentionUpdate); err != nil {
					return err
				}
			}
		}
		return nil
	})

	// Update stats
	duration := time.Since(startTime)
	wb.mu.Lock()
	wb.stats.TotalBatches++
	wb.stats.TotalEvents += uint64(len(toFlush))
	wb.stats.TotalBytes += uint64(toFlushSize)
	wb.stats.LastFlushTime = time.Now()
	wb.stats.LastFlushDuration = duration
	latencyMs := float64(duration.Milliseconds())
	wb.stats.AverageLatencyMs = (wb.stats.AverageLatencyMs*float64(wb.stats.TotalBatches-1) + latencyMs) / float64(wb.stats.TotalBatches)
	wb.mu.Unlock()

	if err == nil {
		wb.logger.Debugf("bbolt: flushed %d events (%d bytes) in %v", len(toFlush), toFlushSize, duration)
	}

	return err
}

// updatePubkeyVertex reads, updates, and writes a pubkey vertex
func (wb *WriteBatcher) updatePubkeyVertex(tx *bolt.Tx, update *PubkeyVertexUpdate) error {
	if update == nil {
		return nil
	}

	pvBucket := tx.Bucket(bucketPv)
	if pvBucket == nil {
		return nil
	}

	key := makeSerialKey(update.PubkeySerial)

	// Load existing vertex or create new
	var pv PubkeyVertex
	existing := pvBucket.Get(key)
	if existing != nil {
		if err := pv.Decode(existing); chk.E(err) {
			// If decode fails, start fresh
			pv = PubkeyVertex{}
		}
	}

	// Apply updates
	if update.AddAuthored != 0 {
		pv.AddAuthored(update.AddAuthored)
	}
	if update.AddMention != 0 {
		pv.AddMention(update.AddMention)
	}

	// Write back
	return pvBucket.Put(key, pv.Encode())
}

// Shutdown gracefully shuts down the batcher
func (wb *WriteBatcher) Shutdown() error {
	wb.mu.Lock()
	wb.stopped = true
	wb.mu.Unlock()

	close(wb.shutdownCh)
	<-wb.doneCh
	return nil
}

// Stats returns current batcher statistics
func (wb *WriteBatcher) Stats() BatcherStats {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	return wb.stats
}

// PendingCount returns the number of pending events
func (wb *WriteBatcher) PendingCount() int {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	return len(wb.pending)
}

// ErrBatcherStopped is returned when adding to a stopped batcher
var ErrBatcherStopped = &batcherStoppedError{}

type batcherStoppedError struct{}

func (e *batcherStoppedError) Error() string {
	return "batcher has been stopped"
}
