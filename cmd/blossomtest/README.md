# Blossom Test Tool

A simple command-line tool to test the Blossom blob storage service by performing upload, fetch, and delete operations.

## Building

```bash
# From the repository root
CGO_ENABLED=0 go build -o cmd/blossomtest/blossomtest ./cmd/blossomtest
```

## Usage

```bash
# Basic usage with auto-generated key
./cmd/blossomtest/blossomtest

# Specify relay URL
./cmd/blossomtest/blossomtest -url http://localhost:3334

# Use a specific Nostr key (nsec format)
./cmd/blossomtest/blossomtest -nsec nsec1...

# Test with larger blob
./cmd/blossomtest/blossomtest -size 10240

# Verbose output to see HTTP requests and auth events
./cmd/blossomtest/blossomtest -v

# Test anonymous uploads (for open relays)
./cmd/blossomtest/blossomtest -no-auth
```

## Options

- `-url` - Relay base URL (default: `http://localhost:3334`)
- `-nsec` - Nostr private key in nsec format (generates new key if not provided)
- `-size` - Size of test blob in bytes (default: 1024)
- `-v` - Verbose output showing HTTP requests and authentication events
- `-no-auth` - Skip authentication and test anonymous uploads (useful for open relays)

## What It Tests

The tool performs the following operations in sequence:

1. **Upload** - Uploads random test data to the Blossom server
   - Creates a Blossom authorization event (kind 24242)
   - Sends a PUT request to `/blossom/upload`
   - Verifies the returned descriptor

2. **Fetch** - Retrieves the uploaded blob
   - Sends a GET request to `/blossom/<sha256>`
   - Verifies the data matches what was uploaded

3. **Delete** - Removes the blob from the server
   - Creates another authorization event for deletion
   - Sends a DELETE request to `/blossom/<sha256>`

4. **Verify** - Confirms deletion was successful
   - Attempts to fetch the blob again
   - Expects a 404 Not Found response

## Example Output

```
🌸 Blossom Test Tool
===================

ℹ️  No key provided, generated new keypair
Using identity: npub1...
Relay URL: http://localhost:3334

📦 Generated 1024 bytes of random data
   SHA256: a1b2c3d4...

📤 Step 1: Uploading blob...
✅ Upload successful!
   URL: http://localhost:3334/blossom/a1b2c3d4...
   SHA256: a1b2c3d4...
   Size: 1024 bytes

📥 Step 2: Fetching blob...
✅ Fetch successful! Retrieved 1024 bytes
✅ Data verification passed - hashes match!

🗑️  Step 3: Deleting blob...
✅ Delete successful!

🔍 Step 4: Verifying deletion...
✅ Blob successfully deleted - returns 404 as expected

🎉 All tests passed! Blossom service is working correctly.
```

## Requirements

- A running ORLY relay with Blossom enabled
- The relay must be using the Badger backend (Blossom is not available with DGraph)
- Network connectivity to the relay

## Troubleshooting

**"connection refused"**
- Make sure your relay is running
- Check the URL is correct (default: `http://localhost:3334`)

**"unauthorized" or "403 Forbidden"**
- Check your relay's ACL settings
- If using `ORLY_AUTH_TO_WRITE=true`, make sure authentication is working
- Try adding your test key to `ORLY_ADMINS` if using follows mode

**"blossom server not initialized"**
- Blossom only works with the Badger backend
- Check `ORLY_DB_TYPE` is set to `badger` or not set (defaults to badger)
