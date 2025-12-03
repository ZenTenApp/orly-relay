//go:build !(js && wasm)

package database

import (
	"context"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"next.orly.dev/pkg/interfaces/store"
)

// QueryForSerials takes a filter and returns the serials of events that match,
// sorted in reverse chronological order.
func (d *D) QueryForSerials(c context.Context, f *filter.F) (
	sers types.Uint40s, err error,
) {
	var founds []*types.Uint40
	var idPkTs []*store.IdPkTs
	if f.Ids != nil && f.Ids.Len() > 0 {
		// Use batch lookup to minimize transactions when resolving IDs to serials
		var serialMap map[string]*types.Uint40
		if serialMap, err = d.GetSerialsByIds(f.Ids); chk.E(err) {
			return
		}
		for _, ser := range serialMap {
			founds = append(founds, ser)
		}
		var tmp []*store.IdPkTs
		if tmp, err = d.GetFullIdPubkeyBySerials(founds); chk.E(err) {
			return
		}
		idPkTs = append(idPkTs, tmp...)
	} else {
		if idPkTs, err = d.QueryForIds(c, f); chk.E(err) {
			return
		}
	}
	// extract the serials
	for _, idpk := range idPkTs {
		ser := new(types.Uint40)
		if err = ser.Set(idpk.Ser); chk.E(err) {
			continue
		}
		sers = append(sers, ser)
	}
	return
}
