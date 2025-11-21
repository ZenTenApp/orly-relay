package find

import (
	"fmt"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/hex"
)

// ConsensusEngine handles the consensus algorithm for name registrations
type ConsensusEngine struct {
	db             database.Database
	trustGraph     *TrustGraph
	threshold      float64 // Consensus threshold (e.g., 0.51 for 51%)
	minCoverage    float64 // Minimum trust graph coverage required
	conflictMargin float64 // Margin for declaring conflicts (e.g., 0.05 for 5%)
}

// NewConsensusEngine creates a new consensus engine
func NewConsensusEngine(db database.Database, trustGraph *TrustGraph) *ConsensusEngine {
	return &ConsensusEngine{
		db:             db,
		trustGraph:     trustGraph,
		threshold:      0.51, // 51% threshold
		minCoverage:    0.30, // 30% minimum coverage
		conflictMargin: 0.05, // 5% conflict margin
	}
}

// ProposalScore holds scoring information for a proposal
type ProposalScore struct {
	Proposal     *RegistrationProposal
	Score        float64
	Attestations []*Attestation
	Weights      map[string]float64 // Attester pubkey -> weighted score
}

// ConsensusResult represents the result of consensus computation
type ConsensusResult struct {
	Winner      *RegistrationProposal
	Score       float64
	Confidence  float64 // 0.0 to 1.0
	Attestations int
	Conflicted  bool
	Reason      string
}

// ComputeConsensus computes consensus for a set of competing proposals
func (ce *ConsensusEngine) ComputeConsensus(proposals []*RegistrationProposal, attestations []*Attestation) (*ConsensusResult, error) {
	if len(proposals) == 0 {
		return nil, errorf.E("no proposals to evaluate")
	}

	// Group attestations by proposal ID
	attestationMap := make(map[string][]*Attestation)
	for _, att := range attestations {
		if att.Decision == DecisionApprove {
			attestationMap[att.ProposalID] = append(attestationMap[att.ProposalID], att)
		}
	}

	// Score each proposal
	scores := make([]*ProposalScore, 0, len(proposals))
	totalWeight := 0.0

	for _, proposal := range proposals {
		proposalAtts := attestationMap[hex.Enc(proposal.Event.ID)]
		score, weights := ce.ScoreProposal(proposal, proposalAtts)

		scores = append(scores, &ProposalScore{
			Proposal:     proposal,
			Score:        score,
			Attestations: proposalAtts,
			Weights:      weights,
		})

		totalWeight += score
	}

	// Check if we have sufficient coverage
	if totalWeight < ce.minCoverage {
		return &ConsensusResult{
			Conflicted: true,
			Reason:     fmt.Sprintf("insufficient attestations: %.2f%% < %.2f%%", totalWeight*100, ce.minCoverage*100),
		}, nil
	}

	// Find highest scoring proposal
	var winner *ProposalScore
	for _, ps := range scores {
		if winner == nil || ps.Score > winner.Score {
			winner = ps
		}
	}

	// Calculate relative score
	relativeScore := winner.Score / totalWeight

	// Check for conflicts (multiple proposals within margin)
	conflicted := false
	for _, ps := range scores {
		if hex.Enc(ps.Proposal.Event.ID) != hex.Enc(winner.Proposal.Event.ID) {
			otherRelative := ps.Score / totalWeight
			if (relativeScore - otherRelative) < ce.conflictMargin {
				conflicted = true
				break
			}
		}
	}

	// Check if winner meets threshold
	if relativeScore < ce.threshold {
		return &ConsensusResult{
			Winner:      winner.Proposal,
			Score:       winner.Score,
			Confidence:  relativeScore,
			Attestations: len(winner.Attestations),
			Conflicted:  true,
			Reason:      fmt.Sprintf("score %.2f%% below threshold %.2f%%", relativeScore*100, ce.threshold*100),
		}, nil
	}

	// Check for conflicts
	if conflicted {
		return &ConsensusResult{
			Winner:      winner.Proposal,
			Score:       winner.Score,
			Confidence:  relativeScore,
			Attestations: len(winner.Attestations),
			Conflicted:  true,
			Reason:      "competing proposals within conflict margin",
		}, nil
	}

	// Success!
	return &ConsensusResult{
		Winner:      winner.Proposal,
		Score:       winner.Score,
		Confidence:  relativeScore,
		Attestations: len(winner.Attestations),
		Conflicted:  false,
		Reason:      "consensus reached",
	}, nil
}

// ScoreProposal computes the trust-weighted score for a proposal
func (ce *ConsensusEngine) ScoreProposal(proposal *RegistrationProposal, attestations []*Attestation) (float64, map[string]float64) {
	totalScore := 0.0
	weights := make(map[string]float64)

	for _, att := range attestations {
		if att.Decision != DecisionApprove {
			continue
		}

		// Get attestation weight (default 100)
		attWeight := float64(att.Weight)
		if attWeight <= 0 {
			attWeight = 100
		}

		// Get trust level for this attester
		trustLevel := ce.trustGraph.GetTrustLevel(att.Event.Pubkey)

		// Calculate weighted score
		// Score = attestation_weight * trust_level / 100
		score := (attWeight / 100.0) * trustLevel

		weights[hex.Enc(att.Event.Pubkey)] = score
		totalScore += score
	}

	return totalScore, weights
}

// ValidateProposal validates a registration proposal against current state
func (ce *ConsensusEngine) ValidateProposal(proposal *RegistrationProposal) error {
	// Validate name format
	if err := ValidateName(proposal.Name); err != nil {
		return errorf.E("invalid name format: %w", err)
	}

	// Check if proposal is expired
	if !proposal.Expiration.IsZero() && time.Now().After(proposal.Expiration) {
		return errorf.E("proposal expired at %v", proposal.Expiration)
	}

	// Validate subdomain authority (if applicable)
	if !IsTLD(proposal.Name) {
		parent := GetParentDomain(proposal.Name)
		if parent == "" {
			return errorf.E("invalid subdomain structure")
		}

		// Query parent domain ownership
		parentState, err := ce.QueryNameState(parent)
		if err != nil {
			return errorf.E("failed to query parent domain: %w", err)
		}

		if parentState == nil {
			return errorf.E("parent domain %s not registered", parent)
		}

		// Verify proposer owns parent domain
		proposerPubkey := hex.Enc(proposal.Event.Pubkey)
		if parentState.Owner != proposerPubkey {
			return errorf.E("proposer does not own parent domain %s", parent)
		}
	}

	// Validate against current name state
	nameState, err := ce.QueryNameState(proposal.Name)
	if err != nil {
		return errorf.E("failed to query name state: %w", err)
	}

	now := time.Now()

	// Name is not registered - anyone can register
	if nameState == nil {
		return nil
	}

	// Name is expired - anyone can register
	if !nameState.Expiration.IsZero() && now.After(nameState.Expiration) {
		return nil
	}

	// Calculate renewal window start (30 days before expiration)
	renewalStart := nameState.Expiration.Add(-PreferentialRenewalDays * 24 * time.Hour)

	// Before renewal window - reject all proposals
	if now.Before(renewalStart) {
		return errorf.E("name is currently owned and not in renewal window")
	}

	// During renewal window - only current owner can register
	if now.Before(nameState.Expiration) {
		proposerPubkey := hex.Enc(proposal.Event.Pubkey)
		if proposerPubkey != nameState.Owner {
			return errorf.E("only current owner can renew during preferential renewal window")
		}
		return nil
	}

	// Should not reach here, but allow registration if we do
	return nil
}

// ValidateTransfer validates a transfer proposal
func (ce *ConsensusEngine) ValidateTransfer(proposal *RegistrationProposal) error {
	if proposal.Action != ActionTransfer {
		return errorf.E("not a transfer proposal")
	}

	// Must have previous owner and signature
	if proposal.PrevOwner == "" {
		return errorf.E("missing previous owner")
	}
	if proposal.PrevSig == "" {
		return errorf.E("missing previous owner signature")
	}

	// Query current name state
	nameState, err := ce.QueryNameState(proposal.Name)
	if err != nil {
		return errorf.E("failed to query name state: %w", err)
	}

	if nameState == nil {
		return errorf.E("name not registered")
	}

	// Verify previous owner matches current owner
	if nameState.Owner != proposal.PrevOwner {
		return errorf.E("previous owner mismatch")
	}

	// Verify name is not expired
	if !nameState.Expiration.IsZero() && time.Now().After(nameState.Expiration) {
		return errorf.E("name expired")
	}

	// TODO: Verify signature over transfer message
	// Message format: "transfer:<name>:<new_owner_pubkey>:<timestamp>"

	return nil
}

// QueryNameState queries the current name state from the database
func (ce *ConsensusEngine) QueryNameState(name string) (*NameState, error) {
	// Query kind 30102 events with d tag = name
	filter := &struct {
		Kinds  []uint16
		DTags  []string
		Limit  int
	}{
		Kinds:  []uint16{KindNameState},
		DTags:  []string{name},
		Limit:  10,
	}

	// Note: This would use the actual database query method
	// For now, return nil to indicate not found
	// TODO: Implement actual database query
	_ = filter
	return nil, nil
}

// CreateNameState creates a name state event from consensus result
func (ce *ConsensusEngine) CreateNameState(result *ConsensusResult, registryPubkey []byte) (*NameState, error) {
	if result.Winner == nil {
		return nil, errorf.E("no winner in consensus result")
	}

	proposal := result.Winner

	return &NameState{
		Name:         proposal.Name,
		Owner:        hex.Enc(proposal.Event.Pubkey),
		RegisteredAt: time.Now(),
		ProposalID:   hex.Enc(proposal.Event.ID),
		Attestations: result.Attestations,
		Confidence:   result.Confidence,
		Expiration:   time.Now().Add(NameRegistrationPeriod),
	}, nil
}

// ProcessProposalBatch processes a batch of proposals and returns consensus results
func (ce *ConsensusEngine) ProcessProposalBatch(proposals []*RegistrationProposal, attestations []*Attestation) ([]*ConsensusResult, error) {
	// Group proposals by name
	proposalsByName := make(map[string][]*RegistrationProposal)
	for _, proposal := range proposals {
		proposalsByName[proposal.Name] = append(proposalsByName[proposal.Name], proposal)
	}

	results := make([]*ConsensusResult, 0)

	// Process each name's proposals independently
	for name, nameProposals := range proposalsByName {
		// Filter attestations for this name's proposals
		proposalIDs := make(map[string]bool)
		for _, p := range nameProposals {
			proposalIDs[hex.Enc(p.Event.ID)] = true
		}

		nameAttestations := make([]*Attestation, 0)
		for _, att := range attestations {
			if proposalIDs[att.ProposalID] {
				nameAttestations = append(nameAttestations, att)
			}
		}

		// Compute consensus for this name
		result, err := ce.ComputeConsensus(nameProposals, nameAttestations)
		if chk.E(err) {
			// Log error but continue processing other names
			result = &ConsensusResult{
				Conflicted: true,
				Reason:     fmt.Sprintf("error: %v", err),
			}
		}

		// Add name to result for tracking
		if result.Winner != nil {
			result.Winner.Name = name
		}

		results = append(results, result)
	}

	return results, nil
}
