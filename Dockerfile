# Multi-stage Dockerfile for ORLY relay

# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

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
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates curl wget

# Create app user
RUN addgroup -g 1000 orly && \
    adduser -D -u 1000 -G orly orly

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/orly /app/orly

# Download libsecp256k1.so from nostr repository (optional for performance)
RUN wget -q https://git.mleku.dev/mleku/nostr/raw/branch/main/crypto/p8k/libsecp256k1.so \
    -O /app/libsecp256k1.so || echo "Warning: libsecp256k1.so download failed (optional)"

# Set library path
ENV LD_LIBRARY_PATH=/app

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
