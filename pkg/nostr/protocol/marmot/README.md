# marmot

MLS-based E2E encrypted messaging for Nostr (NIP-EE / Marmot protocol).

Wraps a vendored fork of `github.com/emersion/go-mls` (at `pkg/nostr/crypto/mls/`)
to provide 1:1 DM groups using MLS key ratcheting.

## Event kinds

| Kind  | Name            | Spec   | Description                              |
|-------|-----------------|--------|------------------------------------------|
| 443   | KeyPackage      | MIP-00 | MLS key material, base64 TLS-serialized  |
| 444   | Welcome         | MIP-02 | MLS Welcome inside kind 1059 gift-wrap   |
| 445   | GroupMessage    | MIP-03 | ChaCha20-Poly1305 encrypted MLS payload  |
| 1059  | GiftWrap        | NIP-59 | Outer wrapper for Welcome delivery       |
| 10051 | KPRelays        | NIP-EE | Relay list for key package discovery      |

## Files

| File              | Purpose                                          |
|-------------------|--------------------------------------------------|
| `kinds.go`        | Event kind constants                             |
| `keypackage.go`   | Generate/serialize/parse kind 443 key packages   |
| `welcome.go`      | Wrap/unwrap kind 444 welcome in kind 1059        |
| `message.go`      | Encrypt/decrypt kind 445 group messages          |
| `group.go`        | Create/join MLS groups, derive exporter secret   |
| `extension.go`    | NostrGroupData MLS extension codec               |
| `store.go`        | Group state persistence (memory + file backends) |
| `marmot.go`       | High-level Client for DM group lifecycle         |

## Wire format

Kind 443 content is `base64(raw_tls_bytes)` -- bare TLS serialization of the
MLS KeyPackage struct, no MLSMessage envelope. This matches MDK/OpenMLS.

Kind 445 content is `base64(nonce[12] || ciphertext)` where the ciphertext is
ChaCha20-Poly1305 encrypted using a key derived from the MLS exporter secret.
The plaintext inside is an MLSMessage-wrapped MLS ciphertext.

## Testing

### Unit tests (no dependencies)

```
go test ./pkg/nostr/protocol/marmot/
```

Runs 11 pure-Go tests covering key package round-trips, group create/join,
encrypt/decrypt, gift-wrap, ephemeral signing, exporter secret derivation,
and group store persistence.

### Interop tests (requires Rust toolchain)

```
go test -tags interop ./pkg/nostr/protocol/marmot/
```

Adds 3 cross-implementation tests that validate Go against the Rust MDK
(Marmot Development Kit) reference implementation. On first run, `TestMain`
builds the interop binary from `testdata/interop/` via `cargo build --release`.
Cargo handles staleness on subsequent runs.

Requires: `cargo` in PATH, internet access for initial crate download.

Dependencies are pinned to crates.io releases (`mdk-core 0.7`, `openmls 0.8`).

#### Interop test coverage

**TestInterop_KeyPackageFormat** -- Bidirectional key package validation.
Go generates a kind 443 event, Rust parses the raw TLS bytes through OpenMLS
`KeyPackageIn::tls_deserialize` and validates the full event. Then Rust
generates a key package and Go parses it.

**TestInterop_WelcomeAndMessage** -- Full group lifecycle. Rust creates a
group using Go's key package, sends a Welcome. Go joins via the Welcome,
verifies the NostrGroupID matches. Rust encrypts a message, Go decrypts
through both the ChaCha20-Poly1305 outer layer and the MLS inner layer,
verifies the plaintext.

**TestDebug_CompareWireFormat** -- Byte-level comparison of Go and Rust
raw key package serialization. Dumps hex, finds first divergence byte,
and cross-parses in both directions.

### Interop binary protocol

The binary at `testdata/interop/` is a JSON-line stdin/stdout process.
It prints `{"ready":true,"pubkey":"..."}` on startup, then accepts one
JSON command per line and returns one JSON response per line.

Commands: `generate_key_package`, `process_key_package`, `create_group`,
`process_welcome`, `create_message`, `process_message`, `get_group_info`,
`parse_kp_raw`, `dump_kp_bytes`.

## Key compatibility notes

go-mls differs from OpenMLS in a few places that matter for interop:

- **LastResort extension** goes in `KeyPackageExtensions`, not `LeafExtensions`.
  OpenMLS rejects LastResort in leaf nodes via `is_valid_in_leaf_node()`.

- **Capabilities.extensions** should list only `[0x000a, 0xf2ee]` (LastResort
  + NostrGroupData). RatchetTree (0x0002) is a default extension in OpenMLS
  and must not be listed.

- **Lifetime** needs a clock skew margin. `not_before = now - 1h` to satisfy
  OpenMLS's strict `not_before < now` check.

- **GREASE values** in capabilities are injected by OpenMLS at runtime. go-mls
  does not inject GREASE. OpenMLS tolerates their absence.
