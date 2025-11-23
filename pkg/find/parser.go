package find

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// getTagValue retrieves the value of the first tag with the given key
func getTagValue(ev *event.E, key string) string {
	t := ev.Tags.GetFirst([]byte(key))
	if t == nil {
		return ""
	}
	return string(t.Value())
}

// getAllTags retrieves all tags with the given key
func getAllTags(ev *event.E, key string) []*tag.T {
	return ev.Tags.GetAll([]byte(key))
}

// ParseRegistrationProposal parses a kind 30100 event into a RegistrationProposal
func ParseRegistrationProposal(ev *event.E) (*RegistrationProposal, error) {
	if uint16(ev.Kind) != KindRegistrationProposal {
		return nil, fmt.Errorf("invalid event kind: expected %d, got %d", KindRegistrationProposal, ev.Kind)
	}

	name := getTagValue(ev, "d")
	if name == "" {
		return nil, fmt.Errorf("missing 'd' tag (name)")
	}

	action := getTagValue(ev, "action")
	if action == "" {
		return nil, fmt.Errorf("missing 'action' tag")
	}

	expirationStr := getTagValue(ev, "expiration")
	var expiration time.Time
	if expirationStr != "" {
		expirationUnix, err := strconv.ParseInt(expirationStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration timestamp: %w", err)
		}
		expiration = time.Unix(expirationUnix, 0)
	}

	proposal := &RegistrationProposal{
		Event:      ev,
		Name:       name,
		Action:     action,
		PrevOwner:  getTagValue(ev, "prev_owner"),
		PrevSig:    getTagValue(ev, "prev_sig"),
		Expiration: expiration,
	}

	return proposal, nil
}

// ParseAttestation parses a kind 20100 event into an Attestation
func ParseAttestation(ev *event.E) (*Attestation, error) {
	if uint16(ev.Kind) != KindAttestation {
		return nil, fmt.Errorf("invalid event kind: expected %d, got %d", KindAttestation, ev.Kind)
	}

	proposalID := getTagValue(ev, "e")
	if proposalID == "" {
		return nil, fmt.Errorf("missing 'e' tag (proposal ID)")
	}

	decision := getTagValue(ev, "decision")
	if decision == "" {
		return nil, fmt.Errorf("missing 'decision' tag")
	}

	weightStr := getTagValue(ev, "weight")
	weight := 100 // default weight
	if weightStr != "" {
		w, err := strconv.Atoi(weightStr)
		if err != nil {
			return nil, fmt.Errorf("invalid weight value: %w", err)
		}
		weight = w
	}

	expirationStr := getTagValue(ev, "expiration")
	var expiration time.Time
	if expirationStr != "" {
		expirationUnix, err := strconv.ParseInt(expirationStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration timestamp: %w", err)
		}
		expiration = time.Unix(expirationUnix, 0)
	}

	attestation := &Attestation{
		Event:      ev,
		ProposalID: proposalID,
		Decision:   decision,
		Weight:     weight,
		Reason:     getTagValue(ev, "reason"),
		ServiceURL: getTagValue(ev, "service"),
		Expiration: expiration,
	}

	return attestation, nil
}

// ParseTrustGraph parses a kind 30101 event into a TrustGraphEvent
func ParseTrustGraph(ev *event.E) (*TrustGraphEvent, error) {
	if uint16(ev.Kind) != KindTrustGraph {
		return nil, fmt.Errorf("invalid event kind: expected %d, got %d", KindTrustGraph, ev.Kind)
	}

	expirationStr := getTagValue(ev, "expiration")
	var expiration time.Time
	if expirationStr != "" {
		expirationUnix, err := strconv.ParseInt(expirationStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration timestamp: %w", err)
		}
		expiration = time.Unix(expirationUnix, 0)
	}

	// Parse p tags (trust entries)
	var entries []TrustEntry
	pTags := getAllTags(ev, "p")
	for _, t := range pTags {
		if len(t.T) < 2 {
			continue // Skip malformed tags
		}

		pubkey := string(t.T[1])
		serviceURL := ""
		trustScore := 0.5 // default

		if len(t.T) > 2 {
			serviceURL = string(t.T[2])
		}

		if len(t.T) > 3 {
			score, err := strconv.ParseFloat(string(t.T[3]), 64)
			if err == nil {
				trustScore = score
			}
		}

		entries = append(entries, TrustEntry{
			Pubkey:     pubkey,
			ServiceURL: serviceURL,
			TrustScore: trustScore,
		})
	}

	return &TrustGraphEvent{
		Event:      ev,
		Entries:    entries,
		Expiration: expiration,
	}, nil
}

// ParseNameState parses a kind 30102 event into a NameState
func ParseNameState(ev *event.E) (*NameState, error) {
	if uint16(ev.Kind) != KindNameState {
		return nil, fmt.Errorf("invalid event kind: expected %d, got %d", KindNameState, ev.Kind)
	}

	name := getTagValue(ev, "d")
	if name == "" {
		return nil, fmt.Errorf("missing 'd' tag (name)")
	}

	owner := getTagValue(ev, "owner")
	if owner == "" {
		return nil, fmt.Errorf("missing 'owner' tag")
	}

	registeredAtStr := getTagValue(ev, "registered_at")
	if registeredAtStr == "" {
		return nil, fmt.Errorf("missing 'registered_at' tag")
	}
	registeredAtUnix, err := strconv.ParseInt(registeredAtStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid registered_at timestamp: %w", err)
	}
	registeredAt := time.Unix(registeredAtUnix, 0)

	attestationsStr := getTagValue(ev, "attestations")
	attestations := 0
	if attestationsStr != "" {
		a, err := strconv.Atoi(attestationsStr)
		if err == nil {
			attestations = a
		}
	}

	confidenceStr := getTagValue(ev, "confidence")
	confidence := 0.0
	if confidenceStr != "" {
		c, err := strconv.ParseFloat(confidenceStr, 64)
		if err == nil {
			confidence = c
		}
	}

	expirationStr := getTagValue(ev, "expiration")
	var expiration time.Time
	if expirationStr != "" {
		expirationUnix, err := strconv.ParseInt(expirationStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration timestamp: %w", err)
		}
		expiration = time.Unix(expirationUnix, 0)
	}

	return &NameState{
		Event:        ev,
		Name:         name,
		Owner:        owner,
		RegisteredAt: registeredAt,
		ProposalID:   getTagValue(ev, "proposal"),
		Attestations: attestations,
		Confidence:   confidence,
		Expiration:   expiration,
	}, nil
}

// ParseNameRecord parses a kind 30103 event into a NameRecord
func ParseNameRecord(ev *event.E) (*NameRecord, error) {
	if uint16(ev.Kind) != KindNameRecords {
		return nil, fmt.Errorf("invalid event kind: expected %d, got %d", KindNameRecords, ev.Kind)
	}

	name := getTagValue(ev, "name")
	if name == "" {
		return nil, fmt.Errorf("missing 'name' tag")
	}

	recordType := getTagValue(ev, "type")
	if recordType == "" {
		return nil, fmt.Errorf("missing 'type' tag")
	}

	value := getTagValue(ev, "value")
	if value == "" {
		return nil, fmt.Errorf("missing 'value' tag")
	}

	ttlStr := getTagValue(ev, "ttl")
	ttl := 3600 // default TTL
	if ttlStr != "" {
		t, err := strconv.Atoi(ttlStr)
		if err == nil {
			ttl = t
		}
	}

	priorityStr := getTagValue(ev, "priority")
	priority := 0
	if priorityStr != "" {
		p, err := strconv.Atoi(priorityStr)
		if err == nil {
			priority = p
		}
	}

	weightStr := getTagValue(ev, "weight")
	weight := 0
	if weightStr != "" {
		w, err := strconv.Atoi(weightStr)
		if err == nil {
			weight = w
		}
	}

	portStr := getTagValue(ev, "port")
	port := 0
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err == nil {
			port = p
		}
	}

	return &NameRecord{
		Event:    ev,
		Name:     name,
		Type:     recordType,
		Value:    value,
		TTL:      ttl,
		Priority: priority,
		Weight:   weight,
		Port:     port,
	}, nil
}

// ParseCertificate parses a kind 30104 event into a Certificate
func ParseCertificate(ev *event.E) (*Certificate, error) {
	if uint16(ev.Kind) != KindCertificate {
		return nil, fmt.Errorf("invalid event kind: expected %d, got %d", KindCertificate, ev.Kind)
	}

	name := getTagValue(ev, "name")
	if name == "" {
		return nil, fmt.Errorf("missing 'name' tag")
	}

	certPubkey := getTagValue(ev, "cert_pubkey")
	if certPubkey == "" {
		return nil, fmt.Errorf("missing 'cert_pubkey' tag")
	}

	validFromStr := getTagValue(ev, "valid_from")
	if validFromStr == "" {
		return nil, fmt.Errorf("missing 'valid_from' tag")
	}
	validFromUnix, err := strconv.ParseInt(validFromStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid valid_from timestamp: %w", err)
	}
	validFrom := time.Unix(validFromUnix, 0)

	validUntilStr := getTagValue(ev, "valid_until")
	if validUntilStr == "" {
		return nil, fmt.Errorf("missing 'valid_until' tag")
	}
	validUntilUnix, err := strconv.ParseInt(validUntilStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid valid_until timestamp: %w", err)
	}
	validUntil := time.Unix(validUntilUnix, 0)

	// Parse witness tags
	var witnesses []WitnessSignature
	witnessTags := getAllTags(ev, "witness")
	for _, t := range witnessTags {
		if len(t.T) < 3 {
			continue // Skip malformed tags
		}

		witnesses = append(witnesses, WitnessSignature{
			Pubkey:    string(t.T[1]),
			Signature: string(t.T[2]),
		})
	}

	// Parse content JSON
	algorithm := "secp256k1-schnorr"
	usage := "tls-replacement"
	if len(ev.Content) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal(ev.Content, &metadata); err == nil {
			if alg, ok := metadata["algorithm"].(string); ok {
				algorithm = alg
			}
			if u, ok := metadata["usage"].(string); ok {
				usage = u
			}
		}
	}

	return &Certificate{
		Event:          ev,
		Name:           name,
		CertPubkey:     certPubkey,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
		Challenge:      getTagValue(ev, "challenge"),
		ChallengeProof: getTagValue(ev, "challenge_proof"),
		Witnesses:      witnesses,
		Algorithm:      algorithm,
		Usage:          usage,
	}, nil
}

// ParseWitnessService parses a kind 30105 event into a WitnessService
func ParseWitnessService(ev *event.E) (*WitnessService, error) {
	if uint16(ev.Kind) != KindWitnessService {
		return nil, fmt.Errorf("invalid event kind: expected %d, got %d", KindWitnessService, ev.Kind)
	}

	endpoint := getTagValue(ev, "endpoint")
	if endpoint == "" {
		return nil, fmt.Errorf("missing 'endpoint' tag")
	}

	// Parse challenge tags
	var challenges []string
	challengeTags := getAllTags(ev, "challenges")
	for _, t := range challengeTags {
		if len(t.T) >= 2 {
			challenges = append(challenges, string(t.T[1]))
		}
	}

	maxValidityStr := getTagValue(ev, "max_validity")
	maxValidity := 0
	if maxValidityStr != "" {
		mv, err := strconv.Atoi(maxValidityStr)
		if err == nil {
			maxValidity = mv
		}
	}

	feeStr := getTagValue(ev, "fee")
	fee := 0
	if feeStr != "" {
		f, err := strconv.Atoi(feeStr)
		if err == nil {
			fee = f
		}
	}

	expirationStr := getTagValue(ev, "expiration")
	var expiration time.Time
	if expirationStr != "" {
		expirationUnix, err := strconv.ParseInt(expirationStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration timestamp: %w", err)
		}
		expiration = time.Unix(expirationUnix, 0)
	}

	// Parse content JSON
	description := ""
	contact := ""
	if len(ev.Content) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal(ev.Content, &metadata); err == nil {
			if desc, ok := metadata["description"].(string); ok {
				description = desc
			}
			if cont, ok := metadata["contact"].(string); ok {
				contact = cont
			}
		}
	}

	return &WitnessService{
		Event:        ev,
		Endpoint:     endpoint,
		Challenges:   challenges,
		MaxValidity:  maxValidity,
		Fee:          fee,
		ReputationID: getTagValue(ev, "reputation"),
		Description:  description,
		Contact:      contact,
		Expiration:   expiration,
	}, nil
}
