package database

import (
	"bytes"
	"math"
	"sort"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	types2 "next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/tag"
)

type Range struct {
	Start, End []byte
}

// IsHexString checks if the byte slice contains only hex characters
func IsHexString(data []byte) (isHex bool) {
	if len(data)%2 != 0 {
		return false
	}
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}
	return true
}

// CreateIdHashFromData creates an IdHash from data that could be hex or binary
func CreateIdHashFromData(data []byte) (i *types2.IdHash, err error) {
	i = new(types2.IdHash)

	// If data looks like hex string and has the right length for hex-encoded
	// sha256
	if len(data) == 64 {
		if err = i.FromIdHex(string(data)); chk.E(err) {
			err = nil
		} else {
			return
		}
	}
	// Assume it's binary data
	if err = i.FromId(data); chk.E(err) {
		return
	}
	return
}

// CreatePubHashFromData creates a PubHash from data that could be hex or binary
func CreatePubHashFromData(data []byte) (p *types2.PubHash, err error) {
	p = new(types2.PubHash)

	// If data looks like hex string and has the right length for hex-encoded
	// pubkey
	if len(data) == 64 {
		if err = p.FromPubkeyHex(string(data)); chk.E(err) {
			err = nil
		} else {
			return
		}
	} else {
		// Assume it's binary data
		if err = p.FromPubkey(data); chk.E(err) {
			return
		}
	}
	return
}

// GetIndexesFromFilter returns encoded indexes based on the given filter.
//
// An error is returned if any input values are invalid during encoding.
//
// The indexes are designed so that only one table needs to be iterated, being a
// complete set of combinations of all fields in the event, thus there is no
// need to decode events until they are to be delivered.
func GetIndexesFromFilter(f *filter.F) (idxs []Range, err error) {
	// ID eid
	//
	// If there is any Ids in the filter, none of the other fields matter. It
	// should be an error, but convention just ignores it.
	if f.Ids.Len() > 0 {
		for _, id := range f.Ids.T {
			if err = func() (err error) {
				var i *types2.IdHash
				if i, err = CreateIdHashFromData(id); chk.E(err) {
					return
				}
				buf := new(bytes.Buffer)
				// Create an index prefix without the serial number
				idx := indexes.IdEnc(i, nil)
				if err = idx.MarshalWrite(buf); chk.E(err) {
					return
				}
				b := buf.Bytes()
				// For ID filters, both start and end indexes are the same (exact match)
				r := Range{b, b}
				idxs = append(idxs, r)
				return
			}(); chk.E(err) {
				return
			}
		}
		return
	}

	// Word search: if Search field is present, generate word index ranges
	if len(f.Search) > 0 {
		for _, h := range TokenHashes(f.Search) {
			w := new(types2.Word)
			w.FromWord(h)
			buf := new(bytes.Buffer)
			idx := indexes.WordEnc(w, nil)
			if err = idx.MarshalWrite(buf); chk.E(err) {
				return
			}
			b := buf.Bytes()
			end := make([]byte, len(b))
			copy(end, b)
			for i := 0; i < 5; i++ { // match any serial
				end = append(end, 0xff)
			}
			idxs = append(idxs, Range{b, end})
		}
		return
	}

	caStart := new(types2.Uint64)
	caEnd := new(types2.Uint64)

	// Set the start of range (Since or default to zero)
	if f.Since != nil && f.Since.V != 0 {
		caStart.Set(uint64(f.Since.V))
	} else {
		caStart.Set(uint64(0))
	}

	// Set the end of range (Until or default to math.MaxInt64)
	if f.Until != nil && f.Until.V != 0 {
		caEnd.Set(uint64(f.Until.V))
	} else {
		caEnd.Set(uint64(math.MaxInt64))
	}

	// Filter out special tags that shouldn't affect index selection
	var filteredTags *tag.S
	var pTags *tag.S // Separate collection for p-tags that can use graph index
	if f.Tags != nil && f.Tags.Len() > 0 {
		filteredTags = tag.NewSWithCap(f.Tags.Len())
		pTags = tag.NewS()
		for _, t := range *f.Tags {
			// Skip the special "show_all_versions" tag
			if bytes.Equal(t.Key(), []byte("show_all_versions")) {
				continue
			}
			// Collect p-tags separately for potential graph optimization
			keyBytes := t.Key()
			if (len(keyBytes) == 1 && keyBytes[0] == 'p') ||
			   (len(keyBytes) == 2 && keyBytes[0] == '#' && keyBytes[1] == 'p') {
				pTags.Append(t)
			}
			filteredTags.Append(t)
		}
		// sort the filtered tags so they are in iteration order (reverse)
		if filteredTags.Len() > 0 {
			sort.Sort(filteredTags)
		}
	}

	// Note: P-tag graph optimization is handled in query-for-ptag-graph.go
	// when appropriate (requires database context for serial lookup)

	// TagKindPubkey tkp
	if f.Kinds != nil && f.Kinds.Len() > 0 && f.Authors != nil && f.Authors.Len() > 0 && filteredTags != nil && filteredTags.Len() > 0 {
		for _, k := range f.Kinds.ToUint16() {
			for _, author := range f.Authors.T {
				for _, t := range *filteredTags {
					// accept single-letter keys like "e" or filter-style keys like "#e"
					if t.Len() >= 2 && (len(t.Key()) == 1 || (len(t.Key()) == 2 && t.Key()[0] == '#')) {
						kind := new(types2.Uint16)
						kind.Set(k)
						var p *types2.PubHash
						if p, err = CreatePubHashFromData(author); chk.E(err) {
							return
						}
						keyBytes := t.Key()
						key := new(types2.Letter)
						// If the tag key starts with '#', use the second character as the key
						if len(keyBytes) == 2 && keyBytes[0] == '#' {
							key.Set(keyBytes[1])
						} else {
							key.Set(keyBytes[0])
						}
						for _, valueBytes := range t.T[1:] {
							valueHash := new(types2.Ident)
							valueHash.FromIdent(valueBytes)
							start, end := new(bytes.Buffer), new(bytes.Buffer)
							idxS := indexes.TagKindPubkeyEnc(
								key, valueHash, kind, p, caStart, nil,
							)
							if err = idxS.MarshalWrite(start); chk.E(err) {
								return
							}
							idxE := indexes.TagKindPubkeyEnc(
								key, valueHash, kind, p, caEnd, nil,
							)
							if err = idxE.MarshalWrite(end); chk.E(err) {
								return
							}
							idxs = append(
								idxs, Range{
									start.Bytes(), end.Bytes(),
								},
							)
						}
					}
				}
			}
		}
		return
	}

	// TagKind tkc
	if f.Kinds != nil && f.Kinds.Len() > 0 && filteredTags != nil && filteredTags.Len() > 0 {
		for _, k := range f.Kinds.ToUint16() {
			for _, t := range *filteredTags {
				if t.Len() >= 2 && (len(t.Key()) == 1 || (len(t.Key()) == 2 && t.Key()[0] == '#')) {
					kind := new(types2.Uint16)
					kind.Set(k)
					keyBytes := t.Key()
					key := new(types2.Letter)
					// If the tag key starts with '#', use the second character as the key
					if len(keyBytes) == 2 && keyBytes[0] == '#' {
						key.Set(keyBytes[1])
					} else {
						key.Set(keyBytes[0])
					}
					for _, valueBytes := range t.T[1:] {
						valueHash := new(types2.Ident)
						valueHash.FromIdent(valueBytes)
						start, end := new(bytes.Buffer), new(bytes.Buffer)
						idxS := indexes.TagKindEnc(
							key, valueHash, kind, caStart, nil,
						)
						if err = idxS.MarshalWrite(start); chk.E(err) {
							return
						}
						idxE := indexes.TagKindEnc(
							key, valueHash, kind, caEnd, nil,
						)
						if err = idxE.MarshalWrite(end); chk.E(err) {
							return
						}
						idxs = append(
							idxs, Range{
								start.Bytes(), end.Bytes(),
							},
						)
					}
				}
			}
		}
		return
	}

	// TagPubkey tpc
	if f.Authors != nil && f.Authors.Len() > 0 && filteredTags != nil && filteredTags.Len() > 0 {
		for _, author := range f.Authors.T {
			for _, t := range *filteredTags {
				if t.Len() >= 2 && (len(t.Key()) == 1 || (len(t.Key()) == 2 && t.Key()[0] == '#')) {
					var p *types2.PubHash
					log.I.S(author)
					if p, err = CreatePubHashFromData(author); chk.E(err) {
						return
					}
					keyBytes := t.Key()
					key := new(types2.Letter)
					// If the tag key starts with '#', use the second character as the key
					if len(keyBytes) == 2 && keyBytes[0] == '#' {
						key.Set(keyBytes[1])
					} else {
						key.Set(keyBytes[0])
					}
					for _, valueBytes := range t.T[1:] {
						valueHash := new(types2.Ident)
						valueHash.FromIdent(valueBytes)
						start, end := new(bytes.Buffer), new(bytes.Buffer)
						idxS := indexes.TagPubkeyEnc(
							key, valueHash, p, caStart, nil,
						)
						if err = idxS.MarshalWrite(start); chk.E(err) {
							return
						}
						idxE := indexes.TagPubkeyEnc(
							key, valueHash, p, caEnd, nil,
						)
						if err = idxE.MarshalWrite(end); chk.E(err) {
							return
						}
						idxs = append(
							idxs, Range{start.Bytes(), end.Bytes()},
						)
					}
				}
			}
		}
		return
	}

	// Tag tc-
	if filteredTags != nil && filteredTags.Len() > 0 && (f.Authors == nil || f.Authors.Len() == 0) && (f.Kinds == nil || f.Kinds.Len() == 0) {
		for _, t := range *filteredTags {
			if t.Len() >= 2 && (len(t.Key()) == 1 || (len(t.Key()) == 2 && t.Key()[0] == '#')) {
				keyBytes := t.Key()
				key := new(types2.Letter)
				// If the tag key starts with '#', use the second character as the key
				if len(keyBytes) == 2 && keyBytes[0] == '#' {
					key.Set(keyBytes[1])
				} else {
					key.Set(keyBytes[0])
				}
				for _, valueBytes := range t.T[1:] {
					valueHash := new(types2.Ident)
					valueHash.FromIdent(valueBytes)
					start, end := new(bytes.Buffer), new(bytes.Buffer)
					idxS := indexes.TagEnc(key, valueHash, caStart, nil)
					if err = idxS.MarshalWrite(start); chk.E(err) {
						return
					}
					idxE := indexes.TagEnc(key, valueHash, caEnd, nil)
					if err = idxE.MarshalWrite(end); chk.E(err) {
						return
					}
					idxs = append(
						idxs, Range{start.Bytes(), end.Bytes()},
					)
				}
			}
		}
		return
	}

	// KindPubkey kpc
	if f.Kinds != nil && f.Kinds.Len() > 0 && f.Authors != nil && f.Authors.Len() > 0 {
		for _, k := range f.Kinds.ToUint16() {
			for _, author := range f.Authors.T {
				kind := new(types2.Uint16)
				kind.Set(k)
				var p *types2.PubHash
				if p, err = CreatePubHashFromData(author); chk.E(err) {
					return
				}
				start, end := new(bytes.Buffer), new(bytes.Buffer)
				idxS := indexes.KindPubkeyEnc(kind, p, caStart, nil)
				if err = idxS.MarshalWrite(start); chk.E(err) {
					return
				}
				idxE := indexes.KindPubkeyEnc(kind, p, caEnd, nil)
				if err = idxE.MarshalWrite(end); chk.E(err) {
					return
				}
				idxs = append(
					idxs, Range{start.Bytes(), end.Bytes()},
				)
			}
		}
		return
	}

	// Kind kc-
	if f.Kinds != nil && f.Kinds.Len() > 0 && (f.Authors == nil || f.Authors.Len() == 0) && (filteredTags == nil || filteredTags.Len() == 0) {
		for _, k := range f.Kinds.ToUint16() {
			kind := new(types2.Uint16)
			kind.Set(k)
			start, end := new(bytes.Buffer), new(bytes.Buffer)
			idxS := indexes.KindEnc(kind, caStart, nil)
			if err = idxS.MarshalWrite(start); chk.E(err) {
				return
			}
			idxE := indexes.KindEnc(kind, caEnd, nil)
			if err = idxE.MarshalWrite(end); chk.E(err) {
				return
			}
			idxs = append(
				idxs, Range{start.Bytes(), end.Bytes()},
			)
		}
		return
	}

	// Pubkey pc-
	if f.Authors != nil && f.Authors.Len() > 0 {
		for _, author := range f.Authors.T {
			var p *types2.PubHash
			if p, err = CreatePubHashFromData(author); chk.E(err) {
				return
			}
			start, end := new(bytes.Buffer), new(bytes.Buffer)
			idxS := indexes.PubkeyEnc(p, caStart, nil)
			if err = idxS.MarshalWrite(start); chk.E(err) {
				return
			}
			idxE := indexes.PubkeyEnc(p, caEnd, nil)
			if err = idxE.MarshalWrite(end); chk.E(err) {
				return
			}
			idxs = append(
				idxs, Range{start.Bytes(), end.Bytes()},
			)
		}
		return
	}

	// CreatedAt c--
	start, end := new(bytes.Buffer), new(bytes.Buffer)
	idxS := indexes.CreatedAtEnc(caStart, nil)
	if err = idxS.MarshalWrite(start); chk.E(err) {
		return
	}
	idxE := indexes.CreatedAtEnc(caEnd, nil)
	if err = idxE.MarshalWrite(end); chk.E(err) {
		return
	}
	idxs = append(
		idxs, Range{start.Bytes(), end.Bytes()},
	)
	return
}
