package find

import (
	"crypto/sha256"
	"fmt"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer"
)

// SignTransferAuth creates a signature for transfer authorization
// Message format: transfer:<name>:<new_owner_pubkey>:<timestamp>
func SignTransferAuth(name, newOwner string, timestamp time.Time, s signer.I) (string, error) {
	// Normalize name
	name = NormalizeName(name)

	// Construct message
	message := fmt.Sprintf("transfer:%s:%s:%d", name, newOwner, timestamp.Unix())

	// Hash the message
	hash := sha256.Sum256([]byte(message))

	// Sign the hash
	sig, err := s.Sign(hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign transfer authorization: %w", err)
	}

	// Return hex-encoded signature
	return hex.Enc(sig), nil
}

// SignChallengeProof creates a signature for certificate challenge proof
// Message format: challenge||name||cert_pubkey||valid_until
func SignChallengeProof(challenge, name, certPubkey string, validUntil time.Time, s signer.I) (string, error) {
	// Normalize name
	name = NormalizeName(name)

	// Construct message
	message := fmt.Sprintf("%s||%s||%s||%d", challenge, name, certPubkey, validUntil.Unix())

	// Hash the message
	hash := sha256.Sum256([]byte(message))

	// Sign the hash
	sig, err := s.Sign(hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign challenge proof: %w", err)
	}

	// Return hex-encoded signature
	return hex.Enc(sig), nil
}

// SignWitnessMessage creates a witness signature for a certificate
// Message format: cert_pubkey||name||valid_from||valid_until||challenge
func SignWitnessMessage(certPubkey, name string, validFrom, validUntil time.Time, challenge string, s signer.I) (string, error) {
	// Normalize name
	name = NormalizeName(name)

	// Construct message
	message := fmt.Sprintf("%s||%s||%d||%d||%s",
		certPubkey, name, validFrom.Unix(), validUntil.Unix(), challenge)

	// Hash the message
	hash := sha256.Sum256([]byte(message))

	// Sign the hash
	sig, err := s.Sign(hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign witness message: %w", err)
	}

	// Return hex-encoded signature
	return hex.Enc(sig), nil
}

// CreateTransferAuthMessage constructs the transfer authorization message
// This is used for verification
func CreateTransferAuthMessage(name, newOwner string, timestamp time.Time) []byte {
	name = NormalizeName(name)
	message := fmt.Sprintf("transfer:%s:%s:%d", name, newOwner, timestamp.Unix())
	hash := sha256.Sum256([]byte(message))
	return hash[:]
}

// CreateChallengeProofMessage constructs the challenge proof message
// This is used for verification
func CreateChallengeProofMessage(challenge, name, certPubkey string, validUntil time.Time) []byte {
	name = NormalizeName(name)
	message := fmt.Sprintf("%s||%s||%s||%d", challenge, name, certPubkey, validUntil.Unix())
	hash := sha256.Sum256([]byte(message))
	return hash[:]
}

// CreateWitnessMessage constructs the witness message
// This is used for verification
func CreateWitnessMessage(certPubkey, name string, validFrom, validUntil time.Time, challenge string) []byte {
	name = NormalizeName(name)
	message := fmt.Sprintf("%s||%s||%d||%d||%s",
		certPubkey, name, validFrom.Unix(), validUntil.Unix(), challenge)
	hash := sha256.Sum256([]byte(message))
	return hash[:]
}

// ParseTimestampFromProposal extracts the timestamp from a transfer authorization message
// Used for verification when the timestamp is embedded in the signature
func ParseTimestampFromProposal(proposalTime time.Time) time.Time {
	// Round to nearest second for consistency
	return proposalTime.Truncate(time.Second)
}

// FormatTransferAuthString formats the transfer auth message for display/debugging
func FormatTransferAuthString(name, newOwner string, timestamp time.Time) string {
	name = NormalizeName(name)
	return fmt.Sprintf("transfer:%s:%s:%d", name, newOwner, timestamp.Unix())
}

// FormatChallengeProofString formats the challenge proof message for display/debugging
func FormatChallengeProofString(challenge, name, certPubkey string, validUntil time.Time) string {
	name = NormalizeName(name)
	return fmt.Sprintf("%s||%s||%s||%d", challenge, name, certPubkey, validUntil.Unix())
}

// FormatWitnessString formats the witness message for display/debugging
func FormatWitnessString(certPubkey, name string, validFrom, validUntil time.Time, challenge string) string {
	name = NormalizeName(name)
	return fmt.Sprintf("%s||%s||%d||%d||%s",
		certPubkey, name, validFrom.Unix(), validUntil.Unix(), challenge)
}

// SignProposal signs a registration proposal event
func SignProposal(ev *event.E, s signer.I) error {
	return ev.Sign(s)
}

// SignAttestation signs an attestation event
func SignAttestation(ev *event.E, s signer.I) error {
	return ev.Sign(s)
}

// SignTrustGraph signs a trust graph event
func SignTrustGraph(ev *event.E, s signer.I) error {
	return ev.Sign(s)
}

// SignNameState signs a name state event
func SignNameState(ev *event.E, s signer.I) error {
	return ev.Sign(s)
}

// SignNameRecord signs a name record event
func SignNameRecord(ev *event.E, s signer.I) error {
	return ev.Sign(s)
}

// SignCertificate signs a certificate event
func SignCertificate(ev *event.E, s signer.I) error {
	return ev.Sign(s)
}

// SignWitnessService signs a witness service event
func SignWitnessService(ev *event.E, s signer.I) error {
	return ev.Sign(s)
}
