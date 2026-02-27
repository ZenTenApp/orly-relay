package tag

import (
	"testing"

	"next.orly.dev/pkg/nostr/encoders/hex"
	"lukechampine.com/frand"
)

func createTestTag() *T {
	t := New()
	t.T = [][]byte{
		[]byte("e"),
		hex.EncAppend(nil, frand.Bytes(32)),
	}
	return t
}

func createTestTagWithManyFields() *T {
	t := New()
	t.T = [][]byte{
		[]byte("p"),
		hex.EncAppend(nil, frand.Bytes(32)),
		[]byte("wss://relay.example.com"),
		[]byte("auth"),
		[]byte("read"),
		[]byte("write"),
	}
	return t
}

func createTestTags() *S {
	tags := NewSWithCap(10)
	tags.Append(
		NewFromBytesSlice([]byte("e"), hex.EncAppend(nil, frand.Bytes(32))),
		NewFromBytesSlice([]byte("p"), hex.EncAppend(nil, frand.Bytes(32))),
		NewFromBytesSlice([]byte("t"), []byte("hashtag")),
		NewFromBytesSlice([]byte("t"), []byte("nostr")),
		NewFromBytesSlice([]byte("p"), hex.EncAppend(nil, frand.Bytes(32))),
	)
	return tags
}

func createTestTagsLarge() *S {
	tags := NewSWithCap(100)
	for i := 0; i < 100; i++ {
		if i%3 == 0 {
			tags.Append(
				NewFromBytesSlice(
					[]byte("e"), hex.EncAppend(nil, frand.Bytes(32)),
				),
			)
		} else if i%3 == 1 {
			tags.Append(
				NewFromBytesSlice(
					[]byte("p"), hex.EncAppend(nil, frand.Bytes(32)),
				),
			)
		} else {
			tags.Append(NewFromBytesSlice([]byte("t"), []byte("hashtag")))
		}
	}
	return tags
}

func BenchmarkTagMarshal(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTag()
			dst := make([]byte, 0, 100)
			for i := 0; i < b.N; i++ {
				dst = t.Marshal(dst[:0])
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTagWithManyFields()
			dst := make([]byte, 0, 200)
			for i := 0; i < b.N; i++ {
				dst = t.Marshal(dst[:0])
			}
		},
	)
}

func BenchmarkTagUnmarshal(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTag()
			marshaled := t.Marshal(nil)
			for i := 0; i < b.N; i++ {
				marshaledCopy := make([]byte, len(marshaled))
				copy(marshaledCopy, marshaled)
				t2 := New()
				_, _ = t2.Unmarshal(marshaledCopy)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTagWithManyFields()
			marshaled := t.Marshal(nil)
			for i := 0; i < b.N; i++ {
				marshaledCopy := make([]byte, len(marshaled))
				copy(marshaledCopy, marshaled)
				t2 := New()
				_, _ = t2.Unmarshal(marshaledCopy)
			}
		},
	)
}

func BenchmarkTagRoundTrip(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTag()
			for i := 0; i < b.N; i++ {
				marshaled := t.Marshal(nil)
				t2 := New()
				_, _ = t2.Unmarshal(marshaled)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTagWithManyFields()
			for i := 0; i < b.N; i++ {
				marshaled := t.Marshal(nil)
				t2 := New()
				_, _ = t2.Unmarshal(marshaled)
			}
		},
	)
}

func BenchmarkTagContains(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTag()
			search := []byte("e")
			for i := 0; i < b.N; i++ {
				_ = t.Contains(search)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTagWithManyFields()
			search := []byte("p")
			for i := 0; i < b.N; i++ {
				_ = t.Contains(search)
			}
		},
	)
}

func BenchmarkTagToSliceOfStrings(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTag()
			for i := 0; i < b.N; i++ {
				_ = t.ToSliceOfStrings()
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			t := createTestTagWithManyFields()
			for i := 0; i < b.N; i++ {
				_ = t.ToSliceOfStrings()
			}
		},
	)
}

func BenchmarkTagsMarshal(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTags()
			dst := make([]byte, 0, 500)
			for i := 0; i < b.N; i++ {
				dst = tags.Marshal(dst[:0])
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTagsLarge()
			dst := make([]byte, 0, 10000)
			for i := 0; i < b.N; i++ {
				dst = tags.Marshal(dst[:0])
			}
		},
	)
}

func BenchmarkTagsUnmarshal(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTags()
			marshaled := tags.Marshal(nil)
			for i := 0; i < b.N; i++ {
				marshaledCopy := make([]byte, len(marshaled))
				copy(marshaledCopy, marshaled)
				tags2 := NewSWithCap(10)
				_, _ = tags2.Unmarshal(marshaledCopy)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTagsLarge()
			marshaled := tags.Marshal(nil)
			for i := 0; i < b.N; i++ {
				marshaledCopy := make([]byte, len(marshaled))
				copy(marshaledCopy, marshaled)
				tags2 := NewSWithCap(100)
				_, _ = tags2.Unmarshal(marshaledCopy)
			}
		},
	)
}

func BenchmarkTagsRoundTrip(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTags()
			for i := 0; i < b.N; i++ {
				marshaled := tags.Marshal(nil)
				tags2 := NewSWithCap(10)
				_, _ = tags2.Unmarshal(marshaled)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTagsLarge()
			for i := 0; i < b.N; i++ {
				marshaled := tags.Marshal(nil)
				tags2 := NewSWithCap(100)
				_, _ = tags2.Unmarshal(marshaled)
			}
		},
	)
}

func BenchmarkTagsContainsAny(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTags()
			values := [][]byte{[]byte("hashtag"), []byte("nostr")}
			for i := 0; i < b.N; i++ {
				_ = tags.ContainsAny([]byte("t"), values)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTagsLarge()
			values := [][]byte{[]byte("hashtag")}
			for i := 0; i < b.N; i++ {
				_ = tags.ContainsAny([]byte("t"), values)
			}
		},
	)
}

func BenchmarkTagsGetFirst(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTags()
			for i := 0; i < b.N; i++ {
				_ = tags.GetFirst([]byte("e"))
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTagsLarge()
			for i := 0; i < b.N; i++ {
				_ = tags.GetFirst([]byte("e"))
			}
		},
	)
}

func BenchmarkTagsGetAll(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTags()
			for i := 0; i < b.N; i++ {
				_ = tags.GetAll([]byte("p"))
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTagsLarge()
			for i := 0; i < b.N; i++ {
				_ = tags.GetAll([]byte("p"))
			}
		},
	)
}

func BenchmarkTagsToSliceOfSliceOfStrings(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTags()
			for i := 0; i < b.N; i++ {
				_ = tags.ToSliceOfSliceOfStrings()
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			tags := createTestTagsLarge()
			for i := 0; i < b.N; i++ {
				_ = tags.ToSliceOfSliceOfStrings()
			}
		},
	)
}

func BenchmarkTagEquals(b *testing.B) {
	b.Run(
		"BinaryToBinary", func(b *testing.B) {
			b.ReportAllocs()
			// Create two tags with same binary-encoded value
			tag1 := New()
			_, _ = tag1.Unmarshal([]byte(`["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`))
			tag2 := New()
			_, _ = tag2.Unmarshal([]byte(`["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tag1.Equals(tag2)
			}
		},
	)
	b.Run(
		"BinaryToHex", func(b *testing.B) {
			b.ReportAllocs()
			// One binary-encoded, one hex (simulate comparison with non-optimized tag)
			tag1 := New()
			_, _ = tag1.Unmarshal([]byte(`["e","0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]`))
			// Create hex version manually (simulating older format)
			tag2 := NewFromBytesSlice(
				[]byte("e"),
				[]byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tag1.Equals(tag2)
			}
		},
	)
	b.Run(
		"HexToHex", func(b *testing.B) {
			b.ReportAllocs()
			// Both hex (non-optimized tags)
			tag1 := NewFromBytesSlice(
				[]byte("t"),
				[]byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			)
			tag2 := NewFromBytesSlice(
				[]byte("t"),
				[]byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
			)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tag1.Equals(tag2)
			}
		},
	)
}
