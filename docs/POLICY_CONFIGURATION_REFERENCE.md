# ORLY Policy Configuration Reference

This document provides a definitive reference for all policy configuration options and when each rule applies. Use this as the authoritative source for understanding policy behavior.

## Quick Reference: Read vs Write Applicability

| Rule Field | Write (EVENT) | Read (REQ) | Notes |
|------------|:-------------:|:----------:|-------|
| `size_limit` | ✅ | ❌ | Validates incoming events only |
| `content_limit` | ✅ | ❌ | Validates incoming events only |
| `max_age_of_event` | ✅ | ❌ | Prevents replay attacks |
| `max_age_event_in_future` | ✅ | ❌ | Prevents future-dated events |
| `max_expiry_duration` | ✅ | ❌ | Requires expiration tag |
| `must_have_tags` | ✅ | ❌ | Validates required tags |
| `allowed_tags` | ✅ | ❌ | Exclusive tag key whitelist |
| `protected_required` | ✅ | ❌ | Requires NIP-70 "-" tag |
| `identifier_regex` | ✅ | ❌ | Validates "d" tag format |
| `tag_validation` | ✅ | ❌ | Validates tag values with regex |
| `write_allow` | ✅ | ❌ | Pubkey whitelist for writing |
| `write_deny` | ✅ | ❌ | Pubkey blacklist for writing |
| `read_allow` | ❌ | ✅ | Pubkey whitelist for reading |
| `read_deny` | ❌ | ✅ | Pubkey blacklist for reading |
| `privileged` | ❌ | ✅ | Party-involved access control |
| `write_allow_follows` | ✅ | ✅ | Grants **both** read AND write |
| `follows_whitelist_admins` | ✅ | ✅ | Grants **both** read AND write |
| `script` | ✅ | ❌ | Scripts only run for writes |

---

## Core Principle: Validation vs Filtering

The policy system has two distinct modes of operation:

### Write Operations (EVENT messages)
- **Purpose**: Validate and accept/reject incoming events
- **All rules apply** except `read_allow`, `read_deny`, and `privileged`
- Events are checked **before storage**
- Rejected events are never stored

### Read Operations (REQ messages)
- **Purpose**: Filter which stored events a user can retrieve
- **Only access control rules apply**: `read_allow`, `read_deny`, `privileged`, `write_allow_follows`, `follows_whitelist_admins`
- Validation rules (size, age, tags) do NOT apply
- Scripts are NOT executed for reads
- Filtering happens **after database query**

---

## Configuration Structure

```json
{
  "default_policy": "allow|deny",
  "kind": {
    "whitelist": [1, 3, 7],
    "blacklist": [4, 42]
  },
  "owners": ["hex_pubkey_64_chars"],
  "policy_admins": ["hex_pubkey_64_chars"],
  "policy_follow_whitelist_enabled": true,
  "global": { /* Rule object */ },
  "rules": {
    "1": { /* Rule object for kind 1 */ },
    "30023": { /* Rule object for kind 30023 */ }
  }
}
```

---

## Top-Level Configuration Fields

### `default_policy`
**Type**: `string`
**Values**: `"allow"` (default) or `"deny"`
**Applies to**: Both read and write

The fallback behavior when no specific rule makes a decision.

```json
{
  "default_policy": "deny"
}
```

### `kind.whitelist` and `kind.blacklist`
**Type**: `[]int`
**Applies to**: Both read and write

Controls which event kinds are processed at all.

- **Whitelist** takes precedence: If present, ONLY whitelisted kinds are allowed
- **Blacklist**: If no whitelist, these kinds are denied
- **Neither**: Behavior depends on `default_policy` and whether rules exist

```json
{
  "kind": {
    "whitelist": [0, 1, 3, 7, 30023]
  }
}
```

### `owners`
**Type**: `[]string` (64-character hex pubkeys)
**Applies to**: Policy administration

Relay owners with full control. Merged with `ORLY_OWNERS` environment variable.

```json
{
  "owners": ["4a93c5ac0c6f49d2c7e7a5b8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8"]
}
```

### `policy_admins`
**Type**: `[]string` (64-character hex pubkeys)
**Applies to**: Policy administration

Pubkeys that can update policy via kind 12345 events (with restrictions).

### `policy_follow_whitelist_enabled`
**Type**: `boolean`
**Default**: `false`
**Applies to**: Both read and write (when `write_allow_follows` is true)

When enabled, allows `write_allow_follows` rules to grant access to policy admin follows.

---

## Rule Object Fields

Rules can be defined in `global` (applies to all events) or `rules[kind]` (applies to specific kind).

### Access Control Fields

#### `write_allow`
**Type**: `[]string` (hex pubkeys)
**Applies to**: Write only
**Behavior**: Exclusive whitelist

When present with entries, ONLY these pubkeys can write events of this kind. All others are denied.

```json
{
  "rules": {
    "1": {
      "write_allow": ["pubkey1_hex", "pubkey2_hex"]
    }
  }
}
```

**Special case**: Empty array `[]` explicitly allows all writers.

#### `write_deny`
**Type**: `[]string` (hex pubkeys)
**Applies to**: Write only
**Behavior**: Blacklist (highest priority)

These pubkeys cannot write events of this kind. **Checked before allow lists.**

```json
{
  "rules": {
    "1": {
      "write_deny": ["banned_pubkey_hex"]
    }
  }
}
```

#### `read_allow`
**Type**: `[]string` (hex pubkeys)
**Applies to**: Read only
**Behavior**: Exclusive whitelist (with OR logic for privileged)

When present with entries:
- If `privileged: false`: ONLY these pubkeys can read
- If `privileged: true`: These pubkeys OR parties involved can read

```json
{
  "rules": {
    "4": {
      "read_allow": ["trusted_pubkey_hex"],
      "privileged": true
    }
  }
}
```

#### `read_deny`
**Type**: `[]string` (hex pubkeys)
**Applies to**: Read only
**Behavior**: Blacklist (highest priority)

These pubkeys cannot read events of this kind. **Checked before allow lists.**

#### `privileged`
**Type**: `boolean`
**Default**: `false`
**Applies to**: Read only

When `true`, events are only readable by "parties involved":
- The event author (`event.pubkey`)
- Users mentioned in `p` tags

**Interaction with `read_allow`**:
- `read_allow` present + `privileged: true` = OR logic (in list OR party involved)
- `read_allow` empty + `privileged: true` = Only parties involved
- `privileged: true` alone = Only parties involved

```json
{
  "rules": {
    "4": {
      "description": "DMs - only sender and recipient can read",
      "privileged": true
    }
  }
}
```

#### `write_allow_follows`
**Type**: `boolean`
**Default**: `false`
**Applies to**: Both read AND write
**Requires**: `policy_follow_whitelist_enabled: true` at top level

Grants **both read and write access** to pubkeys followed by policy admins.

> **Important**: Despite the name, this grants BOTH read and write access.

```json
{
  "policy_follow_whitelist_enabled": true,
  "rules": {
    "1": {
      "write_allow_follows": true
    }
  }
}
```

#### `follows_whitelist_admins`
**Type**: `[]string` (hex pubkeys)
**Applies to**: Both read AND write

Alternative to `write_allow_follows` that specifies which admin pubkeys' follows are whitelisted for this specific rule.

```json
{
  "rules": {
    "30023": {
      "follows_whitelist_admins": ["curator_pubkey_hex"]
    }
  }
}
```

---

### Validation Fields (Write-Only)

These fields validate incoming events and are **completely ignored for read operations**.

#### `size_limit`
**Type**: `int64` (bytes)
**Applies to**: Write only

Maximum total serialized event size.

```json
{
  "global": {
    "size_limit": 100000
  }
}
```

#### `content_limit`
**Type**: `int64` (bytes)
**Applies to**: Write only

Maximum size of the `content` field.

```json
{
  "rules": {
    "1": {
      "content_limit": 10000
    }
  }
}
```

#### `max_age_of_event`
**Type**: `int64` (seconds)
**Applies to**: Write only

Maximum age of events. Events with `created_at` older than `now - max_age_of_event` are rejected.

```json
{
  "global": {
    "max_age_of_event": 86400
  }
}
```

#### `max_age_event_in_future`
**Type**: `int64` (seconds)
**Applies to**: Write only

Maximum time events can be dated in the future. Events with `created_at` later than `now + max_age_event_in_future` are rejected.

```json
{
  "global": {
    "max_age_event_in_future": 300
  }
}
```

#### `max_expiry_duration`
**Type**: `string` (ISO-8601 duration)
**Applies to**: Write only

Maximum allowed expiry time from event creation. Events **must** have an `expiration` tag when this is set.

**Format**: `P[n]Y[n]M[n]W[n]DT[n]H[n]M[n]S`

**Examples**:
- `P7D` = 7 days
- `PT1H` = 1 hour
- `P1DT12H` = 1 day 12 hours
- `PT30M` = 30 minutes

```json
{
  "rules": {
    "20": {
      "description": "Ephemeral events must expire within 24 hours",
      "max_expiry_duration": "P1D"
    }
  }
}
```

#### `must_have_tags`
**Type**: `[]string` (tag names)
**Applies to**: Write only

Required tags that must be present on the event.

```json
{
  "rules": {
    "1": {
      "must_have_tags": ["p", "e"]
    }
  }
}
```

#### `allowed_tags`
**Type**: `[]string` (tag key letters)
**Applies to**: Write only
**Behavior**: Exclusive tag key whitelist

When present with entries, ONLY tags whose key is in this list may appear on the
event. Any event containing a tag key **not** in the list is rejected. This is the
inverse of `must_have_tags`:

- `must_have_tags` requires certain tags to be **present** (but allows extras)
- `allowed_tags` forbids any tag **outside** the set (but does not require them)

Combine both to express "exactly these tags" semantics.

```json
{
  "rules": {
    "37801": {
      "description": "Only d and p tags permitted",
      "allowed_tags": ["d", "p"]
    }
  }
}
```

> **Important gotchas**:
> - An empty array or omitted field means **no restriction** (all tags allowed).
> - If you also set `protected_required`, `max_expiry_duration`, or
>   `identifier_regex` on the same kind, you must include the required tag keys
>   (`-`, `expiration`, `d`) in `allowed_tags`, or every event will be rejected.
> - Policy admins cannot introduce or narrow an `allowed_tags` whitelist via kind
>   12345 updates (it is a restriction only owners may add); they may widen it.

#### `protected_required`
**Type**: `boolean`
**Default**: `false`
**Applies to**: Write only

Requires events to have a `-` tag (NIP-70 protected events).

```json
{
  "rules": {
    "4": {
      "protected_required": true
    }
  }
}
```

#### `identifier_regex`
**Type**: `string` (regex pattern)
**Applies to**: Write only

Regex pattern that `d` tag values must match. Events **must** have a `d` tag when this is set.

```json
{
  "rules": {
    "30023": {
      "identifier_regex": "^[a-z0-9-]{1,64}$"
    }
  }
}
```

#### `tag_validation`
**Type**: `map[string]string` (tag name → regex pattern)
**Applies to**: Write only

Regex patterns for validating specific tag values. Only validates tags that are **present** on the event.

> **Note**: To require a tag to exist, use `must_have_tags`. `tag_validation` only validates format.

```json
{
  "rules": {
    "30023": {
      "tag_validation": {
        "t": "^[a-z0-9-]{1,32}$",
        "d": "^[a-z0-9-]+$"
      }
    }
  }
}
```

---

### Script Configuration

#### `script`
**Type**: `string` (file path)
**Applies to**: Write only

Path to a custom validation script. **Scripts are NOT executed for read operations.**

```json
{
  "rules": {
    "1": {
      "script": "/etc/orly/scripts/spam-filter.py"
    }
  }
}
```

---

## Policy Evaluation Order

### For Write Operations

```
1. Global Rule Check (all fields apply)
   ├─ Universal constraints (size, tags, age, etc.)
   ├─ write_deny check
   ├─ write_allow_follows / follows_whitelist_admins check
   └─ write_allow check

2. Kind Filtering (whitelist/blacklist)

3. Kind-Specific Rule Check (same as global)
   ├─ Universal constraints
   ├─ write_deny check
   ├─ write_allow_follows / follows_whitelist_admins check
   ├─ write_allow check
   └─ Script execution (if configured)

4. Default Policy (if no rules matched)
```

### For Read Operations

```
1. Global Rule Check (access control only)
   ├─ read_deny check
   ├─ write_allow_follows / follows_whitelist_admins check
   ├─ read_allow check
   └─ privileged check (party involved)

2. Kind Filtering (whitelist/blacklist)

3. Kind-Specific Rule Check (access control only)
   ├─ read_deny check
   ├─ write_allow_follows / follows_whitelist_admins check
   ├─ read_allow + privileged (OR logic)
   └─ privileged-only check

4. Default Policy (if no rules matched)

NOTE: Scripts are NOT executed for read operations
```

---

## Common Configuration Patterns

### Private Relay (Whitelist Only)

```json
{
  "default_policy": "deny",
  "global": {
    "write_allow": ["trusted_pubkey_1", "trusted_pubkey_2"],
    "read_allow": ["trusted_pubkey_1", "trusted_pubkey_2"]
  }
}
```

### Open Relay with Spam Protection

```json
{
  "default_policy": "allow",
  "global": {
    "size_limit": 100000,
    "max_age_of_event": 86400,
    "max_age_event_in_future": 300
  },
  "rules": {
    "1": {
      "script": "/etc/orly/scripts/spam-filter.sh"
    }
  }
}
```

### Community Relay (Follows-Based)

```json
{
  "default_policy": "deny",
  "policy_admins": ["community_admin_pubkey"],
  "policy_follow_whitelist_enabled": true,
  "global": {
    "write_allow_follows": true
  }
}
```

### Encrypted DMs (Privileged Access)

```json
{
  "rules": {
    "4": {
      "description": "Encrypted DMs - only sender/recipient",
      "privileged": true,
      "protected_required": true
    }
  }
}
```

### Long-Form Content with Validation

```json
{
  "rules": {
    "30023": {
      "description": "Long-form articles",
      "size_limit": 100000,
      "content_limit": 50000,
      "max_expiry_duration": "P30D",
      "identifier_regex": "^[a-z0-9-]{1,64}$",
      "tag_validation": {
        "t": "^[a-z0-9-]{1,32}$"
      }
    }
  }
}
```

---

## Important Behaviors

### Whitelist vs Blacklist Precedence

1. **Deny lists** (`write_deny`, `read_deny`) are checked **first** and have highest priority
2. **Allow lists** are exclusive when populated - ONLY listed pubkeys are allowed
3. **Deny-only configuration**: If only deny list exists (no allow list), all non-denied pubkeys are allowed

### Empty Arrays vs Null

- `[]` (empty array explicitly set) = Allow all
- `null` or field omitted = No list configured, use other rules

### Global Rules Are Additive

Global rules are always evaluated **in addition to** kind-specific rules. They cannot be overridden at the kind level.

### Implicit Kind Whitelist

When rules are defined but no explicit `kind.whitelist`:
- If `default_policy: "allow"`: All kinds allowed
- If `default_policy: "deny"` or unset: Only kinds with rules allowed

---

## Debugging Policy Issues

Enable debug logging to see policy decisions:

```bash
export ORLY_LOG_LEVEL=debug
```

Log messages include:
- Policy evaluation steps
- Rule matching
- Access decisions with reasons

---

## Source Code Reference

- Policy struct definition: `pkg/policy/policy.go:75-144` (Rule struct)
- Policy struct definition: `pkg/policy/policy.go:380-412` (P struct)
- Check evaluation: `pkg/policy/policy.go:1260-1595` (checkRulePolicy)
- Write handler: `app/handle-event.go:114-138`
- Read handler: `app/handle-req.go:420-438`