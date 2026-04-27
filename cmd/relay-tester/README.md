# relay-tester

A comprehensive command-line tool for testing Nostr relay implementations against the NIP-01 specification and related NIPs. This tool validates relay compliance and helps developers ensure their implementations work correctly.

## Features

- **Comprehensive Test Coverage**: Tests all major Nostr protocol features
- **NIP Compliance Validation**: Ensures relays follow Nostr Improvement Proposals
- **Flexible Testing Options**: Run all tests or focus on specific areas
- **Multiple Output Formats**: Human-readable or JSON output for automation
- **Dependency-Aware Testing**: Tests run in correct order with proper dependencies
- **Integration with Build Pipeline**: Suitable for CI/CD integration

## Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd git.smesh.lol/orly

# Build the relay-tester
go build -o relay-tester ./cmd/relay-tester

# Optionally install globally
sudo mv relay-tester /usr/local/bin/
```

### Using the Install Script

```bash
# Use the provided installation script
./scripts/relaytester-install.sh
```

## Usage

```bash
relay-tester -url <relay-url> [options]
```

## Options

| Option | Description | Default |
|--------|-------------|---------|
| `-url` | **Required.** Relay websocket URL (e.g., `ws://127.0.0.1:3334` or `wss://relay.example.com`) | - |
| `-test <name>` | Run a specific test by name | Run all tests |
| `-json` | Output results in JSON format for automation | Human-readable |
| `-v` | Verbose output (shows additional info for each test) | false |
| `-list` | List all available tests and exit | false |
| `-timeout <duration>` | Timeout for individual test operations | 30s |

## Examples

### Basic Testing

Run all tests against a local relay:
```bash
relay-tester -url ws://127.0.0.1:3334
```

Run all tests with verbose output:
```bash
relay-tester -url ws://127.0.0.1:3334 -v
```

### Specific Test Execution

Run a specific test:
```bash
relay-tester -url ws://127.0.0.1:3334 -test "Publishes basic event"
```

List all available tests:
```bash
relay-tester -list
```

### Output Formats

Output results as JSON for automation:
```bash
relay-tester -url ws://127.0.0.1:3334 -json
```

### Remote Relay Testing

Test a remote relay:
```bash
relay-tester -url wss://relay.damus.io
```

Test with custom timeout:
```bash
relay-tester -url ws://127.0.0.1:3334 -timeout 60s
```

## Exit Codes

- `0`: All required tests passed - relay is compliant
- `1`: One or more required tests failed, or an error occurred
- `2`: Invalid command-line arguments

## Test Categories

The relay-tester runs comprehensive tests covering:

### Core Protocol (NIP-01)

- **Basic Event Operations**:
  - Publishing events
  - Finding events by ID, author, kind, and tags
  - Event retrieval and validation

- **Filtering**:
  - Time range filters (`since`, `until`)
  - Limit and pagination
  - Multiple concurrent filters
  - Scrape queries for bulk data

- **Event Types**:
  - Regular events (kind 1+)
  - Replaceable events (kinds 0, 3, etc.)
  - Parameterized replaceable events (addressable events with `d` tags)
  - Ephemeral events (kinds 20000+)

### Extended Protocol Features

- **Event Deletion (NIP-09)**: Testing deletion event handling
- **EOSE Handling**: Proper "end of stored events" signaling
- **Event Validation**: Signature verification and ID hash validation
- **JSON Compliance**: NIP-01 JSON escape sequences and formatting

### Authentication & Access Control

- **Authentication Testing**: NIP-42 AUTH command support
- **Access Control**: Testing relay-specific access rules
- **Rate Limiting**: Basic rate limit validation

## Test Results Interpretation

### Successful Tests

```
✅ Publishes basic event
✅ Finds event by ID
✅ Filters events by time range
```

### Failed Tests

```
❌ Publishes basic event: timeout waiting for OK
❌ Filters events by time range: unexpected EOSE timing
```

### JSON Output Format

```json
{
  "relay_url": "ws://127.0.0.1:3334",
  "timestamp": "2024-01-01T12:00:00Z",
  "tests_run": 25,
  "tests_passed": 23,
  "tests_failed": 2,
  "results": [
    {
      "name": "Publishes basic event",
      "status": "passed",
      "duration": "0.123s"
    },
    {
      "name": "Filters events by time range",
      "status": "failed",
      "error": "unexpected EOSE timing",
      "duration": "0.456s"
    }
  ]
}
```

## Integration with Build Scripts

The relay-tester is integrated with the project's testing scripts:

```bash
# Test relay with default configuration
./scripts/relaytester-test.sh

# Test relay with policy enabled
ORLY_POLICY_ENABLED=true ./scripts/relaytester-test.sh

# Test relay with ACL enabled
ORLY_ACL_MODE=follows ./scripts/relaytester-test.sh
```

## Testing Strategy

### Development Testing

During development, run tests frequently:

```bash
# Quick test against local relay
go run ./cmd/relay-tester -url ws://127.0.0.1:3334

# Test specific functionality
go run ./cmd/relay-tester -url ws://127.0.0.1:3334 -test "EOSE handling"
```

### CI/CD Integration

For automated testing in CI/CD pipelines:

```bash
# JSON output for parsing
relay-tester -url $RELAY_URL -json > test_results.json

# Check exit code
if [ $? -eq 0 ]; then
  echo "All tests passed!"
else
  echo "Some tests failed"
  cat test_results.json
  exit 1
fi
```

### Performance Testing

The relay-tester can be combined with performance testing:

```bash
# Start relay
./orly &
RELAY_PID=$!

# Run compliance tests
relay-tester -url ws://127.0.0.1:3334

# Run performance tests
./scripts/runtests.sh

# Cleanup
kill $RELAY_PID
```

## Troubleshooting

### Common Issues

1. **Connection Refused**: Ensure relay is running and accessible
2. **Timeout Errors**: Increase timeout with `-timeout` flag
3. **Authentication Required**: Some relays require NIP-42 AUTH
4. **WebSocket Errors**: Check firewall and network configuration

### Debug Output

Use verbose mode for detailed information:

```bash
relay-tester -url ws://127.0.0.1:3334 -v
```

### Test Dependencies

Tests are run in dependency order. If a foundational test fails, subsequent tests may also fail. Always fix basic event publishing before debugging complex filtering.

## Development

### Running Tests

```bash
# Run relay-tester unit tests
go test ./cmd/relay-tester

# Run all tests including relay-tester
go test ./...

# Run with coverage
go test -cover ./cmd/relay-tester
```

### Adding New Tests

1. Add test case to the test suite
2. Update test dependencies if needed
3. Ensure proper error handling
4. Update documentation

## License

This tool is part of the git.smesh.lol/orly project and follows the same licensing terms.

