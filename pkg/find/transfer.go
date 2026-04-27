package find

import (
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
)

// CreateTransferProposal creates a complete transfer proposal with authorization from previous owner
func CreateTransferProposal(name string, prevOwnerSigner, newOwnerSigner signer.I) (*event.E, error) {
	// Normalize name
	name = NormalizeName(name)

	// Validate name
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Get public keys
	prevOwnerPubkey := hex.Enc(prevOwnerSigner.Pub())
	newOwnerPubkey := hex.Enc(newOwnerSigner.Pub())

	// Create timestamp for the transfer
	timestamp := time.Now()

	// Sign the transfer authorization with previous owner's key
	prevSig, err := SignTransferAuth(name, newOwnerPubkey, timestamp, prevOwnerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer authorization: %w", err)
	}

	// Create the transfer proposal event signed by new owner
	proposal, err := NewRegistrationProposalWithTransfer(name, prevOwnerPubkey, prevSig, newOwnerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer proposal: %w", err)
	}

	return proposal, nil
}

// ValidateTransferProposal validates a transfer proposal against the current owner
func ValidateTransferProposal(proposal *RegistrationProposal, currentOwner string) error {
	// Check that this is a transfer action
	if proposal.Action != ActionTransfer {
		return fmt.Errorf("not a transfer action: %s", proposal.Action)
	}

	// Check that prev_owner is set
	if proposal.PrevOwner == "" {
		return fmt.Errorf("missing prev_owner in transfer proposal")
	}

	// Check that prev_sig is set
	if proposal.PrevSig == "" {
		return fmt.Errorf("missing prev_sig in transfer proposal")
	}

	// Verify that prev_owner matches current owner
	if proposal.PrevOwner != currentOwner {
		return fmt.Errorf("prev_owner %s does not match current owner %s",
			proposal.PrevOwner, currentOwner)
	}

	// Get new owner from proposal event
	newOwnerPubkey := hex.Enc(proposal.Event.Pubkey)

	// Verify the transfer authorization signature
	// Use proposal creation time as timestamp
	timestamp := time.Unix(proposal.Event.CreatedAt, 0)

	ok, err := VerifyTransferAuth(proposal.Name, newOwnerPubkey, proposal.PrevOwner,
		timestamp, proposal.PrevSig)
	if err != nil {
		return fmt.Errorf("transfer authorization verification failed: %w", err)
	}

	if !ok {
		return fmt.Errorf("invalid transfer authorization signature")
	}

	return nil
}

// PrepareTransferAuth prepares the transfer authorization data that needs to be signed
// This is a helper for wallets/clients that want to show what they're signing
func PrepareTransferAuth(name, newOwner string, timestamp time.Time) TransferAuthorization {
	return TransferAuthorization{
		Name:      NormalizeName(name),
		NewOwner:  newOwner,
		Timestamp: timestamp,
	}
}

// AuthorizeTransfer creates a transfer authorization signature
// This is meant to be used by the current owner to authorize a transfer to a new owner
func AuthorizeTransfer(name, newOwnerPubkey string, ownerSigner signer.I) (prevSig string, timestamp time.Time, err error) {
	// Normalize name
	name = NormalizeName(name)

	// Validate name
	if err := ValidateName(name); err != nil {
		return "", time.Time{}, fmt.Errorf("invalid name: %w", err)
	}

	// Create timestamp
	timestamp = time.Now()

	// Sign the authorization
	prevSig, err = SignTransferAuth(name, newOwnerPubkey, timestamp, ownerSigner)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign transfer auth: %w", err)
	}

	return prevSig, timestamp, nil
}

// CreateTransferProposalWithAuth creates a transfer proposal using a pre-existing authorization
// This is useful when the previous owner has already provided their signature
func CreateTransferProposalWithAuth(name, prevOwnerPubkey, prevSig string, newOwnerSigner signer.I) (*event.E, error) {
	// Normalize name
	name = NormalizeName(name)

	// Validate name
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Create the transfer proposal event
	proposal, err := NewRegistrationProposalWithTransfer(name, prevOwnerPubkey, prevSig, newOwnerSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer proposal: %w", err)
	}

	return proposal, nil
}

// VerifyTransferProposalSignature verifies both the event signature and transfer authorization
func VerifyTransferProposalSignature(proposal *RegistrationProposal) error {
	// Verify the event signature itself
	if err := VerifyEvent(proposal.Event); err != nil {
		return fmt.Errorf("invalid event signature: %w", err)
	}

	// If this is a transfer, verify the transfer authorization
	if proposal.Action == ActionTransfer {
		// Get new owner from proposal event
		newOwnerPubkey := hex.Enc(proposal.Event.Pubkey)

		// Use proposal creation time as timestamp
		timestamp := time.Unix(proposal.Event.CreatedAt, 0)

		// Verify transfer auth
		ok, err := VerifyTransferAuth(proposal.Name, newOwnerPubkey, proposal.PrevOwner,
			timestamp, proposal.PrevSig)
		if err != nil {
			return fmt.Errorf("transfer authorization verification failed: %w", err)
		}

		if !ok {
			return fmt.Errorf("invalid transfer authorization signature")
		}
	}

	return nil
}
