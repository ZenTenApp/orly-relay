# Update Testing

Rebuild the relay and restart it. Smesh hot-reloads automatically via Vite.

## Steps

1. Run the update testing script:
   ```bash
   ./scripts/updatetesting.sh
   ```

2. Report the result to the user. If successful, confirm the relay has been rebuilt and restarted.

## Requirements

- Local testing must already be running (started with `/localtesting`)
- `SUPERUSER` env var must be set to your npub

## What it does

- Rebuilds the relay binary from `./cmd/orly`
- Kills the existing relay launcher process tree
- Restarts with the same configuration
- Smesh doesn't need restarting (Vite hot-reloads)
