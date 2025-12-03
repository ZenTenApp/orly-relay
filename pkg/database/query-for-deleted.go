//go:build !(js && wasm)

package database

import (
	"fmt"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/tag/atag"
	"next.orly.dev/pkg/interfaces/store"
)

// CheckForDeleted checks if the event is deleted, and returns an error with
// prefix "blocked:" if it is. This function also allows designating admin
// pubkeys that also may delete the event, normally only the author is allowed
// to delete an event.
func (d *D) CheckForDeleted(ev *event.E, admins [][]byte) (err error) {
	// log.T.F("CheckForDeleted: checking event %x", ev.ID)
	keys := append([][]byte{ev.Pubkey}, admins...)
	authors := tag.NewFromBytesSlice(keys...)
	// if the event is addressable, check for a deletion event with the same
	// kind/pubkey/dtag
	if kind.IsParameterizedReplaceable(ev.Kind) {
		var idxs []Range
		// construct a tag
		t := ev.Tags.GetFirst([]byte("d"))
		a := atag.T{
			Kind:   kind.New(ev.Kind),
			Pubkey: ev.Pubkey,
			DTag:   t.Value(),
		}
		at := a.Marshal(nil)
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: authors,
				Kinds:   kind.NewS(kind.Deletion),
				Tags:    tag.NewS(tag.NewFromAny("#a", at)),
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
			// there can be multiple of these because the author/kind/tag is a
			// stable value but refers to any event from the author, of the
			// kind, with the identifier. so we need to fetch the full ID index
			// to get the timestamp and ensure that the event post-dates it.
			// otherwise, it should be rejected.
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = d.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// find the newest deletion timestamp without sorting to reduce cost
			maxTs := idPkTss[0].Ts
			for i := 1; i < len(idPkTss); i++ {
				if idPkTss[i].Ts > maxTs {
					maxTs = idPkTss[i].Ts
				}
			}
			if ev.CreatedAt < maxTs {
				err = errorf.E(
					"blocked: %0x was deleted by address %s because it is older than the delete: event: %d delete: %d",
					ev.ID, at, ev.CreatedAt, maxTs,
				)
				return
			}
			return
		}
		return
	}
	// if the event is replaceable, check if there is a deletion event newer
	// than the event, it must specify the same kind/pubkey. this type of delete
	// only has the k tag to specify the kind, it can be what an author would
	// use, as the author is part of the replaceable event specification.
	if kind.IsReplaceable(ev.Kind) {
		var idxs []Range
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: tag.NewFromBytesSlice(ev.Pubkey),
				Kinds:   kind.NewS(kind.Deletion),
				Tags: tag.NewS(
					tag.NewFromAny("#k", fmt.Sprint(ev.Kind)),
				),
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
			// find the newest deletion without sorting to reduce cost
			maxTs := idPkTss[0].Ts
			maxId := idPkTss[0].Id
			for i := 1; i < len(idPkTss); i++ {
				if idPkTss[i].Ts > maxTs {
					maxTs = idPkTss[i].Ts
					maxId = idPkTss[i].Id
				}
			}
			if ev.CreatedAt < maxTs {
				err = fmt.Errorf(
					"blocked: %0x was deleted: the event is older than the delete event %0x: event: %d delete: %d",
					ev.ID, maxId, ev.CreatedAt, maxTs,
				)
				return
			}
		}
		// this type of delete can also use an a tag to specify kind and
		// author, which would be required for admin deletes
		idxs = nil
		// construct a tag
		a := atag.T{
			Kind:   kind.New(ev.Kind),
			Pubkey: ev.Pubkey,
		}
		at := a.Marshal(nil)
		if idxs, err = GetIndexesFromFilter(
			&filter.F{
				Authors: authors,
				Kinds:   kind.NewS(kind.Deletion),
				Tags:    tag.NewS(tag.NewFromAny("#a", at)),
			},
		); chk.E(err) {
			return
		}
		sers = nil
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
			// find the newest deletion without sorting to reduce cost
			maxTs := idPkTss[0].Ts
			// maxId := idPkTss[0].Id
			for i := 1; i < len(idPkTss); i++ {
				if idPkTss[i].Ts > maxTs {
					maxTs = idPkTss[i].Ts
					// maxId = idPkTss[i].Id
				}
			}
			if ev.CreatedAt < maxTs {
				err = errorf.E(
					"blocked: %0x was deleted by address %s because it is older than the delete: event: %d delete: %d",
					ev.ID, at, ev.CreatedAt, maxTs,
				)
				return
			}
			return
		}
		return
	}
	// otherwise we check for a delete by event id
	// log.T.F("CheckForDeleted: checking for e-tag deletion of event %x", ev.ID)
	// log.T.F("CheckForDeleted: authors filter: %v", authors)
	// log.T.F("CheckForDeleted: looking for tag e with value: %s", hex.Enc(ev.ID))
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(
		&filter.F{
			Authors: authors,
			Kinds:   kind.NewS(kind.Deletion),
			Tags: tag.NewS(
				tag.NewFromAny("e", hex.Enc(ev.ID)),
			),
		},
	); chk.E(err) {
		return
	}
	// log.T.F("CheckForDeleted: found %d indexes", len(idxs))
	var sers types.Uint40s
	for _, idx := range idxs {
		// log.T.F("CheckForDeleted: checking index %d: %v", i, idx)
		var s types.Uint40s
		if s, err = d.GetSerialsByRange(idx); chk.E(err) {
			return
		}
		// log.T.F("CheckForDeleted: index %d returned %d serials", i, len(s))
		if len(s) > 0 {
			// Any e-tag deletion found means the exact event was deleted and cannot be resubmitted
			// log.T.F("CheckForDeleted: found e-tag deletion for event %x", ev.ID)
			err = errorf.E("blocked: %0x has been deleted", ev.ID)
			return
		}
	}
	if len(sers) > 0 {
		// Any e-tag deletion found means the exact event was deleted and cannot be resubmitted
		err = errorf.E("blocked: %0x has been deleted", ev.ID)
		return
	}

	return
}
