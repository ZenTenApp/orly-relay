package find

import (
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// VerifyEvent verifies the signature of a Nostr event
func VerifyEvent(ev *event.E) error {
	ok, err := ev.Verify()
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// VerifyTransferAuth verifies a transfer authorization signature
func VerifyTransferAuth(name, newOwner, prevOwner string, timestamp time.Time, sigHex string) (bool, error) {
	// Create the message
	msgHash := CreateTransferAuthMessage(name, newOwner, timestamp)

	// Decode signature
	sig, err := hex.Dec(sigHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	// Decode pubkey
	pubkey, err := hex.Dec(prevOwner)
	if err != nil {
		return false, fmt.Errorf("invalid pubkey hex: %w", err)
	}

	// Create verifier with public key
	verifier, err := p8k.New()
	if err != nil {
		return false, fmt.Errorf("failed to create verifier: %w", err)
	}

	if err := verifier.InitPub(pubkey); err != nil {
		return false, fmt.Errorf("failed to init pubkey: %w", err)
	}

	// Verify signature
	ok, err := verifier.Verify(msgHash, sig)
	if err != nil {
		return false, fmt.Errorf("verification failed: %w", err)
	}

	return ok, nil
}

// VerifyChallengeProof verifies a certificate challenge proof signature
func VerifyChallengeProof(challenge, name, certPubkey, owner string, validUntil time.Time, sigHex string) (bool, error) {
	// Create the message
	msgHash := CreateChallengeProofMessage(challenge, name, certPubkey, validUntil)

	// Decode signature
	sig, err := hex.Dec(sigHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	// Decode pubkey
	pubkey, err := hex.Dec(owner)
	if err != nil {
		return false, fmt.Errorf("invalid pubkey hex: %w", err)
	}

	// Create verifier with public key
	verifier, err := p8k.New()
	if err != nil {
		return false, fmt.Errorf("failed to create verifier: %w", err)
	}

	if err := verifier.InitPub(pubkey); err != nil {
		return false, fmt.Errorf("failed to init pubkey: %w", err)
	}

	// Verify signature
	ok, err := verifier.Verify(msgHash, sig)
	if err != nil {
		return false, fmt.Errorf("verification failed: %w", err)
	}

	return ok, nil
}

// VerifyWitnessSignature verifies a witness signature on a certificate
func VerifyWitnessSignature(certPubkey, name string, validFrom, validUntil time.Time,
	challenge, witnessPubkey, sigHex string) (bool, error) {

	// Create the message
	msgHash := CreateWitnessMessage(certPubkey, name, validFrom, validUntil, challenge)

	// Decode signature
	sig, err := hex.Dec(sigHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	// Decode pubkey
	pubkey, err := hex.Dec(witnessPubkey)
	if err != nil {
		return false, fmt.Errorf("invalid pubkey hex: %w", err)
	}

	// Create verifier with public key
	verifier, err := p8k.New()
	if err != nil {
		return false, fmt.Errorf("failed to create verifier: %w", err)
	}

	if err := verifier.InitPub(pubkey); err != nil {
		return false, fmt.Errorf("failed to init pubkey: %w", err)
	}

	// Verify signature
	ok, err := verifier.Verify(msgHash, sig)
	if err != nil {
		return false, fmt.Errorf("verification failed: %w", err)
	}

	return ok, nil
}

// VerifyNameOwnership checks if a record's owner matches the name state owner
func VerifyNameOwnership(nameState *NameState, record *NameRecord) error {
	recordOwner := hex.Enc(record.Event.Pubkey)
	if recordOwner != nameState.Owner {
		return fmt.Errorf("%w: record owner %s != name owner %s",
			ErrNotOwner, recordOwner, nameState.Owner)
	}
	return nil
}

// IsExpired checks if a time-based expiration has passed
func IsExpired(expiration time.Time) bool {
	return time.Now().After(expiration)
}

// IsInRenewalWindow checks if the current time is within the preferential renewal window
// (final 30 days before expiration)
func IsInRenewalWindow(expiration time.Time) bool {
	now := time.Now()
	renewalWindowStart := expiration.Add(-PreferentialRenewalDays * 24 * time.Hour)
	return now.After(renewalWindowStart) && now.Before(expiration)
}

// CanRegister checks if a name can be registered based on its state and expiration
func CanRegister(nameState *NameState, proposerPubkey string) error {
	// If no name state exists, anyone can register
	if nameState == nil {
		return nil
	}

	// Check if name is expired
	if IsExpired(nameState.Expiration) {
		// Name is expired, anyone can register
		return nil
	}

	// Check if in renewal window
	if IsInRenewalWindow(nameState.Expiration) {
		// Only current owner can register during renewal window
		if proposerPubkey != nameState.Owner {
			return ErrInRenewalWindow
		}
		return nil
	}

	// Name is still owned and not in renewal window
	return fmt.Errorf("name is owned by %s until %s", nameState.Owner, nameState.Expiration)
}

// VerifyProposalExpiration checks if a proposal has expired
func VerifyProposalExpiration(proposal *RegistrationProposal) error {
	if !proposal.Expiration.IsZero() && IsExpired(proposal.Expiration) {
		return fmt.Errorf("proposal expired at %s", proposal.Expiration)
	}
	return nil
}

// VerifyAttestationExpiration checks if an attestation has expired
func VerifyAttestationExpiration(attestation *Attestation) error {
	if !attestation.Expiration.IsZero() && IsExpired(attestation.Expiration) {
		return fmt.Errorf("attestation expired at %s", attestation.Expiration)
	}
	return nil
}

// VerifyTrustGraphExpiration checks if a trust graph has expired
func VerifyTrustGraphExpiration(trustGraph *TrustGraphEvent) error {
	if !trustGraph.Expiration.IsZero() && IsExpired(trustGraph.Expiration) {
		return fmt.Errorf("trust graph expired at %s", trustGraph.Expiration)
	}
	return nil
}

// VerifyNameStateExpiration checks if a name state has expired
func VerifyNameStateExpiration(nameState *NameState) error {
	if !nameState.Expiration.IsZero() && IsExpired(nameState.Expiration) {
		return ErrNameExpired
	}
	return nil
}

// VerifyCertificateValidity checks if a certificate is currently valid
func VerifyCertificateValidity(cert *Certificate) error {
	now := time.Now()

	if now.Before(cert.ValidFrom) {
		return fmt.Errorf("certificate not yet valid (valid from %s)", cert.ValidFrom)
	}

	if now.After(cert.ValidUntil) {
		return fmt.Errorf("certificate expired at %s", cert.ValidUntil)
	}

	return nil
}

// VerifyCertificate performs complete certificate verification
func VerifyCertificate(cert *Certificate, nameState *NameState, trustedWitnesses []string) error {
	// Verify certificate is not expired
	if err := VerifyCertificateValidity(cert); err != nil {
		return err
	}

	// Verify name is not expired
	if err := VerifyNameStateExpiration(nameState); err != nil {
		return err
	}

	// Verify certificate owner matches name owner
	certOwner := hex.Enc(cert.Event.Pubkey)
	if certOwner != nameState.Owner {
		return fmt.Errorf("certificate owner %s != name owner %s", certOwner, nameState.Owner)
	}

	// Verify challenge proof
	ok, err := VerifyChallengeProof(cert.Challenge, cert.Name, cert.CertPubkey,
		nameState.Owner, cert.ValidUntil, cert.ChallengeProof)
	if err != nil {
		return fmt.Errorf("challenge proof verification failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("invalid challenge proof signature")
	}

	// Count trusted witnesses
	trustedCount := 0
	for _, witness := range cert.Witnesses {
		// Check if witness is in trusted list
		isTrusted := false
		for _, trusted := range trustedWitnesses {
			if witness.Pubkey == trusted {
				isTrusted = true
				break
			}
		}

		if !isTrusted {
			continue
		}

		// Verify witness signature
		ok, err := VerifyWitnessSignature(cert.CertPubkey, cert.Name,
			cert.ValidFrom, cert.ValidUntil, cert.Challenge,
			witness.Pubkey, witness.Signature)
		if err != nil {
			return fmt.Errorf("witness %s signature verification failed: %w", witness.Pubkey, err)
		}
		if !ok {
			return fmt.Errorf("invalid witness %s signature", witness.Pubkey)
		}

		trustedCount++
	}

	// Require at least 3 trusted witnesses
	if trustedCount < 3 {
		return fmt.Errorf("insufficient trusted witnesses: %d < 3", trustedCount)
	}

	return nil
}

// VerifySubdomainAuthority checks if the proposer owns the parent domain
func VerifySubdomainAuthority(name string, proposerPubkey string, parentNameState *NameState) error {
	parent := GetParentDomain(name)

	// TLDs have no parent
	if parent == "" {
		return nil
	}

	// Parent must exist
	if parentNameState == nil {
		return fmt.Errorf("parent domain %s does not exist", parent)
	}

	// Proposer must own parent
	if proposerPubkey != parentNameState.Owner {
		return fmt.Errorf("proposer %s does not own parent domain %s (owner: %s)",
			proposerPubkey, parent, parentNameState.Owner)
	}

	return nil
}
