# Feature Request and Bug Report Protocol

This document describes how to submit effective bug reports and feature requests for ORLY relay. Following these guidelines helps maintainers understand and resolve issues quickly.

## Before Submitting

1. **Search existing issues** - Your issue may already be reported or discussed
2. **Check documentation** - Review `CLAUDE.md`, `docs/`, and `pkg/*/README.md` files
3. **Verify with latest version** - Ensure the issue exists in the current release
4. **Test with default configuration** - Rule out configuration-specific problems

## Bug Reports

### Required Information

**Title**: Concise summary of the problem
- Good: "Kind 3 events with 8000+ follows truncated on save"
- Bad: "Events not saving" or "Bug in database"

**Environment**:
```
ORLY version: (output of ./orly version)
OS: (e.g., Ubuntu 24.04, macOS 14.2)
Go version: (output of go version)
Database backend: (badger/neo4j/wasmdb)
```

**Configuration** (relevant settings only):
```bash
ORLY_DB_TYPE=badger
ORLY_POLICY_ENABLED=true
# Include any non-default settings
```

**Steps to Reproduce**:
1. Start relay with configuration X
2. Connect client and send event Y
3. Query for event with filter Z
4. Observe error/unexpected behavior

**Expected Behavior**: What should happen

**Actual Behavior**: What actually happens

**Logs**: Include relevant log output with `ORLY_LOG_LEVEL=debug` or `trace`

### Minimal Reproduction

The most effective bug reports include a minimal reproduction case:

```bash
# Example: Script that demonstrates the issue
export ORLY_LOG_LEVEL=debug
./orly &
sleep 2

# Send problematic event
echo '["EVENT", {...}]' | websocat ws://localhost:3334

# Show the failure
echo '["REQ", "test", {"kinds": [1]}]' | websocat ws://localhost:3334
```

Or provide a failing test case:

```go
func TestReproduceBug(t *testing.T) {
    // Setup
    db := setupTestDB(t)

    // This should work but fails
    event := createTestEvent(kind, content)
    err := db.SaveEvent(ctx, event)
    require.NoError(t, err)

    // Query returns unexpected result
    results, err := db.QueryEvents(ctx, filter)
    assert.Len(t, results, 1) // Fails: got 0
}
```

## Feature Requests

### Required Information

**Title**: Clear description of the feature
- Good: "Add WebSocket compression support (permessage-deflate)"
- Bad: "Make it faster" or "New feature idea"

**Problem Statement**: What problem does this solve?
```
Currently, clients with high-latency connections experience slow sync times
because event data is transmitted uncompressed. A typical session transfers
50MB of JSON that could be reduced to ~10MB with compression.
```

**Proposed Solution**: How should it work?
```
Add optional permessage-deflate WebSocket extension support:
- New config: ORLY_WS_COMPRESSION=true
- Negotiate compression during WebSocket handshake
- Apply to messages over configurable threshold (default 1KB)
```

**Use Case**: Who benefits and how?
```
- Mobile clients on cellular connections
- Users syncing large follow lists
- Relays with bandwidth constraints
```

**Alternatives Considered** (optional):
```
- Application-level compression: Rejected because it requires client changes
- HTTP/2: Not applicable for WebSocket connections
```

### Implementation Notes (optional)

If you have implementation ideas:

```
Suggested approach:
1. Add compression config to app/config/config.go
2. Modify gorilla/websocket upgrader in app/handle-websocket.go
3. Add compression threshold check before WriteMessage()

Reference: gorilla/websocket has built-in permessage-deflate support
```

## What Makes Reports Effective

**Do**:
- Be specific and factual
- Include version numbers and exact error messages
- Provide reproducible steps
- Attach relevant logs (redact sensitive data)
- Link to related issues or discussions
- Respond to follow-up questions promptly

**Avoid**:
- Vague descriptions ("it doesn't work")
- Multiple unrelated issues in one report
- Assuming the cause without evidence
- Demanding immediate fixes
- Duplicating existing issues

## Issue Labels

When applicable, suggest appropriate labels:

| Label | Use When |
|-------|----------|
| `bug` | Something isn't working as documented |
| `enhancement` | New feature or improvement |
| `performance` | Speed or resource usage issue |
| `documentation` | Docs are missing or incorrect |
| `question` | Clarification needed (not a bug) |
| `good first issue` | Suitable for new contributors |

## Response Expectations

- **Acknowledgment**: Within a few days
- **Triage**: Issue labeled and prioritized
- **Resolution**: Depends on complexity and priority

Complex features may require discussion before implementation. Bug fixes for critical issues are prioritized.

## Following Up

If your issue hasn't received attention:

1. **Check issue status** - It may be labeled or assigned
2. **Add new information** - If you've discovered more details
3. **Politely bump** - A single follow-up comment after 2 weeks is appropriate
4. **Consider contributing** - PRs that fix bugs or implement features are welcome

## Contributing Fixes

If you want to fix a bug or implement a feature yourself:

1. Comment on the issue to avoid duplicate work
2. Follow the coding patterns in `CLAUDE.md`
3. Include tests for your changes
4. Keep PRs focused on a single issue
5. Reference the issue number in your PR

## Security Issues

**Do not report security vulnerabilities in public issues.**

For security-sensitive bugs:
- Contact maintainers directly
- Provide detailed reproduction steps privately
- Allow reasonable time for a fix before disclosure

## Examples

### Good Bug Report

```markdown
## WebSocket disconnects after 60 seconds of inactivity

**Environment**:
- ORLY v0.34.5
- Ubuntu 22.04
- Go 1.25.3
- Badger backend

**Steps to Reproduce**:
1. Connect to relay: `websocat ws://localhost:3334`
2. Send subscription: `["REQ", "test", {"kinds": [1], "limit": 1}]`
3. Wait 60 seconds without sending messages
4. Observe connection closed

**Expected**: Connection remains open (Nostr relays should maintain persistent connections)

**Actual**: Connection closed with code 1000 after exactly 60 seconds

**Logs** (ORLY_LOG_LEVEL=debug):
```
1764783029014485🔎 client timeout, closing connection /app/handle-websocket.go:142
```

**Possible Cause**: May be related to read deadline not being extended on subscription activity
```

### Good Feature Request

```markdown
## Add rate limiting per pubkey

**Problem**:
A single pubkey can flood the relay with events, consuming storage and
bandwidth. Currently there's no way to limit per-author submission rate.

**Proposed Solution**:
Add configurable rate limiting:
```bash
ORLY_RATE_LIMIT_EVENTS_PER_MINUTE=60
ORLY_RATE_LIMIT_BURST=10
```

When exceeded, return OK false with "rate-limited" message per NIP-20.

**Use Case**:
- Public relays protecting against spam
- Community relays with fair-use policies
- Paid relays enforcing subscription tiers

**Alternatives Considered**:
- IP-based limiting: Ineffective because users share IPs and use VPNs
- Global limiting: Punishes all users for one bad actor
```
