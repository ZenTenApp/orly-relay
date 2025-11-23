package database

import (
	"bytes"
	"context"
	"sort"
	"strconv"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	hexenc "git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/ints"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/interfaces/store"
	"next.orly.dev/pkg/utils"
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
		eventId := eTag.Value()
		if len(eventId) != 64 { // hex encoded event ID
			continue
		}
		// Decode hex event ID
		var eid []byte
		if eid, err = hexenc.DecAppend(nil, eventId); chk.E(err) {
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
					if err = d.DeleteEvent(context.Background(), v.Id); chk.E(err) {
						log.W.F("failed to delete event %x via a-tag: %v", v.Id, err)
						continue
					}
					log.D.F("deleted event %x via a-tag deletion", v.Id)
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
						context.Background(), v.Id,
					); chk.E(err) {
						continue
					}
				}
			}
		}
	}
	return
}
