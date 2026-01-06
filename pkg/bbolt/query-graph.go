//go:build !(js && wasm)

package bbolt

import (
	"bytes"

	bolt "go.etcd.io/bbolt"
	"next.orly.dev/pkg/database/indexes/types"
)

// EdgeExists checks if an edge exists between two serials.
// Uses bloom filter for fast negative lookups.
func (b *B) EdgeExists(srcSerial, dstSerial uint64, edgeType byte) (bool, error) {
	// Fast path: check bloom filter first
	if !b.edgeBloom.MayExist(srcSerial, dstSerial, edgeType) {
		return false, nil // Definitely doesn't exist
	}

	// Bloom says maybe - need to verify in adjacency list
	return b.verifyEdgeInAdjacencyList(srcSerial, dstSerial, edgeType)
}

// verifyEdgeInAdjacencyList checks the adjacency list for edge existence.
func (b *B) verifyEdgeInAdjacencyList(srcSerial, dstSerial uint64, edgeType byte) (bool, error) {
	var exists bool

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketEv)
		if bucket == nil {
			return nil
		}

		key := makeSerialKey(srcSerial)
		data := bucket.Get(key)
		if data == nil {
			return nil
		}

		vertex := &EventVertex{}
		if err := vertex.Decode(data); err != nil {
			return err
		}

		switch edgeType {
		case EdgeTypeAuthor:
			exists = vertex.AuthorSerial == dstSerial
		case EdgeTypePTag:
			for _, s := range vertex.PTagSerials {
				if s == dstSerial {
					exists = true
					break
				}
			}
		case EdgeTypeETag:
			for _, s := range vertex.ETagSerials {
				if s == dstSerial {
					exists = true
					break
				}
			}
		}
		return nil
	})

	return exists, err
}

// GetEventVertex retrieves the adjacency list for an event.
func (b *B) GetEventVertex(eventSerial uint64) (*EventVertex, error) {
	var vertex *EventVertex

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketEv)
		if bucket == nil {
			return nil
		}

		key := makeSerialKey(eventSerial)
		data := bucket.Get(key)
		if data == nil {
			return nil
		}

		vertex = &EventVertex{}
		return vertex.Decode(data)
	})

	return vertex, err
}

// GetPubkeyVertex retrieves the adjacency list for a pubkey.
func (b *B) GetPubkeyVertex(pubkeySerial uint64) (*PubkeyVertex, error) {
	var vertex *PubkeyVertex

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketPv)
		if bucket == nil {
			return nil
		}

		key := makeSerialKey(pubkeySerial)
		data := bucket.Get(key)
		if data == nil {
			return nil
		}

		vertex = &PubkeyVertex{}
		return vertex.Decode(data)
	})

	return vertex, err
}

// GetEventsAuthoredBy returns event serials authored by a pubkey.
func (b *B) GetEventsAuthoredBy(pubkeySerial uint64) ([]uint64, error) {
	vertex, err := b.GetPubkeyVertex(pubkeySerial)
	if err != nil || vertex == nil {
		return nil, err
	}
	return vertex.AuthoredEvents, nil
}

// GetEventsMentioning returns event serials that mention a pubkey.
func (b *B) GetEventsMentioning(pubkeySerial uint64) ([]uint64, error) {
	vertex, err := b.GetPubkeyVertex(pubkeySerial)
	if err != nil || vertex == nil {
		return nil, err
	}
	return vertex.MentionedIn, nil
}

// GetPTagsFromEvent returns pubkey serials tagged in an event.
func (b *B) GetPTagsFromEvent(eventSerial uint64) ([]uint64, error) {
	vertex, err := b.GetEventVertex(eventSerial)
	if err != nil || vertex == nil {
		return nil, err
	}
	return vertex.PTagSerials, nil
}

// GetETagsFromEvent returns event serials referenced by an event.
func (b *B) GetETagsFromEvent(eventSerial uint64) ([]uint64, error) {
	vertex, err := b.GetEventVertex(eventSerial)
	if err != nil || vertex == nil {
		return nil, err
	}
	return vertex.ETagSerials, nil
}

// GetFollowsFromPubkeySerial returns the pubkey serials that a user follows.
// This extracts p-tags from the user's kind-3 contact list event.
func (b *B) GetFollowsFromPubkeySerial(pubkeySerial *types.Uint40) ([]*types.Uint40, error) {
	if pubkeySerial == nil {
		return nil, nil
	}

	// Find the kind-3 event for this pubkey
	contactEventSerial, err := b.FindEventByAuthorAndKind(pubkeySerial.Get(), 3)
	if err != nil {
		return nil, nil // No kind-3 event found is not an error
	}
	if contactEventSerial == 0 {
		return nil, nil
	}

	// Get the p-tags from the event vertex
	pTagSerials, err := b.GetPTagsFromEvent(contactEventSerial)
	if err != nil {
		return nil, err
	}

	// Convert to types.Uint40
	result := make([]*types.Uint40, 0, len(pTagSerials))
	for _, s := range pTagSerials {
		ser := new(types.Uint40)
		ser.Set(s)
		result = append(result, ser)
	}
	return result, nil
}

// FindEventByAuthorAndKind finds an event serial by author and kind.
// For replaceable events like kind-3, returns the most recent one.
func (b *B) FindEventByAuthorAndKind(authorSerial uint64, kindNum uint16) (uint64, error) {
	var resultSerial uint64

	err := b.db.View(func(tx *bolt.Tx) error {
		// First, get events authored by this pubkey
		pvBucket := tx.Bucket(bucketPv)
		if pvBucket == nil {
			return nil
		}

		pvKey := makeSerialKey(authorSerial)
		pvData := pvBucket.Get(pvKey)
		if pvData == nil {
			return nil
		}

		vertex := &PubkeyVertex{}
		if err := vertex.Decode(pvData); err != nil {
			return err
		}

		// Search through authored events for matching kind
		evBucket := tx.Bucket(bucketEv)
		if evBucket == nil {
			return nil
		}

		var latestTs int64
		for _, eventSerial := range vertex.AuthoredEvents {
			evKey := makeSerialKey(eventSerial)
			evData := evBucket.Get(evKey)
			if evData == nil {
				continue
			}

			evVertex := &EventVertex{}
			if err := evVertex.Decode(evData); err != nil {
				continue
			}

			if evVertex.Kind == kindNum {
				// For replaceable events, we need to check timestamp
				// Get event to compare timestamps
				fpcBucket := tx.Bucket(bucketFpc)
				if fpcBucket != nil {
					// Scan for matching serial prefix in fpc bucket
					c := fpcBucket.Cursor()
					prefix := makeSerialKey(eventSerial)
					for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
						// Key format: serial(5) | id(32) | pubkey_hash(8) | created_at(8)
						if len(k) >= 53 {
							ts := int64(decodeUint64(k[45:53]))
							if ts > latestTs {
								latestTs = ts
								resultSerial = eventSerial
							}
						}
						break
					}
				} else {
					// If no fpc bucket, just take the first match
					resultSerial = eventSerial
				}
			}
		}
		return nil
	})

	return resultSerial, err
}

// GetReferencingEvents returns event serials that reference a target event via e-tag.
func (b *B) GetReferencingEvents(targetSerial uint64) ([]uint64, error) {
	var result []uint64

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketEv)
		if bucket == nil {
			return nil
		}

		// Scan all event vertices looking for e-tag references
		// Note: This is O(n) - for production, consider a reverse index
		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			vertex := &EventVertex{}
			if err := vertex.Decode(v); err != nil {
				continue
			}

			for _, eTagSerial := range vertex.ETagSerials {
				if eTagSerial == targetSerial {
					eventSerial := decodeUint40(k)
					result = append(result, eventSerial)
					break
				}
			}
		}
		return nil
	})

	return result, err
}
