# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

**stegodon** is an SSH-first fediverse multi-user blog written in Go using [Charm Tools](https://github.com/charmbracelet). Users connect via SSH and create notes in a terminal interface. Notes can be subscribed to via RSS and federate via ActivityPub to the Fediverse (Mastodon, Pleroma, etc.) and optionally viewed in a web browser.

## Build and Run Commands

```bash
# Build and run
go build && ./stegodon

# Run tests
go test ./...

# Development workflow (always run after changes)
go clean && go test ./... && go build

# Run with ActivityPub enabled
STEGODON_WITH_AP=true STEGODON_SSLDOMAIN=yourdomain.com ./stegodon

# Run in single-user mode
STEGODON_SINGLE=true ./stegodon

# Run with closed registration
STEGODON_CLOSED=true ./stegodon

# Run in SSH-only mode (no web UI, but RSS/ActivityPub still work)
STEGODON_SSH_ONLY=true ./stegodon
```

## Configuration

Environment variables:
- `STEGODON_HOST` - Server IP (default: 127.0.0.1)
- `STEGODON_SSHPORT` - SSH port (default: 23232)
- `STEGODON_HTTPPORT` - HTTP port (default: 9999)
- `STEGODON_SSLDOMAIN` - Public domain for ActivityPub (default: example.com)
- `STEGODON_WITH_AP` - Enable ActivityPub (default: false)
- `STEGODON_SINGLE` - Single-user mode (default: false)
- `STEGODON_CLOSED` - Close registration (default: false)
- `STEGODON_SSH_ONLY` - SSH-only mode, disables web UI but keeps RSS/ActivityPub (default: false)
- `STEGODON_NODE_DESCRIPTION` - NodeInfo description
- `STEGODON_MAX_CHARS` - Maximum note length (default: 150, max: 300)
- `STEGODON_SHOW_GLOBAL` - Show global timeline in TUI and web (default: false)
- `STEGODON_SHOW_TOS` - Show terms of service acceptance screen (default: false)
- `STEGODON_WITH_JOURNALD` - Linux journald logging (default: false)
- `STEGODON_WITH_PPROF` - Enable pprof on localhost:6060 (default: false)

File locations:
- Config: `~/.config/stegodon/config.yaml` (or `./config.yaml`)
- Database: `~/.config/stegodon/database.db` (or `./database.db`)
- SSH key: `~/.config/stegodon/.ssh/stegodonhostkey`

## Architecture

### Application Lifecycle

The application uses a structured lifecycle pattern in `app/app.go`:
- `App` struct encapsulates config, SSH server, and HTTP server
- `New()` creates the app instance
- `Initialize()` runs migrations and sets up servers
- `Start()` starts servers and blocks until shutdown signal
- `Shutdown()` gracefully stops HTTP then SSH with 30s timeout

### Dual Server Model

The application runs two concurrent servers:
- **SSH Server** (port 23232): TUI client connections via [wish](https://github.com/charmbracelet/wish)
- **HTTP Server** (port 9999): RSS feeds, web UI, and ActivityPub endpoints via [gin](https://github.com/gin-gonic/gin)

Both servers support graceful shutdown on SIGTERM/SIGINT.

### TUI Architecture

Built with [bubbletea](https://github.com/charmbracelet/bubbletea) MVC pattern. Main orchestrator in `ui/supertui.go`.

**Views:**
- `createuser` - First-time username selection
- `writenote` - Note creation (with reply mode)
- `myposts` - User's own notes with edit/delete
- `hometimeline` - Combined local + federated timeline
- `threadview` - Thread/conversation view
- `followuser` - Follow remote users
- `followers` / `following` - Relationship lists
- `localusers` - Browse local users
- `notifications` - View and manage notifications
- `relay` - Manage ActivityPub relay subscriptions (admin)
- `admin` - Admin panel
- `accountsettings` - Profile editing, avatar upload, account deletion

**Navigation:** Tab cycles forward, Shift+Tab backward. Press Ctrl+N for notifications. Enter opens threads, Esc returns.

### Database Layer

SQLite with WAL mode. Singleton pattern with connection pooling (max 25 connections).

**Core tables:** `accounts`, `notes`, `hashtags`, `note_hashtags`

**ActivityPub tables:** `follows`, `remote_accounts`, `activities`, `likes`, `boosts`, `delivery_queue`, `note_mentions`, `relays`

**Denormalized counters:** `reply_count`, `like_count`, `boost_count` on notes and activities for performance.

### ActivityPub Layer

Located in `activitypub/`:
- `httpsig.go` - RSA-SHA256 HTTP signature signing/verification
- `actors.go` - Remote actor fetching and caching (24h TTL)
- `inbox.go` - Incoming activity processing
- `outbox.go` - Outgoing activity sending
- `delivery.go` - Background queue worker with exponential backoff
- `deps.go` - Database and HTTP client interfaces
- `db_wrapper.go` - Production database adapter

### Web Layer

Located in `web/`:
- `router.go` - Gin HTTP routing
- `rss.go` - RSS feed generation
- `handlers.go` - Web UI handlers
- `activitypub.go` - ActivityPub endpoint handlers

## Directory Structure

```
stegodon/
├── app/             # Application lifecycle (App struct, Start, Shutdown)
├── activitypub/     # ActivityPub federation protocol
├── db/              # Database layer (SQLite operations, migrations)
├── domain/          # Domain models (Account, Note, Activity, Relay, etc.)
├── middleware/      # SSH middleware (auth, TUI handler)
├── remote/          # Remote utilities
├── ui/              # TUI components
│   ├── common/      # Shared styles, commands, session states, layout
│   ├── createuser/  # Username selection
│   ├── writenote/   # Note creation
│   ├── myposts/     # User's notes
│   ├── hometimeline/# Combined timeline
│   ├── threadview/  # Thread display
│   ├── followuser/  # Follow interface
│   ├── followers/   # Followers list
│   ├── following/   # Following list
│   ├── localusers/  # Local user browser
│   ├── relay/       # Relay management (admin)
│   ├── admin/       # Admin panel
│   ├── header/      # Navigation bar
│   ├── accountsettings/ # Profile and account settings
│   └── notifications/ # Notifications view
├── util/            # Utilities (config, crypto, helpers)
├── web/             # HTTP server (RSS, ActivityPub, web UI)
│   ├── templates/   # HTML templates (embedded)
│   └── static/      # Static assets (embedded)
└── main.go          # Entry point
```

## Key Patterns

### Auto-Refresh View Pattern

Timeline views that auto-refresh must track active state to prevent goroutine leaks:

```go
type Model struct {
    isActive bool
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case common.ActivateViewMsg:
        m.isActive = true
        return m, tea.Batch(loadData(), tickRefresh())
    case common.DeactivateViewMsg:
        m.isActive = false
        return m, nil
    case refreshTickMsg:
        if m.isActive {
            return m, tea.Batch(loadData(), tickRefresh())
        }
        return m, nil  // Stop ticker chain
    }
}
```

### Dependency Injection for Testing

ActivityPub handlers use interfaces for testability:

```go
type Database interface {
    ReadAccByUsername(username string) (error, *domain.Account)
    // ... other methods
}

type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

// Production: DBWrapper wraps real database
// Testing: MockDatabase with configurable behavior
```

## ActivityPub Features

**Implemented:**
- Follow/unfollow remote users (WebFinger discovery)
- Auto-accept incoming follows
- Federate notes to followers
- Receive posts from followed accounts
- HTTP signature authentication
- Delivery queue with exponential backoff
- Actor caching (24h TTL)
- Replies and threading (inReplyTo)
- Recursive reply counts
- Like/unlike posts
- Content warnings
- Hashtag parsing
- Relay subscriptions (FediBuzz, YUKIMOCHI)
- Relay pause/resume and content deletion

**Protocol Support:**
- Follow/Accept/Undo activities
- Create/Update/Delete activities
- Like activities
- Announce activities (relay content)
- WebFinger, NodeInfo 2.0/2.1

## Relay Support

Two relay types supported:

**FediBuzz** (hashtag-based):
- Subscribe to `https://relay.fedi.buzz/tag/music`
- Content wrapped in Announce activities

**YUKIMOCHI** (firehose):
- Subscribe to `https://relay.toot.yukimochi.jp`
- Raw Create activities forwarded
- Uses shared inbox (`/inbox`)
- Follow object: `https://www.w3.org/ns/activitystreams#Public`

**Relay features:**
- Pause/resume individual relays (paused relays log but don't save content)
- Delete all relay content from timeline
- Signature verification against relay's key (signer differs from actor)

**Relay management keys (admin panel):**
- `a` - Add relay
- `d` - Delete/unsubscribe
- `p` - Pause/resume
- `r` - Retry failed
- `x` - Clear all relay content

## Test Coverage

Run tests: `go test ./...`
Run with coverage: `go test ./... -cover`

Key coverage areas:
- `domain` - 100% (data structures)
- `createuser` - 87% (username selection)
- `hometimeline` - 78% (timeline view)
- `myposts` - 67% (user notes)
- `util` - 63% (crypto, config)
- `activitypub` - 60% (federation)
- `threadview` - 51% (thread display)
- `db` - 38% (database ops)
- `writenote` - 37% (note creation)

## Development Notes

- Go 1.25+
- Single binary distribution (assets embedded via `embed` package)
- SSH host key auto-generated on first run
- Terminal: 24-bit color, minimum 115x28
- Public keys SHA256-hashed before storage

### SQLite Timestamp Handling (IMPORTANT)

SQLite stores timestamps without timezone info. When reading back, the Go SQLite driver adds a "Z" suffix (e.g., `2026-01-21T01:30:04Z`), but these are **local time, not UTC**.

**Critical**: Always handle Z-suffix timestamps BEFORE RFC3339 parsing. The `parseTimestamp()` function in `db/db.go` strips the Z suffix and parses as local time. If you parse with RFC3339 first, "Z" is interpreted as UTC, causing timestamps to be off by the timezone offset (e.g., 1 hour in CET).

```go
// WRONG: RFC3339 treats Z as UTC
time.Parse(time.RFC3339, "2026-01-21T01:30:04Z") // → 01:30 UTC = 02:30 CET

// CORRECT: Strip Z and parse as local
time.ParseInLocation("2006-01-02 15:04:05", "2026-01-21 01:30:04", time.Local) // → 01:30 CET
```

**Symptoms of incorrect parsing**: `time.Since()` returns negative values (time in future), causing "just now" to display for old timestamps.

## Documentation

- [DATABASE.md](DATABASE.md) - Schema and tables
- [FEDERATION.md](FEDERATION.md) - ActivityPub details
- [DOCKER.md](DOCKER.md) - Docker deployment
- [README.md](README.md) - User documentation
