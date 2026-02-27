# Claude Code Instructions for p256k1

## Project Overview

Pure Go implementation of secp256k1 elliptic curve cryptography. No cgo, no external dependencies.

## Project Structure

```
p256k1/
├── *.go              # Core implementation (field, scalar, group, signatures)
├── *_amd64.go        # AMD64-specific optimizations
├── *_amd64.s         # AMD64 assembly
├── *_32bit.go        # 32-bit fallbacks
├── *_wasm.go         # WebAssembly support
├── *_generic.go      # Generic fallbacks
│
├── ecdsa/            # ECDSA domain package
├── schnorr/          # Schnorr/BIP-340 domain package
├── keys/             # Key management domain package
├── exchange/         # ECDH domain package
├── signer/           # High-level signer abstraction
│
├── avx/              # Experimental AVX2 optimizations
├── wnaf/             # wNAF encoding utilities
├── bench/            # Benchmarking tools
├── testdata/         # Test vectors
└── docs/             # Technical documentation
```

## Key Files

- `field.go`, `field_mul.go`, `field_4x64.go` - Field arithmetic (F_p)
- `scalar.go` - Scalar arithmetic (mod group order)
- `group.go`, `group_4x64.go` - Point operations
- `ecdsa.go` - ECDSA implementation
- `schnorr.go`, `schnorr_batch.go` - Schnorr signatures
- `ecdh.go` - ECDH key exchange
- `hash.go` - SHA-256, HMAC, RFC6979
- `verify.go` - Combined verification with Strauss algorithm
- `ecmult_gen.go` - Generator multiplication with precomputed tables

## Build Tags

- Default: Uses optimized AMD64 code when available
- `//go:build !amd64` - 32-bit/generic fallback
- `//go:build wasm` - WebAssembly build

## Testing

```bash
go test ./...                    # Run all tests
go test -bench=. .               # Run benchmarks
go test -run TestSchnorr .       # Run specific tests
```

## Performance Considerations

- Hot paths avoid heap allocations
- Field elements use 5x52-bit limbs for efficient multiplication
- Scalars use 4x64-bit limbs
- GLV endomorphism splits 256-bit multiplications into 2x128-bit
- Strauss algorithm shares doublings across multiple multiplications
- Precomputed tables for generator point

## Domain Packages

The domain packages (`ecdsa/`, `schnorr/`, `keys/`, `exchange/`) are thin wrappers around the core `p256k1` package. They:

- Re-export core types via type aliases
- Provide cleaner, more focused APIs
- Add domain-specific documentation
- Don't contain implementation logic (just delegation)

## Conventions

- Use existing patterns when adding new functionality
- Keep hot paths allocation-free
- Add `*Raw` variants for functions that can write to caller-provided buffers
- Assembly files use Go's plan9 assembly syntax
- Test files follow `*_test.go` naming
