# Security

This document specifies SSH key hashing, HTTP signatures, and rate limiting security measures.

---

## Overview

Stegodon implements security through:
- **SSH Key Hashing** - SHA-256 hashing of public keys for storage
- **HTTP Signatures** - RSA-SHA256 signing for ActivityPub federation
- **Rate Limiting** - Per-IP request throttling
- **Request Size Limits** - Body size restrictions

---

## SSH Key Hashing

### PkToHash Function

Converts SSH public keys to SHA-256 hashes for database storage.

```go
import (
    "crypto/sha256"
    "encoding/hex"
)

func PkToHash(pk string) string {
    h := sha256.New()
    h.Write([]byte(pk))
    return hex.EncodeToString(h.Sum(nil))
}
```

### Properties

| Property | Value |
|----------|-------|
| Algorithm | SHA-256 |
| Output | 64-character hex string |
| Salt | None (not needed for high-entropy SSH keys) |

### Usage Flow

```
1. User connects via SSH with public key
2. Server extracts public key string
3. Hash = SHA256(public_key)
4. Lookup account by hash in database
5. Create session if account exists
```

### Security Considerations

| Aspect | Status |
|--------|--------|
| Collision resistance | SHA-256 (strong) |
| Preimage resistance | SHA-256 (strong) |
| Salt | Not implemented |
| Timing safety | Standard comparison |

---

## HTTP Signatures

### Overview

ActivityPub requests are signed using RSA-SHA256 HTTP signatures per the Cavage HTTP Signatures specification.

### Signing Implementation

```go
import "code.superseriousbusiness.org/httpsig"

func SignRequest(req *http.Request, privateKey *rsa.PrivateKey, keyId string) error {
    signer, _, err := httpsig.NewSigner(
        []httpsig.Algorithm{httpsig.RSA_SHA256},
        httpsig.DigestSha256,
        []string{"(request-target)", "host", "date", "digest"},
        httpsig.Signature,
        0,
    )
    if err != nil {
        return fmt.Errorf("failed to create signer: %w", err)
    }

    return signer.SignRequest(privateKey, keyId, req, nil)
}
```

### Signed Headers

| Header | Description |
|--------|-------------|
| `(request-target)` | HTTP method and path |
| `host` | Target server hostname |
| `date` | Request timestamp |
| `digest` | SHA-256 hash of body |

### Key ID Format

```
https://example.com/users/alice#main-key
```

The `#main-key` fragment references the public key in the actor document.

### Verification Implementation

```go
func VerifyRequest(req *http.Request, publicKeyPem string) (string, error) {
    verifier, err := httpsig.NewVerifier(req)
    if err != nil {
        return "", fmt.Errorf("failed to create verifier: %w", err)
    }

    // Parse public key (supports PKIX and PKCS#1 formats)
    block, _ := pem.Decode([]byte(publicKeyPem))
    var rsaPubKey *rsa.PublicKey

    // Try PKIX format first (standard)
    pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        // Fallback to PKCS#1 (legacy)
        pkcs1Key, _ := x509.ParsePKCS1PublicKey(block.Bytes)
        rsaPubKey = pkcs1Key
    } else {
        rsaPubKey = pubKey.(*rsa.PublicKey)
    }

    // Verify signature
    err = verifier.Verify(rsaPubKey, httpsig.RSA_SHA256)
    if err != nil {
        return "", fmt.Errorf("signature verification failed: %w", err)
    }

    // Extract actor URI from keyId
    keyId := verifier.KeyId()
    actorURI := strings.Split(keyId, "#")[0]

    return actorURI, nil
}
```

### Key Format Compatibility

| Format | PEM Header | Support |
|--------|------------|---------|
| PKIX (PKCS#8) | `PUBLIC KEY` | Primary |
| PKCS#1 | `RSA PUBLIC KEY` | Fallback |

---

## Rate Limiting

### Architecture

Per-IP rate limiting using Go's `golang.org/x/time/rate` token bucket algorithm.

```go
type limiterEntry struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

type RateLimiter struct {
    limiters map[string]*limiterEntry
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    rl := &RateLimiter{
        limiters: make(map[string]*limiterEntry),
        rate:     r,
        burst:    b,
    }
    go rl.cleanupOldLimiters()
    return rl
}
```

### Rate Limits

| Context | Rate | Burst | Description |
|---------|------|-------|-------------|
| Global | 10 req/sec | 20 | All HTTP endpoints |
| ActivityPub | 5 req/sec | 10 | Federation endpoints |

### Setup in Router

```go
// Global rate limiter: 10 requests per second per IP, burst of 20
globalLimiter := NewRateLimiter(rate.Limit(10), 20)
g.Use(RateLimitMiddleware(globalLimiter))

// ActivityPub endpoints: 5 req/sec per IP
apLimiter := NewRateLimiter(rate.Limit(5), 10)
g.POST("/inbox", RateLimitMiddleware(apLimiter), ...)
```

### Middleware Implementation

```go
func RateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        limiter := rl.getLimiter(ip)

        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "Rate limit exceeded. Please try again later.",
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

### Memory Management

```go
func (rl *RateLimiter) cleanupOldLimiters() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        rl.mu.Lock()
        cutoff := time.Now().Add(-10 * time.Minute)
        for ip, entry := range rl.limiters {
            if entry.lastSeen.Before(cutoff) {
                delete(rl.limiters, ip)
            }
        }
        rl.mu.Unlock()
    }
}
```

| Property | Value |
|----------|-------|
| Cleanup interval | 5 minutes |
| Eviction threshold | 10 minutes inactive |
| Cleanup action | Per-entry eviction |

---

## Request Size Limits

### MaxBytesMiddleware

Limits request body size to prevent resource exhaustion.

```go
func MaxBytesMiddleware(maxBytes int64) gin.HandlerFunc {
    return func(c *gin.Context) {
        if c.Request.ContentLength > maxBytes {
            c.JSON(http.StatusRequestEntityTooLarge, gin.H{
                "error": "Request body too large",
            })
            c.Abort()
            return
        }

        c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
        c.Next()
    }
}
```

### Size Limits

| Endpoint | Limit | Reason |
|----------|-------|--------|
| ActivityPub inbox | 1 MB | Prevent oversized activities |
| General | None | No global limit |

### Usage

```go
maxBodySize := MaxBytesMiddleware(1 * 1024 * 1024) // 1MB
g.POST("/inbox", RateLimitMiddleware(apLimiter), maxBodySize, ...)
```

---

## RSA Key Management

### Key Generation

```go
func GeneratePemKeypair(keypair *RsaKeyPair) error {
    privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
    if err != nil {
        return err
    }

    // Private key in PKCS#8 format
    privateKeyBytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)
    keypair.PrivatePem = string(pem.EncodeToMemory(&pem.Block{
        Type:  "PRIVATE KEY",
        Bytes: privateKeyBytes,
    }))

    // Public key in PKIX format
    publicKeyBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
    keypair.PublicPem = string(pem.EncodeToMemory(&pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: publicKeyBytes,
    }))

    return nil
}
```

### Key Properties

| Property | Value |
|----------|-------|
| Algorithm | RSA |
| Key size | 4096 bits |
| Private format | PKCS#8 |
| Public format | PKIX |

---

## Security Response Codes

| Code | Meaning |
|------|---------|
| `401 Unauthorized` | Missing or invalid signature |
| `403 Forbidden` | Signature verification failed |
| `413 Request Entity Too Large` | Body exceeds size limit |
| `429 Too Many Requests` | Rate limit exceeded |

---

## Error Messages

### Rate Limiting

```json
{
    "error": "Rate limit exceeded. Please try again later."
}
```

### Body Size

```json
{
    "error": "Request body too large"
}
```

---

## Security Best Practices

### Deployment Checklist

| Item | Recommendation |
|------|----------------|
| HTTPS | Required for ActivityPub |
| Reverse proxy | Caddy/Nginx recommended |
| Rate limits | Keep defaults |
| Key rotation | Not implemented |
| Audit logging | Via standard logging |

### Known Limitations

| Area | Limitation |
|------|------------|
| Key hashing | No salt (TODO) |
| Rate limiting | Simple reset, no gradual expiry |
| Key rotation | Not supported |
| Replay protection | Via digest verification only |

---

## Source Files

- `util/util.go` - `PkToHash()` function
- `activitypub/httpsig.go` - HTTP signature implementation
- `web/middleware.go` - Rate limiting, body size middleware
- `web/router.go` - Middleware application
