//go:build !(js && wasm)

package bbolt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/bufpool"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/mode"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// SaveEvent saves an event to the database using the write batcher.
func (b *B) SaveEvent(c context.Context, ev *event.E) (replaced bool, err error) {
	if ev == nil {
		err = errors.New("nil event")
		return
	}

	// Reject ephemeral events (kinds 20000-29999)
	if ev.Kind >= 20000 && ev.Kind <= 29999 {
		err = errors.New("blocked: ephemeral events should not be stored")
		return
	}

	// Validate kind 3 (follow list) events have at least one p tag
	if ev.Kind == 3 {
		hasPTag := false
		if ev.Tags != nil {
			for _, tag := range *ev.Tags {
				if tag != nil && tag.Len() >= 2 {
					key := tag.Key()
					if len(key) == 1 && key[0] == 'p' {
						hasPTag = true
						break
					}
				}
			}
		}
		if !hasPTag {
			err = errors.New("blocked: kind 3 follow list events must have at least one p tag")
			return
		}
	}

	// Check if the event already exists
	var ser *types.Uint40
	if ser, err = b.GetSerialById(ev.ID); err == nil && ser != nil {
		err = errors.New("blocked: event already exists: " + hex.Enc(ev.ID[:]))
		return
	}

	// If the error is "id not found", we can proceed
	if err != nil && strings.Contains(err.Error(), "id not found") {
		err = nil
	} else if err != nil {
		return
	}

	// Check if the event has been deleted
	if !mode.IsOpen() {
		if err = b.CheckForDeleted(ev, nil); err != nil {
			err = fmt.Errorf("blocked: %s", err.Error())
			return
		}
	}

	// Check for replacement
	if kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind) {
		var werr error
		if replaced, _, werr = b.WouldReplaceEvent(ev); werr != nil {
			if errors.Is(werr, database.ErrOlderThanExisting) {
				if kind.IsReplaceable(ev.Kind) {
					err = errors.New("blocked: event is older than existing replaceable event")
				} else {
					err = errors.New("blocked: event is older than existing addressable event")
				}
				return
			}
			if errors.Is(werr, database.ErrMissingDTag) {
				err = database.ErrMissingDTag
				return
			}
			return
		}
	}

	// Get the next serial number
	serial := b.getNextEventSerial()

	// Generate all indexes using the shared function
	var rawIdxs [][]byte
	if rawIdxs, err = database.GetIndexesForEvent(ev, serial); chk.E(err) {
		return
	}

	// Convert raw indexes to BatchedWrites, stripping the 3-byte prefix
	// since we use separate buckets
	batch := &EventBatch{
		Serial:  serial,
		Indexes: make([]BatchedWrite, 0, len(rawIdxs)),
	}

	for _, idx := range rawIdxs {
		if len(idx) < 3 {
			continue
		}

		// Get bucket name from prefix
		bucketName := idx[:3]
		key := idx[3:] // Key without prefix

		batch.Indexes = append(batch.Indexes, BatchedWrite{
			BucketName: bucketName,
			Key:        key,
			Value:      nil, // Index entries have empty values
		})
	}

	// Serialize event in compact format
	resolver := &bboltSerialResolver{b: b}
	compactData, compactErr := database.MarshalCompactEvent(ev, resolver)
	if compactErr != nil {
		// Fall back to legacy format
		legacyBuf := bufpool.GetMedium()
		defer bufpool.PutMedium(legacyBuf)
		ev.MarshalBinary(legacyBuf)
		compactData = bufpool.CopyBytes(legacyBuf)
	}
	batch.EventData = compactData

	// Build event vertex for adjacency list
	var authorSerial uint64
	err = b.db.Update(func(tx *bolt.Tx) error {
		var e error
		authorSerial, e = b.getOrCreatePubkeySerial(tx, ev.Pubkey)
		return e
	})
	if chk.E(err) {
		return
	}

	eventVertex := &EventVertex{
		AuthorSerial: authorSerial,
		Kind:         uint16(ev.Kind),
		PTagSerials:  make([]uint64, 0),
		ETagSerials:  make([]uint64, 0),
	}

	// Collect edge keys for bloom filter
	edgeKeys := make([]EdgeKey, 0)

	// Add author edge to bloom filter
	edgeKeys = append(edgeKeys, EdgeKey{
		SrcSerial: serial,
		DstSerial: authorSerial,
		EdgeType:  EdgeTypeAuthor,
	})

	// Set up pubkey vertex update for author
	batch.PubkeyUpdate = &PubkeyVertexUpdate{
		PubkeySerial: authorSerial,
		AddAuthored:  serial,
	}

	// Process p-tags
	batch.MentionUpdates = make([]*PubkeyVertexUpdate, 0)
	pTags := ev.Tags.GetAll([]byte("p"))
	for _, pTag := range pTags {
		if pTag.Len() >= 2 {
			var ptagPubkey []byte
			if ptagPubkey, err = hex.Dec(string(pTag.ValueHex())); err == nil && len(ptagPubkey) == 32 {
				var ptagSerial uint64
				err = b.db.Update(func(tx *bolt.Tx) error {
					var e error
					ptagSerial, e = b.getOrCreatePubkeySerial(tx, ptagPubkey)
					return e
				})
				if chk.E(err) {
					continue
				}

				eventVertex.PTagSerials = append(eventVertex.PTagSerials, ptagSerial)

				// Add p-tag edge to bloom filter
				edgeKeys = append(edgeKeys, EdgeKey{
					SrcSerial: serial,
					DstSerial: ptagSerial,
					EdgeType:  EdgeTypePTag,
				})

				// Add mention update for this pubkey
				batch.MentionUpdates = append(batch.MentionUpdates, &PubkeyVertexUpdate{
					PubkeySerial: ptagSerial,
					AddMention:   serial,
				})
			}
		}
	}

	// Process e-tags
	eTags := ev.Tags.GetAll([]byte("e"))
	for _, eTag := range eTags {
		if eTag.Len() >= 2 {
			var targetEventID []byte
			if targetEventID, err = hex.Dec(string(eTag.ValueHex())); err != nil || len(targetEventID) != 32 {
				continue
			}

			// Look up the target event's serial
			var targetSerial *types.Uint40
			if targetSerial, err = b.GetSerialById(targetEventID); err != nil {
				err = nil
				continue
			}

			targetSer := targetSerial.Get()
			eventVertex.ETagSerials = append(eventVertex.ETagSerials, targetSer)

			// Add e-tag edge to bloom filter
			edgeKeys = append(edgeKeys, EdgeKey{
				SrcSerial: serial,
				DstSerial: targetSer,
				EdgeType:  EdgeTypeETag,
			})
		}
	}

	batch.EventVertex = eventVertex
	batch.EdgeKeys = edgeKeys

	// Store serial -> event ID mapping
	batch.Indexes = append(batch.Indexes, BatchedWrite{
		BucketName: bucketSei,
		Key:        makeSerialKey(serial),
		Value:      ev.ID[:],
	})

	// Add to batcher
	if err = b.batcher.Add(batch); chk.E(err) {
		return
	}

	// Process deletion events
	if ev.Kind == kind.Deletion.K {
		if err = b.ProcessDelete(ev, nil); chk.E(err) {
			b.Logger.Warningf("failed to process deletion for event %x: %v", ev.ID, err)
			err = nil
		}
	}

	return
}

// GetSerialsFromFilter returns serials matching a filter.
func (b *B) GetSerialsFromFilter(f *filter.F) (sers types.Uint40s, err error) {
	var idxs []database.Range
	if idxs, err = database.GetIndexesFromFilter(f); chk.E(err) {
		return
	}

	sers = make(types.Uint40s, 0, len(idxs)*100)
	for _, idx := range idxs {
		var s types.Uint40s
		if s, err = b.GetSerialsByRange(idx); chk.E(err) {
			continue
		}
		sers = append(sers, s...)
	}
	return
}

// WouldReplaceEvent checks if the event would replace existing events.
func (b *B) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
	if !(kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind)) {
		return false, nil, nil
	}

	var f *filter.F
	if kind.IsReplaceable(ev.Kind) {
		f = &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
		}
	} else {
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag == nil {
			return false, nil, database.ErrMissingDTag
		}
		f = &filter.F{
			Authors: tag.NewFromBytesSlice(ev.Pubkey),
			Kinds:   kind.NewS(kind.New(ev.Kind)),
			Tags: tag.NewS(
				tag.NewFromAny("d", dTag.Value()),
			),
		}
	}

	sers, err := b.GetSerialsFromFilter(f)
	if chk.E(err) {
		return false, nil, err
	}
	if len(sers) == 0 {
		return false, nil, nil
	}

	shouldReplace := true
	for _, s := range sers {
		oldEv, ferr := b.FetchEventBySerial(s)
		if chk.E(ferr) {
			continue
		}
		if ev.CreatedAt < oldEv.CreatedAt {
			shouldReplace = false
			break
		}
	}
	if shouldReplace {
		return true, nil, nil
	}
	return false, nil, database.ErrOlderThanExisting
}

// bboltSerialResolver implements database.SerialResolver for compact event encoding
type bboltSerialResolver struct {
	b *B
}

func (r *bboltSerialResolver) GetOrCreatePubkeySerial(pubkey []byte) (serial uint64, err error) {
	err = r.b.db.Update(func(tx *bolt.Tx) error {
		var e error
		serial, e = r.b.getOrCreatePubkeySerial(tx, pubkey)
		return e
	})
	return
}

func (r *bboltSerialResolver) GetPubkeyBySerial(serial uint64) (pubkey []byte, err error) {
	r.b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketSpk)
		if bucket == nil {
			err = errors.New("bbolt: spk bucket not found")
			return nil
		}
		val := bucket.Get(makeSerialKey(serial))
		if val != nil {
			pubkey = make([]byte, 32)
			copy(pubkey, val)
		} else {
			err = errors.New("bbolt: pubkey serial not found")
		}
		return nil
	})
	return
}

func (r *bboltSerialResolver) GetEventSerialById(eventID []byte) (serial uint64, found bool, err error) {
	ser, e := r.b.GetSerialById(eventID)
	if e != nil || ser == nil {
		return 0, false, nil
	}
	return ser.Get(), true, nil
}

func (r *bboltSerialResolver) GetEventIdBySerial(serial uint64) (eventID []byte, err error) {
	r.b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketSei)
		if bucket == nil {
			err = errors.New("bbolt: sei bucket not found")
			return nil
		}
		val := bucket.Get(makeSerialKey(serial))
		if val != nil {
			eventID = make([]byte, 32)
			copy(eventID, val)
		} else {
			err = errors.New("bbolt: event serial not found")
		}
		return nil
	})
	return
}
