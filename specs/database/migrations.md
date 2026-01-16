# Migrations

This document specifies the database migration system for schema evolution and data backfills.

---

## Overview

Stegodon uses code-based migrations that run on startup:
- **Idempotent**: Safe to run multiple times
- **Non-destructive**: Adds columns/tables without removing data
- **Backwards compatible**: Old data continues to work
- **Data backfills**: Populates denormalized fields

---

## Migration Execution

### Entry Point

```go
func (db *DB) RunActivityPubMigrations() error {
    log.Println("Running ActivityPub migrations...")
    return db.RunMigrations()
}
```

Called during app initialization when ActivityPub is enabled.

### Transaction Wrapper

All migrations run in a single transaction:

```go
func (db *DB) RunMigrations() error {
    return db.wrapTransaction(func(tx *sql.Tx) error {
        // Create tables
        // Create indices
        // Extend existing tables
        // Run data backfills
        return nil
    })
}
```

---

## Table Creation

### Create If Not Exists Pattern

```go
func (db *DB) createTableIfNotExists(tx *sql.Tx, createSQL string, tableName string) error {
    _, err := tx.Exec(createSQL)
    if err != nil {
        log.Printf("Error creating table %s: %v", tableName, err)
        return err
    }
    log.Printf("Table %s created or already exists", tableName)
    return nil
}
```

### Tables Created

| Table | Purpose |
|-------|---------|
| `follows` | Follow relationships |
| `remote_accounts` | Cached remote profiles |
| `activities` | Received ActivityPub activities |
| `likes` | Like/favorite records |
| `boosts` | Boost/reblog records |
| `delivery_queue` | Background delivery queue |
| `hashtags` | Unique hashtag registry |
| `note_hashtags` | Note-hashtag junction |
| `note_mentions` | Note-mention junction |
| `relays` | Relay subscriptions |
| `notifications` | User notifications |
| `info_boxes` | Web UI information boxes |

---

## Index Creation

Indices are created with `CREATE INDEX IF NOT EXISTS`:

```go
if _, err := tx.Exec(sqlCreateFollowsIndices); err != nil {
    log.Printf("Warning: Failed to create follows indices: %v", err)
}
```

**Warning only**: Index creation failures don't abort migration.

### Key Indices

| Table | Index | Purpose |
|-------|-------|---------|
| `activities` | `idx_activities_created_at` | Timeline ordering |
| `activities` | `idx_activities_object_uri` | Deduplication |
| `activities` | `idx_activities_from_relay` | Relay content filtering |
| `notes` | `idx_notes_in_reply_to_uri` | Thread queries |
| `follows` | `idx_follows_account_id` | Follower lookups |
| `delivery_queue` | `idx_delivery_queue_next_retry` | Queue processing |

---

## Column Extensions

### Extend Existing Tables

Adds new columns to existing tables without data loss:

```go
func (db *DB) extendExistingTables(tx *sql.Tx) {
    // Try to add columns (ignore errors if they exist)
    tx.Exec("ALTER TABLE accounts ADD COLUMN display_name TEXT")
    tx.Exec("ALTER TABLE accounts ADD COLUMN summary TEXT")
    tx.Exec("ALTER TABLE accounts ADD COLUMN avatar_url TEXT")
    tx.Exec("ALTER TABLE accounts ADD COLUMN is_admin INTEGER DEFAULT 0")
    tx.Exec("ALTER TABLE accounts ADD COLUMN muted INTEGER DEFAULT 0")
    // ...
}
```

**Error suppression**: `ALTER TABLE ADD COLUMN` fails silently if column exists.

### Extended Columns

| Table | Column | Default | Purpose |
|-------|--------|---------|---------|
| `accounts` | `display_name` | NULL | User display name |
| `accounts` | `summary` | NULL | User bio |
| `accounts` | `avatar_url` | NULL | Profile image |
| `accounts` | `is_admin` | 0 | Admin flag |
| `accounts` | `muted` | 0 | Muted by admin |
| `notes` | `visibility` | 'public' | Post visibility |
| `notes` | `in_reply_to_uri` | NULL | Reply parent |
| `notes` | `edited_at` | NULL | Edit timestamp |
| `notes` | `reply_count` | 0 | Denormalized count |
| `notes` | `like_count` | 0 | Denormalized count |
| `notes` | `boost_count` | 0 | Denormalized count |
| `activities` | `reply_count` | 0 | Denormalized count |
| `activities` | `from_relay` | 0 | Relay content flag |
| `follows` | `is_local` | 0 | Local follow flag |
| `likes` | `object_uri` | NULL | Remote post URI |
| `relays` | `follow_uri` | NULL | For Undo Follow |
| `relays` | `paused` | 0 | Pause flag |

---

## Data Backfills

### Backfill Activity Object URIs

Extracts `object_uri` from `raw_json` for existing activities:

```go
func (db *DB) backfillActivityObjectURIs(tx *sql.Tx) error {
    rows, err := tx.Query(`SELECT id, raw_json FROM activities
                           WHERE object_uri IS NULL OR object_uri = ''`)

    for rows.Next() {
        // Parse raw_json
        // Extract object.id
        // Update object_uri column
    }
}
```

**Purpose**: Enables efficient deduplication queries on `object_uri`.

### Backfill Reply Counts

Calculates denormalized `reply_count` for notes and activities:

```go
func (db *DB) backfillReplyCounts(tx *sql.Tx) error {
    // Skip if already done
    var hasData int
    tx.QueryRow(`SELECT COUNT(*) FROM notes WHERE reply_count > 0`).Scan(&hasData)
    if hasData > 0 {
        return nil
    }

    // Reset all counts
    tx.Exec(`UPDATE notes SET reply_count = 0`)
    tx.Exec(`UPDATE activities SET reply_count = 0`)

    // Count replies for each parent
    // Uses recursive counting for nested threads
}
```

**Features**:
- Skip check prevents redundant work
- Recursive counting for thread depth
- Handles both notes and activities

---

## Data Fixes

### Fix Orphaned Update Activities

Converts `Update` activities without corresponding `Create` to `Create`:

```go
func (db *DB) fixOrphanedUpdateActivities(tx *sql.Tx) error {
    rows, err := tx.Query(`
        SELECT u.id, u.activity_uri, u.actor_uri, u.object_uri, u.raw_json
        FROM activities u
        WHERE u.activity_type = 'Update'
        AND NOT EXISTS (
            SELECT 1 FROM activities c
            WHERE c.object_uri = u.object_uri
            AND c.activity_type = 'Create'
        )
        ...
    `)

    for rows.Next() {
        // Convert Update to Create
        tx.Exec(`UPDATE activities SET activity_type = 'Create' WHERE id = ?`, id)
    }
}
```

**Use case**: User followed after original post, only received Update.

### Seed Default Info Boxes

Creates default info boxes on first run:

```go
func (db *DB) seedDefaultInfoBoxes(tx *sql.Tx) error {
    // Check if any info boxes already exist
    var count int
    err := tx.QueryRow(`SELECT COUNT(*) FROM info_boxes`).Scan(&count)
    if count > 0 {
        return nil  // Skip if already seeded
    }

    // Seed default boxes
    defaultBoxes := []struct {
        title    string
        content  string
        orderNum int
    }{
        {title: "ssh-first fediverse blog", content: "...", orderNum: 1},
        {title: "features", content: "...", orderNum: 2},
        {title: "<svg>...</svg>github", content: "...", orderNum: 3},
    }

    for _, box := range defaultBoxes {
        tx.Exec(`INSERT INTO info_boxes (id, title, content, order_num, ...) VALUES (...)`)
    }
}
```

**Default boxes:**

| Order | Title | Content |
|-------|-------|---------|
| 1 | "ssh-first fediverse blog" | SSH connection instructions with `{{SSH_PORT}}` placeholder |
| 2 | "features" | Feature list (posts, follows, federation, RSS) |
| 3 | "(GitHub SVG icon) github" | Link to GitHub repository |

---

### Add Username Unique Constraint

Handles duplicate usernames before adding UNIQUE constraint:

```go
func (db *DB) addUsernameUniqueConstraint(tx *sql.Tx) error {
    // Find duplicates
    rows, err := tx.Query(`
        SELECT username, COUNT(*) as count
        FROM accounts
        GROUP BY LOWER(username)
        HAVING count > 1
    `)

    // Rename duplicates (keep oldest)
    for i := 1; i < len(accounts); i++ {
        newUsername := accounts[i].username + "_" + fmt.Sprintf("%d", i+1)
        tx.Exec(`UPDATE accounts SET username = ? WHERE id = ?`, newUsername, id)
    }

    // Add case-insensitive unique index
    tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_username
             ON accounts(username COLLATE NOCASE)`)
}
```

**Features**:
- Case-insensitive duplicate detection
- Keeps oldest account, renames newer ones
- Adds COLLATE NOCASE for future uniqueness

---

## Performance Index Migration

Additional performance indices added post-migration:

```go
func (db *DB) MigratePerformanceIndexes() error {
    db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notes_in_reply_to_uri
                ON notes(in_reply_to_uri)`)
    db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_object_uri
                ON activities(object_uri)`)
    db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_from_relay
                ON activities(from_relay)`)
}
```

Can be called separately from main migration for existing databases.

---

## Migration Order

1. **Create tables** - All `CREATE TABLE IF NOT EXISTS`
2. **Create indices** - All `CREATE INDEX IF NOT EXISTS`
3. **Extend tables** - `ALTER TABLE ADD COLUMN`
4. **Backfill data** - Object URIs, reply counts
5. **Fix data** - Orphaned updates, duplicate usernames
6. **Seed defaults** - Default info boxes

---

## Idempotency Guarantees

| Operation | Idempotent | Method |
|-----------|------------|--------|
| Create table | Yes | `IF NOT EXISTS` |
| Create index | Yes | `IF NOT EXISTS` |
| Add column | Yes | Error suppression |
| Backfill | Yes | Skip check |
| Data fix | Yes | Only processes unfixed |
| Seed defaults | Yes | Count check (skip if > 0) |

---

## Logging

All migrations log progress:

```
Running ActivityPub migrations...
Table follows created or already exists
Table remote_accounts created or already exists
Warning: Failed to create follows indices: ...
Extended existing tables with new columns
Backfilled object_uri for 15 activities
Renamed duplicate username 'admin' (id: xxx) to 'admin_2'
Added UNIQUE constraint to accounts.username column
Completed backfilling reply counts
Converted 3 orphaned Update activities to Create
Info boxes already exist, skipping seed
Seeded 3 default info boxes
```

---

## Source Files

- `db/migrations.go` - All migration logic
- `db/db.go` - RunActivityPubMigrations() entry point
