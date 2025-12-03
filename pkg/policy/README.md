# ORLY Policy System

The policy system provides fine-grained control over event storage and retrieval in the ORLY Nostr relay. It allows relay operators to define rules based on event kinds, pubkeys, content size, timestamps, tags, and custom scripts.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Configuration Structure](#configuration-structure)
- [Policy Fields Reference](#policy-fields-reference)
  - [Top-Level Fields](#top-level-fields)
  - [Kind Filtering](#kind-filtering)
  - [Rule Fields](#rule-fields)
- [ISO-8601 Duration Format](#iso-8601-duration-format)
- [Access Control](#access-control)
- [Follows-Based Whitelisting](#follows-based-whitelisting)
- [Tag Validation](#tag-validation)
- [Policy Scripts](#policy-scripts)
- [Dynamic Policy Updates](#dynamic-policy-updates)
- [Evaluation Order](#evaluation-order)
- [Examples](#examples)

## Overview

The policy system evaluates every event against configured rules before allowing storage (write) or retrieval (read). Rules are evaluated as AND operations—all configured criteria must be satisfied for an event to be allowed.

Key capabilities:
- **Kind filtering**: Whitelist or blacklist specific event kinds
- **Pubkey access control**: Allow/deny lists for reading and writing
- **Size limits**: Restrict total event size and content length
- **Timestamp validation**: Reject events that are too old or too far in the future
- **Expiry enforcement**: Require events to have expiration tags within limits
- **Tag validation**: Enforce regex patterns on tag values
- **Protected events**: Require NIP-70 protected event markers
- **Follows-based access**: Whitelist pubkeys followed by admins
- **Custom scripts**: External scripts for complex validation logic

## Quick Start

### 1. Enable the Policy System

```bash
export ORLY_POLICY_ENABLED=true
```

### 2. Create a Policy Configuration

Create `~/.config/ORLY/policy.json`:

```json
{
  "default_policy": "allow",
  "global": {
    "max_age_of_event": 86400,
    "size_limit": 100000
  },
  "rules": {
    "1": {
      "description": "Text notes",
      "size_limit": 32000,
      "max_expiry_duration": "P7D"
    }
  }
}
```

### 3. Restart the Relay

```bash
sudo systemctl restart orly
```

## Configuration Structure

```json
{
  "default_policy": "allow|deny",
  "kind": {
    "whitelist": [1, 3, 4],
    "blacklist": []
  },
  "global": { /* Rule fields applied to all events */ },
  "rules": {
    "1": { /* Rule fields for kind 1 */ },
    "30023": { /* Rule fields for kind 30023 */ }
  },
  "policy_admins": ["hex_pubkey_1", "hex_pubkey_2"],
  "policy_follow_whitelist_enabled": false
}
```

## Policy Fields Reference

### Top-Level Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `default_policy` | string | `"allow"` | Fallback behavior when no rules match: `"allow"` or `"deny"` |
| `kind` | object | `{}` | Kind whitelist/blacklist configuration |
| `global` | object | `{}` | Rule applied to ALL events regardless of kind |
| `rules` | object | `{}` | Map of kind number (as string) to rule configuration |
| `policy_admins` | array | `[]` | Hex-encoded pubkeys that can update policy via kind 12345 events |
| `policy_follow_whitelist_enabled` | boolean | `false` | Enable follows-based whitelisting for `write_allow_follows` |

### Kind Filtering

```json
"kind": {
  "whitelist": [1, 3, 4, 7, 9735],
  "blacklist": [4]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `whitelist` | array | Only these kinds are allowed. If present, all others are denied. |
| `blacklist` | array | These kinds are denied. Only evaluated if whitelist is empty. |

**Precedence**: Whitelist takes precedence over blacklist. If whitelist has entries, blacklist is ignored.

### Rule Fields

Rules can be applied globally (in `global`) or per-kind (in `rules`). All configured criteria are evaluated as AND operations.

#### Description

```json
{
  "description": "Human-readable description of this rule"
}
```

#### Access Control Lists

| Field | Type | Description |
|-------|------|-------------|
| `write_allow` | array | Hex pubkeys allowed to write. If present, all others denied. |
| `write_deny` | array | Hex pubkeys denied from writing. Only evaluated if `write_allow` is empty. |
| `read_allow` | array | Hex pubkeys allowed to read. If present, all others denied. |
| `read_deny` | array | Hex pubkeys denied from reading. Only evaluated if `read_allow` is empty. |

```json
{
  "write_allow": ["npub1...", "npub2..."],
  "write_deny": ["npub3..."],
  "read_allow": [],
  "read_deny": ["npub4..."]
}
```

#### Size Limits

| Field | Type | Unit | Description |
|-------|------|------|-------------|
| `size_limit` | integer | bytes | Maximum total serialized event size |
| `content_limit` | integer | bytes | Maximum content field size |

```json
{
  "size_limit": 100000,
  "content_limit": 50000
}
```

#### Timestamp Validation

| Field | Type | Unit | Description |
|-------|------|------|-------------|
| `max_age_of_event` | integer | seconds | Maximum age of event's `created_at` (prevents replay attacks) |
| `max_age_event_in_future` | integer | seconds | Maximum time event can be in the future |

```json
{
  "max_age_of_event": 86400,
  "max_age_event_in_future": 300
}
```

#### Expiry Enforcement

| Field | Type | Description |
|-------|------|-------------|
| `max_expiry` | integer | **Deprecated.** Maximum expiry time in raw seconds. |
| `max_expiry_duration` | string | Maximum expiry time in ISO-8601 duration format. Takes precedence over `max_expiry`. |

When set, events **must** have an `expiration` tag, and the expiry time must be within the specified duration from the event's `created_at` time.

```json
{
  "max_expiry_duration": "P7D"
}
```

#### Required Tags

| Field | Type | Description |
|-------|------|-------------|
| `must_have_tags` | array | Tag key letters that must be present on the event |

```json
{
  "must_have_tags": ["d", "t"]
}
```

#### Privileged Events

| Field | Type | Description |
|-------|------|-------------|
| `privileged` | boolean | Only parties involved (author or p-tag recipients) can read/write |

```json
{
  "privileged": true
}
```

#### Protected Events (NIP-70)

| Field | Type | Description |
|-------|------|-------------|
| `protected_required` | boolean | Requires events to have a `-` tag (NIP-70 protected marker) |

Protected events signal that they should only be published to relays that enforce access control.

```json
{
  "protected_required": true
}
```

#### Identifier Regex

| Field | Type | Description |
|-------|------|-------------|
| `identifier_regex` | string | Regex pattern that `d` tag values must match |

When set, events **must** have at least one `d` tag, and **all** `d` tags must match the pattern.

```json
{
  "identifier_regex": "^[a-z0-9-]{1,64}$"
}
```

#### Tag Validation

| Field | Type | Description |
|-------|------|-------------|
| `tag_validation` | object | Map of tag name to regex pattern |

Validates that tag values match the specified regex patterns. Only validates tags that are present—does not require tags to exist.

```json
{
  "tag_validation": {
    "d": "^[a-z0-9-]{1,64}$",
    "t": "^[a-z0-9]+$"
  }
}
```

#### Follows-Based Whitelisting

| Field | Type | Description |
|-------|------|-------------|
| `write_allow_follows` | boolean | **DEPRECATED.** Grant read+write access to policy admin follows |
| `follows_whitelist_admins` | array | **DEPRECATED.** Per-rule admin pubkeys whose follows are whitelisted |
| `read_follows_whitelist` | array | Pubkeys whose follows can READ events. Restricts read access when set. |
| `write_follows_whitelist` | array | Pubkeys whose follows can WRITE events. Restricts write access when set. |

See [Follows-Based Whitelisting](#follows-based-whitelisting) for details.

#### Rate Limiting

| Field | Type | Unit | Description |
|-------|------|------|-------------|
| `rate_limit` | integer | bytes/second | Maximum data rate per authenticated connection |

```json
{
  "rate_limit": 10000
}
```

#### Custom Scripts

| Field | Type | Description |
|-------|------|-------------|
| `script` | string | Path to external validation script |

See [Policy Scripts](#policy-scripts) for details.

## ISO-8601 Duration Format

The `max_expiry_duration` field uses strict ISO-8601 duration format, parsed by the [sosodev/duration](https://github.com/sosodev/duration) library.

### Format

```
P[n]Y[n]M[n]W[n]DT[n]H[n]M[n]S
```

| Component | Meaning | Example |
|-----------|---------|---------|
| `P` | **Required** prefix (Period) | `P1D` |
| `Y` | Years (~365.25 days) | `P1Y` |
| `M` | Months (~30.44 days) - date part | `P1M` |
| `W` | Weeks (7 days) | `P2W` |
| `D` | Days | `P7D` |
| `T` | **Required** separator before time | `PT1H` |
| `H` | Hours (requires T) | `PT2H` |
| `M` | Minutes (requires T) - time part | `PT30M` |
| `S` | Seconds (requires T) | `PT90S` |

### Examples

| Duration | Meaning | Seconds |
|----------|---------|---------|
| `P1D` | 1 day | 86,400 |
| `P7D` | 7 days | 604,800 |
| `P30D` | 30 days | 2,592,000 |
| `PT1H` | 1 hour | 3,600 |
| `PT30M` | 30 minutes | 1,800 |
| `PT90S` | 90 seconds | 90 |
| `P1DT12H` | 1 day 12 hours | 129,600 |
| `P1DT2H30M` | 1 day 2 hours 30 minutes | 95,400 |
| `P1W` | 1 week | 604,800 |
| `P1M` | 1 month | 2,628,000 |
| `P1Y` | 1 year | 31,536,000 |
| `PT1.5H` | 1.5 hours | 5,400 |
| `P0.5D` | 12 hours | 43,200 |

### Important Notes

1. **P prefix is required**: `1D` is invalid, use `P1D`
2. **T separator is required before time**: `P1H` is invalid, use `PT1H`
3. **Date components before T**: `PT1D` is invalid (D is a date component)
4. **Case insensitive**: `p1d` and `P1D` are equivalent
5. **Fractional values supported**: `PT1.5H`, `P0.5D`

### Invalid Examples

| Invalid | Why | Correct |
|---------|-----|---------|
| `1D` | Missing P prefix | `P1D` |
| `P1H` | H needs T separator | `PT1H` |
| `PT1D` | D is date component | `P1D` |
| `P30S` | S needs T separator | `PT30S` |
| `P-5D` | Negative not allowed | `P5D` |
| `PD` | Missing number | `P1D` |

## Access Control

### Default-Permissive Access Model

The policy system uses a **default-permissive** model for both read and write access:

- **Read**: Allowed by default unless a read restriction is configured
- **Write**: Allowed by default unless a write restriction is configured

Restrictions become active when any of the following fields are set:

| Access | Restrictions |
|--------|--------------|
| Read | `read_allow`, `read_follows_whitelist`, or `privileged` |
| Write | `write_allow`, `write_follows_whitelist` |

**Important**: `privileged` ONLY applies to READ operations.

### Write Access Evaluation (Default-Permissive)

```
1. Universal constraints (size, tags, age) - must pass
2. If pubkey in write_deny → DENY
3. If write_allow_follows enabled and pubkey in admin follows → ALLOW
4. If write_follows_whitelist set and pubkey in follows → ALLOW
5. If write_allow set and pubkey in list → ALLOW
6. If ANY write restriction is set → DENY (not in any whitelist)
7. Otherwise → ALLOW (default-permissive)
```

### Read Access Evaluation (Default-Permissive)

```
1. If pubkey in read_deny → DENY
2. If read_allow_follows enabled and pubkey in admin follows → ALLOW
3. If read_follows_whitelist set and pubkey in follows → ALLOW
4. If read_allow set and pubkey in list → ALLOW
5. If privileged set and pubkey is party to event → ALLOW
6. If ANY read restriction is set → DENY (not in any whitelist)
7. Otherwise → ALLOW (default-permissive)
```

### Privileged Events (Read-Only)

When `privileged: true`, only the author and p-tag recipients can access the event:

```json
{
  "rules": {
    "4": {
      "description": "Encrypted DMs",
      "privileged": true
    }
  }
}
```

## Follows-Based Whitelisting

The policy system supports whitelisting pubkeys based on follow lists (kind 3 events). There are two approaches:

### 1. Separate Read/Write Follows Whitelists (Recommended)

Use `read_follows_whitelist` and `write_follows_whitelist` for fine-grained control:

```json
{
  "global": {
    "read_follows_whitelist": ["curator_pubkey_hex"],
    "write_follows_whitelist": ["moderator_pubkey_hex"]
  },
  "rules": {
    "30023": {
      "description": "Articles - curated reading, moderated writing",
      "read_follows_whitelist": ["article_curator_hex"],
      "write_follows_whitelist": ["article_moderator_hex"]
    }
  }
}
```

**How it works:**
- The pubkeys listed AND their follows (from kind 3 events) can access the events
- `read_follows_whitelist`: Restricts WHO can read (when set)
- `write_follows_whitelist`: Restricts WHO can write (when set)
- If not set, the default-permissive behavior applies

**Important:** The relay will fail to start if the named pubkeys don't have kind 3 follow list events in the database. This ensures the follow lists are available for access control.

### 2. Legacy: Global Policy Admin Follows (DEPRECATED)

Enable whitelisting for all pubkeys followed by policy admins:

```json
{
  "policy_admins": ["admin_pubkey_hex"],
  "policy_follow_whitelist_enabled": true,
  "rules": {
    "1": {
      "write_allow_follows": true
    }
  }
}
```

When `write_allow_follows` is true, pubkeys in the policy admins' kind 3 follow lists get both read AND write access.

### 3. Legacy: Per-Rule Follows Whitelist (DEPRECATED)

Configure specific admins per rule:

```json
{
  "rules": {
    "30023": {
      "description": "Long-form articles from curator's follows",
      "follows_whitelist_admins": ["curator_pubkey_hex"]
    }
  }
}
```

This allows different rules to use different admin follow lists.

### Loading Follow Lists at Startup

The application must load follow lists at startup. The new API provides separate methods:

```go
// Get all pubkeys that need follow lists loaded (combines read + write + legacy)
allPubkeys := policy.GetAllFollowsWhitelistPubkeys()

// Or get them separately
readPubkeys := policy.GetAllReadFollowsWhitelistPubkeys()
writePubkeys := policy.GetAllWriteFollowsWhitelistPubkeys()
legacyAdmins := policy.GetAllFollowsWhitelistAdmins()

// Load follows and update the policy
for _, pubkeyHex := range readPubkeys {
    follows := loadFollowsFromKind3(pubkeyHex)
    // Update read follows whitelist for specific kinds
    policy.UpdateRuleReadFollowsWhitelist(kind, follows)
    // Or for global rule
    policy.UpdateGlobalReadFollowsWhitelist(follows)
}

for _, pubkeyHex := range writePubkeys {
    follows := loadFollowsFromKind3(pubkeyHex)
    policy.UpdateRuleWriteFollowsWhitelist(kind, follows)
    // Or for global rule
    policy.UpdateGlobalWriteFollowsWhitelist(follows)
}
```

## Tag Validation

### Using tag_validation

Validate multiple tags with regex patterns:

```json
{
  "rules": {
    "30023": {
      "tag_validation": {
        "d": "^[a-z0-9-]{1,64}$",
        "t": "^[a-z0-9]+$",
        "title": "^.{1,100}$"
      }
    }
  }
}
```

- Only validates tags that are **present** on the event
- Does **not** require tags to exist (use `must_have_tags` for that)
- **All** values of a repeated tag must match the pattern

### Using identifier_regex

Shorthand for `d` tag validation:

```json
{
  "identifier_regex": "^[a-z0-9-]{1,64}$"
}
```

This is equivalent to:
```json
{
  "tag_validation": {
    "d": "^[a-z0-9-]{1,64}$"
  }
}
```

**Important**: When `identifier_regex` is set, events **must** have at least one `d` tag.

### Common Patterns

| Pattern | Description |
|---------|-------------|
| `^[a-z0-9-]{1,64}$` | URL-friendly slug |
| `^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$` | UUID |
| `^[a-zA-Z0-9_]+$` | Alphanumeric with underscores |
| `^.{1,100}$` | Any characters, max 100 |

## Policy Scripts

External scripts provide custom validation logic.

### Script Interface

**Input**: JSON event objects on stdin (one per line):

```json
{
  "id": "event_id_hex",
  "pubkey": "author_pubkey_hex",
  "kind": 1,
  "content": "Hello, world!",
  "tags": [["p", "recipient_hex"]],
  "created_at": 1640995200,
  "sig": "signature_hex",
  "logged_in_pubkey": "authenticated_user_hex",
  "ip_address": "127.0.0.1",
  "access_type": "write"
}
```

**Output**: JSON response on stdout:

```json
{"id": "event_id_hex", "action": "accept", "msg": ""}
```

### Actions

| Action | OK Response | Effect |
|--------|-------------|--------|
| `accept` | true | Store/retrieve event normally |
| `reject` | false | Reject with error message |
| `shadowReject` | true | Silently drop (appears successful to client) |

### Script Requirements

1. **Long-lived process**: Read stdin in a loop, don't exit after one event
2. **JSON only on stdout**: Use stderr for debug logging
3. **Flush after each response**: Call `sys.stdout.flush()` (Python) or equivalent
4. **Handle errors gracefully**: Always return valid JSON

### Example Script (Python)

```python
#!/usr/bin/env python3
import json
import sys

def process_event(event):
    if 'spam' in event.get('content', '').lower():
        return {'id': event['id'], 'action': 'reject', 'msg': 'Spam detected'}
    return {'id': event['id'], 'action': 'accept', 'msg': ''}

for line in sys.stdin:
    if line.strip():
        try:
            event = json.loads(line)
            response = process_event(event)
            print(json.dumps(response))
            sys.stdout.flush()
        except json.JSONDecodeError:
            print(json.dumps({'id': '', 'action': 'reject', 'msg': 'Invalid JSON'}))
            sys.stdout.flush()
```

### Configuration

```json
{
  "rules": {
    "1": {
      "script": "/etc/orly/scripts/spam-filter.py"
    }
  }
}
```

## Dynamic Policy Updates

Policy admins can update configuration at runtime by publishing kind 12345 events.

### Setup

```json
{
  "policy_admins": ["admin_pubkey_hex"],
  "default_policy": "allow"
}
```

### Publishing Updates

Send a kind 12345 event with the new policy as JSON content:

```json
{
  "kind": 12345,
  "content": "{\"default_policy\": \"deny\", \"kind\": {\"whitelist\": [1,3,7]}}",
  "tags": [],
  "created_at": 1234567890
}
```

### Security

- Only pubkeys in `policy_admins` can update policy
- Invalid JSON or configuration is rejected (existing policy preserved)
- All updates are logged for audit purposes

## Evaluation Order

Events are evaluated in this order:

1. **Global Rules** - Applied to all events first
2. **Kind Filtering** - Whitelist/blacklist check
3. **Kind-Specific Rules** - Rules for the event's kind
4. **Script Evaluation** - If configured and running
5. **Default Policy** - Fallback if no rules deny

The first rule that denies access stops evaluation. If all rules pass, the event is allowed.

### Rule Criteria (AND Logic)

Within a rule, all configured criteria must be satisfied:

```
access_allowed = (
    pubkey_check_passed AND
    size_check_passed AND
    timestamp_check_passed AND
    expiry_check_passed AND
    tag_check_passed AND
    protected_check_passed AND
    script_check_passed
)
```

## Examples

### Open Relay with Size Limits

```json
{
  "default_policy": "allow",
  "global": {
    "size_limit": 100000,
    "max_age_of_event": 86400,
    "max_age_event_in_future": 300
  }
}
```

### Private Relay

```json
{
  "default_policy": "deny",
  "global": {
    "write_allow": ["trusted_pubkey_1", "trusted_pubkey_2"],
    "read_allow": ["trusted_pubkey_1", "trusted_pubkey_2"]
  }
}
```

### Ephemeral Events with Expiry

```json
{
  "default_policy": "allow",
  "rules": {
    "20": {
      "description": "Ephemeral events must expire within 24 hours",
      "max_expiry_duration": "P1D"
    }
  }
}
```

### Long-Form Content with Strict Validation

```json
{
  "default_policy": "deny",
  "rules": {
    "30023": {
      "description": "Long-form articles with strict requirements",
      "max_expiry_duration": "P30D",
      "protected_required": true,
      "identifier_regex": "^[a-z0-9-]{1,64}$",
      "follows_whitelist_admins": ["curator_pubkey_hex"],
      "tag_validation": {
        "t": "^[a-z0-9-]{1,32}$"
      },
      "size_limit": 100000,
      "content_limit": 50000
    }
  }
}
```

### Encrypted DMs with Privacy

```json
{
  "default_policy": "allow",
  "rules": {
    "4": {
      "description": "Encrypted DMs - private and protected",
      "protected_required": true,
      "privileged": true
    }
  }
}
```

### Community-Curated Content

```json
{
  "default_policy": "deny",
  "policy_admins": ["community_admin_hex"],
  "policy_follow_whitelist_enabled": true,
  "rules": {
    "1": {
      "description": "Only community members can post",
      "write_allow_follows": true,
      "size_limit": 32000
    }
  }
}
```

### Kind Whitelist with Global Limits

```json
{
  "default_policy": "deny",
  "kind": {
    "whitelist": [0, 1, 3, 4, 7, 9735, 30023]
  },
  "global": {
    "size_limit": 100000,
    "max_age_of_event": 604800,
    "max_age_event_in_future": 60
  }
}
```

## Testing

### Run Policy Tests

```bash
CGO_ENABLED=0 go test -v ./pkg/policy/...
```

### Test Scripts Manually

```bash
echo '{"id":"test","kind":1,"content":"test"}' | ./policy-script.py
```

Expected output:
```json
{"id":"test","action":"accept","msg":""}
```

## Troubleshooting

### Policy Not Loading

```bash
# Check file exists and is valid JSON
cat ~/.config/ORLY/policy.json | jq .
```

### Script Not Working

```bash
# Check script is executable
ls -la /path/to/script.py

# Test script independently
echo '{"id":"test","kind":1}' | /path/to/script.py
```

### Enable Debug Logging

```bash
export ORLY_LOG_LEVEL=debug
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "invalid ISO-8601 duration" | Wrong format | Use `P1D` not `1d` |
| "H requires T separator" | Missing T | Use `PT1H` not `P1H` |
| Script timeout | Script not responding | Ensure flush after each response |
| Broken pipe | Script exited | Script must run continuously |
