# Policy System Troubleshooting Guide

This guide helps you configure and troubleshoot the ORLY relay policy system based on the requirements from [Issue #5](https://git.nostrdev.com/mleku/next.orly.dev/issues/5).

## Definition of Done Requirements

The policy system must support:

1. **Configure relay to accept only certain kind events** ✅
2. **Scenario A**: Only certain users should be allowed to write events ✅
3. **Scenario B**: Only certain users should be allowed to read events ✅
4. **Scenario C**: Only users involved in events should be able to read the events (privileged) ✅
5. **Scenario D**: Scripting option for complex validation ✅

All requirements are **implemented and tested** (see `pkg/policy/comprehensive_test.go`).

## Policy Evaluation Order (CRITICAL FOR CORRECT CONFIGURATION)

The policy system evaluates rules in a specific order. **Understanding this order is crucial for correct configuration:**

### Overall Evaluation Flow:
1. **Global Rules** (age, size) - Universal constraints applied first
2. **Kind Whitelist/Blacklist** - Absolute gatekeepers for event types
3. **Script Execution** (if configured and enabled)
4. **Rule-based Filtering** (see detailed order below)

### Rule-based Filtering Order (within checkRulePolicy):
1. **Universal Constraints** - Size limits, required tags, timestamps
2. **Explicit Denials** (deny lists) - **Highest priority blacklist**
3. **Privileged Access Check** - Parties involved **override allow lists**
4. **Exclusive Allow Lists** - **ONLY** listed users get access
5. **Privileged Final Check** - Non-involved users denied for privileged events
6. **Default Behavior** - Fallback when no specific rules apply

### Key Concepts:

- **Allow lists are EXCLUSIVE**: When `write_allow` or `read_allow` is specified, **ONLY** those users can access. Others are denied regardless of default policy.
- **Deny lists have highest priority**: Users in deny lists are **always denied**, even if they're in allow lists or involved in privileged events.
- **Allow lists override privileged access**: When BOTH `privileged: true` AND allow lists are specified, the allow list is **authoritative** - even parties involved must be in the allow list.
- **Privileged without allow lists**: If `privileged: true` but no allow lists, parties involved get automatic access.
- **Default policy rarely applies**: Only used when no specific rules exist for a kind.

### Common Misunderstandings:

1. **"Allow lists should be inclusive"** - NO! Allow lists are exclusive. If you want some users to have guaranteed access while others follow default policy, use privileged events or scripting.

2. **"Default policy should apply when not in allow list"** - NO! When an allow list exists, it completely overrides default policy for that kind.

3. **"Privileged should be checked last"** - NO! Privileged access is checked early to override allow lists for parties involved.

## Quick Start

### Step 1: Enable Policy System

Set the environment variable:

```bash
export ORLY_POLICY_ENABLED=true
```

Or add to your service file:

```ini
Environment="ORLY_POLICY_ENABLED=true"
```

### Step 2: Create Policy Configuration File

The policy configuration file must be located at:

```
$HOME/.config/ORLY/policy.json
```

Or if using a custom app name:

```
$HOME/.config/<YOUR_APP_NAME>/policy.json
```

### Step 3: Configure Your Policy

Create `~/.config/ORLY/policy.json` with your desired rules. See examples below.

### Step 4: Restart Relay

```bash
sudo systemctl restart orly
```

### Step 5: Verify Policy is Loaded

Check the logs:

```bash
sudo journalctl -u orly -f | grep -i policy
```

You should see:

```
loaded policy configuration from /home/user/.config/ORLY/policy.json
```

## Configuration Examples

### Example 1: Kind Whitelist (Requirement 1)

Only accept kinds 1, 3, 4, and 7:

```json
{
  "kind": {
    "whitelist": [1, 3, 4, 7]
  },
  "default_policy": "allow"
}
```

**How it works:**
- Events with kinds 1, 3, 4, or 7 are allowed
- Events with any other kind are **automatically rejected**
- This is checked BEFORE any rule-specific policies

### Example 2: Per-Kind Write Access (Scenario A)

Only specific users can write kind 10 events:

```json
{
  "rules": {
    "10": {
      "description": "Only Alice can write kind 10",
      "write_allow": ["ALICE_PUBKEY_HEX"]
    }
  },
  "default_policy": "allow"
}
```

**How it works:**
- Only the pubkey in `write_allow` can publish kind 10 events
- All other users are denied
- The pubkey in the event MUST match one in `write_allow`

### Example 3: Per-Kind Read Access (Scenario B)

Only specific users can read kind 20 events:

```json
{
  "rules": {
    "20": {
      "description": "Only Bob can read kind 20",
      "read_allow": ["BOB_PUBKEY_HEX"]
    }
  },
  "default_policy": "allow"
}
```

**How it works:**
- Only users authenticated as the pubkey in `read_allow` can see kind 20 events in REQ responses
- Unauthenticated users cannot see these events
- Users authenticated as different pubkeys cannot see these events

### Example 4: Privileged Events (Scenario C)

Only users involved in the event can read it:

```json
{
  "rules": {
    "4": {
      "description": "Encrypted DMs - only parties involved",
      "privileged": true
    },
    "14": {
      "description": "Direct Messages - only parties involved",
      "privileged": true
    }
  },
  "default_policy": "allow"
}
```

**How it works:**
- A user can read a privileged event ONLY if they are:
  1. The author of the event (`ev.pubkey == user.pubkey`), OR
  2. Mentioned in a `p` tag (`["p", "user_pubkey_hex"]`)
- Unauthenticated users cannot see privileged events
- Third parties cannot see privileged events

### Example 5: Script-Based Validation (Scenario D)

Use a custom script for complex validation:

```json
{
  "rules": {
    "30078": {
      "description": "Custom validation via script",
      "script": "/home/user/.config/ORLY/validate-30078.sh"
    }
  },
  "default_policy": "allow"
}
```

**Script Requirements:**
1. Must be executable (`chmod +x script.sh`)
2. Reads JSONL (one event per line) from stdin
3. Writes JSONL responses to stdout
4. Each response must have: `{"id":"event_id","action":"accept|reject|shadowReject","msg":"reason"}`

Example script:

```bash
#!/bin/bash
while IFS= read -r line; do
  # Parse event JSON and apply custom logic
  if echo "$line" | jq -e '.kind == 30078 and (.content | length) < 1000' > /dev/null; then
    echo "{\"id\":\"$(echo "$line" | jq -r .id)\",\"action\":\"accept\",\"msg\":\"ok\"}"
  else
    echo "{\"id\":\"$(echo "$line" | jq -r .id)\",\"action\":\"reject\",\"msg\":\"content too long\"}"
  fi
done
```

### Example 6: Combined Policy

All features together:

```json
{
  "kind": {
    "whitelist": [1, 3, 4, 10, 20, 30]
  },
  "rules": {
    "10": {
      "description": "Only Alice can write",
      "write_allow": ["ALICE_PUBKEY_HEX"]
    },
    "20": {
      "description": "Only Bob can read",
      "read_allow": ["BOB_PUBKEY_HEX"]
    },
    "4": {
      "description": "Encrypted DMs - privileged",
      "privileged": true
    },
    "30": {
      "description": "Custom validation",
      "script": "/home/user/.config/ORLY/validate.sh",
      "write_allow": ["ALICE_PUBKEY_HEX"]
    }
  },
  "global": {
    "description": "Global rules for all events",
    "max_age_of_event": 31536000,
    "max_age_event_in_future": 3600
  },
  "default_policy": "allow"
}
```

## Common Issues and Solutions

### Issue 1: Events Outside Whitelist Are Accepted

**Symptoms:**
- You configured a kind whitelist
- Events with kinds NOT in the whitelist are still accepted

**Solution:**
Check that policy is enabled:

```bash
# Check if policy is enabled
echo $ORLY_POLICY_ENABLED

# Check if config file exists
ls -l ~/.config/ORLY/policy.json

# Check logs for policy loading
sudo journalctl -u orly | grep -i policy
```

If policy is not loading:

1. Verify `ORLY_POLICY_ENABLED=true` is set
2. Verify config file is in correct location
3. Verify JSON is valid (use `jq . < ~/.config/ORLY/policy.json`)
4. Restart the relay

### Issue 2: Read Restrictions Not Enforced

**Symptoms:**
- You configured `read_allow` for a kind
- Unauthorized users can still see those events

**Solution:**

1. **Check authentication**: Users MUST be authenticated via NIP-42 AUTH
   - Set `ORLY_AUTH_REQUIRED=true` to force authentication
   - Or use ACL mode: `ORLY_ACL_MODE=managed` or `ORLY_ACL_MODE=follows`

2. **Check policy configuration**:
   ```bash
   cat ~/.config/ORLY/policy.json | jq '.rules["YOUR_KIND"].read_allow'
   ```

3. **Check relay logs** when a REQ is made:
   ```bash
   sudo journalctl -u orly -f | grep -E "(policy|CheckPolicy|read)"
   ```

4. **Verify pubkey format**: Use hex (64 chars), not npub

Example to convert npub to hex:

```bash
# Using nak (nostr army knife)
nak decode npub1...

# Or use your client's developer tools
```

### Issue 3: Kind Whitelist Not Working

**Symptoms:**
- You have `"whitelist": [1,3,4]`
- Events with kind 5 are still accepted

**Possible Causes:**

1. **Policy not enabled**
   ```bash
   # Check environment variable
   systemctl show orly | grep ORLY_POLICY_ENABLED
   ```

2. **Config file not loaded**
   - Check file path: `~/.config/ORLY/policy.json`
   - Check file permissions: `chmod 644 ~/.config/ORLY/policy.json`
   - Check JSON syntax: `jq . < ~/.config/ORLY/policy.json`

3. **Default policy overriding**
   - If `default_policy` is not set correctly
   - Kind whitelist is checked BEFORE default policy

### Issue 4: Privileged Events Visible to Everyone

**Symptoms:**
- You set `"privileged": true` for a kind
- Users can see events they're not involved in

**Solution:**

1. **Check authentication**: Users MUST authenticate via NIP-42
   ```bash
   # Force authentication
   export ORLY_AUTH_REQUIRED=true
   ```

2. **Check event has p-tags**: For users to be "involved", they must be:
   - The author (`ev.pubkey`), OR
   - In a p-tag: `["p", "user_pubkey_hex"]`

3. **Verify policy configuration**:
   ```json
   {
     "rules": {
       "4": {
         "privileged": true
       }
     }
   }
   ```

4. **Check logs**:
   ```bash
   sudo journalctl -u orly -f | grep -E "(privileged|IsPartyInvolved)"
   ```

### Issue 5: Script Not Running

**Symptoms:**
- You configured a script path
- Script is not being executed

**Solution:**

1. **Check script exists and is executable**:
   ```bash
   ls -l ~/.config/ORLY/policy.sh
   chmod +x ~/.config/ORLY/policy.sh
   ```

2. **Check policy manager is enabled**:
   ```bash
   echo $ORLY_POLICY_ENABLED  # Must be "true"
   ```

3. **Test script manually**:
   ```bash
   echo '{"id":"test","pubkey":"abc","created_at":1234567890,"kind":1,"content":"test","tags":[],"sig":"def"}' | ~/.config/ORLY/policy.sh
   ```

4. **Check script output format**: Must output JSONL:
   ```json
   {"id":"event_id","action":"accept","msg":"ok"}
   ```

5. **Check relay logs**:
   ```bash
   sudo journalctl -u orly -f | grep -E "(policy script|script)"
   ```

## Testing Your Policy Configuration

### Test 1: Kind Whitelist

```bash
# 1. Configure whitelist for kinds 1,3
cat > ~/.config/ORLY/policy.json <<EOF
{
  "kind": {
    "whitelist": [1, 3]
  },
  "default_policy": "allow"
}
EOF

# 2. Restart relay
sudo systemctl restart orly

# 3. Try to publish kind 1 (should succeed)
# 4. Try to publish kind 5 (should fail)
```

### Test 2: Write Access Control

```bash
# 1. Get your pubkey
YOUR_PUBKEY="$(nak key public)"

# 2. Configure write access
cat > ~/.config/ORLY/policy.json <<EOF
{
  "rules": {
    "10": {
      "write_allow": ["$YOUR_PUBKEY"]
    }
  },
  "default_policy": "allow"
}
EOF

# 3. Restart relay
sudo systemctl restart orly

# 4. Publish kind 10 with your key (should succeed)
# 5. Publish kind 10 with different key (should fail)
```

### Test 3: Read Access Control

```bash
# 1. Configure read access
cat > ~/.config/ORLY/policy.json <<EOF
{
  "rules": {
    "20": {
      "read_allow": ["$YOUR_PUBKEY"]
    }
  },
  "default_policy": "allow"
}
EOF

# 2. Enable authentication
export ORLY_AUTH_REQUIRED=true

# 3. Restart relay
sudo systemctl restart orly

# 4. Authenticate with your key and query kind 20 (should see events)
# 5. Query without auth or with different key (should not see events)
```

### Test 4: Privileged Events

```bash
# 1. Configure privileged
cat > ~/.config/ORLY/policy.json <<EOF
{
  "rules": {
    "4": {
      "privileged": true
    }
  },
  "default_policy": "allow"
}
EOF

# 2. Restart relay
sudo systemctl restart orly

# 3. Publish kind 4 with p-tag to Bob
# 4. Query as Bob (authenticated) - should see event
# 5. Query as Alice (authenticated) - should NOT see event
```

## Policy Evaluation Order

The policy system evaluates in this order:

1. **Global Rules** - Applied to ALL events first
2. **Kind Whitelist/Blacklist** - Checked before specific rules
3. **Specific Kind Rules** - Rule for the event's kind
4. **Script Validation** (if configured) - Custom script logic
5. **Default Policy** - Applied if no rule denies

```
Event Arrives
     ↓
Global Rules (max_age, size_limit, etc.)
     ↓ (if passes)
Kind Whitelist/Blacklist
     ↓ (if passes)
Specific Rule for Kind
     ├─ Script (if configured)
     ├─ write_allow/write_deny
     ├─ read_allow/read_deny
     ├─ privileged
     └─ Other rule criteria
     ↓ (if no rule found or passes)
Default Policy (allow or deny)
```

## Getting Your Pubkey in Hex Format

### From npub:

```bash
# Using nak
nak decode npub1abc...

# Using Python
python3 -c "from nostr_sdk import PublicKey; print(PublicKey.from_bech32('npub1abc...').to_hex())"
```

### From nsec:

```bash
# Using nak
nak key public nsec1abc...

# Using Python
python3 -c "from nostr_sdk import Keys; print(Keys.from_sk_str('nsec1abc...').public_key().to_hex())"
```

## Additional Configuration

### Combine with ACL System

Policy and ACL work together:

```bash
# Enable managed ACL + Policy
export ORLY_ACL_MODE=managed
export ORLY_POLICY_ENABLED=true
export ORLY_AUTH_REQUIRED=true
```

### Query Cache with Policy

Policy filtering happens BEFORE cache, so cached results respect policy:

```bash
export ORLY_QUERY_CACHE_SIZE_MB=512
export ORLY_QUERY_CACHE_MAX_AGE=5m
```

## Debugging Tips

### Enable Debug Logging

```bash
export ORLY_LOG_LEVEL=debug
sudo systemctl restart orly
sudo journalctl -u orly -f
```

### Test Policy in Isolation

Use the comprehensive test:

```bash
cd /home/mleku/src/next.orly.dev
CGO_ENABLED=0 go test -v ./pkg/policy -run TestPolicyDefinitionOfDone
```

### Check Policy Manager Status

Look for these log messages:

```
✅ "loaded policy configuration from ..."
✅ "policy script started: ..."
❌ "failed to load policy configuration: ..."
❌ "policy script does not exist at ..."
```

## Support

If you're still experiencing issues:

1. Check logs: `sudo journalctl -u orly -f | grep -i policy`
2. Verify configuration: `cat ~/.config/ORLY/policy.json | jq .`
3. Run tests: `go test -v ./pkg/policy`
4. File an issue: https://git.nostrdev.com/mleku/next.orly.dev/issues

## Summary

✅ **All requirements are implemented and working**
✅ **Comprehensive tests verify all scenarios**
✅ **Configuration examples provided**
✅ **Troubleshooting guide available**

The policy system is fully functional. Most issues are due to:
- Policy not enabled (`ORLY_POLICY_ENABLED=true`)
- Config file in wrong location (`~/.config/ORLY/policy.json`)
- Authentication not required for read restrictions
- Invalid JSON syntax in config file
