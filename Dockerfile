# Multi-stage Dockerfile for ORLY relay

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

# Build the binary with CGO disabled
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o orly -ldflags="-w -s" .

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

# Create data directory
RUN mkdir -p /data && chown -R orly:orly /data /app

# Switch to app user
USER orly

# Expose ports
EXPOSE 3334

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -f http://localhost:3334/ || exit 1

# Set default environment variables
ENV ORLY_LISTEN=0.0.0.0 \
    ORLY_PORT=3334 \
    ORLY_DATA_DIR=/data \
    ORLY_LOG_LEVEL=info

# Run the binary
ENTRYPOINT ["/app/orly"]
