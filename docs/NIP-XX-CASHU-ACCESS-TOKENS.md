# NIP-XX: Cashu Access Tokens for Relay Authorization

`draft` `optional`

This NIP defines a protocol for relays to issue privacy-preserving access tokens using Cashu blind signatures. Tokens prove relay membership without linking issuance to usage, enabling spam protection while preserving user privacy.

## Motivation

Relays need to control access to prevent spam and abuse. Current approaches (NIP-42, NIP-98) require per-request authentication that links all user activity. Cashu blind signatures allow relays to issue bearer tokens that prove authorization without revealing which specific user is connecting.

This is particularly useful for:
- NIP-46 remote signing (bunker) access control
- Premium relay tiers
- Rate limit bypass for trusted users
- Any service requiring proof of relay membership

## Overview

1. Relay operates as a Cashu mint for its authorized users
2. Users authenticate via NIP-98 to obtain blinded signatures
3. Tokens specify permitted event kinds and expiry
4. Two-token rotation allows seamless renewal before expiry
5. Tokens are bearer credentials passed in HTTP/WebSocket headers

## Token Format

### Token Structure

```json
{
  "k": "<keyset_id>",
  "s": "<secret_hex>",
  "c": "<signature_hex>",
  "p": "<pubkey_hex>",
  "e": <expiry_unix>,
  "kinds": [0, 1, 3, 10002],
  "kind_ranges": [[20000, 29999]],
  "scope": "relay"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `k` | string | Keyset ID (hex) identifying the signing key |
| `s` | string | 32-byte random secret (hex) |
| `c` | string | 33-byte compressed signature point (hex) |
| `p` | string | 32-byte user pubkey (hex) |
| `e` | number | Unix timestamp when token expires |
| `kinds` | number[] | Explicit list of permitted event kinds |
| `kind_ranges` | number[][] | Ranges of permitted kinds as [min, max] pairs |
| `scope` | string | Token scope: "relay", "nip46", "blossom", or custom |

### Kind Permissions

Tokens specify which event kinds the bearer may publish:

- `kinds`: Explicit list of individual kinds (e.g., `[0, 1, 3]`)
- `kind_ranges`: Inclusive ranges (e.g., `[[20000, 29999]]` for ephemeral events)

A token permits a kind if it appears in `kinds` OR falls within any `kind_ranges` entry.

Special values:
- Empty `kinds` and `kind_ranges`: No write access (read-only token)
- `kinds: [-1]`: All kinds permitted (wildcard)

### Scopes

| Scope | Description |
|-------|-------------|
| `relay` | Standard relay WebSocket access (REQ, EVENT, COUNT) |
| `nip46` | NIP-46 remote signing / bunker access |
| `blossom` | Blossom media server access |
| `api` | HTTP API access |

Custom scopes may be defined by applications.

### Serialization

Tokens are serialized as:
```
cashuA<base64url(json)>
```

The `cashuA` prefix indicates version 1 of this specification.

## Keyset Management

### Keyset Structure

Relays maintain signing keysets that rotate periodically:

```json
{
  "id": "<keyset_id>",
  "pubkey": "<compressed_pubkey_hex>",
  "active": true,
  "created_at": 1735300000,
  "expires_at": 1736510000
}
```

### Keyset ID Calculation

```
keyset_id = SHA256(compressed_pubkey)[0:7] as hex (14 characters)
```

### Rotation Policy

- **Active period**: 1 week (tokens can be issued)
- **Verification period**: 3 weeks (tokens can still be validated)
- **Total validity**: Active keyset + 2 previous keysets accepted

This ensures tokens issued at the end of an active period remain valid for their full lifetime.

## Two-Token Rotation

Users may hold up to two tokens:

| Token | State | Purpose |
|-------|-------|---------|
| Active | In use | Current authentication credential |
| Pending | Awaiting | Pre-fetched for seamless rotation |

### Rotation Flow

1. User obtains initial token (becomes Active)
2. When Active token reaches 50% lifetime, user requests new token (becomes Pending)
3. When Active token expires, Pending becomes Active
4. User requests new Pending token
5. Repeat

This ensures continuous access without authentication gaps.

### Blacklist Behavior

If a user is removed from the relay's whitelist:
- Active token continues working until expiry (max 1 week)
- Pending token continues working until expiry (max 1 week)
- **Maximum access after blacklist: 2 weeks**

New token requests will fail immediately upon blacklist.

## HTTP Endpoints

### Token Issuance

```http
POST /cashu/mint
Authorization: Nostr <base64_nip98_event>
Content-Type: application/json

{
  "blinded_message": "<B_hex>",
  "scope": "relay",
  "kinds": [0, 1, 3, 7],
  "kind_ranges": [[30000, 39999]]
}
```

**Response:**
```json
{
  "blinded_signature": "<C_hex>",
  "keyset_id": "<keyset_id>",
  "expiry": 1736294400,
  "pubkey": "<mint_pubkey_hex>"
}
```

The user must:
1. Generate random secret `x` and blinding factor `r`
2. Compute `Y = hash_to_curve(x)`
3. Compute `B_ = Y + r*G`
4. Send `B_` as `blinded_message`
5. Receive `C_` as `blinded_signature`
6. Compute `C = C_ - r*K` (K is mint pubkey)
7. Token is `(x, C)` with metadata

### Keyset Discovery

```http
GET /cashu/keysets

Response:
{
  "keysets": [
    {
      "id": "0a1b2c3d4e5f67",
      "pubkey": "02...",
      "active": true,
      "expires_at": 1736510000
    }
  ]
}
```

### Token Info (Optional)

```http
GET /cashu/info

Response:
{
  "name": "Relay Name",
  "version": "NIP-XX/1",
  "token_ttl": 604800,
  "max_kinds": 100,
  "supported_scopes": ["relay", "nip46", "blossom"]
}
```

## Authentication Headers

### WebSocket Upgrade

```http
GET / HTTP/1.1
Upgrade: websocket
X-Cashu-Token: cashuA<base64url>
```

### HTTP Requests

```http
GET /api/resource HTTP/1.1
Authorization: Cashu cashuA<base64url>
```

Or as dedicated header:
```http
X-Cashu-Token: cashuA<base64url>
```

### NIP-46 Integration

For NIP-46 bunker connections, the token is passed in the WebSocket upgrade:

```http
GET /nip46 HTTP/1.1
Upgrade: websocket
X-Cashu-Token: cashuA<base64url>
```

The bunker verifies:
1. Token signature is valid
2. Token has not expired
3. Token scope is "nip46"
4. User pubkey in token matches NIP-46 connect pubkey

## Cryptographic Details

### Blind Diffie-Hellman Key Exchange (BDHKE)

Uses secp256k1 curve with Cashu's hash-to-curve:

```
hash_to_curve(message):
  msg_hash = SHA256("Secp256k1_HashToCurve_Cashu_" || message)
  for counter in 0..65536:
    hash = SHA256(msg_hash || counter_le32)
    point = try_parse("02" || hash)
    if point is valid:
      return point
  fail
```

**Blinding:**
```
Y = hash_to_curve(secret)
r = random_scalar()
B_ = Y + r*G
```

**Signing (mint):**
```
C_ = k * B_
```

**Unblinding (user):**
```
C = C_ - r*K
```

**Verification (mint):**
```
valid = (C == k * hash_to_curve(secret))
```

## Verification Flow

When relay receives a token:

1. Parse token from header
2. Find keyset by ID (must be active or recently expired)
3. Verify: `C == k * hash_to_curve(secret)`
4. Check: `expiry > now`
5. Check: scope matches service
6. Check: requested kind in `kinds` or `kind_ranges`
7. **Optional**: Re-check user pubkey against current ACL

Step 7 provides "stateless revocation" - tokens become invalid immediately when user is removed from ACL, not just when they expire.

## Security Considerations

### Token as Bearer Credential

Tokens are bearer credentials. Compromise allows impersonation until expiry. Mitigations:
- Short TTL (1 week recommended)
- TLS for all transport
- Secure client storage

### Privacy Properties

- **Unlinkability**: Relay cannot link token issuance to token use
- **No tracking**: Different secrets prevent correlation across tokens
- **Pubkey binding**: Token is bound to user's Nostr pubkey

### Keyset Compromise

If keyset private key is compromised:
- Rotate immediately (new keyset)
- Old keyset enters verification-only mode
- Tokens issued by compromised keyset expire naturally (max 3 weeks)

### Replay Prevention

- Tokens have expiry timestamps
- Optional: Relay tracks used secrets (adds state, breaks unlinkability)
- Scope prevents cross-service replay

## Example Flow

```
1. Alice wants NIP-46 bunker access to relay.example.com

2. Alice authenticates via NIP-98:
   POST /cashu/mint
   Authorization: Nostr <signed_kind_27235>
   {"blinded_message": "02abc...", "scope": "nip46"}

3. Relay checks:
   - NIP-98 signature valid
   - Alice's pubkey in whitelist with write access

4. Relay responds:
   {"blinded_signature": "03def...", "keyset_id": "a1b2c3...", "expiry": 1736294400}

5. Alice unblinds signature, constructs token

6. Alice connects to bunker:
   GET /nip46 HTTP/1.1
   Upgrade: websocket
   X-Cashu-Token: cashuA<token>

7. Bunker verifies token, establishes NIP-46 session

8. One week later, Alice's token approaches expiry
   - Alice requests new token (step 2-5)
   - New token becomes Pending
   - When Active expires, Pending becomes Active
```

## Relay Implementation Notes

### Recommended Defaults

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Token TTL | 7 days | Balance between convenience and revocation speed |
| Keyset rotation | Weekly | Limits key exposure |
| Verification keysets | 3 | Covers full token lifetime + grace period |
| Re-check ACL | On every use | Enables immediate revocation |

### Error Codes

| Code | Meaning |
|------|---------|
| 401 | Missing or malformed token |
| 403 | Valid token but insufficient permissions (wrong scope/kinds) |
| 410 | Token expired |
| 421 | Unknown keyset ID |

## References

- [Cashu Protocol](https://github.com/cashubtc/nuts)
- [NIP-42: Authentication of clients to relays](https://github.com/nostr-protocol/nips/blob/master/42.md)
- [NIP-46: Nostr Remote Signing](https://github.com/nostr-protocol/nips/blob/master/46.md)
- [NIP-98: HTTP Auth](https://github.com/nostr-protocol/nips/blob/master/98.md)
- [Blind Signatures for Untraceable Payments](http://www.hit.bme.hu/~buttyan/courses/BMEVIHIM219/2009/Chaum.BlindSigForPayworx.1662.pdf)
