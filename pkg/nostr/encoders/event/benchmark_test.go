package event

import (
	"bytes"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"lukechampine.com/frand"
)

// createTestEvent creates a realistic test event with proper signing
func createTestEvent() *E {
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		panic(err)
	}

	ev := New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind.TextNote.K

	// Create realistic tags
	ev.Tags = tag.NewS(
		tag.NewFromBytesSlice([]byte("t"), []byte("hashtag")),
		tag.NewFromBytesSlice([]byte("e"), hex.EncAppend(nil, frand.Bytes(32))),
		tag.NewFromBytesSlice([]byte("p"), hex.EncAppend(nil, frand.Bytes(32))),
	)

	// Create realistic content
	ev.Content = []byte(`This is a test event with some content that includes special characters like < > & and "quotes" and various other things that might need escaping.`)

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		panic(err)
	}

	return ev
}

// createLargeTestEvent creates a larger event with more tags and content
func createLargeTestEvent() *E {
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		panic(err)
	}

	ev := New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind.TextNote.K

	// Create many tags
	tags := tag.NewS()
	for i := 0; i < 20; i++ {
		tags.Append(
			tag.NewFromBytesSlice(
				[]byte("t"),
				[]byte("hashtag"+string(rune('0'+i))),
			),
		)
		if i%3 == 0 {
			tags.Append(
				tag.NewFromBytesSlice(
					[]byte("e"),
					hex.EncAppend(nil, frand.Bytes(32)),
				),
			)
		}
	}
	ev.Tags = tags

	// Large content
	content := make([]byte, 0, 4096)
	for i := 0; i < 50; i++ {
		content = append(
			content,
			[]byte("This is a longer piece of content that simulates real-world event content. ")...,
		)
		if i%10 == 0 {
			content = append(
				content, []byte("With special chars: < > & \" ' ")...,
			)
		}
	}
	ev.Content = content

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		panic(err)
	}

	return ev
}

// BenchmarkJSONMarshal benchmarks the JSON marshaling
func BenchmarkJSONMarshal(b *testing.B) {
	ev := createTestEvent()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ev.Marshal(nil)
	}
}

// BenchmarkJSONMarshalLarge benchmarks JSON marshaling with large events
func BenchmarkJSONMarshalLarge(b *testing.B) {
	ev := createLargeTestEvent()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ev.Marshal(nil)
	}
}

// BenchmarkJSONUnmarshal benchmarks JSON unmarshaling
func BenchmarkJSONUnmarshal(b *testing.B) {
	ev := createTestEvent()
	jsonData := ev.Marshal(nil)
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ev2 := New()
		_, err := ev2.Unmarshal(jsonData)
		if err != nil {
			b.Fatal(err)
		}
		ev2.Free()
	}
}

// BenchmarkBinaryMarshal benchmarks binary marshaling
func BenchmarkBinaryMarshal(b *testing.B) {
	ev := createTestEvent()
	defer ev.Free()

	buf := &bytes.Buffer{}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		ev.MarshalBinary(buf)
	}
}

// BenchmarkBinaryMarshalLarge benchmarks binary marshaling with large events
func BenchmarkBinaryMarshalLarge(b *testing.B) {
	ev := createLargeTestEvent()
	defer ev.Free()

	buf := &bytes.Buffer{}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		ev.MarshalBinary(buf)
	}
}

// BenchmarkBinaryUnmarshal benchmarks binary unmarshaling
func BenchmarkBinaryUnmarshal(b *testing.B) {
	ev := createTestEvent()
	buf := &bytes.Buffer{}
	ev.MarshalBinary(buf)
	binaryData := buf.Bytes()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ev2 := New()
		reader := bytes.NewReader(binaryData)
		if err := ev2.UnmarshalBinary(reader); err != nil {
			b.Fatal(err)
		}
		ev2.Free()
	}
}

// BenchmarkCanonical benchmarks canonical encoding
func BenchmarkCanonical(b *testing.B) {
	ev := createTestEvent()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ev.ToCanonical(nil)
	}
}

// BenchmarkCanonicalLarge benchmarks canonical encoding with large events
func BenchmarkCanonicalLarge(b *testing.B) {
	ev := createLargeTestEvent()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ev.ToCanonical(nil)
	}
}

// BenchmarkGetIDBytes benchmarks ID generation (canonical + hash)
func BenchmarkGetIDBytes(b *testing.B) {
	ev := createTestEvent()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ev.GetIDBytes()
	}
}

// BenchmarkRoundTripJSON benchmarks JSON marshal/unmarshal round trip
func BenchmarkRoundTripJSON(b *testing.B) {
	ev := createTestEvent()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jsonData := ev.Marshal(nil)
		ev2 := New()
		_, err := ev2.Unmarshal(jsonData)
		if err != nil {
			b.Fatal(err)
		}
		ev2.Free()
	}
}

// BenchmarkRoundTripBinary benchmarks binary marshal/unmarshal round trip
func BenchmarkRoundTripBinary(b *testing.B) {
	ev := createTestEvent()
	defer ev.Free()

	buf := &bytes.Buffer{}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		ev.MarshalBinary(buf)

		ev2 := New()
		reader := bytes.NewReader(buf.Bytes())
		if err := ev2.UnmarshalBinary(reader); err != nil {
			b.Fatal(err)
		}
		ev2.Free()
	}
}

// BenchmarkEstimateSize benchmarks size estimation
func BenchmarkEstimateSize(b *testing.B) {
	ev := createTestEvent()
	defer ev.Free()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ev.EstimateSize()
	}
}
