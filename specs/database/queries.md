# Query Patterns

This document specifies the common query patterns and denormalized counter strategies used in stegodon.

---

## Overview

Stegodon uses several query patterns:
- **Denormalized counters**: Pre-computed counts for performance
- **Nullable column handling**: SQL NULL to Go pointer conversion
- **Timestamp parsing**: SQLite timestamp format handling
- **Optimized joins**: INNER JOIN for related data
- **Index-aware queries**: Leverage created indices

---

## Denormalized Counters

### Pattern

Instead of `COUNT(*)` subqueries, engagement metrics are stored directly:

```sql
-- In notes and activities tables
reply_count INTEGER DEFAULT 0,
like_count INTEGER DEFAULT 0,
boost_count INTEGER DEFAULT 0
```

### Benefits

| Approach | Cost |
|----------|------|
| `COUNT(*)` subquery | O(n) per read |
| Denormalized counter | O(1) per read |

### Increment on Create

```go
func (db *DB) insertNoteWithReply(tx *sql.Tx, userId uuid.UUID, message string, inReplyToURI string) (uuid.UUID, error) {
    // Insert note
    _, err := tx.Exec(`INSERT INTO notes(...) VALUES (...)`, ...)

    // Increment parent's reply count
    if inReplyToURI != "" {
        db.incrementReplyCount(tx, inReplyToURI)
    }

    return noteId, nil
}
```

### Decrement on Delete

```go
func (db *DB) deleteNote(tx *sql.Tx, noteId uuid.UUID) error {
    // Get parent URI before delete
    var inReplyToURI sql.NullString
    tx.QueryRow(`SELECT in_reply_to_uri FROM notes WHERE id = ?`, noteId).Scan(&inReplyToURI)

    // Delete note
    tx.Exec(sqlDeleteNote, noteId)

    // Decrement parent's count
    if inReplyToURI.Valid && inReplyToURI.String != "" {
        db.decrementReplyCount(tx, inReplyToURI.String)
    }
}
```

### Increment Implementation

```go
func (db *DB) incrementReplyCount(tx *sql.Tx, objectURI string) {
    // Try notes table
    result, _ := tx.Exec(`UPDATE notes SET reply_count = reply_count + 1
                          WHERE object_uri = ?`, objectURI)
    if rows, _ := result.RowsAffected(); rows > 0 {
        return
    }

    // Try activities table
    tx.Exec(`UPDATE activities SET reply_count = reply_count + 1
             WHERE object_uri = ?`, objectURI)
}
```

---

## Nullable Column Handling

### sql.NullString Pattern

```go
var displayName, summary, avatarURL sql.NullString
var isAdmin, muted sql.NullInt64

err := row.Scan(
    &tempAcc.Id,
    &tempAcc.Username,
    // ... other fields
    &displayName,
    &summary,
    &avatarURL,
    &isAdmin,
    &muted,
)

// Convert to domain model
tempAcc.DisplayName = displayName.String
tempAcc.Summary = summary.String
tempAcc.AvatarURL = avatarURL.String
tempAcc.IsAdmin = isAdmin.Int64 == 1
tempAcc.Muted = muted.Int64 == 1
```

### sql.NullTime for Timestamps

```go
var editedAtStr sql.NullString
rows.Scan(..., &editedAtStr, ...)

if editedAtStr.Valid {
    if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
        note.EditedAt = &parsedTime  // Go pointer
    }
}
```

---

## Timestamp Parsing

### Problem

SQLite returns timestamps in various formats:
- `2024-01-15 10:30:00`
- `2024-01-15T10:30:00Z`

### Solution

```go
func parseTimestamp(timestampStr string) (time.Time, error) {
    if timestampStr == "" {
        return time.Time{}, fmt.Errorf("empty timestamp")
    }

    // Remove Z suffix and convert T to space for ISO 8601 format
    if strings.HasSuffix(timestampStr, "Z") {
        timestampStr = strings.TrimSuffix(timestampStr, "Z")
        timestampStr = strings.Replace(timestampStr, "T", " ", 1)
    }

    return time.ParseInLocation("2006-01-02 15:04:05", timestampStr, time.Local)
}
```

### Usage

```go
var createdAtStr string
row.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, ...)

note.CreatedAt, err = parseTimestamp(createdAtStr)
```

---

## Common Query Patterns

### Read by Primary Key

```go
const sqlSelectUserById = `SELECT id, username, publickey, created_at, first_time_login,
    web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted
    FROM accounts WHERE id = ?`

func (db *DB) ReadAccById(id uuid.UUID) (error, *domain.Account) {
    row := db.db.QueryRow(sqlSelectUserById, id)
    // Scan all columns...
    if err == sql.ErrNoRows {
        return err, nil
    }
    return err, &tempAcc
}
```

### Read with Join

```go
const sqlSelectNotesByUserId = `SELECT notes.id, accounts.username, notes.message,
    notes.created_at, notes.edited_at, notes.in_reply_to_uri, notes.like_count, notes.boost_count
    FROM notes
    INNER JOIN accounts ON accounts.id = notes.user_id
    WHERE notes.user_id = ?
    ORDER BY notes.created_at DESC`
```

### Read Multiple Rows

```go
func (db *DB) ReadNotesByUserId(userId uuid.UUID) (error, *[]domain.Note) {
    rows, err := db.db.Query(sqlSelectNotesByUserId, userId)
    if err != nil {
        return err, nil
    }
    defer rows.Close()

    var notes []domain.Note
    for rows.Next() {
        var note domain.Note
        if err := rows.Scan(...); err != nil {
            return err, &notes
        }
        notes = append(notes, note)
    }

    if err = rows.Err(); err != nil {
        return err, &notes
    }
    return nil, &notes
}
```

---

## Timeline Queries

### Local Timeline (All Notes)

```go
const sqlSelectAllNotes = `SELECT notes.id, accounts.username, notes.message,
    notes.created_at, notes.edited_at, notes.in_reply_to_uri,
    COALESCE(notes.like_count, 0), COALESCE(notes.boost_count, 0)
    FROM notes
    INNER JOIN accounts ON accounts.id = notes.user_id
    ORDER BY notes.created_at DESC`
```

### Following Timeline

```go
const sqlSelectLocalTimelineNotesByFollows = `SELECT notes.id, accounts.username,
    notes.message, notes.created_at, notes.edited_at
    FROM notes
    INNER JOIN accounts ON accounts.id = notes.user_id
    WHERE (notes.in_reply_to_uri IS NULL OR notes.in_reply_to_uri = '')
    AND (notes.user_id = ? OR notes.user_id IN (
        SELECT target_account_id FROM follows
        WHERE account_id = ? AND accepted = 1 AND is_local = 1
    ))
    ORDER BY notes.created_at DESC LIMIT ?`
```

**Features**:
- Excludes replies (top-level posts only)
- Includes user's own posts
- Includes posts from accepted local follows
- Limited result set

---

## Statistics Queries

### User Counts

```go
const sqlCountAccounts = `SELECT COUNT(*) FROM accounts`
const sqlCountLocalPosts = `SELECT COUNT(*) FROM notes`
```

### Active Users

```go
const sqlCountActiveUsersMonth = `SELECT COUNT(DISTINCT user_id) FROM notes
    WHERE created_at >= datetime('now', '-30 days')`

const sqlCountActiveUsersHalfYear = `SELECT COUNT(DISTINCT user_id) FROM notes
    WHERE created_at >= datetime('now', '-180 days')`
```

---

## Outbox Query

For ActivityPub outbox collection:

```go
const sqlSelectPublicNotesByUsername = `SELECT notes.id, notes.user_id, notes.message,
    notes.created_at, notes.edited_at, notes.visibility, notes.object_uri
    FROM notes
    INNER JOIN accounts ON accounts.id = notes.user_id
    WHERE accounts.username = ? AND notes.visibility = 'public'
    ORDER BY notes.created_at DESC
    LIMIT ? OFFSET ?`
```

**Features**:
- Filters to public visibility only
- Supports pagination with LIMIT/OFFSET
- Ordered by creation time (newest first)

---

## Insert Patterns

### Insert with Generated UUID

```go
func (db *DB) insertNoteWithReply(tx *sql.Tx, userId uuid.UUID, message string,
                                   inReplyToURI string) (uuid.UUID, error) {
    noteId := uuid.New()
    _, err := tx.Exec(`INSERT INTO notes(id, user_id, message, created_at, in_reply_to_uri)
                       VALUES (?, ?, ?, ?, ?)`,
        noteId, userId, message, time.Now().Format("2006-01-02 15:04:05"), inReplyToURI)
    return noteId, err
}
```

### Insert First User as Admin

```go
func (db *DB) insertUser(tx *sql.Tx, username string, publicKey string,
                         webKeyPair *util.RsaKeyPair) error {
    // Check if first user
    var count int
    tx.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)

    isAdmin := 0
    if count == 0 {
        isAdmin = 1
        log.Println("Creating first user as admin:", username)
    }

    _, err = tx.Exec(sqlInsertUser, uuid.New(), username,
                     util.PkToHash(publicKey), webKeyPair.Public,
                     webKeyPair.Private, time.Now())

    // Update is_admin
    tx.Exec("UPDATE accounts SET is_admin = ? WHERE username = ?", isAdmin, username)
    return err
}
```

---

## Update Patterns

### Update with Timestamp

```go
const sqlUpdateNote = `UPDATE notes SET message = ?, edited_at = ? WHERE id = ?`

func (db *DB) updateNote(tx *sql.Tx, noteId uuid.UUID, message string) error {
    _, err := tx.Exec(sqlUpdateNote, message,
                      time.Now().Format("2006-01-02 15:04:05"), noteId)
    return err
}
```

### Conditional Update

```go
func (db *DB) UpdateLoginById(username string, displayName string,
                              summary string, id uuid.UUID) error {
    // Check uniqueness first
    err, existingAcc := db.ReadAccByUsername(username)
    if err == nil && existingAcc != nil && existingAcc.Id != id {
        return fmt.Errorf("username '%s' is already taken", username)
    }

    return db.wrapTransaction(func(tx *sql.Tx) error {
        return db.updateLoginUserById(tx, username, displayName, summary, id)
    })
}
```

---

## Delete Patterns

### Delete with Cascade Effect

```go
func (db *DB) deleteNote(tx *sql.Tx, noteId uuid.UUID) error {
    // Get parent before delete (for count decrement)
    var inReplyToURI sql.NullString
    tx.QueryRow(`SELECT in_reply_to_uri FROM notes WHERE id = ?`, noteId).Scan(&inReplyToURI)

    // Delete
    _, err = tx.Exec(sqlDeleteNote, noteId)

    // Decrement parent count
    if inReplyToURI.Valid && inReplyToURI.String != "" {
        db.decrementReplyCount(tx, inReplyToURI.String)
    }
    return nil
}
```

---

## COALESCE for Null Safety

```go
const sqlSelectNoteById = `SELECT notes.id, accounts.username, notes.message,
    notes.created_at, notes.edited_at,
    COALESCE(notes.like_count, 0),
    COALESCE(notes.boost_count, 0)
    FROM notes
    INNER JOIN accounts ON accounts.id = notes.user_id
    WHERE notes.id = ?`
```

`COALESCE(column, 0)` returns 0 if column is NULL, avoiding null handling in Go.

---

## Error Handling

### No Rows Found

```go
err := row.Scan(...)
if err == sql.ErrNoRows {
    return err, nil  // Return nil object
}
```

### Return Style

The codebase uses `(error, *Type)` return pattern:

```go
func (db *DB) ReadAccById(id uuid.UUID) (error, *domain.Account) {
    // ...
    return nil, &tempAcc  // Success
    return err, nil       // Error or not found
}
```

---

## Info Box Queries

### SQL Definitions

```go
const (
    sqlSelectAllInfoBoxes     = `SELECT id, title, content, order_num, enabled, created_at, updated_at
                                 FROM info_boxes ORDER BY order_num ASC`
    sqlSelectEnabledInfoBoxes = `SELECT id, title, content, order_num, enabled, created_at, updated_at
                                 FROM info_boxes WHERE enabled = 1 ORDER BY order_num ASC`
    sqlSelectInfoBoxById      = `SELECT id, title, content, order_num, enabled, created_at, updated_at
                                 FROM info_boxes WHERE id = ?`
    sqlInsertInfoBox          = `INSERT INTO info_boxes(id, title, content, order_num, enabled, created_at, updated_at)
                                 VALUES (?, ?, ?, ?, ?, ?, ?)`
    sqlUpdateInfoBox          = `UPDATE info_boxes SET title = ?, content = ?, order_num = ?, enabled = ?, updated_at = ?
                                 WHERE id = ?`
    sqlDeleteInfoBox          = `DELETE FROM info_boxes WHERE id = ?`
    sqlToggleInfoBoxEnabled   = `UPDATE info_boxes SET enabled = NOT enabled, updated_at = ? WHERE id = ?`
)
```

### Read Operations

```go
// ReadAllInfoBoxes returns all info boxes ordered by order_num
func (db *DB) ReadAllInfoBoxes() (error, *[]domain.InfoBox)

// ReadEnabledInfoBoxes returns only enabled info boxes (for web display)
func (db *DB) ReadEnabledInfoBoxes() (error, *[]domain.InfoBox)

// ReadInfoBoxById returns a single info box
func (db *DB) ReadInfoBoxById(id uuid.UUID) (error, *domain.InfoBox)
```

### Write Operations

```go
// CreateInfoBox creates a new info box
func (db *DB) CreateInfoBox(box *domain.InfoBox) error

// UpdateInfoBox updates an existing info box
func (db *DB) UpdateInfoBox(box *domain.InfoBox) error

// DeleteInfoBox removes an info box
func (db *DB) DeleteInfoBox(id uuid.UUID) error

// ToggleInfoBoxEnabled flips the enabled status
func (db *DB) ToggleInfoBoxEnabled(id uuid.UUID) error
```

### Boolean Handling

SQLite stores booleans as integers:

```go
// Reading
var enabled int
rows.Scan(&idStr, &box.Title, &box.Content, &box.OrderNum, &enabled, ...)
box.Enabled = enabled == 1

// Writing
enabledInt := 0
if box.Enabled {
    enabledInt = 1
}
tx.Exec(sqlInsertInfoBox, ..., enabledInt, ...)
```

---

## Source Files

- `db/db.go` - All query implementations
- `db/migrations.go` - Backfill queries
- `domain/*.go` - Entity definitions scanned into
