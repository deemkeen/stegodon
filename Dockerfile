# Build stage
FROM golang:1.26-alpine3.23 AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary (pure Go, no CGO needed)
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o stegodon .

# Final stage
FROM alpine:3.23

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget ncurses-terminfo-base

# Create non-root user
RUN addgroup -g 1000 stegodon && \
    adduser -D -u 1000 -G stegodon stegodon

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
    STEGODON_HTTPPORT=9999 \
    STEGODON_MAX_CHARS=150 \
    TERM=xterm-256color

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9999/feed || exit 1

# Run stegodon
CMD ["stegodon"]
