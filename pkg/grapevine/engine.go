package grapevine

import (
	"encoding/json"
	"math"
	"time"

	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

// Engine computes GrapeVine influence and WoT intersection scores.
type Engine struct {
	graph GraphSource
	store ScoreStore
	cfg   Config
}

// NewEngine creates a new scoring engine.
func NewEngine(graph GraphSource, store ScoreStore, cfg Config) *Engine {
	return &Engine{graph: graph, store: store, cfg: cfg}
}

// Compute calculates influence and WoT scores for the given observer.
// The result is persisted via the ScoreStore and also returned.
func (e *Engine) Compute(observerHex string) (*ScoreSet, error) {
	start := time.Now()

	// Decode observer hex to binary for graph traversal
	seedPubkey, err := hex.Dec(observerHex)
	if err != nil {
		return nil, err
	}

	// BFS outward from observer to get the full hop set
	pubkeysByDepth, allHop, err := e.graph.TraverseFollowsOutbound(seedPubkey, e.cfg.MaxDepth)
	if err != nil {
		return nil, err
	}
	_ = pubkeysByDepth // depth info captured in allHop values

	if len(allHop) == 0 {
		return &ScoreSet{
			ObserverHex:  observerHex,
			ComputedAt:   time.Now(),
			ComputeMs:    time.Since(start).Milliseconds(),
			TotalPubkeys: 0,
		}, nil
	}

	log.I.F("grapevine: computing scores for observer %s, hop set size: %d", observerHex[:12], len(allHop))

	// Initialize influence scores
	infScores := make(map[string]float64, len(allHop))
	avgScores := make(map[string]float64, len(allHop))
	certaintyScores := make(map[string]float64, len(allHop))
	inputScores := make(map[string]float64, len(allHop))

	// Observer starts at 1.0, everyone else at 0.0
	infScores[observerHex] = 1.0
	avgScores[observerHex] = 1.0
	inputScores[observerHex] = 9999
	certaintyScores[observerHex] = 1.0

	// Cache follower lookups within this computation since the graph is static
	followerCache := make(map[string][]string, len(allHop))
	getFollowers := func(pk string) []string {
		if cached, ok := followerCache[pk]; ok {
			return cached
		}
		followers, err := e.graph.GetFollowerPubkeys(pk)
		if err != nil {
			followers = nil
		}
		followerCache[pk] = followers
		return followers
	}

	// Convergence loop
	rigority := -math.Log(e.cfg.Rigor)
	for cycle := 0; cycle < e.cfg.Cycles; cycle++ {
		for ratee := range allHop {
			if ratee == observerHex {
				continue
			}

			sumOfWeights := 0.0
			sumOfProducts := 0.0

			followers := getFollowers(ratee)
			for _, rater := range followers {
				if rater == ratee {
					continue
				}
				// Only consider raters within the hop set
				if _, inHop := allHop[rater]; !inHop {
					continue
				}

				rating := 1.0 // followInterpretationScore
				weight := e.cfg.AttenuationFactor * infScores[rater] * e.cfg.FollowConfidence
				if rater == observerHex {
					// No attenuation for observer's own follows
					weight = infScores[rater] * e.cfg.FollowConfidence
				}

				// TODO: mutes - skip muted pubkeys, apply muteInterpretationScore
				// TODO: reports - apply report penalty weight

				product := weight * rating
				sumOfWeights += weight
				sumOfProducts += product
			}

			if sumOfWeights > 0 {
				average := sumOfProducts / sumOfWeights
				input := sumOfWeights
				certainty := 1 - math.Exp(-input*rigority)
				influence := average * certainty

				infScores[ratee] = influence
				avgScores[ratee] = average
				certaintyScores[ratee] = certainty
				inputScores[ratee] = input
			}
		}
		log.D.F("grapevine: convergence cycle %d/%d complete", cycle+1, e.cfg.Cycles)
	}

	// WoT intersection scores: for each target, count how many of
	// observer's direct follows also follow the target
	observerFollows, err := e.graph.GetFollowsPubkeys(observerHex)
	if err != nil {
		observerFollows = nil
	}
	observerFollowSet := make(map[string]struct{}, len(observerFollows))
	for _, f := range observerFollows {
		observerFollowSet[f] = struct{}{}
	}

	wotScores := make(map[string]int, len(allHop))
	for target := range allHop {
		followers := getFollowers(target)
		count := 0
		for _, follower := range followers {
			if _, ok := observerFollowSet[follower]; ok {
				count++
			}
		}
		wotScores[target] = count
	}

	// Build score entries
	scores := make([]ScoreEntry, 0, len(allHop))
	for pk, depth := range allHop {
		inf := infScores[pk]
		if inf <= 0 && wotScores[pk] <= 0 && pk != observerHex {
			continue // skip zero-score entries to keep payload smaller
		}
		scores = append(scores, ScoreEntry{
			PubkeyHex: pk,
			Influence: inf,
			Average:   avgScores[pk],
			Certainty: certaintyScores[pk],
			Input:     inputScores[pk],
			WoTScore:  wotScores[pk],
			Depth:     depth,
		})
	}

	set := &ScoreSet{
		ObserverHex:  observerHex,
		Scores:       scores,
		ComputedAt:   time.Now(),
		ComputeMs:    time.Since(start).Milliseconds(),
		TotalPubkeys: len(allHop),
	}

	// Marshal for storage
	setData, err := json.Marshal(set)
	if err != nil {
		return set, err
	}
	entries := make(map[string][]byte, len(scores))
	for i := range scores {
		entryData, err := json.Marshal(&scores[i])
		if err != nil {
			continue
		}
		entries[scores[i].PubkeyHex] = entryData
	}
	if err := e.store.SaveScoreSet(set.ObserverHex, setData, entries); err != nil {
		return set, err
	}

	log.I.F("grapevine: computed %d scores for observer %s in %dms",
		len(scores), observerHex[:12], set.ComputeMs)

	return set, nil
}

// GetScores returns the stored score set for an observer, or nil if not computed.
func (e *Engine) GetScores(observerHex string) (*ScoreSet, error) {
	data, err := e.store.GetScoreSet(observerHex)
	if err != nil || data == nil {
		return nil, err
	}
	var set ScoreSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, err
	}
	return &set, nil
}

// GetScore returns a single stored score entry, or nil if not found.
func (e *Engine) GetScore(observerHex, targetHex string) (*ScoreEntry, error) {
	data, err := e.store.GetScore(observerHex, targetHex)
	if err != nil || data == nil {
		return nil, err
	}
	var entry ScoreEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}
