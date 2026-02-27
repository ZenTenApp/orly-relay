package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"next.orly.dev/pkg/nostr/crypto/ec/schnorr"
	"next.orly.dev/pkg/nostr/crypto/ec/secp256k1"
	b32 "next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: convert [--secret] <key>\n")
	fmt.Fprintf(
		os.Stderr, "  <key> can be hex (64 chars) or bech32 (npub/nsec).\n",
	)
	fmt.Fprintf(
		os.Stderr,
		"  --secret: interpret input key as a secret key; print both nsec and npub in hex and bech32.\n"+
			"  --secret is implied if <key> starts with nsec.\n",
	)
}

func main() {
	var isSecret bool
	flag.BoolVar(
		&isSecret, "secret", false, "interpret the input as a secret key",
	)
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	input := strings.TrimSpace(flag.Arg(0))

	// Auto-detect secret if input starts with nsec
	if strings.HasPrefix(input, string(b32.SecHRP)) {
		isSecret = true
	}

	if isSecret {
		if err := handleSecret(input); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := handlePublic(input); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handleSecret(input string) error {
	// Accept nsec bech32 or 64-char hex as secret key
	var sk *secp256k1.SecretKey
	var err error

	if strings.HasPrefix(input, string(b32.SecHRP)) { // nsec...
		if sk, err = b32.NsecToSecretKey([]byte(input)); err != nil {
			return fmt.Errorf("failed to decode nsec: %w", err)
		}
	} else {
		// Expect hex
		if len(input) != b32.HexKeyLen {
			return fmt.Errorf("secret key hex must be %d chars", b32.HexKeyLen)
		}
		var b []byte
		if b, err = hex.Dec(input); err != nil {
			return fmt.Errorf("invalid secret hex: %w", err)
		}
		sk = secp256k1.SecKeyFromBytes(b)
	}

	// Prepare outputs for secret
	nsec, err := b32.SecretKeyToNsec(sk)
	if err != nil {
		return fmt.Errorf("encode nsec: %w", err)
	}
	secHex := hex.EncAppend(nil, sk.Serialize())

	// Derive public key
	pk := sk.PubKey()
	npub, err := b32.PublicKeyToNpub(pk)
	if err != nil {
		return fmt.Errorf("encode npub: %w", err)
	}
	pkBytes := schnorr.SerializePubKey(pk)
	pkHex := hex.EncAppend(nil, pkBytes)

	// Print results
	fmt.Printf("nsec (hex): %s\n", string(secHex))
	fmt.Printf("nsec (bech32): %s\n", string(nsec))
	fmt.Printf("npub (hex): %s\n", string(pkHex))
	fmt.Printf("npub (bech32): %s\n", string(npub))
	return nil
}

func handlePublic(input string) error {
	// Accept npub bech32, nsec bech32 (derive pub), or 64-char hex pubkey
	var pubBytes []byte
	var err error

	if strings.HasPrefix(input, string(b32.PubHRP)) { // npub...
		if pubBytes, err = b32.NpubToBytes([]byte(input)); err != nil {
			return fmt.Errorf("failed to decode npub: %w", err)
		}
	} else if strings.HasPrefix(
		input, string(b32.SecHRP),
	) { // nsec without --secret: show pub only
		var sk *secp256k1.SecretKey
		if sk, err = b32.NsecToSecretKey([]byte(input)); err != nil {
			return fmt.Errorf("failed to decode nsec: %w", err)
		}
		pubBytes = schnorr.SerializePubKey(sk.PubKey())
	} else {
		// Expect hex pubkey
		if len(input) != b32.HexKeyLen {
			return fmt.Errorf("public key hex must be %d chars", b32.HexKeyLen)
		}
		if pubBytes, err = hex.Dec(input); err != nil {
			return fmt.Errorf("invalid public hex: %w", err)
		}
	}

	// Compute encodings
	npub, err := b32.BinToNpub(pubBytes)
	if err != nil {
		return fmt.Errorf("encode npub: %w", err)
	}
	pubHex := hex.EncAppend(nil, pubBytes)

	// Print only pubkey representations
	fmt.Printf("npub (hex): %s\n", string(pubHex))
	fmt.Printf("npub (bech32): %s\n", string(npub))
	return nil
}
