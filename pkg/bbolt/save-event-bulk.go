//go:build !(js && wasm)

package bbolt

import (
	"errors"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/bufpool"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

// SaveEventForImport saves an event optimized for bulk import.
// It skips duplicate checking, deletion checking, and graph vertex creation
// to maximize import throughput. Use only for trusted data migration.
func (b *B) SaveEventForImport(ev *event.E) error {
	if ev == nil {
		return errors.New("nil event")
	}

	// Reject ephemeral events (kinds 20000-29999)
	if ev.Kind >= 20000 && ev.Kind <= 29999 {
		return nil // silently skip
	}

	// Get the next serial number
	serial := b.getNextEventSerial()

	// Generate all indexes using the shared function
	rawIdxs, err := database.GetIndexesForEvent(ev, serial)
	if chk.E(err) {
		return err
	}

	// Convert raw indexes to BatchedWrites, stripping the 3-byte prefix
	batch := &EventBatch{
		Serial:  serial,
		Indexes: make([]BatchedWrite, 0, len(rawIdxs)+1),
	}

	for _, idx := range rawIdxs {
		if len(idx) < 3 {
			continue
		}
		bucketName := idx[:3]
		key := idx[3:]
		batch.Indexes = append(batch.Indexes, BatchedWrite{
			BucketName: bucketName,
			Key:        key,
			Value:      nil,
		})
	}

	// Serialize event in compact format (without graph references for import)
	resolver := &nullSerialResolver{}
	compactData, compactErr := database.MarshalCompactEvent(ev, resolver)
	if compactErr != nil {
		// Fall back to legacy format
		legacyBuf := bufpool.GetMedium()
		defer bufpool.PutMedium(legacyBuf)
		ev.MarshalBinary(legacyBuf)
		compactData = bufpool.CopyBytes(legacyBuf)
	}
	batch.EventData = compactData

	// Store serial -> event ID mapping
	batch.Indexes = append(batch.Indexes, BatchedWrite{
		BucketName: bucketSei,
		Key:        makeSerialKey(serial),
		Value:      ev.ID[:],
	})

	// Add to batcher (no graph vertex, no pubkey lookups)
	return b.batcher.Add(batch)
}

// nullSerialResolver returns 0 for all lookups, used for fast import
// where we don't need pubkey/event serial references in compact format
type nullSerialResolver struct{}

func (r *nullSerialResolver) GetOrCreatePubkeySerial(pubkey []byte) (uint64, error) {
	return 0, nil
}

func (r *nullSerialResolver) GetPubkeyBySerial(serial uint64) ([]byte, error) {
	return nil, nil
}

func (r *nullSerialResolver) GetEventSerialById(eventID []byte) (uint64, bool, error) {
	return 0, false, nil
}

func (r *nullSerialResolver) GetEventIdBySerial(serial uint64) ([]byte, error) {
	return nil, nil
}
