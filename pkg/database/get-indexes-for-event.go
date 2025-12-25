package database

import (
	"bytes"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/bufpool"
	"next.orly.dev/pkg/database/indexes"
	. "next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

// appendIndexBytes marshals an index to a byte slice and appends it to the idxs slice.
// It reuses the provided buffer (resetting it first) to avoid allocations.
func appendIndexBytes(idxs *[][]byte, idx *indexes.T, buf *bytes.Buffer) (err error) {
	buf.Reset()
	// Marshal the index to the buffer
	if err = idx.MarshalWrite(buf); chk.E(err) {
		return
	}
	// Copy the buffer's bytes to a new byte slice and append
	*idxs = append(*idxs, bufpool.CopyBytes(buf))
	return
}

// GetIndexesForEvent creates all the indexes for an event.E instance as defined
// in keys.go. It returns a slice of byte slices that can be used to store the
// event in the database.
func GetIndexesForEvent(ev *event.E, serial uint64) (
	idxs [][]byte, err error,
) {
	// Get a reusable buffer for all index serializations
	buf := bufpool.GetSmall()
	defer bufpool.PutSmall(buf)

	defer func() {
		if chk.E(err) {
			idxs = nil
		}
	}()
	// Convert serial to Uint40
	ser := new(Uint40)
	if err = ser.Set(serial); chk.E(err) {
		return
	}
	// ID index
	idHash := new(IdHash)
	if err = idHash.FromId(ev.ID); chk.E(err) {
		return
	}
	idIndex := indexes.IdEnc(idHash, ser)
	if err = appendIndexBytes(&idxs, idIndex, buf); chk.E(err) {
		return
	}
	// FullIdPubkey index
	fullID := new(Id)
	if err = fullID.FromId(ev.ID); chk.E(err) {
		return
	}
	pubHash := new(PubHash)
	if err = pubHash.FromPubkey(ev.Pubkey); chk.E(err) {
		return
	}
	createdAt := new(Uint64)
	createdAt.Set(uint64(ev.CreatedAt))
	idPubkeyIndex := indexes.FullIdPubkeyEnc(
		ser, fullID, pubHash, createdAt,
	)
	if err = appendIndexBytes(&idxs, idPubkeyIndex, buf); chk.E(err) {
		return
	}
	// CreatedAt index
	createdAtIndex := indexes.CreatedAtEnc(createdAt, ser)
	if err = appendIndexBytes(&idxs, createdAtIndex, buf); chk.E(err) {
		return
	}
	// PubkeyCreatedAt index
	pubkeyIndex := indexes.PubkeyEnc(pubHash, createdAt, ser)
	if err = appendIndexBytes(&idxs, pubkeyIndex, buf); chk.E(err) {
		return
	}
	// Process tags for tag-related indexes
	if ev.Tags != nil && ev.Tags.Len() > 0 {
		for _, t := range *ev.Tags {
			// only index tags with a value field and the key is a single character
			if t.Len() >= 2 {
				// Get the key and value from the tag
				keyBytes := t.Key()
				// require single-letter key
				if len(keyBytes) != 1 {
					continue
				}
				// if the key is not a-zA-Z skip
				if (keyBytes[0] < 'a' || keyBytes[0] > 'z') &&
					(keyBytes[0] < 'A' || keyBytes[0] > 'Z') {
					continue
				}
				valueBytes := t.Value()
				// Create tag key and value
				key := new(Letter)
				key.Set(keyBytes[0])
				valueHash := new(Ident)
				valueHash.FromIdent(valueBytes)
				// TagPubkey index
				pubkeyTagIndex := indexes.TagPubkeyEnc(
					key, valueHash, pubHash, createdAt, ser,
				)
				if err = appendIndexBytes(
					&idxs, pubkeyTagIndex, buf,
				); chk.E(err) {
					return
				}
				// Tag index
				tagIndex := indexes.TagEnc(
					key, valueHash, createdAt, ser,
				)
				if err = appendIndexBytes(
					&idxs, tagIndex, buf,
				); chk.E(err) {
					return
				}
				// Kind-related tag indexes
				kind := new(Uint16)
				kind.Set(ev.Kind)
				// TagKind index
				kindTagIndex := indexes.TagKindEnc(
					key, valueHash, kind, createdAt, ser,
				)
				if err = appendIndexBytes(
					&idxs, kindTagIndex, buf,
				); chk.E(err) {
					return
				}
				// TagKindPubkey index
				kindPubkeyTagIndex := indexes.TagKindPubkeyEnc(
					key, valueHash, kind, pubHash, createdAt, ser,
				)
				if err = appendIndexBytes(
					&idxs, kindPubkeyTagIndex, buf,
				); chk.E(err) {
					return
				}
			}
		}
	}
	kind := new(Uint16)
	kind.Set(uint16(ev.Kind))
	// Kind index
	kindIndex := indexes.KindEnc(kind, createdAt, ser)
	if err = appendIndexBytes(&idxs, kindIndex, buf); chk.E(err) {
		return
	}
	// KindPubkey index
	// Using the correct parameters based on the function signature
	kindPubkeyIndex := indexes.KindPubkeyEnc(
		kind, pubHash, createdAt, ser,
	)
	if err = appendIndexBytes(&idxs, kindPubkeyIndex, buf); chk.E(err) {
		return
	}

	// Word token indexes (from content)
	if len(ev.Content) > 0 {
		for _, h := range TokenHashes(ev.Content) {
			w := new(Word)
			w.FromWord(h) // 8-byte truncated hash
			wIdx := indexes.WordEnc(w, ser)
			if err = appendIndexBytes(&idxs, wIdx, buf); chk.E(err) {
				return
			}
		}
	}
	// Extend full-text search to include all fields of all tags
	if ev.Tags != nil && ev.Tags.Len() > 0 {
		for _, t := range *ev.Tags {
			for _, field := range t.T { // include key and all values
				if len(field) == 0 {
					continue
				}
				for _, h := range TokenHashes(field) {
					w := new(Word)
					w.FromWord(h)
					wIdx := indexes.WordEnc(w, ser)
					if err = appendIndexBytes(&idxs, wIdx, buf); chk.E(err) {
						return
					}
				}
			}
		}
	}
	return
}
