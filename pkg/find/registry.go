package find

import (
	"context"
	"fmt"
	"sync"
	"time"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer"
)

// RegistryService implements the FIND name registry consensus protocol
type RegistryService struct {
	ctx              context.Context
	cancel           context.CancelFunc
	db               database.Database
	signer           signer.I
	trustGraph       *TrustGraph
	consensus        *ConsensusEngine
	config           *RegistryConfig
	pendingProposals map[string]*ProposalState
	mu               sync.RWMutex
	wg               sync.WaitGroup
}

// RegistryConfig holds configuration for the registry service
type RegistryConfig struct {
	Enabled            bool
	AttestationDelay   time.Duration
	SparseEnabled      bool
	SamplingRate       int
	BootstrapServices  []string
	MinimumAttesters   int
}

// ProposalState tracks a proposal during its attestation window
type ProposalState struct {
	Proposal     *RegistrationProposal
	Attestations []*Attestation
	ReceivedAt   time.Time
	ProcessedAt  *time.Time
	Timer        *time.Timer
}

// NewRegistryService creates a new registry service
func NewRegistryService(ctx context.Context, db database.Database, signer signer.I, config *RegistryConfig) (*RegistryService, error) {
	if !config.Enabled {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)

	trustGraph := NewTrustGraph(signer.Pub())
	consensus := NewConsensusEngine(db, trustGraph)

	rs := &RegistryService{
		ctx:              ctx,
		cancel:           cancel,
		db:               db,
		signer:           signer,
		trustGraph:       trustGraph,
		consensus:        consensus,
		config:           config,
		pendingProposals: make(map[string]*ProposalState),
	}

	// Bootstrap trust graph if configured
	if len(config.BootstrapServices) > 0 {
		if err := rs.bootstrapTrustGraph(); chk.E(err) {
			fmt.Printf("failed to bootstrap trust graph: %v\n", err)
		}
	}

	return rs, nil
}

// Start starts the registry service
func (rs *RegistryService) Start() error {
	fmt.Println("starting FIND registry service")

	// Start proposal monitoring goroutine
	rs.wg.Add(1)
	go rs.monitorProposals()

	// Start attestation collection goroutine
	rs.wg.Add(1)
	go rs.collectAttestations()

	// Start trust graph refresh goroutine
	rs.wg.Add(1)
	go rs.refreshTrustGraph()

	return nil
}

// Stop stops the registry service
func (rs *RegistryService) Stop() error {
	fmt.Println("stopping FIND registry service")

	rs.cancel()
	rs.wg.Wait()

	return nil
}

// monitorProposals monitors for new registration proposals
func (rs *RegistryService) monitorProposals() {
	defer rs.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			return
		case <-ticker.C:
			rs.checkForNewProposals()
		}
	}
}

// checkForNewProposals checks database for new registration proposals
func (rs *RegistryService) checkForNewProposals() {
	// Query recent kind 30100 events (registration proposals)
	// This would use the actual database query API
	// For now, this is a stub

	// TODO: Implement database query for kind 30100 events
	// TODO: Parse proposals and add to pendingProposals map
	// TODO: Start attestation timer for each new proposal
}

// OnProposalReceived is called when a new proposal is received
func (rs *RegistryService) OnProposalReceived(proposal *RegistrationProposal) error {
	// Validate proposal
	if err := rs.consensus.ValidateProposal(proposal); chk.E(err) {
		fmt.Printf("invalid proposal: %v\n", err)
		return err
	}

	proposalID := hex.Enc(proposal.Event.ID)

	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Check if already processing
	if _, exists := rs.pendingProposals[proposalID]; exists {
		return nil
	}

	fmt.Printf("received new proposal: %s name: %s\n", proposalID, proposal.Name)

	// Create proposal state
	state := &ProposalState{
		Proposal:     proposal,
		Attestations: make([]*Attestation, 0),
		ReceivedAt:   time.Now(),
	}

	// Start attestation timer
	state.Timer = time.AfterFunc(rs.config.AttestationDelay, func() {
		rs.processProposal(proposalID)
	})

	rs.pendingProposals[proposalID] = state

	// Publish attestation (if not using sparse or if dice roll succeeds)
	if rs.shouldAttest(proposalID) {
		go rs.publishAttestation(proposal, DecisionApprove, "valid_proposal")
	}

	return nil
}

// shouldAttest determines if this service should attest to a proposal
func (rs *RegistryService) shouldAttest(proposalID string) bool {
	if !rs.config.SparseEnabled {
		return true
	}

	// Sparse attestation: use hash of (proposal_id || service_pubkey) % K == 0
	// This provides deterministic but distributed attestation
	hash, err := hex.Dec(proposalID)
	if err != nil || len(hash) == 0 {
		return false
	}

	// Simple modulo check using first byte of hash
	return int(hash[0])%rs.config.SamplingRate == 0
}

// publishAttestation publishes an attestation for a proposal
func (rs *RegistryService) publishAttestation(proposal *RegistrationProposal, decision string, reason string) {
	attestation := &Attestation{
		ProposalID: hex.Enc(proposal.Event.ID),
		Decision:   decision,
		Weight:     100,
		Reason:     reason,
		ServiceURL: "", // TODO: Get from config
		Expiration: time.Now().Add(AttestationExpiry),
	}

	// TODO: Create and sign attestation event (kind 20100)
	// TODO: Publish to database
	_ = attestation

	fmt.Printf("published attestation for proposal: %s decision: %s\n", proposal.Name, decision)
}

// collectAttestations collects attestations from other registry services
func (rs *RegistryService) collectAttestations() {
	defer rs.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			return
		case <-ticker.C:
			rs.updateAttestations()
		}
	}
}

// updateAttestations fetches new attestations from database
func (rs *RegistryService) updateAttestations() {
	rs.mu.RLock()
	proposalIDs := make([]string, 0, len(rs.pendingProposals))
	for id := range rs.pendingProposals {
		proposalIDs = append(proposalIDs, id)
	}
	rs.mu.RUnlock()

	if len(proposalIDs) == 0 {
		return
	}

	// TODO: Query kind 20100 events (attestations) for pending proposals
	// TODO: Add attestations to proposal states
}

// processProposal processes a proposal after the attestation window expires
func (rs *RegistryService) processProposal(proposalID string) {
	rs.mu.Lock()
	state, exists := rs.pendingProposals[proposalID]
	if !exists {
		rs.mu.Unlock()
		return
	}

	// Mark as processed
	now := time.Now()
	state.ProcessedAt = &now
	rs.mu.Unlock()

	fmt.Printf("processing proposal: %s name: %s\n", proposalID, state.Proposal.Name)

	// Check for competing proposals for the same name
	competingProposals := rs.getCompetingProposals(state.Proposal.Name)

	// Gather all attestations
	allAttestations := make([]*Attestation, 0)
	for _, p := range competingProposals {
		allAttestations = append(allAttestations, p.Attestations...)
	}

	// Compute consensus
	proposalList := make([]*RegistrationProposal, 0, len(competingProposals))
	for _, p := range competingProposals {
		proposalList = append(proposalList, p.Proposal)
	}

	result, err := rs.consensus.ComputeConsensus(proposalList, allAttestations)
	if chk.E(err) {
		fmt.Printf("consensus computation failed: %v\n", err)
		return
	}

	// Log result
	if result.Conflicted {
		fmt.Printf("consensus conflicted for name: %s reason: %s\n", state.Proposal.Name, result.Reason)
		return
	}

	fmt.Printf("consensus reached for name: %s winner: %s confidence: %f\n",
		state.Proposal.Name,
		hex.Enc(result.Winner.Event.ID),
		result.Confidence)

	// Publish name state (kind 30102)
	if err := rs.publishNameState(result); chk.E(err) {
		fmt.Printf("failed to publish name state: %v\n", err)
		return
	}

	// Clean up processed proposals
	rs.cleanupProposals(state.Proposal.Name)
}

// getCompetingProposals returns all pending proposals for the same name
func (rs *RegistryService) getCompetingProposals(name string) []*ProposalState {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	proposals := make([]*ProposalState, 0)
	for _, state := range rs.pendingProposals {
		if state.Proposal.Name == name {
			proposals = append(proposals, state)
		}
	}

	return proposals
}

// publishNameState publishes a name state event after consensus
func (rs *RegistryService) publishNameState(result *ConsensusResult) error {
	nameState, err := rs.consensus.CreateNameState(result, rs.signer.Pub())
	if err != nil {
		return err
	}

	// TODO: Create kind 30102 event
	// TODO: Sign with registry service key
	// TODO: Publish to database
	_ = nameState

	return nil
}

// cleanupProposals removes processed proposals from the pending map
func (rs *RegistryService) cleanupProposals(name string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for id, state := range rs.pendingProposals {
		if state.Proposal.Name == name && state.ProcessedAt != nil {
			// Cancel timer if still running
			if state.Timer != nil {
				state.Timer.Stop()
			}
			delete(rs.pendingProposals, id)
		}
	}
}

// refreshTrustGraph periodically refreshes the trust graph from other services
func (rs *RegistryService) refreshTrustGraph() {
	defer rs.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			return
		case <-ticker.C:
			rs.updateTrustGraph()
		}
	}
}

// updateTrustGraph fetches trust graphs from other services
func (rs *RegistryService) updateTrustGraph() {
	fmt.Println("updating trust graph")

	// TODO: Query kind 30101 events (trust graphs) from database
	// TODO: Parse and update trust graph
	// TODO: Remove expired trust graphs
}

// bootstrapTrustGraph initializes trust relationships with bootstrap services
func (rs *RegistryService) bootstrapTrustGraph() error {
	fmt.Printf("bootstrapping trust graph with %d services\n", len(rs.config.BootstrapServices))

	for _, pubkeyHex := range rs.config.BootstrapServices {
		entry := TrustEntry{
			Pubkey:     pubkeyHex,
			ServiceURL: "",
			TrustScore: 0.7, // Medium trust for bootstrap services
		}

		if err := rs.trustGraph.AddEntry(entry); chk.E(err) {
			fmt.Printf("failed to add bootstrap trust entry: %v\n", err)
			continue
		}
	}

	return nil
}

// GetTrustGraph returns the current trust graph
func (rs *RegistryService) GetTrustGraph() *TrustGraph {
	return rs.trustGraph
}

// GetMetrics returns registry service metrics
func (rs *RegistryService) GetMetrics() *RegistryMetrics {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	metrics := &RegistryMetrics{
		PendingProposals: len(rs.pendingProposals),
		TrustMetrics:     rs.trustGraph.CalculateTrustMetrics(),
	}

	return metrics
}

// RegistryMetrics holds metrics about the registry service
type RegistryMetrics struct {
	PendingProposals int
	TrustMetrics     *TrustMetrics
}

// QueryNameOwnership queries the ownership state of a name
func (rs *RegistryService) QueryNameOwnership(name string) (*NameState, error) {
	return rs.consensus.QueryNameState(name)
}

// ValidateProposal validates a proposal without adding it to pending
func (rs *RegistryService) ValidateProposal(proposal *RegistrationProposal) error {
	return rs.consensus.ValidateProposal(proposal)
}

// HandleEvent processes incoming FIND-related events
func (rs *RegistryService) HandleEvent(ev *event.E) error {
	switch ev.Kind {
	case KindRegistrationProposal:
		// Parse proposal
		proposal, err := ParseRegistrationProposal(ev)
		if err != nil {
			return err
		}
		return rs.OnProposalReceived(proposal)

	case KindAttestation:
		// Parse attestation
		// TODO: Implement attestation parsing and handling
		return nil

	case KindTrustGraph:
		// Parse trust graph
		// TODO: Implement trust graph parsing and integration
		return nil

	default:
		return nil
	}
}
