// notes.go — Note CRUD, hashtags, and mentions.
// Principle: One File, One Mental Model — note lifecycle from creation to deletion, including hashtag and mention linking.
package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// Note SQL constants
const (
	sqlInsertNote     = `INSERT INTO notes(id, user_id, message, created_at) VALUES (?, ?, ?, ?)`
	sqlUpdateNote     = `UPDATE notes SET message = ?, edited_at = ? WHERE id = ?`
	sqlDeleteNote     = `DELETE FROM notes WHERE id = ?`
	sqlSelectNoteById = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at, COALESCE(notes.like_count, 0), COALESCE(notes.boost_count, 0) FROM notes
   														INNER JOIN accounts ON accounts.id = notes.user_id
                                                           WHERE notes.id = ?`
	sqlSelectNotesByUserId = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at, notes.in_reply_to_uri, notes.like_count, notes.boost_count FROM notes
   														INNER JOIN accounts ON accounts.id = notes.user_id
                                                           WHERE notes.user_id = ?
                                                           ORDER BY notes.created_at DESC`
	sqlSelectNotesByUsername = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at, notes.in_reply_to_uri FROM notes
   														INNER JOIN accounts ON accounts.id = notes.user_id
                                                           WHERE accounts.username = ?
                                                           ORDER BY notes.created_at DESC`
	sqlSelectAllNotes = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at, notes.in_reply_to_uri, COALESCE(notes.like_count, 0), COALESCE(notes.boost_count, 0) FROM notes
   														INNER JOIN accounts ON accounts.id = notes.user_id
                                                           ORDER BY notes.created_at DESC`
)

// Hashtag queries
const (
	sqlInsertHashtag          = `INSERT INTO hashtags(name, usage_count, last_used_at) VALUES (?, 1, CURRENT_TIMESTAMP) ON CONFLICT(name) DO UPDATE SET usage_count = usage_count + 1, last_used_at = CURRENT_TIMESTAMP RETURNING id`
	sqlSelectHashtagByName    = `SELECT id, name, usage_count, last_used_at FROM hashtags WHERE name = ?`
	sqlInsertNoteHashtag      = `INSERT OR IGNORE INTO note_hashtags(note_id, hashtag_id) VALUES (?, ?)`
	sqlSelectHashtagsByNoteId = `SELECT h.name FROM hashtags h INNER JOIN note_hashtags nh ON h.id = nh.hashtag_id WHERE nh.note_id = ?`
	sqlSelectNotesByHashtag   = `SELECT n.id, a.username, n.message, n.created_at, n.edited_at, COALESCE(n.like_count, 0), COALESCE(n.boost_count, 0)
								FROM notes n
								INNER JOIN accounts a ON a.id = n.user_id
								INNER JOIN note_hashtags nh ON nh.note_id = n.id
								INNER JOIN hashtags h ON h.id = nh.hashtag_id
								WHERE h.name = ?
								ORDER BY n.created_at DESC
								LIMIT ? OFFSET ?`
	sqlCountNotesByHashtag = `SELECT COUNT(*) FROM note_hashtags nh INNER JOIN hashtags h ON h.id = nh.hashtag_id WHERE h.name = ?`
)

// Mention queries
const (
	sqlInsertNoteMention        = `INSERT INTO note_mentions(id, note_id, mentioned_actor_uri, mentioned_username, mentioned_domain, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlSelectMentionsByNoteId   = `SELECT id, note_id, mentioned_actor_uri, mentioned_username, mentioned_domain, created_at FROM note_mentions WHERE note_id = ?`
	sqlSelectMentionsByActorURI = `SELECT nm.id, nm.note_id, nm.mentioned_actor_uri, nm.mentioned_username, nm.mentioned_domain, nm.created_at
								   FROM note_mentions nm ORDER BY nm.created_at DESC LIMIT ? OFFSET ?`
	sqlDeleteMentionsByNoteId = `DELETE FROM note_mentions WHERE note_id = ?`
)

func (db *DB) CreateNote(userId uuid.UUID, message string) (uuid.UUID, error) {
	return db.CreateNoteWithReply(userId, message, "")
}

// CreateNoteWithReply creates a note with an optional inReplyToURI for replies
func (db *DB) CreateNoteWithReply(userId uuid.UUID, message string, inReplyToURI string) (uuid.UUID, error) {
	var noteId uuid.UUID
	err := db.wrapTransaction(func(tx *sql.Tx) error {
		id, err := db.insertNoteWithReply(tx, userId, message, inReplyToURI)
		if err != nil {
			return err
		}
		noteId = id
		return nil
	})
	if err == nil {
		// Index in FTS (best-effort)
		err2, acc := db.ReadAccById(userId)
		if err2 == nil {
			author := "@" + acc.Username
			createdAt := time.Now().Format("2006-01-02 15:04:05")
			db.InsertNoteFTS(noteId, author, message, createdAt, "")
		}
	}
	return noteId, err
}

func (db *DB) UpdateNote(noteId uuid.UUID, message string) error {
	err := db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.updateNote(tx, noteId, message)
		if err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		// Update FTS index (best-effort)
		err2, note := db.ReadNoteId(noteId)
		if err2 == nil {
			author := "@" + note.CreatedBy
			createdAt := note.CreatedAt.Format("2006-01-02 15:04:05")
			db.UpdateNoteFTS(noteId, author, message, createdAt, note.ObjectURI)
		}
	}
	return err
}

func (db *DB) DeleteNoteById(noteId uuid.UUID) error {
	// Delete from FTS before deleting from DB (need row data for standalone FTS delete)
	db.DeleteFromFTS(noteId.String())

	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.deleteNote(tx, noteId)
		if err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) ReadNotesByUserId(userId uuid.UUID) (error, *[]domain.Note) {
	rows, err := db.db.Query(sqlSelectNotesByUserId, userId)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notes []domain.Note

	for rows.Next() {
		var note domain.Note
		var createdAtStr string
		var editedAtStr sql.NullString
		var inReplyToURI sql.NullString
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &inReplyToURI, &note.LikeCount, &note.BoostCount); err != nil {
			return err, &notes
		}

		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			note.CreatedAt = parsedTime
		}

		if editedAtStr.Valid {
			if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
				note.EditedAt = &parsedTime
			}
		}

		if inReplyToURI.Valid {
			note.InReplyToURI = inReplyToURI.String
		}

		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return err, &notes
	}

	return nil, &notes
}

func (db *DB) ReadNotesByUsername(username string) (error, *[]domain.Note) {
	rows, err := db.db.Query(sqlSelectNotesByUsername, username)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notes []domain.Note

	for rows.Next() {
		var note domain.Note
		var createdAtStr string
		var editedAtStr sql.NullString
		var inReplyToURI sql.NullString
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &inReplyToURI); err != nil {
			return err, &notes
		}

		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			note.CreatedAt = parsedTime
		}

		if editedAtStr.Valid {
			if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
				note.EditedAt = &parsedTime
			}
		}

		if inReplyToURI.Valid {
			note.InReplyToURI = inReplyToURI.String
		}

		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return err, &notes
	}

	return nil, &notes
}

func (db *DB) ReadNoteId(id uuid.UUID) (error, *domain.Note) {
	row := db.db.QueryRow(sqlSelectNoteById, id)
	var note domain.Note
	var createdAtStr string
	var editedAtStr sql.NullString
	err := row.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &note.LikeCount, &note.BoostCount)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}

	// Parse created_at timestamp
	note.CreatedAt, err = parseTimestamp(createdAtStr)
	if err != nil {
		return err, nil
	}

	// Parse edited_at if present
	if editedAtStr.Valid {
		if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
			note.EditedAt = &parsedTime
		}
	}
	return nil, &note
}

func (db *DB) ReadAllNotes() (error, *[]domain.Note) {
	rows, err := db.db.Query(sqlSelectAllNotes)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notes []domain.Note

	for rows.Next() {
		var note domain.Note
		var createdAtStr string
		var editedAtStr sql.NullString
		var inReplyToURI sql.NullString
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &inReplyToURI, &note.LikeCount, &note.BoostCount); err != nil {
			return err, &notes
		}

		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			note.CreatedAt = parsedTime
		}

		if editedAtStr.Valid {
			if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
				note.EditedAt = &parsedTime
			}
		}

		if inReplyToURI.Valid {
			note.InReplyToURI = inReplyToURI.String
		}

		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return err, &notes
	}

	return nil, &notes
}

func (db *DB) insertNote(tx *sql.Tx, userId uuid.UUID, message string) (uuid.UUID, error) {
	return db.insertNoteWithReply(tx, userId, message, "")
}

func (db *DB) insertNoteWithReply(tx *sql.Tx, userId uuid.UUID, message string, inReplyToURI string) (uuid.UUID, error) {
	noteId := uuid.New()
	if inReplyToURI == "" {
		_, err := tx.Exec(sqlInsertNote, noteId, userId, message, time.Now().Format("2006-01-02 15:04:05"))
		return noteId, err
	}
	// Insert note with inReplyToURI
	_, err := tx.Exec(`INSERT INTO notes(id, user_id, message, created_at, in_reply_to_uri) VALUES (?, ?, ?, ?, ?)`,
		noteId, userId, message, time.Now().Format("2006-01-02 15:04:05"), inReplyToURI)
	if err != nil {
		return noteId, err
	}

	// Increment reply count on the parent (handles both notes and activities)
	db.incrementReplyCount(tx, inReplyToURI)

	return noteId, nil
}

func (db *DB) updateNote(tx *sql.Tx, noteId uuid.UUID, message string) error {
	_, err := tx.Exec(sqlUpdateNote, message, time.Now().Format("2006-01-02 15:04:05"), noteId)
	return err
}

func (db *DB) deleteNote(tx *sql.Tx, noteId uuid.UUID) error {
	// First, get the note's in_reply_to_uri so we can decrement the parent's count
	var inReplyToURI sql.NullString
	err := tx.QueryRow(`SELECT in_reply_to_uri FROM notes WHERE id = ?`, noteId.String()).Scan(&inReplyToURI)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Delete the note
	_, err = tx.Exec(sqlDeleteNote, noteId)
	if err != nil {
		return err
	}

	// Decrement reply count on the parent if this was a reply
	if inReplyToURI.Valid && inReplyToURI.String != "" {
		db.decrementReplyCount(tx, inReplyToURI.String)
	}

	return nil
}

// CreateOrUpdateHashtag creates a new hashtag or increments usage count if it exists
// Returns the hashtag ID
func (db *DB) CreateOrUpdateHashtag(name string) (int64, error) {
	var hashtagId int64
	err := db.wrapTransaction(func(tx *sql.Tx) error {
		err := tx.QueryRow(sqlInsertHashtag, strings.ToLower(name)).Scan(&hashtagId)
		if err != nil {
			return err
		}
		return nil
	})
	return hashtagId, err
}

// LinkNoteHashtags creates links between a note and multiple hashtags
func (db *DB) LinkNoteHashtags(noteId uuid.UUID, hashtagIds []int64) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		for _, hashtagId := range hashtagIds {
			_, err := tx.Exec(sqlInsertNoteHashtag, noteId.String(), hashtagId)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// ReadHashtagsByNoteId returns all hashtag names for a given note
func (db *DB) ReadHashtagsByNoteId(noteId uuid.UUID) (error, []string) {
	rows, err := db.db.Query(sqlSelectHashtagsByNoteId, noteId.String())
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var hashtags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err, hashtags
		}
		hashtags = append(hashtags, name)
	}
	if err = rows.Err(); err != nil {
		return err, hashtags
	}
	return nil, hashtags
}

// ReadNotesByHashtag returns notes that contain a specific hashtag with pagination
func (db *DB) ReadNotesByHashtag(tag string, limit, offset int) (error, *[]domain.Note) {
	rows, err := db.db.Query(sqlSelectNotesByHashtag, strings.ToLower(tag), limit, offset)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notes []domain.Note
	for rows.Next() {
		var note domain.Note
		var createdAtStr string
		var editedAtStr sql.NullString
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &note.LikeCount, &note.BoostCount); err != nil {
			return err, &notes
		}

		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			note.CreatedAt = parsedTime
		}

		if editedAtStr.Valid {
			if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
				note.EditedAt = &parsedTime
			}
		}

		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return err, &notes
	}
	return nil, &notes
}

// CountNotesByHashtag returns the total count of notes with a specific hashtag
func (db *DB) CountNotesByHashtag(tag string) (int, error) {
	var count int
	err := db.db.QueryRow(sqlCountNotesByHashtag, strings.ToLower(tag)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CreateNoteMention creates a new mention record for a note
func (db *DB) CreateNoteMention(mention *domain.NoteMention) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertNoteMention,
			mention.Id.String(),
			mention.NoteId.String(),
			mention.MentionedActorURI,
			strings.ToLower(mention.MentionedUsername),
			strings.ToLower(mention.MentionedDomain),
			mention.CreatedAt)
		return err
	})
}

// LinkNoteMentions creates mention records for all mentions in a note
func (db *DB) LinkNoteMentions(noteId uuid.UUID, mentions []domain.NoteMention) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		for _, mention := range mentions {
			_, err := tx.Exec(sqlInsertNoteMention,
				mention.Id.String(),
				noteId.String(),
				mention.MentionedActorURI,
				strings.ToLower(mention.MentionedUsername),
				strings.ToLower(mention.MentionedDomain),
				time.Now())
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// ReadMentionsByNoteId returns all mentions for a given note
func (db *DB) ReadMentionsByNoteId(noteId uuid.UUID) (error, []domain.NoteMention) {
	rows, err := db.db.Query(sqlSelectMentionsByNoteId, noteId.String())
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var mentions []domain.NoteMention
	for rows.Next() {
		var mention domain.NoteMention
		var idStr, noteIdStr, createdAtStr string
		if err := rows.Scan(&idStr, &noteIdStr, &mention.MentionedActorURI, &mention.MentionedUsername, &mention.MentionedDomain, &createdAtStr); err != nil {
			return err, mentions
		}
		mention.Id = uuid.MustParse(idStr)
		mention.NoteId = uuid.MustParse(noteIdStr)
		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			mention.CreatedAt = parsedTime
		}
		mentions = append(mentions, mention)
	}
	if err = rows.Err(); err != nil {
		return err, mentions
	}
	return nil, mentions
}

// DeleteMentionsByNoteId removes all mentions for a specific note
func (db *DB) DeleteMentionsByNoteId(noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteMentionsByNoteId, noteId.String())
		return err
	})
}

// ReadNoteByURI finds a local note by its ActivityPub object_uri
// It first tries an exact match on the object_uri column, then falls back
// to extracting the UUID from the URI pattern /notes/{uuid}
func (db *DB) ReadNoteByURI(objectURI string) (error, *domain.Note) {
	// First try exact match on object_uri column
	row := db.db.QueryRow(`
		SELECT n.id, a.username, n.message, n.created_at, n.edited_at, n.in_reply_to_uri, n.object_uri, COALESCE(n.like_count, 0), COALESCE(n.boost_count, 0)
		FROM notes n
		INNER JOIN accounts a ON a.id = n.user_id
		WHERE n.object_uri = ?`,
		objectURI)

	var note domain.Note
	var createdAtStr string
	var editedAtStr, inReplyToURI, noteObjectURI sql.NullString
	err := row.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &inReplyToURI, &noteObjectURI, &note.LikeCount, &note.BoostCount)
	if err == nil {
		note.CreatedAt, _ = parseTimestamp(createdAtStr)
		if editedAtStr.Valid {
			if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
				note.EditedAt = &parsedTime
			}
		}
		note.InReplyToURI = inReplyToURI.String
		note.ObjectURI = noteObjectURI.String
		return nil, &note
	}

	// If not found by object_uri, try extracting UUID from URI pattern /notes/{uuid}
	// This handles cases where object_uri wasn't stored in the database
	if err == sql.ErrNoRows {
		// Extract UUID from URI like "https://domain/notes/414b193d-0b53-4657-b1bf-eb3a6091d672"
		parts := strings.Split(objectURI, "/notes/")
		if len(parts) == 2 {
			noteIdStr := parts[1]
			// Remove any trailing path segments
			if idx := strings.Index(noteIdStr, "/"); idx > 0 {
				noteIdStr = noteIdStr[:idx]
			}
			if noteId, parseErr := uuid.Parse(noteIdStr); parseErr == nil {
				// Found a valid UUID, look up by ID
				return db.ReadNoteIdWithReplyInfo(noteId)
			}
		}
	}

	return err, nil
}

// ReadNoteIdWithReplyInfo returns a note with full reply information
func (db *DB) ReadNoteIdWithReplyInfo(id uuid.UUID) (error, *domain.Note) {
	row := db.db.QueryRow(`
		SELECT n.id, a.username, n.message, n.created_at, n.edited_at, n.in_reply_to_uri, n.object_uri, COALESCE(n.like_count, 0), COALESCE(n.boost_count, 0)
		FROM notes n
		INNER JOIN accounts a ON a.id = n.user_id
		WHERE n.id = ?`,
		id)

	var note domain.Note
	var createdAtStr string
	var editedAtStr, inReplyToURI, objectURI sql.NullString
	err := row.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &inReplyToURI, &objectURI, &note.LikeCount, &note.BoostCount)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}

	note.CreatedAt, _ = parseTimestamp(createdAtStr)
	if editedAtStr.Valid {
		if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
			note.EditedAt = &parsedTime
		}
	}
	note.InReplyToURI = inReplyToURI.String
	note.ObjectURI = objectURI.String

	return nil, &note
}

// scanNotesWithReplyInfo is a helper to scan notes rows including reply info
func (db *DB) scanNotesWithReplyInfo(rows *sql.Rows) (error, *[]domain.Note) {
	var notes []domain.Note
	for rows.Next() {
		var note domain.Note
		var createdAtStr string
		var editedAtStr, inReplyToURI, objectURI sql.NullString
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr, &inReplyToURI, &objectURI, &note.LikeCount, &note.BoostCount); err != nil {
			return err, &notes
		}

		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			note.CreatedAt = parsedTime
		}
		if editedAtStr.Valid {
			if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
				note.EditedAt = &parsedTime
			}
		}
		note.InReplyToURI = inReplyToURI.String
		note.ObjectURI = objectURI.String

		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return err, &notes
	}
	return nil, &notes
}
