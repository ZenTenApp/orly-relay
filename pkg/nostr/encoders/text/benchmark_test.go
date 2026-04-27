package text

import (
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"github.com/minio/sha256-simd"
	"lukechampine.com/frand"
)

func createTestData() []byte {
	return []byte(`some text content with line breaks and tabs and other stuff, and also some < > & " ' / \ control chars \u0000 \u001f`)
}

func createTestDataLarge() []byte {
	data := make([]byte, 8192)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

func createTestHexArray() [][]byte {
	ha := make([][]byte, 20)
	h := make([]byte, sha256.Size)
	frand.Read(h)
	for i := range ha {
		hh := sha256.Sum256(h)
		h = hh[:]
		ha[i] = make([]byte, sha256.Size)
		copy(ha[i], h)
	}
	return ha
}

func BenchmarkNostrEscape(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestData()
			dst := make([]byte, 0, len(src)*2)
			for i := 0; i < b.N; i++ {
				dst = NostrEscape(dst[:0], src)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestDataLarge()
			dst := make([]byte, 0, len(src)*2)
			for i := 0; i < b.N; i++ {
				dst = NostrEscape(dst[:0], src)
			}
		},
	)
	b.Run(
		"NoEscapes", func(b *testing.B) {
			b.ReportAllocs()
			src := []byte("this is a normal string with no special characters")
			dst := make([]byte, 0, len(src))
			for i := 0; i < b.N; i++ {
				dst = NostrEscape(dst[:0], src)
			}
		},
	)
	b.Run(
		"ManyEscapes", func(b *testing.B) {
			b.ReportAllocs()
			src := []byte("\"test\"\n\t\r\b\f\\control\x00\x01\x02")
			dst := make([]byte, 0, len(src)*3)
			for i := 0; i < b.N; i++ {
				dst = NostrEscape(dst[:0], src)
			}
		},
	)
}

func BenchmarkNostrUnescape(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestData()
			escaped := NostrEscape(nil, src)
			for i := 0; i < b.N; i++ {
				escapedCopy := make([]byte, len(escaped))
				copy(escapedCopy, escaped)
				_ = NostrUnescape(escapedCopy)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestDataLarge()
			escaped := NostrEscape(nil, src)
			for i := 0; i < b.N; i++ {
				escapedCopy := make([]byte, len(escaped))
				copy(escapedCopy, escaped)
				_ = NostrUnescape(escapedCopy)
			}
		},
	)
}

func BenchmarkRoundTripEscape(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestData()
			for i := 0; i < b.N; i++ {
				escaped := NostrEscape(nil, src)
				escapedCopy := make([]byte, len(escaped))
				copy(escapedCopy, escaped)
				_ = NostrUnescape(escapedCopy)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestDataLarge()
			for i := 0; i < b.N; i++ {
				escaped := NostrEscape(nil, src)
				escapedCopy := make([]byte, len(escaped))
				copy(escapedCopy, escaped)
				_ = NostrUnescape(escapedCopy)
			}
		},
	)
}

func BenchmarkJSONKey(b *testing.B) {
	b.ReportAllocs()
	key := []byte("testkey")
	dst := make([]byte, 0, 20)
	for i := 0; i < b.N; i++ {
		dst = JSONKey(dst[:0], key)
	}
}

func BenchmarkUnmarshalHex(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			h := make([]byte, sha256.Size)
			frand.Read(h)
			hexStr := hex.EncAppend(nil, h)
			quoted := AppendQuote(nil, hexStr, Noop)
			for i := 0; i < b.N; i++ {
				_, _, _ = UnmarshalHex(quoted)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			h := make([]byte, 1024)
			frand.Read(h)
			hexStr := hex.EncAppend(nil, h)
			quoted := AppendQuote(nil, hexStr, Noop)
			for i := 0; i < b.N; i++ {
				_, _, _ = UnmarshalHex(quoted)
			}
		},
	)
}

func BenchmarkUnmarshalQuoted(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestData()
			quoted := AppendQuote(nil, src, NostrEscape)
			for i := 0; i < b.N; i++ {
				quotedCopy := make([]byte, len(quoted))
				copy(quotedCopy, quoted)
				_, _, _ = UnmarshalQuoted(quotedCopy)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestDataLarge()
			quoted := AppendQuote(nil, src, NostrEscape)
			for i := 0; i < b.N; i++ {
				quotedCopy := make([]byte, len(quoted))
				copy(quotedCopy, quoted)
				_, _, _ = UnmarshalQuoted(quotedCopy)
			}
		},
	)
}

func BenchmarkMarshalHexArray(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			ha := createTestHexArray()
			dst := make([]byte, 0, len(ha)*sha256.Size*3)
			for i := 0; i < b.N; i++ {
				dst = MarshalHexArray(dst[:0], ha)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			ha := make([][]byte, 100)
			h := make([]byte, sha256.Size)
			frand.Read(h)
			for i := range ha {
				hh := sha256.Sum256(h)
				h = hh[:]
				ha[i] = make([]byte, sha256.Size)
				copy(ha[i], h)
			}
			dst := make([]byte, 0, len(ha)*sha256.Size*3)
			for i := 0; i < b.N; i++ {
				dst = MarshalHexArray(dst[:0], ha)
			}
		},
	)
}

func BenchmarkUnmarshalHexArray(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			ha := createTestHexArray()
			marshaled := MarshalHexArray(nil, ha)
			for i := 0; i < b.N; i++ {
				marshaledCopy := make([]byte, len(marshaled))
				copy(marshaledCopy, marshaled)
				_, _, _ = UnmarshalHexArray(marshaledCopy, sha256.Size)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			ha := make([][]byte, 100)
			h := make([]byte, sha256.Size)
			frand.Read(h)
			for i := range ha {
				hh := sha256.Sum256(h)
				h = hh[:]
				ha[i] = make([]byte, sha256.Size)
				copy(ha[i], h)
			}
			marshaled := MarshalHexArray(nil, ha)
			for i := 0; i < b.N; i++ {
				marshaledCopy := make([]byte, len(marshaled))
				copy(marshaledCopy, marshaled)
				_, _, _ = UnmarshalHexArray(marshaledCopy, sha256.Size)
			}
		},
	)
}

func BenchmarkUnmarshalStringArray(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			strings := [][]byte{
				[]byte("string1"),
				[]byte("string2"),
				[]byte("string3"),
			}
			dst := make([]byte, 0, 100)
			dst = append(dst, '[')
			for i, s := range strings {
				dst = AppendQuote(dst, s, NostrEscape)
				if i < len(strings)-1 {
					dst = append(dst, ',')
				}
			}
			dst = append(dst, ']')
			for i := 0; i < b.N; i++ {
				dstCopy := make([]byte, len(dst))
				copy(dstCopy, dst)
				_, _, _ = UnmarshalStringArray(dstCopy)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			strings := make([][]byte, 100)
			for i := range strings {
				strings[i] = []byte("test string " + string(rune(i)))
			}
			dst := make([]byte, 0, 2000)
			dst = append(dst, '[')
			for i, s := range strings {
				dst = AppendQuote(dst, s, NostrEscape)
				if i < len(strings)-1 {
					dst = append(dst, ',')
				}
			}
			dst = append(dst, ']')
			for i := 0; i < b.N; i++ {
				dstCopy := make([]byte, len(dst))
				copy(dstCopy, dst)
				_, _, _ = UnmarshalStringArray(dstCopy)
			}
		},
	)
}

func BenchmarkAppendQuote(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestData()
			dst := make([]byte, 0, len(src)*2)
			for i := 0; i < b.N; i++ {
				dst = AppendQuote(dst[:0], src, NostrEscape)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			src := createTestDataLarge()
			dst := make([]byte, 0, len(src)*2)
			for i := 0; i < b.N; i++ {
				dst = AppendQuote(dst[:0], src, NostrEscape)
			}
		},
	)
	b.Run(
		"NoEscape", func(b *testing.B) {
			b.ReportAllocs()
			src := []byte("normal string")
			dst := make([]byte, 0, len(src)+2)
			for i := 0; i < b.N; i++ {
				dst = AppendQuote(dst[:0], src, Noop)
			}
		},
	)
}

func BenchmarkAppendList(b *testing.B) {
	b.Run(
		"Small", func(b *testing.B) {
			b.ReportAllocs()
			src := [][]byte{
				[]byte("item1"),
				[]byte("item2"),
				[]byte("item3"),
			}
			dst := make([]byte, 0, 50)
			for i := 0; i < b.N; i++ {
				dst = AppendList(dst[:0], src, ',', NostrEscape)
			}
		},
	)
	b.Run(
		"Large", func(b *testing.B) {
			b.ReportAllocs()
			src := make([][]byte, 100)
			for i := range src {
				src[i] = []byte("item" + string(rune(i)))
			}
			dst := make([]byte, 0, 2000)
			for i := 0; i < b.N; i++ {
				dst = AppendList(dst[:0], src, ',', NostrEscape)
			}
		},
	)
}

func BenchmarkMarshalBool(b *testing.B) {
	b.ReportAllocs()
	dst := make([]byte, 0, 10)
	for i := 0; i < b.N; i++ {
		dst = MarshalBool(dst[:0], i%2 == 0)
	}
}

func BenchmarkUnmarshalBool(b *testing.B) {
	b.Run(
		"True", func(b *testing.B) {
			b.ReportAllocs()
			src := []byte("true")
			for i := 0; i < b.N; i++ {
				srcCopy := make([]byte, len(src))
				copy(srcCopy, src)
				_, _, _ = UnmarshalBool(srcCopy)
			}
		},
	)
	b.Run(
		"False", func(b *testing.B) {
			b.ReportAllocs()
			src := []byte("false")
			for i := 0; i < b.N; i++ {
				srcCopy := make([]byte, len(src))
				copy(srcCopy, src)
				_, _, _ = UnmarshalBool(srcCopy)
			}
		},
	)
}
