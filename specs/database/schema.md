# Database Schema

This document specifies the complete database schema for stegodon.

---

## Overview

Stegodon uses SQLite with WAL mode for persistent storage. The schema consists of:
- **Core tables**: `accounts`, `notes` - Essential user and content data
- **ActivityPub tables**: Federation-related data
- **Junction tables**: Many-to-many relationships
- **Queue tables**: Background processing
- **Configuration tables**: `info_boxes` - Customizable UI content

---

## Core Tables

### accounts

Stores local user accounts with SSH and ActivityPub credentials.

```sql
CREATE TABLE IF NOT EXISTS accounts (
    id uuid NOT NULL PRIMARY KEY,
    username varchar(100) UNIQUE NOT NULL,
    publickey varchar(1000) UNIQUE,
    created_at timestamp default current_timestamp,
    first_time_login int default 1,
    web_public_key text,
    web_private_key text,
    display_name TEXT,
    summary TEXT,
    avatar_url TEXT,
    is_admin INTEGER DEFAULT 0,
    muted INTEGER DEFAULT 0
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `username` | VARCHAR(100) | Unique username |
| `publickey` | VARCHAR(1000) | SHA256 hash of SSH public key |
| `created_at` | TIMESTAMP | Account creation time |
| `first_time_login` | INTEGER | 1 if user hasn't completed setup |
| `web_public_key` | TEXT | RSA public key (PEM) for ActivityPub |
| `web_private_key` | TEXT | RSA private key (PEM) for HTTP signatures |
| `display_name` | TEXT | User's display name |
| `summary` | TEXT | User bio/description |
| `avatar_url` | TEXT | Profile image URL |
| `is_admin` | INTEGER | 1 for admin users (first user auto-admin) |
| `muted` | INTEGER | 1 if user is muted by admin |

**Indexes:**
```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_username ON accounts(username COLLATE NOCASE);
```

---

### notes

Stores user-created posts with engagement counters.

```sql
CREATE TABLE IF NOT EXISTS notes (
    id uuid NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL,
    message varchar(1000),
    created_at timestamp default current_timestamp,
    edited_at TIMESTAMP,
    visibility TEXT DEFAULT 'public',
    in_reply_to_uri TEXT,
    object_uri TEXT,
    federated INTEGER DEFAULT 1,
    sensitive INTEGER DEFAULT 0,
    content_warning TEXT,
    reply_count INTEGER DEFAULT 0,
    like_count INTEGER DEFAULT 0,
    boost_count INTEGER DEFAULT 0
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | Foreign key to accounts |
| `message` | VARCHAR(1000) | Note content |
| `created_at` | TIMESTAMP | Creation time |
| `edited_at` | TIMESTAMP | Last edit time (NULL if never edited) |
| `visibility` | TEXT | `public`, `unlisted`, `followers`, `direct` |
| `in_reply_to_uri` | TEXT | Parent note/activity URI (for replies) |
| `object_uri` | TEXT | ActivityPub object URI |
| `federated` | INTEGER | 1 if sent to federation |
| `sensitive` | INTEGER | 1 if marked sensitive |
| `content_warning` | TEXT | CW text (if sensitive) |
| `reply_count` | INTEGER | Denormalized reply count |
| `like_count` | INTEGER | Denormalized like count |
| `boost_count` | INTEGER | Denormalized boost count |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes(user_id);
CREATE INDEX IF NOT EXISTS idx_notes_created_at ON notes(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notes_object_uri ON notes(object_uri);
CREATE INDEX IF NOT EXISTS idx_notes_in_reply_to_uri ON notes(in_reply_to_uri);
```

---

## ActivityPub Tables

### remote_accounts

Caches federated user profiles (24-hour TTL).

```sql
CREATE TABLE IF NOT EXISTS remote_accounts (
    id TEXT NOT NULL PRIMARY KEY,
    username TEXT NOT NULL,
    domain TEXT NOT NULL,
    actor_uri TEXT UNIQUE NOT NULL,
    display_name TEXT,
    summary TEXT,
    inbox_uri TEXT NOT NULL,
    outbox_uri TEXT,
    public_key_pem TEXT NOT NULL,
    avatar_url TEXT,
    last_fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(username, domain)
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `username` | TEXT | Remote username |
| `domain` | TEXT | Remote server domain |
| `actor_uri` | TEXT | Full ActivityPub actor URI |
| `display_name` | TEXT | Display name |
| `summary` | TEXT | Profile bio |
| `inbox_uri` | TEXT | Inbox endpoint for delivery |
| `outbox_uri` | TEXT | Outbox endpoint |
| `public_key_pem` | TEXT | RSA public key for signature verification |
| `avatar_url` | TEXT | Profile image URL |
| `last_fetched_at` | TIMESTAMP | Cache timestamp (refresh after 24h) |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_remote_accounts_actor_uri ON remote_accounts(actor_uri);
CREATE INDEX IF NOT EXISTS idx_remote_accounts_domain ON remote_accounts(domain);
```

---

### follows

Stores follow relationships (both local and remote).

```sql
CREATE TABLE IF NOT EXISTS follows (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    target_account_id TEXT NOT NULL,
    uri TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    accepted INTEGER DEFAULT 0,
    is_local INTEGER DEFAULT 0
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `account_id` | TEXT | Follower account ID |
| `target_account_id` | TEXT | Followed account ID |
| `uri` | TEXT | ActivityPub Follow activity URI |
| `created_at` | TIMESTAMP | Follow creation time |
| `accepted` | INTEGER | 1 if follow is accepted |
| `is_local` | INTEGER | 1 if following local user |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_follows_account_id ON follows(account_id);
CREATE INDEX IF NOT EXISTS idx_follows_target_account_id ON follows(target_account_id);
CREATE INDEX IF NOT EXISTS idx_follows_uri ON follows(uri);
```

---

### activities

Stores received ActivityPub activities for timeline and deduplication.

```sql
CREATE TABLE IF NOT EXISTS activities (
    id TEXT NOT NULL PRIMARY KEY,
    activity_uri TEXT UNIQUE NOT NULL,
    activity_type TEXT NOT NULL,
    actor_uri TEXT NOT NULL,
    object_uri TEXT,
    raw_json TEXT NOT NULL,
    processed INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    local INTEGER DEFAULT 0,
    reply_count INTEGER DEFAULT 0,
    like_count INTEGER DEFAULT 0,
    boost_count INTEGER DEFAULT 0,
    from_relay INTEGER DEFAULT 0
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `activity_uri` | TEXT | Unique ActivityPub activity URI |
| `activity_type` | TEXT | `Create`, `Update`, `Delete`, `Like`, etc. |
| `actor_uri` | TEXT | Actor who performed the activity |
| `object_uri` | TEXT | Object URI (extracted from raw_json) |
| `raw_json` | TEXT | Full activity JSON |
| `processed` | INTEGER | 1 if fully processed |
| `created_at` | TIMESTAMP | When received |
| `local` | INTEGER | 1 if local activity |
| `reply_count` | INTEGER | Denormalized reply count |
| `like_count` | INTEGER | Denormalized like count |
| `boost_count` | INTEGER | Denormalized boost count |
| `from_relay` | INTEGER | 1 if received via relay |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_activities_uri ON activities(activity_uri);
CREATE INDEX IF NOT EXISTS idx_activities_processed ON activities(processed);
CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(activity_type);
CREATE INDEX IF NOT EXISTS idx_activities_created_at ON activities(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activities_object_uri ON activities(object_uri);
CREATE INDEX IF NOT EXISTS idx_activities_from_relay ON activities(from_relay);
```

---

### likes

Stores like/favorite relationships.

```sql
CREATE TABLE IF NOT EXISTS likes (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    note_id TEXT NOT NULL,
    uri TEXT NOT NULL,
    object_uri TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, note_id)
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `account_id` | TEXT | Account that liked |
| `note_id` | TEXT | Local note ID (or empty for remote) |
| `uri` | TEXT | Like activity URI |
| `object_uri` | TEXT | Remote note URI (for remote likes) |
| `created_at` | TIMESTAMP | When liked |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_likes_note_id ON likes(note_id);
CREATE INDEX IF NOT EXISTS idx_likes_account_id ON likes(account_id);
CREATE INDEX IF NOT EXISTS idx_likes_object_uri ON likes(object_uri);
CREATE UNIQUE INDEX IF NOT EXISTS idx_likes_account_object_uri ON likes(account_id, object_uri)
    WHERE object_uri IS NOT NULL AND object_uri != '';
```

---

### boosts

Stores boost/reblog relationships.

```sql
CREATE TABLE IF NOT EXISTS boosts (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    note_id TEXT NOT NULL,
    uri TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, note_id)
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `account_id` | TEXT | Account that boosted |
| `note_id` | TEXT | Boosted note ID |
| `uri` | TEXT | Boost activity URI |
| `created_at` | TIMESTAMP | When boosted |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_boosts_note_id ON boosts(note_id);
CREATE INDEX IF NOT EXISTS idx_boosts_account_id ON boosts(account_id);
```

---

### relays

Stores ActivityPub relay subscriptions.

```sql
CREATE TABLE IF NOT EXISTS relays (
    id TEXT NOT NULL PRIMARY KEY,
    actor_uri TEXT UNIQUE NOT NULL,
    inbox_uri TEXT NOT NULL,
    follow_uri TEXT,
    name TEXT,
    status TEXT DEFAULT 'pending',
    paused INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    accepted_at TIMESTAMP
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `actor_uri` | TEXT | Relay actor URI |
| `inbox_uri` | TEXT | Relay inbox for delivery |
| `follow_uri` | TEXT | Our Follow activity URI (for Undo) |
| `name` | TEXT | Relay display name |
| `status` | TEXT | `pending`, `active`, `failed` |
| `paused` | INTEGER | 1 if user paused relay |
| `created_at` | TIMESTAMP | Subscription time |
| `accepted_at` | TIMESTAMP | When relay accepted |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_relays_status ON relays(status);
```

---

### notifications

Stores user notifications.

```sql
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    notification_type TEXT NOT NULL,
    actor_id TEXT,
    actor_username TEXT,
    actor_domain TEXT,
    note_id TEXT,
    note_uri TEXT,
    note_preview TEXT,
    read INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `account_id` | TEXT | Recipient account |
| `notification_type` | TEXT | `follow`, `like`, `reply`, `mention` |
| `actor_id` | TEXT | Actor who triggered notification |
| `actor_username` | TEXT | Actor's username |
| `actor_domain` | TEXT | Actor's domain |
| `note_id` | TEXT | Related note ID |
| `note_uri` | TEXT | Related note URI |
| `note_preview` | TEXT | Preview text |
| `read` | INTEGER | 1 if read |
| `created_at` | TIMESTAMP | When created |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_notifications_account_id ON notifications(account_id);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_account_read ON notifications(account_id, read);
```

---

### info_boxes

Stores customizable information boxes for the web UI sidebar.

```sql
CREATE TABLE IF NOT EXISTS info_boxes (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    order_num INTEGER DEFAULT 0,
    enabled INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `title` | TEXT | Box title (can include HTML/SVG) |
| `content` | TEXT | Markdown-formatted content |
| `order_num` | INTEGER | Display order (lower first) |
| `enabled` | INTEGER | 1 if visible on web UI |
| `created_at` | TIMESTAMP | Creation time |
| `updated_at` | TIMESTAMP | Last modification time |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_info_boxes_order ON info_boxes(order_num);
CREATE INDEX IF NOT EXISTS idx_info_boxes_enabled ON info_boxes(enabled);
```

---

### delivery_queue

Background queue for ActivityPub delivery.

```sql
CREATE TABLE IF NOT EXISTS delivery_queue (
    id TEXT NOT NULL PRIMARY KEY,
    inbox_uri TEXT NOT NULL,
    activity_json TEXT NOT NULL,
    attempts INTEGER DEFAULT 0,
    next_retry_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    account_id TEXT
)
```

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | UUID primary key |
| `inbox_uri` | TEXT | Target inbox URL |
| `activity_json` | TEXT | Serialized activity |
| `attempts` | INTEGER | Delivery attempt count |
| `next_retry_at` | TIMESTAMP | When to retry next |
| `created_at` | TIMESTAMP | When queued |
| `account_id` | TEXT | Source account (for cleanup) |

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_delivery_queue_next_retry ON delivery_queue(next_retry_at);
```

---

## Junction Tables

### hashtags

Stores unique hashtags with usage counts.

```sql
CREATE TABLE IF NOT EXISTS hashtags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    usage_count INTEGER DEFAULT 0,
    last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_hashtags_name ON hashtags(name);
CREATE INDEX IF NOT EXISTS idx_hashtags_usage ON hashtags(usage_count DESC);
```

---

### note_hashtags

Links notes to hashtags (many-to-many).

```sql
CREATE TABLE IF NOT EXISTS note_hashtags (
    note_id TEXT NOT NULL,
    hashtag_id INTEGER NOT NULL,
    PRIMARY KEY (note_id, hashtag_id),
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
    FOREIGN KEY (hashtag_id) REFERENCES hashtags(id) ON DELETE CASCADE
)
```

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_note_hashtags_note_id ON note_hashtags(note_id);
CREATE INDEX IF NOT EXISTS idx_note_hashtags_hashtag_id ON note_hashtags(hashtag_id);
```

---

### note_mentions

Stores @mentions in notes.

```sql
CREATE TABLE IF NOT EXISTS note_mentions (
    id TEXT PRIMARY KEY,
    note_id TEXT NOT NULL,
    mentioned_actor_uri TEXT NOT NULL,
    mentioned_username TEXT NOT NULL,
    mentioned_domain TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
)
```

**Indexes:**
```sql
CREATE INDEX IF NOT EXISTS idx_note_mentions_note_id ON note_mentions(note_id);
CREATE INDEX IF NOT EXISTS idx_note_mentions_actor_uri ON note_mentions(mentioned_actor_uri);
```

---

## Entity Relationships

```
accounts 1--* notes           (user creates notes)
accounts 1--* follows         (user follows others)
accounts 1--* likes           (user likes notes)
accounts 1--* boosts          (user boosts notes)
accounts 1--* notifications   (user receives notifications)

notes *--* hashtags           (via note_hashtags)
notes 1--* note_mentions      (note contains mentions)
notes 1--* likes              (note receives likes)
notes 1--* boosts             (note receives boosts)

remote_accounts 1--* follows  (remote users as targets)
activities 1--* likes         (activity receives likes)
activities 1--* boosts        (activity receives boosts)
```

---

## Source Files

- `db/db.go` - Core table definitions and CRUD operations
- `db/migrations.go` - ActivityPub table definitions and migrations
- `domain/*.go` - Entity struct definitions
