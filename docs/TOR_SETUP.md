# Tor Hidden Service Setup for ORLY Relay

This guide explains how to configure ORLY to automatically mirror your relay as a Tor hidden service, making it accessible via a `.onion` address.

## Overview

When Tor support is enabled:
1. ORLY listens on a dedicated internal port for Tor traffic
2. The Tor daemon forwards `.onion` traffic to this port
3. ORLY automatically detects the `.onion` address
4. The `.onion` address is included in NIP-11 relay information

## Quick Start

### Development (Local Testing)

```bash
# One-time setup (requires Tor installed)
./scripts/tor-dev-setup.sh

# Start relay with Tor
ORLY_TOR_ENABLED=true ORLY_TOR_HS_DIR=~/.tor/orly-dev/hidden_service ./orly
```

### Production

```bash
# One-time setup (requires root)
sudo ./scripts/tor-setup.sh

# Start relay with Tor
ORLY_TOR_ENABLED=true ORLY_TOR_HS_DIR=/var/lib/tor/orly-relay ./orly
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_TOR_ENABLED` | `false` | Enable Tor hidden service integration |
| `ORLY_TOR_PORT` | `3336` | Internal port Tor forwards traffic to |
| `ORLY_TOR_HS_DIR` | - | Path to Tor's HiddenServiceDir |
| `ORLY_TOR_ONION_ADDRESS` | - | Manual `.onion` override (optional) |

### Example Configurations

**Basic Tor setup:**
```bash
export ORLY_TOR_ENABLED=true
export ORLY_TOR_HS_DIR=/var/lib/tor/orly-relay
./orly
```

**Custom port:**
```bash
export ORLY_TOR_ENABLED=true
export ORLY_TOR_PORT=3337
export ORLY_TOR_HS_DIR=/var/lib/tor/orly-relay
./orly
```

**Manual address (if auto-detection doesn't work):**
```bash
export ORLY_TOR_ENABLED=true
export ORLY_TOR_ONION_ADDRESS=abc123xyz.onion
./orly
```

## How It Works

### Architecture

```
Internet Users              Tor Users
      │                         │
      ▼                         ▼
┌──────────┐            ┌──────────────┐
│ Regular  │            │   Tor        │
│ Traffic  │            │   Network    │
│ (HTTPS)  │            │              │
└────┬─────┘            └──────┬───────┘
     │                         │
     │  Port 443               │  .onion:80
     ▼                         ▼
┌─────────────────────────────────────┐
│           ORLY Relay                 │
│                                      │
│  ┌─────────────┐  ┌───────────────┐ │
│  │ Main Server │  │  Tor Service  │ │
│  │  Port 3334  │  │   Port 3336   │ │
│  └──────┬──────┘  └───────┬───────┘ │
│         │                 │          │
│         └────────┬────────┘          │
│                  ▼                   │
│          ┌────────────┐              │
│          │  Database  │              │
│          └────────────┘              │
└─────────────────────────────────────┘
```

### Address Detection

1. The Tor daemon creates a hidden service directory containing:
   - `hostname` - The `.onion` address
   - `hs_ed25519_secret_key` - Private key (persistent)
   - `hs_ed25519_public_key` - Public key

2. ORLY watches the `hostname` file and automatically detects the address

3. The address is included in NIP-11 relay information under the `addresses` field

### NIP-11 Integration

When Tor is enabled and the `.onion` address is detected, the NIP-11 relay info includes:

```json
{
  "name": "ORLY",
  "description": "...",
  "pubkey": "...",
  "addresses": [
    "wss://relay.example.com",
    "ws://abc123xyz.onion/"
  ]
}
```

## Manual Tor Configuration

If you prefer to configure Tor manually instead of using the setup scripts:

### 1. Install Tor

**Debian/Ubuntu:**
```bash
sudo apt update && sudo apt install tor
```

**Arch Linux:**
```bash
sudo pacman -S tor
```

**macOS:**
```bash
brew install tor
```

### 2. Configure Hidden Service

Add to `/etc/tor/torrc`:

```
HiddenServiceDir /var/lib/tor/orly-relay/
HiddenServicePort 80 127.0.0.1:3336
```

### 3. Set Permissions

```bash
# Create directory
sudo mkdir -p /var/lib/tor/orly-relay

# Set ownership (Debian/Ubuntu)
sudo chown debian-tor:debian-tor /var/lib/tor/orly-relay
sudo chmod 700 /var/lib/tor/orly-relay

# Or on other systems
sudo chown tor:tor /var/lib/tor/orly-relay
```

### 4. Restart Tor

```bash
sudo systemctl restart tor
```

### 5. Verify

```bash
# Check the .onion address
sudo cat /var/lib/tor/orly-relay/hostname
```

## Systemd Service

For production deployments, use the provided systemd unit:

```bash
# Copy unit file
sudo cp deploy/orly-tor.service /etc/systemd/system/

# Edit configuration
sudo nano /etc/systemd/system/orly-tor.service

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable orly-tor
sudo systemctl start orly-tor
```

### Permissions for Hidden Service Directory

The ORLY process needs read access to the Tor hidden service directory:

**Option 1: Add user to Tor group**
```bash
sudo usermod -aG debian-tor orly
```

**Option 2: Use ACLs**
```bash
sudo setfacl -R -m u:orly:rx /var/lib/tor/orly-relay
```

## Troubleshooting

### .onion address not appearing in NIP-11

1. Check if Tor is running:
   ```bash
   systemctl status tor
   ```

2. Check if hostname file exists:
   ```bash
   cat /var/lib/tor/orly-relay/hostname
   ```

3. Check ORLY logs for Tor-related messages

4. Verify environment variables are set:
   ```bash
   echo $ORLY_TOR_ENABLED
   echo $ORLY_TOR_HS_DIR
   ```

### Permission denied errors

Ensure ORLY can read the hidden service directory:
```bash
# Check permissions
ls -la /var/lib/tor/orly-relay/

# Fix with ACL
sudo setfacl -m u:$(whoami):rx /var/lib/tor/orly-relay
```

### Tor connection timeouts

1. Check Tor logs:
   ```bash
   journalctl -u tor -f
   ```

2. For development, check:
   ```bash
   tail -f ~/.tor/orly-dev/tor.log
   ```

3. Ensure Tor can reach the network (check firewall rules)

### Different .onion address after restart

This means the hidden service key was lost. The key is stored in:
- Production: `/var/lib/tor/orly-relay/hs_ed25519_secret_key`
- Development: `~/.tor/orly-dev/hidden_service/hs_ed25519_secret_key`

To preserve your `.onion` address, back up the entire hidden service directory.

## Security Considerations

1. **Keep the hidden service key safe** - The `hs_ed25519_secret_key` file is your identity. If compromised, attackers can impersonate your relay.

2. **Restrict file permissions** - Hidden service directories should be `chmod 700`.

3. **Separate Tor traffic** - The dedicated Tor port (3336) keeps Tor traffic isolated from regular traffic.

4. **Regular updates** - Keep Tor updated for security patches.

## Testing with Tor Browser

1. Download Tor Browser from https://www.torproject.org/

2. Navigate to your `.onion` address:
   ```
   ws://your-address.onion/
   ```

3. Or test with curl over Tor:
   ```bash
   curl --socks5-hostname localhost:9050 -H "Accept: application/nostr+json" http://your-address.onion/
   ```
