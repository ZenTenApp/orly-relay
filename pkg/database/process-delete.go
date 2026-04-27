//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"sort"
	"strconv"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	hexenc "git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/ints"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/interfaces/store"
	"git.smesh.lol/orly/pkg/utils"
)

func (d *D) ProcessDelete(ev *event.E, admins [][]byte) (err error) {
	eTags := ev.Tags.GetAll([]byte("e"))
	aTags := ev.Tags.GetAll([]byte("a"))
	kTags := ev.Tags.GetAll([]byte("k"))
	
	// Process e-tags: delete specific events by ID
	for _, eTag := range eTags {
		if eTag.Len() < 2 {
			continue
		}
		// Use ValueHex() to handle both binary and hex storage formats
		eventIdHex := eTag.ValueHex()
		if len(eventIdHex) != 64 { // hex encoded event ID
			continue
		}
		// Decode hex event ID
		var eid []byte
		if eid, err = hexenc.DecAppend(nil, eventIdHex); chk.E(err) {
			continue
		}
		// Fetch the event to verify ownership
		var ser *types.Uint40
		if ser, err = d.GetSerialById(eid); chk.E(err) || ser == nil {
			continue
		}
		var targetEv *event.E
		if targetEv, err = d.FetchEventBySerial(ser); chk.E(err) || targetEv == nil {
			continue
		}
		// Only allow users to delete their own events
		if !utils.FastEqual(targetEv.Pubkey, ev.Pubkey) {
			continue
		}
		// Delete the event
		if err = d.DeleteEvent(context.Background(), eid); chk.E(err) {
			log.W.F("failed to delete event %x via e-tag: %v", eid, err)
			continue
		}
		log.D.F("deleted event %x via e-tag deletion", eid)
	}
	
	// Process a-tags: delete addressable events by kind:pubkey:d-tag
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
		kindInt, parseErr := strconv.Atoi(kindStr)
		if parseErr != nil {
			continue
		}
		kk := kind.New(uint16(kindInt))
		// Parse the pubkey
		var pk []byte
		if pk, err = hexenc.DecAppend(nil, split[1]); chk.E(err) {
			continue
		}
		// Only allow users to delete their own events
		if !utils.FastEqual(pk, ev.Pubkey) {
			continue
		}
		
		// Build filter for events to delete
		delFilter := &filter.F{
			Authors: tag.NewFromBytesSlice(pk),
			Kinds:   kind.NewS(kk),
		}
		
		// For parameterized replaceable events, add d-tag filter
		if kind.IsParameterizedReplaceable(kk.K) && len(split) >= 3 {
			dValue := split[2]
			delFilter.Tags = tag.NewS(tag.NewFromAny([]byte("d"), dValue))
		}
		
		// Find matching events
		var idxs []Range
		if idxs, err = GetIndexesFromFilter(delFilter); chk.E(err) {
			continue
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = d.GetSerialsByRange(idx); chk.E(err) {
				continue
			}
			sers = append(sers, s...)
		}
		
		// Delete events older than the deletion event
		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = d.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				continue
			}
			idPkTss = append(idPkTss, tmp...)
			// Sort by timestamp
			sort.Slice(idPkTss, func(i, j int) bool {
				return idPkTss[i].Ts > idPkTss[j].Ts
			})
			for _, v := range idPkTss {
				if v.Ts < ev.CreatedAt {
					if err = d.DeleteEvent(context.Background(), v.Id[:]); chk.E(err) {
						log.W.F("failed to delete event %x via a-tag: %v", v.Id[:], err)
						continue
					}
					log.D.F("deleted event %x via a-tag deletion", v.Id[:])
				}
			}
		}
	}
	
	// h-tag deletion: admin-only bulk delete of kind 445 events by group ID.
	// Kind 445 (MLS group messages) uses ephemeral keys per MIP-03, so
	// standard pubkey ownership checks cannot apply. Relay admins can
	// delete by h-tag to support forward secrecy wipe after group ratchet.
	hTags := ev.Tags.GetAll([]byte("h"))
	if len(hTags) > 0 {
		isAdmin := false
		for _, admin := range admins {
			if utils.FastEqual(admin, ev.Pubkey) {
				isAdmin = true
				break
			}
		}
		if isAdmin {
			for _, hTag := range hTags {
				hVal := hTag.Value()
				if len(hVal) == 0 {
					continue
				}
				delFilter := &filter.F{
					Kinds: kind.NewS(kind.New(445)),
					Tags:  tag.NewS(tag.NewFromAny([]byte("h"), hVal)),
				}
				var idxs []Range
				if idxs, err = GetIndexesFromFilter(delFilter); chk.E(err) {
					continue
				}
				var sers types.Uint40s
				for _, idx := range idxs {
					var s types.Uint40s
					if s, err = d.GetSerialsByRange(idx); chk.E(err) {
						continue
					}
					sers = append(sers, s...)
				}
				deleted := 0
				for _, ser := range sers {
					var targetEv *event.E
					if targetEv, err = d.FetchEventBySerial(ser); chk.E(err) || targetEv == nil {
						continue
					}
					if targetEv.CreatedAt < ev.CreatedAt {
						if err = d.DeleteEvent(context.Background(), targetEv.ID); chk.E(err) {
							continue
						}
						deleted++
					}
				}
				if deleted > 0 {
					log.D.F("admin deleted %d kind 445 events via h-tag %s", deleted, string(hVal))
				}
			}
		}
	}

	// if there are no e or a tags, we assume the intent is to delete all
	// replaceable events of the kinds specified by the k tags for the pubkey of
	// the delete event.
	if len(eTags) == 0 && len(aTags) == 0 {
		// parse the kind tags
		var kinds []*kind.K
		for _, k := range kTags {
			kv := k.Value()
			iv := ints.New(0)
			if _, err = iv.Unmarshal(kv); chk.E(err) {
				continue
			}
			kinds = append(kinds, kind.New(iv.N))
		}
		var idxs []Range
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: tag.NewFromBytesSlice(ev.Pubkey),
				Kinds:   kind.NewS(kinds...),
			},
		); chk.E(err) {
			return
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = d.GetSerialsByRange(idx); chk.E(err) {
				return
			}
			sers = append(sers, s...)
		}
		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = d.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// sort by timestamp, so the first is the oldest, so we can collect
			// all of them until the delete event created_at.
			sort.Slice(
				idPkTss, func(i, j int) bool {
					return idPkTss[i].Ts > idPkTss[j].Ts
				},
			)
			for _, v := range idPkTss {
				if v.Ts < ev.CreatedAt {
					if err = d.DeleteEvent(
						context.Background(), v.Id[:],
					); chk.E(err) {
						continue
					}
				}
			}
		}
	}
	return
}
