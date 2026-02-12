# Local Testing

Start a local ORLY relay + smesh dev server for testing.

## Steps

1. Run the local testing script:
   ```bash
   ./scripts/localtesting.sh
   ```

2. Report the result to the user. If successful, remind them:
   - Relay: `ws://localhost:3334`
   - Smesh: `http://localhost:5173`
   - Stop with: `/stoptesting`
   - Rebuild relay with: `/updatetesting`

## Requirements

- `SUPERUSER` env var must be set to your npub
- `SMESH_SRC` env var (optional, defaults to `~/src/git.mleku.dev/mleku/smesh`)

## What it does

- Builds the relay binary (`./cmd/orly`)
- Starts `orly launcher` with split IPC mode (badger DB on :50061, follows ACL on :50062)
- Starts smesh dev server with Vite hot reload on :5173
- Data stored in `/tmp/orly-localtest/`
