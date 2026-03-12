//go:build !(js && wasm)

package grapevine

import (
	"next.orly.dev/pkg/database"
)

// BadgerGraphSource adapts Badger's graph indexes to the GraphSource interface.
type BadgerGraphSource struct {
	db *database.D
}

// NewBadgerGraphSource creates a new BadgerGraphSource wrapping a Badger database.
func NewBadgerGraphSource(db *database.D) *BadgerGraphSource {
	return &BadgerGraphSource{db: db}
}

// TraverseFollowsOutbound does BFS outward from seed pubkey using the ppg index.
func (s *BadgerGraphSource) TraverseFollowsOutbound(seedPubkey []byte, maxDepth int) (
	pubkeysByDepth map[int][]string, allPubkeys map[string]int, err error,
) {
	result, err := s.db.TraversePubkeyPubkey(seedPubkey, maxDepth, "out")
	if err != nil {
		return nil, nil, err
	}
	allPubkeys = make(map[string]int, result.TotalPubkeys)
	for depth, pks := range result.PubkeysByDepth {
		for _, pk := range pks {
			allPubkeys[pk] = depth
		}
	}
	return result.PubkeysByDepth, allPubkeys, nil
}

// GetFollowerPubkeys returns hex pubkeys of accounts that follow the target.
func (s *BadgerGraphSource) GetFollowerPubkeys(targetHex string) ([]string, error) {
	serial, err := s.db.PubkeyHexToSerial(targetHex)
	if err != nil {
		return nil, nil // unknown pubkey, no followers
	}
	followers, err := s.db.GetFollowersViaGPP(serial)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(followers))
	for _, f := range followers {
		h, err := s.db.GetPubkeyHexFromSerial(f)
		if err != nil {
			continue
		}
		result = append(result, h)
	}
	return result, nil
}

// GetFollowsPubkeys returns hex pubkeys that the source follows.
func (s *BadgerGraphSource) GetFollowsPubkeys(sourceHex string) ([]string, error) {
	serial, err := s.db.PubkeyHexToSerial(sourceHex)
	if err != nil {
		return nil, nil // unknown pubkey, no follows
	}
	follows, err := s.db.GetFollowsViaPPG(serial)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(follows))
	for _, f := range follows {
		h, err := s.db.GetPubkeyHexFromSerial(f)
		if err != nil {
			continue
		}
		result = append(result, h)
	}
	return result, nil
}
