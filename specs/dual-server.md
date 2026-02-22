# Dual Server Model

This document specifies stegodon's dual-server architecture, which runs SSH and HTTP servers concurrently to serve different client types.

---

## Overview

Stegodon runs two independent servers:

| Server | Default Port | Purpose | Library |
|--------|--------------|---------|---------|
| **SSH** | 23232 | TUI client connections | [Wish](https://github.com/charmbracelet/wish) |
| **HTTP** | 9999 | Web UI, RSS, ActivityPub | [Gin](https://github.com/gin-gonic/gin) |

Both servers:
- Start concurrently in separate goroutines
- Share the same configuration
- Support graceful shutdown
- Run independently (failure of one doesn't affect the other)

---

## SSH Server

### Purpose

Provides the primary user interface via terminal. Users connect with any SSH client and interact with a BubbleTea-based TUI.

### Connection Flow

```
User's Terminal
      │
      ▼
  SSH Client
      │
      ▼
┌─────────────────────────────────────┐
│           SSH Server                │
│  ┌───────────────────────────────┐  │
│  │    Middleware Stack           │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │ logging.Middleware      │  │  │
│  │  │   (request logging)     │  │  │
│  │  └───────────┬─────────────┘  │  │
│  │              ▼                │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │ middleware.AuthMiddleware│  │  │
│  │  │   (account lookup/create)│  │  │
│  │  └───────────┬─────────────┘  │  │
│  │              ▼                │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │ middleware.MainTui      │  │  │
│  │  │   (BubbleTea program)   │  │  │
│  │  └─────────────────────────┘  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

### Configuration

```go
sshServer, err := wish.NewServer(
    wish.WithAddress(fmt.Sprintf("%s:%d", conf.Conf.Host, conf.Conf.SshPort)),
    wish.WithHostKeyPath(sshKeyPath),
    wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
    wish.WithMiddleware(
        middleware.MainTui(),
        middleware.AuthMiddleware(conf),
        logging.MiddlewareWithLogger(log.Default()),
    ),
)
```

### SSH Host Key

- **Location**: `~/.config/stegodon/.ssh/stegodonhostkey` or `./.ssh/stegodonhostkey`
- **Format**: Ed25519 or RSA (auto-generated)
- **Persistence**: Generated once, reused for server identity

### Middleware Stack

Middleware executes in reverse order (last registered runs first):

| Order | Middleware | Function |
|-------|------------|----------|
| 1 | `logging.Middleware` | Log SSH connections |
| 2 | `AuthMiddleware` | Account lookup, registration check, mute check |
| 3 | `MainTui` | Start BubbleTea TUI program |

### Authentication

All public keys are accepted for connection. Account is looked up or created in AuthMiddleware:

```go
wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true })
```

Account identification uses SHA256 hash of the public key.

### Terminal Requirements

- **Minimum size**: 115x28 characters
- **Color support**: TrueColor (24-bit)
- **Alt-screen**: Required for TUI

---

## HTTP Server

### Purpose

Provides web-accessible endpoints for:
- Public profile pages
- RSS feeds
- ActivityPub federation
- Static assets

### Route Structure

```
HTTP Server (Gin)
      │
      ├── Middleware
      │     ├── gin.Logger()
      │     ├── gin.Recovery()
      │     ├── gzip.Gzip()
      │     └── RateLimitMiddleware()
      │
      ├── Static Assets
      │     ├── GET /static/stegologo.png
      │     └── GET /static/style.css
      │
      ├── Web UI
      │     ├── GET /                    (home/index)
      │     ├── GET /u/:username         (user profile)
      │     ├── GET /u/:username/:noteid (single post)
      │     ├── GET /@:username          (redirect to /u/)
      │     └── GET /tags/:tag           (tag feed)
      │
      ├── RSS
      │     └── GET /feed?username=      (RSS feed)
      │
      └── ActivityPub
            ├── GET /.well-known/webfinger
            ├── GET /.well-known/nodeinfo
            ├── GET /nodeinfo/2.0
            ├── GET /nodeinfo/2.1
            ├── GET /users/:username           (actor profile)
            ├── GET /users/:username/outbox
            ├── GET /users/:username/followers
            ├── GET /users/:username/following
            ├── POST /users/:username/inbox
            └── POST /inbox                    (shared inbox)
```

### Configuration

```go
router, err := web.Router(conf)
httpServer = &http.Server{
    Addr:    fmt.Sprintf(":%d", conf.Conf.HttpPort),
    Handler: router,
}
```

### Middleware

| Middleware | Purpose |
|------------|---------|
| `gin.Logger()` | Request logging |
| `gin.Recovery()` | Panic recovery |
| `gzip.Gzip()` | Response compression |
| `RateLimitMiddleware()` | 10 req/sec per IP, burst 20 |

### Rate Limiting

Global rate limiter applied to all routes:

```go
globalLimiter := NewRateLimiter(rate.Limit(10), 20)
g.Use(RateLimitMiddleware(globalLimiter))
```

- **Limit**: 10 requests per second per IP
- **Burst**: 20 requests allowed in burst
- **Response**: HTTP 429 Too Many Requests

### Embedded Assets

Templates and static files are embedded in the binary:

```go
//go:embed templates/*.html
var embeddedTemplates embed.FS

//go:embed static/stegologo.png
var embeddedLogo []byte

//go:embed static/style.css
var embeddedCSS []byte
```

### Caching

Static assets use cache headers:

```go
c.Header("Cache-Control", "public, max-age=86400") // 24 hours
```

---

## Server Lifecycle

### Startup

Both servers start in parallel goroutines:

```go
// SSH Server
go func() {
    if err := sshServer.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
        log.Fatalf("SSH server error: %v", err)
    }
}()

// HTTP Server
go func() {
    if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("HTTP server error: %v", err)
    }
}()
```

### Shutdown Order

HTTP server shuts down before SSH to allow in-flight web requests to complete:

1. **HTTP Server** - Stop accepting new requests, complete existing
2. **SSH Server** - Disconnect active sessions

```go
// 1. HTTP shutdown
httpServer.Shutdown(ctx)

// 2. SSH shutdown
sshServer.Shutdown(ctx)
```

### Timeout

Both servers share a 30-second shutdown timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
```

---

## Port Configuration

### Default Ports

| Server | Default | Environment Variable |
|--------|---------|---------------------|
| SSH | 23232 | `STEGODON_SSHPORT` |
| HTTP | 9999 | `STEGODON_HTTPPORT` |

### Bind Address

Both servers bind to the same host address:

```yaml
conf:
  host: 127.0.0.1  # localhost only
  # or
  host: 0.0.0.0    # all interfaces
```

### Production Deployment

Typical production setup with reverse proxy:

```
Internet
    │
    ▼
┌────────────────┐
│  Reverse Proxy │  (nginx, caddy)
│  :443 (HTTPS)  │
└───────┬────────┘
        │
        ▼
┌────────────────┐
│  HTTP Server   │
│  :9999         │
└────────────────┘

SSH Clients ─────────────────────────► SSH Server :23232
```

---

## Error Handling

### SSH Server Errors

```go
if err := sshServer.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
    log.Fatalf("SSH server error: %v", err)
}
```

- `ssh.ErrServerClosed`: Normal shutdown, ignored
- Other errors: Fatal, process exits

### HTTP Server Errors

```go
if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
    log.Fatalf("HTTP server error: %v", err)
}
```

- `http.ErrServerClosed`: Normal shutdown, ignored
- Other errors: Fatal, process exits

### Independence

If one server fails during runtime, the other continues operating. The process only exits if startup fails or a shutdown signal is received.

---

## Logging

### SSH Connections

```
[wish] 192.168.1.100:54321 - publickey - accepted - SHA256:abc123...
```

### HTTP Requests

```
[GIN] 2024/01/15 - 10:30:45 | 200 | 1.234ms | 192.168.1.100 | GET "/u/username"
```

### Server Lifecycle

```
Starting SSH server on 127.0.0.1:23232
Starting HTTP server on 127.0.0.1:9999
...
Stopping HTTP server...
HTTP server stopped gracefully
Stopping SSH server...
SSH server stopped gracefully
```

---

## Security Considerations

### SSH Server

- Public key authentication only (no passwords)
- Host key persisted for server identity
- Account identified by public key hash

### HTTP Server

- Rate limiting prevents DoS
- GZIP compression for bandwidth
- No sensitive data in URLs
- ActivityPub uses HTTP signatures

### Network Isolation

For security, bind to localhost and use a reverse proxy:

```yaml
conf:
  host: 127.0.0.1  # Only accessible locally
```

---

## Source Files

- `app/app.go` - Server initialization and lifecycle
- `web/router.go` - HTTP router and middleware setup
- `middleware/maintui.go` - SSH TUI handler
- `middleware/auth.go` - SSH authentication middleware
