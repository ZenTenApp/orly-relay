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
	MaxDepth                 int     // BFS hop depth (default 6)
	MaxCycles                int     // max convergence iterations safety cap (default 20)
	DeltaThreshold           float64 // convergence stop threshold: max delta across all nodes (default 0.001)
	AttenuationFactor        float64 // weight decay per hop for non-observer raters (default 0.8)
	Rigor                    float64 // certainty curve steepness (default 0.25)
	FollowConfidence         float64 // base confidence for a follow edge from any rater (default 0.05)
	ObserverFollowConfidence float64 // confidence for the observer's own follow edges (default 1.0)
	MuteRating               float64 // interpretation score for a mute edge (default -1.0)
	MuteConfidence           float64 // confidence weight for a mute edge (default 0.1)
	ReportRating             float64 // interpretation score for a report edge (default -1.0)
	ReportConfidence         float64 // confidence weight for a report edge (default 0.1)
}

// DefaultConfig returns sensible defaults aligned with cloudfodder's GrapeRank algorithm.
func DefaultConfig() Config {
	return Config{
		MaxDepth:                 6,
		MaxCycles:                20,
		DeltaThreshold:           0.001,
		AttenuationFactor:        0.8,
		Rigor:                    0.25,
		FollowConfidence:         0.05,
		ObserverFollowConfidence: 1.0,
		MuteRating:               -1.0,
		MuteConfidence:           0.1,
		ReportRating:             -1.0,
		ReportConfidence:         0.1,
	}
}

// GraphSource provides read access to the follow graph.
type GraphSource interface {
	// TraverseFollowsOutbound does BFS outward from seed pubkey, returning
	// pubkeys grouped by depth and a flat map of all pubkeys to their depth.
	TraverseFollowsOutbound(seedPubkey []byte, maxDepth int) (
		pubkeysByDepth map[int][]string, allPubkeys map[string]int, err error,
	)
	// GetFollowerPubkeys returns hex pubkeys of accounts that follow the target (kind-3).
	GetFollowerPubkeys(targetHex string) ([]string, error)
	// GetFollowsPubkeys returns hex pubkeys that the source follows.
	GetFollowsPubkeys(sourceHex string) ([]string, error)
	// GetMuterPubkeys returns hex pubkeys of accounts that mute the target (kind-10000).
	GetMuterPubkeys(targetHex string) ([]string, error)
	// GetReporterPubkeys returns hex pubkeys of accounts that report the target (kind-1984).
	GetReporterPubkeys(targetHex string) ([]string, error)
}

// ScoreStore persists computed score sets as raw JSON blobs.
// This avoids circular dependencies between the database and grapevine packages.
type ScoreStore interface {
	SaveScoreSet(observerHex string, setData []byte, entries map[string][]byte) error
	GetScoreSet(observerHex string) ([]byte, error)
	GetScore(observerHex, targetHex string) ([]byte, error)
}
