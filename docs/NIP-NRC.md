# NIP-XX: Nostr Relay Connect (NRC)

`draft` `optional`

## Abstract

This NIP defines a protocol for exposing a private Nostr relay through a public relay, enabling access to relays behind NAT, firewalls, or on devices without public IP addresses. It uses end-to-end encrypted events to tunnel standard Nostr protocol messages through a rendezvous relay.

## Motivation

Users want to run personal relays for:
- Private data synchronization across devices
- Full control over event storage
- Offline-first applications with sync capability

However, personal relays often run:
- Behind NAT without public IP addresses
- On mobile devices
- On home servers without port forwarding capability

NRC solves this by tunneling Nostr protocol messages through encrypted events on a public relay, similar to how [NIP-47](https://github.com/nostr-protocol/nips/blob/master/47.md) tunnels wallet operations.

## Specification

### Event Kinds

| Kind  | Name         | Description                              |
|-------|--------------|------------------------------------------|
| 24891 | NRC Request  | Ephemeral, client→relay wrapped message  |
| 24892 | NRC Response | Ephemeral, relay→client wrapped message  |

### Connection URI

The connection URI format is:

```
nostr+relayconnect://<relay-pubkey>?relay=<rendezvous-relay>&secret=<client-secret>[&name=<device-name>]
```

Parameters:
- `relay-pubkey`: The public key of the private relay (64-char hex)
- `relay`: The WebSocket URL of the rendezvous relay (URL-encoded)
- `secret`: A 32-byte hex-encoded secret used to derive the conversation key
- `name` (optional): Human-readable device identifier for management

Example:
```
nostr+relayconnect://a1b2c3d4e5f6...?relay=wss%3A%2F%2Frelay.example.com&secret=0123456789abcdef...&name=phone
```

### Alternative: CAT Token Authentication

For privacy-preserving access, NRC supports Cashu Access Tokens (CAT) instead of static secrets:

```
nostr+relayconnect://<relay-pubkey>?relay=<rendezvous-relay>&auth=cat&mint=<mint-url>
```

When using CAT authentication:
1. Client obtains a CAT token from the mint with scope `nrc`
2. Client includes the token in request events using a `cashu` tag
3. Bridge verifies the token and re-authorizes via ACL on each request

### Message Flow

```
┌─────────┐     ┌─────────────┐     ┌─────────┐     ┌─────────────┐
│ Client  │────▶│ Public Relay│────▶│ Bridge  │────▶│Private Relay│
│         │◀────│ (rendezvous)│◀────│         │◀────│             │
└─────────┘     └─────────────┘     └─────────┘     └─────────────┘
     │                                    │
     └────────── NIP-44 encrypted ────────┘
```

1. **Client** wraps Nostr messages in kind 24891 events, encrypts content with NIP-44
2. **Public relay** forwards events based on `p` tags (cannot decrypt content)
3. **Bridge** (running alongside private relay) decrypts and forwards to local relay
4. **Private relay** processes the message normally
5. **Bridge** wraps response in kind 24892, encrypts, and publishes
6. **Client** receives kind 24892 events and decrypts the response

### Request Event (Kind 24891)

```json
{
  "kind": 24891,
  "content": "<nip44_encrypted_json>",
  "tags": [
    ["p", "<relay_pubkey>"],
    ["encryption", "nip44_v2"],
    ["session", "<session_id>"]
  ],
  "pubkey": "<client_pubkey>",
  "created_at": <unix_timestamp>,
  "sig": "<signature>"
}
```

With CAT authentication, add:
```json
["cashu", "cashuA..."]
```

The encrypted content structure:
```json
{
  "type": "EVENT" | "REQ" | "CLOSE" | "AUTH" | "COUNT",
  "payload": <standard_nostr_message_array>
}
```

Where `payload` is the standard Nostr message array, e.g.:
- `["EVENT", <event_object>]`
- `["REQ", "<sub_id>", <filter1>, <filter2>, ...]`
- `["CLOSE", "<sub_id>"]`
- `["AUTH", <auth_event>]`
- `["COUNT", "<sub_id>", <filter1>, ...]`

### Response Event (Kind 24892)

```json
{
  "kind": 24892,
  "content": "<nip44_encrypted_json>",
  "tags": [
    ["p", "<client_pubkey>"],
    ["encryption", "nip44_v2"],
    ["session", "<session_id>"],
    ["e", "<request_event_id>"]
  ],
  "pubkey": "<relay_pubkey>",
  "created_at": <unix_timestamp>,
  "sig": "<signature>"
}
```

The encrypted content structure:
```json
{
  "type": "EVENT" | "OK" | "EOSE" | "NOTICE" | "CLOSED" | "COUNT" | "AUTH",
  "payload": <standard_nostr_response_array>
}
```

Where `payload` is the standard Nostr response array, e.g.:
- `["EVENT", "<sub_id>", <event_object>]`
- `["OK", "<event_id>", <success_bool>, "<message>"]`
- `["EOSE", "<sub_id>"]`
- `["NOTICE", "<message>"]`
- `["CLOSED", "<sub_id>", "<message>"]`
- `["COUNT", "<sub_id>", {"count": <n>}]`
- `["AUTH", "<challenge>"]`

### Session Management

The `session` tag groups related request/response events, enabling:
- Multiple concurrent subscriptions through a single tunnel
- Correlation of responses to requests
- Session state tracking on the bridge

Session IDs SHOULD be randomly generated UUIDs or 32-byte hex strings.

### Encryption

All content is encrypted using [NIP-44](https://github.com/nostr-protocol/nips/blob/master/44.md) v2.

The conversation key is derived from:
- **Secret-based auth**: ECDH between client's secret key (derived from URI secret) and relay's public key
- **CAT auth**: ECDH between client's Nostr key and relay's public key

### Authentication

#### Secret-Based Authentication

1. Client derives a keypair from the `secret` parameter in the URI
2. Client signs all request events with this derived key
3. Bridge verifies the client's pubkey is in its authorized list
4. Conversation key provides implicit authentication (only authorized clients can decrypt responses)

#### CAT Token Authentication

1. Client obtains a CAT token from the relay's mint with scope `nrc`
2. Token is bound to client's Nostr pubkey
3. Client includes token in the `cashu` tag of request events
4. Bridge verifies token signature and scope
5. Bridge re-authorizes via ACL on each request (enables immediate revocation)

### Access Revocation

**Secret-based**: Remove the client's derived pubkey from the authorized list.

**CAT-based**: Remove the client's Nostr pubkey from the ACL. Takes effect immediately due to re-authorization on each request.

## Security Considerations

1. **End-to-end encryption**: The rendezvous relay cannot read tunneled messages
2. **Perfect forward secrecy**: Not provided; if secret is compromised, past messages can be decrypted
3. **Rate limiting**: Bridges SHOULD enforce rate limits to prevent abuse
4. **Session expiry**: Sessions SHOULD timeout after a period of inactivity
5. **TLS**: The rendezvous relay connection SHOULD use TLS (wss://)
6. **Secret storage**: Clients SHOULD store connection URIs securely (they contain secrets)

## Client Implementation Notes

1. Generate a random session ID on connection
2. Subscribe to kind 24892 events with `#p` filter for client's pubkey
3. For each outgoing message, wrap in kind 24891 and publish
4. Match responses using the `e` tag (references request event ID)
5. Handle EOSE by waiting for kind 24892 with type "EOSE" in content
6. For subscriptions, maintain mapping of internal sub IDs to tunnel session

## Bridge Implementation Notes

1. Subscribe to kind 24891 events with `#p` filter for relay's pubkey
2. Verify client authorization (secret-based or CAT)
3. Decrypt content and forward to local relay via internal WebSocket
4. Capture all relay responses and wrap in kind 24892
5. Sign with relay's key and publish to rendezvous relay
6. Maintain session state for subscription mapping

## Reference Implementations

- ORLY Relay: [https://git.mleku.dev/mleku/next.orly.dev](https://git.mleku.dev/mleku/next.orly.dev)

## See Also

- [NIP-44: Encrypted Payloads](https://github.com/nostr-protocol/nips/blob/master/44.md)
- [NIP-47: Nostr Wallet Connect](https://github.com/nostr-protocol/nips/blob/master/47.md)
- [NIP-46: Nostr Remote Signing](https://github.com/nostr-protocol/nips/blob/master/46.md)
