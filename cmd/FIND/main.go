package main

import (
	"fmt"
	"os"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/find"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "register":
		handleRegister()
	case "transfer":
		handleTransfer()
	case "verify-name":
		handleVerifyName()
	case "generate-key":
		handleGenerateKey()
	case "issue-cert":
		handleIssueCert()
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("FIND - Free Internet Name Daemon")
	fmt.Println("Usage: find <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  register <name>              Create a registration proposal for a name")
	fmt.Println("  transfer <name> <new-owner>  Transfer a name to a new owner")
	fmt.Println("  verify-name <name>           Validate a name format")
	fmt.Println("  generate-key                 Generate a new key pair")
	fmt.Println("  issue-cert <name>            Issue a certificate for a name")
	fmt.Println("  help                         Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  find verify-name example.com")
	fmt.Println("  find register myname.nostr")
	fmt.Println("  find generate-key")
}

func handleRegister() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: find register <name>")
		os.Exit(1)
	}

	name := os.Args[2]

	// Validate the name
	if err := find.ValidateName(name); err != nil {
		fmt.Printf("Invalid name: %v\n", err)
		os.Exit(1)
	}

	// Generate a key pair for this example
	// In production, this would load from a secure keystore
	signer, err := p8k.New()
	if err != nil {
		fmt.Printf("Failed to create signer: %v\n", err)
		os.Exit(1)
	}

	if err := signer.Generate(); err != nil {
		fmt.Printf("Failed to generate key: %v\n", err)
		os.Exit(1)
	}

	// Create registration proposal
	proposal, err := find.NewRegistrationProposal(name, find.ActionRegister, signer)
	if err != nil {
		fmt.Printf("Failed to create proposal: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Registration Proposal Created\n")
	fmt.Printf("==============================\n")
	fmt.Printf("Name:       %s\n", name)
	fmt.Printf("Pubkey:     %s\n", hex.Enc(signer.Pub()))
	fmt.Printf("Event ID:   %s\n", hex.Enc(proposal.GetIDBytes()))
	fmt.Printf("Kind:       %d\n", proposal.Kind)
	fmt.Printf("Created At: %s\n", time.Unix(proposal.CreatedAt, 0))
	fmt.Printf("\nEvent JSON:\n")
	json := proposal.Marshal(nil)
	fmt.Println(string(json))
}

func handleTransfer() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: find transfer <name> <new-owner-pubkey>")
		os.Exit(1)
	}

	name := os.Args[2]
	newOwnerPubkey := os.Args[3]

	// Validate the name
	if err := find.ValidateName(name); err != nil {
		fmt.Printf("Invalid name: %v\n", err)
		os.Exit(1)
	}

	// Generate current owner key (in production, load from keystore)
	currentOwner, err := p8k.New()
	if err != nil {
		fmt.Printf("Failed to create current owner signer: %v\n", err)
		os.Exit(1)
	}

	if err := currentOwner.Generate(); err != nil {
		fmt.Printf("Failed to generate current owner key: %v\n", err)
		os.Exit(1)
	}

	// Authorize the transfer
	prevSig, timestamp, err := find.AuthorizeTransfer(name, newOwnerPubkey, currentOwner)
	if err != nil {
		fmt.Printf("Failed to authorize transfer: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Transfer Authorization Created\n")
	fmt.Printf("===============================\n")
	fmt.Printf("Name:           %s\n", name)
	fmt.Printf("Current Owner:  %s\n", hex.Enc(currentOwner.Pub()))
	fmt.Printf("New Owner:      %s\n", newOwnerPubkey)
	fmt.Printf("Timestamp:      %s\n", timestamp)
	fmt.Printf("Signature:      %s\n", prevSig)
	fmt.Printf("\nTo complete the transfer, the new owner must create a proposal with:")
	fmt.Printf("  prev_owner: %s\n", hex.Enc(currentOwner.Pub()))
	fmt.Printf("  prev_sig: %s\n", prevSig)
}

func handleVerifyName() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: find verify-name <name>")
		os.Exit(1)
	}

	name := os.Args[2]

	// Validate the name
	if err := find.ValidateName(name); err != nil {
		fmt.Printf("❌ Invalid name: %v\n", err)
		os.Exit(1)
	}

	normalized := find.NormalizeName(name)
	isTLD := find.IsTLD(normalized)
	parent := find.GetParentDomain(normalized)

	fmt.Printf("✓ Valid name\n")
	fmt.Printf("==============\n")
	fmt.Printf("Original:   %s\n", name)
	fmt.Printf("Normalized: %s\n", normalized)
	fmt.Printf("Is TLD:     %v\n", isTLD)
	if parent != "" {
		fmt.Printf("Parent:     %s\n", parent)
	}
}

func handleGenerateKey() {
	// Generate a new key pair
	secKey, err := keys.GenerateSecretKey()
	if err != nil {
		fmt.Printf("Failed to generate secret key: %v\n", err)
		os.Exit(1)
	}

	secKeyHex := hex.Enc(secKey)
	pubKeyHex, err := keys.GetPublicKeyHex(secKeyHex)
	if err != nil {
		fmt.Printf("Failed to derive public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("New Key Pair Generated")
	fmt.Println("======================")
	fmt.Printf("Secret Key (keep safe!): %s\n", secKeyHex)
	fmt.Printf("Public Key:              %s\n", pubKeyHex)
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Store the secret key securely. Anyone with access to it can control your names.")
}

func handleIssueCert() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: find issue-cert <name>")
		os.Exit(1)
	}

	name := os.Args[2]

	// Validate the name
	if err := find.ValidateName(name); err != nil {
		fmt.Printf("Invalid name: %v\n", err)
		os.Exit(1)
	}

	// Generate name owner key
	owner, err := p8k.New()
	if err != nil {
		fmt.Printf("Failed to create owner signer: %v\n", err)
		os.Exit(1)
	}

	if err := owner.Generate(); err != nil {
		fmt.Printf("Failed to generate owner key: %v\n", err)
		os.Exit(1)
	}

	// Generate certificate key (different from name owner)
	certSigner, err := p8k.New()
	if err != nil {
		fmt.Printf("Failed to create cert signer: %v\n", err)
		os.Exit(1)
	}

	if err := certSigner.Generate(); err != nil {
		fmt.Printf("Failed to generate cert key: %v\n", err)
		os.Exit(1)
	}

	certPubkey := hex.Enc(certSigner.Pub())

	// Generate 3 witness signers (in production, these would be separate services)
	var witnesses []signer.I
	for i := 0; i < 3; i++ {
		witness, err := p8k.New()
		if err != nil {
			fmt.Printf("Failed to create witness %d: %v\n", i, err)
			os.Exit(1)
		}

		if err := witness.Generate(); err != nil {
			fmt.Printf("Failed to generate witness %d key: %v\n", i, err)
			os.Exit(1)
		}

		witnesses = append(witnesses, witness)
	}

	// Issue certificate (90 day validity)
	cert, err := find.IssueCertificate(name, certPubkey, find.CertificateValidity, owner, witnesses)
	if err != nil {
		fmt.Printf("Failed to issue certificate: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Certificate Issued\n")
	fmt.Printf("==================\n")
	fmt.Printf("Name:          %s\n", cert.Name)
	fmt.Printf("Cert Pubkey:   %s\n", cert.CertPubkey)
	fmt.Printf("Valid From:    %s\n", cert.ValidFrom)
	fmt.Printf("Valid Until:   %s\n", cert.ValidUntil)
	fmt.Printf("Challenge:     %s\n", cert.Challenge)
	fmt.Printf("Witnesses:     %d\n", len(cert.Witnesses))
	fmt.Printf("Algorithm:     %s\n", cert.Algorithm)
	fmt.Printf("Usage:         %s\n", cert.Usage)

	fmt.Printf("\nWitness Pubkeys:\n")
	for i, w := range cert.Witnesses {
		fmt.Printf("  %d: %s\n", i+1, w.Pubkey)
	}
}
