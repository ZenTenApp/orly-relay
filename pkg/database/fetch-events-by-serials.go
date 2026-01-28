//go:build !(js && wasm)

package database

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

// FetchEventsBySerials fetches multiple events by their serials in a single database transaction.
// Returns a map of serial uint64 value to event, only including successfully fetched events.
//
// This function tries multiple storage formats in order:
// 1. cmp (compact format with serial references) - newest, most space-efficient
// 2. sev (small event inline) - legacy Reiser4 optimization
// 3. evt (traditional separate storage) - legacy fallback
func (d *D) FetchEventsBySerials(serials []*types.Uint40) (events map[uint64]*event.E, err error) {
	// Pre-allocate map with estimated capacity to reduce reallocations
	events = make(map[uint64]*event.E, len(serials))

	if len(serials) == 0 {
		return events, nil
	}

	// Create resolver for compact event decoding
	resolver := NewDatabaseSerialResolver(d, d.serialCache)

	if err = d.View(
		func(txn *badger.Txn) (err error) {
			// Create ONE iterator for sev prefix lookups - reused for all serials
			// This dramatically reduces memory usage vs creating one per serial
			sevOpts := badger.DefaultIteratorOptions
			sevOpts.PrefetchValues = false // We read from key, not value
			sevOpts.PrefetchSize = 1
			sevIt := txn.NewIterator(sevOpts)
			defer sevIt.Close()

			for _, ser := range serials {
				var ev *event.E
				serialVal := ser.Get()

				// Try cmp (compact format) first - most efficient
				ev, err = d.fetchCompactEvent(txn, ser, resolver)
				if err == nil && ev != nil {
					events[serialVal] = ev
					continue
				}
				err = nil // Reset error, try legacy formats

				// Try sev (small event inline) using shared iterator
				ev, err = d.fetchSmallEventWithIterator(txn, ser, sevIt)
				if err == nil && ev != nil {
					events[serialVal] = ev
					continue
				}
				err = nil // Reset error, try evt

				// Not found in sev table, try evt (traditional) prefix
				ev, err = d.fetchLegacyEvent(txn, ser)
				if err == nil && ev != nil {
					events[serialVal] = ev
					continue
				}
				err = nil // Reset error, event not found
			}
			return nil
		},
	); err != nil {
		return
	}

	return events, nil
}

// fetchSmallEventWithIterator uses a provided iterator for sev lookups (memory efficient)
func (d *D) fetchSmallEventWithIterator(txn *badger.Txn, ser *types.Uint40, it *badger.Iterator) (ev *event.E, err error) {
	smallBuf := new(bytes.Buffer)
	if err = indexes.SmallEventEnc(ser).MarshalWrite(smallBuf); chk.E(err) {
		return nil, err
	}
	prefix := smallBuf.Bytes()

	// Seek to the prefix for this serial
	it.Seek(prefix)
	if !it.ValidForPrefix(prefix) {
		return nil, nil // Not found
	}

	// Found in sev table - extract inline data
	key := it.Item().KeyCopy(nil) // Copy key as iterator may be reused
	// Key format: sev|serial|size_uint16|event_data
	if len(key) <= 8+2 { // prefix(3) + serial(5) + size(2) = 10 bytes minimum
		return nil, nil
	}

	sizeIdx := 8 // After sev(3) + serial(5)
	// Read uint16 big-endian size
	size := int(key[sizeIdx])<<8 | int(key[sizeIdx+1])
	dataStart := sizeIdx + 2

	if len(key) < dataStart+size {
		return nil, nil
	}

	eventData := key[dataStart : dataStart+size]

	// Check if this is compact format (starts with version byte 1)
	// Note: Legacy events whose ID starts with 0x01 will also match this check,
	// so we fall back to legacy format if the SerialEventId mapping doesn't exist.
	if len(eventData) > 0 && eventData[0] == CompactFormatVersion {
		// Try to get event ID mapping - if it exists, this is truly compact format
		eventId, idErr := d.GetEventIdBySerial(ser)
		if idErr == nil {
			// SerialEventId mapping exists - this is compact format
			resolver := NewDatabaseSerialResolver(d, d.serialCache)
			return UnmarshalCompactEvent(eventData, eventId, resolver)
		}
		// No SerialEventId mapping - this is likely a legacy event whose ID starts with 0x01
		// Fall through to legacy unmarshal
		log.T.F("fetchSmallEventWithIterator: no sei mapping for serial %d, trying legacy format", ser.Get())
	}

	// Legacy binary format
	ev = new(event.E)
	if err = ev.UnmarshalBinary(bytes.NewBuffer(eventData)); err != nil {
		return nil, err
	}

	return ev, nil
}

// fetchCompactEvent tries to fetch an event from the compact format (cmp prefix).
func (d *D) fetchCompactEvent(txn *badger.Txn, ser *types.Uint40, resolver SerialResolver) (ev *event.E, err error) {
	// Build cmp key
	keyBuf := new(bytes.Buffer)
	if err = indexes.CompactEventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
		return nil, err
	}

	item, err := txn.Get(keyBuf.Bytes())
	if err != nil {
		return nil, err
	}

	var compactData []byte
	if compactData, err = item.ValueCopy(nil); chk.E(err) {
		return nil, err
	}

	// Need to get the event ID from SerialEventId table
	eventId, err := d.GetEventIdBySerial(ser)
	if err != nil {
		log.D.F("fetchCompactEvent: failed to get event ID for serial %d: %v", ser.Get(), err)
		return nil, err
	}

	// Unmarshal compact event
	ev, err = UnmarshalCompactEvent(compactData, eventId, resolver)
	if err != nil {
		log.D.F("fetchCompactEvent: failed to unmarshal compact event for serial %d: %v", ser.Get(), err)
		return nil, err
	}

	return ev, nil
}

// fetchSmallEvent tries to fetch an event from the small event inline format (sev prefix).
func (d *D) fetchSmallEvent(txn *badger.Txn, ser *types.Uint40) (ev *event.E, err error) {
	smallBuf := new(bytes.Buffer)
	if err = indexes.SmallEventEnc(ser).MarshalWrite(smallBuf); chk.E(err) {
		return nil, err
	}

	// Iterate with prefix to find the small event key
	opts := badger.DefaultIteratorOptions
	opts.Prefix = smallBuf.Bytes()
	opts.PrefetchValues = true
	opts.PrefetchSize = 1
	it := txn.NewIterator(opts)
	defer it.Close()

	it.Rewind()
	if !it.Valid() {
		return nil, nil // Not found
	}

	// Found in sev table - extract inline data
	key := it.Item().Key()
	// Key format: sev|serial|size_uint16|event_data
	if len(key) <= 8+2 { // prefix(3) + serial(5) + size(2) = 10 bytes minimum
		return nil, nil
	}

	sizeIdx := 8 // After sev(3) + serial(5)
	// Read uint16 big-endian size
	size := int(key[sizeIdx])<<8 | int(key[sizeIdx+1])
	dataStart := sizeIdx + 2

	if len(key) < dataStart+size {
		return nil, nil
	}

	eventData := key[dataStart : dataStart+size]

	// Check if this is compact format (starts with version byte 1)
	// Note: Legacy events whose ID starts with 0x01 will also match this check,
	// so we fall back to legacy format if the SerialEventId mapping doesn't exist.
	if len(eventData) > 0 && eventData[0] == CompactFormatVersion {
		// Try to get event ID mapping - if it exists, this is truly compact format
		eventId, idErr := d.GetEventIdBySerial(ser)
		if idErr == nil {
			// SerialEventId mapping exists - this is compact format
			resolver := NewDatabaseSerialResolver(d, d.serialCache)
			return UnmarshalCompactEvent(eventData, eventId, resolver)
		}
		// No SerialEventId mapping - this is likely a legacy event whose ID starts with 0x01
		// Fall through to legacy unmarshal
		log.T.F("fetchSmallEvent: no sei mapping for serial %d, trying legacy format", ser.Get())
	}

	// Legacy binary format
	ev = new(event.E)
	if err = ev.UnmarshalBinary(bytes.NewBuffer(eventData)); err != nil {
		return nil, err
	}

	return ev, nil
}

// fetchLegacyEvent tries to fetch an event from the legacy format (evt prefix).
func (d *D) fetchLegacyEvent(txn *badger.Txn, ser *types.Uint40) (ev *event.E, err error) {
	buf := new(bytes.Buffer)
	if err = indexes.EventEnc(ser).MarshalWrite(buf); chk.E(err) {
		return nil, err
	}

	item, err := txn.Get(buf.Bytes())
	if err != nil {
		return nil, err
	}

	var v []byte
	if v, err = item.ValueCopy(nil); chk.E(err) {
		return nil, err
	}

	// Check if we have valid data before attempting to unmarshal
	if len(v) < 32+32+1+2+1+1+64 { // ID + Pubkey + min varint fields + Sig
		return nil, nil
	}

	// Check if this is compact format (starts with version byte 1)
	// Note: Legacy events whose ID starts with 0x01 will also match this check,
	// so we fall back to legacy format if the SerialEventId mapping doesn't exist.
	if len(v) > 0 && v[0] == CompactFormatVersion {
		// Try to get event ID mapping - if it exists, this is truly compact format
		eventId, idErr := d.GetEventIdBySerial(ser)
		if idErr == nil {
			// SerialEventId mapping exists - this is compact format
			resolver := NewDatabaseSerialResolver(d, d.serialCache)
			return UnmarshalCompactEvent(v, eventId, resolver)
		}
		// No SerialEventId mapping - this is likely a legacy event whose ID starts with 0x01
		// Fall through to legacy unmarshal
		log.T.F("fetchLegacyEvent: no sei mapping for serial %d, trying legacy format", ser.Get())
	}

	// Legacy binary format
	ev = new(event.E)
	if err = ev.UnmarshalBinary(bytes.NewBuffer(v)); err != nil {
		return nil, err
	}

	return ev, nil
}

