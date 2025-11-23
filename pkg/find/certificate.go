package find

import (
	"crypto/rand"
	"fmt"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
)

// GenerateChallenge generates a random 32-byte challenge token
func GenerateChallenge() (string, error) {
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return "", fmt.Errorf("failed to generate random challenge: %w", err)
	}
	return hex.Enc(challenge), nil
}

// CreateChallengeTXTRecord creates a TXT record event for challenge-response verification
func CreateChallengeTXTRecord(name, challenge string, ttl int, signer signer.I) (*event.E, error) {
	// Normalize name
	name = NormalizeName(name)

	// Validate name
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Create TXT record value
	txtValue := fmt.Sprintf("_nostr-challenge=%s", challenge)

	// Create the TXT record event
	record, err := NewNameRecord(name, RecordTypeTXT, txtValue, ttl, signer)
	if err != nil {
		return nil, fmt.Errorf("failed to create challenge TXT record: %w", err)
	}

	return record, nil
}

// ExtractChallengeFromTXTRecord extracts the challenge token from a TXT record value
func ExtractChallengeFromTXTRecord(txtValue string) (string, error) {
	const prefix = "_nostr-challenge="

	if len(txtValue) < len(prefix) {
		return "", fmt.Errorf("TXT record too short")
	}

	if txtValue[:len(prefix)] != prefix {
		return "", fmt.Errorf("not a challenge TXT record")
	}

	challenge := txtValue[len(prefix):]
	if len(challenge) != 64 { // 32 bytes in hex = 64 characters
		return "", fmt.Errorf("invalid challenge length: %d", len(challenge))
	}

	return challenge, nil
}

// CreateChallengeProof creates a challenge proof signature
func CreateChallengeProof(challenge, name, certPubkey string, validUntil time.Time, signer signer.I) (string, error) {
	// Normalize name
	name = NormalizeName(name)

	// Sign the challenge proof
	proof, err := SignChallengeProof(challenge, name, certPubkey, validUntil, signer)
	if err != nil {
		return "", fmt.Errorf("failed to create challenge proof: %w", err)
	}

	return proof, nil
}

// RequestWitnessSignature creates a witness signature for a certificate
// This would typically be called by a witness service
func RequestWitnessSignature(cert *Certificate, witnessSigner signer.I) (WitnessSignature, error) {
	// Sign the witness message
	sig, err := SignWitnessMessage(cert.CertPubkey, cert.Name,
		cert.ValidFrom, cert.ValidUntil, cert.Challenge, witnessSigner)
	if err != nil {
		return WitnessSignature{}, fmt.Errorf("failed to create witness signature: %w", err)
	}

	// Get witness pubkey
	witnessPubkey := hex.Enc(witnessSigner.Pub())

	return WitnessSignature{
		Pubkey:    witnessPubkey,
		Signature: sig,
	}, nil
}

// PrepareCertificateRequest prepares all the data needed for a certificate request
type CertificateRequest struct {
	Name           string
	CertPubkey     string
	ValidFrom      time.Time
	ValidUntil     time.Time
	Challenge      string
	ChallengeProof string
}

// CreateCertificateRequest creates a certificate request with challenge-response
func CreateCertificateRequest(name, certPubkey string, validityDuration time.Duration,
	challenge string, ownerSigner signer.I) (*CertificateRequest, error) {

	// Normalize name
	name = NormalizeName(name)

	// Validate name
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Set validity period
	validFrom := time.Now()
	validUntil := validFrom.Add(validityDuration)

	// Create challenge proof
	proof, err := CreateChallengeProof(challenge, name, certPubkey, validUntil, ownerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create challenge proof: %w", err)
	}

	return &CertificateRequest{
		Name:           name,
		CertPubkey:     certPubkey,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
		Challenge:      challenge,
		ChallengeProof: proof,
	}, nil
}

// CreateCertificateWithWitnesses creates a complete certificate event with witness signatures
func CreateCertificateWithWitnesses(req *CertificateRequest, witnesses []WitnessSignature,
	algorithm, usage string, ownerSigner signer.I) (*event.E, error) {

	// Create the certificate event
	certEvent, err := NewCertificate(
		req.Name,
		req.CertPubkey,
		req.ValidFrom,
		req.ValidUntil,
		req.Challenge,
		req.ChallengeProof,
		witnesses,
		algorithm,
		usage,
		ownerSigner,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	return certEvent, nil
}

// VerifyChallengeTXTRecord verifies that a TXT record contains the expected challenge
func VerifyChallengeTXTRecord(record *NameRecord, expectedChallenge string, nameOwner string) error {
	// Check record type
	if record.Type != RecordTypeTXT {
		return fmt.Errorf("not a TXT record: %s", record.Type)
	}

	// Check record owner matches name owner
	recordOwner := hex.Enc(record.Event.Pubkey)
	if recordOwner != nameOwner {
		return fmt.Errorf("record owner %s != name owner %s", recordOwner, nameOwner)
	}

	// Extract challenge from TXT record
	challenge, err := ExtractChallengeFromTXTRecord(record.Value)
	if err != nil {
		return fmt.Errorf("failed to extract challenge: %w", err)
	}

	// Verify challenge matches
	if challenge != expectedChallenge {
		return fmt.Errorf("challenge mismatch: got %s, expected %s", challenge, expectedChallenge)
	}

	return nil
}

// IssueCertificate is a helper that goes through the full certificate issuance process
// This would typically be used by a name owner to request a certificate
func IssueCertificate(name, certPubkey string, validityDuration time.Duration,
	ownerSigner signer.I, witnessSigners []signer.I) (*Certificate, error) {

	// Generate challenge
	challenge, err := GenerateChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	// Create certificate request
	req, err := CreateCertificateRequest(name, certPubkey, validityDuration, challenge, ownerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate request: %w", err)
	}

	// Collect witness signatures
	var witnesses []WitnessSignature
	for i, ws := range witnessSigners {
		// Create temporary certificate for witness to sign
		tempCert := &Certificate{
			Name:       req.Name,
			CertPubkey: req.CertPubkey,
			ValidFrom:  req.ValidFrom,
			ValidUntil: req.ValidUntil,
			Challenge:  req.Challenge,
		}

		witness, err := RequestWitnessSignature(tempCert, ws)
		if err != nil {
			return nil, fmt.Errorf("failed to get witness %d signature: %w", i, err)
		}

		witnesses = append(witnesses, witness)
	}

	// Create certificate event
	certEvent, err := CreateCertificateWithWitnesses(req, witnesses, "secp256k1-schnorr", "tls-replacement", ownerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate event: %w", err)
	}

	// Parse back to Certificate struct
	cert, err := ParseCertificate(certEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// RenewCertificate creates a renewed certificate with a new validity period
func RenewCertificate(oldCert *Certificate, newValidityDuration time.Duration,
	ownerSigner signer.I, witnessSigners []signer.I) (*Certificate, error) {

	// Generate new challenge
	challenge, err := GenerateChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	// Set new validity period (with 7-day overlap)
	validFrom := oldCert.ValidUntil.Add(-7 * 24 * time.Hour)
	validUntil := validFrom.Add(newValidityDuration)

	// Create challenge proof
	proof, err := CreateChallengeProof(challenge, oldCert.Name, oldCert.CertPubkey, validUntil, ownerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create challenge proof: %w", err)
	}

	// Create request
	req := &CertificateRequest{
		Name:           oldCert.Name,
		CertPubkey:     oldCert.CertPubkey,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
		Challenge:      challenge,
		ChallengeProof: proof,
	}

	// Collect witness signatures
	var witnesses []WitnessSignature
	for i, ws := range witnessSigners {
		tempCert := &Certificate{
			Name:       req.Name,
			CertPubkey: req.CertPubkey,
			ValidFrom:  req.ValidFrom,
			ValidUntil: req.ValidUntil,
			Challenge:  req.Challenge,
		}

		witness, err := RequestWitnessSignature(tempCert, ws)
		if err != nil {
			return nil, fmt.Errorf("failed to get witness %d signature: %w", i, err)
		}

		witnesses = append(witnesses, witness)
	}

	// Create certificate event
	certEvent, err := CreateCertificateWithWitnesses(req, witnesses, oldCert.Algorithm, oldCert.Usage, ownerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate event: %w", err)
	}

	// Parse back to Certificate struct
	cert, err := ParseCertificate(certEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// CheckCertificateExpiry returns the time until expiration, or error if expired
func CheckCertificateExpiry(cert *Certificate) (time.Duration, error) {
	now := time.Now()

	if now.After(cert.ValidUntil) {
		return 0, fmt.Errorf("certificate expired %v ago", now.Sub(cert.ValidUntil))
	}

	return cert.ValidUntil.Sub(now), nil
}

// ShouldRenewCertificate checks if a certificate should be renewed (< 30 days until expiry)
func ShouldRenewCertificate(cert *Certificate) bool {
	timeUntilExpiry, err := CheckCertificateExpiry(cert)
	if err != nil {
		return true // Expired, definitely should renew
	}

	return timeUntilExpiry < 30*24*time.Hour
}
