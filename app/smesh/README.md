# Smesh

Nostr client. React 18 + TypeScript + Vite + Tailwind. Runs on ORLY relay.

## Build

```bash
bun install
bun run build   # production build to dist/
bun run dev     # dev server with hot reload
```

## Deploy

Static files served by Caddy at smesh.mleku.dev.

```bash
rsync -avz --delete -e "ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes" \
  dist/ root@relay.orly.dev:/home/mleku/smesh/dist/
```

## Features

- Notes feed with social graph filtering
- DMs (NIP-04 and NIP-17 gift-wrap)
- Profile management
- Relay configuration
- Notifications
- Bookmarks and pins
- Media upload (Blossom)
- Zaps (NWC)
- [NIRC — relay-local IRC](docs/NIRC.md)

## NIRC

Relay-scoped public chat channels. IRC model: relay is chanserv, npubs are identity, channel owners control membership. Channels are invite-only by default. Owner delegates mods who can hide messages and block users.

See [docs/NIRC.md](docs/NIRC.md) for the full specification.
