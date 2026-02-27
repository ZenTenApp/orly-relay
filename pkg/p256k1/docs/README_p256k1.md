# p256k1 - Minimal secp256k1 Library for BIP-340

This is a minimal extraction of the secp256k1 library containing only the code required for:

- **BIP-340 Schnorr Signatures**: X-only public keys and signatures
- **X-only Public Keys**: 32-byte compressed public keys (extrakeys module)
- **ECDH**: Elliptic Curve Diffie-Hellman key exchange (XDH)

## Features

- BIP-340 compliant Schnorr signatures
- X-only public key operations (parse, serialize, tweak)
- Keypair operations for X-only keys
- ECDH operations with standard and X-only keys
- Minimal dependencies and footprint

## Building

### Using Make
```bash
make
```

### Using CMake
```bash
mkdir build && cd build
cmake ..
make
```

## Usage

### BIP-340 Schnorr Signatures

```c
#include "secp256k1.h"
#include "secp256k1_extrakeys.h"
#include "secp256k1_schnorrsig.h"

secp256k1_context *ctx = secp256k1_context_create(SECP256K1_CONTEXT_NONE);
secp256k1_keypair keypair;
secp256k1_xonly_pubkey pubkey;
unsigned char seckey[32] = { /* your secret key */ };
unsigned char msg[32] = { /* your message hash */ };
unsigned char sig[64];

// Create keypair
secp256k1_keypair_create(ctx, &keypair, seckey);

// Get X-only public key
secp256k1_keypair_xonly_pub(ctx, &pubkey, NULL, &keypair);

// Sign message
secp256k1_schnorrsig_sign32(ctx, sig, msg, &keypair, NULL);

// Verify signature
int valid = secp256k1_schnorrsig_verify(ctx, sig, msg, 32, &pubkey);
```

### ECDH

```c
#include "secp256k1.h"
#include "secp256k1_ecdh.h"

secp256k1_context *ctx = secp256k1_context_create(SECP256K1_CONTEXT_NONE);
secp256k1_pubkey pubkey;
unsigned char seckey[32] = { /* your secret key */ };
unsigned char shared_secret[32];

// Parse public key
secp256k1_ec_pubkey_parse(ctx, &pubkey, pubkey_bytes, pubkey_len);

// Compute ECDH
secp256k1_ecdh(ctx, shared_secret, &pubkey, seckey, NULL, NULL);
```

## Examples

See the `examples/` directory for complete working examples:
- `examples/schnorr.c` - BIP-340 Schnorr signature example
- `examples/ecdh.c` - ECDH key exchange example

## License

This code is derived from the bitcoin-core/secp256k1 library and maintains the same MIT license.
See `COPYING` for details.

## Source

This library is extracted from: https://github.com/bitcoin-core/secp256k1

Only the minimal code required for BIP-340, X-only keys, and ECDH has been included.
