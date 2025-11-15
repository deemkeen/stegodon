![demo](./demo.gif)

# 🦣 stegodon

**stegodon** is an SSH-first federated blogging platform. Users connect via SSH to create notes in a terminal interface. Notes federate to the Fediverse via ActivityPub and are available through RSS feeds and a web interface.

Built with Go and [Charm Tools](https://github.com/charmbracelet).

## Features

- **SSH-First TUI** - Connect via SSH, authenticate with your public key, create notes in a beautiful terminal interface
- **ActivityPub Federation** - Follow/unfollow users, federate posts to Mastodon/Pleroma with HTTP signatures
- **RSS Feeds** - Per-user and aggregated feeds with full content
- **Web Interface** - Browse posts with terminal-themed design and SEO optimization
- **Multi-User** - Admin panel, user management, single-user mode, closed registration
- **Markdown Links** - Clickable links in TUI (OSC 8), web UI, and federation: `[text](url)`

## Quick Start

**Docker (Recommended):**
```bash
docker pull ghcr.io/deemkeen/stegodon:latest
docker-compose up -d
```

**Binary:**
```bash
# Download the binary from GitHub Releases
chmod +x stegodon
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
- **↑/↓** or **j/k** - Navigate lists
- **u** - Edit note (in list)
- **d** - Delete note with confirmation
- **Ctrl+S** - Save/post note
- **Ctrl+C** or **q** - Quit

## Configuration

Environment variables override embedded defaults:

```bash
# Basic settings
STEGODON_HOST=127.0.0.1          # Server IP
STEGODON_SSHPORT=23232            # SSH port
STEGODON_HTTPPORT=9999            # HTTP port

# ActivityPub federation
STEGODON_WITH_AP=true             # Enable federation
STEGODON_SSLDOMAIN=yourdomain.com # Your public domain (required for ActivityPub)

# Access control
STEGODON_SINGLE=true              # Single-user mode
STEGODON_CLOSED=true              # Closed registration
```

**File locations:**
- Config: `./config.yaml` → `~/.config/stegodon/config.yaml` → embedded defaults
- Database: `./database.db` → `~/.config/stegodon/database.db`
- SSH key: `./.ssh/stegodonhostkey` → `~/.config/stegodon/.ssh/stegodonhostkey`

## ActivityPub Setup

1. Set `STEGODON_WITH_AP=true` and `STEGODON_SSLDOMAIN=yourdomain.com`
2. Make your server publicly accessible with HTTPS
3. Proxy HTTP port (9999) through nginx/caddy with TLS
4. Follow users: Go to the "Follow" view, enter `username@domain.com`

**Your profile:** `https://yourdomain.com/users/<username>`

## RSS Feeds

- Personal: `http://localhost:9999/feed?username=<user>`
- Aggregated: `http://localhost:9999/feed`
- Single note: `http://localhost:9999/feed/<uuid>`

## Web UI

Browse posts through a terminal-themed web interface:

- **Homepage:** `http://localhost:9999/` - View all posts from all users
- **User profile:** `http://localhost:9999/users/<username>` - View posts by a specific user
- **Single post:** `http://localhost:9999/posts/<uuid>` - View individual post

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
- Terminal with 24-bit color, 115×28 minimum
- OSC 8 support for clickable links (optional: Ghostty, iTerm2, Kitty)

## Tech Stack

- **SSH:** [wish](https://github.com/charmbracelet/wish)
- **TUI:** [bubbletea](https://github.com/charmbracelet/bubbletea), [lipgloss](https://github.com/charmbracelet/lipgloss)
- **Web:** [gin](https://github.com/gin-gonic/gin)
- **Database:** SQLite with WAL mode
- **Federation:** Custom ActivityPub implementation with HTTP signatures

## License

MIT - See LICENSE file

## Contributing

Contributions welcome! Open an issue or pull request on [GitHub](https://github.com/deemkeen/stegodon).
