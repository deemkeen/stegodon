# Docker Deployment

This document specifies Docker container configuration and deployment patterns.

---

## Overview

Stegodon provides Docker deployment via:
- Multi-stage Dockerfile for minimal images
- Docker Compose for orchestration
- GitHub Container Registry for pre-built images
- TrueColor (24-bit) rendering for consistent colors

---

## Container Image

### Registry

```
ghcr.io/deemkeen/stegodon:latest
```

Images are automatically built and published on every commit.

### Base Images

| Stage | Image | Purpose |
|-------|-------|---------|
| Builder | `golang:1.26-alpine3.23` | Compile Go binary |
| Runtime | `alpine:3.23` | Minimal production image |

---

## Dockerfile

### Multi-Stage Build

```dockerfile
# Build stage
FROM golang:1.26-alpine3.23 AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o stegodon .

# Final stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget ncurses-terminfo-base
RUN addgroup -g 1000 stegodon && \
    adduser -D -u 1000 -G stegodon stegodon
WORKDIR /home/stegodon
COPY --from=builder /build/stegodon /usr/local/bin/stegodon
RUN mkdir -p /home/stegodon/.config/stegodon && \
    chown -R stegodon:stegodon /home/stegodon
USER stegodon
EXPOSE 23232 9999
ENV STEGODON_HOST=0.0.0.0 \
    STEGODON_SSHPORT=23232 \
    STEGODON_HTTPPORT=9999 \
    TERM=xterm-256color
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9999/feed || exit 1
CMD ["stegodon"]
```

### Build Flags

| Flag | Purpose |
|------|---------|
| `CGO_ENABLED=0` | Pure Go build, no C dependencies |
| `-ldflags="-s -w"` | Strip debug symbols, reduce size |

### Runtime Dependencies

| Package | Purpose |
|---------|---------|
| `ca-certificates` | HTTPS for federation |
| `wget` | Health check |
| `ncurses-terminfo-base` | Terminal database |

---

## Docker Compose

### Basic Configuration

```yaml
version: '3.8'

services:
  stegodon:
    image: ghcr.io/deemkeen/stegodon:latest
    container_name: stegodon
    restart: unless-stopped
    ports:
      - "23232:23232"  # SSH
      - "9999:9999"    # HTTP
    environment:
      - STEGODON_HOST=0.0.0.0
      - STEGODON_SSHPORT=23232
      - STEGODON_HTTPPORT=9999
    volumes:
      - stegodon-data:/home/stegodon/.config/stegodon
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 128M

volumes:
  stegodon-data:
    driver: local
```

### Resource Limits

| Resource | Limit | Reservation |
|----------|-------|-------------|
| CPU | 1 core | 0.25 cores |
| Memory | 512 MB | 128 MB |

---

## Environment Variables

### Container Defaults

```dockerfile
ENV STEGODON_HOST=0.0.0.0 \
    STEGODON_SSHPORT=23232 \
    STEGODON_HTTPPORT=9999 \
    TERM=xterm-256color
```

### ActivityPub Configuration

```yaml
environment:
  - STEGODON_WITH_AP=true
  - STEGODON_SSLDOMAIN=yourdomain.com
```

### Mode Configuration

```yaml
environment:
  - STEGODON_SINGLE=true   # Single-user mode
  - STEGODON_CLOSED=true   # Closed registration
```

### Content Configuration

```yaml
environment:
  - STEGODON_MAX_CHARS=200  # Maximum characters per note (1-300, default: 150)
```

---

## TrueColor Rendering

### Terminal Configuration

```dockerfile
ENV TERM=xterm-256color
```

### Color Profile in Code

```go
lipgloss.SetColorProfile(termenv.TrueColor)
```

Uses TrueColor (24-bit) for palette-independent color rendering:
- Hex colors render identically across all terminals
- No dependency on terminal's 256-color palette mapping
- `TERM=xterm-256color` ensures the terminal advertises color support

---

## Data Persistence

### Volume Mount

```yaml
volumes:
  - stegodon-data:/home/stegodon/.config/stegodon
```

### Persisted Data

| Path | Content |
|------|---------|
| `database.db` | SQLite database |
| `.ssh/stegodonhostkey` | SSH host key |
| `config.yaml` | Configuration |

### Backup

```bash
docker run --rm \
  -v stegodon-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/stegodon-backup.tar.gz -C /data .
```

### Restore

```bash
docker run --rm \
  -v stegodon-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/stegodon-backup.tar.gz -C /data
```

---

## Health Check

### Configuration

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9999/feed || exit 1
```

| Parameter | Value |
|-----------|-------|
| Interval | 30 seconds |
| Timeout | 10 seconds |
| Start period | 5 seconds |
| Retries | 3 |
| Endpoint | `/feed` RSS |

### Check Status

```bash
docker ps
# Look for "healthy" in STATUS column
```

---

## Security

### Non-Root User

```dockerfile
RUN addgroup -g 1000 stegodon && \
    adduser -D -u 1000 -G stegodon stegodon
USER stegodon
```

Container runs as UID/GID 1000, not root.

### Port Exposure

| Port | Service | Exposure |
|------|---------|----------|
| 23232 | SSH | Direct access |
| 9999 | HTTP | Behind reverse proxy |

---

## Reverse Proxy Setup

### Caddy (Recommended)

```
yourdomain.com {
    reverse_proxy stegodon:9999
}
```

### Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://stegodon:9999;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Production Deployment

### Requirements

1. Reverse proxy with HTTPS (Caddy/Nginx)
2. DNS records configured
3. `STEGODON_SSLDOMAIN` set to domain
4. `STEGODON_WITH_AP=true` for federation
5. Regular volume backups

### Production Compose

```yaml
services:
  stegodon:
    image: ghcr.io/deemkeen/stegodon:latest
    restart: unless-stopped
    environment:
      - STEGODON_HOST=0.0.0.0
      - STEGODON_WITH_AP=true
      - STEGODON_SSLDOMAIN=stegodon.example.com
    volumes:
      - stegodon-data:/home/stegodon/.config/stegodon
    networks:
      - stegodon-net
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "23232:23232"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
    networks:
      - stegodon-net
```

---

## Commands

### Start

```bash
docker-compose up -d
```

### Stop

```bash
docker-compose down
```

### View Logs

```bash
docker-compose logs -f stegodon
```

### Update

```bash
docker-compose pull
docker-compose up -d
```

### Reset (Warning: Deletes Data)

```bash
docker-compose down -v
docker-compose up -d
```

---

## Troubleshooting

### Container Won't Start

```bash
docker-compose logs stegodon
```

### SSH Connection Failed

```bash
# Check port exposure
docker ps

# Test connectivity
nc -zv localhost 23232
```

### Database Locked

```bash
docker-compose down
docker-compose up -d
```

---

## Source Files

- `Dockerfile` - Multi-stage build
- `docker-compose.yml` - Compose orchestration
- `DOCKER.md` - Deployment documentation
