# ORLY Policy System Usage Guide

The ORLY relay implements a comprehensive policy system that provides fine-grained control over event storage and retrieval. This guide explains how to configure and use the policy system to implement custom relay behavior.

## Overview

The policy system allows relay operators to:

- Control which events are stored and retrieved
- Implement custom validation logic
- Set size and age limits for events
- Define access control based on pubkeys
- Use scripts for complex policy rules
- Filter events by content, kind, or other criteria

## Quick Start

### 1. Enable the Policy System

Set the environment variable to enable policy checking:

```bash
export ORLY_POLICY_ENABLED=true
```

### 2. Create a Policy Configuration

Create the policy file at `~/.config/ORLY/policy.json`:

```json
{
  "default_policy": "allow",
  "global": {
    "max_age_of_event": 86400,
    "max_age_event_in_future": 300,
    "size_limit": 100000
  },
  "rules": {
    "1": {
      "description": "Text notes - basic validation",
      "max_age_of_event": 3600,
      "size_limit": 32000
    }
  }
}
```

### 3. Restart the Relay

```bash
# Restart your relay to load the policy
sudo systemctl restart orly
```

## Configuration Structure

### Top-Level Configuration

```json
{
  "default_policy": "allow|deny",
  "kind": {
    "whitelist": ["1", "3", "4"],
    "blacklist": []
  },
  "global": { ... },
  "rules": { ... }
}
```

### default_policy

Determines the fallback behavior when no specific rules apply:

- `"allow"`: Allow events unless explicitly denied (default)
- `"deny"`: Deny events unless explicitly allowed

### kind Filtering

Controls which event kinds are processed:

```json
"kind": {
  "whitelist": ["1", "3", "4", "9735"],
  "blacklist": []
}
```

- `whitelist`: Only these kinds are allowed (if present)
- `blacklist`: These kinds are denied (if present)
- Empty arrays allow all kinds

### Global Rules

Rules that apply to **all events** regardless of kind:

```json
"global": {
  "description": "Site-wide security rules",
  "write_allow": [],
  "write_deny": [],
  "read_allow": [],
  "read_deny": [],
  "size_limit": 100000,
  "content_limit": 50000,
  "max_age_of_event": 86400,
  "max_age_event_in_future": 300,
  "privileged": false
}
```

### Kind-Specific Rules

Rules that apply to specific event kinds:

```json
"rules": {
  "1": {
    "description": "Text notes",
    "write_allow": [],
    "write_deny": [],
    "read_allow": [],
    "read_deny": [],
    "size_limit": 32000,
    "content_limit": 10000,
    "max_age_of_event": 3600,
    "max_age_event_in_future": 60,
    "privileged": false
  }
}
```

## Policy Fields

### Access Control

#### write_allow / write_deny

Control who can publish events:

```json
{
  "write_allow": ["npub1allowed...", "npub1another..."],
  "write_deny": ["npub1blocked..."]
}
```

- `write_allow`: Only these pubkeys can write (empty = allow all)
- `write_deny`: These pubkeys cannot write

#### read_allow / read_deny

Control who can read events:

```json
{
  "read_allow": ["npub1trusted..."],
  "read_deny": ["npub1suspicious..."]
}
```

- `read_allow`: Only these pubkeys can read (empty = allow all)
- `read_deny`: These pubkeys cannot read

### Size Limits

#### size_limit

Maximum total event size in bytes:

```json
{
  "size_limit": 32000
}
```

Includes ID, pubkey, sig, tags, content, and metadata.

#### content_limit

Maximum content field size in bytes:

```json
{
  "content_limit": 10000
}
```

Only applies to the `content` field.

### Age Validation

#### max_age_of_event

Maximum age of events in seconds (prevents replay attacks):

```json
{
  "max_age_of_event": 3600
}
```

Events older than `current_time - max_age_of_event` are rejected.

#### max_age_event_in_future

Maximum time events can be in the future in seconds:

```json
{
  "max_age_event_in_future": 300
}
```

Events with `created_at > current_time + max_age_event_in_future` are rejected.

### Advanced Options

#### privileged

Require events to be authored by authenticated users or contain authenticated users in p-tags:

```json
{
  "privileged": true
}
```

Useful for private content that should only be accessible to specific users.

#### script

Path to a custom script for complex validation logic:

```json
{
  "script": "/path/to/custom-policy.sh"
}
```

See the script section below for details.

## Policy Scripts

For complex validation logic, use custom scripts that receive events via stdin and return decisions via stdout.

### Script Interface

**Input**: JSON event objects, one per line:

```json
{
  "id": "event_id",
  "pubkey": "author_pubkey",
  "kind": 1,
  "content": "Hello, world!",
  "tags": [["p", "recipient"]],
  "created_at": 1640995200,
  "sig": "signature"
}
```

Additional fields provided:
- `logged_in_pubkey`: Hex pubkey of authenticated user (if any)
- `ip_address`: Client IP address

**Output**: JSONL responses:

```json
{"id": "event_id", "action": "accept", "msg": ""}
{"id": "event_id", "action": "reject", "msg": "Blocked content"}
{"id": "event_id", "action": "shadowReject", "msg": ""}
```

### Actions

- `accept`: Store/retrieve the event normally
- `reject`: Reject with OK=false and message
- `shadowReject`: Accept with OK=true but don't store (useful for spam filtering)

### Example Scripts

#### Bash Script

```bash
#!/bin/bash
while read -r line; do
    if [[ -n "$line" ]]; then
        event_id=$(echo "$line" | jq -r '.id')

        # Check for spam content
        if echo "$line" | jq -r '.content' | grep -qi "spam"; then
            echo "{\"id\":\"$event_id\",\"action\":\"reject\",\"msg\":\"Spam detected\"}"
        else
            echo "{\"id\":\"$event_id\",\"action\":\"accept\",\"msg\":\"\"}"
        fi
    fi
done
```

#### Python Script

```python
#!/usr/bin/env python3
import json
import sys

def process_event(event):
    event_id = event.get('id', '')
    content = event.get('content', '')
    pubkey = event.get('pubkey', '')
    logged_in = event.get('logged_in_pubkey', '')

    # Block spam
    if 'spam' in content.lower():
        return {
            'id': event_id,
            'action': 'reject',
            'msg': 'Content contains spam'
        }

    # Require authentication for certain content
    if 'private' in content.lower() and not logged_in:
        return {
            'id': event_id,
            'action': 'reject',
            'msg': 'Authentication required'
        }

    return {
        'id': event_id,
        'action': 'accept',
        'msg': ''
    }

for line in sys.stdin:
    if line.strip():
        try:
            event = json.loads(line)
            response = process_event(event)
            print(json.dumps(response))
            sys.stdout.flush()
        except json.JSONDecodeError:
            continue
```

### Script Configuration

Place scripts in a secure location and reference them in policy:

```json
{
  "rules": {
    "1": {
      "script": "/etc/orly/policy/text-note-policy.py",
      "description": "Custom validation for text notes"
    }
  }
}
```

Ensure scripts are executable and have appropriate permissions.

### Script Requirements and Best Practices

#### Critical Requirements

**1. Output Only JSON to stdout**

Scripts MUST write ONLY JSON responses to stdout. Any other output (debug messages, logs, etc.) will break the JSONL protocol and cause errors.

**Debug Output**: Use stderr for debug messages - all stderr output from policy scripts is automatically logged to the relay log with the prefix `[policy script /path/to/script]`.

```javascript
// ❌ WRONG - This will cause "broken pipe" errors
console.log("Policy script starting...");  // This goes to stdout!
console.log(JSON.stringify(response));     // Correct

// ✅ CORRECT - Use stderr or file for debug output
console.error("Policy script starting...");  // This goes to stderr (appears in relay log)
fs.appendFileSync('/tmp/policy.log', 'Starting...\n');  // This goes to file (OK)
console.log(JSON.stringify(response));      // Stdout for JSON only
```

**2. Flush stdout After Each Response**

Always flush stdout after writing a response to ensure immediate delivery:

```python
# Python
print(json.dumps(response))
sys.stdout.flush()  # Critical!
```

```javascript
// Node.js (usually automatic, but can be forced)
process.stdout.write(JSON.stringify(response) + '\n');
```

**3. Run as a Long-Lived Process**

Scripts should run continuously, reading from stdin in a loop. They should NOT:
- Exit after processing one event
- Use batch processing
- Close stdin/stdout prematurely

```javascript
// ✅ CORRECT - Long-lived process
const readline = require('readline');
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

rl.on('line', (line) => {
  const event = JSON.parse(line);
  const response = processEvent(event);
  console.log(JSON.stringify(response));
});
```

**4. Handle Errors Gracefully**

Always catch errors and return a valid JSON response:

```javascript
rl.on('line', (line) => {
  try {
    const event = JSON.parse(line);
    const response = processEvent(event);
    console.log(JSON.stringify(response));
  } catch (err) {
    // Log to stderr or file, not stdout!
    console.error(`Error: ${err.message}`);

    // Return reject response
    console.log(JSON.stringify({
      id: '',
      action: 'reject',
      msg: 'Policy script error'
    }));
  }
});
```

**5. Response Format**

Every response MUST include these fields:

```json
{
  "id": "event_id",      // Must match input event ID
  "action": "accept",    // Must be: accept, reject, or shadowReject
  "msg": ""              // Required (can be empty string)
}
```

#### Common Issues and Solutions

**Broken Pipe Error**

```
ERROR: policy script /path/to/script.js stdin closed (broken pipe)
```

**Causes:**
- Script exited prematurely
- Script wrote non-JSON output to stdout
- Script crashed or encountered an error
- Script closed stdin/stdout incorrectly

**Solutions:**
1. Remove ALL `console.log()` statements except JSON responses
2. Use `console.error()` or log files for debugging
3. Add error handling to catch and log exceptions
4. Ensure script runs continuously (doesn't exit)

**Response Timeout**

```
WARN: policy script /path/to/script.js response timeout - script may not be responding correctly
```

**Causes:**
- Script not flushing stdout
- Script processing taking > 5 seconds
- Script not responding to input
- Non-JSON output consuming a response slot

**Solutions:**
1. Add `sys.stdout.flush()` (Python) after each response
2. Optimize processing logic to be faster
3. Check that script is reading from stdin correctly
4. Remove debug output from stdout

**Invalid JSON Response**

```
ERROR: failed to parse policy response from /path/to/script.js
WARN: policy script produced non-JSON output on stdout: "Debug message"
```

**Solutions:**
1. Validate JSON before outputting
2. Use a JSON library, don't build strings manually
3. Move debug output to stderr or files

#### Testing Your Script

Before deploying, test your script:

```bash
# 1. Test basic functionality
echo '{"id":"test123","pubkey":"abc","kind":1,"content":"test","tags":[],"created_at":1234567890,"sig":"def"}' | node policy-script.js

# 2. Check for non-JSON output
echo '{"id":"test123","pubkey":"abc","kind":1,"content":"test","tags":[],"created_at":1234567890,"sig":"def"}' | node policy-script.js 2>/dev/null | jq .

# 3. Test error handling
echo 'invalid json' | node policy-script.js
```

Expected output (valid JSON only):
```json
{"id":"test123","action":"accept","msg":""}
```

#### Node.js Example (Complete)

```javascript
#!/usr/bin/env node

const readline = require('readline');

// Use stderr for debug logging - appears in relay log automatically
function debug(msg) {
  console.error(`[policy] ${msg}`);
}

// Create readline interface
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

debug('Policy script started');

// Process each event
rl.on('line', (line) => {
  try {
    const event = JSON.parse(line);
    debug(`Processing event ${event.id}, kind: ${event.kind}, access: ${event.access_type}`);

    // Your policy logic here
    const action = shouldAccept(event) ? 'accept' : 'reject';

    if (action === 'reject') {
      debug(`Rejected event ${event.id}: policy violation`);
    }

    // ONLY JSON to stdout
    console.log(JSON.stringify({
      id: event.id,
      action: action,
      msg: action === 'reject' ? 'Policy rejected' : ''
    }));

  } catch (err) {
    debug(`Error: ${err.message}`);

    // Still return valid JSON
    console.log(JSON.stringify({
      id: '',
      action: 'reject',
      msg: 'Policy script error'
    }));
  }
});

rl.on('close', () => {
  debug('Policy script stopped');
});

function shouldAccept(event) {
  // Your policy logic
  if (event.content.toLowerCase().includes('spam')) {
    return false;
  }

  // Different logic for read vs write
  if (event.access_type === 'write') {
    // Write control logic
    return event.content.length < 10000;
  } else if (event.access_type === 'read') {
    // Read control logic
    return true; // Allow all reads
  }

  return true;
}
```

**Relay Log Output Example:**
```
INFO [policy script /home/orly/.config/ORLY/policy.js] [policy] Policy script started
INFO [policy script /home/orly/.config/ORLY/policy.js] [policy] Processing event abc123, kind: 1, access: write
INFO [policy script /home/orly/.config/ORLY/policy.js] [policy] Processing event def456, kind: 1, access: read
```

#### Event Fields

Scripts receive additional context fields:

```json
{
  "id": "event_id",
  "pubkey": "author_pubkey",
  "kind": 1,
  "content": "Event content",
  "tags": [],
  "created_at": 1234567890,
  "sig": "signature",
  "logged_in_pubkey": "authenticated_user_pubkey",
  "ip_address": "127.0.0.1",
  "access_type": "read"
}
```

**access_type values:**
- `"write"`: Event is being stored (EVENT message)
- `"read"`: Event is being retrieved (REQ message)

Use this to implement different policies for reads vs writes.

## Policy Evaluation Order

Events are evaluated in this order:

1. **Global Rules** - Applied first to all events
2. **Kind Filtering** - Whitelist/blacklist check
3. **Kind-specific Rules** - Rules for the event's kind
4. **Script Rules** - Custom script logic (if configured)
5. **Default Policy** - Fallback behavior

The first rule that makes a decision (allow/deny) stops evaluation.

## Event Processing Integration

### Write Operations (EVENT)

When `ORLY_POLICY_ENABLED=true`, each incoming EVENT is checked:

```go
// Pseudo-code for policy integration
func handleEvent(event *Event, client *Client) {
    decision := policy.CheckPolicy("write", event, client.Pubkey, client.IP)
    if decision.Action == "reject" {
        client.SendOK(event.ID, false, decision.Message)
        return
    }
    if decision.Action == "shadowReject" {
        client.SendOK(event.ID, true, "")
        return
    }
    // Store event
    storeEvent(event)
    client.SendOK(event.ID, true, "")
}
```

### Read Operations (REQ)

Events returned in REQ responses are filtered:

```go
func handleReq(filter *Filter, client *Client) {
    events := queryEvents(filter)
    filteredEvents := []Event{}

    for _, event := range events {
        decision := policy.CheckPolicy("read", &event, client.Pubkey, client.IP)
        if decision.Action != "reject" {
            filteredEvents = append(filteredEvents, event)
        }
    }

    sendEvents(client, filteredEvents)
}
```

## Common Use Cases

### Basic Spam Filtering

```json
{
  "global": {
    "max_age_of_event": 86400,
    "size_limit": 100000
  },
  "rules": {
    "1": {
      "script": "/etc/orly/scripts/spam-filter.sh",
      "max_age_of_event": 3600,
      "size_limit": 32000
    }
  }
}
```

### Private Relay

```json
{
  "default_policy": "deny",
  "global": {
    "write_allow": ["npub1trusted1...", "npub1trusted2..."],
    "read_allow": ["npub1trusted1...", "npub1trusted2..."]
  }
}
```

### Content Moderation

```json
{
  "rules": {
    "1": {
      "script": "/etc/orly/scripts/content-moderation.py",
      "description": "AI-powered content moderation"
    }
  }
}
```

### Rate Limiting

```json
{
  "global": {
    "script": "/etc/orly/scripts/rate-limiter.sh"
  }
}
```

### Follows-Based Access

Combined with ACL system:

```bash
export ORLY_ACL_MODE=follows
export ORLY_ADMINS=npub1admin1...,npub1admin2...
export ORLY_POLICY_ENABLED=true
```

## Monitoring and Debugging

### Log Messages

Policy decisions are logged:

```
policy allowed event <id>
policy rejected event <id>: reason
policy filtered out event <id> for read access
```

### Script Health

Script failures are logged:

```
policy rule for kind <N> is inactive (script not running), falling back to default policy (allow)
policy rule for kind <N> failed (script processing error: timeout), falling back to default policy (allow)
```

### Testing Policies

Use the policy test tools:

```bash
# Test policy with sample events
./scripts/run-policy-test.sh

# Test policy filter integration
./scripts/run-policy-filter-test.sh
```

### Debugging Scripts

Test scripts independently:

```bash
# Test script with sample event
echo '{"id":"test","kind":1,"content":"test message"}' | ./policy-script.sh

# Expected output:
# {"id":"test","action":"accept","msg":""}
```

## Performance Considerations

### Script Performance

- Scripts run synchronously and can block event processing
- Keep script logic efficient (< 100ms per event)
- Consider using `shadowReject` for non-blocking filtering
- Scripts should handle malformed input gracefully

### Memory Usage

- Policy configuration is loaded once at startup
- Scripts are kept running for performance
- Large configurations may impact startup time

### Scaling

- For high-throughput relays, prefer built-in policy rules over scripts
- Use script timeouts to prevent hanging
- Monitor script performance and resource usage

## Security Considerations

### Script Security

- Scripts run with relay process privileges
- Validate all inputs in scripts
- Use secure file permissions for policy files
- Regularly audit custom scripts

### Access Control

- Test policy rules thoroughly before production use
- Use `privileged: true` for sensitive content
- Combine with authentication requirements
- Log policy violations for monitoring

### Data Validation

- Age validation prevents replay attacks
- Size limits prevent DoS attacks
- Content validation prevents malicious payloads

## Troubleshooting

### Policy Not Loading

Check file permissions and path:

```bash
ls -la ~/.config/ORLY/policy.json
cat ~/.config/ORLY/policy.json
```

### Scripts Not Working

Verify script is executable and working:

```bash
ls -la /path/to/script.sh
./path/to/script.sh < /dev/null
```

### Unexpected Behavior

Enable debug logging:

```bash
export ORLY_LOG_LEVEL=debug
```

Check logs for policy decisions and errors.

### Common Issues

1. **Script timeouts**: Increase script timeouts or optimize script performance
2. **Memory issues**: Reduce script memory usage or use built-in rules
3. **Permission errors**: Fix file permissions on policy files and scripts
4. **Configuration errors**: Validate JSON syntax and field names

## Dynamic Policy Configuration via Kind 12345

Policy administrators can update the relay policy dynamically by publishing kind 12345 events. This enables runtime policy changes without relay restarts.

### Enabling Dynamic Policy Updates

1. Add yourself as a policy admin in the initial policy.json:

```json
{
  "default_policy": "allow",
  "policy_admins": ["YOUR_HEX_PUBKEY_HERE"],
  "policy_follow_whitelist_enabled": false
}
```

2. Ensure policy is enabled:

```bash
export ORLY_POLICY_ENABLED=true
```

### Publishing a Policy Update

Send a kind 12345 event with the new policy configuration as JSON content:

```json
{
  "kind": 12345,
  "content": "{\"default_policy\": \"deny\", \"kind\": {\"whitelist\": [1,3,7]}, \"policy_admins\": [\"YOUR_HEX_PUBKEY\"]}",
  "tags": [],
  "created_at": 1234567890
}
```

### Policy Admin Follow List Whitelisting

When `policy_follow_whitelist_enabled` is `true`, the relay automatically grants access to all pubkeys followed by policy admins.

```json
{
  "policy_admins": ["ADMIN_PUBKEY_HEX"],
  "policy_follow_whitelist_enabled": true
}
```

- When an admin updates their follow list (kind 3), the relay automatically refreshes the whitelist
- The `write_allow_follows` rule option grants both read AND write access to follows
- This enables community-based access control without manual pubkey management

### Security Considerations

- Only pubkeys listed in `policy_admins` can update the policy
- Policy updates are validated before applying (invalid JSON or pubkeys are rejected)
- Failed updates preserve the existing policy (no corruption)
- All policy updates are logged for audit purposes

## Testing the Policy System

### Edge Cases Discovered During Testing

When writing tests for the policy system, the following edge cases were discovered:

1. **Config File Requirement**: `NewWithManager()` with `enabled=true` requires the XDG config file (`~/.config/APP_NAME/policy.json`) to exist before initialization. Tests must create this file first.

2. **Error Message Format**: Validation errors use underscores in field names (e.g., `invalid policy_admin pubkey`) - tests should match this exact format.

3. **Binary Tag Storage**: When comparing pubkeys from e/p tags, always use `tag.ValueHex()` instead of `tag.Value()` due to binary optimization.

4. **Concurrent Access**: The policy system uses `sync.RWMutex` for thread-safe access to the follows list during updates.

5. **Message Processing Pause**: Policy updates pause message processing with an exclusive lock to ensure atomic updates.

### Running Policy Tests

```bash
# Run all policy package tests
CGO_ENABLED=0 go test -v ./pkg/policy/...

# Run handler tests for kind 12345
CGO_ENABLED=0 go test -v ./app/... -run "PolicyConfig|PolicyAdmin"

# Run specific test categories
CGO_ENABLED=0 go test -v ./pkg/policy/... -run "ValidateJSON|Reload|Follow|TagValidation"
```

## Advanced Configuration

### Multiple Policies

Use different policies for different relay instances:

```bash
# Production relay
export ORLY_APP_NAME=production
# Policy at ~/.config/production/policy.json

# Staging relay
export ORLY_APP_NAME=staging
# Policy at ~/.config/staging/policy.json
```

### Dynamic Policies

Policies can be updated without restart by modifying the JSON file. Changes take effect immediately for new events.

### Integration with External Systems

Scripts can integrate with external services:

```python
import requests

def check_external_service(content):
    response = requests.post('http://moderation-service:8080/check',
                           json={'content': content}, timeout=5)
    return response.json().get('approved', False)
```

## Examples Repository

See the `docs/` directory for complete examples:

- `example-policy.json`: Complete policy configuration
- `example-policy.sh`: Sample policy script
- Various test scripts in `scripts/`

## Support

For issues with policy configuration:

1. Check the logs for error messages
2. Validate your JSON configuration
3. Test scripts independently
4. Review the examples in `docs/`
5. Check file permissions and paths

## Migration from Other Systems

### From Simple Filtering

Replace simple filters with policy rules:

```json
// Before: Simple size limit
// After: Policy-based size limit
{
  "global": {
    "size_limit": 50000
  }
}
```

### From Custom Code

Migrate custom validation logic to policy scripts:

```json
{
  "rules": {
    "1": {
      "script": "/etc/orly/scripts/custom-validation.py"
    }
  }
}
```

The policy system provides a flexible, maintainable way to implement complex relay behavior while maintaining performance and security.




