//go:build !(js && wasm)

package database

import (
	"context"
	"errors"
	"sort"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/interfaces/store"
)

// QueryForIds retrieves a list of IdPkTs based on the provided filter.
// It supports filtering by ranges and tags but disallows filtering by Ids.
// Results are sorted by timestamp in reverse chronological order by default.
// When a search query is present, results are ranked by a 50/50 blend of
// match count (how many distinct search terms matched) and recency.
// Returns an error if the filter contains Ids or if any operation fails.
func (d *D) QueryForIds(c context.Context, f *filter.F) (
	idPkTs []*store.IdPkTs, err error,
) {
	if f.Ids != nil && f.Ids.Len() > 0 {
		// if there is Ids in the query, this is an error for this query
		err = errors.New("query for Ids is invalid for a filter with Ids")
		return
	}
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(f); chk.E(err) {
		return
	}
	var results []*store.IdPkTs
	var founds []*types.Uint40
	// Pre-allocate results slice with estimated capacity to reduce reallocations
	results = make([]*store.IdPkTs, 0, len(idxs)*100) // Estimate 100 results per index
	// When searching, we want to count how many index ranges (search terms)
	// matched each note. We'll track counts by serial.
	counts := make(map[uint64]int)
	for _, idx := range idxs {
		if founds, err = d.GetSerialsByRange(idx); chk.E(err) {
			return
		}
		var tmp []*store.IdPkTs
		if tmp, err = d.GetFullIdPubkeyBySerials(founds); chk.E(err) {
			return
		}
		// If this query is driven by Search terms, increment count per serial
		if len(f.Search) > 0 {
			for _, v := range tmp {
				counts[v.Ser]++
			}
		}
		results = append(results, tmp...)
	}
	// deduplicate in case this somehow happened (such as two or more
	// from one tag matched, only need it once)
	seen := make(map[uint64]struct{}, len(results))
	idPkTs = make([]*store.IdPkTs, 0, len(results))
	for _, idpk := range results {
		if _, ok := seen[idpk.Ser]; !ok {
			seen[idpk.Ser] = struct{}{}
			idPkTs = append(idPkTs, idpk)
		}
	}
	
	// If search is combined with Authors/Kinds/Tags, require events to match ALL of those present fields in addition to the word match.
	if len(f.Search) > 0 && ((f.Authors != nil && f.Authors.Len() > 0) || (f.Kinds != nil && f.Kinds.Len() > 0) || (f.Tags != nil && f.Tags.Len() > 0)) {
		// Build serial list for fetching full events
		serials := make([]*types.Uint40, 0, len(idPkTs))
		for _, v := range idPkTs {
			s := new(types.Uint40)
			s.Set(v.Ser)
			serials = append(serials, s)
		}
		var evs map[uint64]*event.E
		if evs, err = d.FetchEventsBySerials(serials); chk.E(err) {
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
				// Require the event to satisfy all tag filters as in MatchesIgnoringTimestampConstraints
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
	
	if len(f.Search) == 0 {
		// No search query: sort by timestamp in reverse chronological order
		sort.Slice(
			idPkTs, func(i, j int) bool {
				return idPkTs[i].Ts > idPkTs[j].Ts
			},
		)
	} else {
		// Search query present: blend match count relevance with recency (50/50)
		// Normalize both match count and timestamp to [0,1] and compute score.
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
		// Precompute denominator to avoid div-by-zero
		tsSpan := maxTs - minTs
		if tsSpan <= 0 {
			tsSpan = 1
		}
		if maxCount <= 0 {
			maxCount = 1
		}
		sort.Slice(
			idPkTs, func(i, j int) bool {
				ci := float64(counts[idPkTs[i].Ser]) / float64(maxCount)
				cj := float64(counts[idPkTs[j].Ser]) / float64(maxCount)
				ai := float64(idPkTs[i].Ts-minTs) / float64(tsSpan)
				aj := float64(idPkTs[j].Ts-minTs) / float64(tsSpan)
				si := 0.5*ci + 0.5*ai
				sj := 0.5*cj + 0.5*aj
				if si == sj {
					// tie-break by recency
					return idPkTs[i].Ts > idPkTs[j].Ts
				}
				return si > sj
			},
		)
	}

	if f.Limit != nil && len(idPkTs) > int(*f.Limit) {
		idPkTs = idPkTs[:*f.Limit]
	}
	return
}
