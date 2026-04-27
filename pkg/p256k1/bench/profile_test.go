//go:build !js && !wasm && !tinygo && !wasm32

package bench

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"

	"git.smesh.lol/orly/pkg/p256k1"
)

// Generate memory profiles for optimization

func TestGenerateMemProfile(t *testing.T) {
	// Setup
	seckey := make([]byte, 32)
	rand.Read(seckey)

	kp, err := p256k1.KeyPairCreate(seckey)
	if err != nil {
		t.Fatal(err)
	}

	xonly, _ := kp.XOnlyPubkey()
	pubkey := kp.Pubkey()

	msg := make([]byte, 32)
	rand.Read(msg)

	var schnorrSig [64]byte
	p256k1.SchnorrSign(schnorrSig[:], msg, kp, nil)

	var ecdsaSig p256k1.ECDSASignature
	p256k1.ECDSASign(&ecdsaSig, msg, seckey)

	// Force GC to get clean state
	runtime.GC()

	// Create memory profile file
	f, err := os.Create(filepath.Join(t.TempDir(), "mem.prof"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Run operations many times to accumulate allocations
	iterations := 10000

	t.Log("Running Schnorr Sign...")
	for i := 0; i < iterations; i++ {
		p256k1.SchnorrSign(schnorrSig[:], msg, kp, nil)
	}

	t.Log("Running Schnorr Verify...")
	for i := 0; i < iterations; i++ {
		p256k1.SchnorrVerify(schnorrSig[:], msg, xonly)
	}

	t.Log("Running ECDSA Sign...")
	for i := 0; i < iterations; i++ {
		p256k1.ECDSASign(&ecdsaSig, msg, seckey)
	}

	t.Log("Running ECDSA Verify...")
	for i := 0; i < iterations; i++ {
		p256k1.ECDSAVerify(&ecdsaSig, msg, pubkey)
	}

	// Write heap profile
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatal(err)
	}

	t.Log("Memory profile written to", f.Name())
}

// TestAllocationsBreakdown shows allocation counts per operation
func TestAllocationsBreakdown(t *testing.T) {
	// Setup
	seckey := make([]byte, 32)
	rand.Read(seckey)

	kp, err := p256k1.KeyPairCreate(seckey)
	if err != nil {
		t.Fatal(err)
	}

	xonly, _ := kp.XOnlyPubkey()
	pubkey := kp.Pubkey()

	msg := make([]byte, 32)
	rand.Read(msg)

	var schnorrSig [64]byte
	p256k1.SchnorrSign(schnorrSig[:], msg, kp, nil)

	var ecdsaSig p256k1.ECDSASignature
	p256k1.ECDSASign(&ecdsaSig, msg, seckey)

	// Measure allocations for each operation
	var m1, m2 runtime.MemStats

	// Schnorr Sign
	runtime.GC()
	runtime.ReadMemStats(&m1)
	for i := 0; i < 1000; i++ {
		p256k1.SchnorrSign(schnorrSig[:], msg, kp, nil)
	}
	runtime.ReadMemStats(&m2)
	t.Logf("Schnorr Sign: %d allocs, %d bytes/op", (m2.Mallocs-m1.Mallocs)/1000, (m2.TotalAlloc-m1.TotalAlloc)/1000)

	// Schnorr Verify
	runtime.GC()
	runtime.ReadMemStats(&m1)
	for i := 0; i < 1000; i++ {
		p256k1.SchnorrVerify(schnorrSig[:], msg, xonly)
	}
	runtime.ReadMemStats(&m2)
	t.Logf("Schnorr Verify: %d allocs, %d bytes/op", (m2.Mallocs-m1.Mallocs)/1000, (m2.TotalAlloc-m1.TotalAlloc)/1000)

	// ECDSA Sign
	runtime.GC()
	runtime.ReadMemStats(&m1)
	for i := 0; i < 1000; i++ {
		p256k1.ECDSASign(&ecdsaSig, msg, seckey)
	}
	runtime.ReadMemStats(&m2)
	t.Logf("ECDSA Sign: %d allocs, %d bytes/op", (m2.Mallocs-m1.Mallocs)/1000, (m2.TotalAlloc-m1.TotalAlloc)/1000)

	// ECDSA Verify
	runtime.GC()
	runtime.ReadMemStats(&m1)
	for i := 0; i < 1000; i++ {
		p256k1.ECDSAVerify(&ecdsaSig, msg, pubkey)
	}
	runtime.ReadMemStats(&m2)
	t.Logf("ECDSA Verify: %d allocs, %d bytes/op", (m2.Mallocs-m1.Mallocs)/1000, (m2.TotalAlloc-m1.TotalAlloc)/1000)
}
