package avx

import "math/bits"

// Field operations modulo the secp256k1 field prime p.
// p = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
//   = 2^256 - 2^32 - 977

// SetBytes sets a field element from a 32-byte big-endian slice.
// Returns true if the value was >= p and was reduced.
func (f *FieldElement) SetBytes(b []byte) bool {
	if len(b) != 32 {
		panic("field element must be 32 bytes")
	}

	// Convert big-endian bytes to little-endian limbs
	f.N[0].Lo = uint64(b[31]) | uint64(b[30])<<8 | uint64(b[29])<<16 | uint64(b[28])<<24 |
		uint64(b[27])<<32 | uint64(b[26])<<40 | uint64(b[25])<<48 | uint64(b[24])<<56
	f.N[0].Hi = uint64(b[23]) | uint64(b[22])<<8 | uint64(b[21])<<16 | uint64(b[20])<<24 |
		uint64(b[19])<<32 | uint64(b[18])<<40 | uint64(b[17])<<48 | uint64(b[16])<<56
	f.N[1].Lo = uint64(b[15]) | uint64(b[14])<<8 | uint64(b[13])<<16 | uint64(b[12])<<24 |
		uint64(b[11])<<32 | uint64(b[10])<<40 | uint64(b[9])<<48 | uint64(b[8])<<56
	f.N[1].Hi = uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
		uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56

	// Check overflow and reduce if necessary
	overflow := f.checkOverflow()
	if overflow {
		f.reduce()
	}
	return overflow
}

// Bytes returns the field element as a 32-byte big-endian slice.
func (f *FieldElement) Bytes() [32]byte {
	var b [32]byte
	b[31] = byte(f.N[0].Lo)
	b[30] = byte(f.N[0].Lo >> 8)
	b[29] = byte(f.N[0].Lo >> 16)
	b[28] = byte(f.N[0].Lo >> 24)
	b[27] = byte(f.N[0].Lo >> 32)
	b[26] = byte(f.N[0].Lo >> 40)
	b[25] = byte(f.N[0].Lo >> 48)
	b[24] = byte(f.N[0].Lo >> 56)

	b[23] = byte(f.N[0].Hi)
	b[22] = byte(f.N[0].Hi >> 8)
	b[21] = byte(f.N[0].Hi >> 16)
	b[20] = byte(f.N[0].Hi >> 24)
	b[19] = byte(f.N[0].Hi >> 32)
	b[18] = byte(f.N[0].Hi >> 40)
	b[17] = byte(f.N[0].Hi >> 48)
	b[16] = byte(f.N[0].Hi >> 56)

	b[15] = byte(f.N[1].Lo)
	b[14] = byte(f.N[1].Lo >> 8)
	b[13] = byte(f.N[1].Lo >> 16)
	b[12] = byte(f.N[1].Lo >> 24)
	b[11] = byte(f.N[1].Lo >> 32)
	b[10] = byte(f.N[1].Lo >> 40)
	b[9] = byte(f.N[1].Lo >> 48)
	b[8] = byte(f.N[1].Lo >> 56)

	b[7] = byte(f.N[1].Hi)
	b[6] = byte(f.N[1].Hi >> 8)
	b[5] = byte(f.N[1].Hi >> 16)
	b[4] = byte(f.N[1].Hi >> 24)
	b[3] = byte(f.N[1].Hi >> 32)
	b[2] = byte(f.N[1].Hi >> 40)
	b[1] = byte(f.N[1].Hi >> 48)
	b[0] = byte(f.N[1].Hi >> 56)

	return b
}

// IsZero returns true if the field element is zero.
func (f *FieldElement) IsZero() bool {
	return f.N[0].IsZero() && f.N[1].IsZero()
}

// IsOne returns true if the field element is one.
func (f *FieldElement) IsOne() bool {
	return f.N[0].Lo == 1 && f.N[0].Hi == 0 && f.N[1].IsZero()
}

// Equal returns true if two field elements are equal.
func (f *FieldElement) Equal(other *FieldElement) bool {
	return f.N[0].Lo == other.N[0].Lo && f.N[0].Hi == other.N[0].Hi &&
		f.N[1].Lo == other.N[1].Lo && f.N[1].Hi == other.N[1].Hi
}

// IsOdd returns true if the field element is odd.
func (f *FieldElement) IsOdd() bool {
	return f.N[0].Lo&1 == 1
}

// checkOverflow returns true if f >= p.
func (f *FieldElement) checkOverflow() bool {
	// p = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
	// Compare high to low
	if f.N[1].Hi > FieldP.N[1].Hi {
		return true
	}
	if f.N[1].Hi < FieldP.N[1].Hi {
		return false
	}
	if f.N[1].Lo > FieldP.N[1].Lo {
		return true
	}
	if f.N[1].Lo < FieldP.N[1].Lo {
		return false
	}
	if f.N[0].Hi > FieldP.N[0].Hi {
		return true
	}
	if f.N[0].Hi < FieldP.N[0].Hi {
		return false
	}
	return f.N[0].Lo >= FieldP.N[0].Lo
}

// reduce reduces f modulo p by adding the complement (2^256 - p = 2^32 + 977).
func (f *FieldElement) reduce() {
	// f = f - p = f + (2^256 - p) mod 2^256
	// 2^256 - p = 0x1000003D1
	var carry uint64
	f.N[0].Lo, carry = bits.Add64(f.N[0].Lo, 0x1000003D1, 0)
	f.N[0].Hi, carry = bits.Add64(f.N[0].Hi, 0, carry)
	f.N[1].Lo, carry = bits.Add64(f.N[1].Lo, 0, carry)
	f.N[1].Hi, _ = bits.Add64(f.N[1].Hi, 0, carry)
}

// Add sets f = a + b mod p.
func (f *FieldElement) Add(a, b *FieldElement) *FieldElement {
	var carry uint64
	f.N[0].Lo, carry = bits.Add64(a.N[0].Lo, b.N[0].Lo, 0)
	f.N[0].Hi, carry = bits.Add64(a.N[0].Hi, b.N[0].Hi, carry)
	f.N[1].Lo, carry = bits.Add64(a.N[1].Lo, b.N[1].Lo, carry)
	f.N[1].Hi, carry = bits.Add64(a.N[1].Hi, b.N[1].Hi, carry)

	// If there was a carry or if result >= p, reduce
	if carry != 0 || f.checkOverflow() {
		f.reduce()
	}
	return f
}

// Sub sets f = a - b mod p.
func (f *FieldElement) Sub(a, b *FieldElement) *FieldElement {
	var borrow uint64
	f.N[0].Lo, borrow = bits.Sub64(a.N[0].Lo, b.N[0].Lo, 0)
	f.N[0].Hi, borrow = bits.Sub64(a.N[0].Hi, b.N[0].Hi, borrow)
	f.N[1].Lo, borrow = bits.Sub64(a.N[1].Lo, b.N[1].Lo, borrow)
	f.N[1].Hi, borrow = bits.Sub64(a.N[1].Hi, b.N[1].Hi, borrow)

	// If there was a borrow, add p back
	if borrow != 0 {
		var carry uint64
		f.N[0].Lo, carry = bits.Add64(f.N[0].Lo, FieldP.N[0].Lo, 0)
		f.N[0].Hi, carry = bits.Add64(f.N[0].Hi, FieldP.N[0].Hi, carry)
		f.N[1].Lo, carry = bits.Add64(f.N[1].Lo, FieldP.N[1].Lo, carry)
		f.N[1].Hi, _ = bits.Add64(f.N[1].Hi, FieldP.N[1].Hi, carry)
	}
	return f
}

// Negate sets f = -a mod p.
func (f *FieldElement) Negate(a *FieldElement) *FieldElement {
	if a.IsZero() {
		*f = FieldZero
		return f
	}
	// f = p - a
	var borrow uint64
	f.N[0].Lo, borrow = bits.Sub64(FieldP.N[0].Lo, a.N[0].Lo, 0)
	f.N[0].Hi, borrow = bits.Sub64(FieldP.N[0].Hi, a.N[0].Hi, borrow)
	f.N[1].Lo, borrow = bits.Sub64(FieldP.N[1].Lo, a.N[1].Lo, borrow)
	f.N[1].Hi, _ = bits.Sub64(FieldP.N[1].Hi, a.N[1].Hi, borrow)
	return f
}

// Mul sets f = a * b mod p.
func (f *FieldElement) Mul(a, b *FieldElement) *FieldElement {
	// Compute 512-bit product
	var prod [8]uint64
	fieldMul512(&prod, a, b)

	// Reduce mod p using secp256k1's special structure
	fieldReduce512(f, &prod)
	return f
}

// fieldMul512 computes the 512-bit product of two 256-bit field elements.
func fieldMul512(prod *[8]uint64, a, b *FieldElement) {
	aLimbs := [4]uint64{a.N[0].Lo, a.N[0].Hi, a.N[1].Lo, a.N[1].Hi}
	bLimbs := [4]uint64{b.N[0].Lo, b.N[0].Hi, b.N[1].Lo, b.N[1].Hi}

	// Clear product
	for i := range prod {
		prod[i] = 0
	}

	// Schoolbook multiplication
	for i := 0; i < 4; i++ {
		var carry uint64
		for j := 0; j < 4; j++ {
			hi, lo := bits.Mul64(aLimbs[i], bLimbs[j])
			lo, c := bits.Add64(lo, prod[i+j], 0)
			hi, _ = bits.Add64(hi, 0, c)
			lo, c = bits.Add64(lo, carry, 0)
			hi, _ = bits.Add64(hi, 0, c)
			prod[i+j] = lo
			carry = hi
		}
		prod[i+4] = carry
	}
}

// fieldReduce512 reduces a 512-bit value mod p using secp256k1's special structure.
// p = 2^256 - 2^32 - 977, so 2^256 ≡ 2^32 + 977 (mod p)
func fieldReduce512(f *FieldElement, prod *[8]uint64) {
	// The key insight: if we have a 512-bit number split as H*2^256 + L
	// then H*2^256 + L ≡ H*(2^32 + 977) + L (mod p)

	// Extract low and high 256-bit parts
	low := [4]uint64{prod[0], prod[1], prod[2], prod[3]}
	high := [4]uint64{prod[4], prod[5], prod[6], prod[7]}

	// Compute high * (2^32 + 977) = high * 0x1000003D1
	// This gives us at most a 289-bit result (256 + 33 bits)
	const c = uint64(0x1000003D1)

	var reduction [5]uint64
	var carry uint64

	for i := 0; i < 4; i++ {
		hi, lo := bits.Mul64(high[i], c)
		lo, cc := bits.Add64(lo, carry, 0)
		hi, _ = bits.Add64(hi, 0, cc)
		reduction[i] = lo
		carry = hi
	}
	reduction[4] = carry

	// Add low + reduction
	var result [5]uint64
	carry = 0
	for i := 0; i < 4; i++ {
		result[i], carry = bits.Add64(low[i], reduction[i], carry)
	}
	result[4] = carry + reduction[4]

	// If result[4] is non-zero, we need to reduce again
	// result[4] * 2^256 ≡ result[4] * (2^32 + 977) (mod p)
	if result[4] != 0 {
		hi, lo := bits.Mul64(result[4], c)
		result[0], carry = bits.Add64(result[0], lo, 0)
		result[1], carry = bits.Add64(result[1], hi, carry)
		result[2], carry = bits.Add64(result[2], 0, carry)
		result[3], _ = bits.Add64(result[3], 0, carry)
		result[4] = 0
	}

	// Store result
	f.N[0].Lo = result[0]
	f.N[0].Hi = result[1]
	f.N[1].Lo = result[2]
	f.N[1].Hi = result[3]

	// Final reduction if >= p
	if f.checkOverflow() {
		f.reduce()
	}
}

// Sqr sets f = a^2 mod p.
func (f *FieldElement) Sqr(a *FieldElement) *FieldElement {
	// Optimized squaring could save some multiplications, but for now use Mul
	return f.Mul(a, a)
}

// Inverse sets f = a^(-1) mod p using Fermat's little theorem.
// a^(-1) = a^(p-2) mod p
func (f *FieldElement) Inverse(a *FieldElement) *FieldElement {
	// p-2 in bytes (big-endian)
	// p = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
	// p-2 = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2D
	pMinus2 := [32]byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFE, 0xFF, 0xFF, 0xFC, 0x2D,
	}

	var result, base FieldElement
	result = FieldOne
	base = *a

	for i := 0; i < 32; i++ {
		b := pMinus2[31-i]
		for j := 0; j < 8; j++ {
			if (b>>j)&1 == 1 {
				result.Mul(&result, &base)
			}
			base.Sqr(&base)
		}
	}

	*f = result
	return f
}

// Sqrt sets f = sqrt(a) mod p if it exists, returns true if successful.
// For secp256k1, p ≡ 3 (mod 4), so sqrt(a) = a^((p+1)/4) mod p
func (f *FieldElement) Sqrt(a *FieldElement) bool {
	// (p+1)/4 in bytes
	// p+1 = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC30
	// (p+1)/4 = 3FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFBFFFFF0C
	pPlus1Div4 := [32]byte{
		0x3F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xBF, 0xFF, 0xFF, 0x0C,
	}

	var result, base FieldElement
	result = FieldOne
	base = *a

	for i := 0; i < 32; i++ {
		b := pPlus1Div4[31-i]
		for j := 0; j < 8; j++ {
			if (b>>j)&1 == 1 {
				result.Mul(&result, &base)
			}
			base.Sqr(&base)
		}
	}

	// Verify: result^2 should equal a
	var check FieldElement
	check.Sqr(&result)

	if check.Equal(a) {
		*f = result
		return true
	}
	return false
}

// MulInt sets f = a * n mod p where n is a small integer.
func (f *FieldElement) MulInt(a *FieldElement, n uint64) *FieldElement {
	if n == 0 {
		*f = FieldZero
		return f
	}
	if n == 1 {
		*f = *a
		return f
	}

	// Multiply by small integer using proper carry chain
	// We need to compute a 320-bit result (256 + 64 bits max)
	var result [5]uint64
	var carry uint64

	// Multiply each 64-bit limb by n
	var hi uint64
	hi, result[0] = bits.Mul64(a.N[0].Lo, n)
	carry = hi

	hi, result[1] = bits.Mul64(a.N[0].Hi, n)
	result[1], carry = bits.Add64(result[1], carry, 0)
	carry = hi + carry // carry can be at most 1 here, so no overflow

	hi, result[2] = bits.Mul64(a.N[1].Lo, n)
	result[2], carry = bits.Add64(result[2], carry, 0)
	carry = hi + carry

	hi, result[3] = bits.Mul64(a.N[1].Hi, n)
	result[3], carry = bits.Add64(result[3], carry, 0)
	result[4] = hi + carry

	// Store preliminary result
	f.N[0].Lo = result[0]
	f.N[0].Hi = result[1]
	f.N[1].Lo = result[2]
	f.N[1].Hi = result[3]

	// Reduce overflow
	if result[4] != 0 {
		// overflow * 2^256 ≡ overflow * (2^32 + 977) (mod p)
		hi, lo := bits.Mul64(result[4], 0x1000003D1)
		f.N[0].Lo, carry = bits.Add64(f.N[0].Lo, lo, 0)
		f.N[0].Hi, carry = bits.Add64(f.N[0].Hi, hi, carry)
		f.N[1].Lo, carry = bits.Add64(f.N[1].Lo, 0, carry)
		f.N[1].Hi, _ = bits.Add64(f.N[1].Hi, 0, carry)
	}

	if f.checkOverflow() {
		f.reduce()
	}
	return f
}

// Double sets f = 2*a mod p (optimized addition).
func (f *FieldElement) Double(a *FieldElement) *FieldElement {
	return f.Add(a, a)
}

// Half sets f = a/2 mod p.
func (f *FieldElement) Half(a *FieldElement) *FieldElement {
	// If a is even, just shift right
	// If a is odd, add p first (which makes it even), then shift right
	var result FieldElement = *a

	if result.N[0].Lo&1 == 1 {
		// Add p
		var carry uint64
		result.N[0].Lo, carry = bits.Add64(result.N[0].Lo, FieldP.N[0].Lo, 0)
		result.N[0].Hi, carry = bits.Add64(result.N[0].Hi, FieldP.N[0].Hi, carry)
		result.N[1].Lo, carry = bits.Add64(result.N[1].Lo, FieldP.N[1].Lo, carry)
		result.N[1].Hi, _ = bits.Add64(result.N[1].Hi, FieldP.N[1].Hi, carry)
	}

	// Shift right by 1
	f.N[0].Lo = (result.N[0].Lo >> 1) | (result.N[0].Hi << 63)
	f.N[0].Hi = (result.N[0].Hi >> 1) | (result.N[1].Lo << 63)
	f.N[1].Lo = (result.N[1].Lo >> 1) | (result.N[1].Hi << 63)
	f.N[1].Hi = result.N[1].Hi >> 1

	return f
}

// CMov conditionally moves b into f if cond is true (constant-time).
func (f *FieldElement) CMov(b *FieldElement, cond bool) *FieldElement {
	mask := uint64(0)
	if cond {
		mask = ^uint64(0)
	}
	f.N[0].Lo = (f.N[0].Lo &^ mask) | (b.N[0].Lo & mask)
	f.N[0].Hi = (f.N[0].Hi &^ mask) | (b.N[0].Hi & mask)
	f.N[1].Lo = (f.N[1].Lo &^ mask) | (b.N[1].Lo & mask)
	f.N[1].Hi = (f.N[1].Hi &^ mask) | (b.N[1].Hi & mask)
	return f
}
