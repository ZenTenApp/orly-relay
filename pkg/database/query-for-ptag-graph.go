package database

import (
	"bytes"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// CanUsePTagGraph determines if a filter can benefit from p-tag graph optimization.
//
// Requirements:
// - Filter must have #p tags
// - Filter should NOT have authors (different index is better for that case)
// - Optimization works best with kinds filter but is optional
func CanUsePTagGraph(f *filter.F) bool {
	// Must have tags
	if f.Tags == nil || f.Tags.Len() == 0 {
		return false
	}

	// Check if there are any p-tags
	hasPTags := false
	for _, t := range *f.Tags {
		keyBytes := t.Key()
		if (len(keyBytes) == 1 && keyBytes[0] == 'p') ||
			(len(keyBytes) == 2 && keyBytes[0] == '#' && keyBytes[1] == 'p') {
			hasPTags = true
			break
		}
	}
	if !hasPTags {
		return false
	}

	// Don't use graph if there's an authors filter
	// (TagPubkey index handles that case better)
	if f.Authors != nil && f.Authors.Len() > 0 {
		return false
	}

	return true
}

// QueryPTagGraph uses the pubkey graph index for efficient p-tag queries.
//
// This query path is optimized for filters like:
//   {"#p": ["<pubkey>"], "kinds": [1, 6, 7]}
//
// Performance benefits:
// - 41% smaller index keys (16 bytes vs 27 bytes)
// - No hash collisions (exact serial match)
// - Kind-indexed in key structure
// - Direction-aware filtering
func (d *D) QueryPTagGraph(f *filter.F) (sers types.Uint40s, err error) {
	// Extract p-tags from filter
	var pTags [][]byte
	for _, t := range *f.Tags {
		keyBytes := t.Key()
		if (len(keyBytes) == 1 && keyBytes[0] == 'p') ||
			(len(keyBytes) == 2 && keyBytes[0] == '#' && keyBytes[1] == 'p') {
			// Get all values for this p-tag
			for _, valueBytes := range t.T[1:] {
				pTags = append(pTags, valueBytes)
			}
		}
	}

	if len(pTags) == 0 {
		return nil, nil
	}

	// Resolve pubkey hex → serials
	var pubkeySerials []*types.Uint40
	for _, pTagBytes := range pTags {
		var pubkeyBytes []byte

		// Handle both binary-encoded (33 bytes) and hex-encoded (64 chars) values
		// Filter tags may come as either format depending on how they were parsed
		if IsBinaryEncoded(pTagBytes) {
			// Already binary-encoded, extract the 32-byte hash
			pubkeyBytes = pTagBytes[:HashLen]
		} else {
			// Try to decode as hex using NormalizeTagToHex for consistent handling
			hexBytes := NormalizeTagToHex(pTagBytes)
			var decErr error
			if pubkeyBytes, decErr = hex.Dec(string(hexBytes)); chk.E(decErr) {
				log.D.F("QueryPTagGraph: failed to decode pubkey hex: %v", decErr)
				continue
			}
		}
		if len(pubkeyBytes) != 32 {
			log.D.F("QueryPTagGraph: invalid pubkey length: %d", len(pubkeyBytes))
			continue
		}

		// Get serial for this pubkey
		var serial *types.Uint40
		if serial, err = d.GetPubkeySerial(pubkeyBytes); chk.E(err) {
			log.D.F("QueryPTagGraph: pubkey not found in database: %s", hex.Enc(pubkeyBytes))
			err = nil // Reset error - this just means no events reference this pubkey
			continue
		}

		pubkeySerials = append(pubkeySerials, serial)
	}

	if len(pubkeySerials) == 0 {
		// None of the pubkeys have serials = no events reference them
		return nil, nil
	}

	// Build index ranges for each pubkey serial
	var ranges []Range

	// Get kinds from filter (if present)
	var kinds []uint16
	if f.Kinds != nil && f.Kinds.Len() > 0 {
		kinds = f.Kinds.ToUint16()
	}

	// For each pubkey serial, create a range
	for _, pkSerial := range pubkeySerials {
		if len(kinds) > 0 {
			// With kinds: peg|pubkey_serial|kind|direction|event_serial
			for _, k := range kinds {
				kind := new(types.Uint16)
				kind.Set(k)
				direction := new(types.Letter)
				direction.Set(types.EdgeDirectionPTagIn) // Direction 2: inbound p-tags

				start := new(bytes.Buffer)
				idx := indexes.PubkeyEventGraphEnc(pkSerial, kind, direction, nil)
				if err = idx.MarshalWrite(start); chk.E(err) {
					return
				}

				// End range: same prefix with all 0xFF for event serial
				end := start.Bytes()
				endWithSerial := make([]byte, len(end)+5)
				copy(endWithSerial, end)
				for i := 0; i < 5; i++ {
					endWithSerial[len(end)+i] = 0xFF
				}

				ranges = append(ranges, Range{
					Start: start.Bytes(),
					End:   endWithSerial,
				})
			}
		} else {
			// Without kinds: we need to scan all kinds for this pubkey
			// Key structure: peg|pubkey_serial(5)|kind(2)|direction(1)|event_serial(5)
			// Since direction comes after kind, we can't easily prefix-scan for a specific direction
			// across all kinds. Instead, we'll iterate through common kinds.
			//
			// Common Nostr kinds that use p-tags:
			// 1 (text note), 6 (repost), 7 (reaction), 9735 (zap), 10002 (relay list)
			commonKinds := []uint16{1, 6, 7, 9735, 10002, 3, 4, 5, 30023}

			for _, k := range commonKinds {
				kind := new(types.Uint16)
				kind.Set(k)
				direction := new(types.Letter)
				direction.Set(types.EdgeDirectionPTagIn) // Direction 2: inbound p-tags

				start := new(bytes.Buffer)
				idx := indexes.PubkeyEventGraphEnc(pkSerial, kind, direction, nil)
				if err = idx.MarshalWrite(start); chk.E(err) {
					return
				}

				// End range: same prefix with all 0xFF for event serial
				end := start.Bytes()
				endWithSerial := make([]byte, len(end)+5)
				copy(endWithSerial, end)
				for i := 0; i < 5; i++ {
					endWithSerial[len(end)+i] = 0xFF
				}

				ranges = append(ranges, Range{
					Start: start.Bytes(),
					End:   endWithSerial,
				})
			}
		}
	}

	// Execute scans for each range
	sers = make(types.Uint40s, 0, len(ranges)*100)
	for _, rng := range ranges {
		var rangeSers types.Uint40s
		if rangeSers, err = d.GetSerialsByRange(rng); chk.E(err) {
			continue
		}
		sers = append(sers, rangeSers...)
	}

	log.D.F("QueryPTagGraph: found %d events for %d pubkeys", len(sers), len(pubkeySerials))
	return
}
