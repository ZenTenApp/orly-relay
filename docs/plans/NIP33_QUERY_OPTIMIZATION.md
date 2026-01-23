# Plan: NIP-33 Parameterized Replaceable Event Query Optimization

## Issue Reference
https://git.nostrdev.com/mleku/next.orly.dev/issues/29

## Problem Statement

Queries filtering by kinds, authors, and d-tags for NIP-33 parameterized replaceable events (kinds 30000-39999) take 10+ seconds when they should be nearly instantaneous.

**Example slow query:**
```json
["REQ","UserCards",{"kinds":[30382],"authors":["89330ec6f7fa08717d36d203e74644d7cccbda280547e5a3ce81ff389110d2e5"],"#d":["55f04590674f3648f4cdc9dc8ce32da2a282074cd0b020596ee033d12d385185"]}]
```

This query uniquely identifies a single event by its NIP-33 address (kind + pubkey + d-tag).

## Root Cause Analysis

### Current Query Path

1. Query enters `GetIndexesFromFilter()` (get-indexes-from-filter.go:199)
2. Detects all three criteria (kinds, authors, tags) → Uses `TagKindPubkey` index
3. `TagKindPubkey` index format: `tkp | tag_letter | value_hash | kind | pubkey_hash | timestamp | serial`
4. Query iterates **backwards through all timestamps** to find matching events
5. With large databases, this timestamp iteration is extremely slow

### Unused Optimization

An `AddressableEvent` index exists but is **never used for queries**:

```go
// indexes/keys.go:300-315
// Key format: aev | pubkey_hash | kind | dtag_hash
var AddressableEvent = next()
```

This index is:
- ✅ Defined in indexes/keys.go
- ✅ Created during migrations (migrations.go:519)
- ❌ **NOT written during save-event**
- ❌ **NOT used in GetIndexesFromFilter**

## Solution Design

### Phase 1: Enable AddressableEvent Index Writes

**File:** `pkg/database/get-indexes-for-event.go`

Add AddressableEvent index writes for parameterized replaceable events:

```go
// In getIndexesForEvent(), after existing index writes:

// For parameterized replaceable events (kinds 30000-39999), write AddressableEvent index
if kind.IsParameterizedReplaceable(ev.Kind) {
    dTag := ev.Tags.GetFirst([]byte("d"))
    if dTag != nil {
        dValue := dTag.Value()
        dTagHash := new(types.Ident)
        dTagHash.FromIdent(dValue)

        aevKey := indexes.AddressableEventEnc(pubHash, kindVal, dTagHash)
        aevKeyBuf := new(bytes.Buffer)
        if err = aevKey.MarshalWrite(aevKeyBuf); chk.E(err) {
            return
        }
        // Store serial as value for direct lookup
        serialBytes := make([]byte, 5)
        serial.Put(serialBytes)
        keys = append(keys, KeyValue{
            Key:   aevKeyBuf.Bytes(),
            Value: serialBytes,
        })
    }
}
```

### Phase 2: Fast Path for NIP-33 Queries

**File:** `pkg/database/query-for-ids.go` or new file `query-addressable.go`

Add detection and fast path for NIP-33 queries:

```go
// QueryForAddressableEvent performs a direct lookup for NIP-33 events
func (d *D) QueryForAddressableEvent(f *filter.F) (serial types.Uint40, found bool, err error) {
    // Check if this is a NIP-33 query pattern
    if !isAddressableEventQuery(f) {
        return 0, false, nil
    }

    // Extract components
    kindVal := f.Kinds.T[0]
    if !kind.IsParameterizedReplaceable(kindVal) {
        return 0, false, nil
    }

    author := f.Authors.T[0]
    pubHash, err := CreatePubHashFromData(author)
    if err != nil {
        return 0, false, err
    }

    dTag := f.Tags.GetFirst([]byte("#d"))
    if dTag == nil || dTag.Len() < 2 {
        return 0, false, nil
    }
    dValue := dTag.T[1]
    dTagHash := new(types.Ident)
    dTagHash.FromIdent(dValue)

    // Build direct lookup key
    kindType := new(types.Uint16)
    kindType.Set(kindVal)

    aevKey := indexes.AddressableEventEnc(pubHash, kindType, dTagHash)
    keyBuf := new(bytes.Buffer)
    if err = aevKey.MarshalWrite(keyBuf); chk.E(err) {
        return 0, false, err
    }

    // Direct key lookup - O(1)
    err = d.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get(keyBuf.Bytes())
        if err == badger.ErrKeyNotFound {
            return nil
        }
        if err != nil {
            return err
        }
        return item.Value(func(val []byte) error {
            if len(val) >= 5 {
                serial.Get(val)
                found = true
            }
            return nil
        })
    })

    return serial, found, err
}

func isAddressableEventQuery(f *filter.F) bool {
    // Must have exactly one kind, one author, and one d-tag
    if f.Kinds == nil || f.Kinds.Len() != 1 {
        return false
    }
    if f.Authors == nil || f.Authors.Len() != 1 {
        return false
    }
    if f.Tags == nil {
        return false
    }
    dTag := f.Tags.GetFirst([]byte("#d"))
    if dTag == nil || dTag.Len() != 2 {
        return false
    }
    // Must be a parameterized replaceable kind
    return kind.IsParameterizedReplaceable(f.Kinds.T[0])
}
```

### Phase 3: Integrate Fast Path into Query Flow

**File:** `pkg/database/query-events.go`

Add fast path check at the beginning of QueryEvents:

```go
func (d *D) QueryEvents(f *filter.F, ...) (res iter.I, err error) {
    // Fast path for NIP-33 addressable event lookups
    if serial, found, err := d.QueryForAddressableEvent(f); err == nil && found {
        ev, err := d.FetchEventBySerial(serial)
        if err == nil && ev != nil {
            // Apply any remaining filters (since, until) and return
            if f.Since != nil && ev.CreatedAt < *f.Since {
                return iter.Empty(), nil
            }
            if f.Until != nil && ev.CreatedAt > *f.Until {
                return iter.Empty(), nil
            }
            return iter.Single(ev), nil
        }
    }

    // Fall through to existing query logic
    ...
}
```

### Phase 4: Optimize WouldReplaceEvent

**File:** `pkg/database/save-event.go`

The `WouldReplaceEvent` function (line 70) also benefits from this optimization since it constructs the same filter pattern:

```go
func (d *D) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
    if kind.IsParameterizedReplaceable(ev.Kind) {
        dTag := ev.Tags.GetFirst([]byte("d"))
        if dTag == nil {
            return false, nil, ErrMissingDTag
        }

        // Use fast path for NIP-33 lookup
        f := &filter.F{
            Authors: tag.NewFromBytesSlice(ev.Pubkey),
            Kinds:   kind.NewS(kind.New(ev.Kind)),
            Tags:    tag.NewS(tag.NewFromAny("d", dTag.Value())),
        }

        if serial, found, err := d.QueryForAddressableEvent(f); err == nil && found {
            oldEv, err := d.FetchEventBySerial(serial)
            if err == nil && oldEv != nil {
                if ev.CreatedAt < oldEv.CreatedAt {
                    return false, types.Uint40s{serial}, nil
                }
                return true, nil, nil
            }
        }
    }

    // Fall through to existing logic for regular replaceables
    ...
}
```

### Phase 5: Handle Index Updates on Replace

When a parameterized replaceable event is replaced, the old AddressableEvent index entry must be deleted:

**File:** `pkg/database/save-event.go`

In the replacement logic, delete the old AddressableEvent entry:

```go
// When replacing a parameterized replaceable event:
if kind.IsParameterizedReplaceable(ev.Kind) && len(serialsToDelete) > 0 {
    // Delete old AddressableEvent index entries
    for _, oldSerial := range serialsToDelete {
        oldEv, _ := d.FetchEventBySerial(oldSerial)
        if oldEv != nil {
            dTag := oldEv.Tags.GetFirst([]byte("d"))
            if dTag != nil {
                // Build and delete old index key
                ...
            }
        }
    }
}
```

## Migration Strategy

### For Existing Data

Events stored before this optimization won't have AddressableEvent index entries. Options:

1. **Background rebuild** (recommended): Add a migration that scans existing events and builds AddressableEvent indexes
2. **Lazy rebuild**: Fall back to existing query path if AddressableEvent lookup fails
3. **Full reindex**: Trigger a full database reindex on upgrade

**Recommended approach:** Implement lazy rebuild with background migration:

```go
func (d *D) QueryForAddressableEvent(f *filter.F) (serial types.Uint40, found bool, err error) {
    // Try direct lookup first
    serial, found, err = d.directAddressableLookup(f)
    if found || err != nil {
        return
    }

    // Fall back to existing query path (slow but correct)
    sers, err := d.GetSerialsFromFilter(f)
    if err != nil || len(sers) == 0 {
        return 0, false, err
    }

    // Opportunistically build the index for next time
    go d.buildAddressableIndex(f, sers[0])

    return sers[0], true, nil
}
```

## Performance Impact

| Operation | Before | After |
|-----------|--------|-------|
| NIP-33 query (kind+author+d-tag) | 10+ seconds (index scan) | <1ms (direct lookup) |
| WouldReplaceEvent check | 10+ seconds | <1ms |
| Event save (NIP-33) | N/A | +1 index write (~µs) |
| Storage overhead | N/A | ~20 bytes per NIP-33 event |

## Files to Modify

1. `pkg/database/indexes/keys.go` - Already defined, no changes needed
2. `pkg/database/get-indexes-for-event.go` - Add AddressableEvent index writes
3. `pkg/database/query-addressable.go` - New file for fast path logic
4. `pkg/database/query-events.go` - Integrate fast path
5. `pkg/database/save-event.go` - Optimize WouldReplaceEvent, handle index updates
6. `pkg/database/migrations.go` - Add migration for existing data

## Testing

1. **Unit tests:**
   - `TestQueryAddressableEvent` - Direct lookup works
   - `TestAddressableEventIndexWrite` - Index written on save
   - `TestAddressableEventReplace` - Index updated on replace

2. **Benchmark tests:**
   - Compare query time before/after for NIP-33 patterns
   - Measure with varying database sizes (100k, 1M, 10M events)

3. **Integration tests:**
   - Verify compatibility with existing queries
   - Test migration on real database

## Rollout Plan

1. Implement Phase 1-2 with lazy rebuild (safe, no migration needed)
2. Add background migration for existing databases
3. Monitor performance metrics
4. Remove lazy rebuild fallback once migration complete
