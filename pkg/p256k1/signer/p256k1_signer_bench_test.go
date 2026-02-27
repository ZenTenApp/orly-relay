//go:build !js && !wasm && !tinygo && !wasm32

package signer

import (
	"crypto/rand"
	"testing"

	"next.orly.dev/pkg/p256k1"
)

// BenchmarkP256K1Signer_Generate benchmarks key generation
func BenchmarkP256K1Signer_Generate(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := NewP256K1Signer()
		if err := s.Generate(); err != nil {
			b.Fatal(err)
		}
		s.Zero()
	}
}

// BenchmarkP256K1Signer_InitSec benchmarks secret key initialization
func BenchmarkP256K1Signer_InitSec(b *testing.B) {
	// Pre-generate a secret key
	sec := make([]byte, 32)
	rand.Read(sec)
	
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := NewP256K1Signer()
		if err := s.InitSec(sec); err != nil {
			b.Fatal(err)
		}
		s.Zero()
	}
}

// BenchmarkP256K1Signer_InitPub benchmarks public key initialization
func BenchmarkP256K1Signer_InitPub(b *testing.B) {
	// Pre-generate a public key
	kp, _ := p256k1.KeyPairGenerate()
	xonly, _ := kp.XOnlyPubkey()
	pub := xonly.Serialize()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := NewP256K1Signer()
		if err := s.InitPub(pub[:]); err != nil {
			b.Fatal(err)
		}
		s.Zero()
	}
}

// BenchmarkP256K1Signer_Sign benchmarks signing
func BenchmarkP256K1Signer_Sign(b *testing.B) {
	s := NewP256K1Signer()
	if err := s.Generate(); err != nil {
		b.Fatal(err)
	}
	defer s.Zero()

	msg := make([]byte, 32)
	rand.Read(msg)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := s.Sign(msg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkP256K1Signer_Verify benchmarks verification
func BenchmarkP256K1Signer_Verify(b *testing.B) {
	s := NewP256K1Signer()
	if err := s.Generate(); err != nil {
		b.Fatal(err)
	}
	defer s.Zero()

	msg := make([]byte, 32)
	rand.Read(msg)
	sig, _ := s.Sign(msg)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := s.Verify(msg, sig); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkP256K1Signer_ECDH benchmarks ECDH computation
func BenchmarkP256K1Signer_ECDH(b *testing.B) {
	s1 := NewP256K1Signer()
	if err := s1.Generate(); err != nil {
		b.Fatal(err)
	}
	defer s1.Zero()

	s2 := NewP256K1Signer()
	if err := s2.Generate(); err != nil {
		b.Fatal(err)
	}
	defer s2.Zero()

	pub2 := s2.Pub()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := s1.ECDH(pub2); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkP256K1Signer_Pub benchmarks public key retrieval
func BenchmarkP256K1Signer_Pub(b *testing.B) {
	s := NewP256K1Signer()
	if err := s.Generate(); err != nil {
		b.Fatal(err)
	}
	defer s.Zero()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.Pub()
	}
}

// BenchmarkP256K1Gen_Generate benchmarks Gen.Generate
func BenchmarkP256K1Gen_Generate(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g := NewP256K1Gen()
		if _, err := g.Generate(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkP256K1Gen_Negate benchmarks Gen.Negate
func BenchmarkP256K1Gen_Negate(b *testing.B) {
	g := NewP256K1Gen()
	if _, err := g.Generate(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g.Negate()
	}
}

// BenchmarkP256K1Gen_KeyPairBytes benchmarks Gen.KeyPairBytes
func BenchmarkP256K1Gen_KeyPairBytes(b *testing.B) {
	g := NewP256K1Gen()
	if _, err := g.Generate(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = g.KeyPairBytes()
	}
}

