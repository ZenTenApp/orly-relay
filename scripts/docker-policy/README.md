# ORLY Policy Engine Docker Test

This directory contains a Docker-based test environment to verify that the `cs-policy.js` script is executed by the ORLY relay's policy engine when events are received.

## Test Structure

```
test-docker-policy/
├── Dockerfile           # Ubuntu 22.04.5 based image
├── docker-compose.yml   # Container orchestration
├── cs-policy.js         # Policy script that writes to a file
├── policy.json          # Policy configuration pointing to the script
├── env                  # Environment variables for ORLY
├── start.sh            # Container startup script
├── test-policy.sh      # Automated test runner
└── README.md           # This file
```

## What the Test Does

1. **Builds** an Ubuntu 22.04.5 Docker image with ORLY relay
2. **Configures** the policy engine with `cs-policy-daemon.js`
3. **Starts** the relay with policy engine enabled
4. **Publishes 2 events** to test write control (EVENT messages)
5. **Queries for those events** to test read control (REQ messages)
6. **Verifies** that:
   - Both events were published successfully
   - Events can be queried and retrieved
   - Policy script processed both write and read operations
   - Policy script logged to both file and relay log (stderr)
7. **Reports** detailed results with policy invocation counts

## How cs-policy-daemon.js Works

The policy script is a long-lived process that:
1. Reads events from stdin (one JSON event per line)
2. Processes each event and returns a JSON response to stdout
3. Logs debug information to:
   - `/home/orly/cs-policy-output.txt` (file output)
   - stderr (appears in relay log with prefix `[policy script /path]`)

**Key Features:**
- Logs event details including kind, ID, and access type (read/write)
- Writes debug output to stderr which appears in the relay log
- Returns JSON responses to stdout for policy decisions

## Quick Start

Run the automated test:

```bash
./scripts/docker-policy/test-policy.sh
```

## Policy Test Tool

The `policytest` tool is a command-line utility for testing policy enforcement:

```bash
# Test write control (EVENT messages)
./policytest -url ws://localhost:8777 -type event -kind 1

# Test read control (REQ messages)
./policytest -url ws://localhost:8777 -type req -kind 1

# Test both write and read control
./policytest -url ws://localhost:8777 -type both -kind 1

# Publish multiple events and query for them (full integration test)
./policytest -url ws://localhost:8777 -type publish-and-query -kind 1 -count 2
```

### Options

- `-url` - Relay WebSocket URL (default: `ws://127.0.0.1:3334`)
- `-type` - Test type:
  - `event` - Test write control only
  - `req` - Test read control only
  - `both` - Test write then read
  - `publish-and-query` - Publish events then query for them (full test)
- `-kind` - Event kind to test (default: `4678`)
- `-count` - Number of events to publish for `publish-and-query` (default: `2`)
- `-timeout` - Operation timeout (default: `20s`)

### Output

The `publish-and-query` test provides detailed output:

```
Publishing 2 events of kind 1...
Event 1/2 published successfully (id: a1b2c3d4...)
Event 2/2 published successfully (id: e5f6g7h8...)
PUBLISH: 2 accepted, 0 rejected out of 2 total

Querying for events of kind 1...
Query returned 2 events
QUERY: found 2/2 published events (total returned: 2)
SUCCESS: All published events were retrieved
```

## Manual Testing

### 1. Build and Start Container

```bash
cd /home/mleku/src/git.smesh.lol/orly
docker-compose -f test-docker-policy/docker-compose.yml up -d
```

### 2. Check Relay Logs

```bash
docker logs orly-policy-test -f
```

### 3. Send Test Event

```bash
# Using websocat
echo '["EVENT",{"id":"test123","pubkey":"4db2c42f3c02079dd6feae3f88f6c8693940a00ade3cc8e5d72050bd6e577cd5","created_at":'$(date +%s)',"kind":1,"tags":[],"content":"Test","sig":"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}]' | websocat ws://localhost:8777
```

### 4. Verify Output File

```bash
# Check if file exists
docker exec orly-policy-test test -f /home/orly/cs-policy-output.txt && echo "File exists!"

# View contents
docker exec orly-policy-test cat /home/orly/cs-policy-output.txt
```

### 5. Cleanup

```bash
# Stop container
docker-compose -f test-docker-policy/docker-compose.yml down

# Remove volumes
docker-compose -f test-docker-policy/docker-compose.yml down -v
```

## Troubleshooting

### Policy Script Not Running

Check if policy is enabled:
```bash
docker exec orly-policy-test cat /home/orly/env | grep POLICY
```

Check policy configuration:
```bash
docker exec orly-policy-test cat /home/orly/.config/ORLY/policy.json
```

### Node.js Issues

Verify Node.js is installed:
```bash
docker exec orly-policy-test node --version
```

Test the script manually:
```bash
docker exec orly-policy-test node /home/orly/cs-policy.js
docker exec orly-policy-test cat /home/orly/cs-policy-output.txt
```

### Relay Not Starting

View full logs:
```bash
docker logs orly-policy-test
```

Check if relay is listening:
```bash
docker exec orly-policy-test netstat -tlnp | grep 8777
```

## Expected Output

When successful, you should see:

```
=== Step 9: Publishing 2 events and querying for them ===

--- Publishing and querying events ---
Publishing 2 events of kind 1...
Event 1/2 published successfully (id: abc12345...)
Event 2/2 published successfully (id: def67890...)
PUBLISH: 2 accepted, 0 rejected out of 2 total

Querying for events of kind 1...
Query returned 2 events
QUERY: found 2/2 published events (total returned: 2)
SUCCESS: All published events were retrieved

=== Step 10: Checking relay logs ===
INFO [policy script /home/orly/cs-policy-daemon.js] [cs-policy] Policy script started
INFO [policy script /home/orly/cs-policy-daemon.js] [cs-policy] Processing event abc12345, kind: 1, access: write
INFO [policy script /home/orly/cs-policy-daemon.js] [cs-policy] Processing event def67890, kind: 1, access: write
INFO [policy script /home/orly/cs-policy-daemon.js] [cs-policy] Processing event abc12345, kind: 1, access: read
INFO [policy script /home/orly/cs-policy-daemon.js] [cs-policy] Processing event def67890, kind: 1, access: read

=== Step 12: Checking output file ===
✓ SUCCESS: cs-policy-output.txt file exists!

Output file contents:
1234567890123: Policy script started
1234567890456: Event ID: abc12345..., Kind: 1, Access: write
1234567890789: Event ID: def67890..., Kind: 1, Access: write
1234567891012: Event ID: abc12345..., Kind: 1, Access: read
1234567891234: Event ID: def67890..., Kind: 1, Access: read

Policy invocations summary:
  - Write operations (EVENT): 2 (expected: 2)
  - Read operations (REQ):    2 (expected: >=1)

✓ SUCCESS: Policy script processed both write and read operations!
  - Published 2 events (write control)
  - Queried events (read control)
```

The test verifies:
- **Write Control**: Policy script processes EVENT messages (2 publications)
- **Read Control**: Policy script processes REQ messages (query retrieves events)
- **Dual Logging**: Script output appears in both file and relay log (stderr)
- **Event Lifecycle**: Events are stored and can be retrieved

## Configuration Files

### env
Environment variables for ORLY relay:
- `ORLY_PORT=8777` - WebSocket port
- `ORLY_POLICY_ENABLED=true` - Enable policy engine
- `ORLY_LOG_LEVEL=debug` - Verbose logging

### policy.json
Policy configuration:
```json
{
  "script": "/home/orly/cs-policy.js"
}
```

Points to the policy script that will be executed for each event.
