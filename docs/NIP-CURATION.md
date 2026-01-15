# NIP-XX: Relay Curation Mode

`draft` `optional`

This NIP defines a relay operating mode where operators can curate content through a three-tier publisher classification system (trusted, blacklisted, unclassified) with rate limiting, IP-based flood protection, and event kind filtering. Configuration and management are performed through Nostr events and a NIP-86 JSON-RPC API.

## Motivation

Public relays face challenges managing spam, abuse, and resource consumption. Traditional approaches (pay-to-relay, invite-only, WoT-based) each have limitations. Curation mode provides relay operators with fine-grained control over who can publish what, while maintaining an open-by-default stance that allows unknown users to participate within limits.

## Overview

Curation mode introduces:

1. **Publisher Classification**: Three-tier system (trusted, blacklisted, unclassified)
2. **Rate Limiting**: Per-pubkey and per-IP daily event limits
3. **Kind Filtering**: Configurable allowed event kinds
4. **Configuration Event**: Kind 30078 replaceable event for relay configuration
5. **Management API**: NIP-86 JSON-RPC endpoints for administration

## Configuration Event (Kind 30078)

The relay MUST be configured with a kind 30078 replaceable event before accepting events from non-owner/admin pubkeys. This event uses the `d` tag value `curating-config`.

### Event Structure

```json
{
  "kind": 30078,
  "tags": [
    ["d", "curating-config"],
    ["daily_limit", "<number>"],
    ["ip_daily_limit", "<number>"],
    ["first_ban_hours", "<number>"],
    ["second_ban_hours", "<number>"],
    ["kind_category", "<category_id>"],
    ["kind", "<kind_number>"],
    ["kind_range", "<start>-<end>"]
  ],
  "content": "{}",
  "pubkey": "<owner_or_admin_pubkey>",
  "created_at": <unix_timestamp>
}
```

### Configuration Tags

| Tag | Description | Default |
|-----|-------------|---------|
| `d` | MUST be `"curating-config"` | Required |
| `daily_limit` | Max events per day for unclassified users | 50 |
| `ip_daily_limit` | Max events per day from a single IP | 500 |
| `first_ban_hours` | First offense IP ban duration (hours) | 1 |
| `second_ban_hours` | Subsequent offense IP ban duration (hours) | 168 |
| `kind_category` | Predefined kind category (repeatable) | - |
| `kind` | Individual allowed kind number (repeatable) | - |
| `kind_range` | Allowed kind range as "start-end" (repeatable) | - |

### Kind Categories

Relays SHOULD support these predefined categories:

| Category ID | Kinds | Description |
|-------------|-------|-------------|
| `social` | 0, 1, 3, 6, 7, 10002 | Profiles, notes, contacts, reposts, reactions, relay lists |
| `dm` | 4, 14, 1059 | Direct messages (NIP-04, NIP-17, gift wraps) |
| `longform` | 30023, 30024 | Long-form articles and drafts |
| `media` | 1063, 20, 21, 22 | File metadata, picture, video, audio events |
| `lists` | 10000, 10001, 10003, 30000, 30001, 30003 | Mute lists, pins, bookmarks, people lists |
| `groups_nip29` | 9-12, 9000-9002, 39000-39002 | NIP-29 relay-based groups |
| `groups_nip72` | 34550, 1111, 4550 | NIP-72 moderated communities |
| `marketplace_nip15` | 30017-30020, 1021, 1022 | NIP-15 stalls and products |
| `marketplace_nip99` | 30402, 30403, 30405, 30406, 31555 | NIP-99 classified listings |
| `order_communication` | 16, 17 | Marketplace order messages |

Relays MAY define additional categories.

### Example Configuration Event

```json
{
  "kind": 30078,
  "tags": [
    ["d", "curating-config"],
    ["daily_limit", "100"],
    ["ip_daily_limit", "1000"],
    ["first_ban_hours", "2"],
    ["second_ban_hours", "336"],
    ["kind_category", "social"],
    ["kind_category", "dm"],
    ["kind", "1984"],
    ["kind_range", "30000-39999"]
  ],
  "content": "{}",
  "pubkey": "a1b2c3...",
  "created_at": 1700000000
}
```

## Publisher Classification

### Trusted Publishers

- Unlimited publishing rights
- Bypass rate limiting and IP flood protection
- Events visible to all users

### Blacklisted Publishers

- Cannot publish any events
- Events rejected with `"blocked: pubkey is blacklisted"` notice
- Existing events hidden from queries (visible only to admins/owners)

### Unclassified Publishers (Default)

- Subject to daily event limit
- Subject to IP flood protection
- Events visible to all users
- Can be promoted to trusted or demoted to blacklisted

## Event Processing Flow

When an event is received, the relay MUST process it as follows:

1. **Configuration Check**: Reject if relay is not configured (no kind 30078 event)
2. **Access Level Check**: Determine pubkey's access level
   - Owners and admins: always accept, bypass all limits
   - IP-blocked: reject with temporary block notice
   - Blacklisted: reject with blacklist notice
   - Trusted: accept, bypass rate limits
   - Unclassified: continue to rate limit checks
3. **Kind Filter**: Reject if event kind is not in allowed list
4. **Rate Limit Check**:
   - Check pubkey's daily event count against `daily_limit`
   - Check IP's daily event count against `ip_daily_limit`
5. **Accept or Reject**: Accept if all checks pass

### IP Flood Protection

When a pubkey exceeds `daily_limit`:

1. Record IP offense
2. If first offense: block IP for `first_ban_hours`
3. If subsequent offense: block IP for `second_ban_hours`
4. Track which pubkeys triggered the offense for admin review

## Management API (NIP-86)

All management endpoints require NIP-98 HTTP authentication from an owner or admin pubkey.

### Trust Management

| Method | Parameters | Description |
|--------|------------|-------------|
| `trustpubkey` | `[pubkey_hex, note?]` | Add pubkey to trusted list |
| `untrustpubkey` | `[pubkey_hex]` | Remove pubkey from trusted list |
| `listtrustedpubkeys` | `[]` | List all trusted pubkeys |

### Blacklist Management

| Method | Parameters | Description |
|--------|------------|-------------|
| `blacklistpubkey` | `[pubkey_hex, reason?]` | Add pubkey to blacklist |
| `unblacklistpubkey` | `[pubkey_hex]` | Remove pubkey from blacklist |
| `listblacklistedpubkeys` | `[]` | List all blacklisted pubkeys |

### User Inspection

| Method | Parameters | Description |
|--------|------------|-------------|
| `listunclassifiedusers` | `[limit?]` | List unclassified users sorted by event count |
| `geteventsforpubkey` | `[pubkey_hex, limit?, offset?]` | Get events from a pubkey |
| `deleteeventsforpubkey` | `[pubkey_hex]` | Delete all events from a blacklisted pubkey |
| `scanpubkeys` | `[]` | Scan database to populate unclassified users list |

### Spam Management

| Method | Parameters | Description |
|--------|------------|-------------|
| `markspam` | `[event_id_hex, pubkey?, reason?]` | Flag event as spam (hides from queries) |
| `unmarkspam` | `[event_id_hex]` | Remove spam flag |
| `listspamevents` | `[]` | List spam-flagged events |
| `deleteevent` | `[event_id_hex]` | Permanently delete an event |

### IP Management

| Method | Parameters | Description |
|--------|------------|-------------|
| `listblockedips` | `[]` | List currently blocked IPs |
| `unblockip` | `[ip_address]` | Remove IP block |

### Configuration

| Method | Parameters | Description |
|--------|------------|-------------|
| `getcuratingconfig` | `[]` | Get current configuration |
| `isconfigured` | `[]` | Check if relay is configured |
| `supportedmethods` | `[]` | List available management methods |

### Example API Request

```http
POST /api HTTP/1.1
Host: relay.example.com
Authorization: Nostr <base64_nip98_event>
Content-Type: application/json

{
  "method": "trustpubkey",
  "params": ["a1b2c3d4...", "Trusted friend"]
}
```

### Example API Response

```json
{
  "result": {
    "success": true,
    "message": "Pubkey added to trusted list"
  }
}
```

## Event Visibility

| Viewer | Sees Trusted Events | Sees Blacklisted Events | Sees Spam-Flagged Events |
|--------|---------------------|-------------------------|--------------------------|
| Owner/Admin | Yes | Yes | Yes |
| Regular User | Yes | No | No |

## Relay Information Document

Relays implementing this NIP SHOULD advertise it in their NIP-11 relay information document:

```json
{
  "supported_nips": [11, 86, "XX"],
  "limitation": {
    "curation_mode": true,
    "daily_limit": 50,
    "ip_daily_limit": 500
  }
}
```

## Implementation Notes

### Rate Limit Reset

Daily counters SHOULD reset at UTC midnight (00:00:00 UTC).

### Caching

Implementations SHOULD cache trusted/blacklisted status and allowed kinds in memory for performance, refreshing periodically (e.g., hourly).

### Database Keys

Suggested key prefixes for persistent storage:

- `CURATING_ACL_CONFIG` - Current configuration
- `CURATING_ACL_TRUSTED_PUBKEY_{pubkey}` - Trusted publishers
- `CURATING_ACL_BLACKLISTED_PUBKEY_{pubkey}` - Blacklisted publishers
- `CURATING_ACL_EVENT_COUNT_{pubkey}_{date}` - Daily event counts
- `CURATING_ACL_IP_EVENT_COUNT_{ip}_{date}` - IP daily event counts
- `CURATING_ACL_IP_OFFENSE_{ip}` - IP offense tracking
- `CURATING_ACL_BLOCKED_IP_{ip}` - Active IP blocks
- `CURATING_ACL_SPAM_EVENT_{event_id}` - Spam-flagged events

## Security Considerations

1. **NIP-98 Authentication**: All management API calls MUST require valid NIP-98 authentication from owner or admin pubkeys
2. **IP Spoofing**: Relays SHOULD use `X-Forwarded-For` or `X-Real-IP` headers carefully, only trusting them from known reverse proxies
3. **Rate Limit Bypass**: Trusted status should be granted carefully as it bypasses all rate limiting
4. **Event Deletion**: Deleted events cannot be recovered; implementations SHOULD consider soft-delete with admin recovery option

## Compatibility

This NIP is compatible with:
- NIP-42 (Authentication): Can require auth before accepting events
- NIP-86 (Relay Management API): Uses NIP-86 for management endpoints
- NIP-98 (HTTP Auth): Uses NIP-98 for API authentication

## Reference Implementation

- ORLY Relay: https://github.com/mleku/orly

## Changelog

- Initial draft
