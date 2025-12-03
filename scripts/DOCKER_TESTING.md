# Docker-Based Integration Testing

This guide covers running ORLY in Docker containers for integration testing.

## Overview

The Docker setup provides:
- **Isolated Environment**: ORLY in containers
- **Automated Testing**: Health checks and dependency management
- **Reproducible Tests**: Consistent environment across systems
- **Easy Cleanup**: Remove everything with one command

## Architecture

```
┌─────────────────────────────────────────────┐
│           Docker Network (orly-network)     │
│                                             │
│  ┌─────────────────┐  ┌─────────────────┐  │
│  │   ORLY Relay    │  │  relay-tester   │  │
│  │  (badger mode)  │  │  (optional)     │  │
│  │                 │  │                 │  │
│  │   :3334 (WS)    │  │                 │  │
│  └─────────────────┘  └─────────────────┘  │
│         │                                   │
└─────────┼───────────────────────────────────┘
          │
      Published
      to host
```

## Quick Start

### 1. Build Images

```bash
# Build ORLY image only
./scripts/docker-build.sh

# Build ORLY + relay-tester
./scripts/docker-build.sh --with-tester
```

### 2. Run Integration Tests

```bash
# Basic test (start containers, verify connectivity)
./scripts/test-docker.sh

# Run with relay-tester
./scripts/test-docker.sh --relay-tester

# Keep containers running after test
./scripts/test-docker.sh --keep-running

# Skip rebuild (use existing images)
./scripts/test-docker.sh --skip-build
```

### 3. Manual Container Management

```bash
# Start containers
cd scripts
docker-compose -f docker-compose-test.yml up -d

# View logs
docker-compose -f docker-compose-test.yml logs -f

# Stop containers
docker-compose -f docker-compose-test.yml down

# Stop and remove volumes
docker-compose -f docker-compose-test.yml down -v
```

## Docker Files

### Dockerfile

Multi-stage build for ORLY:

**Stage 1: Builder**
- Based on golang:1.25-alpine
- Downloads dependencies
- Builds static binary with `CGO_ENABLED=0`
- Copies libsecp256k1.so for crypto operations

**Stage 2: Runtime**
- Based on alpine:latest (minimal)
- Copies binary and shared library
- Creates non-root user
- Sets up health checks
- ~50MB final image size

### Dockerfile.relay-tester

Builds relay-tester for automated testing:
- Static binary from cmd/relay-tester
- Configurable RELAY_URL
- Runs as part of test profile

### docker-compose-test.yml

Orchestrates the full stack:

**Services:**
1. **orly** - Relay server
   - Configured with ORLY_DB_TYPE=badger (default)
   - Health check via HTTP
   - Auto-restart on failure

2. **relay-tester** - Test runner
   - Profile: test (optional)
   - Runs tests against ORLY
   - Exits after completion

## Configuration

### Environment Variables

The docker-compose file sets:

```yaml
# Server
ORLY_LISTEN: 0.0.0.0
ORLY_PORT: 3334
ORLY_DATA_DIR: /data

# Application
ORLY_LOG_LEVEL: info
ORLY_APP_NAME: ORLY-Test
ORLY_ACL_MODE: none
```

Override via environment or .env file:

```bash
# Create .env file in scripts/
cat > scripts/.env << EOF
ORLY_LOG_LEVEL=debug
ORLY_ADMINS=npub1...
EOF
```

### Volumes

**Persistent Data:**
- `orly-data:/data` - ORLY database and metadata

**Inspect Volumes:**
```bash
docker volume ls
docker volume inspect scripts_orly-data
```

### Networks

**Custom Bridge Network:**
- Name: orly-network
- Subnet: 172.28.0.0/16
- Allows container-to-container communication
- DNS resolution by service name

## Testing Workflows

### Basic Integration Test

```bash
./scripts/test-docker.sh
```

**What it does:**
1. Stops any existing containers
2. Starts ORLY and waits for health
3. Verifies HTTP connectivity
4. Tests WebSocket (if websocat installed)
5. Shows container status
6. Cleans up (unless --keep-running)

### With Relay-Tester

```bash
./scripts/test-docker.sh --relay-tester
```

**Additional steps:**
1. Builds relay-tester image
2. Runs comprehensive protocol tests
3. Reports pass/fail
4. Shows ORLY logs on failure

### Development Workflow

```bash
# Start and keep running
./scripts/test-docker.sh --keep-running

# Make changes to code
vim pkg/database/save-event.go

# Rebuild and restart
docker-compose -f scripts/docker-compose-test.yml up -d --build orly

# View logs
docker logs orly-relay -f

# Test changes
go run cmd/relay-tester/main.go -url ws://localhost:3334

# Stop when done
cd scripts && docker-compose -f docker-compose-test.yml down
```

## Debugging

### View Container Logs

```bash
# All services
docker-compose -f scripts/docker-compose-test.yml logs -f

# Specific service
docker logs orly-relay -f

# Last N lines
docker logs orly-relay --tail 50
```

### Execute Commands in Container

```bash
# ORLY version
docker exec orly-relay /app/orly version

# Check ORLY processes
docker exec orly-relay ps aux

# Inspect data directory
docker exec orly-relay ls -la /data
```

### Network Inspection

```bash
# List networks
docker network ls

# Inspect orly network
docker network inspect scripts_orly-network
```

### Health Check Status

```bash
# Check health
docker inspect orly-relay | grep -A 10 Health

# View health check logs
docker inspect --format='{{json .State.Health}}' orly-relay | jq
```

## Troubleshooting

### Build Failures

**Error: Cannot find libsecp256k1.so**

```bash
# Ensure library exists
ls -l libsecp256k1.so

# Library should be in repository root
```

**Error: Go module download fails**

```bash
# Clear module cache
go clean -modcache

# Try building locally first
CGO_ENABLED=0 go build
```

### Runtime Failures

**ORLY fails health check**

```bash
# Check logs
docker logs orly-relay

# Common issues:
# - Port already in use: docker ps (check for conflicts)
# - Bad configuration: docker exec orly-relay env
```

**WebSocket connection fails**

```bash
# Test from host
websocat ws://localhost:3334

# Test from container
docker exec orly-relay curl -v http://localhost:3334

# Check firewall
sudo iptables -L | grep 3334
```

### Performance Issues

**Slow startup**

```bash
# Increase health check timeouts in docker-compose-test.yml
start_period: 60s  # Default is 20-30s

# Pre-pull images
docker pull golang:1.25-alpine
```

**High memory usage**

```bash
# Check resource usage
docker stats

# Limit container resources
# Add to docker-compose-test.yml:
deploy:
  resources:
    limits:
      memory: 2G
      cpus: '2'
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Docker Integration Tests

on: [push, pull_request]

jobs:
  docker-test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Build images
        run: ./scripts/docker-build.sh --with-tester

      - name: Run integration tests
        run: ./scripts/test-docker.sh --relay-tester

      - name: Upload logs on failure
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: container-logs
          path: |
            scripts/orly-relay.log
```

### GitLab CI Example

```yaml
docker-test:
  image: docker:latest
  services:
    - docker:dind
  script:
    - ./scripts/docker-build.sh --with-tester
    - ./scripts/test-docker.sh --relay-tester
  artifacts:
    when: on_failure
    paths:
      - scripts/*.log
```

## Advanced Usage

### Custom Configuration

Create a custom docker-compose override:

```yaml
# docker-compose.override.yml
version: '3.8'

services:
  orly:
    environment:
      - ORLY_LOG_LEVEL=debug
      - ORLY_ADMINS=npub1...
      - ORLY_ACL_MODE=follows
    ports:
      - "3335:3334"  # Different host port
```

### Multi-Instance Testing

Test multiple ORLY instances:

```yaml
# docker-compose-multi.yml
services:
  orly-1:
    extends:
      file: docker-compose-test.yml
      service: orly
    container_name: orly-relay-1
    ports:
      - "3334:3334"

  orly-2:
    extends:
      file: docker-compose-test.yml
      service: orly
    container_name: orly-relay-2
    ports:
      - "3335:3334"
```

### Performance Benchmarking

```bash
# Start with --keep-running
./scripts/test-docker.sh --keep-running

# Run stress test
go run cmd/stresstest/main.go -url ws://localhost:3334 -connections 100

# Monitor resources
docker stats

# Profile ORLY
docker exec orly-relay sh -c 'curl http://localhost:6060/debug/pprof/profile?seconds=30 > /tmp/cpu.prof'
```

## Cleanup

### Remove Everything

```bash
# Stop and remove containers
cd scripts && docker-compose -f docker-compose-test.yml down

# Remove volumes (data)
docker-compose -f docker-compose-test.yml down -v

# Remove images
docker rmi orly:latest orly-relay-tester:latest

# Remove networks
docker network rm scripts_orly-network

# Prune everything (careful!)
docker system prune -a --volumes
```

### Selective Cleanup

```bash
# Just stop containers (keep data)
docker-compose -f docker-compose-test.yml stop

# Remove only one service
docker-compose -f docker-compose-test.yml rm -s -f orly

# Clear ORLY data
docker volume rm scripts_orly-data
```

## Related Documentation

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose](https://docs.docker.com/compose/)

## Best Practices

1. **Always use health checks** - Ensure services are ready
2. **Use specific tags** - Don't rely on :latest in production
3. **Limit resources** - Prevent container resource exhaustion
4. **Volume backups** - Backup orly-data volume before updates
5. **Network isolation** - Use custom networks for security
6. **Read-only root** - Run as non-root user
7. **Clean up regularly** - Remove unused containers/volumes
