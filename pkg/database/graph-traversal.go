//go:build !(js && wasm)

package database

import (
	"bytes"
	"errors"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// Graph traversal errors
var (
	ErrPubkeyNotFound = errors.New("pubkey not found in database")
	ErrEventNotFound  = errors.New("event not found in database")
)

// GetPTagsFromEventSerial extracts p-tag pubkey serials from an event by its serial.
// This is a pure index-based operation - no event decoding required.
// It scans the epg (event-pubkey-graph) index for p-tag edges.
func (d *D) GetPTagsFromEventSerial(eventSerial *types.Uint40) ([]*types.Uint40, error) {
	var pubkeySerials []*types.Uint40

	// Build prefix: epg|event_serial
	prefix := new(bytes.Buffer)
	prefix.Write([]byte(indexes.EventPubkeyGraphPrefix))
	if err := eventSerial.MarshalWrite(prefix); chk.E(err) {
		return nil, err
	}
	searchPrefix := prefix.Bytes()

	err := d.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = searchPrefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(searchPrefix); it.ValidForPrefix(searchPrefix); it.Next() {
			key := it.Item().KeyCopy(nil)

			// Decode key: epg(3)|event_serial(5)|pubkey_serial(5)|kind(2)|direction(1)
			if len(key) != 16 {
				continue
			}

			// Extract direction to filter for p-tags only
			direction := key[15]
			if direction != types.EdgeDirectionPTagOut {
				continue // Skip author edges, only want p-tag edges
			}

			// Extract pubkey serial (bytes 8-12)
			pubkeySerial := new(types.Uint40)
			serialReader := bytes.NewReader(key[8:13])
			if err := pubkeySerial.UnmarshalRead(serialReader); chk.E(err) {
				continue
			}

			pubkeySerials = append(pubkeySerials, pubkeySerial)
		}
		return nil
	})

	return pubkeySerials, err
}

// GetETagsFromEventSerial extracts e-tag event serials from an event by its serial.
// This is a pure index-based operation - no event decoding required.
// It scans the eeg (event-event-graph) index for outbound e-tag edges.
func (d *D) GetETagsFromEventSerial(eventSerial *types.Uint40) ([]*types.Uint40, error) {
	var targetSerials []*types.Uint40

	// Build prefix: eeg|source_event_serial
	prefix := new(bytes.Buffer)
	prefix.Write([]byte(indexes.EventEventGraphPrefix))
	if err := eventSerial.MarshalWrite(prefix); chk.E(err) {
		return nil, err
	}
	searchPrefix := prefix.Bytes()

	err := d.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = searchPrefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(searchPrefix); it.ValidForPrefix(searchPrefix); it.Next() {
			key := it.Item().KeyCopy(nil)

			// Decode key: eeg(3)|source_serial(5)|target_serial(5)|kind(2)|direction(1)
			if len(key) != 16 {
				continue
			}

			// Extract target serial (bytes 8-12)
			targetSerial := new(types.Uint40)
			serialReader := bytes.NewReader(key[8:13])
			if err := targetSerial.UnmarshalRead(serialReader); chk.E(err) {
				continue
			}

			targetSerials = append(targetSerials, targetSerial)
		}
		return nil
	})

	return targetSerials, err
}

// GetReferencingEvents finds all events that reference a target event via e-tags.
// Optionally filters by event kinds. Uses the gee (reverse e-tag graph) index.
func (d *D) GetReferencingEvents(targetSerial *types.Uint40, kinds []uint16) ([]*types.Uint40, error) {
	var sourceSerials []*types.Uint40

	if len(kinds) == 0 {
		// No kind filter - scan all kinds
		prefix := new(bytes.Buffer)
		prefix.Write([]byte(indexes.GraphEventEventPrefix))
		if err := targetSerial.MarshalWrite(prefix); chk.E(err) {
			return nil, err
		}
		searchPrefix := prefix.Bytes()

		err := d.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = searchPrefix

			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(searchPrefix); it.ValidForPrefix(searchPrefix); it.Next() {
				key := it.Item().KeyCopy(nil)

				// Decode key: gee(3)|target_serial(5)|kind(2)|direction(1)|source_serial(5)
				if len(key) != 16 {
					continue
				}

				// Extract source serial (bytes 11-15)
				sourceSerial := new(types.Uint40)
				serialReader := bytes.NewReader(key[11:16])
				if err := sourceSerial.UnmarshalRead(serialReader); chk.E(err) {
					continue
				}

				sourceSerials = append(sourceSerials, sourceSerial)
			}
			return nil
		})
		return sourceSerials, err
	}

	// With kind filter - scan each kind's prefix
	for _, k := range kinds {
		kind := new(types.Uint16)
		kind.Set(k)

		direction := new(types.Letter)
		direction.Set(types.EdgeDirectionETagIn)

		prefix := new(bytes.Buffer)
		if err := indexes.GraphEventEventEnc(targetSerial, kind, direction, nil).MarshalWrite(prefix); chk.E(err) {
			return nil, err
		}
		searchPrefix := prefix.Bytes()

		err := d.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = searchPrefix

			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(searchPrefix); it.ValidForPrefix(searchPrefix); it.Next() {
				key := it.Item().KeyCopy(nil)

				// Extract source serial (last 5 bytes)
				if len(key) < 5 {
					continue
				}
				sourceSerial := new(types.Uint40)
				serialReader := bytes.NewReader(key[len(key)-5:])
				if err := sourceSerial.UnmarshalRead(serialReader); chk.E(err) {
					continue
				}

				sourceSerials = append(sourceSerials, sourceSerial)
			}
			return nil
		})
		if chk.E(err) {
			return nil, err
		}
	}

	return sourceSerials, nil
}

// FindEventByAuthorAndKind finds the most recent event of a specific kind by an author.
// This is used to find kind-3 contact lists for follow graph traversal.
// Returns nil, nil if no matching event is found.
func (d *D) FindEventByAuthorAndKind(authorSerial *types.Uint40, kind uint16) (*types.Uint40, error) {
	var eventSerial *types.Uint40

	// First, get the full pubkey from the serial
	pubkey, err := d.GetPubkeyBySerial(authorSerial)
	if err != nil {
		return nil, err
	}

	// Build prefix for kind-pubkey index: kpc|kind|pubkey_hash
	pubHash := new(types.PubHash)
	if err := pubHash.FromPubkey(pubkey); chk.E(err) {
		return nil, err
	}

	kindType := new(types.Uint16)
	kindType.Set(kind)

	prefix := new(bytes.Buffer)
	prefix.Write([]byte(indexes.KindPubkeyPrefix))
	if err := kindType.MarshalWrite(prefix); chk.E(err) {
		return nil, err
	}
	if err := pubHash.MarshalWrite(prefix); chk.E(err) {
		return nil, err
	}
	searchPrefix := prefix.Bytes()

	err = d.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = searchPrefix
		opts.Reverse = true // Most recent first (highest created_at)

		it := txn.NewIterator(opts)
		defer it.Close()

		// Seek to end of prefix range for reverse iteration
		seekKey := make([]byte, len(searchPrefix)+8+5) // prefix + max timestamp + max serial
		copy(seekKey, searchPrefix)
		for i := len(searchPrefix); i < len(seekKey); i++ {
			seekKey[i] = 0xFF
		}

		it.Seek(seekKey)
		if !it.ValidForPrefix(searchPrefix) {
			// Try going to the first valid key if seek went past
			it.Rewind()
			it.Seek(searchPrefix)
		}

		if it.ValidForPrefix(searchPrefix) {
			key := it.Item().KeyCopy(nil)

			// Decode key: kpc(3)|kind(2)|pubkey_hash(8)|created_at(8)|serial(5)
			// Total: 26 bytes
			if len(key) < 26 {
				return nil
			}

			// Extract serial (last 5 bytes)
			eventSerial = new(types.Uint40)
			serialReader := bytes.NewReader(key[len(key)-5:])
			if err := eventSerial.UnmarshalRead(serialReader); chk.E(err) {
				return err
			}
		}
		return nil
	})

	return eventSerial, err
}

// GetPubkeyHexFromSerial converts a pubkey serial to its hex string representation.
func (d *D) GetPubkeyHexFromSerial(serial *types.Uint40) (string, error) {
	pubkey, err := d.GetPubkeyBySerial(serial)
	if err != nil {
		return "", err
	}
	return hex.Enc(pubkey), nil
}

// GetEventIDFromSerial converts an event serial to its hex ID string.
func (d *D) GetEventIDFromSerial(serial *types.Uint40) (string, error) {
	eventID, err := d.GetEventIdBySerial(serial)
	if err != nil {
		return "", err
	}
	return hex.Enc(eventID), nil
}

// GetEventsReferencingPubkey finds all events that reference a pubkey via p-tags.
// Uses the peg (pubkey-event-graph) index with direction filter for inbound p-tags.
// Optionally filters by event kinds.
func (d *D) GetEventsReferencingPubkey(pubkeySerial *types.Uint40, kinds []uint16) ([]*types.Uint40, error) {
	var eventSerials []*types.Uint40

	if len(kinds) == 0 {
		// No kind filter - we need to scan common kinds since direction comes after kind in the key
		// Use same approach as QueryPTagGraph
		commonKinds := []uint16{1, 6, 7, 9735, 10002, 3, 4, 5, 30023}
		kinds = commonKinds
	}

	for _, k := range kinds {
		kind := new(types.Uint16)
		kind.Set(k)

		direction := new(types.Letter)
		direction.Set(types.EdgeDirectionPTagIn) // Inbound p-tags

		prefix := new(bytes.Buffer)
		if err := indexes.PubkeyEventGraphEnc(pubkeySerial, kind, direction, nil).MarshalWrite(prefix); chk.E(err) {
			return nil, err
		}
		searchPrefix := prefix.Bytes()

		err := d.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = searchPrefix

			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(searchPrefix); it.ValidForPrefix(searchPrefix); it.Next() {
				key := it.Item().KeyCopy(nil)

				// Key format: peg(3)|pubkey_serial(5)|kind(2)|direction(1)|event_serial(5) = 16 bytes
				if len(key) != 16 {
					continue
				}

				// Extract event serial (last 5 bytes)
				eventSerial := new(types.Uint40)
				serialReader := bytes.NewReader(key[11:16])
				if err := eventSerial.UnmarshalRead(serialReader); chk.E(err) {
					continue
				}

				eventSerials = append(eventSerials, eventSerial)
			}
			return nil
		})
		if chk.E(err) {
			return nil, err
		}
	}

	return eventSerials, nil
}

// GetEventsByAuthor finds all events authored by a pubkey.
// Uses the peg (pubkey-event-graph) index with direction filter for author edges.
// Optionally filters by event kinds.
func (d *D) GetEventsByAuthor(authorSerial *types.Uint40, kinds []uint16) ([]*types.Uint40, error) {
	var eventSerials []*types.Uint40

	if len(kinds) == 0 {
		// No kind filter - scan for author direction across common kinds
		// This is less efficient but necessary without kind filter
		commonKinds := []uint16{0, 1, 3, 6, 7, 30023, 10002}
		kinds = commonKinds
	}

	for _, k := range kinds {
		kind := new(types.Uint16)
		kind.Set(k)

		direction := new(types.Letter)
		direction.Set(types.EdgeDirectionAuthor) // Author edges

		prefix := new(bytes.Buffer)
		if err := indexes.PubkeyEventGraphEnc(authorSerial, kind, direction, nil).MarshalWrite(prefix); chk.E(err) {
			return nil, err
		}
		searchPrefix := prefix.Bytes()

		err := d.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = searchPrefix

			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(searchPrefix); it.ValidForPrefix(searchPrefix); it.Next() {
				key := it.Item().KeyCopy(nil)

				// Key format: peg(3)|pubkey_serial(5)|kind(2)|direction(1)|event_serial(5) = 16 bytes
				if len(key) != 16 {
					continue
				}

				// Extract event serial (last 5 bytes)
				eventSerial := new(types.Uint40)
				serialReader := bytes.NewReader(key[11:16])
				if err := eventSerial.UnmarshalRead(serialReader); chk.E(err) {
					continue
				}

				eventSerials = append(eventSerials, eventSerial)
			}
			return nil
		})
		if chk.E(err) {
			return nil, err
		}
	}

	return eventSerials, nil
}

// GetFollowsFromPubkeySerial returns the pubkey serials that a user follows.
// This extracts p-tags from the user's kind-3 contact list event.
// Returns an empty slice if no kind-3 event is found.
func (d *D) GetFollowsFromPubkeySerial(pubkeySerial *types.Uint40) ([]*types.Uint40, error) {
	// Find the kind-3 event for this pubkey
	contactEventSerial, err := d.FindEventByAuthorAndKind(pubkeySerial, 3)
	if err != nil {
		log.D.F("GetFollowsFromPubkeySerial: error finding kind-3 for serial %d: %v", pubkeySerial.Get(), err)
		return nil, nil // No kind-3 event found is not an error
	}
	if contactEventSerial == nil {
		log.T.F("GetFollowsFromPubkeySerial: no kind-3 event found for serial %d", pubkeySerial.Get())
		return nil, nil
	}

	// Extract p-tags from the contact list event
	follows, err := d.GetPTagsFromEventSerial(contactEventSerial)
	if err != nil {
		return nil, err
	}

	log.T.F("GetFollowsFromPubkeySerial: found %d follows for serial %d", len(follows), pubkeySerial.Get())
	return follows, nil
}

// GetFollowersOfPubkeySerial returns the pubkey serials of users who follow a given pubkey.
// This finds all kind-3 events that have a p-tag referencing the target pubkey.
func (d *D) GetFollowersOfPubkeySerial(targetSerial *types.Uint40) ([]*types.Uint40, error) {
	// Find all kind-3 events that reference this pubkey via p-tag
	kind3Events, err := d.GetEventsReferencingPubkey(targetSerial, []uint16{3})
	if err != nil {
		return nil, err
	}

	// Extract the author serials from these events
	var followerSerials []*types.Uint40
	seen := make(map[uint64]bool)

	for _, eventSerial := range kind3Events {
		// Get the author of this kind-3 event
		// We need to look up the event to get its author
		// Use the epg index to find the author edge
		authorSerial, err := d.GetEventAuthorSerial(eventSerial)
		if err != nil {
			log.D.F("GetFollowersOfPubkeySerial: couldn't get author for event %d: %v", eventSerial.Get(), err)
			continue
		}

		// Deduplicate (a user might have multiple kind-3 events)
		if seen[authorSerial.Get()] {
			continue
		}
		seen[authorSerial.Get()] = true
		followerSerials = append(followerSerials, authorSerial)
	}

	log.T.F("GetFollowersOfPubkeySerial: found %d followers for serial %d", len(followerSerials), targetSerial.Get())
	return followerSerials, nil
}

// GetEventAuthorSerial finds the author pubkey serial for an event.
// Uses the epg (event-pubkey-graph) index with author direction.
func (d *D) GetEventAuthorSerial(eventSerial *types.Uint40) (*types.Uint40, error) {
	var authorSerial *types.Uint40

	// Build prefix: epg|event_serial
	prefix := new(bytes.Buffer)
	prefix.Write([]byte(indexes.EventPubkeyGraphPrefix))
	if err := eventSerial.MarshalWrite(prefix); chk.E(err) {
		return nil, err
	}
	searchPrefix := prefix.Bytes()

	err := d.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = searchPrefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(searchPrefix); it.ValidForPrefix(searchPrefix); it.Next() {
			key := it.Item().KeyCopy(nil)

			// Decode key: epg(3)|event_serial(5)|pubkey_serial(5)|kind(2)|direction(1)
			if len(key) != 16 {
				continue
			}

			// Check direction - we want author (0)
			direction := key[15]
			if direction != types.EdgeDirectionAuthor {
				continue
			}

			// Extract pubkey serial (bytes 8-12)
			authorSerial = new(types.Uint40)
			serialReader := bytes.NewReader(key[8:13])
			if err := authorSerial.UnmarshalRead(serialReader); chk.E(err) {
				continue
			}

			return nil // Found the author
		}
		return ErrEventNotFound
	})

	return authorSerial, err
}

// PubkeyHexToSerial converts a pubkey hex string to its serial, if it exists.
// Returns an error if the pubkey is not in the database.
func (d *D) PubkeyHexToSerial(pubkeyHex string) (*types.Uint40, error) {
	pubkeyBytes, err := hex.Dec(pubkeyHex)
	if err != nil {
		return nil, err
	}
	if len(pubkeyBytes) != 32 {
		return nil, errors.New("invalid pubkey length")
	}
	return d.GetPubkeySerial(pubkeyBytes)
}

// EventIDHexToSerial converts an event ID hex string to its serial, if it exists.
// Returns an error if the event is not in the database.
func (d *D) EventIDHexToSerial(eventIDHex string) (*types.Uint40, error) {
	eventIDBytes, err := hex.Dec(eventIDHex)
	if err != nil {
		return nil, err
	}
	if len(eventIDBytes) != 32 {
		return nil, errors.New("invalid event ID length")
	}
	return d.GetSerialById(eventIDBytes)
}
