package negentropy

import (
	"fmt"
)

// Negentropy handles the set reconciliation protocol.
type Negentropy struct {
	storage        Storage
	frameSizeLimit int
	idSize         int

	// Result slices populated during Reconcile
	havesList    []string // IDs we have that peer needs
	haveNotsList []string // IDs peer has that we need

	// Protocol state
	isInitiator bool
}

// New creates a new Negentropy instance.
func New(storage Storage, frameSizeLimit int) *Negentropy {
	if frameSizeLimit < 4096 {
		frameSizeLimit = 4096
	}

	return &Negentropy{
		storage:        storage,
		frameSizeLimit: frameSizeLimit,
		idSize:         FullIDSize,
	}
}

// Haves returns the IDs we have that the peer needs.
func (n *Negentropy) Haves() []string {
	return n.havesList
}

// HaveNots returns the IDs the peer has that we need.
func (n *Negentropy) HaveNots() []string {
	return n.haveNotsList
}

// Start creates the initial message to begin reconciliation (client side).
func (n *Negentropy) Start() ([]byte, error) {
	n.isInitiator = true

	enc := NewEncoder(n.frameSizeLimit)

	// Write protocol version
	enc.WriteByte(ProtocolVersion)

	// Initial range covers everything
	n.splitRange(enc, 0, n.storage.Size(), MinBound(), MaxBound())

	return enc.Bytes(), nil
}

// Reconcile processes an incoming message and generates a response.
// Returns the response message and whether reconciliation is complete.
func (n *Negentropy) Reconcile(msg []byte) (response []byte, complete bool, err error) {
	if len(msg) == 0 {
		return nil, false, ErrInvalidMessage
	}

	dec := NewDecoder(msg)

	// Check protocol version
	version, err := dec.ReadByte()
	if err != nil {
		return nil, false, err
	}
	if version != ProtocolVersion {
		return nil, false, fmt.Errorf("unsupported protocol version: %x", version)
	}

	enc := NewEncoder(n.frameSizeLimit)
	enc.WriteByte(ProtocolVersion)

	// Process all ranges in the message
	prevBound := MinBound()
	prevIndex := 0
	exceededFrameSize := false

	for dec.HasMore() {
		// Read upper bound of this range
		upperBound, err := dec.ReadBound()
		if err != nil {
			return nil, false, fmt.Errorf("failed to read bound: %w", err)
		}

		// Read mode
		mode, err := dec.ReadMode()
		if err != nil {
			return nil, false, fmt.Errorf("failed to read mode: %w", err)
		}

		// Find the range in our storage
		lower := n.storage.FindLowerBound(prevIndex, n.storage.Size(), prevBound)
		upper := n.storage.FindLowerBound(lower, n.storage.Size(), upperBound)

		// If frame size exceeded, emit compact fingerprint for remaining ranges
		// instead of detailed data. Using Fingerprint (not Skip) ensures the
		// peer will re-examine these ranges in the next round.
		if exceededFrameSize {
			// Must still advance decoder past mode data
			n.skipModeData(dec, mode)

			// Emit fingerprint so peer continues reconciliation for this range
			fp := n.storage.Fingerprint(lower, upper)
			enc.WriteBound(upperBound)
			enc.WriteMode(ModeFingerprint)
			enc.WriteFingerprint(fp)

			prevBound = upperBound
			prevIndex = upper
			continue
		}

		switch mode {
		case ModeSkip:
			// Range is in sync, skip it
			n.skipRange(enc, upperBound)

		case ModeFingerprint:
			// Read their fingerprint
			theirFP, err := dec.ReadFingerprint()
			if err != nil {
				return nil, false, fmt.Errorf("failed to read fingerprint: %w", err)
			}

			// Compare with our fingerprint
			ourFP := n.storage.Fingerprint(lower, upper)

			if ourFP == theirFP {
				// Fingerprints match, skip this range
				n.skipRange(enc, upperBound)
			} else {
				// Fingerprints differ, need to split or send IDs
				n.splitRange(enc, lower, upper, prevBound, upperBound)
			}

		case ModeIdList:
			// Read their ID count
			numIds, err := dec.ReadVarInt()
			if err != nil {
				return nil, false, fmt.Errorf("failed to read id count: %w", err)
			}

			if n.isInitiator {
				// Initiator: read their IDs, compare with ours, record diffs
				theirIds := make(map[string]bool)
				for i := uint64(0); i < numIds; i++ {
					idBytes, err := dec.ReadBytes(n.idSize)
					if err != nil {
						return nil, false, fmt.Errorf("failed to read id: %w", err)
					}
					theirIds[encodeHex(idBytes)] = true
				}

				// Find differences
				for _, item := range n.storage.Range(lower, upper) {
					fullID := item.ID[:n.idSize*2] // hex is 2 chars per byte
					if !theirIds[fullID] {
						// We have it, they don't
						n.havesList = append(n.havesList, item.ID)
					}
					delete(theirIds, fullID)
				}

				// Remaining IDs are ones they have that we don't
				for id := range theirIds {
					n.haveNotsList = append(n.haveNotsList, id)
				}

				// Initiator: skip this range (diffs already computed)
				n.skipRange(enc, upperBound)
			} else {
				// Responder: read past their IDs to advance decoder position
				for i := uint64(0); i < numIds; i++ {
					_, err := dec.ReadBytes(n.idSize)
					if err != nil {
						return nil, false, fmt.Errorf("failed to read id: %w", err)
					}
				}

				// Check if our range would exceed frame size as an ID list
				numOurItems := upper - lower
				if n.estimateIdListSize(numOurItems)+len(enc.Bytes()) > n.frameSizeLimit {
					// Too large for ID list - use splitRange for compact fingerprint buckets
					n.splitRange(enc, lower, upper, prevBound, upperBound)
				} else {
					// Fits in frame - send our own ID list so initiator can compute diffs
					enc.WriteBound(upperBound)
					enc.WriteMode(ModeIdList)
					enc.WriteVarInt(uint64(numOurItems))
					for _, item := range n.storage.Range(lower, upper) {
						idBytes, _ := decodeHex(item.ID)
						if len(idBytes) >= FullIDSize {
							enc.WriteBytes(idBytes[:FullIDSize])
						} else if len(idBytes) > 0 {
							// Pad short IDs with zeros to FullIDSize
							padded := make([]byte, FullIDSize)
							copy(padded, idBytes)
							enc.WriteBytes(padded)
						}
					}
				}
			}
		}

		// Check if we've exceeded the frame size limit after processing this range
		if len(enc.Bytes()) > n.frameSizeLimit {
			exceededFrameSize = true
		}

		prevBound = upperBound
		prevIndex = upper
	}

	response = enc.Bytes()

	// Check if reconciliation is complete
	// Complete when response only contains version + all skips
	complete = n.isResponseComplete(response)

	return response, complete, nil
}

// skipModeData advances the decoder past the data for a given mode without
// processing it. Used when frame size is exceeded and we need to defer ranges.
func (n *Negentropy) skipModeData(dec *Decoder, mode Mode) {
	switch mode {
	case ModeSkip:
		// No additional data
	case ModeFingerprint:
		// Read and discard fingerprint (16 bytes)
		dec.ReadBytes(DefaultIDSize)
	case ModeIdList:
		// Read count, then skip that many IDs
		numIds, err := dec.ReadVarInt()
		if err != nil {
			return
		}
		for i := uint64(0); i < numIds; i++ {
			dec.ReadBytes(n.idSize)
		}
	}
}

// skipRange writes a skip mode for the given bound.
func (n *Negentropy) skipRange(enc *Encoder, upperBound Bound) {
	enc.WriteBound(upperBound)
	enc.WriteMode(ModeSkip)
}

// splitRange splits a range into buckets and writes fingerprints or ID lists.
func (n *Negentropy) splitRange(enc *Encoder, lower, upper int, lowerBound, upperBound Bound) {
	numItems := upper - lower

	if numItems == 0 {
		// Empty range, send as ID list with 0 items
		enc.WriteBound(upperBound)
		enc.WriteMode(ModeIdList)
		enc.WriteVarInt(0)
		return
	}

	// For small ranges, send full ID list if it fits in the remaining frame space
	if numItems <= 2 || n.estimateIdListSize(numItems) < n.frameSizeLimit/10 {
		// Also check cumulative frame size before writing ID list
		if n.estimateIdListSize(numItems)+len(enc.Bytes()) <= n.frameSizeLimit {
			enc.WriteBound(upperBound)
			enc.WriteMode(ModeIdList)
			enc.WriteVarInt(uint64(numItems))
			for _, item := range n.storage.Range(lower, upper) {
				idBytes, _ := decodeHex(item.ID)
				if len(idBytes) >= FullIDSize {
					enc.WriteBytes(idBytes[:FullIDSize])
				} else if len(idBytes) > 0 {
					// Pad short IDs with zeros to FullIDSize
					padded := make([]byte, FullIDSize)
					copy(padded, idBytes)
					enc.WriteBytes(padded)
				}
			}
			return
		}
		// ID list would exceed frame - fall through to fingerprint buckets
	}

	// For larger ranges, split into buckets with fingerprints
	numBuckets := n.calculateBuckets(numItems)
	itemsPerBucket := numItems / numBuckets

	for i := 0; i < numBuckets; i++ {
		bucketStart := lower + i*itemsPerBucket
		bucketEnd := lower + (i+1)*itemsPerBucket
		if i == numBuckets-1 {
			bucketEnd = upper // Last bucket gets remainder
		}

		var bucketBound Bound
		if i == numBuckets-1 {
			// Last bucket must use the original upperBound to maintain
			// range alignment with the peer. Using GetBound(bucketEnd)
			// can produce a different bound when the peer's boundary
			// event doesn't exist in our storage, causing range
			// misalignment and false fingerprint mismatches.
			bucketBound = upperBound
		} else {
			bucketBound = n.storage.GetBound(bucketEnd)
		}

		enc.WriteBound(bucketBound)
		enc.WriteMode(ModeFingerprint)
		fp := n.storage.Fingerprint(bucketStart, bucketEnd)
		enc.WriteFingerprint(fp)
	}
}

// estimateIdListSize estimates the encoded size of an ID list.
func (n *Negentropy) estimateIdListSize(numItems int) int {
	// Bound + mode + count varint + (idSize * numItems)
	return 20 + VarIntSize(uint64(numItems)) + n.idSize*numItems
}

// calculateBuckets determines how many buckets to split a range into.
func (n *Negentropy) calculateBuckets(numItems int) int {
	// Use square root heuristic, clamped to reasonable bounds
	buckets := 1
	for buckets*buckets < numItems {
		buckets++
	}
	if buckets < 2 {
		buckets = 2
	}
	if buckets > 16 {
		buckets = 16
	}
	return buckets
}

// isResponseComplete checks if the response indicates reconciliation is complete.
func (n *Negentropy) isResponseComplete(response []byte) bool {
	if len(response) <= 1 {
		return true
	}

	dec := NewDecoder(response)

	// Skip version
	_, err := dec.ReadByte()
	if err != nil {
		return false
	}

	// Check if all ranges are skips
	for dec.HasMore() {
		_, err := dec.ReadBound()
		if err != nil {
			return false
		}

		mode, err := dec.ReadMode()
		if err != nil {
			return false
		}

		if mode != ModeSkip {
			return false
		}
	}

	return true
}

// Close is a no-op (retained for API compatibility).
func (n *Negentropy) Close() {
}

// CollectHaves returns all IDs we have that the peer needs, and resets the list.
func (n *Negentropy) CollectHaves() []string {
	result := n.havesList
	n.havesList = nil
	return result
}

// CollectHaveNots returns all IDs the peer has that we need, and resets the list.
func (n *Negentropy) CollectHaveNots() []string {
	result := n.haveNotsList
	n.haveNotsList = nil
	return result
}
