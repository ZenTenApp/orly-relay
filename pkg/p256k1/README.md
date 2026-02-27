# p256k1

Pure Go implementation of secp256k1 elliptic curve cryptography.

## Features

- **ECDSA signatures** - Sign, verify, and recover public keys
- **Schnorr signatures** - BIP-340 compliant with batch verification
- **ECDH key exchange** - Shared secret computation with HKDF support
- **Key management** - Generation, parsing, serialization, tweaking
- **Zero dependencies** - No cgo, pure Go
- **Platform optimized** - AMD64 assembly for critical paths

## Installation

```bash
go get p256k1.mleku.dev@latest
```

## Quick Start

### Using Domain Packages (Recommended)

```go
import (
    "p256k1.mleku.dev/keys"
    "p256k1.mleku.dev/schnorr"
    "p256k1.mleku.dev/ecdsa"
    "p256k1.mleku.dev/exchange"
)

// Generate a key pair
kp, _ := keys.Generate()

// Schnorr signing (BIP-340)
message := make([]byte, 32) // 32-byte message hash
sig, _ := schnorr.Sign(message, kp, nil)

// Verify
xonly, _, _ := keys.XOnlyFromPublic(kp.Pubkey())
valid := schnorr.Verify(sig, message, xonly)

// ECDSA signing
ecdsaSig, _ := ecdsa.Sign(message, kp.Seckey())
valid = ecdsa.Verify(ecdsaSig, message, kp.Pubkey())

// ECDH key exchange
kp2, _ := keys.Generate()
shared, _ := exchange.SharedSecret(kp2.Pubkey(), kp.Seckey())
```

### Using Core Package

```go
import "p256k1.mleku.dev"

// Generate key pair
kp, _ := p256k1.KeyPairGenerate()

// Sign with Schnorr
var sig [64]byte
p256k1.SchnorrSign(sig[:], messageHash, kp, nil)

// Verify
xonly, _, _ := p256k1.XOnlyPubkeyFromPubkey(kp.Pubkey())
valid := p256k1.SchnorrVerify(sig[:], messageHash, xonly)
```

## Packages

| Package | Description |
|---------|-------------|
| `p256k1.mleku.dev` | Core implementation with all primitives |
| `p256k1.mleku.dev/ecdsa` | ECDSA signatures (sign, verify, recover) |
| `p256k1.mleku.dev/schnorr` | BIP-340 Schnorr signatures with batch verify |
| `p256k1.mleku.dev/keys` | Key generation, parsing, serialization |
| `p256k1.mleku.dev/exchange` | ECDH key exchange and HKDF derivation |
| `p256k1.mleku.dev/signer` | High-level signer abstraction |

## Performance

Benchmarks on AMD64 (Ryzen 5):

| Operation | Time |
|-----------|------|
| Schnorr Sign | ~45 µs |
| Schnorr Verify | ~52 µs |
| ECDSA Sign | ~42 µs |
| ECDSA Verify | ~85 µs |
| ECDH | ~45 µs |

Batch Schnorr verification provides significant speedups for multiple signatures.

## Architecture

The implementation is organized into layers following DDD principles:

1. **Field Arithmetic** - Operations in F_p (5x52-bit limbs)
2. **Scalar Arithmetic** - Operations mod group order (4x64-bit limbs)
3. **Group Operations** - Point arithmetic (affine/Jacobian)
4. **Cryptographic Hashing** - SHA-256, HMAC, tagged hashes
5. **Signature Schemes** - ECDSA and Schnorr
6. **Key Management** - KeyPair aggregate root
7. **Key Exchange** - ECDH with HKDF

## Optimizations

- GLV endomorphism for faster scalar multiplication
- Strauss algorithm for combined multiplications
- wNAF representation for reduced point additions
- Precomputed generator tables
- AMD64 assembly for field/scalar operations
- Batch normalization and verification
- Zero-allocation hot paths

## Platform Support

- **AMD64** - Optimized assembly (AVX2/BMI2 when available)
- **32-bit** - Pure Go fallback
- **WASM** - WebAssembly compatible
- **Other** - Generic pure Go implementation

## Security

- Constant-time operations for secret-dependent code paths
- RFC 6979 deterministic nonce generation for ECDSA
- BIP-340 compliant Schnorr signatures
- Secure memory clearing for sensitive data

## License

MIT License - derived from libsecp256k1.
