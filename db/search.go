// search.go — Full-text search (FTS5) indexing and querying.
// Principle: One File, One Mental Model — search index management as a self-contained concept.
package db

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// sanitizeFTSQuery removes FTS5 operators and converts to prefix-match tokens
func sanitizeFTSQuery(query string) string {
	// Remove FTS5 special characters
	replacer := strings.NewReplacer(
		`"`, ``,
		`(`, ``,
		`)`, ``,
		`*`, ``,
		`{`, ``,
		`}`, ``,
		`:`, ``,
		`^`, ``,
	)
	query = replacer.Replace(query)

	// Split into tokens and wrap each for prefix matching
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return ""
	}

	var parts []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		// Wrap in quotes and add wildcard for prefix matching
		parts = append(parts, `"`+token+`"*`)
	}
	return strings.Join(parts, " ")
}

// SearchPosts searches the FTS5 index for posts matching the query.
// The FTS5 table only stores content+author. Metadata comes from the lookup table,
// and object_uri/object_url are loaded from the source tables (notes/activities).
func (db *DB) SearchPosts(query string, maxResults int) (error, []domain.SearchResult) {
	ftsQuery := sanitizeFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}

	rows, err := db.db.Query(`
		SELECT
			l.source_id,
			l.source_type,
			f.author,
			snippet(posts_fts, 0, '<<', '>>', '...', 32),
			l.created_at,
			CASE l.source_type
				WHEN 'note' THEN COALESCE(n.object_uri, '')
				WHEN 'activity' THEN COALESCE(a.object_uri, '')
				ELSE ''
			END,
			CASE l.source_type
				WHEN 'activity' THEN COALESCE(a.object_url, '')
				ELSE ''
			END
		FROM posts_fts f
		JOIN posts_fts_lookup l ON l.fts_rowid = f.rowid
		LEFT JOIN notes n ON l.source_type = 'note' AND l.source_id = n.id
		LEFT JOIN activities a ON l.source_type = 'activity' AND l.source_id = a.id
		WHERE posts_fts MATCH ?
		ORDER BY rank, l.created_at DESC
		LIMIT ?
	`, ftsQuery, maxResults)
	if err != nil {
		return fmt.Errorf("FTS5 search failed: %w", err), nil
	}
	defer rows.Close()

	var results []domain.SearchResult
	for rows.Next() {
		var sourceID, sourceType, author, snippet, createdAtStr, objectURI, objectURL string
		if err := rows.Scan(&sourceID, &sourceType, &author, &snippet, &createdAtStr, &objectURI, &objectURL); err != nil {
			log.Printf("Warning: Failed to scan search result: %v", err)
			continue
		}

		createdAt, err := parseTimestamp(createdAtStr)
		if err != nil {
			createdAt = time.Time{}
		}

		result := domain.SearchResult{
			Author:     author,
			Snippet:    snippet,
			Time:       createdAt,
			ObjectURI:  objectURI,
			ObjectURL:  objectURL,
			IsLocal:    sourceType == "note",
			SourceID:   sourceID,
			SourceType: sourceType,
		}

		// Parse source ID as UUID
		if parsed, err := uuid.Parse(sourceID); err == nil {
			result.ID = parsed
			if sourceType == "note" {
				result.NoteID = parsed
			}
		}

		results = append(results, result)
	}

	return rows.Err(), results
}

// InsertNoteFTS adds a local note to the FTS index
func (db *DB) InsertNoteFTS(noteId uuid.UUID, author, message, createdAt, objectURI string) {
	result, err := db.db.Exec(
		`INSERT INTO posts_fts(content, author) VALUES (?, ?)`,
		message, author,
	)
	if err != nil {
		log.Printf("Warning: Failed to insert note %s into FTS: %v", noteId, err)
		return
	}
	if ftsRowid, err := result.LastInsertId(); err == nil {
		_, err = db.db.Exec(`INSERT OR REPLACE INTO posts_fts_lookup(source_id, fts_rowid, source_type, created_at) VALUES (?, ?, 'note', ?)`, noteId.String(), ftsRowid, createdAt)
		if err != nil {
			log.Printf("Warning: Failed to insert FTS lookup for note %s: %v", noteId, err)
		}
	}
}

// InsertActivityFTS adds a remote activity to the FTS index
func (db *DB) InsertActivityFTS(activityId uuid.UUID, actorURI, rawJSON, createdAt, objectURI, objectURL string) {
	content := extractContentFromJSON(rawJSON)
	if content == "" {
		return
	}
	author := extractAuthorFromActorURI(actorURI)

	result, err := db.db.Exec(
		`INSERT INTO posts_fts(content, author) VALUES (?, ?)`,
		content, author,
	)
	if err != nil {
		log.Printf("Warning: Failed to insert activity %s into FTS: %v", activityId, err)
		return
	}
	if ftsRowid, err := result.LastInsertId(); err == nil {
		_, err = db.db.Exec(`INSERT OR REPLACE INTO posts_fts_lookup(source_id, fts_rowid, source_type, created_at) VALUES (?, ?, 'activity', ?)`, activityId.String(), ftsRowid, createdAt)
		if err != nil {
			log.Printf("Warning: Failed to insert FTS lookup for activity %s: %v", activityId, err)
		}
	}
}

// DeleteFromFTS removes a post from the FTS index by source_id.
// Uses the lookup table to find the FTS rowid, then deletes by rowid.
func (db *DB) DeleteFromFTS(sourceID string) {
	// Look up the FTS rowid from the mapping table
	var ftsRowid int64
	err := db.db.QueryRow(`SELECT fts_rowid FROM posts_fts_lookup WHERE source_id = ?`, sourceID).Scan(&ftsRowid)
	if err != nil {
		// Row may not exist in FTS (e.g. table doesn't exist, or activity with empty content was never indexed)
		return
	}

	// Delete from standalone FTS5 table by rowid
	_, err = db.db.Exec(`DELETE FROM posts_fts WHERE rowid = ?`, ftsRowid)
	if err != nil {
		log.Printf("Warning: Failed to delete %s from FTS: %v", sourceID, err)
	}

	// Remove the lookup entry
	db.db.Exec(`DELETE FROM posts_fts_lookup WHERE source_id = ?`, sourceID)
}

// UpdateNoteFTS updates a note in the FTS index (delete + re-insert)
func (db *DB) UpdateNoteFTS(noteId uuid.UUID, author, message, createdAt, objectURI string) {
	db.DeleteFromFTS(noteId.String())
	db.InsertNoteFTS(noteId, author, message, createdAt, objectURI)
}

// DeleteRelayActivitiesFTS removes all relay-forwarded activities from the FTS index
func (db *DB) DeleteRelayActivitiesFTS() {
	// Get all relay activity IDs
	rows, err := db.db.Query(`SELECT id FROM activities WHERE from_relay = 1`)
	if err != nil {
		log.Printf("Warning: Failed to query relay activities for FTS cleanup: %v", err)
		return
	}
	defer rows.Close()

	deleted := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		db.DeleteFromFTS(id)
		deleted++
	}
	if deleted > 0 {
		log.Printf("Deleted %d relay activities from FTS index", deleted)
	}
}
