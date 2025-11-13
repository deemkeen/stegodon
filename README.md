![screenshot](./screenshot.png)

# stegodon

**stegodon** is a federated blog TUI application written in Golang, using the wonderful [Charm Tools](https://github.com/charmbracelet).
Users connect via SSH and can create notes in a clean terminal interface. Notes can be subscribed to via RSS and federate to the Fediverse via ActivityPub (Mastodon, Pleroma, etc.).

## Features

- **Note Management**
  - Create, edit, and delete notes with visual confirmation
  - Arrow key navigation with selection highlighting
  - Character limit (150 chars) with counter
  - Timestamps with "(edited)" indicators

- **RSS Feeds**
  - Per-user RSS feeds
  - Aggregated feed for all users
  - Individual note feeds by UUID

- **ActivityPub Federation**
  - Follow/unfollow remote users via WebFinger
  - Automatic follow request acceptance
  - Federate posts to all followers
  - Receive posts from followed accounts
  - Edit and delete federation with Update/Delete activities
  - HTTP signature authentication (RSA-SHA256)
  - Reliable delivery with exponential backoff retry
  - Actor caching with 24-hour TTL

- **Multi-User Support**
  - SSH public key authentication
  - Per-user accounts with unique usernames
  - RSA keypairs for ActivityPub signing
  - Federated timeline and followers list

## Installation

1. Clone the repository: `git clone https://github.com/deemkeen/stegodon`
2. Install dependencies: `go get -d`
3. Build the application: `go build`
4. Start the server: `./stegodon`

## Usage

### Basic Usage

Once the server is started, open an SSH session via `ssh 127.0.0.1 -p 23232` to access the application.
You will be authenticated with your SSH public key. On your first login, you'll be prompted to choose a username.

**Navigation:**
- `Tab`: Cycle through views (New Note → Notes List → Follow User → Followers → Federated Timeline)
- `↑/↓` or `j/k`: Navigate items in lists
- `u`: Edit selected note (in Notes List view)
- `d`: Delete selected note with confirmation (in Notes List view)
- `Ctrl+S`: Save/post note
- `Ctrl+C` or `q`: Quit

### RSS Feeds

- Personal feed: `http://127.0.0.1:9999/feed?username=<youruser>`
- Aggregated feed (all users): `http://127.0.0.1:9999/feed`
- Individual note: `http://127.0.0.1:9999/feed/<note-uuid>`

### ActivityPub Federation

**stegodon** can federate with Mastodon, Pleroma, and other ActivityPub-compatible servers.

**To enable ActivityPub:**
1. Set `STEGODON_WITH_AP=true`
2. Set `STEGODON_SSLDOMAIN` to your public domain (e.g., `yourdomain.com`)
3. Ensure your domain is publicly accessible with HTTPS
4. Proxy the HTTP port through a reverse proxy with TLS (e.g., nginx, caddy)

**Following users:**
1. Press `3` or Tab to "Follow User" view
2. Enter remote user in format: `username@domain.com` or `@username@domain.com`
3. Your follow request will be sent automatically

**Your ActivityPub profile:**
- Actor: `https://yourdomain.com/users/<username>`
- WebFinger: `https://yourdomain.com/.well-known/webfinger?resource=acct:<username>@yourdomain.com`

### Multi-User Setup

**stegodon** can be used as a multi-user system when exposed to the internet. Each user gets a dedicated account, accessible with their personal SSH key.

## Configuration

Configuration is managed via environment variables:

- **STEGODON_HOST** - Server IP (default: `127.0.0.1`)
- **STEGODON_SSHPORT** - SSH login port (default: `23232`)
- **STEGODON_HTTPPORT** - HTTP port (default: `9999`)
- **STEGODON_SSLDOMAIN** - **Required for ActivityPub** - Your public domain (default: `example.com`)
- **STEGODON_WITH_AP** - Enable ActivityPub functionality (default: `false`)

Default configuration is in `config.yaml`.

## Tech Stack

- **SSH Server**: [wish](https://github.com/charmbracelet/wish) - SSH server with middleware
- **TUI Framework**: [bubbletea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- **Styling**: [lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for terminal UIs
- **HTTP Router**: [gin](https://github.com/gin-gonic/gin) - Web framework
- **Database**: SQLite with WAL mode for concurrency
- **ActivityPub**: Custom implementation with HTTP signatures

### Requirements

For optimal results, use a terminal with:
- True Color (24-bit) support
- At least 115 columns × 28 rows

## ActivityPub Implementation

**Supported Activities:**
- Follow/Accept/Undo (full support)
- Create (send and receive)
- Update (send and receive)
- Delete (send and receive)
- Like (receive only, display pending)

**Endpoints:**
- `GET /users/:actor` - Actor profile (application/activity+json)
- `GET /.well-known/webfinger` - WebFinger discovery
- `GET /notes/:id` - Individual note objects
- `POST /inbox` - Shared inbox
- `POST /users/:actor/inbox` - Personal inbox
- `GET /users/:actor/outbox` - Outbox collection
- `GET /users/:actor/followers` - Followers collection
- `GET /users/:actor/following` - Following collection

**Features:**
- HTTP signature verification (RSA-SHA256)
- Delivery queue with retry logic
- Remote actor caching (24h TTL)
- WebFinger discovery

## Database

All data is persisted in a local SQLite database (`database.db`) with the following tables:

- `accounts` - User accounts with SSH key hashes and RSA keypairs
- `notes` - User notes with timestamps and edit history
- `remote_accounts` - Cached remote ActivityPub actors
- `follows` - Follow relationships (local and remote)
- `followers` - Follower relationships
- `activities` - Received ActivityPub activities
- `likes` - Like activities
- `delivery_queue` - Outgoing activity delivery queue

The database can be deleted to wipe all data and start fresh.

## Version

Current version: **1.0.0**

## LICENSE

MIT

## Contributing

Contributions are welcome! Please open a pull request or issue on the GitHub repository.
