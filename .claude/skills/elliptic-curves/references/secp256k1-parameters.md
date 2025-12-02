# secp256k1 Complete Parameters

## Curve Definition

**Name**: secp256k1 (Standards for Efficient Cryptography, prime field, 256-bit, Koblitz curve #1)

**Equation**: y² = x³ + 7 (mod p)

This is the short Weierstrass form with coefficients a = 0, b = 7.

## Field Parameters

### Prime Modulus p

```
Decimal:
115792089237316195423570985008687907853269984665640564039457584007908834671663

Hexadecimal:
0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F

Binary representation:
2²⁵⁶ - 2³² - 2⁹ - 2⁸ - 2⁷ - 2⁶ - 2⁴ - 1
= 2²⁵⁶ - 2³² - 977
```

**Special form benefits**:
- Efficient modular reduction using: c mod p = c_low + c_high × (2³² + 977)
- Near-Mersenne prime enables fast arithmetic

### Group Order n

```
Decimal:
115792089237316195423570985008687907852837564279074904382605163141518161494337

Hexadecimal:
0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
```

The number of points on the curve, including the point at infinity.

### Cofactor h

```
h = 1
```

Cofactor 1 means the group order n equals the curve order, simplifying security analysis and eliminating small subgroup attacks.

## Generator Point G

### Compressed Form

```
02 79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
```

The 02 prefix indicates the y-coordinate is even.

### Uncompressed Form

```
04 79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
   483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8
```

### Individual Coordinates

**Gx**:
```
Decimal:
55066263022277343669578718895168534326250603453777594175500187360389116729240

Hexadecimal:
0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
```

**Gy**:
```
Decimal:
32670510020758816978083085130507043184471273380659243275938904335757337482424

Hexadecimal:
0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8
```

## Endomorphism Parameters

secp256k1 has an efficiently computable endomorphism φ: (x, y) → (βx, y).

### β (Beta)

```
Hexadecimal:
0x7AE96A2B657C07106E64479EAC3434E99CF0497512F58995C1396C28719501EE

Property: β³ ≡ 1 (mod p)
```

### λ (Lambda)

```
Hexadecimal:
0x5363AD4CC05C30E0A5261C028812645A122E22EA20816678DF02967C1B23BD72

Property: λ³ ≡ 1 (mod n)
Relationship: φ(P) = λP for all points P
```

### GLV Decomposition Constants

For splitting scalar k into k₁ + k₂λ:

```
a₁ = 0x3086D221A7D46BCDE86C90E49284EB15
b₁ = -0xE4437ED6010E88286F547FA90ABFE4C3
a₂ = 0x114CA50F7A8E2F3F657C1108D9D44CFD8
b₂ = a₁
```

## Derived Constants

### Field Characteristics

```
(p + 1) / 4 = 0x3FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFBFFFFF0C
Used for computing modular square roots via Tonelli-Shanks shortcut
```

### Order Characteristics

```
(n - 1) / 2 = 0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0
Used in low-S normalization for ECDSA signatures
```

## Validation Formulas

### Point on Curve Check

For point (x, y), verify:
```
y² ≡ x³ + 7 (mod p)
```

### Generator Verification

Verify G is on curve:
```
Gy² mod p = 0x9C47D08FFB10D4B8 ... (truncated for display)
Gx³ + 7 mod p = same value
```

### Order Verification

Verify nG = O (point at infinity):
```
Computing n × G should yield the identity element
```

## Bit Lengths

| Parameter | Bits | Bytes |
|-----------|------|-------|
| p (prime) | 256  | 32    |
| n (order) | 256  | 32    |
| Private key | 256 | 32   |
| Public key (compressed) | 257 | 33 |
| Public key (uncompressed) | 513 | 65 |
| ECDSA signature | 512 | 64 |
| Schnorr signature | 512 | 64 |

## Security Level

- **Equivalent symmetric key strength**: 128 bits
- **Best known attack complexity**: ~2¹²⁸ operations (Pollard's rho)
- **Safe until**: Quantum computers with ~1500+ logical qubits

## ASN.1 OID

```
1.3.132.0.10
iso(1) identified-organization(3) certicom(132) curve(0) secp256k1(10)
```

## Comparison with Other Curves

| Curve | Field Size | Security | Speed | Use Case |
|-------|------------|----------|-------|----------|
| secp256k1 | 256-bit | 128-bit | Fast (Koblitz) | Bitcoin, Nostr |
| secp256r1 (P-256) | 256-bit | 128-bit | Moderate | TLS, general |
| Curve25519 | 255-bit | ~128-bit | Very fast | Modern crypto |
| secp384r1 (P-384) | 384-bit | 192-bit | Slower | High security |
