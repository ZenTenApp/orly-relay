package ratelimit

// QueryCostLevel categorizes query expense for adaptive rate limiting.
// Higher levels incur greater delays under load and are rejected first
// during emergency mode.
type QueryCostLevel int

const (
	// CostTrivial is a single ID lookup or very constrained query.
	CostTrivial QueryCostLevel = iota
	// CostLight is a small query: few authors, small limit.
	CostLight
	// CostMedium is a moderate query: 11-100 authors or moderate complexity.
	CostMedium
	// CostHeavy is an expensive query: 101-1000 authors or no limit with wide scope.
	CostHeavy
	// CostExtreme is a very expensive query: 1000+ authors (follow-list queries).
	CostExtreme
)

// String returns a human-readable name for the cost level.
func (c QueryCostLevel) String() string {
	switch c {
	case CostTrivial:
		return "trivial"
	case CostLight:
		return "light"
	case CostMedium:
		return "medium"
	case CostHeavy:
		return "heavy"
	case CostExtreme:
		return "extreme"
	default:
		return "unknown"
	}
}

// QueryCost holds the computed cost of a query.
type QueryCost struct {
	Level       QueryCostLevel
	Multiplier  float64 // Delay multiplier (0.1 = cheap, 5.0 = expensive)
	AuthorCount int
	KindCount   int
	IDCount     int
	HasLimit    bool
	Limit       int
}

// ClassifyQuery computes the cost of a query based on filter parameters.
// It uses primitive types to avoid import cycles with nostr encoder packages.
func ClassifyQuery(authorCount, kindCount, idCount int, hasLimit bool, limit int) QueryCost {
	cost := QueryCost{
		AuthorCount: authorCount,
		KindCount:   kindCount,
		IDCount:     idCount,
		HasLimit:    hasLimit,
		Limit:       limit,
	}

	// ID-only queries are trivial (direct key lookups)
	if idCount > 0 && authorCount == 0 {
		cost.Level = CostTrivial
		cost.Multiplier = 0.1
		return cost
	}

	switch {
	case authorCount > 1000:
		cost.Level = CostExtreme
		cost.Multiplier = 5.0
	case authorCount > 100:
		cost.Level = CostHeavy
		cost.Multiplier = 2.5
	case authorCount > 10:
		cost.Level = CostMedium
		cost.Multiplier = 1.0
	case authorCount <= 10 && hasLimit && limit <= 100:
		cost.Level = CostLight
		cost.Multiplier = 0.5
	default:
		// No authors specified but also no IDs — could be a wide scan
		if !hasLimit {
			cost.Level = CostMedium
			cost.Multiplier = 1.0
		} else {
			cost.Level = CostLight
			cost.Multiplier = 0.5
		}
	}

	// A tight limit significantly reduces actual cost regardless of author count
	if hasLimit && limit > 0 && limit <= 10 && cost.Level > CostLight {
		cost.Level--
		cost.Multiplier *= 0.5
	}

	return cost
}
