//go:build !(js && wasm)

package database

import (
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

// TraverseFollows performs BFS traversal of the follow graph starting from a seed pubkey.
// Returns pubkeys grouped by first-discovered depth (no duplicates across depths).
//
// The traversal works by:
// 1. Starting with the seed pubkey at depth 0 (not included in results)
// 2. For each pubkey at the current depth, find their kind-3 contact list
// 3. Extract p-tags from the contact list to get follows
// 4. Add new (unseen) follows to the next depth
// 5. Continue until maxDepth is reached or no new pubkeys are found
//
// Early termination occurs if two consecutive depths yield no new pubkeys.
func (d *D) TraverseFollows(seedPubkey []byte, maxDepth int) (*GraphResult, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, ErrPubkeyNotFound
	}

	// Get seed pubkey serial
	seedSerial, err := d.GetPubkeySerial(seedPubkey)
	if err != nil {
		log.D.F("TraverseFollows: seed pubkey not in database: %s", hex.Enc(seedPubkey))
		return result, nil // Not an error - just no results
	}

	// Track visited pubkeys by serial to avoid cycles
	visited := make(map[uint64]bool)
	visited[seedSerial.Get()] = true // Mark seed as visited but don't add to results

	// Current frontier (pubkeys to process at this depth)
	currentFrontier := []*types.Uint40{seedSerial}

	// Track consecutive empty depths for early termination
	consecutiveEmptyDepths := 0

	for currentDepth := 1; currentDepth <= maxDepth; currentDepth++ {
		var nextFrontier []*types.Uint40
		newPubkeysAtDepth := 0

		for _, pubkeySerial := range currentFrontier {
			// Get follows for this pubkey
			follows, err := d.GetFollowsFromPubkeySerial(pubkeySerial)
			if err != nil {
				log.D.F("TraverseFollows: error getting follows for serial %d: %v", pubkeySerial.Get(), err)
				continue
			}

			for _, followSerial := range follows {
				// Skip if already visited
				if visited[followSerial.Get()] {
					continue
				}
				visited[followSerial.Get()] = true

				// Get pubkey hex for result
				pubkeyHex, err := d.GetPubkeyHexFromSerial(followSerial)
				if err != nil {
					log.D.F("TraverseFollows: error getting pubkey hex for serial %d: %v", followSerial.Get(), err)
					continue
				}

				// Add to results at this depth
				result.AddPubkeyAtDepth(pubkeyHex, currentDepth)
				newPubkeysAtDepth++

				// Add to next frontier for further traversal
				nextFrontier = append(nextFrontier, followSerial)
			}
		}

		log.T.F("TraverseFollows: depth %d found %d new pubkeys", currentDepth, newPubkeysAtDepth)

		// Check for early termination
		if newPubkeysAtDepth == 0 {
			consecutiveEmptyDepths++
			if consecutiveEmptyDepths >= 2 {
				log.T.F("TraverseFollows: early termination at depth %d (2 consecutive empty depths)", currentDepth)
				break
			}
		} else {
			consecutiveEmptyDepths = 0
		}

		// Move to next depth
		currentFrontier = nextFrontier
	}

	log.D.F("TraverseFollows: completed with %d total pubkeys across %d depths",
		result.TotalPubkeys, len(result.PubkeysByDepth))

	return result, nil
}

// TraverseFollowers performs BFS traversal to find who follows the seed pubkey.
// This is the reverse of TraverseFollows - it finds users whose kind-3 lists
// contain the target pubkey(s).
//
// At each depth:
// - Depth 1: Users who directly follow the seed
// - Depth 2: Users who follow anyone at depth 1 (followers of followers)
// - etc.
func (d *D) TraverseFollowers(seedPubkey []byte, maxDepth int) (*GraphResult, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, ErrPubkeyNotFound
	}

	// Get seed pubkey serial
	seedSerial, err := d.GetPubkeySerial(seedPubkey)
	if err != nil {
		log.D.F("TraverseFollowers: seed pubkey not in database: %s", hex.Enc(seedPubkey))
		return result, nil
	}

	// Track visited pubkeys
	visited := make(map[uint64]bool)
	visited[seedSerial.Get()] = true

	// Current frontier
	currentFrontier := []*types.Uint40{seedSerial}

	consecutiveEmptyDepths := 0

	for currentDepth := 1; currentDepth <= maxDepth; currentDepth++ {
		var nextFrontier []*types.Uint40
		newPubkeysAtDepth := 0

		for _, targetSerial := range currentFrontier {
			// Get followers of this pubkey
			followers, err := d.GetFollowersOfPubkeySerial(targetSerial)
			if err != nil {
				log.D.F("TraverseFollowers: error getting followers for serial %d: %v", targetSerial.Get(), err)
				continue
			}

			for _, followerSerial := range followers {
				if visited[followerSerial.Get()] {
					continue
				}
				visited[followerSerial.Get()] = true

				pubkeyHex, err := d.GetPubkeyHexFromSerial(followerSerial)
				if err != nil {
					continue
				}

				result.AddPubkeyAtDepth(pubkeyHex, currentDepth)
				newPubkeysAtDepth++
				nextFrontier = append(nextFrontier, followerSerial)
			}
		}

		log.T.F("TraverseFollowers: depth %d found %d new pubkeys", currentDepth, newPubkeysAtDepth)

		if newPubkeysAtDepth == 0 {
			consecutiveEmptyDepths++
			if consecutiveEmptyDepths >= 2 {
				break
			}
		} else {
			consecutiveEmptyDepths = 0
		}

		currentFrontier = nextFrontier
	}

	log.D.F("TraverseFollowers: completed with %d total pubkeys", result.TotalPubkeys)

	return result, nil
}

// TraverseFollowsFromHex is a convenience wrapper that accepts hex-encoded pubkey.
func (d *D) TraverseFollowsFromHex(seedPubkeyHex string, maxDepth int) (*GraphResult, error) {
	seedPubkey, err := hex.Dec(seedPubkeyHex)
	if err != nil {
		return nil, err
	}
	return d.TraverseFollows(seedPubkey, maxDepth)
}

// TraverseFollowersFromHex is a convenience wrapper that accepts hex-encoded pubkey.
func (d *D) TraverseFollowersFromHex(seedPubkeyHex string, maxDepth int) (*GraphResult, error) {
	seedPubkey, err := hex.Dec(seedPubkeyHex)
	if err != nil {
		return nil, err
	}
	return d.TraverseFollowers(seedPubkey, maxDepth)
}
