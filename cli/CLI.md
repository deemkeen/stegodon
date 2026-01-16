# CLI Mode

Stegodon supports non-interactive CLI commands via SSH, enabling scripting and automation.

## Usage

```bash
ssh -p <port> <server> <command> [options]
```

## Commands

| Command | Description |
|---------|-------------|
| `post <message>` | Create a new note |
| `post -` | Read message from stdin |
| `timeline` | Show recent home timeline |
| `timeline -n <N>` | Limit to N posts |
| `notifications` | Show unread notifications |
| `help` | Show help message |

## Global Flags

| Flag | Description |
|------|-------------|
| `--json`, `-j` | Output in JSON format |

## Examples

```bash
# Post a message
ssh -p 23232 localhost post "Hello world"

# Post with JSON response
ssh -p 23232 localhost post "Hello" -j

# Post from stdin (piping)
echo "Multi-line content" | ssh -p 23232 localhost post -

# View timeline
ssh -p 23232 localhost timeline

# View last 5 posts as JSON
ssh -p 23232 localhost timeline -n 5 -j

# View notifications as JSON
ssh -p 23232 localhost notifications -j
```

## JSON Output

All commands support `--json` / `-j` for machine-readable output.

**Post response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Hello world",
  "created_at": "2026-01-15T10:30:00Z"
}
```

**Timeline response:**
```json
{
  "posts": [
    {
      "id": "...",
      "author": "alice",
      "domain": "",
      "message": "Hello from Alice",
      "created_at": "2026-01-15T10:30:00Z",
      "reply_count": 0,
      "like_count": 2,
      "boost_count": 0
    }
  ],
  "count": 1
}
```

**Notifications response:**
```json
{
  "notifications": [
    {
      "id": "...",
      "type": "follow",
      "actor": "@alice@mastodon.social",
      "created_at": "2026-01-15T10:00:00Z"
    }
  ],
  "unread_count": 1
}
```

**Error response:**
```json
{
  "error": "message too long",
  "details": "165 chars, max 150"
}
```

## Scripting Examples

```bash
# Post and capture ID
NOTE_ID=$(ssh -p 23232 localhost post "Automated post" -j | jq -r '.id')

# Check for new notifications
UNREAD=$(ssh -p 23232 localhost notifications -j | jq '.unread_count')
[ "$UNREAD" -gt 0 ] && echo "You have $UNREAD unread notifications"

# Export timeline to file
ssh -p 23232 localhost timeline -n 100 -j > timeline.json
```
