# Curation Mode Guide

Curation mode is a sophisticated access control system for Nostr relays that provides three-tier publisher classification, rate limiting, IP-based flood protection, and event kind whitelisting.

## Overview

Unlike simple allow/deny lists, curation mode classifies publishers into three tiers:

| Tier | Rate Limited | Daily Limit | Visibility |
|------|--------------|-------------|------------|
| **Trusted** | No | Unlimited | Full |
| **Blacklisted** | N/A (blocked) | 0 | Hidden from regular users |
| **Unclassified** | Yes | 50 events/day (default) | Full |

This allows relay operators to:
- Reward quality contributors with unlimited publishing
- Block bad actors while preserving their events for admin review
- Allow new users to participate with reasonable rate limits
- Prevent spam floods through automatic IP-based protections

## Quick Start

### 1. Start the Relay

```bash
export ORLY_ACL_MODE=curating
export ORLY_OWNERS=npub1your_owner_pubkey
./orly
```

### 2. Publish Configuration

The relay will not accept events until you publish a configuration event. Use the web UI at `http://your-relay/#curation` or publish a kind 30078 event:

```json
{
  "kind": 30078,
  "tags": [["d", "curating-config"]],
  "content": "{\"dailyLimit\":50,\"ipDailyLimit\":500,\"firstBanHours\":1,\"secondBanHours\":168,\"kindCategories\":[\"social\"]}"
}
```

### 3. Manage Publishers

Use the web UI or NIP-86 API to:
- Trust quality publishers
- Blacklist spammers
- Review unclassified users by activity
- Unblock IPs if needed

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_ACL_MODE` | `none` | Set to `curating` to enable |
| `ORLY_OWNERS` | | Owner pubkeys (can configure relay) |
| `ORLY_ADMINS` | | Admin pubkeys (can manage publishers) |

### Configuration Event (Kind 30078)

Configuration is stored as a replaceable Nostr event (kind 30078) with d-tag `curating-config`. Only owners and admins can publish configuration.

```typescript
interface CuratingConfig {
  // Rate Limiting
  dailyLimit: number;      // Max events/day for unclassified users (default: 50)
  ipDailyLimit: number;    // Max events/day from single IP (default: 500)

  // IP Ban Durations
  firstBanHours: number;   // First offense ban duration (default: 1 hour)
  secondBanHours: number;  // Subsequent offense ban duration (default: 168 hours / 1 week)

  // Kind Filtering (choose one or combine)
  allowedKinds: number[];     // Explicit kind numbers: [0, 1, 3, 7]
  allowedRanges: string[];    // Kind ranges: ["1000-1999", "30000-39999"]
  kindCategories: string[];   // Pre-defined categories: ["social", "dm"]
}
```

### Event Kind Categories

Pre-defined categories for convenient kind whitelisting:

| Category | Kinds | Description |
|----------|-------|-------------|
| `social` | 0, 1, 3, 6, 7, 10002 | Profiles, notes, contacts, reposts, reactions |
| `dm` | 4, 14, 1059 | Direct messages (NIP-04, NIP-17, gift wraps) |
| `longform` | 30023, 30024 | Long-form articles and drafts |
| `media` | 1063, 20, 21, 22 | File metadata, picture/video/audio events |
| `marketplace` | 30017-30020, 1021, 1022 | Products, stalls, auctions, bids |
| `groups_nip29` | 9-12, 9000-9002, 39000-39002 | NIP-29 relay-based groups |
| `groups_nip72` | 34550, 1111, 4550 | NIP-72 moderated communities |
| `lists` | 10000, 10001, 10003, 30000, 30001, 30003 | Mute, pin, bookmark lists |

Example configuration allowing social interactions and DMs:

```json
{
  "kindCategories": ["social", "dm"],
  "dailyLimit": 100,
  "ipDailyLimit": 1000
}
```

## Three-Tier Classification

### Trusted Publishers

Trusted publishers have unlimited publishing rights:
- Bypass all rate limiting
- Can publish any allowed kind
- Events always visible to all users

**Use case**: Known quality contributors, verified community members, partner relays.

### Blacklisted Publishers

Blacklisted publishers are blocked from publishing:
- All events rejected with `"pubkey is blacklisted"` error
- Existing events become invisible to regular users
- Admins and owners can still see blacklisted events (for review)

**Use case**: Spammers, abusive users, bad actors.

### Unclassified Publishers

Everyone else falls into the unclassified tier:
- Subject to daily event limit (default: 50 events/day)
- Subject to IP-based flood protection
- Events visible to all users
- Can be promoted to trusted or demoted to blacklisted

**Use case**: New users, general public.

## Rate Limiting & Flood Protection

### Per-Pubkey Limits

Unclassified publishers are limited to a configurable number of events per day (default: 50). The count resets at midnight UTC.

When a user exceeds their limit:
1. Event is rejected with `"daily event limit exceeded"` error
2. Their IP is flagged for potential abuse

### Per-IP Limits

To prevent Sybil attacks (creating many pubkeys from one IP), there's also an IP-based daily limit (default: 500 events).

When an IP exceeds its limit:
1. All events from that IP are rejected
2. The IP is temporarily banned

### Automatic IP Banning

When rate limits are exceeded:

| Offense | Ban Duration | Description |
|---------|--------------|-------------|
| First | 1 hour | Quick timeout for accidental over-posting |
| Second+ | 1 week | Extended ban for repeated abuse |

Ban durations are configurable via `firstBanHours` and `secondBanHours`.

### Offense Tracking

The system tracks which pubkeys triggered rate limits from each IP:

```
IP 192.168.1.100:
  - npub1abc... exceeded limit at 2024-01-15 10:30:00
  - npub1xyz... exceeded limit at 2024-01-15 10:45:00
  Offense count: 2
  Status: Banned until 2024-01-22 10:45:00
```

This helps identify coordinated spam attacks.

## Spam Flagging

Events can be flagged as spam without deletion:

- Flagged events are hidden from regular users
- Admins can review flagged events
- Events can be unflagged if incorrectly marked
- Original event data is preserved

This is useful for:
- Moderation review queues
- Training spam detection systems
- Preserving evidence of abuse

## NIP-86 Management API

All management operations use NIP-98 HTTP authentication.

### Trust Management

```bash
# Trust a pubkey
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"trustpubkey","params":["<pubkey_hex>"]}'

# Untrust a pubkey
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"untrustpubkey","params":["<pubkey_hex>"]}'

# List trusted pubkeys
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"listtrustedpubkeys","params":[]}'
```

### Blacklist Management

```bash
# Blacklist a pubkey
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"blacklistpubkey","params":["<pubkey_hex>"]}'

# Remove from blacklist
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"unblacklistpubkey","params":["<pubkey_hex>"]}'

# List blacklisted pubkeys
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"listblacklistedpubkeys","params":[]}'
```

### Unclassified User Management

```bash
# List unclassified users sorted by event count
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"listunclassifiedusers","params":[]}'
```

Response includes pubkey, event count, and last activity for each user.

### Spam Management

```bash
# Mark event as spam
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"markspam","params":["<event_id_hex>"]}'

# Unmark spam
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"unmarkspam","params":["<event_id_hex>"]}'

# List spam events
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"listspamevents","params":[]}'
```

### IP Block Management

```bash
# List blocked IPs
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"listblockedips","params":[]}'

# Unblock an IP
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"unblockip","params":["<ip_address>"]}'
```

### Configuration Management

```bash
# Get current configuration
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"getcuratingconfig","params":[]}'

# Set allowed kind categories
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"setallowedkindcategories","params":[["social","dm","longform"]]}'

# Get allowed kind categories
curl -X POST https://relay.example.com \
  -H "Authorization: Nostr <nip98_token>" \
  -d '{"method":"getallowedkindcategories","params":[]}'
```

## Web UI

The curation web UI is available at `/#curation` and provides:

- **Configuration Panel**: Set rate limits, ban durations, and allowed kinds
- **Publisher Management**: Trust/blacklist pubkeys with one click
- **Unclassified Users**: View users sorted by activity, promote or blacklist
- **IP Blocks**: View and unblock banned IP addresses
- **Spam Queue**: Review flagged events, confirm or unflag

## Database Storage

Curation data is stored in the relay database with the following key prefixes:

| Prefix | Purpose |
|--------|---------|
| `CURATING_ACL_CONFIG` | Current configuration |
| `CURATING_ACL_TRUSTED_PUBKEY_{pubkey}` | Trusted publisher list |
| `CURATING_ACL_BLACKLISTED_PUBKEY_{pubkey}` | Blacklisted publisher list |
| `CURATING_ACL_EVENT_COUNT_{pubkey}_{date}` | Daily event counts per pubkey |
| `CURATING_ACL_IP_EVENT_COUNT_{ip}_{date}` | Daily event counts per IP |
| `CURATING_ACL_IP_OFFENSE_{ip}` | Offense tracking per IP |
| `CURATING_ACL_BLOCKED_IP_{ip}` | Active IP blocks |
| `CURATING_ACL_SPAM_EVENT_{eventID}` | Spam-flagged events |

## Caching

For performance, the following data is cached in memory:

- Trusted pubkey set
- Blacklisted pubkey set
- Allowed kinds set
- Current configuration

Caches are refreshed every hour by the background cleanup goroutine.

## Background Maintenance

A background goroutine runs hourly to:

1. Remove expired IP blocks
2. Clean up old event count entries (older than 2 days)
3. Refresh in-memory caches
4. Log maintenance statistics

## Best Practices

### Starting a New Curated Relay

1. Start with permissive settings:
   ```json
   {"dailyLimit": 100, "ipDailyLimit": 1000, "kindCategories": ["social"]}
   ```

2. Monitor unclassified users for a few days

3. Trust active, quality contributors

4. Blacklist obvious spammers

5. Adjust rate limits based on observed patterns

### Handling Spam Waves

During spam attacks:

1. The IP-based flood protection will auto-ban attack sources
2. Review blocked IPs via web UI or API
3. Blacklist any pubkeys that got through
4. Consider temporarily lowering `ipDailyLimit`

### Recovering from Mistakes

- **Accidentally blacklisted someone**: Use `unblacklistpubkey` - their events become visible again
- **Wrongly flagged spam**: Use `unmarkspam` - event becomes visible again
- **Blocked legitimate IP**: Use `unblockip` - IP can publish again immediately

## Comparison with Other ACL Modes

| Feature | None | Follows | Managed | Curating |
|---------|------|---------|---------|----------|
| Default Access | Write | Write if followed | Explicit allow | Rate-limited |
| Rate Limiting | No | No | No | Yes |
| Kind Filtering | No | No | Optional | Yes |
| IP Protection | No | No | No | Yes |
| Spam Flagging | No | No | No | Yes |
| Configuration | Env vars | Follow lists | NIP-86 | Kind 30078 events |
| Web UI | Basic | Basic | Basic | Full curation panel |

## Troubleshooting

### "Relay not accepting events"

The relay requires a configuration event before accepting any events. Publish a kind 30078 event with d-tag `curating-config`.

### "daily event limit exceeded"

The user has exceeded their daily limit. Options:
1. Wait until midnight UTC for reset
2. Trust the pubkey if they're a quality contributor
3. Increase `dailyLimit` in configuration

### "pubkey is blacklisted"

The pubkey is on the blacklist. Use `unblacklistpubkey` if this was a mistake.

### "IP is blocked"

The IP has been auto-banned due to rate limit violations. Use `unblockip` if legitimate, or wait for the ban to expire.

### Events disappearing for users

Check if the event author has been blacklisted. Blacklisted authors' events are hidden from regular users but visible to admins.
