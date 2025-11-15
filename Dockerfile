# Build stage
FROM golang:1.25-bookworm AS builder

# Install build dependencies (gcc for CGO/SQLite)
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO enabled for SQLite
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o stegodon .

# Final stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    sqlite3 \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 stegodon && \
    useradd -u 1000 -g stegodon -m -s /bin/bash stegodon

# Set working directory
WORKDIR /home/stegodon

# Copy binary from builder
COPY --from=builder /build/stegodon /usr/local/bin/stegodon

# Create data directory
RUN mkdir -p /home/stegodon/.config/stegodon && \
    chown -R stegodon:stegodon /home/stegodon

# Switch to non-root user
USER stegodon

# Expose ports
EXPOSE 23232 9999

# Set default environment variables
ENV STEGODON_HOST=0.0.0.0 \
    STEGODON_SSHPORT=23232 \
    STEGODON_HTTPPORT=9999

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9999/feed || exit 1

# Run stegodon
CMD ["stegodon"]
