//go:build interop

package marmot

import (
	"encoding/hex"
	"testing"

	"github.com/emersion/go-mls"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

// TestDebug_CompareWireFormat dumps both Go and Rust raw KP bytes for comparison.
func TestDebug_CompareWireFormat(t *testing.T) {
	// Generate Go key package
	sign, err := p8k.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := sign.Generate(); err != nil {
		t.Fatal(err)
	}
	kpp, err := GenerateKeyPackage(&LocalCrypto{Sign: sign})
	if err != nil {
		t.Fatal(err)
	}
	goBytes := kpp.Public.RawBytes()
	goHex := hex.EncodeToString(goBytes)

	// Get Rust key package bytes via interop binary
	rust, _ := startRust(t)
	resp := rust.send(t, map[string]string{
		"cmd":   "dump_kp_bytes",
		"relay": "wss://relay.example.com",
	})
	if !resp.OK {
		t.Fatalf("rust dump_kp_bytes failed: %s", resp.Error)
	}
	rustHex := resp.Content

	rustBytes, err := hex.DecodeString(rustHex)
	if err != nil {
		t.Fatalf("decode rust hex: %v", err)
	}

	t.Logf("Go  raw KP (%d bytes): %s", len(goBytes), goHex)
	t.Logf("Rust raw KP (%d bytes): %s", len(rustBytes), rustHex)

	// Find first byte difference
	minLen := len(goBytes)
	if len(rustBytes) < minLen {
		minLen = len(rustBytes)
	}
	for i := 0; i < minLen; i++ {
		if goBytes[i] != rustBytes[i] {
			start := i - 4
			if start < 0 {
				start = 0
			}
			end := i + 20
			if end > minLen {
				end = minLen
			}
			t.Logf("FIRST DIFF at byte %d:", i)
			t.Logf("  Go   [%d:%d]: %s", start, end, hex.EncodeToString(goBytes[start:end]))
			t.Logf("  Rust [%d:%d]: %s", start, end, hex.EncodeToString(rustBytes[start:end]))
			break
		}
	}
	if len(goBytes) != len(rustBytes) {
		t.Logf("LENGTH DIFF: Go=%d, Rust=%d", len(goBytes), len(rustBytes))
	}

	// Try to parse the Rust raw bytes through go-mls
	_, err = mls.UnmarshalRawKeyPackage(rustBytes)
	if err != nil {
		t.Logf("Go failed to parse Rust raw bytes: %v", err)
	} else {
		t.Log("Go successfully parsed Rust raw bytes")
	}
}
