# Elliptic Curve Security Analysis

Security properties, attack vectors, and mitigations for elliptic curve cryptography.

## The Discrete Logarithm Problem (ECDLP)

### Definition

Given points P and Q = kP on an elliptic curve, find the scalar k.

**Security assumption**: For properly chosen curves, this problem is computationally infeasible.

### Best Known Attacks

#### Generic Attacks (Work on Any Group)

| Attack | Complexity | Notes |
|--------|------------|-------|
| Baby-step Giant-step | O(√n) space and time | Requires √n storage |
| Pollard's rho | O(√n) time, O(1) space | Practical for large groups |
| Pollard's lambda | O(√n) | When k is in known range |
| Pohlig-Hellman | O(√p) where p is largest prime factor | Exploits factorization of n |

For secp256k1 (n ≈ 2²⁵⁶):
- Generic attack complexity: ~2¹²⁸ operations
- Equivalent to 128-bit symmetric security

#### Curve-Specific Attacks

| Attack | Applicable When | Mitigation |
|--------|-----------------|------------|
| MOV/FR reduction | Low embedding degree | Use curves with high embedding degree |
| Anomalous curve attack | n = p | Ensure n ≠ p |
| GHS attack | Extension field curves | Use prime field curves |

**secp256k1 is immune to all known curve-specific attacks**.

## Side-Channel Attacks

### Timing Attacks

**Vulnerability**: Execution time varies based on secret data.

**Examples**:
- Conditional branches on secret bits
- Early exit conditions
- Variable-time modular operations

**Mitigations**:
- Constant-time algorithms (Montgomery ladder)
- Fixed execution paths
- Dummy operations to equalize timing

### Power Analysis

**Simple Power Analysis (SPA)**: Single trace reveals operations.
- Double-and-add visible as different power signatures
- Mitigation: Montgomery ladder (uniform operations)

**Differential Power Analysis (DPA)**: Statistical analysis of many traces.
- Mitigation: Point blinding, scalar blinding

### Cache Attacks

**FLUSH+RELOAD Attack**:
```
1. Attacker flushes cache line containing lookup table
2. Victim performs table lookup based on secret
3. Attacker measures reload time to determine which entry was accessed
```

**Mitigations**:
- Avoid secret-dependent table lookups
- Use constant-time table access patterns
- Scatter tables to prevent cache line sharing

### Electromagnetic (EM) Attacks

Similar to power analysis but captures electromagnetic emissions.

**Mitigations**:
- Shielding
- Same algorithmic protections as power analysis

## Implementation Vulnerabilities

### k-Reuse in ECDSA

**The Sony PS3 Hack (2010)**:

If the same k is used for two signatures (r₁, s₁) and (r₂, s₂) on messages m₁ and m₂:

```
s₁ = k⁻¹(e₁ + rd) mod n
s₂ = k⁻¹(e₂ + rd) mod n

Since k is the same:
s₁ - s₂ = k⁻¹(e₁ - e₂) mod n
k = (e₁ - e₂)(s₁ - s₂)⁻¹ mod n

Once k is known:
d = (s₁k - e₁)r⁻¹ mod n
```

**Mitigation**: Use deterministic k (RFC 6979).

### Weak Random k

Even with unique k values, if the RNG is biased:
- Lattice-based attacks can recover private key
- Only ~1% bias in k can be exploitable with enough signatures

**Mitigations**:
- Use cryptographically secure RNG
- Use deterministic k (RFC 6979)
- Verify k is in valid range [1, n-1]

### Invalid Curve Attacks

**Attack**: Attacker provides point not on the curve.
- Point may be on a weaker curve
- Operations may leak information

**Mitigation**: Always validate points are on curve:
```
Verify: y² ≡ x³ + ax + b (mod p)
```

### Small Subgroup Attacks

**Attack**: If cofactor h > 1, points of small order exist.
- Attacker sends point of small order
- Response reveals private key mod (small order)

**Mitigation**:
- Use curves with cofactor 1 (secp256k1 has h = 1)
- Multiply received points by cofactor
- Validate point order

### Fault Attacks

**Attack**: Induce computational errors (voltage glitches, radiation).
- Corrupted intermediate values may leak information
- Differential fault analysis can recover keys

**Mitigations**:
- Redundant computations with comparison
- Verify final results
- Hardware protections

## Signature Malleability

### ECDSA Malleability

Given valid signature (r, s), signature (r, n - s) is also valid for the same message.

**Impact**: Transaction ID malleability (historical Bitcoin issue)

**Mitigation**: Enforce low-S normalization:
```
if s > n/2:
    s = n - s
```

### Schnorr Non-Malleability

BIP-340 Schnorr signatures are non-malleable by design:
- Use x-only public keys
- Deterministic nonce derivation

## Quantum Threats

### Shor's Algorithm

**Threat**: Polynomial-time discrete log on quantum computers.
- Requires ~1500-2000 logical qubits for secp256k1
- Current quantum computers: <100 noisy qubits

**Timeline**: Estimated 10-20+ years for cryptographically relevant quantum computers.

### Migration Strategy

1. **Monitor**: Track quantum computing progress
2. **Prepare**: Develop post-quantum alternatives
3. **Hybrid**: Use classical + post-quantum in transition
4. **Migrate**: Full transition when necessary

### Post-Quantum Alternatives

- Lattice-based signatures (CRYSTALS-Dilithium)
- Hash-based signatures (SPHINCS+)
- Code-based cryptography

## Best Practices

### Key Generation

```
DO:
- Use cryptographically secure RNG
- Validate private key is in [1, n-1]
- Verify public key is on curve
- Verify public key is not point at infinity

DON'T:
- Use predictable seeds
- Use truncated random values
- Skip validation
```

### Signature Generation

```
DO:
- Use RFC 6979 for deterministic k
- Validate all inputs
- Use constant-time operations
- Clear sensitive memory after use

DON'T:
- Reuse k values
- Use weak/biased RNG
- Skip low-S normalization (ECDSA)
```

### Signature Verification

```
DO:
- Validate r, s are in [1, n-1]
- Validate public key is on curve
- Validate public key is not infinity
- Use batch verification when possible

DON'T:
- Skip any validation steps
- Accept malformed signatures
```

### Public Key Handling

```
DO:
- Validate received points are on curve
- Check point is not infinity
- Prefer compressed format for storage

DON'T:
- Accept unvalidated points
- Skip curve membership check
```

## Security Checklist

### Implementation Review

- [ ] All scalar multiplications are constant-time
- [ ] No secret-dependent branches
- [ ] No secret-indexed table lookups
- [ ] Memory is zeroized after use
- [ ] Random k uses CSPRNG or RFC 6979
- [ ] All received points are validated
- [ ] Private keys are in valid range
- [ ] Signatures use low-S normalization

### Operational Security

- [ ] Private keys stored securely (HSM, secure enclave)
- [ ] Key derivation uses proper KDF
- [ ] Backups are encrypted
- [ ] Key rotation policy exists
- [ ] Audit logging enabled
- [ ] Incident response plan exists

## Security Levels Comparison

| Curve | Bits | Symmetric Equivalent | RSA Equivalent |
|-------|------|---------------------|----------------|
| secp192r1 | 192 | 96 | 1536 |
| secp224r1 | 224 | 112 | 2048 |
| secp256k1 | 256 | 128 | 3072 |
| secp384r1 | 384 | 192 | 7680 |
| secp521r1 | 521 | 256 | 15360 |

## References

- NIST SP 800-57: Recommendation for Key Management
- SEC 1: Elliptic Curve Cryptography
- RFC 6979: Deterministic Usage of DSA and ECDSA
- BIP-340: Schnorr Signatures for secp256k1
- SafeCurves: Choosing Safe Curves for Elliptic-Curve Cryptography
