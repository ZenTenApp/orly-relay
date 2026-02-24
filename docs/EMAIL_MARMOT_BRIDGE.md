# ORLY Nostr-Email Bridge — Scope & Deliverables

**Version:** 0.2 (Revised Draft)
**Date:** 2026-02-19
**Client:** mleku
**Contractor:** [name]
**Budget:** $1,400 USD (35 hours × $40/hr)
**Terms:** 30% upfront ($420), remainder on delivery
**Billing:** Trackstr

---

## 1. Overview

A bidirectional Nostr ↔ Email bridge integrated into the ORLY relay, enabling
whitelisted npubs to send and receive email as `npub@domain`. The Nostr side
uses the Marmot protocol (MLS-based E2E encrypted messaging) for all
communication between users and the bridge. Access is gated by paid
subscriptions via NWC lightning invoices.

---

## 2. Core Architecture

```
                    Marmot DM (receive email, subscribe)
┌─────────────┐  ◄──────────────────────────┐
│  Nostr User  │                             │
│  (npub)      │  Marmot DM (send email)     │
│              │ ──────────────────────────►  │
└──────┬──────┘  (paste from compose form)   │
       │                               ┌─────┴────────┐     SMTP      ┌──────────┐
       │  browser                      │  Bridge       │ ◄───────────► │  Email   │
       │  opens                        │  (ORLY)       │   (25/587)    │  World   │
       ▼                               └──────┬───────┘               └──────────┘
┌──────────────┐                              │
│ Compose Form  │  (no auth, pure client JS)  │
│ (static HTML) │  uploads attachments ──────►│
└──────────────┘                        ┌─────┴─────┐
  [Copy to Clipboard]                   │  Blossom   │
  user pastes into DM                   │  Server    │
                                        └───────────┘
                                        (attachments)
```

The bridge operates using the **relay's own identity** (the pubkey from
NIP-11). Users DM the relay to send email, subscribe, or contact the
operator. The bridge:
- Receives inbound email via SMTP and delivers it as Marmot DMs to the
  recipient npub
- Receives outbound email as Marmot DMs (formatted by the compose form) and
  sends it via SMTP
- Handles subscription commands (`subscribe`) via Marmot DM
- Forwards non-email DMs to the relay operator as a blind contact proxy

---

## 3. Identity Model

### 3.1 Bridge Identity

The bridge uses the **relay's keypair** rather than maintaining its own
identity. This means the NIP-11 `pubkey` field serves as both the relay
contact point and the email bridge endpoint — users DM the relay's npub to
interact with the bridge.

**Identity resolution order:**

1. **ORLY monolithic mode:** Bridge reads the relay identity directly from
   the database (`relay:identity:sk`) — same process, no configuration needed
2. **ORLY split IPC mode:** The launcher reads the relay identity from the
   database and injects it into the bridge subprocess's environment
   (`ORLY_BRIDGE_NSEC`)
3. **Standalone / external relay:** Bridge reads identity from a local file
   (`<data-dir>/bridge.nsec`). If the file doesn't exist, generates a new
   keypair and persists it. This mode requires the operator to ensure the
   bridge's pubkey matches what the relay advertises in NIP-11 (or the relay
   doesn't advertise it at all and the bridge operates independently)

The bridge logs its npub at startup in all modes.

### 3.2 User Email Addresses

- Each whitelisted npub gets the email address: `<npub>@<domain>`
- The npub is the bech32-encoded public key (e.g., `npub1abc...xyz@relay.example.com`)
- The same npub can have addresses on multiple bridge domains — changing
  provider means changing the domain, not the identity
- **Optional NIP-05 support:** The bridge MAY serve a
  `/.well-known/nostr.json` endpoint mapping the npub portion to the
  corresponding hex pubkey, enabling NIP-05 verification at the bridge domain

### 3.3 Relay Contact Forwarding (Blind Proxy)

Any DM sent to the relay's npub that is **not** an email command (no `To:`
header, not `subscribe`, etc.) is treated as a message to the relay operator.
The bridge acts as a blind proxy:

1. User A sends a Marmot DM to the relay npub R
2. Bridge decrypts the DM (it holds R's secret key)
3. Bridge generates a random opaque reference ID for this conversation
4. Bridge re-encrypts the message content and sends it as a Marmot DM from R
   to the operator's configured npub O, prefixed with the reference ID
5. Operator O replies to R with the same reference ID
6. Bridge looks up the reference, finds A's pubkey, re-encrypts and sends
   the reply from R to A

**Neither party learns the other's pubkey.** User A only sees the relay npub.
Operator O only sees the relay npub plus a reference ID. The relay is a dead
drop.

This provides a built-in contact mechanism for any relay — users can message
the operator via the NIP-11 pubkey without either party revealing their
identity to the other. Works with any DM protocol the bridge supports
(Marmot, NIP-17, etc.)

**Configuration:**
- `ORLY_BRIDGE_OPERATOR_PUBKEY`: hex pubkey of the relay operator (required
  for contact forwarding to work; if unset, non-email DMs get a "not
  supported" auto-reply)
- Reference IDs expire after a configurable TTL (default: 30 days)

---

## 4. Functional Requirements

### 4.1 Inbound Email (Email → Nostr)

1. Bridge receives email via SMTP on port 25
2. Recipient address is parsed: extract npub from local part
3. If npub is not whitelisted or subscription is expired → reject with 550
4. The `text/plain` MIME part is used as the DM body (no processing)
5. DM body is truncated to **64 KB** (NIP-44 plaintext limit: 65,535 bytes)
6. All non-plaintext MIME parts (text/html, attachments) are bundled:
   a. Collected into a single zip file (flat structure: `email.html`,
      `attachment1.pdf`, `photo.jpg`, etc.)
   b. Zip encrypted with a random 256-bit symmetric key (ChaCha20-Poly1305)
   c. Ciphertext uploaded to Blossom via `PUT /upload` with kind 24242 auth
   d. Fragment-key URL included in the DM:
      `https://blossom.example/<sha256>#<hex-key>`
7. If no non-plaintext parts exist, no zip/upload occurs — just the DM text
8. Bridge sends a Marmot DM to the recipient npub containing:
   - **From:** sender email address
   - **Subject:** email subject line
   - **Body:** text/plain content (unmodified, ≤64 KB)
   - **Attachment URL:** single Blossom fragment-key URL (if any parts were
     zipped)
   - **Reply link:** pre-populated compose form URL (see §4.2)
9. All other email headers are discarded

**Example received DM:**

```
From: alice@example.com
Subject: Meeting tomorrow

Hey, are we still on for 3pm?
Let me know if the time works.

https://blossom.example/abc123def456#9a8b7c6d5e4f

---
Reply: https://bridge.domain/compose#from=npub1xxx%40bridge.domain&to=alice%40example.com&subject=Re%3A%20Meeting%20tomorrow&quote=...
```

### 4.2 Outbound Email (Nostr → Email)

> **This section contains a proposed revision. Both the original and proposed
> designs are shown below for comparison. Please approve one.**

---

#### 4.2 OLD: Outbound via Raw DM

The user hand-writes RFC 822-style headers inside a Marmot DM:

```
To: alice@example.com, bob@example.com
Cc: carol@example.com
Subject: Hello from Nostr

Message body starts here.
Everything after the blank line is the email body.
```

No `From:` header is needed — the bridge derives the sender address from the
Nostr event's pubkey (`<sender-npub>@<domain>`).

The bridge parses the headers, validates the sender, and sends SMTP.

**Problems:**

1. **Users must know SMTP header format** — `To:`, `Subject:`, blank line
   separator. Non-technical users will get this wrong.
2. **No input validation before send** — if the format is wrong, the error
   takes a full MLS-encrypted round-trip back as a DM.
3. **No attachment support** — no way to attach files from within a DM.

---

#### 4.2 PROPOSED: Compose Form + DM Paste

Replace hand-written headers with a **static HTML compose form** that
formats the message for the user. The form requires **no authentication**
and has **no server-side logic** — it is a pure client-side formatting and
attachment-upload tool. The user pastes the formatted result into their
Marmot client as a DM. This works on every platform (desktop, mobile, any
browser) with zero signer/extension dependencies.

**Data flow comparison:**

```
OLD:  User hand-writes headers in DM → hope format is correct →
      MLS encrypt → relay → bridge → parse → error? another DM round-trip

NEW:  User fills browser form → form validates fields → form uploads
      attachments to Blossom → form formats message → [Copy to Clipboard]
      → user pastes into Marmot DM to bridge → bridge parses → SMTP send
```

The bridge-side DM parsing is identical to OLD (same RFC 822-style format).
The difference is that the **user never writes it by hand** — the form
generates a correctly formatted message every time, with client-side
validation catching errors before the DM is sent.

##### Compose Form UI

The bridge serves a static HTML page (embedded in the binary or hosted on
Blossom/CDN). No server-side logic. No authentication. Pure client-side JS.

```
┌─────────────────────────────────────────────────────────────────┐
│ Compose Email                                                    │
├─────────────────────────────────────────────────────────────────┤
│ From: npub1xxx@bridge.domain                    (static, fixed) │
│ To:   alice@example.com                     (pre-filled / input)│
│ Cc:   [_________________________________________]    (optional) │
│ Bcc:  [_________________________________________]    (optional) │
│ Subject: Re: Meeting tomorrow               (pre-filled / input)│
├─────────────────────────────────────────────────────────────────┤
│ ▌                                          ← cursor starts here│
│                                                                 │
│ ─────────────────────────────────────────── ← separator         │
│ > Hey, are we still on for 3pm?            ← quoted original    │
│ > Let me know if the time works.                                │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ Attachments: report.pdf, photo.jpg          [Attach more files] │
├─────────────────────────────────────────────────────────────────┤
│                         [Copy to Clipboard]                      │
└─────────────────────────────────────────────────────────────────┘
```

After clicking **[Copy to Clipboard]**, if attachments were uploaded, an
info panel appears:

```
┌─────────────────────────────────────────────────────────────────┐
│  ✓  Your email is in your clipboard. Attachments have been      │
│     uploaded to Blossom. Paste into a new DM to the bridge:     │
│                                                                  │
│     npub1<bridge-npub>              [Copy bridge npub]           │
└─────────────────────────────────────────────────────────────────┘
```

**Field behavior:**

| Field | Behavior |
|-------|----------|
| **From** | Always `<sender-npub>@<domain>`. Static, not editable. Populated from URL param. Display-only — not included in the clipboard output (bridge derives it from the event pubkey). |
| **To** | Pre-filled in reply mode. Editable input in fresh compose. |
| **Cc / Bcc** | Empty by default, shown as input fields. Space-separated email addresses (commas stripped as decoration, space is the delimiter). |
| **Subject** | Pre-filled with `Re:` prefix in reply mode. Editable in fresh compose. |
| **Body** | Reply mode: cursor at top, separator line, `>` prefixed quoted original below. User can interlinear reply (type between quoted lines) or top-post (new text above separator, quoted chain below). Fresh compose: empty. |
| **Attachments** | Multi-file picker button. Can be clicked multiple times to add files from different directories. Files accumulate in a list shown above the button. Each file encrypted client-side (ChaCha20-Poly1305 via WebCrypto), uploaded to Blossom (no auth), fragment-key URL added as `Attachment:` header in the formatted output (after Subject, before body). |

**All outbound email is plaintext only.** No HTML composition. Blossom URLs
(including `#key` fragments) appear as-is in the email body.

##### What [Copy to Clipboard] Produces

The form generates a correctly formatted plaintext message. No `From:` header
is included — the bridge derives the sender from the Nostr event's pubkey.
Attachment URLs appear in the header block after Subject, before the body:

```
To: alice@example.com
Cc: bob@example.com
Subject: Re: Meeting tomorrow
Attachment: https://blossom.example/abc123def456#9a8b7c6d5e4f
Attachment: https://blossom.example/789xyz012abc#1a2b3c4d5e6f

Sure, 3pm works for me.

---
> Hey, are we still on for 3pm?
> Let me know if the time works.
```

The user pastes this into a new Marmot DM addressed to the bridge npub.
The bridge parses it identically to the OLD §4.2 format.

##### Form Flow (step by step)

1. User opens compose link (from a received DM or subscription activation)
2. Form reads fragment parameters (`window.location.hash`) and pre-fills
   From, To, Subject, quoted body
3. User edits fields, types reply, optionally attaches files
4. User clicks **[Attach files]** one or more times to accumulate files
5. User clicks **[Copy to Clipboard]**
6. Form validates fields (non-empty To, Subject)
7. If attachments exist: JS encrypts each file (ChaCha20-Poly1305 via
   WebCrypto) → uploads ciphertext to Blossom (no auth) → adds
   `Attachment:` headers (after Subject, before body blank line)
8. Form copies formatted plaintext to clipboard
9. Info panel appears with confirmation + bridge npub
10. User opens Marmot client → new DM to bridge npub → paste → send

##### Bridge-Side Parsing (unchanged from OLD)

The bridge receives a Marmot DM and parses it as before:
- Headers (To, Cc, Subject, Attachment) terminated by first blank line
- Everything after the blank line is the email body
- From is derived from the Nostr event's pubkey (`<sender-npub>@<domain>`)
- `Attachment:` headers carry Blossom fragment-key URLs; the bridge includes
  these as-is in the email body (after user content)
- Validates sender subscription + whitelist + rate limit
- Sends plaintext SMTP email (DKIM signed)

##### Reply and Compose Links

All compose URLs use **fragment parameters** (`#key=value`) instead of query
parameters (`?key=value`). Fragments are never sent to the server in HTTP
requests, so the compose form parameters (recipient addresses, subjects,
quoted body text) never appear in server access logs, CDN logs, or reverse
proxy logs. The compose form JS reads them via `window.location.hash`. This
follows the same privacy pattern used for attachment encryption keys.

**Reply link (appended to every received DM):**

```
---
Reply: https://bridge.domain/compose#from=npub1xxx%40bridge.domain&to=alice%40example.com&subject=Re%3A%20Meeting&quote=...
```

The `from` param displays the sender address. The `quote` param carries the
original text (URL-encoded) so the form renders it with `>` prefixes. User
clicks → form opens pre-populated → type reply → Copy to Clipboard → paste
into DM.

**Fresh compose link (sent on subscription activation):**

```
Your email bridge is active: npub1xxx@bridge.domain

Compose new email: https://bridge.domain/compose#from=npub1xxx%40bridge.domain
```

---

### 4.3 Attachment Handling (Blossom + Fragment Key Encryption)

Attachments are encrypted before upload and the decryption key is carried in
the URL fragment, following the Mega.nz / PrivateBin pattern. The fragment
(`#key`) is never sent to the server in HTTP requests, so the Blossom server
stores ciphertext it cannot decrypt.

**Inbound (bridge-side, email → Nostr):**

1. Collect all non-plaintext MIME parts into a single zip (flat structure)
2. Generate a random 256-bit symmetric key
3. Encrypt zip with **ChaCha20-Poly1305** using the random key
4. Upload **ciphertext** to Blossom via `PUT /upload` with kind 24242 auth
5. Blossom returns URL: `https://blossom.example/<ciphertext-sha256>`
6. Append key as URL fragment: `https://blossom.example/<sha256>#<hex-key>`
7. Include full URL in the Marmot DM

**Outbound (client-side, compose form):**

1. User selects files via the compose form (can click [Attach] multiple
   times to add files from different directories)
2. On [Copy to Clipboard]: browser JS generates a random 256-bit key per file
3. JS encrypts each file with ChaCha20-Poly1305 (WebCrypto API)
4. JS uploads each ciphertext to Blossom (no auth gate)
5. JS appends fragment-key URLs to the formatted message
6. Fragment-key URLs appear as-is in the sent email

**Decryption (recipient-side):**

- Nostr user: Marmot client extracts fragment from URL, downloads ciphertext,
  decrypts. Or: use a standalone decryption tool.
- Email recipient: clicks the URL, decrypts via a static HTML page or CLI
  tool (documented in README)

**Notes:**

- The SHA-256 hash in the URL is of the **ciphertext**, not the plaintext —
  consistent with Blossom's content-addressable model
- Maximum: 25 MB per zip (inbound) or per file (outbound)
- No Blossom server-side changes needed — the fragment never reaches the
  server
- A future BUD spec could standardize this pattern for the broader ecosystem

### 4.4 Optional NIP-05 Verification

- Bridge serves `GET /.well-known/nostr.json?name=<npub-bech32>`
- Returns the standard NIP-05 response mapping the bech32 npub string to the
  hex pubkey
- This allows users to set their NIP-05 identifier to `<npub>@<domain>` and
  have it resolve correctly
- This is a low-effort addition but valuable for discoverability

---

## 5. Subscription & Payment

### 5.1 Flow

1. User sends a Marmot DM to the bridge npub with content: `subscribe`
2. Bridge generates a Lightning invoice via **NWC** (`make_invoice`) for the
   subscription amount
3. Bridge responds with a Marmot DM containing the BOLT-11 invoice
4. Upon payment confirmation (via `lookup_invoice` polling or NWC
   notification kind 23197), the bridge activates the subscription
5. Bridge confirms activation via Marmot DM containing:
   - The user's email address (`npub1xxx@bridge.domain`)
   - A compose link: `https://bridge.domain/compose#from=npub1xxx%40bridge.domain`

### 5.2 Subscription Regime Options

The bridge operator configures one of the following models:

| Model | Description | Suggested Price |
|-------|-------------|-----------------|
| **Monthly flat** | Unlimited send/receive for 30 days | 10,000–25,000 sats/month (~$4–10 USD) |
| **Per-message** | Pay per outbound email sent | 100–500 sats/email |
| **Quota pack** | Pre-pay for N outbound emails | 5,000 sats / 50 emails |
| **Annual flat** | Discounted yearly subscription | 80,000–200,000 sats/year |

**Anti-spam pricing rationale:**

- A spammer sending 10,000 emails at 100 sats/email would spend 1,000,000
  sats (~$400 USD). This is prohibitively expensive for spam at volume.
- Monthly flat rate at 10,000+ sats means even creating throwaway accounts
  costs real money.
- The bridge operator should configure pricing based on their IP reputation
  risk tolerance — higher prices = less abuse risk.
- **Recommendation:** Start with monthly flat at 15,000 sats ($6 USD). Low
  enough for legitimate users, expensive enough to deter bulk abuse.

### 5.3 Configuration

Bridge operator sets via environment variables or config file:
- `BRIDGE_SUBSCRIPTION_MODEL`: `monthly` | `per_message` | `quota` | `annual`
- `BRIDGE_SUBSCRIPTION_PRICE_SATS`: price in satoshis
- `BRIDGE_QUOTA_SIZE`: number of outbound emails per quota pack (if applicable)
- `BRIDGE_NWC_URI`: NWC connection string (restricted to `make_invoice` +
  `lookup_invoice` permissions only)

---

## 6. Rate Limiting

Outbound email rate limiting to protect IP reputation:

| Limit | Value | Rationale |
|-------|-------|-----------|
| Per-user per-hour | 10 emails | Prevents scripted bursts |
| Per-user per-day | 50 emails | Normal human usage ceiling |
| Global per-hour | 100 emails | Protects IP reputation |
| Global per-day | 500 emails | Protects IP reputation |
| Minimum interval | 30 seconds between sends | Prevents machine-gun sending |

These should be configurable by the bridge operator. The defaults above are
conservative — a new IP with no sending history needs to warm up gradually.

When a rate limit is hit, the bridge responds to the user via Marmot DM with
a clear message indicating when they can send again.

---

## 7. DNS Requirements (Documented in README)

For the bridge domain, the operator must configure:

```
; MX record — points to the server running the bridge
@       MX  10  mail.example.com.

; SPF — authorizes the bridge server to send email for this domain
@       TXT "v=spf1 ip4:<server-ip> -all"

; DKIM — bridge generates keypair, public key published in DNS
default._domainkey  TXT "v=DKIM1; k=rsa; p=<public-key>"

; DMARC — policy for handling authentication failures
_dmarc  TXT "v=DMARC1; p=reject; rua=mailto:postmaster@example.com"

; rDNS / PTR — server IP must reverse-resolve to the mail domain
; (configured at the VPS/hosting provider level)
```

The README must include step-by-step instructions for each record.

**Explicit non-goal:** Gmail/Google Workspace deliverability. Google's
requirements (bulk sender guidelines, FBL enrollment, etc.) are onerous and
change arbitrarily. The bridge prioritizes deliverability to non-Google mail
servers.

---

## 8. Deployment

### 8.1 Deployment Modes

The bridge communicates with the relay exclusively via standard Nostr
protocol (NIP-01 WebSocket) and with Blossom via HTTP. It has no
relay-specific dependencies — it works with any relay, not just ORLY.

**ORLY monolithic mode:** Bridge runs in-process with the relay. All
subsystems (SMTP server, Marmot client, NWC) start as goroutines within the
same process. Communication with the relay uses in-process channels (no
WebSocket hop). Identity shared from the database.

**ORLY split IPC mode:** `orly launcher` spawns `orly bridge` as a separate
subprocess (same self-exec pattern as `orly db` and `orly acl`). The bridge
connects to the relay via WebSocket. The launcher injects the relay identity
into the bridge's environment.

**Standalone mode (any relay):** The bridge runs as an independent process,
connecting to any relay via `ORLY_BRIDGE_RELAY_URL`. Uses its own identity
from `ORLY_BRIDGE_NSEC` or auto-generated file.

```
# ORLY monolithic
./orly                    # relay + bridge in one process

# ORLY split IPC (via launcher)
./orly launcher           # spawns: orly db, orly acl, orly bridge, orly relay
./orly bridge             # standalone bridge subprocess

# Standalone (any relay)
ORLY_BRIDGE_RELAY_URL=wss://other-relay.example.com ./orly bridge
```

### 8.2 Container

- Single Docker container (or Podman-compatible OCI image)
- Dockerfile in the repository
- `docker-compose.yml` with all required services

### 8.3 Environment Variables

```
ORLY_BRIDGE_ENABLED=true
ORLY_BRIDGE_DOMAIN=mail.example.com
ORLY_BRIDGE_RELAY_URL=wss://relay.example.com
ORLY_BRIDGE_BLOSSOM_URL=https://blossom.example.com
ORLY_BRIDGE_NWC_URI=nostr+walletconnect://<wallet-pubkey>?relay=...&secret=...
ORLY_BRIDGE_SUBSCRIPTION_MODEL=monthly
ORLY_BRIDGE_SUBSCRIPTION_PRICE_SATS=15000
ORLY_BRIDGE_SMTP_PORT=25
ORLY_BRIDGE_DKIM_PRIVATE_KEY_PATH=/path/to/dkim.key
ORLY_BRIDGE_OPERATOR_PUBKEY=<hex-pubkey>    # for contact forwarding (§3.3)
ORLY_BRIDGE_NSEC=                           # standalone mode only (§3.1)
```

**Bridge identity:**

The bridge uses the relay's keypair (see §3.1 for resolution order). In ORLY
modes, no identity configuration is needed — the bridge shares the relay's
key. In standalone mode, set `ORLY_BRIDGE_NSEC` or let the bridge
auto-generate and persist to `<data-dir>/bridge.nsec`.

### 8.4 README

Must include:
- Architecture overview (both deployment modes)
- DNS configuration (step-by-step with examples)
- Docker deployment instructions
- DKIM key generation
- NWC wallet setup (restricted permissions)
- Blossom server configuration
- Subscription model configuration
- Compose form usage
- Troubleshooting (SPF/DKIM/DMARC validation tools)

---

## 9. Technical Constraints

| Constraint | Value | Reason |
|------------|-------|--------|
| Max email body | 64 KB | NIP-44 plaintext limit (65,535 bytes) |
| DM protocol | Marmot (MLS) | Client requirement, not NIP-17 |
| Marmot implementation | Go (native, in `git.mleku.dev/mleku/nostr`) | No Rust FFI, no JS subprocess |
| Compose form | Static HTML, no auth, clipboard output | Works on all platforms, no signer needed |
| NWC permissions | `make_invoice` + `lookup_invoice` only | Security: never grant `pay_invoice` |
| Encryption | NIP-44 v2 (via Marmot) | No PGP |
| Attachment encryption | ChaCha20-Poly1305 | Fragment-key pattern (Mega.nz style) |
| Inbound attachments | Single zip (all non-plaintext parts) | One encryption, one upload |
| Outbound email format | Plaintext only | No HTML composition |
| Blossom upload (outbound) | No auth gate | Compose form is client-side only |
| HTML handling | Zip to Blossom attachment | No stripping or processing |
| Relay coupling | None (NIP-01 WebSocket only) | Works with any relay, ORLY integration optional |
| Bridge identity | Relay keypair (ORLY) or standalone file | NIP-11 pubkey = bridge pubkey |

---

## 10. Out of Scope

- PGP encryption
- Gmail/Google deliverability optimization
- Zap-based payment (NWC invoices only)
- Multiple email addresses per npub (one npub = one address per domain)
- Email threading / conversation tracking
- Spam filtering on inbound email (beyond basic SPF/DKIM validation)
- Calendar invites, read receipts, or other email extensions
- HTML email composition (outbound is plaintext only)

---

## 11. Deliverables

1. **Marmot Go package** — native Go Marmot protocol implementation in
   `git.mleku.dev/mleku/nostr` (MLS key packages, 1:1 groups, send/receive)
2. **Bridge source code** — Nostr-Email bridge (Go, relay-agnostic, with
   ORLY integration as `orly bridge` subcommand)
3. **Compose form** — Static HTML/JS email composition page (no auth, pure
   client-side, clipboard output)
4. **Dockerfile** + **docker-compose.yml** — containerized deployment
5. **README** — deployment instructions including DNS, DKIM, NWC, Blossom
   setup
6. **DKIM key generation tool/script**
7. **Decryption tool** — Static HTML page for email recipients to decrypt
   Blossom fragment-key attachments
8. **Working demo** — demonstrated send and receive on a test domain

---

## 12. Acceptance Criteria

- [ ] Inbound email to `npub@domain` arrives as Marmot DM with reply link
- [ ] Compose form generates correctly formatted DM, copies to clipboard
- [ ] Pasted DM to bridge sends email from `npub@domain`
- [ ] Cc/Bcc fields in compose form produce correct email headers
- [ ] Reply link opens compose form pre-populated with To, Subject, quoted body
- [ ] Non-plaintext email parts are zipped and uploaded to Blossom as one blob
- [ ] Attachments encrypted (ChaCha20-Poly1305) with fragment-key URLs
- [ ] Compose form attachment upload works (multiple rounds, no auth)
- [ ] Subscription payment flow works via NWC lightning invoice
- [ ] Rate limiting prevents burst sending
- [ ] SPF, DKIM, DMARC pass validation (tested against non-Google servers)
- [ ] NIP-05 verification resolves correctly (if enabled)
- [ ] Bridge runs as `orly bridge` subcommand (monolithic + split IPC modes)
- [ ] Bridge works standalone with a non-ORLY relay via `ORLY_BRIDGE_RELAY_URL`
- [ ] Bridge uses relay identity in ORLY modes (no separate keypair)
- [ ] Bridge auto-generates and persists identity in standalone mode
- [ ] Contact forwarding: non-email DMs proxied to operator without revealing sender
- [ ] Contact forwarding: operator replies proxied back without revealing operator
- [ ] Marmot Go package in `git.mleku.dev/mleku/nostr` passes tests
- [ ] Solution can be deployed to a VPS and configured by a regular IT administrator user from the README instructions without any specific nostr / email / orly related knowledge or understanding
- [ ] Email body respects 64 KB limit
