# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**stegodon** is a single-user/multi-user blog TUI application written in Go using [Charm Tools](https://github.com/charmbracelet). Users connect via SSH and create notes in a terminal interface. Notes can be subscribed to via RSS, with experimental ActivityPub federation support (WIP).

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
```

## Configuration

Configuration is managed via environment variables:
- `STEGODON_HOST` - Server IP (default: 127.0.0.1)
- `STEGODON_SSHPORT` - SSH login port (default: 23232)
- `STEGODON_HTTPPORT` - HTTP port (default: 9999)
- `STEGODON_SSLDOMAIN` - SSL domain for ActivityPub (default: example.com)
- `STEGODON_WITH_AP` - Enable ActivityPub functionality (default: false)

Default configuration is in `config.yaml`.

## Architecture

### Application Flow

1. **Dual Server Model**: The application runs two concurrent servers:
   - **SSH Server** (port 23232): Handles TUI client connections
   - **HTTP Server** (port 9999): Serves RSS feeds and ActivityPub endpoints

2. **Authentication Flow** (`middleware/auth.go`):
   - Users authenticate via SSH public key
   - On first connection, a random username is generated
   - On first login, users are prompted to choose their username (`ui/createuser/`)
   - Public keys are hashed and stored in SQLite for user identification
   - Each account gets an RSA keypair for ActivityPub functionality

3. **TUI Architecture** (`ui/supertui.go`):
   - Built with [bubbletea](https://github.com/charmbracelet/bubbletea) MVC pattern
   - Main model manages three sub-models:
     - `createuser.Model`: First-time username selection
     - `writenote.Model`: Note creation interface (left panel)
     - `listnotes.Model`: Note viewing/pagination (right panel)
   - Users toggle between panels with TAB key
   - State machine driven by `common.SessionState` enum

4. **Database Layer** (`db/db.go`):
   - SQLite database (`database.db`) stores all data
   - Two main tables: `accounts` and `notes`
   - Database connections are created per-operation (see TODO at line 245)
   - Transactions use retry logic to handle SQLite busy states

5. **Web Layer** (`web/router.go`):
   - Gin framework handles HTTP routing
   - RSS feed endpoints:
     - `/feed` - All user notes
     - `/feed?username=<user>` - User-specific feed
     - `/feed/:id` - Single note by UUID
   - ActivityPub endpoints (experimental, enabled via `STEGODON_WITH_AP`):
     - `/users/:actor` - Actor profile
     - `/.well-known/webfinger` - WebFinger discovery
     - `/users/:actor/inbox`, `/users/:actor/outbox`, etc.

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

### Directory Structure

- `db/` - Database layer with SQLite operations
- `domain/` - Domain models (Account, Note)
- `middleware/` - SSH middleware (auth, TUI handler)
- `ui/` - TUI components:
  - `common/` - Shared styles, commands, session states
  - `createuser/` - Username selection screen
  - `header/` - Top navigation bar
  - `listnotes/` - Note list with pagination
  - `writenote/` - Note creation textarea
  - `supertui.go` - Main TUI orchestrator
- `util/` - Utilities (config, crypto, helpers)
- `web/` - HTTP server (RSS, ActivityPub, routing)

## Development Notes

- Go version: 1.25 (updated from 1.19)
- No test files exist in the codebase currently
- The `.ssh/hostkey` file is auto-generated on first run via `util.GeneratePemKeypair()`
- Database file `database.db` is created in working directory
- Terminal requirements: 24-bit color support, minimum 160 cols x 50 rows
- Public keys are hashed with SHA256 before storage for privacy

## Recent Updates (2025)

The project has been updated to use the latest stable versions of all dependencies:
- **SSH Migration**: Migrated from `gliderlabs/ssh` to `charmbracelet/ssh` (required by newer wish versions)
- All Charm tools updated to latest stable releases (bubbletea v1.3.10, bubbles v0.21.0, lipgloss v1.1.0, wish v1.4.7)
- **Note**: Charm v2 releases exist (bubbletea v2.0.0-rc.1, bubbles v2.0.0-beta.1, lipgloss v2.0.0-beta.3) but are not yet stable
