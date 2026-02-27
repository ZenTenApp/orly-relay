package encryption

import (
	"testing"

	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"lukechampine.com/frand"
)

// createTestConversationKey creates a test conversation key
func createTestConversationKey() []byte {
	return frand.Bytes(32)
}

// createTestKeyPair creates a key pair for ECDH testing
func createTestKeyPair() (*p8k.Signer, []byte) {
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		panic(err)
	}
	return signer, signer.Pub()
}

// BenchmarkNIP44Encrypt benchmarks NIP-44 encryption
func BenchmarkNIP44Encrypt(b *testing.B) {
	conversationKey := createTestConversationKey()
	plaintext := []byte("This is a test message for encryption benchmarking")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Encrypt(conversationKey, plaintext, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP44EncryptSmall benchmarks encryption of small messages
func BenchmarkNIP44EncryptSmall(b *testing.B) {
	conversationKey := createTestConversationKey()
	plaintext := []byte("a")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Encrypt(conversationKey, plaintext, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP44EncryptLarge benchmarks encryption of large messages
func BenchmarkNIP44EncryptLarge(b *testing.B) {
	conversationKey := createTestConversationKey()
	plaintext := make([]byte, 4096)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Encrypt(conversationKey, plaintext, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP44Decrypt benchmarks NIP-44 decryption
func BenchmarkNIP44Decrypt(b *testing.B) {
	conversationKey := createTestConversationKey()
	plaintext := []byte("This is a test message for encryption benchmarking")
	ciphertext, err := Encrypt(conversationKey, plaintext, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Decrypt(conversationKey, ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP44DecryptSmall benchmarks decryption of small messages
func BenchmarkNIP44DecryptSmall(b *testing.B) {
	conversationKey := createTestConversationKey()
	plaintext := []byte("a")
	ciphertext, err := Encrypt(conversationKey, plaintext, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Decrypt(conversationKey, ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP44DecryptLarge benchmarks decryption of large messages
func BenchmarkNIP44DecryptLarge(b *testing.B) {
	conversationKey := createTestConversationKey()
	plaintext := make([]byte, 4096)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}
	ciphertext, err := Encrypt(conversationKey, plaintext, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Decrypt(conversationKey, ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP44RoundTrip benchmarks encrypt/decrypt round trip
func BenchmarkNIP44RoundTrip(b *testing.B) {
	conversationKey := createTestConversationKey()
	plaintext := []byte("This is a test message for encryption benchmarking")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ciphertext, err := Encrypt(conversationKey, plaintext, nil)
		if err != nil {
			b.Fatal(err)
		}
		_, err = Decrypt(conversationKey, ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP4Encrypt benchmarks NIP-4 encryption
func BenchmarkNIP4Encrypt(b *testing.B) {
	key := createTestConversationKey()
	msg := []byte("This is a test message for NIP-4 encryption benchmarking")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := EncryptNip4(msg, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNIP4Decrypt benchmarks NIP-4 decryption
func BenchmarkNIP4Decrypt(b *testing.B) {
	key := createTestConversationKey()
	msg := []byte("This is a test message for NIP-4 encryption benchmarking")
	ciphertext, err := EncryptNip4(msg, key)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		decrypted, err := DecryptNip4(ciphertext, key)
		if err != nil {
			b.Fatal(err)
		}
		if len(decrypted) == 0 {
			b.Fatal("decrypted message is empty")
		}
	}
}

// BenchmarkNIP4RoundTrip benchmarks NIP-4 encrypt/decrypt round trip
func BenchmarkNIP4RoundTrip(b *testing.B) {
	key := createTestConversationKey()
	msg := []byte("This is a test message for NIP-4 encryption benchmarking")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ciphertext, err := EncryptNip4(msg, key)
		if err != nil {
			b.Fatal(err)
		}
		_, err = DecryptNip4(ciphertext, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateConversationKey benchmarks conversation key generation
func BenchmarkGenerateConversationKey(b *testing.B) {
	signer1, _ := createTestKeyPair()
	signer2, _ := createTestKeyPair()

	// Get compressed public keys
	pub1, err := signer1.PubCompressed()
	if err != nil {
		b.Fatal(err)
	}
	pub2, err := signer2.PubCompressed()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := GenerateConversationKey(signer1.Sec(), pub1)
		if err != nil {
			b.Fatal(err)
		}
		// Use signer2's pubkey for next iteration to vary inputs
		pub1 = pub2
	}
}

// BenchmarkCalcPadding benchmarks padding calculation
func BenchmarkCalcPadding(b *testing.B) {
	sizes := []int{
		1, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		size := sizes[i%len(sizes)]
		_ = calcPadding(size)
	}
}

// BenchmarkGetKeys benchmarks key derivation
func BenchmarkGetKeys(b *testing.B) {
	conversationKey := createTestConversationKey()
	nonce := frand.Bytes(32)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, _, err := MessageKeys(conversationKey, nonce)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncryptInternal benchmarks internal encrypt function
func BenchmarkEncryptInternal(b *testing.B) {
	key := createTestConversationKey()
	nonce := frand.Bytes(12)
	message := make([]byte, 256)
	for i := range message {
		message[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := chacha20_(key, nonce, message)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSHA256Hmac benchmarks HMAC calculation
func BenchmarkSHA256Hmac(b *testing.B) {
	key := createTestConversationKey()
	nonce := frand.Bytes(32)
	ciphertext := make([]byte, 256)
	for i := range ciphertext {
		ciphertext[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := sha256Hmac(key, ciphertext, nonce)
		if err != nil {
			b.Fatal(err)
		}
	}
}
