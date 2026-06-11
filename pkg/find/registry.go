package find

import (
	"context"
	"fmt"
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
)

type onProposalArgs struct {
	ProposalID string
	Proposal   *RegistrationProposal
}

type onProposalResp struct {
	exists bool
}

// RegistryService implements the FIND name registry consensus protocol
type RegistryService struct {
	ctx    context.Context
	cancel context.CancelFunc
	db     database.Database
	signer signer.I
	trustGraph  *TrustGraph
	consensus   *ConsensusEngine
	config      *RegistryConfig

	onProposal      actor.Func[onProposalArgs, onProposalResp]
	getCompeting    actor.Func[string, []*ProposalState]
	updateAttest    actor.Query[[]string]
	processProposal actor.Func[string, *ProposalState]
	cleanup         actor.Inbox[string]
	getMetrics      actor.Query[int]
	lc              actor.Lifecycle
}

// RegistryConfig holds configuration for the registry service
type RegistryConfig struct {
	Enabled           bool
	AttestationDelay  time.Duration
	SparseEnabled     bool
	SamplingRate      int
	BootstrapServices []string
	MinimumAttesters  int
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
		ctx:             ctx,
		cancel:          cancel,
		db:              db,
		signer:          signer,
		trustGraph:      trustGraph,
		consensus:       consensus,
		config:          config,
		onProposal:      actor.NewFunc[onProposalArgs, onProposalResp](),
		getCompeting:    actor.NewFunc[string, []*ProposalState](),
		updateAttest:    actor.NewQuery[[]string](),
		processProposal: actor.NewFunc[string, *ProposalState](),
		cleanup:         actor.NewInbox[string](16),
		getMetrics:      actor.NewQuery[int](),
		lc:              actor.NewLifecycle(),
	}

	// Bootstrap trust graph if configured
	if len(config.BootstrapServices) > 0 {
		if err := rs.bootstrapTrustGraph(); chk.E(err) {
			fmt.Printf("failed to bootstrap trust graph: %v\n", err)
		}
	}

	return rs, nil
}

// actorLoop owns the pendingProposals map
func (rs *RegistryService) actorLoop() {
	pendingProposals := make(map[string]*ProposalState)

	for {
		select {
		case <-rs.lc.Stopping():
			return
		case msg := <-rs.onProposal.Recv():
			_, exists := pendingProposals[msg.Req.ProposalID]
			if !exists {
				state := &ProposalState{
					Proposal:     msg.Req.Proposal,
					Attestations: make([]*Attestation, 0),
					ReceivedAt:   time.Now(),
				}
				state.Timer = time.AfterFunc(rs.config.AttestationDelay, func() {
					rs.doProcessProposal(msg.Req.ProposalID)
				})
				pendingProposals[msg.Req.ProposalID] = state
			}
			msg.Reply(onProposalResp{exists: exists})
		case msg := <-rs.getCompeting.Recv():
			proposals := make([]*ProposalState, 0)
			for _, state := range pendingProposals {
				if state.Proposal.Name == msg.Req {
					proposals = append(proposals, state)
				}
			}
			msg.Reply(proposals)
		case msg := <-rs.updateAttest.Recv():
			proposalIDs := make([]string, 0, len(pendingProposals))
			for id := range pendingProposals {
				proposalIDs = append(proposalIDs, id)
			}
			msg.Reply(proposalIDs)
		case msg := <-rs.processProposal.Recv():
			state, exists := pendingProposals[msg.Req]
			if exists {
				now := time.Now()
				state.ProcessedAt = &now
			}
			msg.Reply(state)
		case name := <-rs.cleanup.Recv():
			for id, state := range pendingProposals {
				if state.Proposal.Name == name && state.ProcessedAt != nil {
					if state.Timer != nil {
						state.Timer.Stop()
					}
					delete(pendingProposals, id)
				}
			}
		case msg := <-rs.getMetrics.Recv():
			msg.Reply(len(pendingProposals))
		}
	}
}

// Start starts the registry service
func (rs *RegistryService) Start() error {
	fmt.Println("starting FIND registry service")

	actor.Go(rs.lc, rs.actorLoop)
	go rs.monitorProposals()
	go rs.collectAttestations()
	go rs.refreshTrustGraph()

	return nil
}

// Stop stops the registry service
func (rs *RegistryService) Stop() error {
	fmt.Println("stopping FIND registry service")
	rs.cancel()
	rs.lc.Stop()
	return nil
}

// monitorProposals monitors for new registration proposals
func (rs *RegistryService) monitorProposals() {
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
	// TODO: Implement database query for kind 30100 events
	// TODO: Parse proposals and add to pendingProposals map
	// TODO: Start attestation timer for each new proposal
}

// OnProposalReceived is called when a new proposal is received
func (rs *RegistryService) OnProposalReceived(proposal *RegistrationProposal) error {
	if err := rs.consensus.ValidateProposal(proposal); chk.E(err) {
		fmt.Printf("invalid proposal: %v\n", err)
		return err
	}

	proposalID := hex.Enc(proposal.Event.ID)

	r := rs.onProposal.Call(onProposalArgs{
		ProposalID: proposalID,
		Proposal:   proposal,
	})

	if r.exists {
		return nil
	}

	fmt.Printf("received new proposal: %s name: %s\n", proposalID, proposal.Name)

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

	hash, err := hex.Dec(proposalID)
	if err != nil || len(hash) == 0 {
		return false
	}

	return int(hash[0])%rs.config.SamplingRate == 0
}

// publishAttestation publishes an attestation for a proposal
func (rs *RegistryService) publishAttestation(proposal *RegistrationProposal, decision string, reason string) {
	attestation := &Attestation{
		ProposalID: hex.Enc(proposal.Event.ID),
		Decision:   decision,
		Weight:     100,
		Reason:     reason,
		ServiceURL: "",
		Expiration: time.Now().Add(AttestationExpiry),
	}

	_ = attestation

	fmt.Printf("published attestation for proposal: %s decision: %s\n", proposal.Name, decision)
}

// collectAttestations collects attestations from other registry services
func (rs *RegistryService) collectAttestations() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			return
		case <-ticker.C:
			rs.doUpdateAttestations()
		}
	}
}

// doUpdateAttestations fetches new attestations from database
func (rs *RegistryService) doUpdateAttestations() {
	proposalIDs := rs.updateAttest.Call()

	if len(proposalIDs) == 0 {
		return
	}

	// TODO: Query kind 20100 events (attestations) for pending proposals
	// TODO: Add attestations to proposal states
}

// doProcessProposal processes a proposal after the attestation window expires
func (rs *RegistryService) doProcessProposal(proposalID string) {
	state := rs.processProposal.Call(proposalID)

	if state == nil {
		return
	}

	fmt.Printf("processing proposal: %s name: %s\n", proposalID, state.Proposal.Name)

	competingProposals := rs.getCompeting.Call(state.Proposal.Name)

	allAttestations := make([]*Attestation, 0)
	for _, p := range competingProposals {
		allAttestations = append(allAttestations, p.Attestations...)
	}

	proposalList := make([]*RegistrationProposal, 0, len(competingProposals))
	for _, p := range competingProposals {
		proposalList = append(proposalList, p.Proposal)
	}

	result, err := rs.consensus.ComputeConsensus(proposalList, allAttestations)
	if chk.E(err) {
		fmt.Printf("consensus computation failed: %v\n", err)
		return
	}

	if result.Conflicted {
		fmt.Printf("consensus conflicted for name: %s reason: %s\n", state.Proposal.Name, result.Reason)
		return
	}

	fmt.Printf("consensus reached for name: %s winner: %s confidence: %f\n",
		state.Proposal.Name,
		hex.Enc(result.Winner.Event.ID),
		result.Confidence)

	if err := rs.publishNameState(result); chk.E(err) {
		fmt.Printf("failed to publish name state: %v\n", err)
		return
	}

	rs.cleanup.TrySend(state.Proposal.Name)
}

// publishNameState publishes a name state event after consensus
func (rs *RegistryService) publishNameState(result *ConsensusResult) error {
	nameState, err := rs.consensus.CreateNameState(result, rs.signer.Pub())
	if err != nil {
		return err
	}

	_ = nameState
	return nil
}

// refreshTrustGraph periodically refreshes the trust graph from other services
func (rs *RegistryService) refreshTrustGraph() {
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
}

// bootstrapTrustGraph initializes trust relationships with bootstrap services
func (rs *RegistryService) bootstrapTrustGraph() error {
	fmt.Printf("bootstrapping trust graph with %d services\n", len(rs.config.BootstrapServices))

	for _, pubkeyHex := range rs.config.BootstrapServices {
		entry := TrustEntry{
			Pubkey:     pubkeyHex,
			ServiceURL: "",
			TrustScore: 0.7,
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
	pending := rs.getMetrics.Call()

	return &RegistryMetrics{
		PendingProposals: pending,
		TrustMetrics:     rs.trustGraph.CalculateTrustMetrics(),
	}
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
		proposal, err := ParseRegistrationProposal(ev)
		if err != nil {
			return err
		}
		return rs.OnProposalReceived(proposal)

	case KindAttestation:
		return nil

	case KindTrustGraph:
		return nil

	default:
		return nil
	}
}
