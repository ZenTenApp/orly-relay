# Nostr-Email Bridge Deployment Guide

This document covers deploying the ORLY Nostr-Email bridge (Marmot DM to SMTP).

---

## Architecture Overview

The bridge converts between Nostr Marmot DMs and standard email:

- **Inbound email** (someone@internet → npub@yourdomain): SMTP server receives the email, parses MIME, zips non-text parts, encrypts with ChaCha20-Poly1305, uploads to Blossom, and delivers as a Marmot DM with a fragment-key URL.
- **Outbound email** (npub sends DM with To: headers): Bridge parses the DM, checks subscription + rate limits, and sends via SMTP with DKIM signing.

### Deployment Modes

| Mode | Description | Config |
|------|-------------|--------|
| **Monolithic** | Bridge runs inside the ORLY relay process | `ORLY_BRIDGE_ENABLED=true` |
| **Standalone** | Bridge runs as a separate process | `orly bridge` subcommand |
| **Launcher** | Bridge managed by the process supervisor | `ORLY_LAUNCHER_BRIDGE_ENABLED=true` |

Monolithic is simplest for single-server deployments. Standalone is useful when the relay and bridge run on different hosts.

---

## Quick Start (Monolithic)

```bash
# 1. Generate DKIM keys
./scripts/generate-dkim.sh yourdomain.com

# 2. Set environment variables
export ORLY_BRIDGE_ENABLED=true
export ORLY_BRIDGE_DOMAIN=yourdomain.com
export ORLY_BRIDGE_SMTP_PORT=2525
export ORLY_BRIDGE_DKIM_KEY=/path/to/dkim-private.pem
export ORLY_BRIDGE_DKIM_SELECTOR=marmot
export ORLY_BRIDGE_COMPOSE_URL=https://yourdomain.com/compose

# 3. Start relay (bridge starts automatically)
./orly
```

The compose form is served at `/compose` and the decrypt page at `/decrypt`.

---

## DNS Configuration

All DNS records are required for email deliverability. Without them, outbound email will be rejected by most providers.

### MX Record

Points incoming email for your domain to your bridge server.

```
yourdomain.com.  IN  MX  10  mail.yourdomain.com.
mail.yourdomain.com.  IN  A  <YOUR_SERVER_IP>
```

If your relay is the same host:

```
yourdomain.com.  IN  MX  10  yourdomain.com.
```

### SPF Record

Authorizes your server to send email for the domain.

```
yourdomain.com.  IN  TXT  "v=spf1 ip4:<YOUR_SERVER_IP> -all"
```

For multiple IPs:

```
yourdomain.com.  IN  TXT  "v=spf1 ip4:1.2.3.4 ip4:5.6.7.8 -all"
```

### DKIM Record

The `generate-dkim.sh` script outputs the exact DNS record. The selector defaults to `marmot`.

```
marmot._domainkey.yourdomain.com.  IN  TXT  "v=DKIM1; k=rsa; p=MIIBIjANBg..."
```

The value is the base64-encoded RSA public key (no line breaks in the DNS TXT record).

### DMARC Record

Tells receivers how to handle mail that fails SPF/DKIM.

```
_dmarc.yourdomain.com.  IN  TXT  "v=DMARC1; p=quarantine; rua=mailto:postmaster@yourdomain.com"
```

For testing, start with `p=none` (report only, don't reject):

```
_dmarc.yourdomain.com.  IN  TXT  "v=DMARC1; p=none; rua=mailto:postmaster@yourdomain.com"
```

### Reverse DNS (PTR)

Your VPS provider's control panel should let you set the PTR record for your IP. Set it to match your MX hostname:

```
<YOUR_IP>  IN  PTR  yourdomain.com.
```

Most providers (Hetzner, DigitalOcean, Vultr, Linode) have a web UI for this under Networking or DNS settings. PTR records are set by the IP owner, not the domain registrar.

---

## DKIM Key Generation

Use the provided script:

```bash
./scripts/generate-dkim.sh yourdomain.com [selector]
```

This generates:
- `dkim-private.pem` — the private key (keep secret, set `ORLY_BRIDGE_DKIM_KEY` to this path)
- Console output with the DNS TXT record to add

If you prefer manual generation:

```bash
openssl genrsa -out dkim-private.pem 2048
openssl rsa -in dkim-private.pem -pubout -outform DER | openssl base64 -A
# Use the base64 output as the "p=" value in the DNS TXT record
```

---

## Port 25 Workarounds

Many cloud providers block port 25 by default. The bridge listens on port 2525 (configurable via `ORLY_BRIDGE_SMTP_PORT`). You need to get traffic from port 25 to your bridge port.

### Method 1: iptables Redirect (Recommended)

Redirect port 25 to your bridge port. No binary changes needed.

```bash
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
```

Make it persistent across reboots:

```bash
sudo apt install iptables-persistent
sudo netfilter-persistent save
```

### Method 2: setcap (Bind Port 25 Directly)

Grant the binary permission to bind low ports without root:

```bash
sudo setcap 'cap_net_bind_service=+ep' ./orly
```

Then set `ORLY_BRIDGE_SMTP_PORT=25`.

Note: `setcap` is cleared if you replace the binary, so re-run after each deploy.

### Method 3: systemd Socket Activation

systemd listens on port 25 and passes the file descriptor to the bridge.

```ini
# /etc/systemd/system/orly-smtp.socket
[Socket]
ListenStream=25
Accept=no

[Install]
WantedBy=sockets.target
```

Then configure the bridge to accept the systemd socket. This requires custom code not yet in the bridge — use iptables for now.

### Method 4: Provider Unblock Request

Some providers will open port 25 on request:

| Provider | Process |
|----------|---------|
| DigitalOcean | Support ticket, explain mail server use case |
| Vultr | Support ticket |
| Linode/Akamai | Support ticket (usually approved for new accounts after a period) |
| AWS EC2 | Request via SES, or use an Elastic IP with port 25 unblock form |

### Method 5: Providers with Port 25 Open

These providers do not block port 25 by default:

- **Hetzner** (cloud and dedicated)
- **OVH**
- **Most dedicated server providers** (Leaseweb, ServerCheap, etc.)
- **Self-hosted / home server** (if your ISP allows it)

### Method 6: SSH Tunnel

If you have a separate server with port 25 open:

```bash
# On the port-25-capable server:
ssh -R 2525:localhost:2525 user@bridge-host
```

Or use autossh for persistent tunnels:

```bash
autossh -M 0 -N -R 25:localhost:2525 user@bridge-host
```

### Method 7: WireGuard / VPN Tunnel

Run a VPN between a port-25-capable server and your bridge host:

```bash
# On the port-25 server, forward port 25 to the bridge via WireGuard peer IP
iptables -t nat -A PREROUTING -p tcp --dport 25 -j DNAT --to-destination 10.0.0.2:2525
iptables -t nat -A POSTROUTING -j MASQUERADE
```

### Method 8: Docker with Host Networking

```bash
docker run --network host -e ORLY_BRIDGE_SMTP_PORT=2525 ...
# Then use iptables method above on the host
```

### Method 9: Separate Mail Server (Postfix Relay)

Run Postfix on a port-25-capable host and pipe to the bridge via LMTP or HTTP webhook:

```
# /etc/postfix/main.cf
transport_maps = hash:/etc/postfix/transport

# /etc/postfix/transport
yourdomain.com  lmtp:bridge-host:2525
```

This is the most complex option but works well if you already run a mail server.

---

## Environment Variables

All bridge configuration is via environment variables with the `ORLY_BRIDGE_` prefix.

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_BRIDGE_ENABLED` | `false` | Enable the email bridge |
| `ORLY_BRIDGE_DOMAIN` | | Email domain (e.g., `relay.example.com`) |
| `ORLY_BRIDGE_NSEC` | | Bridge identity nsec (default: relay identity from database) |
| `ORLY_BRIDGE_RELAY_URL` | | WebSocket relay URL for standalone mode |
| `ORLY_BRIDGE_SMTP_PORT` | `2525` | SMTP server listen port |
| `ORLY_BRIDGE_SMTP_HOST` | `0.0.0.0` | SMTP server listen address |
| `ORLY_BRIDGE_DATA_DIR` | `$ORLY_DATA_DIR/bridge` | Bridge data directory |
| `ORLY_BRIDGE_DKIM_KEY` | | Path to DKIM private key PEM file |
| `ORLY_BRIDGE_DKIM_SELECTOR` | `marmot` | DKIM selector for DNS TXT record |
| `ORLY_BRIDGE_NWC_URI` | | NWC connection string for subscription payments (falls back to `ORLY_NWC_URI`) |
| `ORLY_BRIDGE_MONTHLY_PRICE_SATS` | `2100` | Monthly subscription price in sats |
| `ORLY_BRIDGE_COMPOSE_URL` | | Public URL of the compose form |

---

## NWC Wallet Setup

The bridge uses Nostr Wallet Connect (NWC) to create Lightning invoices for subscription payments.

1. Use a wallet that supports NWC (Alby, Mutiny, etc.)
2. Generate a connection string restricted to `make_invoice` and `lookup_invoice` methods only
3. Set `ORLY_BRIDGE_NWC_URI` (or `ORLY_NWC_URI` if shared with the relay)

The bridge only needs to create invoices and check payment status. It never needs `pay_invoice` or any spending capability.

---

## Subscription Model

Users send the word `subscribe` as a DM to the bridge identity. The bridge:

1. Creates a Lightning invoice via NWC
2. Sends the invoice back as a DM
3. Polls for payment (configurable interval)
4. On payment: activates a 30-day subscription, confirms via DM

Subscription state is stored in `$ORLY_BRIDGE_DATA_DIR/subscriptions.json`.

Rate limits for subscribed users:
- 10 outbound emails per hour per user
- 50 outbound emails per day per user
- 100 outbound emails per hour globally
- 500 outbound emails per day globally
- 30 second minimum interval between sends

---

## Blossom Server Configuration

The bridge uses Blossom for attachment storage. Ensure Blossom is enabled:

```bash
ORLY_BLOSSOM_ENABLED=true
```

When an inbound email has non-plaintext attachments, the bridge:
1. Bundles them into a flat ZIP
2. Encrypts with ChaCha20-Poly1305 (random key)
3. Uploads to Blossom as an opaque blob
4. Includes the URL with `#key` fragment in the DM

The encryption key is in the URL fragment, which per RFC 3986 is never sent to the server. Only the DM recipient (who has the full URL) can decrypt.

---

## Compose Form

The compose form at `/compose` is a static HTML page with no server-side logic. It:

- Reads pre-populated fields from the URL fragment: `#to=alice@example.com&subject=Re: Hello`
- Formats the DM with RFC 822-style headers
- Copies the formatted text to the clipboard
- The user pastes this into a Nostr client as a DM to the bridge

The decrypt page at `/decrypt` allows email recipients to decrypt attachment URLs:

- Enter the Blossom URL with `#key` fragment
- The page downloads the blob, decrypts locally in the browser using ChaCha20-Poly1305
- The decrypted file is offered for download
- The key never leaves the browser

---

## Troubleshooting

### Verify DNS Records

```bash
# Check MX record
dig MX yourdomain.com +short

# Check SPF
dig TXT yourdomain.com +short

# Check DKIM
dig TXT marmot._domainkey.yourdomain.com +short

# Check DMARC
dig TXT _dmarc.yourdomain.com +short

# Check PTR (reverse DNS)
dig -x YOUR_SERVER_IP +short
```

### Online Tools

- [MX Toolbox](https://mxtoolbox.com/) — comprehensive DNS and mail server testing
- [mail-tester.com](https://www.mail-tester.com/) — send a test email, get a deliverability score
- [DKIM Validator](https://dkimvalidator.com/) — verify DKIM signing works

### Common Issues

**Outbound email rejected as spam:**
- Check SPF, DKIM, and DMARC records are all set correctly
- Verify PTR record matches your sending domain
- Check IP reputation at [multirbl.valli.org](http://multirbl.valli.org/)
- New IPs may need "warming up" — start with low volume

**SMTP connection refused on port 25:**
- Your provider likely blocks port 25; see Port 25 Workarounds above
- Verify with: `telnet yourdomain.com 25`

**Bridge identity mismatch:**
- In monolithic mode, the bridge uses the relay's identity from the database
- Set `ORLY_BRIDGE_NSEC` explicitly to override
- Check the log for "bridge identity:" at startup

**Subscription payments not working:**
- Verify NWC URI is correct and the wallet is online
- The NWC URI must support `make_invoice` and `lookup_invoice`
- Check the bridge data directory for `subscriptions.json`

**Attachments not working:**
- Ensure `ORLY_BLOSSOM_ENABLED=true`
- The Blossom server must be reachable from the compose/decrypt pages
- Check that the domain serves Blossom at `/blossom/` or root

---

## Example: Full Production Setup

```bash
# DNS records (set via your registrar):
# yourdomain.com  MX 10  yourdomain.com
# yourdomain.com  TXT    "v=spf1 ip4:1.2.3.4 -all"
# marmot._domainkey.yourdomain.com  TXT  "v=DKIM1; k=rsa; p=..."
# _dmarc.yourdomain.com  TXT  "v=DMARC1; p=quarantine; rua=mailto:postmaster@yourdomain.com"

# Generate DKIM key
./scripts/generate-dkim.sh yourdomain.com marmot

# Environment
export ORLY_PORT=3334
export ORLY_BRIDGE_ENABLED=true
export ORLY_BRIDGE_DOMAIN=yourdomain.com
export ORLY_BRIDGE_SMTP_PORT=2525
export ORLY_BRIDGE_DKIM_KEY=/etc/orly/dkim-private.pem
export ORLY_BRIDGE_DKIM_SELECTOR=marmot
export ORLY_BRIDGE_COMPOSE_URL=https://yourdomain.com/compose
export ORLY_BRIDGE_NWC_URI="nostr+walletconnect://..."
export ORLY_BRIDGE_MONTHLY_PRICE_SATS=2100
export ORLY_BLOSSOM_ENABLED=true

# Redirect port 25 to bridge SMTP port
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo netfilter-persistent save

# Start
./orly
```
