# Follow Relationship

This document specifies the Follow entity, which represents follow relationships between users (both local and remote).

---

## Overview

A Follow represents a subscription relationship where one account follows another. Follows can be:
- **Local-to-Local**: Between two users on this server
- **Local-to-Remote**: Local user following a remote user
- **Remote-to-Local**: Remote user following a local user

Federated follows use ActivityPub's Follow/Accept protocol.

---

## Data Structure

```go
type Follow struct {
    Id              uuid.UUID
    AccountId       uuid.UUID // Follower (can be local or remote)
    TargetAccountId uuid.UUID // Followed (can be local or remote)
    URI             string    // ActivityPub Follow activity URI
    CreatedAt       time.Time
    Accepted        bool
    IsLocal         bool      // true if local-only follow
}
```

---

## Field Definitions

### Identity Fields

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Unique follow identifier |
| `AccountId` | `uuid.UUID` | Required | Who is following |
| `TargetAccountId` | `uuid.UUID` | Required | Who is being followed |

### Unique Constraint

Only one follow relationship per account pair:

```sql
UNIQUE(account_id, target_account_id)
```

### ActivityPub Fields

| Field | Type | Description |
|-------|------|-------------|
| `URI` | `string` | ActivityPub Follow activity URI |

For local follows, URI is empty string.

### State

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `CreatedAt` | `time.Time` | now | When follow was created |
| `Accepted` | `bool` | `false` | Whether follow is accepted |
| `IsLocal` | `bool` | `false` | Local-only (no federation) |

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS follows (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    target_account_id TEXT NOT NULL,
    uri TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    accepted INTEGER DEFAULT 0,
    is_local INTEGER DEFAULT 0,
    UNIQUE(account_id, target_account_id)
)
```

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_follows_account_id ON follows(account_id);
CREATE INDEX IF NOT EXISTS idx_follows_target_account_id ON follows(target_account_id);
CREATE INDEX IF NOT EXISTS idx_follows_uri ON follows(uri);
```

---

## Follow Types

### Local-to-Local

Both accounts exist in `accounts` table:

```
Local Account A → follows → Local Account B
```

- `IsLocal`: `true`
- `URI`: empty string
- `Accepted`: `true` (auto-accepted)

### Local-to-Remote

Follower in `accounts`, target in `remote_accounts`:

```
Local Account → follows → Remote Account
```

- `IsLocal`: `false`
- `URI`: Generated ActivityPub URI
- `Accepted`: `false` until Accept received

### Remote-to-Local

Follower in `remote_accounts`, target in `accounts`:

```
Remote Account → follows → Local Account
```

- `IsLocal`: `false`
- `URI`: Received in Follow activity
- `Accepted`: `true` (auto-accepted in stegodon)

---

## Follow Protocol

### Outgoing Follow (Local-to-Remote)

```
User enters: @user@remote.server
      │
      ▼
WebFinger Resolution
      │
      ├── Lookup remote account
      └── Get inbox URI
            │
            ▼
Create Follow Activity
      │
      ├── Generate Follow URI
      ├── Create Follow record (accepted=false)
      └── Queue delivery
            │
            ▼
Send to Inbox
      │
      ├── Sign request
      └── POST to inbox
            │
            ▼
Wait for Accept
      │
      └── On Accept: set accepted=true
```

### Follow Activity

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Follow",
    "id": "https://stegodon.example/follows/uuid",
    "actor": "https://stegodon.example/users/localuser",
    "object": "https://remote.server/users/remoteuser"
}
```

### Incoming Follow (Remote-to-Local)

```
Receive Follow Activity
      │
      ├── Verify HTTP signature
      ├── Parse Follow activity
      └── Lookup/create remote account
            │
            ▼
Create Follow Record
      │
      ├── Store with URI from activity
      └── Set accepted=true (auto-accept)
            │
            ▼
Send Accept Activity
      │
      ├── Create Accept wrapping Follow
      └── Queue delivery to follower's inbox
```

### Accept Activity

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Accept",
    "id": "https://stegodon.example/accepts/uuid",
    "actor": "https://stegodon.example/users/localuser",
    "object": {
        "type": "Follow",
        "id": "https://remote.server/follows/their-uuid",
        "actor": "https://remote.server/users/remoteuser",
        "object": "https://stegodon.example/users/localuser"
    }
}
```

---

## Unfollow Protocol

### Outgoing Unfollow

```
User clicks Unfollow
      │
      ▼
Create Undo Activity
      │
      ├── Reference original Follow URI
      └── Queue delivery
            │
            ▼
Delete Follow Record
```

### Undo Activity

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Undo",
    "id": "https://stegodon.example/undo/uuid",
    "actor": "https://stegodon.example/users/localuser",
    "object": {
        "type": "Follow",
        "id": "https://stegodon.example/follows/original-uuid",
        "actor": "https://stegodon.example/users/localuser",
        "object": "https://remote.server/users/remoteuser"
    }
}
```

### Incoming Unfollow

```
Receive Undo(Follow) Activity
      │
      ├── Verify signature
      ├── Find Follow by URI
      └── Delete Follow record
```

---

## Database Operations

### Read Operations

| Function | Description |
|----------|-------------|
| `GetFollowers(accountId)` | Get accounts following this user |
| `GetFollowing(accountId)` | Get accounts this user follows |
| `GetFollow(accountId, targetId)` | Check if follow exists |
| `GetFollowByURI(uri)` | Lookup by ActivityPub URI |
| `ReadFollowByAccountIds(accountId, targetAccountId)` | Check if follow exists between two accounts |
| `CountFollowers(accountId)` | Count followers |
| `CountFollowing(accountId)` | Count following |

### ReadFollowByAccountIds

Used by ProfileView to check follow status:

```go
// ReadFollowByAccountIds checks if a follow relationship exists
func (db *DB) ReadFollowByAccountIds(accountId, targetAccountId uuid.UUID) (error, *domain.Follow)
```

**Use cases:**
- Checking if viewing user follows the profile user
- Works with both local and remote account IDs
- Returns full Follow struct including `Accepted` flag
- Returns `nil` if no follow relationship exists

### Write Operations

| Function | Description |
|----------|-------------|
| `CreateFollow(follow)` | Create new follow |
| `AcceptFollow(uri)` | Mark follow as accepted |
| `DeleteFollow(id)` | Remove follow |
| `DeleteFollowByURI(uri)` | Remove by ActivityPub URI |

---

## Auto-Accept Behavior

Stegodon auto-accepts all incoming follow requests:

```go
// In inbox.go
func handleFollow(activity Activity) {
    // Create follow record
    follow := Follow{
        AccountId:       remoteAccountId,
        TargetAccountId: localAccountId,
        URI:             activity.Id,
        Accepted:        true,  // Auto-accept
    }

    // Send Accept back
    sendAcceptActivity(activity)
}
```

This is intentional for a public blog-style server.

---

## Duplicate Prevention

Follows have a unique constraint and migration to clean duplicates:

```go
func (d *Database) MigrateDuplicateFollows() error {
    // Remove duplicate (account_id, target_account_id) pairs
    // Keep the oldest follow
}
```

---

## UI Integration

### FollowUser View

- Enter `@user@domain` to follow
- WebFinger resolution
- Status feedback (already following, pending, success)

### Followers View

- Paginated list of followers
- Shows local and remote accounts
- Handle format: `@user` or `@user@domain`

### Following View

- Paginated list of accounts followed
- Shows follow status (pending/accepted)
- Unfollow action

### LocalUsers View

- Browse local users
- Follow/unfollow buttons
- Shows existing follow state

---

## Self-Follow Prevention

Users cannot follow themselves:

```go
if localAccount.Id == targetAccount.Id {
    return errors.New("cannot follow yourself")
}
```

---

## Pending Follows

When `Accepted` is `false`:

1. Follow was sent but not yet accepted
2. Remote server may have rejected it
3. Activity may still be in delivery queue

UI shows "Pending" status for unaccepted follows.

---

## Relationships

| Related Entity | Relationship | Description |
|----------------|--------------|-------------|
| Account | Many-to-One | Follower (local accounts) |
| Account | Many-to-One | Followed (local accounts) |
| RemoteAccount | Many-to-One | Follower (remote accounts) |
| RemoteAccount | Many-to-One | Followed (remote accounts) |
| Notification | One-to-One | Follow creates notification |

---

## Notification Integration

When a remote user follows a local user:

```go
notification := Notification{
    Type:        "follow",
    AccountId:   localAccountId,
    ActorId:     remoteAccountId,
    ActorHandle: "@user@domain",
}
```

---

## Delivery Behavior

When a local user creates a note, it's delivered to all followers:

```go
followers := db.GetFollowers(authorId)
for _, follower := range followers {
    if !follower.IsLocal {
        // Queue delivery to follower's inbox
        queueDelivery(follower.InboxURI, createActivity)
    }
}
```

---

## Source Files

- `domain/activitypub.go` - Follow struct
- `db/db.go` - Database operations
- `db/migrations.go` - Follow table creation
- `activitypub/inbox.go` - Incoming follow handling
- `activitypub/outbox.go` - Outgoing follow/unfollow
- `ui/followuser/` - Follow user view
- `ui/followers/` - Followers list view
- `ui/following/` - Following list view
