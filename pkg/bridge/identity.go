package bridge

import (
	"fmt"
	"os"
	"path/filepath"

	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

// IdentitySource describes where the bridge identity came from.
type IdentitySource int

const (
	IdentityFromConfig IdentitySource = iota // NSEC provided via env/config
	IdentityFromDB                           // Read from relay database
	IdentityFromFile                         // Read from or generated to file
)

// ResolveIdentity resolves the bridge signer using a three-tier strategy:
//
//  1. Config NSEC (env var ORLY_BRIDGE_NSEC) — highest priority
//  2. Database getter (monolithic mode — reads relay identity)
//  3. File fallback (standalone mode — reads or generates bridge.nsec)
//
// The dbGetter parameter is a function that returns the relay identity secret
// key from the database. Pass nil in standalone mode.
func ResolveIdentity(nsecConfig string, dbGetter func() ([]byte, error), dataDir string) (signer.I, IdentitySource, error) {
	// Tier 1: NSEC from config/environment
	if nsecConfig != "" {
		sign, err := signerFromNSEC(nsecConfig)
		if err != nil {
			return nil, 0, fmt.Errorf("config NSEC: %w", err)
		}
		return sign, IdentityFromConfig, nil
	}

	// Tier 2: Database (relay identity)
	if dbGetter != nil {
		sk, err := dbGetter()
		if err == nil && len(sk) == 32 {
			sign, err := signerFromSecretKey(sk)
			if err != nil {
				return nil, 0, fmt.Errorf("database identity: %w", err)
			}
			return sign, IdentityFromDB, nil
		}
		// Fall through to file if database fails
	}

	// Tier 3: File fallback
	sign, err := identityFromFile(dataDir)
	if err != nil {
		return nil, 0, fmt.Errorf("file identity: %w", err)
	}
	return sign, IdentityFromFile, nil
}

// signerFromNSEC creates a signer from an nsec bech32 string or hex secret key.
func signerFromNSEC(nsec string) (signer.I, error) {
	var sk []byte
	var err error

	// Try nsec bech32 first
	if len(nsec) > 4 && nsec[:4] == "nsec" {
		sk, err = bech32encoding.NsecToBytes(nsec)
		if err != nil {
			return nil, fmt.Errorf("decode nsec: %w", err)
		}
	} else {
		// Try hex
		sk, err = hex.Dec(nsec)
		if err != nil || len(sk) != 32 {
			return nil, fmt.Errorf("invalid secret key (expected 32-byte hex or nsec bech32)")
		}
	}

	return signerFromSecretKey(sk)
}

// signerFromSecretKey creates a signer from a raw 32-byte secret key.
func signerFromSecretKey(sk []byte) (signer.I, error) {
	sign, err := p8k.New()
	if err != nil {
		return nil, fmt.Errorf("create signer: %w", err)
	}
	if err = sign.InitSec(sk); err != nil {
		return nil, fmt.Errorf("init signer: %w", err)
	}
	return sign, nil
}

// identityFromFile reads or generates the bridge identity from a file.
func identityFromFile(dataDir string) (signer.I, error) {
	nsecPath := filepath.Join(dataDir, "bridge.nsec")

	// Try reading existing file
	data, err := os.ReadFile(nsecPath)
	if err == nil {
		nsec := string(trimBytes(data))
		if nsec != "" {
			return signerFromNSEC(nsec)
		}
	}

	// Generate new identity
	sk, err := keys.GenerateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}
	sign, err := signerFromSecretKey(sk)
	if err != nil {
		return nil, err
	}

	// Persist as nsec
	nsec, err := bech32encoding.BinToNsec(sk)
	if err != nil {
		return nil, fmt.Errorf("encode nsec: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.WriteFile(nsecPath, []byte(nsec), 0600); err != nil {
		return nil, fmt.Errorf("write nsec file: %w", err)
	}

	return sign, nil
}

// trimBytes trims whitespace and newlines from byte slices.
func trimBytes(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	return b
}
