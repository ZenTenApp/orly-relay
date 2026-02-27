package find

import (
	"fmt"
	"strconv"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/interfaces/signer"
)

// NewRegistrationProposal creates a new registration proposal event (kind 30100)
func NewRegistrationProposal(name, action string, signer signer.I) (*event.E, error) {
	// Validate and normalize name
	name = NormalizeName(name)
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Validate action
	if action != ActionRegister && action != ActionTransfer {
		return nil, fmt.Errorf("invalid action: must be %s or %s", ActionRegister, ActionTransfer)
	}

	// Create event
	ev := event.New()
	ev.Kind = KindRegistrationProposal
	ev.CreatedAt = timestamp.Now().V
	ev.Pubkey = signer.Pub()

	// Build tags
	tags := tag.NewS()
	tags.Append(tag.NewFromAny("d", name))
	tags.Append(tag.NewFromAny("action", action))

	// Add expiration tag (5 minutes from now)
	expiration := time.Now().Add(ProposalExpiry).Unix()
	tags.Append(tag.NewFromAny("expiration", strconv.FormatInt(expiration, 10)))

	ev.Tags = tags
	ev.Content = []byte{}

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	return ev, nil
}

// NewRegistrationProposalWithTransfer creates a transfer proposal with previous owner signature
func NewRegistrationProposalWithTransfer(name, prevOwner, prevSig string, signer signer.I) (*event.E, error) {
	// Create base proposal
	ev, err := NewRegistrationProposal(name, ActionTransfer, signer)
	if err != nil {
		return nil, err
	}

	// Add transfer-specific tags
	ev.Tags.Append(tag.NewFromAny("prev_owner", prevOwner))
	ev.Tags.Append(tag.NewFromAny("prev_sig", prevSig))

	// Re-sign after adding tags
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign transfer event: %w", err)
	}

	return ev, nil
}

// NewAttestation creates a new attestation event (kind 20100)
func NewAttestation(proposalID, decision string, weight int, reason, serviceURL string, signer signer.I) (*event.E, error) {
	// Validate decision
	if decision != DecisionApprove && decision != DecisionReject && decision != DecisionAbstain {
		return nil, fmt.Errorf("invalid decision: must be approve, reject, or abstain")
	}

	// Create event
	ev := event.New()
	ev.Kind = KindAttestation
	ev.CreatedAt = timestamp.Now().V
	ev.Pubkey = signer.Pub()

	// Build tags
	tags := tag.NewS()
	tags.Append(tag.NewFromAny("e", proposalID))
	tags.Append(tag.NewFromAny("decision", decision))

	if weight > 0 {
		tags.Append(tag.NewFromAny("weight", strconv.Itoa(weight)))
	}

	if reason != "" {
		tags.Append(tag.NewFromAny("reason", reason))
	}

	if serviceURL != "" {
		tags.Append(tag.NewFromAny("service", serviceURL))
	}

	// Add expiration tag (3 minutes from now)
	expiration := time.Now().Add(AttestationExpiry).Unix()
	tags.Append(tag.NewFromAny("expiration", strconv.FormatInt(expiration, 10)))

	ev.Tags = tags
	ev.Content = []byte{}

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign attestation: %w", err)
	}

	return ev, nil
}

// NewTrustGraphEvent creates a new trust graph event (kind 30101)
func NewTrustGraphEvent(entries []TrustEntry, signer signer.I) (*event.E, error) {
	// Validate trust entries
	for i, entry := range entries {
		if err := ValidateTrustScore(entry.TrustScore); err != nil {
			return nil, fmt.Errorf("invalid trust score at index %d: %w", i, err)
		}
	}

	// Create event
	ev := event.New()
	ev.Kind = KindTrustGraph
	ev.CreatedAt = timestamp.Now().V
	ev.Pubkey = signer.Pub()

	// Build tags
	tags := tag.NewS()
	tags.Append(tag.NewFromAny("d", "trust-graph"))

	// Add trust entries as p tags
	for _, entry := range entries {
		tags.Append(tag.NewFromAny("p", entry.Pubkey, entry.ServiceURL,
			strconv.FormatFloat(entry.TrustScore, 'f', 2, 64)))
	}

	// Add expiration tag (30 days from now)
	expiration := time.Now().Add(TrustGraphExpiry).Unix()
	tags.Append(tag.NewFromAny("expiration", strconv.FormatInt(expiration, 10)))

	ev.Tags = tags
	ev.Content = []byte{}

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign trust graph: %w", err)
	}

	return ev, nil
}

// NewNameState creates a new name state event (kind 30102)
func NewNameState(name, owner string, registeredAt time.Time, proposalID string,
	attestations int, confidence float64, signer signer.I) (*event.E, error) {

	// Validate name
	name = NormalizeName(name)
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Create event
	ev := event.New()
	ev.Kind = KindNameState
	ev.CreatedAt = timestamp.Now().V
	ev.Pubkey = signer.Pub()

	// Build tags
	tags := tag.NewS()
	tags.Append(tag.NewFromAny("d", name))
	tags.Append(tag.NewFromAny("owner", owner))
	tags.Append(tag.NewFromAny("registered_at", strconv.FormatInt(registeredAt.Unix(), 10)))
	tags.Append(tag.NewFromAny("proposal", proposalID))
	tags.Append(tag.NewFromAny("attestations", strconv.Itoa(attestations)))
	tags.Append(tag.NewFromAny("confidence", strconv.FormatFloat(confidence, 'f', 2, 64)))

	// Add expiration tag (1 year from registration)
	expiration := registeredAt.Add(NameRegistrationPeriod).Unix()
	tags.Append(tag.NewFromAny("expiration", strconv.FormatInt(expiration, 10)))

	ev.Tags = tags
	ev.Content = []byte{}

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign name state: %w", err)
	}

	return ev, nil
}

// NewNameRecord creates a new name record event (kind 30103)
func NewNameRecord(name, recordType, value string, ttl int, signer signer.I) (*event.E, error) {
	// Validate name
	name = NormalizeName(name)
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Validate record value
	if err := ValidateRecordValue(recordType, value); err != nil {
		return nil, err
	}

	// Create event
	ev := event.New()
	ev.Kind = KindNameRecords
	ev.CreatedAt = timestamp.Now().V
	ev.Pubkey = signer.Pub()

	// Build tags
	tags := tag.NewS()
	tags.Append(tag.NewFromAny("d", fmt.Sprintf("%s:%s", name, recordType)))
	tags.Append(tag.NewFromAny("name", name))
	tags.Append(tag.NewFromAny("type", recordType))
	tags.Append(tag.NewFromAny("value", value))

	if ttl > 0 {
		tags.Append(tag.NewFromAny("ttl", strconv.Itoa(ttl)))
	}

	ev.Tags = tags
	ev.Content = []byte{}

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign name record: %w", err)
	}

	return ev, nil
}

// NewNameRecordWithPriority creates a name record with priority (for MX, SRV)
func NewNameRecordWithPriority(name, recordType, value string, ttl, priority int, signer signer.I) (*event.E, error) {
	// Validate priority
	if err := ValidatePriority(priority); err != nil {
		return nil, err
	}

	// Create base record
	ev, err := NewNameRecord(name, recordType, value, ttl, signer)
	if err != nil {
		return nil, err
	}

	// Add priority tag
	ev.Tags.Append(tag.NewFromAny("priority", strconv.Itoa(priority)))

	// Re-sign
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign record with priority: %w", err)
	}

	return ev, nil
}

// NewSRVRecord creates an SRV record with all required fields
func NewSRVRecord(name, value string, ttl, priority, weight, port int, signer signer.I) (*event.E, error) {
	// Validate SRV-specific fields
	if err := ValidatePriority(priority); err != nil {
		return nil, err
	}
	if err := ValidateWeight(weight); err != nil {
		return nil, err
	}
	if err := ValidatePort(port); err != nil {
		return nil, err
	}

	// Create base record
	ev, err := NewNameRecord(name, RecordTypeSRV, value, ttl, signer)
	if err != nil {
		return nil, err
	}

	// Add SRV-specific tags
	ev.Tags.Append(tag.NewFromAny("priority", strconv.Itoa(priority)))
	ev.Tags.Append(tag.NewFromAny("weight", strconv.Itoa(weight)))
	ev.Tags.Append(tag.NewFromAny("port", strconv.Itoa(port)))

	// Re-sign
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign SRV record: %w", err)
	}

	return ev, nil
}

// NewCertificate creates a new certificate event (kind 30104)
func NewCertificate(name, certPubkey string, validFrom, validUntil time.Time,
	challenge, challengeProof string, witnesses []WitnessSignature,
	algorithm, usage string, signer signer.I) (*event.E, error) {

	// Validate name
	name = NormalizeName(name)
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Create event
	ev := event.New()
	ev.Kind = KindCertificate
	ev.CreatedAt = timestamp.Now().V
	ev.Pubkey = signer.Pub()

	// Build tags
	tags := tag.NewS()
	tags.Append(tag.NewFromAny("d", name))
	tags.Append(tag.NewFromAny("name", name))
	tags.Append(tag.NewFromAny("cert_pubkey", certPubkey))
	tags.Append(tag.NewFromAny("valid_from", strconv.FormatInt(validFrom.Unix(), 10)))
	tags.Append(tag.NewFromAny("valid_until", strconv.FormatInt(validUntil.Unix(), 10)))
	tags.Append(tag.NewFromAny("challenge", challenge))
	tags.Append(tag.NewFromAny("challenge_proof", challengeProof))

	// Add witness signatures
	for _, w := range witnesses {
		tags.Append(tag.NewFromAny("witness", w.Pubkey, w.Signature))
	}

	ev.Tags = tags

	// Add metadata to content
	content := fmt.Sprintf(`{"algorithm":"%s","usage":"%s"}`, algorithm, usage)
	ev.Content = []byte(content)

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	return ev, nil
}

// NewWitnessService creates a new witness service info event (kind 30105)
func NewWitnessService(endpoint string, challenges []string, maxValidity, fee int,
	reputationID, description, contact string, signer signer.I) (*event.E, error) {

	// Create event
	ev := event.New()
	ev.Kind = KindWitnessService
	ev.CreatedAt = timestamp.Now().V
	ev.Pubkey = signer.Pub()

	// Build tags
	tags := tag.NewS()
	tags.Append(tag.NewFromAny("d", "witness-service"))
	tags.Append(tag.NewFromAny("endpoint", endpoint))

	for _, ch := range challenges {
		tags.Append(tag.NewFromAny("challenges", ch))
	}

	if maxValidity > 0 {
		tags.Append(tag.NewFromAny("max_validity", strconv.Itoa(maxValidity)))
	}

	if fee > 0 {
		tags.Append(tag.NewFromAny("fee", strconv.Itoa(fee)))
	}

	if reputationID != "" {
		tags.Append(tag.NewFromAny("reputation", reputationID))
	}

	// Add expiration tag (180 days from now)
	expiration := time.Now().Add(WitnessServiceExpiry).Unix()
	tags.Append(tag.NewFromAny("expiration", strconv.FormatInt(expiration, 10)))

	ev.Tags = tags

	// Add metadata to content
	content := fmt.Sprintf(`{"description":"%s","contact":"%s"}`, description, contact)
	ev.Content = []byte(content)

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		return nil, fmt.Errorf("failed to sign witness service: %w", err)
	}

	return ev, nil
}
