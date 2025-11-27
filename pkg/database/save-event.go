package database

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

var (
	// ErrOlderThanExisting is returned when a candidate event is older than an existing replaceable/addressable event.
	ErrOlderThanExisting = errors.New("older than existing event")
	// ErrMissingDTag is returned when a parameterized replaceable event lacks the required 'd' tag.
	ErrMissingDTag = errors.New("event is missing a d tag identifier")
)

func (d *D) GetSerialsFromFilter(f *filter.F) (
	sers types.Uint40s, err error,
) {
	// Try p-tag graph optimization first
	if CanUsePTagGraph(f) {
		log.D.F("GetSerialsFromFilter: trying p-tag graph optimization")
		if sers, err = d.QueryPTagGraph(f); err == nil && len(sers) >= 0 {
			log.D.F("GetSerialsFromFilter: p-tag graph optimization returned %d serials", len(sers))
			return
		}
		// Fall through to traditional indexes on error
		log.D.F("GetSerialsFromFilter: p-tag graph optimization failed, falling back to traditional indexes: %v", err)
		err = nil
	}

	var idxs []Range
	if idxs, err = GetIndexesFromFilter(f); chk.E(err) {
		return
	}
	// Pre-allocate slice with estimated capacity to reduce reallocations
	sers = make(
		types.Uint40s, 0, len(idxs)*100,
	) // Estimate 100 serials per index
	for _, idx := range idxs {
		var s types.Uint40s
		if s, err = d.GetSerialsByRange(idx); chk.E(err) {
			continue
		}
		sers = append(sers, s...)
	}
	return
}

// WouldReplaceEvent checks if the provided event would replace existing events
// based on Nostr's replaceable or parameterized replaceable semantics. It
// returns true if the candidate is newer-or-equal than existing events.
// If an existing event is newer, it returns (false, nil, ErrOlderThanExisting).
// If no conflicts exist, it returns (false, nil, nil).
func (d *D) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
	// Only relevant for replaceable or parameterized replaceable kinds
	if !(kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind)) {
		return false, nil, nil
	}

	var f *filter.F
	if kind.IsReplaceable(ev.Kind) {
		f = &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
		}
	} else {
		// parameterized replaceable requires 'd' tag
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag == nil {
			return false, nil, ErrMissingDTag
		}
		f = &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
			Tags: tag.NewS(
				tag.NewFromAny("d", dTag.Value()),
			),
		}
	}

	sers, err := d.GetSerialsFromFilter(f)
	if chk.E(err) {
		return false, nil, err
	}
	if len(sers) == 0 {
		return false, nil, nil
	}

	// Determine if any existing event is newer than the candidate
	shouldReplace := true
	for _, s := range sers {
		oldEv, ferr := d.FetchEventBySerial(s)
		if chk.E(ferr) {
			continue
		}
		if ev.CreatedAt < oldEv.CreatedAt {
			shouldReplace = false
			break
		}
	}
	if shouldReplace {
		return true, nil, nil
	}
	return false, nil, ErrOlderThanExisting
}

// SaveEvent saves an event to the database, generating all the necessary indexes.
func (d *D) SaveEvent(c context.Context, ev *event.E) (
	replaced bool, err error,
) {
	if ev == nil {
		err = errors.New("nil event")
		return
	}

	// Reject ephemeral events (kinds 20000-29999) - they should never be stored
	if ev.Kind >= 20000 && ev.Kind <= 29999 {
		err = errors.New("blocked: ephemeral events should not be stored")
		return
	}

	// Validate kind 3 (follow list) events have at least one p tag
	// This prevents storing malformed follow lists that may come from buggy relays
	if ev.Kind == 3 {
		hasPTag := false
		tagCount := 0
		if ev.Tags != nil {
			tagCount = ev.Tags.Len()
			for _, tag := range *ev.Tags {
				if tag != nil && tag.Len() >= 2 {
					key := tag.Key()
					if len(key) == 1 && key[0] == 'p' {
						hasPTag = true
						break
					}
				}
			}
		}
		if !hasPTag {
			log.W.F("SaveEvent: rejecting kind 3 event without p tags from pubkey %x (total tags: %d, event ID: %x)",
				ev.Pubkey, tagCount, ev.ID)
			err = errors.New("blocked: kind 3 follow list events must have at least one p tag")
			return
		}
	}

	// check if the event already exists
	var ser *types.Uint40
	if ser, err = d.GetSerialById(ev.ID); err == nil && ser != nil {
		err = errors.New("blocked: event already exists: " + hex.Enc(ev.ID[:]))
		return
	}

	// If the error is "id not found", we can proceed with saving the event
	if err != nil && strings.Contains(err.Error(), "id not found in database") {
		// Reset error since this is expected for new events
		err = nil
	} else if err != nil {
		// For any other error, return it
		// log.E.F("error checking if event exists: %s", err)
		return
	}

	// Check if the event has been deleted before allowing resubmission
	if err = d.CheckForDeleted(ev, nil); err != nil {
		// log.I.F(
		// 	"SaveEvent: rejecting resubmission of deleted event ID=%s: %v",
		// 	hex.Enc(ev.ID), err,
		// )
		err = fmt.Errorf("blocked: %s", err.Error())
		return
	}
	// check for replacement - only validate, don't delete old events
	if kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind) {
		var werr error
		if replaced, _, werr = d.WouldReplaceEvent(ev); werr != nil {
			if errors.Is(werr, ErrOlderThanExisting) {
				if kind.IsReplaceable(ev.Kind) {
					err = errors.New("blocked: event is older than existing replaceable event")
				} else {
					err = errors.New("blocked: event is older than existing addressable event")
				}
				return
			}
			if errors.Is(werr, ErrMissingDTag) {
				// keep behavior consistent with previous implementation
				err = ErrMissingDTag
				return
			}
			// any other error
			return
		}
		// Note: replaced flag is kept for compatibility but old events are no longer deleted
	}
	// Get the next sequence number for the event
	var serial uint64
	if serial, err = d.seq.Next(); chk.E(err) {
		return
	}
	// Generate all indexes for the event
	var idxs [][]byte
	if idxs, err = GetIndexesForEvent(ev, serial); chk.E(err) {
		return
	}

	// Collect all pubkeys for graph: author + p-tags
	// Store with direction indicator: author (0) vs p-tag (1)
	type pubkeyWithDirection struct {
		serial    *types.Uint40
		isAuthor  bool
	}
	pubkeysForGraph := make(map[string]pubkeyWithDirection)

	// Add author pubkey
	var authorSerial *types.Uint40
	if authorSerial, err = d.GetOrCreatePubkeySerial(ev.Pubkey); chk.E(err) {
		return
	}
	pubkeysForGraph[hex.Enc(ev.Pubkey)] = pubkeyWithDirection{
		serial:   authorSerial,
		isAuthor: true,
	}

	// Extract p-tag pubkeys using GetAll
	pTags := ev.Tags.GetAll([]byte("p"))
	for _, pTag := range pTags {
		if pTag.Len() >= 2 {
			// Get pubkey from p-tag, handling both binary and hex storage formats
			// ValueHex() returns hex regardless of internal storage format
			var ptagPubkey []byte
			if ptagPubkey, err = hex.Dec(string(pTag.ValueHex())); err == nil && len(ptagPubkey) == 32 {
				pkHex := hex.Enc(ptagPubkey)
				// Skip if already added as author
				if _, exists := pubkeysForGraph[pkHex]; !exists {
					var ptagSerial *types.Uint40
					if ptagSerial, err = d.GetOrCreatePubkeySerial(ptagPubkey); chk.E(err) {
						return
					}
					pubkeysForGraph[pkHex] = pubkeyWithDirection{
						serial:   ptagSerial,
						isAuthor: false,
					}
				}
			}
		}
	}
	// log.T.F(
	// 	"SaveEvent: generated %d indexes for event %x (kind %d)", len(idxs),
	// 	ev.ID, ev.Kind,
	// )

	// Serialize event once to check size
	eventDataBuf := new(bytes.Buffer)
	ev.MarshalBinary(eventDataBuf)
	eventData := eventDataBuf.Bytes()

	// Determine storage strategy (Reiser4 optimizations)
	// Get threshold from environment, default to 0 (disabled)
	// When enabled, typical values: 384 (conservative), 512 (recommended), 1024 (aggressive)
	smallEventThreshold := 1024
	if v := os.Getenv("ORLY_INLINE_EVENT_THRESHOLD"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n >= 0 {
			smallEventThreshold = n
		}
	}
	isSmallEvent := smallEventThreshold > 0 && len(eventData) <= smallEventThreshold
	isReplaceableEvent := kind.IsReplaceable(ev.Kind)
	isAddressableEvent := kind.IsParameterizedReplaceable(ev.Kind)

	// Start a transaction to save the event and all its indexes
	err = d.Update(
		func(txn *badger.Txn) (err error) {
			// Pre-allocate key buffer to avoid allocations in loop
			ser := new(types.Uint40)
			if err = ser.Set(serial); chk.E(err) {
				return
			}

			// Save each index
			for _, key := range idxs {
				if err = txn.Set(key, nil); chk.E(err) {
					return
				}
			}

			// Write the event using optimized storage strategy
			// Determine if we should use inline addressable/replaceable storage
			useAddressableInline := false
			var dTag *tag.T
			if isAddressableEvent && isSmallEvent {
				dTag = ev.Tags.GetFirst([]byte("d"))
				useAddressableInline = dTag != nil
			}

			// All small events get a sev key for serial-based access
			if isSmallEvent {
				// Small event: store inline with sev prefix
				// Format: sev|serial|size_uint16|event_data
				keyBuf := new(bytes.Buffer)
				if err = indexes.SmallEventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
					return
				}
				// Append size as uint16 big-endian (2 bytes for size up to 65535)
				sizeBytes := []byte{
					byte(len(eventData) >> 8), byte(len(eventData)),
				}
				keyBuf.Write(sizeBytes)
				// Append event data
				keyBuf.Write(eventData)

				if err = txn.Set(keyBuf.Bytes(), nil); chk.E(err) {
					return
				}
				// log.T.F(
				// 	"SaveEvent: stored small event inline (%d bytes)",
				// 	len(eventData),
				// )
			} else {
				// Large event: store separately with evt prefix
				keyBuf := new(bytes.Buffer)
				if err = indexes.EventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
					return
				}
				if err = txn.Set(keyBuf.Bytes(), eventData); chk.E(err) {
					return
				}
				// log.T.F(
				// "SaveEvent: stored large event separately (%d bytes)",
				// len(eventData),
				// )
			}

			// Additionally, store replaceable/addressable events with specialized keys for direct access
			if useAddressableInline {
				// Addressable event: also store with aev|pubkey_hash|kind|dtag_hash|size|data
				pubHash := new(types.PubHash)
				pubHash.FromPubkey(ev.Pubkey)
				kindVal := new(types.Uint16)
				kindVal.Set(ev.Kind)
				dTagHash := new(types.Ident)
				dTagHash.FromIdent(dTag.Value())

				keyBuf := new(bytes.Buffer)
				if err = indexes.AddressableEventEnc(
					pubHash, kindVal, dTagHash,
				).MarshalWrite(keyBuf); chk.E(err) {
					return
				}
				// Append size as uint16 big-endian
				sizeBytes := []byte{
					byte(len(eventData) >> 8), byte(len(eventData)),
				}
				keyBuf.Write(sizeBytes)
				// Append event data
				keyBuf.Write(eventData)

				if err = txn.Set(keyBuf.Bytes(), nil); chk.E(err) {
					return
				}
				// log.T.F("SaveEvent: also stored addressable event with specialized key")
			} else if isReplaceableEvent && isSmallEvent {
				// Replaceable event: also store with rev|pubkey_hash|kind|size|data
				pubHash := new(types.PubHash)
				pubHash.FromPubkey(ev.Pubkey)
				kindVal := new(types.Uint16)
				kindVal.Set(ev.Kind)

				keyBuf := new(bytes.Buffer)
				if err = indexes.ReplaceableEventEnc(
					pubHash, kindVal,
				).MarshalWrite(keyBuf); chk.E(err) {
					return
				}
				// Append size as uint16 big-endian
				sizeBytes := []byte{
					byte(len(eventData) >> 8), byte(len(eventData)),
				}
				keyBuf.Write(sizeBytes)
				// Append event data
				keyBuf.Write(eventData)

				if err = txn.Set(keyBuf.Bytes(), nil); chk.E(err) {
					return
				}
				log.T.F("SaveEvent: also stored replaceable event with specialized key")
			}

			// Create graph edges between event and all related pubkeys
			// This creates bidirectional edges: event->pubkey and pubkey->event
			// Include the event kind and direction for efficient graph queries
			eventKind := new(types.Uint16)
			eventKind.Set(ev.Kind)

			for _, pkInfo := range pubkeysForGraph {
				// Determine direction for forward edge (event -> pubkey perspective)
				directionForward := new(types.Letter)
				// Determine direction for reverse edge (pubkey -> event perspective)
				directionReverse := new(types.Letter)

				if pkInfo.isAuthor {
					// Event author relationship
					directionForward.Set(types.EdgeDirectionAuthor)  // 0: author
					directionReverse.Set(types.EdgeDirectionAuthor)  // 0: is author of event
				} else {
					// P-tag relationship
					directionForward.Set(types.EdgeDirectionPTagOut) // 1: event references pubkey (outbound)
					directionReverse.Set(types.EdgeDirectionPTagIn)  // 2: pubkey is referenced (inbound)
				}

				// Create event -> pubkey edge (with kind and direction)
				keyBuf := new(bytes.Buffer)
				if err = indexes.EventPubkeyGraphEnc(ser, pkInfo.serial, eventKind, directionForward).MarshalWrite(keyBuf); chk.E(err) {
					return
				}
				if err = txn.Set(keyBuf.Bytes(), nil); chk.E(err) {
					return
				}

				// Create pubkey -> event edge (reverse, with kind and direction for filtering)
				keyBuf.Reset()
				if err = indexes.PubkeyEventGraphEnc(pkInfo.serial, eventKind, directionReverse, ser).MarshalWrite(keyBuf); chk.E(err) {
					return
				}
				if err = txn.Set(keyBuf.Bytes(), nil); chk.E(err) {
					return
				}
			}

			return
		},
	)
	if err != nil {
		return
	}

	// Process deletion events to actually delete the referenced events
	if ev.Kind == kind.Deletion.K {
		if err = d.ProcessDelete(ev, nil); chk.E(err) {
			log.W.F("failed to process deletion for event %x: %v", ev.ID, err)
			// Don't return error - the deletion event was saved successfully
			err = nil
		}
	}

	// Invalidate query cache since a new event was stored
	// This ensures subsequent queries will see the new event
	if d.queryCache != nil {
		d.queryCache.Invalidate()
		// log.T.F("SaveEvent: invalidated query cache")
	}

	return
}
