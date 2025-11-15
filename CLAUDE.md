# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**stegodon** is a ssh-first fediverse multi-user blog written in Go using [Charm Tools](https://github.com/charmbracelet). Users connect via SSH and create notes in a terminal interface. Notes can be subscribed to via RSS and federate via ActivityPub to the Fediverse (Mastodon, Pleroma, etc.) and optionally viewed in a web browser.

## Build and Run Commands

```bash
# Install dependencies
go get -d

# Build the application
go build

# Run the application
./stegodon

# Run directly without building
go run main.go

# Run with ActivityPub enabled
STEGODON_WITH_AP=true STEGODON_SSLDOMAIN=yourdomain.com ./stegodon

# Run in single-user mode
STEGODON_SINGLE=true ./stegodon

# Run with closed registration
STEGODON_CLOSED=true ./stegodon
```

## Development Workflow

**IMPORTANT**: After making any changes to the project, always run:

```bash
go clean && go test ./... && go build
```

This ensures:
1. `go clean` - Removes build artifacts and cached files
2. `go test ./...` - Runs all 155+ unit tests to verify nothing broke
3. `go build` - Compiles the application only if tests pass

The test suite covers critical functionality:
- **domain**: 100% coverage (data structures)
- **util**: 91.2% coverage (crypto, config, helpers)
- **db**: 31.7% coverage (database operations)
- **web**: 18.3% coverage (RSS, WebFinger, UI handlers)
- **activitypub**: 9.5% coverage (inbox/outbox, actors)

## Configuration

Configuration is managed via environment variables:
- `STEGODON_HOST` - Server IP (default: 127.0.0.1)
- `STEGODON_SSHPORT` - SSH login port (default: 23232)
- `STEGODON_HTTPPORT` - HTTP port (default: 9999)
- `STEGODON_SSLDOMAIN` - **Required for ActivityPub** - Your public domain (default: example.com)
- `STEGODON_WITH_AP` - Enable ActivityPub functionality (default: false)
- `STEGODON_SINGLE` - Enable single-user mode (default: false)
- `STEGODON_CLOSED` - Close registration for new users (default: false)

Default configuration is in `config.yaml`.

**For ActivityPub to work**, you must:
1. Set `STEGODON_WITH_AP=true`
2. Set `STEGODON_SSLDOMAIN` to your actual domain
3. Have your domain publicly accessible with proper DNS
4. Proxy the HTTP port (9999) through a reverse proxy with TLS

**For single-user mode**:
- Set `STEGODON_SINGLE=true` to restrict registration to only one user
- When enabled, only the first user can register
- Additional SSH connection attempts are rejected with the message: "This blog is in single-user mode, but you can host your own stegodon!"
- Useful for personal blogs where you want to prevent other users from registering

**For closed registration**:
- Set `STEGODON_CLOSED=true` to completely close registration
- When enabled, no new users can register (regardless of current user count)
- SSH connection attempts from new users are rejected with the message: "Registration is closed. Please contact the administrator."
- Existing users can continue to log in normally
- Useful for invite-only or maintenance periods

## Architecture

### Application Flow

1. **Dual Server Model**: The application runs two concurrent servers:
   - **SSH Server** (port 23232): Handles TUI client connections
   - **HTTP Server** (port 9999): Serves RSS feeds and ActivityPub endpoints

2. **Authentication Flow** (`middleware/auth.go`):
   - Users authenticate via SSH public key
   - Existing users with muted status are rejected
   - In closed mode (`STEGODON_CLOSED=true`), all new registrations are blocked
   - In single-user mode (`STEGODON_SINGLE=true`), new registrations are blocked if one user already exists
   - On first connection, a random username is generated
   - On first login, users are prompted to choose their username (`ui/createuser/`)
   - Public keys are hashed and stored in SQLite for user identification
   - Each account gets an RSA keypair for ActivityPub functionality

3. **TUI Architecture** (`ui/supertui.go`):
   - Built with [bubbletea](https://github.com/charmbracelet/bubbletea) MVC pattern
   - **5-view navigation system**:
     - `createuser.Model`: First-time username selection
     - `writenote.Model`: Note creation interface (key 1)
     - `listnotes.Model`: Local notes viewing with pagination (key 2)
     - `followuser.Model`: Follow remote ActivityPub users (key 3)
     - `followers.Model`: View followers list (key 4)
     - `timeline.Model`: Federated timeline from followed accounts (key 5)
   - **Hybrid navigation**: Tab cycles through views, number keys 1-5 jump directly
   - State machine driven by `common.SessionState` enum

4. **Database Layer** (`db/db.go`):
   - SQLite database (`database.db`) with WAL mode enabled
   - **Singleton pattern** with connection pooling (max 25 connections)
   - **Core tables**: `accounts`, `notes`
   - **ActivityPub tables**: `follows`, `remote_accounts`, `activities`, `likes`, `delivery_queue`
   - Extended accounts with: `display_name`, `summary`, `avatar_url`, RSA keypairs
   - Extended notes with: `visibility`, `in_reply_to_uri`, `object_uri`, `federated`
   - Transactions use retry logic to handle SQLite busy states

5. **Web Layer** (`web/router.go`):
   - Gin framework handles HTTP routing
   - **RSS feed endpoints**:
     - `/feed` - All user notes
     - `/feed?username=<user>` - User-specific feed
     - `/feed/:id` - Single note by UUID
   - **ActivityPub endpoints** (enabled via `STEGODON_WITH_AP=true`):
     - `/users/:actor` - Actor profile (application/activity+json)
     - `/.well-known/webfinger` - WebFinger discovery (application/json)
     - `/users/:actor/inbox` - Receive activities (Follow, Undo, Create, Like)
     - `/users/:actor/outbox` - Activity stream (placeholder)
     - `/users/:actor/followers` - Followers collection (placeholder)
     - `/users/:actor/following` - Following collection (placeholder)

6. **ActivityPub Layer** (`activitypub/`):
   - **HTTP Signatures** (`httpsig.go`): RSA-SHA256 signing and verification
   - **Actor Discovery** (`actors.go`): Fetch and cache remote actors, WebFinger resolution
   - **Inbox Handler** (`inbox.go`): Process Follow, Undo, Create, Like activities
   - **Outbox Handler** (`outbox.go`): Send Accept, Create, Follow activities
   - **Delivery Queue** (`delivery.go`): Background worker with exponential backoff (1m → 24h)

### Key Data Flow

**Creating a Note:**
1. User types in `writenote` TUI component
2. On submit, note is saved via `db.CreateNote()` with user UUID and timestamp
3. `UpdateNoteList` message triggers `listnotes` model refresh
4. Note immediately appears in list panel

**RSS Feed Generation:**
1. HTTP request to `/feed` or `/feed?username=X`
2. `web/rss.go` queries notes from database
3. Notes formatted as RSS 2.0 XML using `gorilla/feeds` library
4. Response served with `application/xml` content-type

**SSH Connection:**
1. `wish` server accepts connection with public key auth
2. `AuthMiddleware` creates/retrieves account from public key hash
3. `MainTui` middleware initializes bubbletea program with user's account
4. TUI renders based on `FirstTimeLogin` flag

**ActivityPub Federation (Note Publishing):**
1. User posts note via TUI (Ctrl+S in writenote view)
2. Note saved to database with `db.CreateNote()`
3. Background goroutine triggers `activitypub.SendCreate()`
4. Create activity generated with note content and addressing
5. Activity queued to `delivery_queue` for each follower's inbox
6. Delivery worker processes queue every 10s with retry logic
7. HTTP POST sent to remote inboxes with signed requests

**ActivityPub Federation (Following Users):**
1. User enters `user@domain.com` in follow view (key 3)
2. WebFinger resolves to ActivityPub actor URI
3. `activitypub.SendFollow()` sends Follow activity to remote inbox
4. Remote server sends Accept activity back
5. Follow relationship stored in `follows` table
6. Remote actor cached in `remote_accounts` table (24h TTL)

**ActivityPub Federation (Receiving Posts):**
1. Remote server POSTs Create activity to `/users/:actor/inbox`
2. HTTP signature verified against remote actor's public key
3. Activity parsed and stored in `activities` table
4. Federated timeline view (key 5) displays recent Create activities
5. Posts shown with actor name, content, and relative timestamp

### Directory Structure

- `db/` - Database layer with SQLite operations
  - `migrations.go` - ActivityPub table creation and schema extensions
- `domain/` - Domain models (Account, Note, RemoteAccount, Follow, Activity, Like, DeliveryQueueItem)
- `activitypub/` - ActivityPub federation protocol
  - `actors.go` - Remote actor fetching and caching
  - `httpsig.go` - HTTP signature signing and verification
  - `inbox.go` - Incoming activity processing
  - `outbox.go` - Outgoing activity sending
  - `delivery.go` - Background delivery queue worker
- `middleware/` - SSH middleware (auth, TUI handler)
- `ui/` - TUI components:
  - `common/` - Shared styles, commands, session states
  - `createuser/` - Username selection screen
  - `followuser/` - Follow remote users interface
  - `followers/` - Followers list display
  - `timeline/` - Federated timeline view
  - `header/` - Top navigation bar
  - `listnotes/` - Note list with pagination
  - `writenote/` - Note creation textarea
  - `supertui.go` - Main TUI orchestrator with 5-view navigation
- `util/` - Utilities (config, crypto, helpers)
- `web/` - HTTP server (RSS, ActivityPub, routing)

## Development Notes

- Go version: 1.25 (updated from 1.19)
- **Test suite**: 155+ passing unit tests covering all critical functionality
- The `.ssh/hostkey` file is auto-generated on first run via `util.GeneratePemKeypair()`
- Database file `database.db` is created in working directory
- Terminal requirements: 24-bit color support, minimum 115 cols x 28 rows
- Public keys are hashed with SHA256 before storage for privacy

## Recent Updates (2025)

The project has been fully updated with ActivityPub federation support:
- **SSH Migration**: Migrated from `gliderlabs/ssh` to `charmbracelet/ssh` (required by newer wish versions)
- **Charm Tools**: Updated to latest stable releases (bubbletea v1.3.10, bubbles v0.21.0, lipgloss v1.1.0, wish v1.4.7)
- **ActivityPub Implementation**: Full Fediverse integration (6 phases over 8 weeks)
  - Phase 1: Database foundation with 5 new tables
  - Phase 2: HTTP signatures and actor discovery
  - Phase 3: Core protocol (inbox/outbox/delivery queue)
  - Phase 4: TUI integration (follow users, federated notes)
  - Phase 5: Federated timeline and followers list
  - Phase 6: Polish and configuration fixes
- **Database Optimization**: WAL mode with connection pooling for concurrent access
- **Navigation Enhancement**: Hybrid Tab + number keys (1-5) navigation system

## ActivityPub Features

**Implemented:**
- ✅ Follow/unfollow remote users via WebFinger (user@domain format)
- ✅ Accept incoming follow requests automatically
- ✅ Federate posted notes to all followers
- ✅ Receive and display posts from followed accounts
- ✅ HTTP signature authentication (RSA-SHA256)
- ✅ Reliable delivery with exponential backoff retry
- ✅ Actor caching with 24-hour TTL
- ✅ 5-view TUI navigation
- ✅ Federated timeline display
- ✅ Followers list view

**Protocol Support:**
- Follow/Accept/Undo activities (full support)
- Create activities (send and receive)
- Like activities (receive only, display pending)
- WebFinger discovery
- Actor profiles (JSON-LD)

**Not Yet Implemented:**
- Likes/favorites sending
- Boosts/announces
- Replies/threading
- Media attachments
- Content warnings
- Hashtags
- Search
- Notifications
- Block/mute functionality

      IMPORTANT: this context may or may not be relevant to your tasks. You should not respond to this context unless it is highly relevant to your task.
