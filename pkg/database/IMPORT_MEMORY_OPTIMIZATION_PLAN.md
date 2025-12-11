# Import Memory Optimization Plan

## Goal

Constrain import memory utilization to ≤1.5GB to ensure system disk cache flushing completes adequately before continuing.

## Test Results (Baseline)

- **File**: `wot_reference.jsonl` (2.7 GB, ~2.16 million events)
- **System**: 15 GB RAM, Linux
- **Events Saved**: 2,130,545
- **Total Time**: 48 minutes 16 seconds
- **Average Rate**: 736 events/sec
- **Peak Memory**: ~6.4 GB (42% of system RAM)

### Memory Timeline (Baseline)

| Time | Memory (RSS) | Events | Notes |
|------|--------------|--------|-------|
| Start | 95 MB | 0 | Initial state |
| +10 min | 2.7 GB | 283k | Warming up |
| +20 min | 4.1 GB | 475k | Memory growing |
| +30 min | 5.2 GB | 720k | Peak approaching |
| +35 min | 5.9 GB | 485k | Near peak |
| +40 min | 5.6 GB | 1.3M | GC recovered memory |
| +48 min | 6.4 GB | 2.1M | Final (42% of RAM) |

## Root Causes of Memory Growth

### 1. Badger Internal Caches (configured in `database.go`)

- Block cache: 1024 MB default
- Index cache: 512 MB default
- Memtables: 8 × 16 MB = 128 MB
- Total baseline: ~1.6 GB just for configured caches

### 2. Badger Write Buffers

- L0 tables buffer (8 tables × 16 MB)
- Value log writes accumulate until compaction

### 3. No Backpressure in Import Loop

- Events are written continuously without waiting for compaction
- `debug.FreeOSMemory()` only runs every 5 seconds
- Badger buffers writes faster than disk can flush

### 4. Transaction Overhead

- Each `SaveEvent` creates a transaction
- Transactions have overhead that accumulates

## Proposed Mitigations

### Phase 1: Reduce Badger Cache Configuration for Import

Add import-specific configuration options in `app/config/config.go`:

```go
ImportBlockCacheMB  int `env:"ORLY_IMPORT_BLOCK_CACHE_MB" default:"256"`
ImportIndexCacheMB  int `env:"ORLY_IMPORT_INDEX_CACHE_MB" default:"128"`
ImportMemTableSize  int `env:"ORLY_IMPORT_MEMTABLE_SIZE_MB" default:"8"`
```

For a 1.5GB target:

| Component | Size | Notes |
|-----------|------|-------|
| Block cache | 256 MB | Reduced from 1024 MB |
| Index cache | 128 MB | Reduced from 512 MB |
| Memtables | 4 × 8 MB = 32 MB | Reduced from 8 × 16 MB |
| Serial cache | ~20 MB | Unchanged |
| Working memory | ~200 MB | Buffer for processing |
| **Total** | **~636 MB** | Leaves headroom for 1.5GB target |

### Phase 2: Add Batching with Sync to Import Loop

Modify `import_utils.go` to batch writes and force sync:

```go
const (
    importBatchSize      = 500    // Events per batch
    importSyncInterval   = 2000   // Events before forcing sync
    importMemCheckEvents = 1000   // Events between memory checks
    importMaxMemoryMB    = 1400   // Target max memory (MB)
)

// In processJSONLEventsWithPolicy:
var batchCount int
for scan.Scan() {
    // ... existing event processing ...

    batchCount++
    count++

    // Force sync periodically to flush writes to disk
    if batchCount >= importSyncInterval {
        d.DB.Sync()  // Force write to disk
        batchCount = 0
    }

    // Memory pressure check
    if count % importMemCheckEvents == 0 {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        heapMB := m.HeapAlloc / 1024 / 1024

        if heapMB > importMaxMemoryMB {
            // Apply backpressure
            d.DB.Sync()
            runtime.GC()
            debug.FreeOSMemory()

            // Wait for compaction to catch up
            time.Sleep(100 * time.Millisecond)
        }
    }
}
```

### Phase 3: Use Batch Transactions

Instead of one transaction per event, batch multiple events:

```go
// Accumulate events for batch write
const txnBatchSize = 100

type pendingWrite struct {
    idxs       [][]byte
    compactKey []byte
    compactVal []byte
    graphKeys  [][]byte
}

var pendingWrites []pendingWrite

// In the event processing loop
pendingWrites = append(pendingWrites, pw)

if len(pendingWrites) >= txnBatchSize {
    err = d.Update(func(txn *badger.Txn) error {
        for _, pw := range pendingWrites {
            for _, key := range pw.idxs {
                txn.Set(key, nil)
            }
            txn.Set(pw.compactKey, pw.compactVal)
            for _, gk := range pw.graphKeys {
                txn.Set(gk, nil)
            }
        }
        return nil
    })
    pendingWrites = pendingWrites[:0]
}
```

### Phase 4: Implement Adaptive Rate Limiting

```go
type importRateLimiter struct {
    targetMemMB   uint64
    checkInterval int
    baseDelay     time.Duration
    maxDelay      time.Duration
}

func (r *importRateLimiter) maybeThrottle(eventCount int) {
    if eventCount % r.checkInterval != 0 {
        return
    }

    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    heapMB := m.HeapAlloc / 1024 / 1024

    if heapMB > r.targetMemMB {
        // Calculate delay proportional to overage
        overage := float64(heapMB - r.targetMemMB) / float64(r.targetMemMB)
        delay := time.Duration(float64(r.baseDelay) * (1 + overage*10))
        if delay > r.maxDelay {
            delay = r.maxDelay
        }

        // Force GC and wait
        runtime.GC()
        debug.FreeOSMemory()
        time.Sleep(delay)
    }
}
```

## Implementation Order

1. **Quick Win**: Add `d.DB.Sync()` call every N events in import loop
2. **Configuration**: Add environment variables for import-specific cache sizes
3. **Batching**: Implement batch transactions to reduce overhead
4. **Adaptive**: Add memory-aware rate limiting

## Expected Results

| Approach | Memory Target | Throughput Impact |
|----------|---------------|-------------------|
| Current | ~6 GB peak | 736 events/sec |
| Phase 1 (cache reduction) | ~2 GB | ~700 events/sec |
| Phase 2 (sync + GC) | ~1.5 GB | ~500 events/sec |
| Phase 3 (batching) | ~1.5 GB | ~600 events/sec |
| Phase 4 (adaptive) | ~1.4 GB | Variable |

## Files to Modify

1. `app/config/config.go` - Add import-specific config options
2. `pkg/database/database.go` - Add import mode with reduced caches
3. `pkg/database/import_utils.go` - Add batching, sync, and rate limiting
4. `pkg/database/save-event.go` - Add batch save method (optional, for Phase 3)

## Environment Variables (Proposed)

```bash
# Import-specific cache settings (only apply during import operations)
ORLY_IMPORT_BLOCK_CACHE_MB=256      # Block cache size during import
ORLY_IMPORT_INDEX_CACHE_MB=128      # Index cache size during import
ORLY_IMPORT_MEMTABLE_SIZE_MB=8      # Memtable size during import

# Import rate limiting
ORLY_IMPORT_SYNC_INTERVAL=2000      # Events between forced syncs
ORLY_IMPORT_MAX_MEMORY_MB=1400      # Target max memory during import
ORLY_IMPORT_BATCH_SIZE=100          # Events per transaction batch
```

## Notes

- The adaptive rate limiting (Phase 4) is the most robust solution but adds complexity
- Phase 2 alone should achieve the 1.5GB target with acceptable throughput
- Batch transactions (Phase 3) can improve throughput but require refactoring `SaveEvent`
- Consider making these settings configurable so users can tune for their hardware

## Test Command

To re-run the import test with memory monitoring:

```bash
# Start relay with import-optimized settings
export ORLY_DATA_DIR=/tmp/orly-import-test
export ORLY_ACL_MODE=none
export ORLY_PORT=10548
export ORLY_LOG_LEVEL=info
./orly &

# Upload test file
curl -X POST \
  -F "file=@/path/to/wot_reference.jsonl" \
  http://localhost:10548/api/import

# Monitor memory
watch -n 5 'ps -p $(pgrep orly) -o pid,rss,pmem --no-headers'
```
