# ORLY Scripts Directory

This directory contains automation scripts for building, testing, and deploying ORLY.

## Quick Reference

### Build & Deploy

```bash
./build-all-platforms.sh    # Build for multiple platforms
./deploy.sh                 # Deploy to systemd
./update-embedded-web.sh    # Build and embed web UI
```

### Testing

```bash
./test.sh                   # Run all Go tests
./test-docker.sh            # Run integration tests in containers
```

## Script Descriptions

### Docker Integration Scripts

#### docker-build.sh
Builds Docker images for ORLY and optionally relay-tester.

**Usage:**
```bash
./docker-build.sh                   # ORLY only
./docker-build.sh --with-tester     # ORLY + relay-tester
```

**Output:**
- orly:latest
- orly-relay-tester:latest (if --with-tester)

#### docker-compose-test.yml
Full-stack docker-compose with ORLY and relay-tester.

**Services:**
- orly: Relay with Badger backend
- relay-tester: Protocol tests (optional, profile: test)

**Features:**
- Health checks for all services
- Custom network with DNS
- Persistent volumes

#### test-docker.sh
Comprehensive integration testing in Docker containers.

**Usage:**
```bash
./test-docker.sh                              # Basic test
./test-docker.sh --relay-tester               # Run relay-tester
./test-docker.sh --keep-running               # Keep containers running
./test-docker.sh --skip-build                 # Use existing images
./test-docker.sh --relay-tester --keep-running  # Full test + keep running
```

**What it does:**
1. Stops any existing containers
2. Optionally rebuilds images
3. Starts ORLY and waits for health
4. Verifies connectivity
5. Optionally runs relay-tester
6. Shows status and endpoints
7. Cleanup (unless --keep-running)

### Build Scripts

#### build-all-platforms.sh
Cross-compiles ORLY for multiple platforms.

**Platforms:**
- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64

**Output:** `dist/` directory with platform-specific binaries

#### update-embedded-web.sh
Builds the Svelte web UI and embeds it in ORLY binary.

**Steps:**
1. Builds web UI with bun
2. Generates embedded assets
3. Rebuilds ORLY with embedded UI

### Deployment Scripts

#### deploy.sh
Automated deployment with systemd service.

**What it does:**
1. Installs Go if needed
2. Builds ORLY with embedded web UI
3. Installs to ~/.local/bin/orly
4. Creates systemd service
5. Enables and starts service
6. Sets up port binding capabilities

### Test Scripts

#### test.sh
Runs all Go tests in the project.

**Usage:**
```bash
./test.sh          # All tests
TEST_LOG=1 ./test.sh  # With logging
```

## Environment Variables

### Common Variables

```bash
# Logging
export ORLY_LOG_LEVEL=debug              # Log verbosity
export TEST_LOG=1                        # Enable test logging

# Server
export ORLY_PORT=3334                    # HTTP/WebSocket port
export ORLY_LISTEN=0.0.0.0               # Listen address

# Data
export ORLY_DATA_DIR=/path/to/data       # Data directory

# Database backend
export ORLY_DB_TYPE=badger               # Use badger backend (default)
export ORLY_DB_TYPE=neo4j                # Use Neo4j backend
```

### Script-Specific Variables

```bash
# Docker scripts
export SKIP_BUILD=true                   # Skip image rebuild
export KEEP_RUNNING=true                 # Don't cleanup containers
```

## File Organization

```
scripts/
├── README.md                    # This file
├── DOCKER_TESTING.md           # Docker testing guide
│
├── docker-build.sh             # Build docker images
├── docker-compose-test.yml     # Full stack docker config
├── test-docker.sh              # Run docker integration tests
│
├── build-all-platforms.sh      # Cross-compile
├── deploy.sh                   # Deploy to systemd
├── update-embedded-web.sh      # Build web UI
└── test.sh                     # Run Go tests
```

## Workflows

### Local Development

```bash
# 1. Run ORLY locally
./orly

# 2. Test changes
go run cmd/relay-tester/main.go -url ws://localhost:3334

# 3. Run unit tests
./scripts/test.sh
```

### Docker Development

```bash
# 1. Build and test in containers
./scripts/test-docker.sh --relay-tester --keep-running

# 2. Make changes

# 3. Rebuild just ORLY
cd scripts
docker-compose -f docker-compose-test.yml up -d --build orly

# 4. View logs
docker logs orly-relay -f

# 5. Stop when done
docker-compose -f docker-compose-test.yml down
```

### CI/CD Testing

```bash
# Quick test (no containers)
./scripts/test.sh

# Full integration test
./scripts/test-docker.sh --relay-tester

# Build for deployment
./scripts/build-all-platforms.sh
```

### Production Deployment

```bash
# Deploy with systemd
./scripts/deploy.sh

# Check status
systemctl status orly

# View logs
journalctl -u orly -f

# Update
./scripts/deploy.sh  # Rebuilds and restarts
```

## Troubleshooting

### Port Conflicts

```bash
# Find what's using port 3334
lsof -i :3334
netstat -tlnp | grep 3334

# Kill process
kill $(lsof -t -i :3334)

# Or use different port
export ORLY_PORT=3335
```

### Docker Build Failures

```bash
# Clear docker cache
docker builder prune

# Rebuild from scratch
docker build --no-cache -t orly:latest -f Dockerfile .

# Check Dockerfile syntax
docker build --dry-run -f Dockerfile .
```

### Permission Issues

```bash
# Fix script permissions
chmod +x scripts/*.sh

# Fix docker socket
sudo usermod -aG docker $USER
newgrp docker
```

## Best Practices

1. **Always use scripts from project root**
   ```bash
   ./scripts/test-docker.sh  # Good
   cd scripts && ./test-docker.sh  # May have path issues
   ```

2. **Check prerequisites before running**
   ```bash
   # Check docker
   docker --version
   docker-compose --version
   ```

3. **Clean up after testing**
   ```bash
   # Stop containers
   cd scripts && docker-compose -f docker-compose-test.yml down

   # Remove volumes if needed
   docker-compose -f docker-compose-test.yml down -v
   ```

4. **Use --keep-running for debugging**
   ```bash
   ./scripts/test-docker.sh --keep-running
   # Inspect, debug, make changes
   docker-compose -f scripts/docker-compose-test.yml down
   ```

5. **Check logs on failures**
   ```bash
   # Container logs
   docker logs orly-relay --tail 100

   # Test output
   ./scripts/test.sh 2>&1 | tee test.log
   ```

## Related Documentation

- [Docker Testing Guide](DOCKER_TESTING.md)

## Contributing

When adding new scripts:

1. **Add executable permission**
   ```bash
   chmod +x scripts/new-script.sh
   ```

2. **Use bash strict mode**
   ```bash
   #!/bin/bash
   set -e  # Exit on error
   ```

3. **Add help text**
   ```bash
   if [ "$1" == "--help" ]; then
       echo "Usage: $0 [options]"
       exit 0
   fi
   ```

4. **Document in this README**
   - Add to appropriate section
   - Include usage examples
   - Note any requirements

5. **Test on fresh system**
   ```bash
   # Use Docker to test
   docker run --rm -v $(pwd):/app -w /app ubuntu:latest ./scripts/new-script.sh
   ```
