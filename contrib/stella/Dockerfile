# Dockerfile for Stella's Nostr Relay (git.smesh.lol/orly)
# Owner: npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx
#
# Build from repository root:
#   docker build -f contrib/stella/Dockerfile -t stella-relay .

# Use Debian-based Go image to match runtime stage (avoids musl/glibc linker mismatch)
FROM golang:1.25-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends git make && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go modules first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the relay with CGO disabled (uses purego for crypto)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o relay .

# Create non-root user for security
RUN useradd -m -u 1000 stella && \
    chown -R 1000:1000 /build

# Final stage - minimal runtime image
# Use Debian slim instead of Alpine because Debian's libsecp256k1 includes
# Schnorr signatures (secp256k1_schnorrsig_*) and ECDH which Nostr requires.
# Alpine's libsecp256k1 is built without these modules.
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl libsecp256k1-1 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary (libsecp256k1.so.1 is already installed via apt)
COPY --from=builder /build/relay /app/relay

# Create runtime user and directories
RUN useradd -m -u 1000 stella && \
    mkdir -p /data /profiles /app && \
    chown -R 1000:1000 /data /profiles /app

# Expose the relay port
EXPOSE 7777

# Set environment variables for Stella's relay
ENV ORLY_DATA_DIR=/data
ENV ORLY_LISTEN=0.0.0.0
ENV ORLY_PORT=7777
ENV ORLY_LOG_LEVEL=info
ENV ORLY_MAX_CONNECTIONS=1000
ENV ORLY_OWNERS=npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx
ENV ORLY_ADMINS=npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx,npub1m4ny6hjqzepn4rxknuq94c2gpqzr29ufkkw7ttcxyak7v43n6vvsajc2jl,npub1l5sga6xg72phsz5422ykujprejwud075ggrr3z2hwyrfgr7eylqstegx9z

# Health check to ensure relay is responding
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD sh -c "code=\$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:7777 || echo 000); echo \$code | grep -E '^(101|200|400|404|426)$' >/dev/null || exit 1"

# Create volume for persistent data
VOLUME ["/data"]

# Drop privileges and run as stella user
USER 1000:1000

# Run Stella's Nostr relay
CMD ["/app/relay"]
