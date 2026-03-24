# sm3sh-deploy

Deploys sm3sh frontend assets to a running orly server. Bundles the asset directory as `tar.xz -9`, signs the bundle hash with BIP-340, uploads in 512 KB chunks, and the server does an atomic symlink pivot to the new version.

## Prerequisites

- `xz` installed on both client and server
- A deploy nsec (bech32-encoded Nostr secret key)
- The server must have the corresponding hex pubkey set in `ORLY_DEPLOY_PUBKEY`
- The server must be running in disk mode (`ORLY_SMESH3_DIR` set)

## Usage

```
go run ./cmd/sm3sh-deploy --url https://sm3sh.mleku.dev --dir app/smesh3
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | (required) | sm3sh base URL |
| `--dir` | `app/smesh3` | directory to bundle |
| `--nsec` | | deploy nsec (bech32) |

### Environment

| Variable | Description |
|----------|-------------|
| `DEPLOY_NSEC` | Fallback for `--nsec` flag. Store in `~/.config/sm3sh-deploy.env` or equivalent. |

## How it works

### Client (`cmd/sm3sh-deploy/main.go`)

1. Walks `--dir`, creates an in-memory tar archive
2. Pipes through `xz -9 --stdout` for compression
3. Computes SHA-256 of the compressed bundle
4. Signs the hash with BIP-340 (Schnorr over secp256k1)
5. `POST /__deploy?action=begin` — opens a session with hash and part count
6. `POST /__deploy?action=part` — uploads each 512 KB chunk
7. `POST /__deploy?action=apply` — sends signature in `X-Sig` header, triggers extraction

### Server (`app/deploy.go`)

1. `begin` — creates a deploy session keyed by bundle hash
2. `part` — stores chunks (max 1 MB each, 50 MB total)
3. `apply` — reassembles chunks, verifies SHA-256 matches, verifies BIP-340 signature against `ORLY_DEPLOY_PUBKEY`, then:
   - Decompresses with `xz -d --stdout` and extracts tar to a versioned directory (`{dir}-{hash8}`)
   - Creates a new symlink `{dir}.new` → versioned directory
   - Atomic `rename(2)` swaps the symlink into place (first deploy: renames old dir out of the way, creates symlink)
   - Deletes the previous version directory
   - Bumps the internal version counter (triggers SSE reload to all connected clients)

### Server config

Set `ORLY_DEPLOY_PUBKEY` to the 64-char hex x-only pubkey. The client prints its pubkey on startup.
