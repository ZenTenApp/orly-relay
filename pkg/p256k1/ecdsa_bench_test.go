package p256k1

import (
	"crypto/rand"
	"testing"
)

var (
	benchSeckey   []byte
	benchPubkey   PublicKey
	benchMsghash  []byte
	benchSignature ECDSASignature
)

func initBenchmarkData() {
	// Generate a fixed secret key for benchmarks
	benchSeckey = []byte{
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	}
	
	// Ensure it's valid
	var scalar Scalar
	for !scalar.setB32Seckey(benchSeckey) {
		if _, err := rand.Read(benchSeckey); err != nil {
			panic(err)
		}
	}
	
	// Create public key
	if err := ECPubkeyCreate(&benchPubkey, benchSeckey); err != nil {
		panic(err)
	}
	
	// Create message hash
	benchMsghash = make([]byte, 32)
	if _, err := rand.Read(benchMsghash); err != nil {
		panic(err)
	}
	
	// Create signature
	if err := ECDSASign(&benchSignature, benchMsghash, benchSeckey); err != nil {
		panic(err)
	}
}

func BenchmarkECDSASign(b *testing.B) {
	if benchSeckey == nil {
		initBenchmarkData()
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sig ECDSASignature
		ECDSASign(&sig, benchMsghash, benchSeckey)
	}
}

func BenchmarkECDSAVerify(b *testing.B) {
	if benchSeckey == nil {
		initBenchmarkData()
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ECDSAVerify(&benchSignature, benchMsghash, &benchPubkey)
	}
}

func BenchmarkECDSASignCompact(b *testing.B) {
	if benchSeckey == nil {
		initBenchmarkData()
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var compactSig ECDSASignatureCompact
		ECDSASignCompact(&compactSig, benchMsghash, benchSeckey)
	}
}

func BenchmarkECDSAVerifyCompact(b *testing.B) {
	if benchSeckey == nil {
		initBenchmarkData()
	}
	
	var compactSig ECDSASignatureCompact
	ECDSASignCompact(&compactSig, benchMsghash, benchSeckey)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ECDSAVerifyCompact(&compactSig, benchMsghash, &benchPubkey)
	}
}

func BenchmarkECSeckeyGenerate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ECSeckeyGenerate()
	}
}

func BenchmarkECKeyPairGenerate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ECKeyPairGenerate()
	}
}

func BenchmarkSHA256(b *testing.B) {
	data := make([]byte, 64)
	rand.Read(data)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := NewSHA256()
		h.Write(data)
		var result [32]byte
		h.Finalize(result[:])
		h.Clear()
	}
}

func BenchmarkHMACSHA256(b *testing.B) {
	key := make([]byte, 32)
	data := make([]byte, 64)
	rand.Read(key)
	rand.Read(data)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hmac := NewHMACSHA256(key)
		hmac.Write(data)
		var result [32]byte
		hmac.Finalize(result[:])
		hmac.Clear()
	}
}

func BenchmarkRFC6979(b *testing.B) {
	key := make([]byte, 64)
	rand.Read(key)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rng := NewRFC6979HMACSHA256(key)
		var nonce [32]byte
		rng.Generate(nonce[:])
		rng.Finalize()
		rng.Clear()
	}
}

func BenchmarkTaggedHash(b *testing.B) {
	tag := []byte("BIP0340/challenge")
	data := make([]byte, 32)
	rand.Read(data)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TaggedHash(tag, data)
	}
}


