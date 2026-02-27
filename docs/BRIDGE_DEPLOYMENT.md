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

The PTR record maps your server's IP address back to your domain. Mail servers check that the IP sending mail resolves to the domain it claims to be.

```
<YOUR_IP>  IN  PTR  yourdomain.com.
```

**Important ordering:** The forward A record (`yourdomain.com → <IP>`) must exist and propagate before you set the PTR. Many providers verify the forward record when you create the reverse, and will reject or silently remove the PTR if the A record doesn't match.

**How to set it:** PTR records are set by the IP owner (your VPS provider), not the domain registrar. Look for "Reverse DNS", "rDNS", or "PTR" in your provider's control panel, usually under Networking or IP settings.

| Provider | Where to find it |
|----------|-----------------|
| Hetzner | Cloud Console → Server → Networking → Primary IPs → Edit |
| DigitalOcean | Droplet → Networking → PTR Records |
| Vultr | Server → Settings → IPv4 → Reverse DNS |
| Linode/Akamai | Linode → Network → IP Addresses → Edit rDNS |
| Mynymbox | Management panel → Reverse DNS |

**Verify after setting:**

```bash
dig -x YOUR_SERVER_IP +short
# Expected: yourdomain.com.
```

If the output still shows the provider's default hostname (e.g., `static.50.235.71.80.virtservers.com`), the PTR hasn't propagated or wasn't accepted. Check that your A record is correct and retry.

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
| `ORLY_BRIDGE_ALIAS_PRICE_SATS` | `4200` | Monthly alias email price in sats |
| `ORLY_BRIDGE_COMPOSE_URL` | | Public URL of the compose form |
| `ORLY_BRIDGE_SMTP_RELAY_HOST` | | SMTP smarthost for outbound delivery (e.g., `smtp.migadu.com`) |
| `ORLY_BRIDGE_SMTP_RELAY_PORT` | `587` | SMTP smarthost port (587 for STARTTLS) |
| `ORLY_BRIDGE_SMTP_RELAY_USERNAME` | | SMTP smarthost AUTH username |
| `ORLY_BRIDGE_SMTP_RELAY_PASSWORD` | | SMTP smarthost AUTH password |
| `ORLY_BRIDGE_ACL_GRPC_SERVER` | | gRPC address of ACL server (split IPC mode with paid ACL) |
| `ORLY_BRIDGE_PROFILE` | `$DATA_DIR/profile.txt` | Path to profile template file (kind 0 metadata) |

---

## NWC Wallet Setup

The bridge uses Nostr Wallet Connect (NWC) to create Lightning invoices for subscription payments. You need a Lightning wallet that supports NWC and can generate a connection string.

### Getting an NWC Connection String

**Option A: Alby (hosted, simplest)**

1. Go to [getalby.com](https://getalby.com) and create an account (or log in)
2. Go to Settings → Wallet Connections (or visit `getalby.com/apps`)
3. Click "Create a new connection"
4. Name it (e.g., "Marmot Bridge")
5. Under permissions, enable **only**: `make_invoice` and `lookup_invoice`
6. Do **not** enable `pay_invoice` or any spending permissions
7. Copy the connection string — it looks like: `nostr+walletconnect://pubkey?relay=wss://...&secret=...`

**Option B: Self-hosted (Alby Hub, LNbits, NWC-enabled node)**

Most self-hosted Lightning wallets with NWC support have an "Apps" or "Connections" section where you can create restricted connection strings. The process is similar — create a new app connection with only `make_invoice` and `lookup_invoice` permissions.

### Set the Environment Variable

```bash
export ORLY_BRIDGE_NWC_URI="nostr+walletconnect://pubkey?relay=wss://relay.getalby.com/v1&secret=hexsecret"
```

Or use `ORLY_NWC_URI` if the bridge shares the wallet with the relay.

The bridge only needs to create invoices and check payment status. It never needs `pay_invoice` or any spending capability. If your wallet forces you to grant all permissions, create a separate wallet for the bridge.

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

## Bridge Profile (Kind 0)

The bridge can publish a kind 0 (profile metadata) event on startup so Nostr clients can discover it and display a name, avatar, and description. Without a profile, clients show the bridge as an unknown pubkey.

Create a `profile.txt` in the bridge data directory using email-header format:

```
name: Marmot Bridge
about: Nostr-Email bridge at yourdomain.com. DM 'subscribe' to get started.
picture: https://yourdomain.com/avatar.png
nip05: bridge@yourdomain.com
lud16: tips@yourdomain.com
website: https://yourdomain.com
banner: https://yourdomain.com/banner.png
```

The bridge reads this file on every startup and publishes a kind 0 event signed with its identity. Lines with empty values are omitted. Comments (lines starting with `#`) are ignored.

To use a custom path instead of `$ORLY_BRIDGE_DATA_DIR/profile.txt`:

```bash
export ORLY_BRIDGE_PROFILE=/etc/orly/bridge-profile.txt
```

A template is provided at `profile.example.txt` in the repository root.

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

## Pre-Flight Checklist

Run this before requesting port 25 from your provider, or before going live. Every check must pass.

```bash
# Replace with your actual domain and IP
DOMAIN=yourdomain.com
IP=1.2.3.4

echo "=== MX ==="
dig MX $DOMAIN +short
# Expected: 10 yourdomain.com.  (or your MX host)

echo "=== SPF ==="
dig TXT $DOMAIN +short | grep spf
# Expected: "v=spf1 ip4:$IP -all"

echo "=== DKIM ==="
dig TXT marmot._domainkey.$DOMAIN +short
# Expected: "v=DKIM1; k=rsa; p=MIIBIj..."

echo "=== DMARC ==="
dig TXT _dmarc.$DOMAIN +short
# Expected: "v=DMARC1; p=none; ..." or "p=quarantine"

echo "=== PTR ==="
dig -x $IP +short
# Expected: yourdomain.com.  (MUST match your domain, not the provider default)

echo "=== A record (forward) ==="
dig A $DOMAIN +short
# Expected: $IP
```

If PTR still shows the provider's default hostname, see the Reverse DNS section above. Many providers require the forward A record to exist before they accept the PTR.

Also check [MX Toolbox email health](https://mxtoolbox.com/emailhealth/) for your domain — it flags common issues that will cause mail rejection.

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

---

## SMTP Smarthost (Migadu, Mailgun, etc.)

Instead of direct MX delivery with self-managed DKIM/SPF, you can relay outbound email through a hosted email provider. This simplifies DNS setup and improves deliverability since the provider handles IP reputation.

When `ORLY_BRIDGE_SMTP_RELAY_HOST` is set, the bridge sends all outbound mail through the smarthost using STARTTLS + PLAIN authentication instead of direct MX delivery.

```bash
export ORLY_BRIDGE_SMTP_RELAY_HOST=smtp.migadu.com
export ORLY_BRIDGE_SMTP_RELAY_PORT=587
export ORLY_BRIDGE_SMTP_RELAY_USERNAME=bridge@yourdomain.com
export ORLY_BRIDGE_SMTP_RELAY_PASSWORD=your-app-password
```

The bridge's own DKIM signing (`ORLY_BRIDGE_DKIM_KEY`) is bypassed when using a smarthost since the provider signs outbound mail with their own DKIM keys.

---

## Example: Migadu Deployment (relay.orly.dev)

This is a worked example of deploying the bridge on a Linode VPS using Migadu as the hosted email provider for the subdomain `relay.orly.dev`. The domain registrar is Namecheap.

### 1. Add Domain in Migadu

Log into `admin.migadu.com`, go to Domains, add `relay.orly.dev`.

Migadu will show a list of required DNS records and a verification TXT value unique to your account.

### 2. DNS Records (Namecheap)

In Namecheap's Advanced DNS panel for `orly.dev`, add the following records. Since `relay.orly.dev` is a subdomain, the Host field is `relay` (not `@`).

**MX Records**

| Type | Host | Value | Priority | TTL |
|------|------|-------|----------|-----|
| MX | relay | aspmx1.migadu.com | 10 | 1 min |
| MX | relay | aspmx2.migadu.com | 20 | 1 min |

**TXT Records**

| Type | Host | Value | TTL |
|------|------|-------|-----|
| TXT | relay | v=spf1 include:spf.migadu.com -all | 1 min |
| TXT | relay | hosted-email-verify=XXXXXXXX | 1 min |
| TXT | _dmarc.relay | v=DMARC1; p=quarantine; | 1 min |

Replace `XXXXXXXX` with the verification value from Migadu's admin panel.

**CNAME Records (DKIM)**

| Type | Host | Value | TTL |
|------|------|-------|-----|
| CNAME | key1._domainkey.relay | key1.relay.orly.dev._domainkey.migadu.com | 1 min |
| CNAME | key2._domainkey.relay | key2.relay.orly.dev._domainkey.migadu.com | 1 min |
| CNAME | key3._domainkey.relay | key3.relay.orly.dev._domainkey.migadu.com | 1 min |

The A record for `relay.orly.dev` pointing to the VPS (e.g., 69.164.249.71) remains unchanged. MX and A records coexist without conflict.

### 3. Verify in Migadu

After adding DNS records (propagation is typically under 5 minutes with 1-minute TTL), go back to Migadu's domain page and run the DNS check. All checks should pass.

Verify locally:

```bash
dig relay.orly.dev MX +short
# Expected: 10 aspmx1.migadu.com.  20 aspmx2.migadu.com.

dig relay.orly.dev TXT +short
# Expected: "v=spf1 include:spf.migadu.com -all"  "hosted-email-verify=..."

dig _dmarc.relay.orly.dev TXT +short
# Expected: "v=DMARC1; p=quarantine;"

dig key1._domainkey.relay.orly.dev CNAME +short
# Expected: key1.relay.orly.dev._domainkey.migadu.com.
```

### 4. Create Mailbox in Migadu

In Migadu admin, create a mailbox for the bridge (e.g., `bridge@relay.orly.dev`). Note the password — this is used for SMTP authentication.

### 5. Inbound Email Forwarding

Migadu receives inbound email for `relay.orly.dev` via its MX servers. To deliver it to the bridge's SMTP server on the VPS, set up forwarding:

**Option A: Migadu forwards to a subdomain with direct MX**

Create a subdomain `bridge.relay.orly.dev` with MX pointing directly at the VPS:

| Type | Host | Value | Priority |
|------|------|-------|----------|
| MX | bridge.relay | relay.orly.dev | 10 |

In Migadu, set up a catch-all forwarding rule to forward `*@relay.orly.dev` to `incoming@bridge.relay.orly.dev`.

On the VPS, open port 25 and redirect to the bridge's SMTP port:

```bash
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo netfilter-persistent save
```

**Option B: Migadu sieve forwarding to VPS**

In Migadu's mailbox settings, add a sieve filter or forwarding rule that re-sends incoming mail to an address handled by the bridge's SMTP server.

### 6. Bridge Environment

Add to the ORLY service environment (`.env` file or systemd unit):

```bash
ORLY_BRIDGE_ENABLED=true
ORLY_BRIDGE_DOMAIN=relay.orly.dev
ORLY_BRIDGE_SMTP_PORT=2525
ORLY_BRIDGE_SMTP_HOST=0.0.0.0
ORLY_BRIDGE_SMTP_RELAY_HOST=smtp.migadu.com
ORLY_BRIDGE_SMTP_RELAY_PORT=587
ORLY_BRIDGE_SMTP_RELAY_USERNAME=bridge@relay.orly.dev
ORLY_BRIDGE_SMTP_RELAY_PASSWORD=<migadu-password>
ORLY_BRIDGE_NWC_URI=nostr+walletconnect://...
ORLY_BRIDGE_MONTHLY_PRICE_SATS=2100
ORLY_BRIDGE_COMPOSE_URL=https://relay.orly.dev/compose
```

### 7. Build and Deploy

```bash
# Build unified binary with web UI
./scripts/update-embedded-web.sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o orly ./cmd/orly

# Deploy
ssh root@relay.orly.dev 'systemctl stop orly'
rsync -avz --compress -e "ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes" \
  orly root@relay.orly.dev:/home/mleku/.local/bin/
ssh root@relay.orly.dev 'chown mleku:mleku /home/mleku/.local/bin/orly && systemctl start orly'
```

### 8. Verify

```bash
# Check bridge started
ssh root@relay.orly.dev 'journalctl -u orly --since "1 min ago" | grep bridge'
# Expected: "bridge identity: npub1..." and "bridge started"

# Test outbound: DM the bridge npub with "subscribe"
# Test inbound: send email to npub1...@relay.orly.dev
```

---

## Client Setup: White Noise

[White Noise](https://whitenoise.ing) is a mobile/desktop Nostr messaging client that uses NIP-17 gift-wrapped DMs. When searching for the bridge by its npub, White Noise may show the profile but say the bridge "isn't on White Noise yet." This means White Noise cannot find the bridge's relay list and doesn't know where to send messages.

### Why This Happens

White Noise (and other NIP-17 clients) discover where to send DMs by looking up the recipient's **kind 10002** event (NIP-65 relay list metadata). This event tells clients which relays a user reads from and writes to. Without it, the client has no delivery target for gift-wrapped DMs.

The bridge currently publishes a kind 0 (profile) on startup but does not publish a kind 10002 relay list. Until the bridge publishes one automatically, you need to publish it manually.

### Publishing a Relay List for the Bridge

Use any Nostr tool that can publish events with an arbitrary key. The event must be signed by the bridge's identity key (the nsec used for `ORLY_BRIDGE_NSEC`, or the relay's auto-generated identity if no explicit nsec is set).

The kind 10002 event looks like:

```json
{
  "kind": 10002,
  "content": "",
  "tags": [
    ["r", "wss://relay.orly.dev/"],
    ["r", "wss://relay.damus.io/", "read"],
    ["r", "wss://nos.lol/", "read"]
  ]
}
```

Each `r` tag is a relay URL. Tags without a marker (no third element) mean both read and write. `"read"` means the bridge reads from that relay but doesn't write there. `"write"` means write-only.

At minimum, include the bridge's own relay as a read+write relay:

```json
["r", "wss://your-relay-domain/"]
```

Adding popular public relays as `read` entries helps clients that don't connect to your relay directly find the bridge.

### Step-by-Step with nak

[nak](https://github.com/fiatjaf/nak) is a command-line Nostr tool. Install it, then publish the relay list:

```bash
# Set the bridge's secret key
export NOSTR_SECRET_KEY=nsec1...  # The bridge identity nsec

# Publish kind 10002 with your relay as read+write, plus popular relays as read-only
nak event \
  --kind 10002 \
  --tag r='wss://relay.orly.dev/' \
  --tag r='wss://relay.damus.io/;read' \
  --tag r='wss://nos.lol/;read' \
  wss://relay.orly.dev wss://relay.damus.io wss://nos.lol
```

Publish to multiple relays so clients can discover it regardless of which relays they check.

### Step-by-Step with nostril

[nostril](https://github.com/jb55/nostril) is another CLI option:

```bash
nostril --kind 10002 \
  --sec <bridge-hex-secret> \
  --tag r 'wss://relay.orly.dev/' \
  --tag r 'wss://relay.damus.io/' read \
  --tag r 'wss://nos.lol/' read \
  | websocat wss://relay.orly.dev
```

### Verifying the Relay List

After publishing, verify that clients can find it:

```bash
# With nak
nak req -k 10002 -a <bridge-pubkey-hex> wss://relay.orly.dev

# Or check on a web viewer
# https://njump.me/<bridge-npub>
```

White Noise should now show the bridge as reachable. Search for the bridge npub again and the "isn't on White Noise yet" message should be replaced with the ability to start a conversation.

### Recommended Relay List

For maximum discoverability, include the bridge's own relay plus 2-3 popular relays:

| Relay | Marker | Purpose |
|-------|--------|---------|
| `wss://your-relay.com/` | (none) | Primary — bridge reads and writes here |
| `wss://relay.damus.io/` | read | Popular relay, helps discovery |
| `wss://nos.lol/` | read | Popular relay, helps discovery |
| `wss://relay.nostr.band/` | read | Indexer relay, helps discovery |

### Future: Automatic Relay List Publishing

A future ORLY release will have the bridge publish kind 10002 automatically on startup alongside the kind 0 profile, eliminating this manual step.
