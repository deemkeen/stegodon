// engagement.go — Likes, boosts, reply counting, and engagement info queries.
// Principle: One File, One Mental Model — all social interaction database
// operations (likes, boosts, replies) live together as a cohesive unit.

package db

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// Like queries
const (
	sqlInsertLike              = `INSERT INTO likes(id, account_id, note_id, uri, created_at) VALUES (?, ?, ?, ?, ?)`
	sqlSelectLikeByURI         = `SELECT id, account_id, note_id, uri, created_at FROM likes WHERE uri = ?`
	sqlSelectLikeExists        = `SELECT COUNT(*) FROM likes WHERE account_id = ? AND note_id = ?`
	sqlSelectLikeByAccountNote = `SELECT id, account_id, note_id, uri, created_at FROM likes WHERE account_id = ? AND note_id = ?`
	sqlSelectLikesByNoteId     = `SELECT id, account_id, note_id, uri, created_at FROM likes WHERE note_id = ?`
	sqlCountLikesByNoteId      = `SELECT COUNT(*) FROM likes WHERE note_id = ?`
	sqlDeleteLikeByURI         = `DELETE FROM likes WHERE uri = ?`
	sqlDeleteLikeByAccountNote = `DELETE FROM likes WHERE account_id = ? AND note_id = ?`
	sqlUpdateNoteLikeCount     = `UPDATE notes SET like_count = ? WHERE id = ?`
)

// CreateLike creates a new like record
func (db *DB) CreateLike(like *domain.Like) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertLike,
			like.Id.String(),
			like.AccountId.String(),
			like.NoteId.String(),
			like.URI,
			like.CreatedAt)
		return err
	})
}

// HasLike checks if a like already exists for this account and note
func (db *DB) HasLike(accountId, noteId uuid.UUID) (bool, error) {
	var count int
	err := db.db.QueryRow(sqlSelectLikeExists, accountId.String(), noteId.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasLikeByURI checks if a like already exists by its activity URI
func (db *DB) HasLikeByURI(uri string) (bool, error) {
	var like domain.Like
	var idStr, accountIdStr, noteIdStr, createdAtStr string
	err := db.db.QueryRow(sqlSelectLikeByURI, uri).Scan(&idStr, &accountIdStr, &noteIdStr, &like.URI, &createdAtStr)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ReadLikesByNoteId returns all likes for a given note
func (db *DB) ReadLikesByNoteId(noteId uuid.UUID) (error, []domain.Like) {
	rows, err := db.db.Query(sqlSelectLikesByNoteId, noteId.String())
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var likes []domain.Like
	for rows.Next() {
		var like domain.Like
		var idStr, accountIdStr, noteIdStr, createdAtStr string
		if err := rows.Scan(&idStr, &accountIdStr, &noteIdStr, &like.URI, &createdAtStr); err != nil {
			return err, likes
		}
		like.Id = uuid.MustParse(idStr)
		like.AccountId = uuid.MustParse(accountIdStr)
		like.NoteId = uuid.MustParse(noteIdStr)
		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			like.CreatedAt = parsedTime
		}
		likes = append(likes, like)
	}
	if err = rows.Err(); err != nil {
		return err, likes
	}
	return nil, likes
}

// CountLikesByNoteId returns the number of likes for a given note
func (db *DB) CountLikesByNoteId(noteId uuid.UUID) (int, error) {
	var count int
	err := db.db.QueryRow(sqlCountLikesByNoteId, noteId.String()).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteLikeByURI removes a like by its activity URI
func (db *DB) DeleteLikeByURI(uri string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteLikeByURI, uri)
		return err
	})
}

// UpdateNoteLikeCount updates the like_count for a specific note
func (db *DB) UpdateNoteLikeCount(noteId uuid.UUID, count int) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateNoteLikeCount, count, noteId.String())
		return err
	})
}

// IncrementLikeCountByNoteId increments the like_count for a note and returns new count
func (db *DB) IncrementLikeCountByNoteId(noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE notes SET like_count = like_count + 1 WHERE id = ?`, noteId.String())
		return err
	})
}

// DecrementLikeCountByNoteId decrements the like_count for a note
func (db *DB) DecrementLikeCountByNoteId(noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE notes SET like_count = CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END WHERE id = ?`, noteId.String())
		return err
	})
}

// ReadLikeByAccountAndNote finds a like by the account that created it and the note it's on
func (db *DB) ReadLikeByAccountAndNote(accountId, noteId uuid.UUID) (error, *domain.Like) {
	var like domain.Like
	var idStr, accountIdStr, noteIdStr, createdAtStr string
	err := db.db.QueryRow(sqlSelectLikeByAccountNote, accountId.String(), noteId.String()).Scan(
		&idStr, &accountIdStr, &noteIdStr, &like.URI, &createdAtStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return err, nil
	}
	like.Id = uuid.MustParse(idStr)
	like.AccountId = uuid.MustParse(accountIdStr)
	like.NoteId = uuid.MustParse(noteIdStr)
	if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
		like.CreatedAt = parsedTime
	}
	return nil, &like
}

// DeleteLikeByAccountAndNote removes a like by the account and note IDs
func (db *DB) DeleteLikeByAccountAndNote(accountId, noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteLikeByAccountNote, accountId.String(), noteId.String())
		return err
	})
}

// IncrementLikeCountByObjectURI increments the like_count for an activity by object URI
func (db *DB) IncrementLikeCountByObjectURI(objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE activities SET like_count = like_count + 1 WHERE object_uri = ?`, objectURI)
		return err
	})
}

// DecrementLikeCountByObjectURI decrements the like_count for an activity by object URI
func (db *DB) DecrementLikeCountByObjectURI(objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE activities SET like_count = CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END WHERE object_uri = ?`, objectURI)
		return err
	})
}

// HasLikeByObjectURI checks if an account has liked a post by its object URI
func (db *DB) HasLikeByObjectURI(accountId uuid.UUID, objectURI string) (bool, error) {
	var count int
	err := db.db.QueryRow(`SELECT COUNT(*) FROM likes WHERE account_id = ? AND object_uri = ?`, accountId.String(), objectURI).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateLikeByObjectURI creates a like for a remote post using object URI
func (db *DB) CreateLikeByObjectURI(like *domain.Like, objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Use a deterministic UUID derived from the object_uri as the note_id placeholder
		// This ensures the unique constraint (account_id, note_id) works correctly for remote posts
		placeholderNoteId := uuid.NewSHA1(uuid.NameSpaceURL, []byte(objectURI))
		_, err := tx.Exec(`INSERT INTO likes(id, account_id, note_id, uri, object_uri, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			like.Id.String(),
			like.AccountId.String(),
			placeholderNoteId.String(), // Deterministic placeholder based on object_uri
			like.URI,
			objectURI,
			like.CreatedAt.Format(time.RFC3339))
		return err
	})
}

// ReadLikeByAccountAndObjectURI finds a like by account ID and object URI
func (db *DB) ReadLikeByAccountAndObjectURI(accountId uuid.UUID, objectURI string) (error, *domain.Like) {
	var like domain.Like
	var idStr, accountIdStr, noteIdStr, createdAtStr string
	var objURI sql.NullString
	err := db.db.QueryRow(`SELECT id, account_id, note_id, uri, object_uri, created_at FROM likes WHERE account_id = ? AND object_uri = ?`,
		accountId.String(), objectURI).Scan(&idStr, &accountIdStr, &noteIdStr, &like.URI, &objURI, &createdAtStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return err, nil
	}
	like.Id = uuid.MustParse(idStr)
	like.AccountId = uuid.MustParse(accountIdStr)
	if noteIdStr != "" {
		like.NoteId = uuid.MustParse(noteIdStr)
	}
	if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
		like.CreatedAt = parsedTime
	}
	return nil, &like
}

// DeleteLikeByAccountAndObjectURI removes a like by account ID and object URI
func (db *DB) DeleteLikeByAccountAndObjectURI(accountId uuid.UUID, objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM likes WHERE account_id = ? AND object_uri = ?`, accountId.String(), objectURI)
		return err
	})
}

// Boost queries
const (
	sqlInsertBoost              = `INSERT INTO boosts(id, account_id, note_id, uri, created_at) VALUES (?, ?, ?, ?, ?)`
	sqlSelectBoostExists        = `SELECT COUNT(*) FROM boosts WHERE account_id = ? AND note_id = ?`
	sqlSelectBoostByAccountNote = `SELECT id, account_id, note_id, uri, created_at FROM boosts WHERE account_id = ? AND note_id = ?`
	sqlDeleteBoostByAccountNote = `DELETE FROM boosts WHERE account_id = ? AND note_id = ?`
)

// CreateBoost creates a new boost record
func (db *DB) CreateBoost(boost *domain.Boost) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertBoost,
			boost.Id.String(),
			boost.AccountId.String(),
			boost.NoteId.String(),
			boost.URI,
			boost.CreatedAt.Format("2006-01-02 15:04:05"))
		return err
	})
}

// HasBoost checks if a boost already exists for this account and note
func (db *DB) HasBoost(accountId, noteId uuid.UUID) (bool, error) {
	var count int
	err := db.db.QueryRow(sqlSelectBoostExists, accountId.String(), noteId.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteBoostByAccountAndNote removes a boost by the account and note IDs
func (db *DB) DeleteBoostByAccountAndNote(accountId, noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteBoostByAccountNote, accountId.String(), noteId.String())
		return err
	})
}

// IncrementBoostCountByNoteId increments the boost_count for a note
func (db *DB) IncrementBoostCountByNoteId(noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE notes SET boost_count = boost_count + 1 WHERE id = ?`, noteId.String())
		return err
	})
}

// DecrementBoostCountByNoteId decrements the boost_count for a note
func (db *DB) DecrementBoostCountByNoteId(noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE notes SET boost_count = CASE WHEN boost_count > 0 THEN boost_count - 1 ELSE 0 END WHERE id = ?`, noteId.String())
		return err
	})
}

// ReadBoostByAccountAndNote finds a boost by account ID and note ID
func (db *DB) ReadBoostByAccountAndNote(accountId, noteId uuid.UUID) (error, *domain.Boost) {
	var boost domain.Boost
	var idStr, accountIdStr, noteIdStr, createdAtStr string
	err := db.db.QueryRow(sqlSelectBoostByAccountNote, accountId.String(), noteId.String()).Scan(
		&idStr, &accountIdStr, &noteIdStr, &boost.URI, &createdAtStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return err, nil
	}
	boost.Id = uuid.MustParse(idStr)
	boost.AccountId = uuid.MustParse(accountIdStr)
	boost.NoteId = uuid.MustParse(noteIdStr)
	if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
		boost.CreatedAt = parsedTime
	}
	return nil, &boost
}

// IncrementBoostCountByObjectURI increments the boost_count for an activity by object URI
func (db *DB) IncrementBoostCountByObjectURI(objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE activities SET boost_count = boost_count + 1 WHERE object_uri = ?`, objectURI)
		return err
	})
}

// DecrementBoostCountByObjectURI decrements the boost_count for an activity by object URI
func (db *DB) DecrementBoostCountByObjectURI(objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE activities SET boost_count = CASE WHEN boost_count > 0 THEN boost_count - 1 ELSE 0 END WHERE object_uri = ?`, objectURI)
		return err
	})
}

// HasBoostByObjectURI checks if an account has boosted a post by its object URI
func (db *DB) HasBoostByObjectURI(accountId uuid.UUID, objectURI string) (bool, error) {
	var count int
	err := db.db.QueryRow(`SELECT COUNT(*) FROM boosts WHERE account_id = ? AND object_uri = ?`, accountId.String(), objectURI).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateBoostByObjectURI creates a boost for a remote post using object URI
func (db *DB) CreateBoostByObjectURI(boost *domain.Boost, objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Use a deterministic UUID derived from the object_uri as the note_id placeholder
		// This ensures the unique constraint (account_id, note_id) works correctly for remote posts
		placeholderNoteId := uuid.NewSHA1(uuid.NameSpaceURL, []byte(objectURI))
		_, err := tx.Exec(`INSERT INTO boosts(id, account_id, note_id, uri, object_uri, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			boost.Id.String(),
			boost.AccountId.String(),
			placeholderNoteId.String(), // Deterministic placeholder based on object_uri
			boost.URI,
			objectURI,
			boost.CreatedAt.Format("2006-01-02 15:04:05"))
		return err
	})
}

// ReadBoostByAccountAndObjectURI finds a boost by account ID and object URI
func (db *DB) ReadBoostByAccountAndObjectURI(accountId uuid.UUID, objectURI string) (error, *domain.Boost) {
	var boost domain.Boost
	var idStr, accountIdStr, noteIdStr, createdAtStr string
	var objURI sql.NullString
	err := db.db.QueryRow(`SELECT id, account_id, note_id, uri, object_uri, created_at FROM boosts WHERE account_id = ? AND object_uri = ?`,
		accountId.String(), objectURI).Scan(&idStr, &accountIdStr, &noteIdStr, &boost.URI, &objURI, &createdAtStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return err, nil
	}
	boost.Id = uuid.MustParse(idStr)
	boost.AccountId = uuid.MustParse(accountIdStr)
	if noteIdStr != "" {
		boost.NoteId = uuid.MustParse(noteIdStr)
	}
	if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
		boost.CreatedAt = parsedTime
	}
	return nil, &boost
}

// DeleteBoostByAccountAndObjectURI removes a boost by account ID and object URI
func (db *DB) DeleteBoostByAccountAndObjectURI(accountId uuid.UUID, objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM boosts WHERE account_id = ? AND object_uri = ?`, accountId.String(), objectURI)
		return err
	})
}

// IsRemoteAccountFollowed checks if any local user follows the given remote account
func (db *DB) IsRemoteAccountFollowed(remoteAccountId uuid.UUID) (bool, error) {
	var exists int
	// Check if any local account follows this remote account
	// Local accounts have their id in the accounts table, remote accounts in remote_accounts
	// Follows: account_id is the follower (local), target_account_id is who they follow (remote)
	// Uses LIMIT 1 for early termination - we only need to know if at least one exists
	err := db.db.QueryRow(`
		SELECT 1
		FROM follows f
		INNER JOIN accounts a ON a.id = f.account_id
		WHERE f.target_account_id = ? AND f.accepted = 1
		LIMIT 1`,
		remoteAccountId.String()).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateBoostFromRemote creates a boost record with remote_account_id set (for boosts from followed remote users)
func (db *DB) CreateBoostFromRemote(boost *domain.Boost) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Use a deterministic UUID derived from the object_uri as the note_id placeholder
		// This ensures the unique constraint works correctly for remote posts
		noteIdStr := ""
		if boost.NoteId != uuid.Nil {
			noteIdStr = boost.NoteId.String()
		} else if boost.ObjectURI != "" {
			// Generate deterministic placeholder for remote posts
			placeholderNoteId := uuid.NewSHA1(uuid.NameSpaceURL, []byte(boost.ObjectURI))
			noteIdStr = placeholderNoteId.String()
		}

		_, err := tx.Exec(`INSERT INTO boosts(id, account_id, remote_account_id, note_id, object_uri, uri, created_at) VALUES (?, '', ?, ?, ?, ?, ?)`,
			boost.Id.String(),
			boost.RemoteAccountId.String(),
			noteIdStr,
			boost.ObjectURI,
			boost.URI,
			boost.CreatedAt.Format("2006-01-02 15:04:05"))
		return err
	})
}

// HasBoostFromRemote checks if a boost already exists from a remote account for a given object URI
func (db *DB) HasBoostFromRemote(remoteAccountId uuid.UUID, objectURI string) (bool, error) {
	var count int
	err := db.db.QueryRow(`SELECT COUNT(*) FROM boosts WHERE remote_account_id = ? AND object_uri = ?`,
		remoteAccountId.String(), objectURI).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteBoostByRemoteAccountAndObjectURI removes a boost by remote account ID and object URI
func (db *DB) DeleteBoostByRemoteAccountAndObjectURI(remoteAccountId uuid.UUID, objectURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM boosts WHERE remote_account_id = ? AND object_uri = ?`,
			remoteAccountId.String(), objectURI)
		return err
	})
}

// Reply query methods

// ReadRepliesByNoteId returns all direct replies to a local note by its UUID
func (db *DB) ReadRepliesByNoteId(noteId uuid.UUID) (error, *[]domain.Note) {
	// Build the object_uri for this note to find replies
	// First get the note to check if it has an object_uri
	err, note := db.ReadNoteId(noteId)
	if err != nil || note == nil {
		return err, nil
	}

	// If the note has an object_uri, search by that
	if note.ObjectURI != "" {
		return db.ReadRepliesByURI(note.ObjectURI)
	}

	// Otherwise search by the note ID in the in_reply_to_uri (for local notes without object_uri)
	rows, err := db.db.Query(`
		SELECT n.id, a.username, n.message, n.created_at, n.edited_at, n.in_reply_to_uri, n.object_uri, COALESCE(n.like_count, 0), COALESCE(n.boost_count, 0)
		FROM notes n
		INNER JOIN accounts a ON a.id = n.user_id
		WHERE n.in_reply_to_uri LIKE ?
		ORDER BY n.created_at ASC`,
		"%"+noteId.String()+"%")
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	return db.scanNotesWithReplyInfo(rows)
}

// ReadRepliesByURI returns all direct replies to a note by its ActivityPub URI
func (db *DB) ReadRepliesByURI(objectURI string) (error, *[]domain.Note) {
	rows, err := db.db.Query(`
		SELECT n.id, a.username, n.message, n.created_at, n.edited_at, n.in_reply_to_uri, n.object_uri, COALESCE(n.like_count, 0), COALESCE(n.boost_count, 0)
		FROM notes n
		INNER JOIN accounts a ON a.id = n.user_id
		WHERE n.in_reply_to_uri = ?
		ORDER BY n.created_at ASC`,
		objectURI)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	return db.scanNotesWithReplyInfo(rows)
}

// CountRepliesByNoteId counts the number of direct replies to a local note
func (db *DB) CountRepliesByNoteId(noteId uuid.UUID) (int, error) {
	// First get the note's object_uri
	err, note := db.ReadNoteId(noteId)
	if err != nil || note == nil {
		return 0, err
	}

	if note.ObjectURI != "" {
		return db.CountRepliesByURI(note.ObjectURI)
	}

	// Count by note ID in in_reply_to_uri
	var count int
	err = db.db.QueryRow(`SELECT COUNT(*) FROM notes WHERE in_reply_to_uri LIKE ?`,
		"%"+noteId.String()+"%").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountRepliesByURI counts the number of direct replies to a note by URI
func (db *DB) CountRepliesByURI(objectURI string) (int, error) {
	var count int
	err := db.db.QueryRow(`SELECT COUNT(*) FROM notes WHERE in_reply_to_uri = ?`, objectURI).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// incrementReplyCount increments the reply_count on the parent note or activity AND all ancestors
// This ensures that root-level posts show the total count of all nested replies
func (db *DB) incrementReplyCount(tx *sql.Tx, parentURI string) {
	db.incrementReplyCountRecursive(tx, parentURI, make(map[string]bool))
}

// incrementReplyCountRecursive walks up the ancestor chain and increments each reply_count
func (db *DB) incrementReplyCountRecursive(tx *sql.Tx, uri string, visited map[string]bool) {
	if uri == "" || visited[uri] {
		return
	}
	visited[uri] = true

	var nextParentURI string

	// Handle local: prefix for local-only mode (e.g., "local:414b193d-0b53-4657-b1bf-eb3a6091d672")
	if strings.HasPrefix(uri, "local:") {
		noteIdStr := strings.TrimPrefix(uri, "local:")
		result, _ := tx.Exec(`UPDATE notes SET reply_count = reply_count + 1 WHERE id = ?`, noteIdStr)
		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			// Found and updated a note, get its parent to continue up the chain
			tx.QueryRow(`SELECT COALESCE(in_reply_to_uri, '') FROM notes WHERE id = ?`, noteIdStr).Scan(&nextParentURI)
			db.incrementReplyCountRecursive(tx, nextParentURI, visited)
			return
		}
	}

	// Try to increment on notes table (by object_uri)
	result, _ := tx.Exec(`UPDATE notes SET reply_count = reply_count + 1 WHERE object_uri = ?`, uri)
	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		// Found and updated a note, now get its parent to continue up the chain
		tx.QueryRow(`SELECT COALESCE(in_reply_to_uri, '') FROM notes WHERE object_uri = ?`, uri).Scan(&nextParentURI)
		db.incrementReplyCountRecursive(tx, nextParentURI, visited)
		return
	}

	// Try to increment on notes table (by note ID in URI pattern like /notes/{uuid})
	if strings.Contains(uri, "/notes/") {
		parts := strings.Split(uri, "/notes/")
		if len(parts) == 2 {
			noteIdStr := parts[1]
			// Remove any trailing path
			if idx := strings.Index(noteIdStr, "/"); idx > 0 {
				noteIdStr = noteIdStr[:idx]
			}
			result, _ = tx.Exec(`UPDATE notes SET reply_count = reply_count + 1 WHERE id = ?`, noteIdStr)
			if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
				// Found and updated a note, get its parent
				tx.QueryRow(`SELECT COALESCE(in_reply_to_uri, '') FROM notes WHERE id = ?`, noteIdStr).Scan(&nextParentURI)
				db.incrementReplyCountRecursive(tx, nextParentURI, visited)
				return
			}
		}
	}

	// Try to increment on activities table (for remote posts)
	result, _ = tx.Exec(`UPDATE activities SET reply_count = reply_count + 1 WHERE object_uri = ?`, uri)
	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		// Found and updated an activity, extract inReplyTo from raw_json
		var rawJSON string
		tx.QueryRow(`SELECT raw_json FROM activities WHERE object_uri = ?`, uri).Scan(&rawJSON)
		nextParentURI = extractInReplyToFromJSON(rawJSON)
		db.incrementReplyCountRecursive(tx, nextParentURI, visited)
	}
}

// extractInReplyToFromJSON extracts the inReplyTo value from activity raw_json
func extractInReplyToFromJSON(rawJSON string) string {
	if rawJSON == "" {
		return ""
	}
	// Parse the JSON to extract inReplyTo from the object
	var activity struct {
		Object struct {
			InReplyTo interface{} `json:"inReplyTo"`
		} `json:"object"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &activity); err != nil {
		return ""
	}
	// inReplyTo can be a string or null
	if inReplyTo, ok := activity.Object.InReplyTo.(string); ok {
		return inReplyTo
	}
	return ""
}

// decrementReplyCount decrements the reply_count on the parent note or activity AND all ancestors
func (db *DB) decrementReplyCount(tx *sql.Tx, parentURI string) {
	db.decrementReplyCountRecursive(tx, parentURI, make(map[string]bool))
}

// decrementReplyCountRecursive walks up the ancestor chain and decrements each reply_count
func (db *DB) decrementReplyCountRecursive(tx *sql.Tx, uri string, visited map[string]bool) {
	if uri == "" || visited[uri] {
		return
	}
	visited[uri] = true

	var nextParentURI string

	// Handle local: prefix for local-only mode (e.g., "local:414b193d-0b53-4657-b1bf-eb3a6091d672")
	if strings.HasPrefix(uri, "local:") {
		noteIdStr := strings.TrimPrefix(uri, "local:")
		result, _ := tx.Exec(`UPDATE notes SET reply_count = MAX(0, reply_count - 1) WHERE id = ?`, noteIdStr)
		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			tx.QueryRow(`SELECT COALESCE(in_reply_to_uri, '') FROM notes WHERE id = ?`, noteIdStr).Scan(&nextParentURI)
			db.decrementReplyCountRecursive(tx, nextParentURI, visited)
			return
		}
	}

	// Try to decrement on notes table (by object_uri)
	result, _ := tx.Exec(`UPDATE notes SET reply_count = MAX(0, reply_count - 1) WHERE object_uri = ?`, uri)
	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		tx.QueryRow(`SELECT COALESCE(in_reply_to_uri, '') FROM notes WHERE object_uri = ?`, uri).Scan(&nextParentURI)
		db.decrementReplyCountRecursive(tx, nextParentURI, visited)
		return
	}

	// Try to decrement on notes table (by note ID in URI pattern)
	if strings.Contains(uri, "/notes/") {
		parts := strings.Split(uri, "/notes/")
		if len(parts) == 2 {
			noteIdStr := parts[1]
			if idx := strings.Index(noteIdStr, "/"); idx > 0 {
				noteIdStr = noteIdStr[:idx]
			}
			result, _ = tx.Exec(`UPDATE notes SET reply_count = MAX(0, reply_count - 1) WHERE id = ?`, noteIdStr)
			if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
				tx.QueryRow(`SELECT COALESCE(in_reply_to_uri, '') FROM notes WHERE id = ?`, noteIdStr).Scan(&nextParentURI)
				db.decrementReplyCountRecursive(tx, nextParentURI, visited)
				return
			}
		}
	}

	// Try to decrement on activities table
	result, _ = tx.Exec(`UPDATE activities SET reply_count = MAX(0, reply_count - 1) WHERE object_uri = ?`, uri)
	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		var rawJSON string
		tx.QueryRow(`SELECT raw_json FROM activities WHERE object_uri = ?`, uri).Scan(&rawJSON)
		nextParentURI = extractInReplyToFromJSON(rawJSON)
		db.decrementReplyCountRecursive(tx, nextParentURI, visited)
	}
}

// IncrementReplyCountByURI increments the reply_count on a note or activity by URI
// This is used when receiving remote replies via ActivityPub inbox
func (db *DB) IncrementReplyCountByURI(parentURI string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		db.incrementReplyCount(tx, parentURI)
		return nil
	})
}

// CountTotalRepliesByNoteId counts all replies (recursively) to a local note
// This includes direct replies and all nested replies in the thread
func (db *DB) CountTotalRepliesByNoteId(noteId uuid.UUID) (int, error) {
	// First get the note's object_uri
	err, note := db.ReadNoteId(noteId)
	if err != nil || note == nil {
		return 0, err
	}

	if note.ObjectURI != "" {
		return db.CountTotalRepliesByURI(note.ObjectURI)
	}

	// For notes without object_uri, use the note ID pattern
	localPattern := "%" + noteId.String() + "%"
	return db.countTotalRepliesRecursive(localPattern, "")
}

// CountTotalRepliesByURI counts all replies (recursively) to a note by URI
// This includes direct replies, remote replies, and all nested replies
func (db *DB) CountTotalRepliesByURI(objectURI string) (int, error) {
	return db.countTotalRepliesRecursive("", objectURI)
}

// countTotalRepliesRecursive recursively counts all replies in a thread
// It handles both local notes (by in_reply_to_uri) and remote activities (by inReplyTo in raw_json)
func (db *DB) countTotalRepliesRecursive(localPattern string, objectURI string) (int, error) {
	totalCount := 0

	// Get direct local replies
	var localReplies []struct {
		ID        uuid.UUID
		ObjectURI string
	}

	var rows *sql.Rows
	var err error

	if objectURI != "" {
		// Search by exact object_uri match
		rows, err = db.db.Query(`
			SELECT n.id, COALESCE(n.object_uri, '') as object_uri
			FROM notes n
			WHERE n.in_reply_to_uri = ?`,
			objectURI)
	} else if localPattern != "" {
		// Search by note ID pattern in in_reply_to_uri
		rows, err = db.db.Query(`
			SELECT n.id, COALESCE(n.object_uri, '') as object_uri
			FROM notes n
			WHERE n.in_reply_to_uri LIKE ?`,
			localPattern)
	} else {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var idStr string
		var uri string
		if err := rows.Scan(&idStr, &uri); err != nil {
			continue
		}
		id, _ := uuid.Parse(idStr)
		localReplies = append(localReplies, struct {
			ID        uuid.UUID
			ObjectURI string
		}{ID: id, ObjectURI: uri})
	}

	// Count direct local replies
	totalCount += len(localReplies)

	// Count direct remote replies (only if we have an objectURI)
	if objectURI != "" {
		remoteCount, _ := db.CountActivitiesByInReplyTo(objectURI)
		totalCount += remoteCount

		// Get remote reply URIs for recursive counting
		err, remoteActivities := db.ReadActivitiesByInReplyTo(objectURI)
		if err == nil && remoteActivities != nil {
			for _, activity := range *remoteActivities {
				if activity.ObjectURI != "" {
					// Recursively count replies to remote replies
					subCount, _ := db.countTotalRepliesRecursive("", activity.ObjectURI)
					totalCount += subCount
				}
			}
		}
	}

	// Recursively count replies to local replies
	for _, reply := range localReplies {
		if reply.ObjectURI != "" {
			subCount, _ := db.countTotalRepliesRecursive("", reply.ObjectURI)
			totalCount += subCount
		} else {
			// For notes without object_uri, we need to check both:
			// 1. Local notes that reply using the note ID pattern
			// 2. Remote activities that reply using the full constructed URI

			// Check local replies by note ID pattern
			subPattern := "%" + reply.ID.String() + "%"
			subCount, _ := db.countTotalRepliesRecursive(subPattern, "")
			totalCount += subCount

			// Also check remote activities that might use the full URI pattern
			// Remote servers use URIs like: https://domain/notes/{uuid}
			// We search for /notes/{uuid} pattern in raw_json directly
			noteURIPattern := "/notes/" + reply.ID.String()
			remoteReplyCount, _ := db.countRemoteRepliesByURIPattern(noteURIPattern)
			totalCount += remoteReplyCount
		}
	}

	return totalCount, nil
}

// countRemoteRepliesByURIPattern counts remote activities where inReplyTo contains the given URI pattern
// and recursively counts replies to those activities
func (db *DB) countRemoteRepliesByURIPattern(uriPattern string) (int, error) {
	// Find activities where inReplyTo contains this pattern
	rows, err := db.db.Query(`
		SELECT object_uri FROM activities
		WHERE activity_type = 'Create'
		AND raw_json LIKE ?`,
		`%"inReplyTo":"%`+uriPattern+`%`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int
	var remoteObjectURIs []string

	for rows.Next() {
		var objectURI string
		if err := rows.Scan(&objectURI); err != nil {
			continue
		}
		count++
		if objectURI != "" {
			remoteObjectURIs = append(remoteObjectURIs, objectURI)
		}
	}

	// Recursively count replies to these remote activities
	for _, uri := range remoteObjectURIs {
		subCount, _ := db.countTotalRepliesRecursive("", uri)
		count += subCount
	}

	return count, nil
}

// ============================================================================
// Engagement Info (Likers and Boosters)
// ============================================================================

// ReadLikersInfoByNoteId returns a list of usernames who liked a local note (local and remote users)
func (db *DB) ReadLikersInfoByNoteId(noteId uuid.UUID) ([]string, error) {
	rows, err := db.db.Query(`
		SELECT DISTINCT
			CASE
				WHEN a.username IS NOT NULL THEN a.username
				WHEN ra.username IS NOT NULL THEN ra.username || '@' || ra.domain
				ELSE 'unknown'
			END as display_name
		FROM likes l
		LEFT JOIN accounts a ON a.id = l.account_id
		LEFT JOIN remote_accounts ra ON ra.id = l.account_id
		WHERE l.note_id = ?
		ORDER BY display_name ASC`,
		noteId.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			continue
		}
		usernames = append(usernames, username)
	}
	return usernames, nil
}

// ReadLikersInfoByObjectURI returns a list of usernames who liked a remote post (local and remote users)
func (db *DB) ReadLikersInfoByObjectURI(objectURI string) ([]string, error) {
	rows, err := db.db.Query(`
		SELECT DISTINCT
			CASE
				WHEN a.username IS NOT NULL THEN a.username
				WHEN ra.username IS NOT NULL THEN ra.username || '@' || ra.domain
				ELSE 'unknown'
			END as display_name
		FROM likes l
		LEFT JOIN accounts a ON a.id = l.account_id
		LEFT JOIN remote_accounts ra ON ra.id = l.account_id
		WHERE l.object_uri = ?
		ORDER BY display_name ASC`,
		objectURI)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			continue
		}
		usernames = append(usernames, username)
	}
	return usernames, nil
}

// ReadBoostersInfoByNoteId returns a list of usernames who boosted a local note (local and remote users)
func (db *DB) ReadBoostersInfoByNoteId(noteId uuid.UUID) ([]string, error) {
	rows, err := db.db.Query(`
		SELECT DISTINCT
			CASE
				WHEN a.username IS NOT NULL THEN a.username
				WHEN ra.username IS NOT NULL THEN ra.username || '@' || ra.domain
				ELSE 'unknown'
			END as display_name
		FROM boosts b
		LEFT JOIN accounts a ON a.id = b.account_id
		LEFT JOIN remote_accounts ra ON ra.id = b.account_id
		WHERE b.note_id = ?
		ORDER BY display_name ASC`,
		noteId.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			continue
		}
		usernames = append(usernames, username)
	}
	return usernames, nil
}

// ReadBoostersInfoByObjectURI returns a list of usernames who boosted a remote post (local and remote users)
func (db *DB) ReadBoostersInfoByObjectURI(objectURI string) ([]string, error) {
	rows, err := db.db.Query(`
		SELECT DISTINCT
			CASE
				WHEN a.username IS NOT NULL THEN a.username
				WHEN ra.username IS NOT NULL THEN ra.username || '@' || ra.domain
				ELSE 'unknown'
			END as display_name
		FROM boosts b
		LEFT JOIN accounts a ON a.id = b.account_id
		LEFT JOIN remote_accounts ra ON ra.id = b.account_id
		WHERE b.object_uri = ?
		ORDER BY display_name ASC`,
		objectURI)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			continue
		}
		usernames = append(usernames, username)
	}
	return usernames, nil
}
