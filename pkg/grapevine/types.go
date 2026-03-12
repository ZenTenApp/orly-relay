package grapevine

import "time"

// ScoreEntry holds computed influence and WoT scores for a single target pubkey.
type ScoreEntry struct {
	PubkeyHex string  `json:"pubkey"`
	Influence float64 `json:"influence"`
	Average   float64 `json:"average"`
	Certainty float64 `json:"certainty"`
	Input     float64 `json:"input"`
	WoTScore  int     `json:"wot_score"` // intersection count: how many of observer's follows also follow this target
	Depth     int     `json:"depth"`     // BFS hop distance from observer
}

// ScoreSet is the complete result for one observer.
type ScoreSet struct {
	ObserverHex  string       `json:"observer"`
	Scores       []ScoreEntry `json:"scores"`
	ComputedAt   time.Time    `json:"computed_at"`
	ComputeMs    int64        `json:"compute_ms"`
	TotalPubkeys int          `json:"total_pubkeys"`
}

// Config holds algorithm tuning parameters.
type Config struct {
	MaxDepth          int     // BFS hop depth (default 6)
	Cycles            int     // convergence iterations (default 5)
	AttenuationFactor float64 // weight decay for non-observer raters (default 0.8)
	Rigor             float64 // certainty curve steepness (default 0.25)
	FollowConfidence  float64 // base confidence for a follow edge (default 0.05)
}

// DefaultConfig returns sensible defaults matching cloudfodder's original algorithm.
func DefaultConfig() Config {
	return Config{
		MaxDepth:          6,
		Cycles:            5,
		AttenuationFactor: 0.8,
		Rigor:             0.25,
		FollowConfidence:  0.05,
	}
}

// GraphSource provides read access to the follow graph.
type GraphSource interface {
	// TraverseFollowsOutbound does BFS outward from seed pubkey, returning
	// pubkeys grouped by depth and a flat map of all pubkeys to their depth.
	TraverseFollowsOutbound(seedPubkey []byte, maxDepth int) (
		pubkeysByDepth map[int][]string, allPubkeys map[string]int, err error,
	)
	// GetFollowerPubkeys returns hex pubkeys of accounts that follow the target.
	GetFollowerPubkeys(targetHex string) ([]string, error)
	// GetFollowsPubkeys returns hex pubkeys that the source follows.
	GetFollowsPubkeys(sourceHex string) ([]string, error)
}

// ScoreStore persists computed score sets as raw JSON blobs.
// This avoids circular dependencies between the database and grapevine packages.
type ScoreStore interface {
	SaveScoreSet(observerHex string, setData []byte, entries map[string][]byte) error
	GetScoreSet(observerHex string) ([]byte, error)
	GetScore(observerHex, targetHex string) ([]byte, error)
}
