# stegodon

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/deemkeen/stegodon)](https://github.com/deemkeen/stegodon/releases) [![Go Version](https://img.shields.io/github/go-mod/go-version/deemkeen/stegodon)](https://github.com/deemkeen/stegodon/blob/main/go.mod) [![License](https://img.shields.io/github/license/deemkeen/stegodon)](https://github.com/deemkeen/stegodon/blob/main/LICENSE) [![Docker Build](https://github.com/deemkeen/stegodon/actions/workflows/docker.yml/badge.svg)](https://github.com/deemkeen/stegodon/actions/workflows/docker.yml) [![Release](https://github.com/deemkeen/stegodon/actions/workflows/release.yml/badge.svg)](https://github.com/deemkeen/stegodon/actions/workflows/release.yml)

**stegodon** is an SSH-first federated blogging platform. Users connect via SSH to create notes in a terminal interface. Notes federate to the Fediverse via ActivityPub and are available through RSS feeds and a web interface.

Built with Go and [Charm Tools](https://github.com/charmbracelet).

## Showtime

![demo](./demo.gif)

## Features

- **SSH-First TUI** - Connect via SSH, authenticate with your public key, create notes in a beautiful terminal interface
- **ActivityPub Federation** - Follow/unfollow users, federate posts to Mastodon/Pleroma with HTTP signatures
- **Relay Support** - Subscribe to ActivityPub relays (FediBuzz, YUKIMOCHI) to discover content beyond direct follows
- **Threading & Replies** - Reply to posts, view threaded conversations with recursive reply counts
- **Mentions** - Tag users with `@username@domain`, autocomplete suggestions, highlighted in TUI/web
- **Hashtags** - Use `#tags` in your posts, highlighted in TUI and stored for discovery
- **RSS Feeds** - Per-user and aggregated feeds with full content
- **Web Interface** - Browse posts with terminal-themed design and SEO optimization
- **Multi-User** - Admin panel, user management, single-user mode, closed registration
- **Markdown Links** - Clickable links in TUI (OSC 8), web UI, and federation: `[text](url)`

## Quick Start

**Docker (Recommended):**
```bash
# Create a directory for Stegodon
sudo mkdir -p /srv/stegodon
cd /srv/stegodon

# Download the docker-compose file
curl -LO https://raw.githubusercontent.com/deemkeen/stegodon/main/docker-compose.yml

# Edit the configuration to suit your server
nano docker-compose.yml

# Start Stegodon
docker compose up -d
```

**Binary:**
```bash
# Download the binary from GitHub Releases
chmod +x stegodon

# Check version
./stegodon -v

# Run
./stegodon
```

**Connect via SSH:**
```bash
ssh 127.0.0.1 -p 23232
```

On first login, choose your username. All data is stored in `~/.config/stegodon/` (or Docker volume).

See [DOCKER.md](DOCKER.md) for complete Docker deployment guide.

## Navigation

- **Tab** - Cycle through views
- **Shift+Tab** - Cycle through views in reverse order
- **Ctrl+N** - Jump to notifications view
- **Up/Down** or **j/k** - Navigate lists
- **Enter** - Open thread view for posts with replies (or delete notification in notifications view)
- **Esc** - Return from thread view
- **r** - Reply to selected post
- **l** - Like/unlike selected post (federated)
- **o** - Toggle URL display for selected post (home timeline)
  - Press once: Show clickable URL
  - Press again or navigate: Show post content
  - Cmd+click (Mac) or Ctrl+click (Linux) URL to open in local browser
- **u** - Edit note (in my posts)
- **d** - Delete note with confirmation
- **a** - Delete all notifications (in notifications view)
- **Ctrl+S** - Save/post note
- **Ctrl+C** or **q** - Quit

## Configuration

Environment variables override embedded defaults:

```bash
# Basic settings
STEGODON_HOST=0.0.0.0             # Server IP (use 127.0.0.1 to prevent remote connections)
STEGODON_SSHPORT=23232            # SSH port
STEGODON_HTTPPORT=9999            # HTTP port

# ActivityPub federation
STEGODON_WITH_AP=true             # Enable federation
STEGODON_SSLDOMAIN=yourdomain.com # Your public domain (required for ActivityPub)

# Access control
STEGODON_SINGLE=true              # Single-user mode
STEGODON_CLOSED=true              # Closed registration

# Customization
STEGODON_NODE_DESCRIPTION="My personal microblog server"  # NodeInfo description
STEGODON_MAX_CHARS=200            # Default is 150, max length is capped to 300 characters

# Logging (Linux only)
STEGODON_WITH_JOURNALD=true       # Send logs to systemd journald

# Profiling (development/debugging)
STEGODON_WITH_PPROF=true          # Enable pprof profiler on localhost:6060
```

**File locations:**
- Config: `./config.yaml` -> `~/.config/stegodon/config.yaml` -> embedded defaults
- Database: `./database.db` -> `~/.config/stegodon/database.db`
- SSH key: `./.ssh/stegodonhostkey` -> `~/.config/stegodon/.ssh/stegodonhostkey`

**Viewing logs (Linux with journald):**
```bash
# Follow logs in real-time
journalctl -t stegodon -f

# View recent logs
journalctl -t stegodon --since "1 hour ago"

# View logs for a specific service
journalctl -u stegodon.service -f
```

**Profiling (when STEGODON_WITH_PPROF=true):**
```bash
# Access web UI
open http://localhost:6060/debug/pprof/

# CPU profile (30 seconds)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine count
curl http://localhost:6060/debug/pprof/goroutine?debug=1 | grep "goroutine profile"
```

## ActivityPub Setup

1. Set `STEGODON_WITH_AP=true` and `STEGODON_SSLDOMAIN=yourdomain.com`
2. Make your server publicly accessible with HTTPS
3. Proxy HTTP port (9999) through nginx/caddy with TLS
4. Follow users: Go to the "Follow" view, enter `username@domain.com`

**Your profile:** `https://yourdomain.com/users/<username>`

## Relay Subscriptions

Relays let you discover content from across the Fediverse without following individual users. Admin users can manage relays from the admin panel.

**Supported relays:**
- **FediBuzz** - Hashtag-based (e.g., `relay.fedi.buzz/tag/music`)
- **YUKIMOCHI** - Full firehose (e.g., `relay.toot.yukimochi.jp`)

**Relay controls:**
- `a` - Add relay (enter URL or domain)
- `d` - Unsubscribe from relay
- `p` - Pause/resume relay (paused relays log but don't save content)
- `r` - Retry failed subscription
- `x` - Delete all relay content from timeline

## RSS Feeds

- Personal: `http://localhost:9999/feed?username=<user>`
- Aggregated: `http://localhost:9999/feed`
- Single note: `http://localhost:9999/feed/<uuid>`

## Web UI

Browse posts through a terminal-themed web interface:

- **Homepage:** `http://localhost:9999/` - View all posts from all users
- **User profile:** `http://localhost:9999/users/<username>` - View posts by a specific user
- **Single post:** `http://localhost:9999/posts/<uuid>` - View individual post with thread context

The web UI features:
- Terminal-style aesthetic matching the SSH TUI
- SEO optimized with proper meta tags
- Clickable Markdown links
- Responsive design
- RSS feed links for each user

Replace `localhost:9999` with your domain when deployed publicly.

## Building from Source

```bash
git clone https://github.com/deemkeen/stegodon
cd stegodon
go build
./stegodon
```

**Requirements:**
- Go 1.25+
- Terminal with 24-bit color, 115x28 minimum
- OSC 8 support for clickable links (optional: Ghostty, iTerm2, Kitty)

## Tech Stack

- **SSH:** [wish](https://github.com/charmbracelet/wish)
- **TUI:** [bubbletea](https://github.com/charmbracelet/bubbletea), [lipgloss](https://github.com/charmbracelet/lipgloss)
- **Web:** [gin](https://github.com/gin-gonic/gin)
- **Database:** SQLite with WAL mode
- **Federation:** Custom ActivityPub implementation with HTTP signatures

## CLI Mode

Stegodon supports non-interactive CLI commands for scripting and automation:

```bash
ssh -p 23232 localhost post "Hello world"
ssh -p 23232 localhost timeline -j
echo "Content" | ssh -p 23232 localhost post -
```

All commands support `--json` / `-j` for machine-readable output. See [cli/CLI.md](cli/CLI.md) for full documentation.

## Documentation

- [cli/CLI.md](cli/CLI.md) - CLI mode commands and JSON output
- [DATABASE.md](DATABASE.md) - Database schema and tables
- [FEDERATION.md](FEDERATION.md) - ActivityPub federation details
- [DOCKER.md](DOCKER.md) - Docker deployment guide

For detailed technical specifications covering the TUI architecture, ActivityPub federation, database schema, and all subsystems, see [specs/overview.md](specs/overview.md).

## License

MIT - See LICENSE file

## Contributing

Contributions welcome! Open an issue or pull request on [GitHub](https://github.com/deemkeen/stegodon).
