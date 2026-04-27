//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"github.com/minio/sha256-simd"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/mode"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/ints"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/interfaces/store"
	"git.smesh.lol/orly/pkg/utils"
)

func CheckExpiration(ev *event.E) (expired bool) {
	var err error
	expTag := ev.Tags.GetFirst([]byte("expiration"))
	if expTag != nil {
		expTS := ints.New(0)
		if _, err = expTS.Unmarshal(expTag.Value()); !chk.E(err) {
			if int64(expTS.N) < time.Now().Unix() {
				return true
			}
		}
	}
	return
}

func (d *D) QueryEvents(c context.Context, f *filter.F) (
	evs event.S, err error,
) {
	return d.QueryEventsWithOptions(c, f, true, false)
}

// QueryAllVersions queries events and returns all versions of replaceable events
func (d *D) QueryAllVersions(c context.Context, f *filter.F) (
	evs event.S, err error,
) {
	return d.QueryEventsWithOptions(c, f, true, true)
}

func (d *D) QueryEventsWithOptions(c context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool) (
	evs event.S, err error,
) {
	// Acquire query slot to limit concurrent queries and prevent memory exhaustion
	if !d.AcquireQuerySlot(c) {
		err = c.Err()
		return
	}
	defer d.ReleaseQuerySlot()

	// Fast path for NIP-33 addressable event queries (kind + author + d-tag)
	// This provides O(1) direct lookup instead of index scanning
	if !showAllVersions && IsAddressableEventQuery(f) {
		var serial *types.Uint40
		if serial, err = d.QueryForAddressableEvent(f); err != nil {
			log.W.F("QueryEvents: addressable event fast path error: %v", err)
			// Fall through to regular query path
			err = nil
		} else if serial != nil {
			// Found via fast path - fetch the event
			ev, fetchErr := d.FetchEventBySerial(serial)
			if fetchErr == nil && ev != nil {
				// Apply time filters if present
				if f.Since != nil && ev.CreatedAt < f.Since.V {
					log.T.F("QueryEvents: addressable event fast path - event before 'since' filter")
					return // Return empty result
				}
				if f.Until != nil && ev.CreatedAt > f.Until.V {
					log.T.F("QueryEvents: addressable event fast path - event after 'until' filter")
					return // Return empty result
				}
				// Check deletion status (skip in open relay mode)
				if !mode.IsOpen() {
					if derr := d.CheckForDeleted(ev, nil); derr != nil {
						log.T.F("QueryEvents: addressable event fast path - event deleted: %v", derr)
						return // Return empty result
					}
				}
				// Check expiration
				if !mode.IsOpen() && CheckExpiration(ev) {
					log.T.F("QueryEvents: addressable event fast path - event expired")
					return // Return empty result
				}
				log.T.F("QueryEvents: addressable event fast path SUCCESS - kind=%d d=%s",
					ev.Kind, string(ev.Tags.GetFirst([]byte("d")).Value()))
				evs = append(evs, ev)
				return
			}
			// Fetch failed - fall through to regular path
			log.T.F("QueryEvents: addressable event fast path - fetch failed, falling back")
		}
		// Not found via fast path - fall through to regular query path
		// This handles the case where the index doesn't exist yet (migration)
	}

	// Determine if we should return multiple versions of replaceable events
	// based on the limit parameter
	wantMultipleVersions := showAllVersions || (f.Limit != nil && *f.Limit > 1)

	// if there is Ids in the query, this overrides anything else
	var expDeletes types.Uint40s
	var expEvs event.S
	if f.Ids != nil && f.Ids.Len() > 0 {
		// Get all serials for the requested IDs in a single batch operation
		log.T.F("QueryEvents: ids path, count=%d", f.Ids.Len())

		// Use GetSerialsByIds to batch process all IDs at once
		serials, idErr := d.GetSerialsByIds(f.Ids)
		if idErr != nil {
			log.E.F("QueryEvents: error looking up ids: %v", idErr)
			// Continue with whatever IDs we found
		}

		// Convert serials map to slice for batch fetch
		var serialsSlice []*types.Uint40
		serialsSlice = make([]*types.Uint40, 0, len(serials))
		idHexToSerial := make(map[uint64]string, len(serials)) // Map serial value back to original ID hex
		for idHex, ser := range serials {
			serialsSlice = append(serialsSlice, ser)
			idHexToSerial[ser.Get()] = idHex
		}

		// Fetch all events in a single batch operation
		var fetchedEvents map[uint64]*event.E
		if fetchedEvents, err = d.FetchEventsBySerials(serialsSlice); err != nil {
			log.E.F("QueryEvents: batch fetch failed: %v", err)
			return
		}

		// Process each successfully fetched event and apply filters
		for serialValue, ev := range fetchedEvents {
			idHex := idHexToSerial[serialValue]

			// Convert serial value back to Uint40 for expiration handling
			ser := new(types.Uint40)
			if err = ser.Set(serialValue); err != nil {
				log.T.F(
					"QueryEvents: error converting serial %d: %v", serialValue,
					err,
				)
				continue
			}

			// check for an expiration tag and delete after returning the result
			// Skip expiration check when ACL is "none" (open relay mode)
			if !mode.IsOpen() && CheckExpiration(ev) {
				log.T.F(
					"QueryEvents: id=%s filtered out due to expiration", idHex,
				)
				expDeletes = append(expDeletes, ser)
				expEvs = append(expEvs, ev)
				continue
			}

			// skip events that have been deleted by a proper deletion event
			// Skip deletion check when ACL is "none" (open relay mode)
			if !mode.IsOpen() {
				if derr := d.CheckForDeleted(ev, nil); derr != nil {
					log.T.F("QueryEvents: id=%s filtered out due to deletion: %v", idHex, derr)
					continue
				}
			}

			// Add the event to the results
			evs = append(evs, ev)
			// log.T.F("QueryEvents: id=%s SUCCESSFULLY FOUND, adding to results", idHex)
		}

		// sort the events by timestamp
		sort.Slice(
			evs, func(i, j int) bool {
				return evs[i].CreatedAt > evs[j].CreatedAt
			},
		)
		// Apply limit after processing
		if f.Limit != nil && len(evs) > int(*f.Limit) {
			evs = evs[:*f.Limit]
		}
	} else {
		// non-IDs path
		var idPkTs []*store.IdPkTs
		// if f.Authors != nil && f.Authors.Len() > 0 && f.Kinds != nil && f.Kinds.Len() > 0 {
		// log.T.F("QueryEvents: authors+kinds path, authors=%d kinds=%d", f.Authors.Len(), f.Kinds.Len())
		// }
		if idPkTs, err = d.QueryForIds(c, f); chk.E(err) {
			return
		}
		// log.T.F("QueryEvents: QueryForIds returned %d candidates", len(idPkTs))
		// Create a map to store versions of replaceable events
		// If wantMultipleVersions is true, we keep multiple versions (sorted by timestamp)
		// Otherwise, we keep only the latest
		replaceableEvents := make(map[string]*event.E)
		replaceableEventVersions := make(map[string]event.S) // For multiple versions
		// Create a map to store the latest version of parameterized replaceable
		// events
		paramReplaceableEvents := make(map[string]map[string]*event.E)
		paramReplaceableEventVersions := make(map[string]map[string]event.S) // For multiple versions
		// Regular events that are not replaceable
		var regularEvents event.S
		// Map to track deletion events by kind and pubkey (for replaceable
		// events)
		deletionsByKindPubkey := make(map[string]bool)
		// Map to track deletion events by kind, pubkey, and d-tag (for
		// parameterized replaceable events). We store the newest delete timestamp per d-tag.
		deletionsByKindPubkeyDTag := make(map[string]map[string]int64)
		// Map to track specific event IDs that have been deleted
		deletedEventIds := make(map[string]bool)
		// Query for deletion events separately if we have authors in the filter
		// We always need to fetch deletion events to build deletion maps, even if
		// they're not explicitly requested in the kind filter
		if f.Authors != nil && f.Authors.Len() > 0 {
			// Create a filter for deletion events with the same authors
			deletionFilter := &filter.F{
				Kinds:   kind.NewS(kind.New(5)), // Kind 5 is deletion
				Authors: f.Authors,
			}

			var deletionIdPkTs []*store.IdPkTs
			if deletionIdPkTs, err = d.QueryForIds(
				c, deletionFilter,
			); chk.E(err) {
				return
			}

			// Add deletion events to the list of events to process
			idPkTs = append(idPkTs, deletionIdPkTs...)
		}
		// Prepare serials for batch fetch
		var allSerials []*types.Uint40
		allSerials = make([]*types.Uint40, 0, len(idPkTs))
		serialToIdPk := make(map[uint64]*store.IdPkTs, len(idPkTs))
		for _, idpk := range idPkTs {
			ser := new(types.Uint40)
			if err = ser.Set(idpk.Ser); err != nil {
				continue
			}
			allSerials = append(allSerials, ser)
			serialToIdPk[ser.Get()] = idpk
		}

		// Fetch all events in batch
		var allEvents map[uint64]*event.E
		if allEvents, err = d.FetchEventsBySerials(allSerials); err != nil {
			log.E.F("QueryEvents: batch fetch failed in non-IDs path: %v", err)
			return
		}

		// First pass: collect all deletion events
		for serialValue, ev := range allEvents {
			// Convert serial value back to Uint40 for expiration handling
			ser := new(types.Uint40)
			if err = ser.Set(serialValue); err != nil {
				continue
			}

			// check for an expiration tag and delete after returning the result
			// Skip expiration check when ACL is "none" (open relay mode)
			if !mode.IsOpen() && CheckExpiration(ev) {
				expDeletes = append(expDeletes, ser)
				expEvs = append(expEvs, ev)
				continue
			}
			// Process deletion events to build our deletion maps
			// Skip deletion processing when ACL is "none" (open relay mode)
			if !mode.IsOpen() && ev.Kind == kind.Deletion.K {
				// Check for 'e' tags that directly reference event IDs
				eTags := ev.Tags.GetAll([]byte("e"))
				for _, eTag := range eTags {
					if eTag.Len() < 2 {
						continue
					}
					// We don't need to do anything with direct event ID
					// references as we will filter those out in the second pass
				}
				// Check for 'a' tags that reference replaceable events
				aTags := ev.Tags.GetAll([]byte("a"))
				for _, aTag := range aTags {
					if aTag.Len() < 2 {
						continue
					}
					// Parse the 'a' tag value: kind:pubkey:d-tag (for parameterized) or kind:pubkey (for regular)
					split := bytes.Split(aTag.Value(), []byte{':'})
					if len(split) < 2 {
						continue
					}
					// Parse the kind
					kindStr := string(split[0])
					kindInt, err := strconv.Atoi(kindStr)
					if err != nil {
						continue
					}
					kk := kind.New(uint16(kindInt))
					// Process both regular and parameterized replaceable events
					if !kind.IsReplaceable(kk.K) {
						continue
					}
					// Parse the pubkey
					var pk []byte
					if pk, err = hex.DecAppend(nil, split[1]); err != nil {
						continue
					}
					// Only allow users to delete their own events
					if !utils.FastEqual(pk, ev.Pubkey) {
						continue
					}
					// Create the key for the deletion map using hex
					// representation of pubkey
					key := hex.Enc(pk) + ":" + strconv.Itoa(int(kk.K))

					if kind.IsParameterizedReplaceable(kk.K) {
						// For parameterized replaceable events, use d-tag specific deletion
						if len(split) < 3 {
							continue
						}
						// Initialize the inner map if it doesn't exist
						if _, exists := deletionsByKindPubkeyDTag[key]; !exists {
							deletionsByKindPubkeyDTag[key] = make(map[string]int64)
						}
						// Record the newest delete timestamp for this d-tag
						dValue := string(split[2])
						if ts, ok := deletionsByKindPubkeyDTag[key][dValue]; !ok || ev.CreatedAt > ts {
							deletionsByKindPubkeyDTag[key][dValue] = ev.CreatedAt
						}
					} else {
						// For regular replaceable events, mark as deleted by kind/pubkey
						deletionsByKindPubkey[key] = true
					}
				}
				// For replaceable events, we need to check if there are any
				// e-tags that reference events with the same kind and pubkey
				for _, eTag := range eTags {
					// Use ValueHex() to handle both binary and hex storage formats
					eTagHex := eTag.ValueHex()
					if len(eTagHex) != 64 {
						continue
					}
					// Get the event ID from the e-tag
					evId := make([]byte, sha256.Size)
					if _, err = hex.DecBytes(evId, eTagHex); err != nil {
						continue
					}

					// Look for the target event in our current batch instead of querying
					var targetEv *event.E
					for _, candidateEv := range allEvents {
						if utils.FastEqual(candidateEv.ID, evId) {
							targetEv = candidateEv
							break
						}
					}

					// DISABLED: Fetching target events not in current batch causes memory
					// exhaustion under high concurrent load. Each GetSerialById call creates
					// a Badger iterator that consumes significant memory. With many connections
					// processing deletion events, this explodes memory usage (~300MB/connection).
					// The deletion will still work when the target event is directly queried.
					if targetEv == nil {
						continue
					}

					// Only allow users to delete their own events
					if !utils.FastEqual(targetEv.Pubkey, ev.Pubkey) {
						continue
					}
					// Mark the specific event ID as deleted
					deletedEventIds[hex.Enc(targetEv.ID)] = true
					// Note: For e-tag deletions, we only mark the specific event as deleted,
					// not all events of the same kind/pubkey
				}
			}
		}
		// Second pass: process all events, filtering out deleted ones
		for _, ev := range allEvents {
			// Add logging for tag filter debugging
			if f.Tags != nil && f.Tags.Len() > 0 {
				// var eventTags []string
				// if ev.Tags != nil && ev.Tags.Len() > 0 {
				// 	for _, t := range *ev.Tags {
				// 		if t.Len() >= 2 {
				// 			eventTags = append(
				// 				eventTags,
				// 				string(t.Key())+"="+string(t.Value()),
				// 			)
				// 		}
				// 	}
				// }
				// log.T.F(
				// 	"QueryEvents: processing event ID=%s kind=%d tags=%v",
				// 	hex.Enc(ev.ID), ev.Kind, eventTags,
				// )
				// Check if this event matches ALL required tags in the filter
				tagMatches := 0
				for _, filterTag := range *f.Tags {
					if filterTag.Len() >= 2 {
						filterKey := filterTag.Key()
						// Handle filter keys that start with # (remove the prefix for comparison)
						var actualKey []byte
						if len(filterKey) == 2 && filterKey[0] == '#' {
							actualKey = filterKey[1:]
						} else {
							actualKey = filterKey
						}
						// Check if event has this tag key with any of the filter's values
						eventHasTag := false
						if ev.Tags != nil {
							for _, eventTag := range *ev.Tags {
								if eventTag.Len() >= 2 && bytes.Equal(
									eventTag.Key(), actualKey,
								) {
									// Check if the event's tag value matches any of the filter's values
									// Using TagValuesMatchUsingTagMethods handles binary/hex conversion
									// for e/p tags automatically
									for _, filterValue := range filterTag.T[1:] {
										if TagValuesMatchUsingTagMethods(eventTag, filterValue) {
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
						// log.T.F(
						// 	"QueryEvents: tag filter %s (actual key: %s) matches: %v (total matches: %d/%d)",
						// 	string(filterKey), string(actualKey), eventHasTag,
						// 	tagMatches, f.Tags.Len(),
						// )
					}
				}

				// If not all tags match, skip this event
				if tagMatches < f.Tags.Len() {
					// log.T.F(
					// 	"QueryEvents: event ID=%s SKIPPED - only matches %d/%d required tags",
					// 	hex.Enc(ev.ID), tagMatches, f.Tags.Len(),
					// )
					continue
				}
				// log.T.F(
				// 	"QueryEvents: event ID=%s PASSES all tag filters",
				// 	hex.Enc(ev.ID),
				// )
			}

			// Skip events with kind 5 (Deletion) unless explicitly requested in the filter
			if ev.Kind == kind.Deletion.K {
				// Check if kind 5 (deletion) is explicitly requested in the filter
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
			// Check if this event's ID is in the filter
			isIdInFilter := false
			if f.Ids != nil && f.Ids.Len() > 0 {
				for i := 0; i < f.Ids.Len(); i++ {
					if utils.FastEqual(ev.ID, (*f.Ids).T[i]) {
						isIdInFilter = true
						break
					}
				}
			}
			// Check if this specific event has been deleted
			// Skip deletion checks when ACL is "none" (open relay mode)
			aclActive := !mode.IsOpen()
			eventIdHex := hex.Enc(ev.ID)
			if aclActive && deletedEventIds[eventIdHex] {
				// Skip this event if it has been specifically deleted
				continue
			}
			if kind.IsReplaceable(ev.Kind) {
				// For replaceable events, we only keep the latest version for
				// each pubkey and kind, and only if it hasn't been deleted
				key := hex.Enc(ev.Pubkey) + ":" + strconv.Itoa(int(ev.Kind))
				// For replaceable events, we need to be more careful with
				// deletion Only skip this event if it has been deleted by
				// kind/pubkey and is not in the filter AND there isn't a newer
				// event with the same kind/pubkey
				if aclActive && deletionsByKindPubkey[key] && !isIdInFilter {
					// This replaceable event has been deleted, skip it
					continue
				} else if wantMultipleVersions {
					// If wantMultipleVersions is true, collect all versions
					replaceableEventVersions[key] = append(replaceableEventVersions[key], ev)
				} else {
					// Normal replaceable event handling - keep only the newest
					existing, exists := replaceableEvents[key]
					if !exists || ev.CreatedAt > existing.CreatedAt {
						replaceableEvents[key] = ev
					}
				}
			} else if kind.IsParameterizedReplaceable(ev.Kind) {
				// For parameterized replaceable events, we need to consider the
				// 'd' tag
				key := hex.Enc(ev.Pubkey) + ":" + strconv.Itoa(int(ev.Kind))

				// Get the 'd' tag value
				dTag := ev.Tags.GetFirst([]byte("d"))
				var dValue string
				if dTag != nil && dTag.Len() > 1 {
					dValue = string(dTag.Value())
				} else {
					// If no 'd' tag, use empty string
					dValue = ""
				}

				// Check if this event has been deleted via an a-tag
				// Skip deletion check when ACL is "none" (open relay mode)
				if aclActive {
					if deletionMap, exists := deletionsByKindPubkeyDTag[key]; exists {
						// If there is a deletion timestamp and this event is older than the deletion,
						// and this event is not specifically requested by ID, skip it
						if delTs, ok := deletionMap[dValue]; ok && ev.CreatedAt < delTs && !isIdInFilter {
							continue
						}
					}
				}

				if wantMultipleVersions {
					// If wantMultipleVersions is true, collect all versions
					if _, exists := paramReplaceableEventVersions[key]; !exists {
						paramReplaceableEventVersions[key] = make(map[string]event.S)
					}
					paramReplaceableEventVersions[key][dValue] = append(paramReplaceableEventVersions[key][dValue], ev)
				} else {
					// Initialize the inner map if it doesn't exist
					if _, exists := paramReplaceableEvents[key]; !exists {
						paramReplaceableEvents[key] = make(map[string]*event.E)
					}

					// Check if we already have an event with this 'd' tag value
					existing, exists := paramReplaceableEvents[key][dValue]
					// Only keep the newer event, regardless of processing order
					if !exists {
						// No existing event, add this one
						paramReplaceableEvents[key][dValue] = ev
					} else if ev.CreatedAt > existing.CreatedAt {
						// This event is newer than the existing one, replace it
						paramReplaceableEvents[key][dValue] = ev
					}
				}
				// If this event is older than the existing one, ignore it
			} else {
				// Regular events
				regularEvents = append(regularEvents, ev)
			}
		}
		// Add all the latest replaceable events to the result
		if wantMultipleVersions {
			// Add all versions (sorted by timestamp, newest first)
			for key, versions := range replaceableEventVersions {
				// Sort versions by timestamp (newest first)
				sort.Slice(versions, func(i, j int) bool {
					return versions[i].CreatedAt > versions[j].CreatedAt
				})
				// Add versions up to the limit
				limit := len(versions)
				if f.Limit != nil && int(*f.Limit) < limit {
					limit = int(*f.Limit)
				}
				for i := 0; i < limit && i < len(versions); i++ {
					evs = append(evs, versions[i])
				}
				_ = key // Use key to avoid unused variable warning
			}
		} else {
			// Add only the newest version of each replaceable event
			for _, ev := range replaceableEvents {
				evs = append(evs, ev)
			}
		}

		// Add all the latest parameterized replaceable events to the result
		if wantMultipleVersions {
			// Add all versions (sorted by timestamp, newest first)
			for key, dTagMap := range paramReplaceableEventVersions {
				for dTag, versions := range dTagMap {
					// Sort versions by timestamp (newest first)
					sort.Slice(versions, func(i, j int) bool {
						return versions[i].CreatedAt > versions[j].CreatedAt
					})
					// Add versions up to the limit
					limit := len(versions)
					if f.Limit != nil && int(*f.Limit) < limit {
						limit = int(*f.Limit)
					}
					for i := 0; i < limit && i < len(versions); i++ {
						evs = append(evs, versions[i])
					}
					_ = key  // Use key to avoid unused variable warning
					_ = dTag // Use dTag to avoid unused variable warning
				}
			}
		} else {
			// Add only the newest version of each parameterized replaceable event
			for _, innerMap := range paramReplaceableEvents {
				for _, ev := range innerMap {
					evs = append(evs, ev)
				}
			}
		}
		// Add all regular events to the result
		evs = append(evs, regularEvents...)
		// Sort all events by timestamp (newest first)
		sort.Slice(
			evs, func(i, j int) bool {
				return evs[i].CreatedAt > evs[j].CreatedAt
			},
		)
		// Apply limit after processing replaceable/addressable events
		if f.Limit != nil && len(evs) > int(*f.Limit) {
			evs = evs[:*f.Limit]
		}
		// TODO: DISABLED - inline deletion of expired events causes Badger race conditions
		// under high concurrent load ("assignment to entry in nil map" panic).
		// Expired events should be cleaned up by a separate, rate-limited background
		// worker instead of being deleted inline during query processing.
		// See: pkg/storage/gc.go TODOs for proper batch deletion implementation.
		if len(expDeletes) > 0 {
			log.D.F("QueryEvents: found %d expired events (deletion disabled)", len(expDeletes))
		}
	}

	return
}

// QueryDeleteEventsByTargetId queries for delete events that target a specific event ID
func (d *D) QueryDeleteEventsByTargetId(c context.Context, targetEventId []byte) (
	evs event.S, err error,
) {
	// Create a filter for deletion events with the target event ID in e-tags
	f := &filter.F{
		Kinds: kind.NewS(kind.Deletion),
		Tags: tag.NewS(
			tag.NewFromAny("#e", hex.Enc(targetEventId)),
		),
	}

	// Query for the delete events
	if evs, err = d.QueryEventsWithOptions(c, f, true, false); chk.E(err) {
		return
	}

	return
}
