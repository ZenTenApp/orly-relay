//go:build !(js && wasm)

package grapevine

import (
	"git.smesh.lol/orly/pkg/database"
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

// GetFollowerPubkeys returns hex pubkeys of accounts that kind-3 follow the target.
func (s *BadgerGraphSource) GetFollowerPubkeys(targetHex string) ([]string, error) {
	serial, err := s.db.PubkeyHexToSerial(targetHex)
	if err != nil {
		return nil, nil // unknown pubkey, no followers
	}
	followers, err := s.db.GetFollowersByKindViaGPP(serial, 3)
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

// GetFollowsPubkeys returns hex pubkeys that the source kind-3 follows.
func (s *BadgerGraphSource) GetFollowsPubkeys(sourceHex string) ([]string, error) {
	serial, err := s.db.PubkeyHexToSerial(sourceHex)
	if err != nil {
		return nil, nil // unknown pubkey, no follows
	}
	follows, err := s.db.GetFollowsByKindViaPPG(serial, 3)
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

// GetMuterPubkeys returns hex pubkeys of accounts that mute the target (kind-10000).
func (s *BadgerGraphSource) GetMuterPubkeys(targetHex string) ([]string, error) {
	serial, err := s.db.PubkeyHexToSerial(targetHex)
	if err != nil {
		return nil, nil // unknown pubkey, no muters
	}
	muters, err := s.db.GetFollowersByKindViaGPP(serial, 10000)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(muters))
	for _, m := range muters {
		h, err := s.db.GetPubkeyHexFromSerial(m)
		if err != nil {
			continue
		}
		result = append(result, h)
	}
	return result, nil
}

// GetReporterPubkeys returns hex pubkeys of accounts that report the target (kind-1984).
func (s *BadgerGraphSource) GetReporterPubkeys(targetHex string) ([]string, error) {
	serial, err := s.db.PubkeyHexToSerial(targetHex)
	if err != nil {
		return nil, nil // unknown pubkey, no reporters
	}
	reporters, err := s.db.GetFollowersByKindViaGPP(serial, 1984)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(reporters))
	for _, r := range reporters {
		h, err := s.db.GetPubkeyHexFromSerial(r)
		if err != nil {
			continue
		}
		result = append(result, h)
	}
	return result, nil
}
