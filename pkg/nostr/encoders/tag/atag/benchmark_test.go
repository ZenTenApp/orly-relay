package atag

import (
	"testing"

	"git.smesh.lol/orly/pkg/nostr/crypto/ec/schnorr"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"lukechampine.com/frand"
)

func createTestATag() *T {
	return &T{
		Kind:   kind.New(1),
		Pubkey: frand.Bytes(schnorr.PubKeyBytesLen),
		DTag:   []byte("test-dtag"),
	}
}

func BenchmarkATagMarshal(b *testing.B) {
	b.ReportAllocs()
	t := createTestATag()
	dst := make([]byte, 0, 100)
	for i := 0; i < b.N; i++ {
		dst = t.Marshal(dst[:0])
	}
}

func BenchmarkATagUnmarshal(b *testing.B) {
	b.ReportAllocs()
	t := createTestATag()
	marshaled := t.Marshal(nil)
	for i := 0; i < b.N; i++ {
		marshaledCopy := make([]byte, len(marshaled))
		copy(marshaledCopy, marshaled)
		t2 := &T{}
		_, _ = t2.Unmarshal(marshaledCopy)
	}
}

func BenchmarkATagRoundTrip(b *testing.B) {
	b.ReportAllocs()
	t := createTestATag()
	for i := 0; i < b.N; i++ {
		marshaled := t.Marshal(nil)
		t2 := &T{}
		_, _ = t2.Unmarshal(marshaled)
	}
}
