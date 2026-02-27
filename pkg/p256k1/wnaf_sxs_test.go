//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"crypto/rand"
	"testing"

	"p256k1.mleku.dev/wnaf"
)

// TestWNAFSXS compares the standalone wnaf.Encode against the internal Scalar.wNAF
// method to verify they produce identical output for random 256-bit scalars.
func TestWNAFSXS(t *testing.T) {
	const iterations = 1000

	for w := uint(2); w <= 8; w++ {
		t.Run("w="+itoa(int(w)), func(t *testing.T) {
			for i := range iterations {
				// Generate random scalar bytes and load into Scalar
				// (setB32 may reduce mod n, so use s.d directly for both paths)
				var buf [32]byte
				if _, err := rand.Read(buf[:]); err != nil {
					t.Fatal(err)
				}

				var s Scalar
				s.setB32(buf[:])

				// Use the (possibly reduced) limbs for both
				standaloneResult := wnaf.Encode(s.d, int(w))

				var internalResult [257]int8
				internalBits := s.wNAF(&internalResult, w)

				// Compare digit-for-digit
				standaloneBits := standaloneResult.Len
				if standaloneBits != internalBits {
					t.Fatalf("iteration %d, w=%d: bit count mismatch: standalone=%d internal=%d",
						i, w, standaloneBits, internalBits)
				}

				for j := range 257 {
					if standaloneResult.D[j] != internalResult[j] {
						t.Fatalf("iteration %d, w=%d: digit mismatch at position %d: standalone=%d internal=%d",
							i, w, j, standaloneResult.D[j], internalResult[j])
					}
				}
			}
		})
	}
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
