//go:build js && wasm

package wasmdb

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/aperturerobotics/go-indexeddb/idb"
	"next.orly.dev/pkg/lol/chk"
	sha256 "github.com/minio/sha256-simd"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/ints"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
	"next.orly.dev/pkg/utils"
)

// CheckExpiration checks if an event has expired based on its "expiration" tag
func CheckExpiration(ev *event.E) (expired bool) {
	var err error
	expTag := ev.Tags.GetFirst([]byte("expiration"))
	if expTag != nil {
		expTS := ints.New(0)
		if _, err = expTS.Unmarshal(expTag.Value()); err == nil {
			if int64(expTS.N) < time.Now().Unix() {
				return true
			}
		}
	}
	return
}

// GetSerialsByRange retrieves serials from an index range using cursor iteration.
// The index keys must end with a 5-byte serial number.
func (w *W) GetSerialsByRange(idx database.Range) (sers types.Uint40s, err error) {
	if len(idx.Start) < 3 {
		return nil, errors.New("invalid range: start key too short")
	}

	// Extract the object store name from the 3-byte prefix
	storeName := string(idx.Start[:3])

	// Open a read transaction
	tx, err := w.db.Transaction(idb.TransactionReadOnly, storeName)
	if err != nil {
		return nil, err
	}

	objStore, err := tx.ObjectStore(storeName)
	if err != nil {
		return nil, err
	}

	// Open cursor in reverse order (newest first like Badger)
	cursorReq, err := objStore.OpenCursor(idb.CursorPrevious)
	if err != nil {
		return nil, err
	}

	// Pre-allocate slice
	sers = make(types.Uint40s, 0, 100)

	// Create end boundary with 0xff suffix for inclusive range
	endBoundary := make([]byte, len(idx.End)+5)
	copy(endBoundary, idx.End)
	for i := len(idx.End); i < len(endBoundary); i++ {
		endBoundary[i] = 0xff
	}

	err = cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		keyVal, keyErr := cursor.Key()
		if keyErr != nil {
			return keyErr
		}

		key := safeValueToBytes(keyVal)
		if len(key) < 8 { // minimum: 3 prefix + 5 serial
			return cursor.Continue()
		}

		// Check if key is within range
		keyWithoutSerial := key[:len(key)-5]

		// Compare with start (lower bound)
		cmp := bytes.Compare(keyWithoutSerial, idx.Start)
		if cmp < 0 {
			// Key is before range start, stop iteration
			return errors.New("done")
		}

		// Compare with end boundary
		if bytes.Compare(key, endBoundary) > 0 {
			// Key is after range end, continue to find keys in range
			return cursor.Continue()
		}

		// Extract serial from last 5 bytes
		ser := new(types.Uint40)
		if err := ser.UnmarshalRead(bytes.NewReader(key[len(key)-5:])); err == nil {
			sers = append(sers, ser)
		}

		return cursor.Continue()
	})

	if err != nil && err.Error() != "done" {
		return nil, err
	}

	// Sort by serial (ascending)
	sort.Slice(sers, func(i, j int) bool {
		return sers[i].Get() < sers[j].Get()
	})

	return sers, nil
}

// QueryForIds retrieves IdPkTs records based on a filter.
// Results are sorted by timestamp in reverse chronological order.
func (w *W) QueryForIds(c context.Context, f *filter.F) (idPkTs []*store.IdPkTs, err error) {
	if f.Ids != nil && f.Ids.Len() > 0 {
		err = errors.New("query for Ids is invalid for a filter with Ids")
		return
	}

	var idxs []database.Range
	if idxs, err = database.GetIndexesFromFilter(f); chk.E(err) {
		return
	}

	var results []*store.IdPkTs
	results = make([]*store.IdPkTs, 0, len(idxs)*100)

	// Track match counts for search ranking
	counts := make(map[uint64]int)

	for _, idx := range idxs {
		var founds types.Uint40s
		if founds, err = w.GetSerialsByRange(idx); err != nil {
			w.Logger.Warnf("QueryForIds: GetSerialsByRange error: %v", err)
			continue
		}

		var tmp []*store.IdPkTs
		if tmp, err = w.GetFullIdPubkeyBySerials(founds); err != nil {
			w.Logger.Warnf("QueryForIds: GetFullIdPubkeyBySerials error: %v", err)
			continue
		}

		// Track match counts for search queries
		if len(f.Search) > 0 {
			for _, v := range tmp {
				counts[v.Ser]++
			}
		}
		results = append(results, tmp...)
	}

	// Deduplicate results
	seen := make(map[uint64]struct{}, len(results))
	idPkTs = make([]*store.IdPkTs, 0, len(results))
	for _, idpk := range results {
		if _, ok := seen[idpk.Ser]; !ok {
			seen[idpk.Ser] = struct{}{}
			idPkTs = append(idPkTs, idpk)
		}
	}

	// For search queries combined with other filters, verify matches
	if len(f.Search) > 0 && ((f.Authors != nil && f.Authors.Len() > 0) ||
		(f.Kinds != nil && f.Kinds.Len() > 0) ||
		(f.Tags != nil && f.Tags.Len() > 0)) {
		// Build serial list for fetching
		serials := make([]*types.Uint40, 0, len(idPkTs))
		for _, v := range idPkTs {
			s := new(types.Uint40)
			s.Set(v.Ser)
			serials = append(serials, s)
		}

		var evs map[uint64]*event.E
		if evs, err = w.FetchEventsBySerials(serials); chk.E(err) {
			return
		}

		filtered := make([]*store.IdPkTs, 0, len(idPkTs))
		for _, v := range idPkTs {
			ev, ok := evs[v.Ser]
			if !ok || ev == nil {
				continue
			}

			matchesAll := true
			if f.Authors != nil && f.Authors.Len() > 0 && !f.Authors.Contains(ev.Pubkey) {
				matchesAll = false
			}
			if matchesAll && f.Kinds != nil && f.Kinds.Len() > 0 && !f.Kinds.Contains(ev.Kind) {
				matchesAll = false
			}
			if matchesAll && f.Tags != nil && f.Tags.Len() > 0 {
				tagOK := true
				for _, t := range *f.Tags {
					if t.Len() < 2 {
						continue
					}
					key := t.Key()
					values := t.T[1:]
					if !ev.Tags.ContainsAny(key, values) {
						tagOK = false
						break
					}
				}
				if !tagOK {
					matchesAll = false
				}
			}
			if matchesAll {
				filtered = append(filtered, v)
			}
		}
		idPkTs = filtered
	}

	// Sort by timestamp (newest first)
	if len(f.Search) == 0 {
		sort.Slice(idPkTs, func(i, j int) bool {
			return idPkTs[i].Ts > idPkTs[j].Ts
		})
	} else {
		// Search ranking: blend match count with recency
		var maxCount int
		var minTs, maxTs int64
		if len(idPkTs) > 0 {
			minTs, maxTs = idPkTs[0].Ts, idPkTs[0].Ts
		}
		for _, v := range idPkTs {
			if c := counts[v.Ser]; c > maxCount {
				maxCount = c
			}
			if v.Ts < minTs {
				minTs = v.Ts
			}
			if v.Ts > maxTs {
				maxTs = v.Ts
			}
		}
		tsSpan := maxTs - minTs
		if tsSpan <= 0 {
			tsSpan = 1
		}
		if maxCount <= 0 {
			maxCount = 1
		}
		sort.Slice(idPkTs, func(i, j int) bool {
			ci := float64(counts[idPkTs[i].Ser]) / float64(maxCount)
			cj := float64(counts[idPkTs[j].Ser]) / float64(maxCount)
			ai := float64(idPkTs[i].Ts-minTs) / float64(tsSpan)
			aj := float64(idPkTs[j].Ts-minTs) / float64(tsSpan)
			si := 0.5*ci + 0.5*ai
			sj := 0.5*cj + 0.5*aj
			if si == sj {
				return idPkTs[i].Ts > idPkTs[j].Ts
			}
			return si > sj
		})
	}

	// Apply limit
	if f.Limit != nil && len(idPkTs) > int(*f.Limit) {
		idPkTs = idPkTs[:*f.Limit]
	}

	return
}

// QueryForSerials takes a filter and returns matching event serials
func (w *W) QueryForSerials(c context.Context, f *filter.F) (sers types.Uint40s, err error) {
	var founds []*types.Uint40
	var idPkTs []*store.IdPkTs

	if f.Ids != nil && f.Ids.Len() > 0 {
		// Use batch lookup for IDs
		var serialMap map[string]*types.Uint40
		if serialMap, err = w.GetSerialsByIds(f.Ids); chk.E(err) {
			return
		}
		for _, ser := range serialMap {
			founds = append(founds, ser)
		}
		var tmp []*store.IdPkTs
		if tmp, err = w.GetFullIdPubkeyBySerials(founds); chk.E(err) {
			return
		}
		idPkTs = append(idPkTs, tmp...)
	} else {
		if idPkTs, err = w.QueryForIds(c, f); chk.E(err) {
			return
		}
	}

	// Extract serials
	for _, idpk := range idPkTs {
		ser := new(types.Uint40)
		if err = ser.Set(idpk.Ser); chk.E(err) {
			continue
		}
		sers = append(sers, ser)
	}
	return
}

// QueryEvents queries events based on a filter
func (w *W) QueryEvents(c context.Context, f *filter.F) (evs event.S, err error) {
	return w.QueryEventsWithOptions(c, f, true, false)
}

// QueryAllVersions queries events and returns all versions of replaceable events
func (w *W) QueryAllVersions(c context.Context, f *filter.F) (evs event.S, err error) {
	return w.QueryEventsWithOptions(c, f, true, true)
}

// QueryEventsWithOptions queries events with additional options for deletion and versioning
func (w *W) QueryEventsWithOptions(c context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool) (evs event.S, err error) {
	wantMultipleVersions := showAllVersions || (f.Limit != nil && *f.Limit > 1)

	var expDeletes types.Uint40s
	var expEvs event.S

	// Handle ID-based queries
	if f.Ids != nil && f.Ids.Len() > 0 {
		w.Logger.Debugf("QueryEvents: ids path, count=%d", f.Ids.Len())

		serials, idErr := w.GetSerialsByIds(f.Ids)
		if idErr != nil {
			w.Logger.Warnf("QueryEvents: error looking up ids: %v", idErr)
		}

		// Convert to slice for batch fetch
		var serialsSlice []*types.Uint40
		idHexToSerial := make(map[uint64]string, len(serials))
		for idHex, ser := range serials {
			serialsSlice = append(serialsSlice, ser)
			idHexToSerial[ser.Get()] = idHex
		}

		// Batch fetch events
		var fetchedEvents map[uint64]*event.E
		if fetchedEvents, err = w.FetchEventsBySerials(serialsSlice); err != nil {
			w.Logger.Warnf("QueryEvents: batch fetch failed: %v", err)
			return
		}

		// Process fetched events
		for serialValue, ev := range fetchedEvents {
			idHex := idHexToSerial[serialValue]

			ser := new(types.Uint40)
			if err = ser.Set(serialValue); err != nil {
				continue
			}

			// Check expiration
			if CheckExpiration(ev) {
				w.Logger.Debugf("QueryEvents: id=%s filtered out due to expiration", idHex)
				expDeletes = append(expDeletes, ser)
				expEvs = append(expEvs, ev)
				continue
			}

			// Check for deletion
			if derr := w.CheckForDeleted(ev, nil); derr != nil {
				w.Logger.Debugf("QueryEvents: id=%s filtered out due to deletion: %v", idHex, derr)
				continue
			}

			evs = append(evs, ev)
		}

		// Sort and apply limit
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].CreatedAt > evs[j].CreatedAt
		})
		if f.Limit != nil && len(evs) > int(*f.Limit) {
			evs = evs[:*f.Limit]
		}
	} else {
		// Non-IDs path
		var idPkTs []*store.IdPkTs
		if idPkTs, err = w.QueryForIds(c, f); chk.E(err) {
			return
		}

		// Maps for replaceable event handling
		replaceableEvents := make(map[string]*event.E)
		replaceableEventVersions := make(map[string]event.S)
		paramReplaceableEvents := make(map[string]map[string]*event.E)
		paramReplaceableEventVersions := make(map[string]map[string]event.S)
		var regularEvents event.S

		// Deletion tracking maps
		deletionsByKindPubkey := make(map[string]bool)
		deletionsByKindPubkeyDTag := make(map[string]map[string]int64)
		deletedEventIds := make(map[string]bool)

		// Query for deletion events if we have authors
		if f.Authors != nil && f.Authors.Len() > 0 {
			deletionFilter := &filter.F{
				Kinds:   kind.NewS(kind.New(5)),
				Authors: f.Authors,
			}
			var deletionIdPkTs []*store.IdPkTs
			if deletionIdPkTs, err = w.QueryForIds(c, deletionFilter); err == nil {
				idPkTs = append(idPkTs, deletionIdPkTs...)
			}
		}

		// Prepare serials for batch fetch
		var allSerials []*types.Uint40
		serialToIdPk := make(map[uint64]*store.IdPkTs, len(idPkTs))
		for _, idpk := range idPkTs {
			ser := new(types.Uint40)
			if err = ser.Set(idpk.Ser); err != nil {
				continue
			}
			allSerials = append(allSerials, ser)
			serialToIdPk[ser.Get()] = idpk
		}

		// Batch fetch all events
		var allEvents map[uint64]*event.E
		if allEvents, err = w.FetchEventsBySerials(allSerials); err != nil {
			w.Logger.Warnf("QueryEvents: batch fetch failed in non-IDs path: %v", err)
			return
		}

		// First pass: collect deletion events
		for serialValue, ev := range allEvents {
			ser := new(types.Uint40)
			if err = ser.Set(serialValue); err != nil {
				continue
			}

			if CheckExpiration(ev) {
				expDeletes = append(expDeletes, ser)
				expEvs = append(expEvs, ev)
				continue
			}

			if ev.Kind == kind.Deletion.K {
				// Process e-tags and a-tags for deletion tracking
				aTags := ev.Tags.GetAll([]byte("a"))
				for _, aTag := range aTags {
					if aTag.Len() < 2 {
						continue
					}
					split := bytes.Split(aTag.Value(), []byte{':'})
					if len(split) < 2 {
						continue
					}
					kindInt, parseErr := strconv.Atoi(string(split[0]))
					if parseErr != nil {
						continue
					}
					kk := kind.New(uint16(kindInt))
					if !kind.IsReplaceable(kk.K) {
						continue
					}
					var pk []byte
					if pk, err = hex.DecAppend(nil, split[1]); err != nil {
						continue
					}
					if !utils.FastEqual(pk, ev.Pubkey) {
						continue
					}
					key := hex.Enc(pk) + ":" + strconv.Itoa(int(kk.K))

					if kind.IsParameterizedReplaceable(kk.K) {
						if len(split) < 3 {
							continue
						}
						if _, exists := deletionsByKindPubkeyDTag[key]; !exists {
							deletionsByKindPubkeyDTag[key] = make(map[string]int64)
						}
						dValue := string(split[2])
						if ts, ok := deletionsByKindPubkeyDTag[key][dValue]; !ok || ev.CreatedAt > ts {
							deletionsByKindPubkeyDTag[key][dValue] = ev.CreatedAt
						}
					} else {
						deletionsByKindPubkey[key] = true
					}
				}

				// Process e-tags for specific event deletions
				eTags := ev.Tags.GetAll([]byte("e"))
				for _, eTag := range eTags {
					eTagHex := eTag.ValueHex()
					if len(eTagHex) != 64 {
						continue
					}
					evId := make([]byte, sha256.Size)
					if _, hexErr := hex.DecBytes(evId, eTagHex); hexErr != nil {
						continue
					}

					// Look for target in current batch
					var targetEv *event.E
					for _, candidateEv := range allEvents {
						if utils.FastEqual(candidateEv.ID, evId) {
							targetEv = candidateEv
							break
						}
					}

					// Try to fetch if not in batch
					if targetEv == nil {
						ser, serErr := w.GetSerialById(evId)
						if serErr != nil || ser == nil {
							continue
						}
						targetEv, serErr = w.FetchEventBySerial(ser)
						if serErr != nil || targetEv == nil {
							continue
						}
					}

					if !utils.FastEqual(targetEv.Pubkey, ev.Pubkey) {
						continue
					}
					deletedEventIds[hex.Enc(targetEv.ID)] = true
				}
			}
		}

		// Second pass: process all events, filtering deleted ones
		for _, ev := range allEvents {
			// Tag filter verification
			if f.Tags != nil && f.Tags.Len() > 0 {
				tagMatches := 0
				for _, filterTag := range *f.Tags {
					if filterTag.Len() >= 2 {
						filterKey := filterTag.Key()
						var actualKey []byte
						if len(filterKey) == 2 && filterKey[0] == '#' {
							actualKey = filterKey[1:]
						} else {
							actualKey = filterKey
						}
						eventHasTag := false
						if ev.Tags != nil {
							for _, eventTag := range *ev.Tags {
								if eventTag.Len() >= 2 && bytes.Equal(eventTag.Key(), actualKey) {
									for _, filterValue := range filterTag.T[1:] {
										if database.TagValuesMatchUsingTagMethods(eventTag, filterValue) {
											eventHasTag = true
											break
										}
									}
									if eventHasTag {
										break
									}
								}
							}
						}
						if eventHasTag {
							tagMatches++
						}
					}
				}
				if tagMatches < f.Tags.Len() {
					continue
				}
			}

			// Skip deletion events unless explicitly requested
			if ev.Kind == kind.Deletion.K {
				kind5Requested := false
				if f.Kinds != nil && f.Kinds.Len() > 0 {
					for i := 0; i < f.Kinds.Len(); i++ {
						if f.Kinds.K[i].K == kind.Deletion.K {
							kind5Requested = true
							break
						}
					}
				}
				if !kind5Requested {
					continue
				}
			}

			// Check if event ID is in filter
			isIdInFilter := false
			if f.Ids != nil && f.Ids.Len() > 0 {
				for i := 0; i < f.Ids.Len(); i++ {
					if utils.FastEqual(ev.ID, (*f.Ids).T[i]) {
						isIdInFilter = true
						break
					}
				}
			}

			// Check if specifically deleted
			eventIdHex := hex.Enc(ev.ID)
			if deletedEventIds[eventIdHex] {
				continue
			}

			// Handle replaceable events
			if kind.IsReplaceable(ev.Kind) {
				key := hex.Enc(ev.Pubkey) + ":" + strconv.Itoa(int(ev.Kind))
				if deletionsByKindPubkey[key] && !isIdInFilter {
					continue
				} else if wantMultipleVersions {
					replaceableEventVersions[key] = append(replaceableEventVersions[key], ev)
				} else {
					existing, exists := replaceableEvents[key]
					if !exists || ev.CreatedAt > existing.CreatedAt {
						replaceableEvents[key] = ev
					}
				}
			} else if kind.IsParameterizedReplaceable(ev.Kind) {
				key := hex.Enc(ev.Pubkey) + ":" + strconv.Itoa(int(ev.Kind))
				dTag := ev.Tags.GetFirst([]byte("d"))
				var dValue string
				if dTag != nil && dTag.Len() > 1 {
					dValue = string(dTag.Value())
				}

				if deletionMap, exists := deletionsByKindPubkeyDTag[key]; exists {
					if delTs, ok := deletionMap[dValue]; ok && ev.CreatedAt < delTs && !isIdInFilter {
						continue
					}
				}

				if wantMultipleVersions {
					if _, exists := paramReplaceableEventVersions[key]; !exists {
						paramReplaceableEventVersions[key] = make(map[string]event.S)
					}
					paramReplaceableEventVersions[key][dValue] = append(paramReplaceableEventVersions[key][dValue], ev)
				} else {
					if _, exists := paramReplaceableEvents[key]; !exists {
						paramReplaceableEvents[key] = make(map[string]*event.E)
					}
					existing, exists := paramReplaceableEvents[key][dValue]
					if !exists || ev.CreatedAt > existing.CreatedAt {
						paramReplaceableEvents[key][dValue] = ev
					}
				}
			} else {
				regularEvents = append(regularEvents, ev)
			}
		}

		// Collect results
		if wantMultipleVersions {
			for _, versions := range replaceableEventVersions {
				sort.Slice(versions, func(i, j int) bool {
					return versions[i].CreatedAt > versions[j].CreatedAt
				})
				limit := len(versions)
				if f.Limit != nil && int(*f.Limit) < limit {
					limit = int(*f.Limit)
				}
				for i := 0; i < limit; i++ {
					evs = append(evs, versions[i])
				}
			}
		} else {
			for _, ev := range replaceableEvents {
				evs = append(evs, ev)
			}
		}

		if wantMultipleVersions {
			for _, dTagMap := range paramReplaceableEventVersions {
				for _, versions := range dTagMap {
					sort.Slice(versions, func(i, j int) bool {
						return versions[i].CreatedAt > versions[j].CreatedAt
					})
					limit := len(versions)
					if f.Limit != nil && int(*f.Limit) < limit {
						limit = int(*f.Limit)
					}
					for i := 0; i < limit; i++ {
						evs = append(evs, versions[i])
					}
				}
			}
		} else {
			for _, innerMap := range paramReplaceableEvents {
				for _, ev := range innerMap {
					evs = append(evs, ev)
				}
			}
		}

		evs = append(evs, regularEvents...)

		// Sort and limit
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].CreatedAt > evs[j].CreatedAt
		})
		if f.Limit != nil && len(evs) > int(*f.Limit) {
			evs = evs[:*f.Limit]
		}

		// Delete expired events in background
		go func() {
			for i, ser := range expDeletes {
				w.DeleteEventBySerial(context.Background(), ser, expEvs[i])
			}
		}()
	}

	return
}

// QueryDeleteEventsByTargetId queries for delete events targeting a specific event ID
func (w *W) QueryDeleteEventsByTargetId(c context.Context, targetEventId []byte) (evs event.S, err error) {
	f := &filter.F{
		Kinds: kind.NewS(kind.Deletion),
		Tags: tag.NewS(
			tag.NewFromAny("#e", hex.Enc(targetEventId)),
		),
	}
	return w.QueryEventsWithOptions(c, f, true, false)
}

// CountEvents counts events matching a filter
func (w *W) CountEvents(c context.Context, f *filter.F) (count int, approx bool, err error) {
	approx = false
	if f == nil {
		return 0, false, nil
	}

	// For ID-based queries, count resolved IDs
	if f.Ids != nil && f.Ids.Len() > 0 {
		serials, idErr := w.GetSerialsByIds(f.Ids)
		if idErr != nil {
			return 0, false, idErr
		}
		return len(serials), false, nil
	}

	// For other queries, get serials and count
	var sers types.Uint40s
	if sers, err = w.QueryForSerials(c, f); err != nil {
		return 0, false, err
	}

	return len(sers), false, nil
}

// GetSerialsFromFilter is an alias for QueryForSerials for interface compatibility
func (w *W) GetSerialsFromFilter(f *filter.F) (serials types.Uint40s, err error) {
	return w.QueryForSerials(w.ctx, f)
}
