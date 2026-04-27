# Nostr Event Aggregator

A comprehensive program that searches for all events related to a specific npub across multiple Nostr relays and outputs them in JSONL format to stdout. The program finds both events authored by the user and events that mention the user in "p" tags. It features dynamic relay discovery from relay list events and progressive backward time-based fetching for complete historical data collection.

## Usage

```bash
go run main.go -key <nsec|npub> [-since <timestamp>] [-until <timestamp>] [-filter <file>] [-output <file>]
```

Where:
- `<nsec|npub>` is either a bech32-encoded Nostr private key (nsec1...) or public key (npub1...)
- `<timestamp>` is a Unix timestamp (seconds since epoch) - optional
- `<file>` is a file path for bloom filter input/output - optional

### Parameters

- **`-key`**: Required. The bech32-encoded Nostr key to search for events
  - **nsec**: Private key (enables authentication to relays that require it)
  - **npub**: Public key (authentication disabled)
- **`-since`**: Optional. Start timestamp (Unix seconds). Only events after this time
- **`-until`**: Optional. End timestamp (Unix seconds). Only events before this time  
- **`-filter`**: Optional. File containing base64-encoded bloom filter from previous runs
- **`-output`**: Optional. Output file for events (default: stdout)

### Authentication

When using an **nsec** (private key), the aggregator will:
- Derive the public key from the private key for event searching
- Attempt to authenticate to relays that require it (NIP-42)
- Continue working even if authentication fails on some relays
- Log authentication success/failure for each relay

When using an **npub** (public key), the aggregator will:
- Search for events using the provided public key
- Skip authentication (no private key available)
- Work with public relays that don't require authentication

### Behavior

- **Without `-filter`**: Creates new bloom filter, outputs to stdout or truncates output file
- **With `-filter`**: Loads existing bloom filter, automatically appends to output file
- **Bloom filter output**: Always written to stderr with timestamp information and base64 data

## Examples

### Basic Usage

```bash
# Get all events related to a user using public key (no authentication)
go run main.go -key npub1234567890abcdef...

# Get all events related to a user using private key (with authentication)
go run main.go -key nsec1234567890abcdef...

# Get events related to a user since January 1, 2022
go run main.go -key npub1234567890abcdef... -since 1640995200

# Get events related to a user between two dates
go run main.go -key npub1234567890abcdef... -since 1640995200 -until 1672531200

# Get events related to a user until December 31, 2022
go run main.go -key npub1234567890abcdef... -until 1672531200
```

### Incremental Collection with Bloom Filter

```bash
# First run: Collect initial events and save bloom filter (using npub)
go run main.go -key npub1234567890abcdef... -since 1640995200 -until 1672531200 -output events.jsonl 2>bloom_filter.txt

# Second run: Continue from where we left off, append new events (using nsec for auth)
go run main.go -key nsec1234567890abcdef... -since 1672531200 -until 1704067200 -filter bloom_filter.txt -output events.jsonl 2>bloom_filter_updated.txt

# Third run: Collect even more recent events
go run main.go -key nsec1234567890abcdef... -since 1704067200 -filter bloom_filter_updated.txt -output events.jsonl 2>bloom_filter_final.txt
```

### Output Redirection

```bash
# Events to file, bloom filter to stderr (visible in terminal)
go run main.go -key npub1... -output events.jsonl

# Events to file, bloom filter to separate file
go run main.go -key npub1... -output events.jsonl 2>bloom_filter.txt

# Events to stdout, bloom filter to file (useful for piping events)
go run main.go -key npub1... 2>bloom_filter.txt | jq .

# Using nsec for authentication to access private relays
go run main.go -key nsec1... -output events.jsonl 2>bloom_filter.txt
```

## Features

### Core Functionality
- **Comprehensive event discovery**: Finds both events authored by the user and events that mention the user
- **Dynamic relay discovery**: Automatically discovers and connects to new relays from relay list events (kind 10002)
- **Progressive backward fetching**: Systematically collects historical data in time-based batches
- **Triple filter approach**: Uses separate filters for authored events, p-tag mentions, and relay list events
- **Intelligent time management**: Works backwards from current time (or until timestamp) to since timestamp

### Authentication & Access
- **Private key support**: Use nsec keys to authenticate to relays that require it (NIP-42)
- **Public key compatibility**: Continue to work with npub keys for public relay access
- **Graceful fallback**: Continue operation even if authentication fails on some relays
- **Auth-required relay access**: Access private notes and restricted content on authenticated relays
- **Flexible key input**: Automatically detects and handles both nsec and npub key formats

### Memory Management
- **Memory-efficient deduplication**: Uses bloom filter with ~0.1% false positive rate instead of unbounded maps
- **Fixed memory footprint**: Bloom filter uses only ~1.75MB for 1M events with controlled memory growth
- **Memory monitoring**: Real-time memory usage tracking and automatic garbage collection
- **Persistent deduplication**: Bloom filter can be saved and reused across multiple runs

### Incremental Collection
- **Bloom filter persistence**: Save deduplication state between runs for efficient incremental collection
- **Automatic append mode**: When loading existing bloom filter, automatically appends to output file
- **Timestamp tracking**: Records actual time range of processed events in bloom filter output
- **Seamless continuation**: Resume collection from where previous run left off without duplicates

### Reliability & Performance
- Connects to multiple relays simultaneously with dynamic expansion
- Outputs events in JSONL format (one JSON object per line)
- Handles connection failures gracefully
- Continues running until all relay connections are closed
- Time-based filtering with Unix timestamps (since/until parameters)
- Input validation for timestamp ranges
- Rate limiting and backoff for relay connection management

## Event Discovery

The aggregator searches for three types of events:

1. **Authored Events**: Events where the specified npub is the author (pubkey field matches)
2. **Mentioned Events**: Events that contain "p" tags referencing the specified npub (replies, mentions, etc.)
3. **Relay List Events**: Kind 10002 events that contain relay URLs for dynamic relay discovery

This comprehensive approach ensures you capture all events related to a user, including:
- Posts authored by the user
- Replies to the user's posts
- Posts that mention or tag the user
- Any other events that reference the user in p-tags
- Relay list metadata for discovering additional relays

## Progressive Fetching

The aggregator uses an intelligent progressive backward fetching strategy:

1. **Time-based batches**: Fetches data in weekly batches working backwards from the end time
2. **Dynamic relay expansion**: As relay list events are discovered, new relays are automatically added to the search
3. **Complete coverage**: Ensures all events between since and until timestamps are collected
4. **Efficient processing**: Processes each time batch completely before moving to the next
5. **Boundary respect**: Stops when reaching the since timestamp or beginning of available data

## Incremental Collection Workflow

The aggregator supports efficient incremental data collection using persistent bloom filters. This allows you to build comprehensive event archives over time without re-processing duplicate events.

### How It Works

1. **First Run**: Creates a new bloom filter and collects events for the specified time range
2. **Bloom Filter Output**: At completion, outputs bloom filter summary to stderr with:
   - Event statistics (processed count, estimated unique events)
   - Time range covered (actual timestamps of collected events)
   - Base64-encoded bloom filter data for reuse
3. **Subsequent Runs**: Load the saved bloom filter to skip already-seen events
4. **Automatic Append**: When using an existing filter, new events are appended to the output file

### Bloom Filter Output Format

The bloom filter output includes comprehensive metadata:

```
=== BLOOM FILTER SUMMARY ===
Events processed: 1247
Estimated unique events: 1247
Bloom filter size: 1.75 MB
False positive rate: ~0.1%
Hash functions: 10
Time range covered: 1640995200 to 1672531200
Time range (human): 2022-01-01T00:00:00Z to 2023-01-01T00:00:00Z

Bloom filter (base64):
[base64-encoded binary data]
=== END BLOOM FILTER ===
```

### Best Practices

- **Save bloom filters**: Always redirect stderr to a file to preserve the bloom filter
- **Sequential time ranges**: Use non-overlapping time ranges for optimal efficiency
- **Regular updates**: Update your bloom filter file after each run for the latest state
- **Backup filters**: Keep copies of bloom filter files for different time periods

### Example Workflow

```bash
# Month 1: January 2022 (using npub for public relays)
go run main.go -key npub1... -since 1640995200 -until 1643673600 -output jan2022.jsonl 2>filter_jan.txt

# Month 2: February 2022 (using nsec for auth-required relays, append to same file)
go run main.go -key nsec1... -since 1643673600 -until 1646092800 -filter filter_jan.txt -output all_events.jsonl 2>filter_feb.txt

# Month 3: March 2022 (continue with authentication for complete coverage)
go run main.go -key nsec1... -since 1646092800 -until 1648771200 -filter filter_feb.txt -output all_events.jsonl 2>filter_mar.txt

# Result: all_events.jsonl contains deduplicated events from all three months, including private relay content
```

## Memory Management

The aggregator uses advanced memory management techniques to handle large-scale data collection:

### Bloom Filter Deduplication
- **Fixed Size**: Uses exactly 1.75MB for the bloom filter regardless of event count
- **Low False Positive Rate**: Configured for ~0.1% false positive rate with 1M events
- **Hash Functions**: Uses 10 independent hash functions based on SHA256 for optimal distribution
- **Thread-Safe**: Concurrent access protected with read-write mutexes

### Memory Monitoring
- **Real-time Tracking**: Monitors total memory usage every 30 seconds
- **Automatic GC**: Triggers garbage collection when approaching memory limits
- **Statistics Logging**: Reports bloom filter usage, estimated event count, and memory consumption
- **Controlled Growth**: Prevents unbounded memory growth through fixed-size data structures

### Performance Characteristics
- **Memory Usage**: ~1.75MB bloom filter + ~256MB total memory limit
- **False Positives**: ~0.1% chance of incorrectly identifying a duplicate (very low impact)
- **Scalability**: Can handle millions of events without memory issues
- **Efficiency**: O(k) time complexity for both add and lookup operations (k = hash functions)

## Relays

The program starts with the following initial relays:

- wss://nostr.wine/
- wss://nostr.land/
- wss://orly-relay.imwald.eu
- wss://relay.orly.dev/
- wss://relay.damus.io/
- wss://nos.lol/
- wss://theforest.nostr1.com/

**Dynamic Relay Discovery**: Additional relays are automatically discovered and added during execution when the program finds relay list events (kind 10002) authored by the target user. This ensures comprehensive coverage across the user's preferred relay network.

## Output Format

### Event Output (stdout or -output file)

Each line of output is a JSON object representing a Nostr event with the following fields:

- `id`: Event ID (hex)
- `pubkey`: Author's public key (hex)
- `created_at`: Unix timestamp
- `kind`: Event kind number
- `tags`: Array of tag arrays
- `content`: Event content string
- `sig`: Event signature (hex)

### Bloom Filter Output (stderr)

At program completion, a comprehensive bloom filter summary is written to stderr containing:

- **Statistics**: Event counts, memory usage, performance metrics
- **Time Range**: Actual timestamp range of collected events (both Unix and human-readable)
- **Configuration**: Bloom filter parameters (size, hash functions, false positive rate)
- **Binary Data**: Base64-encoded bloom filter for reuse in subsequent runs

The bloom filter output is structured with clear markers (`=== BLOOM FILTER SUMMARY ===` and `=== END BLOOM FILTER ===`) making it easy to parse and extract the base64 data programmatically.

### Output Separation

- **Events**: Always go to stdout (default) or the file specified by `-output`
- **Bloom Filter**: Always goes to stderr, allowing separate redirection
- **Logs**: Runtime information and progress updates go to stderr

This separation allows flexible output handling:
```bash
# Events to file, bloom filter visible in terminal
./aggregator -npub npub1... -output events.jsonl

# Both events and bloom filter to separate files
./aggregator -npub npub1... -output events.jsonl 2>bloom_filter.txt

# Events piped to another program, bloom filter saved
./aggregator -npub npub1... 2>bloom_filter.txt | jq '.content'
```

## Testing

The aggregator includes comprehensive tests to ensure reliable data collection:

### Running Tests

```bash
# Run aggregator tests
go test ./cmd/aggregator

# Run all tests including aggregator
go test ./...

# Run with verbose output
go test -v ./cmd/aggregator
```

### Integration Testing

The aggregator is tested as part of the project's integration test suite:

```bash
# Run the full test suite
./scripts/test.sh

# Run benchmarks (which include aggregator performance)
./scripts/runtests.sh
```

### Example Test Usage

```bash
# Test with mock data (if available)
go test -v ./cmd/aggregator -run TestAggregator

# Test bloom filter functionality
go test -v ./cmd/aggregator -run TestBloomFilter
```

## Development

### Building from Source

```bash
# Build the aggregator binary
go build -o aggregator ./cmd/aggregator

# Build with optimizations
go build -ldflags="-s -w" -o aggregator ./cmd/aggregator

# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 go build -o aggregator-linux-amd64 ./cmd/aggregator
GOOS=darwin GOARCH=arm64 go build -o aggregator-darwin-arm64 ./cmd/aggregator
```

### Code Quality

The aggregator follows Go best practices and includes:

- Comprehensive error handling
- Memory-efficient data structures
- Concurrent processing with proper synchronization
- Extensive logging for debugging

## License

This tool is part of the git.smesh.lol/orly project and follows the same licensing terms.
