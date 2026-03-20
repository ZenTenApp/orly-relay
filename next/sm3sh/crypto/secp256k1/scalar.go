package secp256k1

// Scalar arithmetic modulo the curve order n.
// n = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141

// scalarAdd returns (a + b) mod n.
func scalarAdd(a, b Fe) Fe {
	var r Fe
	var carry uint64
	r[0], carry = addWithCarry(a[0], b[0], 0)
	r[1], carry = addWithCarry(a[1], b[1], carry)
	r[2], carry = addWithCarry(a[2], b[2], carry)
	r[3], carry = addWithCarry(a[3], b[3], carry)

	if carry != 0 || feCmp(r, curveN) >= 0 {
		var borrow uint64
		r[0], borrow = subWithBorrow(r[0], curveN[0], 0)
		r[1], borrow = subWithBorrow(r[1], curveN[1], borrow)
		r[2], borrow = subWithBorrow(r[2], curveN[2], borrow)
		r[3], _ = subWithBorrow(r[3], curveN[3], borrow)
	}
	return r
}

// scalarMul returns (a * b) mod n.
func scalarMul(a, b Fe) Fe {
	var t [8]uint64
	for i := 0; i < 4; i++ {
		var carry uint64
		for j := 0; j < 4; j++ {
			hi, lo := mul64(a[i], b[j])
			lo, c1 := addWithCarry(lo, t[i+j], 0)
			hi += c1
			lo, c2 := addWithCarry(lo, carry, 0)
			hi += c2
			t[i+j] = lo
			carry = hi
		}
		t[i+4] = carry
	}
	return scalarReduceFull(t)
}

// scalarNeg returns -a mod n.
func scalarNeg(a Fe) Fe {
	if a == feZero {
		return feZero
	}
	return scalarSub(curveN, a)
}

// scalarSub returns (a - b) mod n.
func scalarSub(a, b Fe) Fe {
	var r Fe
	var borrow uint64
	r[0], borrow = subWithBorrow(a[0], b[0], 0)
	r[1], borrow = subWithBorrow(r[1]+a[1], b[1], borrow)
	r[2], borrow = subWithBorrow(r[2]+a[2], b[2], borrow)
	r[3], borrow = subWithBorrow(r[3]+a[3], b[3], borrow)
	if borrow != 0 {
		var c uint64
		r[0], c = addWithCarry(r[0], curveN[0], 0)
		r[1], c = addWithCarry(r[1], curveN[1], c)
		r[2], c = addWithCarry(r[2], curveN[2], c)
		r[3], _ = addWithCarry(r[3], curveN[3], c)
	}
	return r
}

// scalarInv returns a^(-1) mod n via Fermat: a^(n-2) mod n.
func scalarInv(a Fe) Fe {
	nm2 := Fe{
		0xBFD25E8CD036413F,
		0xBAAEDCE6AF48A03B,
		0xFFFFFFFFFFFFFFFE,
		0xFFFFFFFFFFFFFFFF,
	}
	return scalarExp(a, nm2)
}

func scalarExp(base, exp Fe) Fe {
	result := feOne
	b := base
	for i := 0; i < 4; i++ {
		w := exp[i]
		for bit := 0; bit < 64; bit++ {
			if w&1 == 1 {
				result = scalarMul(result, b)
			}
			b = scalarMul(b, b)
			w >>= 1
		}
	}
	return result
}

// scalarReduceFull reduces a 512-bit number modulo n.
// Uses iterative folding: 2^256 ≡ cc (mod n) where cc = 2^256 - n (129 bits).
// Each fold reduces the high part's bit count by ~127 bits, so two rounds
// plus a final subtraction are always sufficient.
func scalarReduceFull(t [8]uint64) Fe {
	// cc = 2^256 - n (little-endian, 3 non-zero limbs, 129 bits)
	cc := [3]uint64{
		0x402DA1732FC9BEBF,
		0x4551231950B75FC4,
		0x0000000000000001,
	}

	// Round 1: fold t[4..7] into t[0..3] via high*cc + low.
	r := scalarFoldHigh(t, cc)

	// Round 2: after round 1, r[4..7] has at most ~130 bits.
	// One more fold drives it to zero (or a tiny residual in r[4]).
	r = scalarFoldHigh(r, cc)

	// At this point r[4..7] should be zero. If r[4] has a residual
	// from carry propagation, fold it one more time (single limb × cc).
	var res Fe
	res[0] = r[0]
	res[1] = r[1]
	res[2] = r[2]
	res[3] = r[3]

	if r[4] != 0 {
		var carry uint64
		hi, lo := mul64(r[4], cc[0])
		lo, k := addWithCarry(lo, res[0], 0)
		hi += k
		res[0] = lo
		carry = hi

		hi, lo = mul64(r[4], cc[1])
		lo, k = addWithCarry(lo, res[1], 0)
		hi += k
		lo, k = addWithCarry(lo, carry, 0)
		hi += k
		res[1] = lo
		carry = hi

		hi, lo = mul64(r[4], cc[2])
		lo, k = addWithCarry(lo, res[2], 0)
		hi += k
		lo, k = addWithCarry(lo, carry, 0)
		hi += k
		res[2] = lo
		carry = hi

		res[3], _ = addWithCarry(res[3], carry, 0)
	}

	// Final: subtract n while result >= n.
	for feCmp(res, curveN) >= 0 {
		var borrow uint64
		res[0], borrow = subWithBorrow(res[0], curveN[0], 0)
		res[1], borrow = subWithBorrow(res[1], curveN[1], borrow)
		res[2], borrow = subWithBorrow(res[2], curveN[2], borrow)
		res[3], _ = subWithBorrow(res[3], curveN[3], borrow)
	}

	return res
}

// scalarFoldHigh folds the upper 4 limbs of an 8-limb number down into
// the lower 4 limbs using the identity 2^256 ≡ cc (mod n).
func scalarFoldHigh(t [8]uint64, cc [3]uint64) [8]uint64 {
	var r [8]uint64
	r[0] = t[0]
	r[1] = t[1]
	r[2] = t[2]
	r[3] = t[3]

	// Accumulate t[i+4] * cc into r at offset i.
	for i := 0; i < 4; i++ {
		if t[i+4] == 0 {
			continue
		}
		var carry uint64
		for j := 0; j < 3; j++ {
			hi, lo := mul64(t[i+4], cc[j])
			lo, k := addWithCarry(lo, r[i+j], 0)
			hi += k
			lo, k = addWithCarry(lo, carry, 0)
			hi += k
			r[i+j] = lo
			carry = hi
		}
		// Propagate remaining carry upward.
		for k := i + 3; carry != 0 && k < 8; k++ {
			r[k], carry = addWithCarry(r[k], carry, 0)
		}
	}

	return r
}

// scalarIsZero returns true if s == 0.
func scalarIsZero(s Fe) bool {
	return s == feZero
}

// scalarFromBytes reads a 32-byte big-endian scalar.
func scalarFromBytes(b []byte) Fe {
	return feFromBytes(b) // Same format, different modulus.
}
