package filter

import (
	"testing"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"github.com/minio/sha256-simd"
	"lukechampine.com/frand"
)

// createTestFilter creates a realistic test filter
func createTestFilter() *F {
	f := New()

	// Add some IDs
	for i := 0; i < 5; i++ {
		id := frand.Bytes(sha256.Size)
		f.Ids.T = append(f.Ids.T, id)
	}

	// Add some kinds
	f.Kinds.K = append(f.Kinds.K, kind.New(1), kind.New(6), kind.New(7))

	// Add some authors
	for i := 0; i < 3; i++ {
		signer := p8k.MustNew()
		if err := signer.Generate(); err != nil {
			panic(err)
		}
		f.Authors.T = append(f.Authors.T, signer.Pub())
	}

	// Add some tags
	f.Tags.Append(tag.NewFromBytesSlice([]byte("t"), []byte("hashtag")))
	f.Tags.Append(
		tag.NewFromBytesSlice(
			[]byte("e"), hex.EncAppend(nil, frand.Bytes(32)),
		),
	)
	f.Tags.Append(
		tag.NewFromBytesSlice(
			[]byte("p"), hex.EncAppend(nil, frand.Bytes(32)),
		),
	)

	// Add timestamps
	f.Since = timestamp.FromUnix(time.Now().Unix() - 86400)
	f.Until = timestamp.Now()

	// Add limit
	limit := uint(100)
	f.Limit = &limit

	// Add search
	f.Search = []byte("test search query")

	return f
}

// createComplexFilter creates a more complex filter with many tags
func createComplexFilter() *F {
	f := New()

	// Add many IDs
	for i := 0; i < 20; i++ {
		id := frand.Bytes(sha256.Size)
		f.Ids.T = append(f.Ids.T, id)
	}

	// Add many kinds
	for i := 0; i < 10; i++ {
		f.Kinds.K = append(f.Kinds.K, kind.New(uint16(i)))
	}

	// Add many authors
	for i := 0; i < 15; i++ {
		signer := p8k.MustNew()
		if err := signer.Generate(); err != nil {
			panic(err)
		}
		f.Authors.T = append(f.Authors.T, signer.Pub())
	}

	// Add many tags
	for b := 'a'; b <= 'z'; b++ {
		for i := 0; i < 3; i++ {
			f.Tags.Append(
				tag.NewFromBytesSlice(
					[]byte{byte(b)},
					hex.EncAppend(nil, frand.Bytes(32)),
				),
			)
		}
	}

	f.Since = timestamp.FromUnix(time.Now().Unix() - 86400)
	f.Until = timestamp.Now()
	limit := uint(1000)
	f.Limit = &limit
	f.Search = []byte("complex search query with multiple words")

	return f
}

// createTestEvent creates a test event for matching
func createTestEvent() *event.E {
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		panic(err)
	}

	ev := event.New()
	ev.Pubkey = signer.Pub()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind.TextNote.K

	ev.Tags = tag.NewS(
		tag.NewFromBytesSlice([]byte("t"), []byte("hashtag")),
		tag.NewFromBytesSlice([]byte("e"), hex.EncAppend(nil, frand.Bytes(32))),
	)

	ev.Content = []byte("Test event content")

	if err := ev.Sign(signer); err != nil {
		panic(err)
	}

	return ev
}

// BenchmarkFilterMarshal benchmarks filter marshaling
func BenchmarkFilterMarshal(b *testing.B) {
	f := createTestFilter()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = f.Marshal(nil)
	}
}

// BenchmarkFilterMarshalComplex benchmarks marshaling complex filters
func BenchmarkFilterMarshalComplex(b *testing.B) {
	f := createComplexFilter()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = f.Marshal(nil)
	}
}

// BenchmarkFilterUnmarshal benchmarks filter unmarshaling
func BenchmarkFilterUnmarshal(b *testing.B) {
	f := createTestFilter()
	jsonData := f.Marshal(nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		f2 := New()
		_, err := f2.Unmarshal(jsonData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFilterSort benchmarks filter sorting
func BenchmarkFilterSort(b *testing.B) {
	f := createTestFilter()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		f.Sort()
	}
}

// BenchmarkFilterSortComplex benchmarks sorting complex filters
func BenchmarkFilterSortComplex(b *testing.B) {
	f := createComplexFilter()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		f.Sort()
	}
}

// BenchmarkFilterMatches benchmarks filter matching
func BenchmarkFilterMatches(b *testing.B) {
	f := createTestFilter()
	ev := createTestEvent()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = f.Matches(ev)
	}
}

// BenchmarkFilterMatchesIgnoringTimestamp benchmarks matching without timestamp check
func BenchmarkFilterMatchesIgnoringTimestamp(b *testing.B) {
	f := createTestFilter()
	ev := createTestEvent()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = f.MatchesIgnoringTimestampConstraints(ev)
	}
}

// BenchmarkFilterRoundTrip benchmarks marshal/unmarshal round trip
func BenchmarkFilterRoundTrip(b *testing.B) {
	f := createTestFilter()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		jsonData := f.Marshal(nil)
		f2 := New()
		_, err := f2.Unmarshal(jsonData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFilterSliceMarshal benchmarks filter slice marshaling
func BenchmarkFilterSliceMarshal(b *testing.B) {
	fs := NewS()
	for i := 0; i < 5; i++ {
		*fs = append(*fs, createTestFilter())
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = fs.Marshal(nil)
	}
}

// BenchmarkFilterSliceUnmarshal benchmarks filter slice unmarshaling
func BenchmarkFilterSliceUnmarshal(b *testing.B) {
	fs := NewS()
	for i := 0; i < 5; i++ {
		*fs = append(*fs, createTestFilter())
	}
	jsonData := fs.Marshal(nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		fs2 := NewS()
		_, err := fs2.Unmarshal(jsonData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFilterSliceMatch benchmarks filter slice matching
func BenchmarkFilterSliceMatch(b *testing.B) {
	fs := NewS()
	for i := 0; i < 5; i++ {
		*fs = append(*fs, createTestFilter())
	}
	ev := createTestEvent()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = fs.Match(ev)
	}
}
