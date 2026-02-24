# Multi-stage Dockerfile for ORLY relay + bridge (unified binary)
#
# Default: runs the relay (port 3334)
# Bridge:  docker run orly bridge (port 2525)
# Launcher: docker run orly launcher (relay + bridge + db)

# Stage 1: Build stage
# Use Debian-based Go image to match runtime stage (avoids musl/glibc linker mismatch)
FROM golang:1.25-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends git make && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the unified binary (includes all subcommands: relay, bridge, db, acl, launcher)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o orly -ldflags="-w -s" ./cmd/orly

# Stage 2: Runtime stage
# Use Debian slim instead of Alpine because Debian's libsecp256k1 includes
# Schnorr signatures (secp256k1_schnorrsig_*) and ECDH which Nostr requires.
# Alpine's libsecp256k1 is built without these modules.
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl libsecp256k1-1 && \
    rm -rf /var/lib/apt/lists/*

# Create app user
RUN groupadd -g 1000 orly && \
    useradd -m -u 1000 -g orly orly

# Set working directory
WORKDIR /app

# Copy binary (libsecp256k1.so.1 is already installed via apt)
COPY --from=builder /build/orly /app/orly

# Create data and DKIM directories
RUN mkdir -p /data /dkim && chown -R orly:orly /data /dkim /app

# Switch to app user
USER orly

# Expose ports: 3334=relay WebSocket, 2525=bridge SMTP inbound
EXPOSE 3334 2525

# Health check (relay mode — override for bridge mode in compose)
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -f http://localhost:3334/ || exit 1

# Set default environment variables
ENV ORLY_LISTEN=0.0.0.0 \
    ORLY_PORT=3334 \
    ORLY_DATA_DIR=/data \
    ORLY_LOG_LEVEL=info

# Run the binary (default: relay; pass "bridge" to run the email bridge)
ENTRYPOINT ["/app/orly"]
