package database

import (
	"context"

	"git.mleku.dev/mleku/nostr/encoders/filter"
)

// CountEvents mirrors the initial selection logic of QueryEvents but stops
// once we have identified candidate event serials (id/pk/ts). It returns the
// count of those serials. The `approx` flag is always false as requested.
func (d *D) CountEvents(c context.Context, f *filter.F) (
	count int, approx bool, err error,
) {
	approx = false
	if f == nil {
		return 0, false, nil
	}

	// If explicit Ids are provided, count how many of them resolve to serials.
	if f.Ids != nil && f.Ids.Len() > 0 {
		var serials map[string]interface{}
		// Use type inference without importing extra packages by discarding the
		// concrete value type via a two-step assignment.
		if tmp, idErr := d.GetSerialsByIds(f.Ids); idErr != nil {
			return 0, false, idErr
		} else {
			// Reassign to a map with empty interface values to avoid referencing
			// the concrete Uint40 type here.
			serials = make(map[string]interface{}, len(tmp))
			for k := range tmp {
				serials[k] = struct{}{}
			}
		}
		return len(serials), false, nil
	}

	// Otherwise, query for candidate Id/Pubkey/Timestamp triplets and count them.
	if idPkTs, qErr := d.QueryForIds(c, f); qErr != nil {
		return 0, false, qErr
	} else {
		return len(idPkTs), false, nil
	}
}
