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
| `post - [--reply-to <id>]` | Read message from stdin |
| `post <message> --reply-to <id>` | Reply to a post |
| `timeline` | Show recent home timeline |
| `timeline -n <N>` | Limit to N posts |
| `like <id>` | Like/unlike a post (toggle) |
| `boost <id>` | Boost/unboost a post (toggle) |
| `follow <user>` | Follow/unfollow a user (toggle) |
| `notifications` | Show unread notifications |
| `clear-notifications` | Clear all notifications |
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

# Reply to a post
ssh -p 23232 localhost post "Nice post!" --reply-to 550e8400-e29b-41d4-a716-446655440000

# Reply from stdin
echo "My reply" | ssh -p 23232 localhost post - --reply-to 550e8400-e29b-41d4-a716-446655440000

# View timeline
ssh -p 23232 localhost timeline

# View last 5 posts as JSON
ssh -p 23232 localhost timeline -n 5 -j

# Like a post (get ID from timeline output)
ssh -p 23232 localhost like 550e8400-e29b-41d4-a716-446655440000

# Unlike (running again toggles)
ssh -p 23232 localhost like 550e8400-e29b-41d4-a716-446655440000

# Boost a post
ssh -p 23232 localhost boost 550e8400-e29b-41d4-a716-446655440000 -j

# Follow a local user
ssh -p 23232 localhost follow @alice

# Follow a remote user
ssh -p 23232 localhost follow user@mastodon.social

# Follow via profile URL
ssh -p 23232 localhost follow https://mastodon.social/@user

# Unfollow (running again toggles)
ssh -p 23232 localhost follow @alice

# View notifications as JSON
ssh -p 23232 localhost notifications -j

# Clear all notifications
ssh -p 23232 localhost clear-notifications
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

**Reply response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "My reply",
  "created_at": "2026-01-15T10:30:00Z",
  "in_reply_to": "https://example.com/notes/123"
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
      "boost_count": 0,
      "object_uri": "https://example.com/users/alice/statuses/123",
      "is_local": true
    }
  ],
  "count": 1
}
```

**Like response:**
```json
{
  "post_id": "550e8400-e29b-41d4-a716-446655440000",
  "liked": true,
  "message": "Post liked"
}
```

**Boost response:**
```json
{
  "post_id": "550e8400-e29b-41d4-a716-446655440000",
  "boosted": true,
  "message": "Post boosted"
}
```

**Follow response (local user):**
```json
{
  "username": "alice",
  "following": true,
  "message": "Now following @alice"
}
```

**Follow response (remote user):**
```json
{
  "username": "user",
  "domain": "mastodon.social",
  "following": false,
  "pending": true,
  "message": "Sent follow request to @user@mastodon.social"
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

**Clear notifications response:**
```json
{
  "status": "ok",
  "cleared": true
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

# Reply to a post and capture ID
REPLY_ID=$(ssh -p 23232 localhost post "My reply" --reply-to "$NOTE_ID" -j | jq -r '.id')

# Check for new notifications
UNREAD=$(ssh -p 23232 localhost notifications -j | jq '.unread_count')
[ "$UNREAD" -gt 0 ] && echo "You have $UNREAD unread notifications"

# Export timeline to file
ssh -p 23232 localhost timeline -n 100 -j > timeline.json

# Clear notifications after reading them
ssh -p 23232 localhost notifications -j > notifications.json && ssh -p 23232 localhost clear-notifications

# Like the first post in timeline
FIRST_POST=$(ssh -p 23232 localhost timeline -j | jq -r '.posts[0].id')
ssh -p 23232 localhost like "$FIRST_POST" -j

# Reply to the first post in timeline
FIRST_POST=$(ssh -p 23232 localhost timeline -j | jq -r '.posts[0].id')
ssh -p 23232 localhost post "Nice post!" --reply-to "$FIRST_POST" -j

# Boost all posts from a specific user
ssh -p 23232 localhost timeline -n 50 -j | jq -r '.posts[] | select(.author == "alice") | .id' | while read id; do
  ssh -p 23232 localhost boost "$id"
done

# Follow a remote user and check status
ssh -p 23232 localhost follow user@mastodon.social -j | jq '.pending'
```
