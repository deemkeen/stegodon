// timeline.go — Home, global, and federated timeline queries with content extraction helpers.
// Principle: Locality over Abstraction — complex timeline SQL and its supporting helper
// functions live together because they must be understood as a unit.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// SQL constants for local timeline queries
const (
	sqlSelectLocalTimelineNotes = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
														INNER JOIN accounts ON accounts.id = notes.user_id
														ORDER BY notes.created_at DESC LIMIT ?`
	sqlSelectLocalTimelineNotesByFollows = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
														INNER JOIN accounts ON accounts.id = notes.user_id
														WHERE (notes.in_reply_to_uri IS NULL OR notes.in_reply_to_uri = '')
														AND (notes.user_id = ? OR notes.user_id IN (
															SELECT target_account_id FROM follows
															WHERE account_id = ? AND accepted = 1 AND is_local = 1
														))
														ORDER BY notes.created_at DESC LIMIT ?`
)

// ReadFederatedActivities returns recent Create activities from remote actors
const (
	sqlSelectFederatedActivities          = `SELECT id, activity_uri, activity_type, actor_uri, object_uri, raw_json, processed, local, created_at FROM activities WHERE activity_type = 'Create' AND local = 0 ORDER BY created_at DESC LIMIT ?`
	sqlSelectFederatedActivitiesByFollows = `SELECT a.id, a.activity_uri, a.activity_type, a.actor_uri, a.object_uri, a.raw_json, a.processed, a.local, a.created_at
		FROM activities a
		INNER JOIN remote_accounts ra ON ra.actor_uri = a.actor_uri
		INNER JOIN follows f ON f.target_account_id = ra.id
		WHERE a.activity_type = 'Create' AND a.local = 0 AND f.account_id = ? AND f.accepted = 1 AND f.is_local = 0
		ORDER BY a.created_at DESC LIMIT ?`
)

func (db *DB) ReadFederatedActivities(accountId uuid.UUID, limit int) (error, *[]domain.Activity) {
	rows, err := db.db.Query(sqlSelectFederatedActivitiesByFollows, accountId.String(), limit)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var activities []domain.Activity
	for rows.Next() {
		var activity domain.Activity
		var idStr string
		var createdAtStr string
		if err := rows.Scan(&idStr, &activity.ActivityURI, &activity.ActivityType, &activity.ActorURI, &activity.ObjectURI, &activity.RawJSON, &activity.Processed, &activity.Local, &createdAtStr); err != nil {
			return err, &activities
		}
		activity.Id, _ = uuid.Parse(idStr)

		if parsedTime, err := parseTimestamp(createdAtStr); err == nil {
			activity.CreatedAt = parsedTime
		}

		activities = append(activities, activity)
	}
	if err = rows.Err(); err != nil {
		return err, &activities
	}
	return nil, &activities
}

// Home Timeline queries - combines local notes and remote activities
const (
	// Non-boost posts for home timeline: local notes + followed remote + relay activities
	// Uses UNION ALL for efficient SQL-level sorting and pagination
	sqlSelectHomeNonBoostPosts = `
		SELECT id, author, content, created_at, object_uri, object_url,
		       is_local, reply_count, like_count, boost_count, '' as boosted_by
		FROM (
			-- Local notes (own + followed local users, excluding replies)
			SELECT
				n.id as id,
				a.username as author,
				n.message as content,
				n.created_at as created_at,
				COALESCE(n.object_uri, '') as object_uri,
				'' as object_url,
				1 as is_local,
				COALESCE(n.reply_count, 0) as reply_count,
				COALESCE(n.like_count, 0) as like_count,
				COALESCE(n.boost_count, 0) as boost_count,
				'' as raw_json
			FROM notes n
			INNER JOIN accounts a ON a.id = n.user_id
			WHERE (n.in_reply_to_uri IS NULL OR n.in_reply_to_uri = '')
			AND (n.user_id = ? OR n.user_id IN (
				SELECT target_account_id FROM follows
				WHERE account_id = ? AND accepted = 1 AND is_local = 1
			))

			UNION ALL

			-- Remote activities from followed remote users (excluding replies)
			SELECT
				act.id as id,
				'@' || ra.username || '@' || ra.domain as author,
				act.raw_json as content,
				act.created_at as created_at,
				act.object_uri as object_uri,
				COALESCE(act.object_url, '') as object_url,
				0 as is_local,
				COALESCE(act.reply_count, 0) as reply_count,
				COALESCE(act.like_count, 0) as like_count,
				COALESCE(act.boost_count, 0) as boost_count,
				act.raw_json as raw_json
			FROM activities act
			INNER JOIN remote_accounts ra ON ra.actor_uri = act.actor_uri
			INNER JOIN follows f ON f.target_account_id = ra.id
			WHERE act.activity_type = 'Create' AND act.local = 0
			AND f.account_id = ? AND f.accepted = 1 AND f.is_local = 0
			AND (act.in_reply_to IS NULL OR act.in_reply_to = '')

			UNION ALL

			-- Relay-forwarded activities (excluding replies)
			SELECT
				act.id as id,
				act.actor_uri as author,
				act.raw_json as content,
				act.created_at as created_at,
				act.object_uri as object_uri,
				COALESCE(act.object_url, '') as object_url,
				0 as is_local,
				COALESCE(act.reply_count, 0) as reply_count,
				COALESCE(act.like_count, 0) as like_count,
				COALESCE(act.boost_count, 0) as boost_count,
				act.raw_json as raw_json
			FROM activities act
			WHERE act.activity_type = 'Create' AND act.local = 0 AND act.from_relay = 1
			AND (act.in_reply_to IS NULL OR act.in_reply_to = '')
		) combined
		ORDER BY created_at DESC LIMIT ?`

	// Boost posts for home timeline: boosted local + boosted remote (local boosters) + boosted remote (remote boosters)
	// Uses UNION ALL for efficient SQL-level sorting and pagination
	sqlSelectHomeBoostPosts = `
		SELECT id, author, content, created_at, object_uri, object_url,
		       is_local, reply_count, like_count, boost_count, boosted_by
		FROM (
			-- Boosted local posts (own boosts + boosts from followed local users)
			SELECT
				n.id as id,
				a.username as author,
				n.message as content,
				b.created_at as created_at,
				COALESCE(n.object_uri, '') as object_uri,
				'' as object_url,
				1 as is_local,
				COALESCE(n.reply_count, 0) as reply_count,
				COALESCE(n.like_count, 0) as like_count,
				COALESCE(n.boost_count, 0) as boost_count,
				'@' || booster.username as boosted_by,
				'' as raw_json
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN notes n ON n.id = b.note_id
			INNER JOIN accounts a ON a.id = n.user_id
			WHERE b.account_id = ? AND n.user_id != ?

			UNION

			SELECT
				n.id as id,
				a.username as author,
				n.message as content,
				b.created_at as created_at,
				COALESCE(n.object_uri, '') as object_uri,
				'' as object_url,
				1 as is_local,
				COALESCE(n.reply_count, 0) as reply_count,
				COALESCE(n.like_count, 0) as like_count,
				COALESCE(n.boost_count, 0) as boost_count,
				'@' || booster.username as boosted_by,
				'' as raw_json
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN notes n ON n.id = b.note_id
			INNER JOIN accounts a ON a.id = n.user_id
			INNER JOIN follows f ON f.target_account_id = b.account_id AND f.account_id = ? AND f.accepted = 1

			UNION ALL

			-- Boosted remote posts (local boosters: own + followed)
			SELECT
				act.id as id,
				'@' || ra.username || '@' || ra.domain as author,
				act.raw_json as content,
				b.created_at as created_at,
				act.object_uri as object_uri,
				COALESCE(act.object_url, '') as object_url,
				0 as is_local,
				COALESCE(act.reply_count, 0) as reply_count,
				COALESCE(act.like_count, 0) as like_count,
				COALESCE(act.boost_count, 0) as boost_count,
				'@' || booster.username as boosted_by,
				act.raw_json as raw_json
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN activities act ON act.object_uri = b.object_uri
			INNER JOIN remote_accounts ra ON ra.actor_uri = act.actor_uri
			WHERE b.account_id = ? AND b.object_uri IS NOT NULL AND b.object_uri != ''

			UNION

			SELECT
				act.id as id,
				'@' || ra.username || '@' || ra.domain as author,
				act.raw_json as content,
				b.created_at as created_at,
				act.object_uri as object_uri,
				COALESCE(act.object_url, '') as object_url,
				0 as is_local,
				COALESCE(act.reply_count, 0) as reply_count,
				COALESCE(act.like_count, 0) as like_count,
				COALESCE(act.boost_count, 0) as boost_count,
				'@' || booster.username as boosted_by,
				act.raw_json as raw_json
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN activities act ON act.object_uri = b.object_uri
			INNER JOIN remote_accounts ra ON ra.actor_uri = act.actor_uri
			INNER JOIN follows f ON f.target_account_id = b.account_id AND f.account_id = ? AND f.accepted = 1
			WHERE b.object_uri IS NOT NULL AND b.object_uri != ''

			UNION ALL

			-- Boosted remote posts (remote boosters from followed remote users)
			SELECT
				act.id as id,
				'@' || ra_author.username || '@' || ra_author.domain as author,
				act.raw_json as content,
				b.created_at as created_at,
				act.object_uri as object_uri,
				COALESCE(act.object_url, '') as object_url,
				0 as is_local,
				COALESCE(act.reply_count, 0) as reply_count,
				COALESCE(act.like_count, 0) as like_count,
				COALESCE(act.boost_count, 0) as boost_count,
				'@' || ra_booster.username || '@' || ra_booster.domain as boosted_by,
				act.raw_json as raw_json
			FROM boosts b
			INNER JOIN remote_accounts ra_booster ON ra_booster.id = b.remote_account_id
			INNER JOIN activities act ON act.object_uri = b.object_uri
			INNER JOIN remote_accounts ra_author ON ra_author.actor_uri = act.actor_uri
			INNER JOIN follows f ON f.target_account_id = b.remote_account_id AND f.account_id = ? AND f.accepted = 1
			WHERE b.remote_account_id IS NOT NULL AND b.remote_account_id != ''
			AND b.object_uri IS NOT NULL AND b.object_uri != ''
		) combined
		ORDER BY created_at DESC LIMIT ?`
)

// ReadHomeTimelinePosts returns a unified home timeline combining local and remote posts.
// Uses 2 UNION ALL queries (non-boost + boost) for efficient SQL-level sorting.
func (db *DB) ReadHomeTimelinePosts(accountId uuid.UUID, limit int) (error, *[]domain.HomePost) {
	var posts []domain.HomePost
	aid := accountId.String()

	// scanRows processes rows from either the non-boost or boost query into HomePosts.
	// Both queries return the same column shape: id, author, content, created_at,
	// object_uri, object_url, is_local, reply_count, like_count, boost_count, boosted_by
	scanRows := func(rows *sql.Rows) error {
		defer rows.Close()
		for rows.Next() {
			var idStr string
			var author string
			var content string
			var createdAtStr string
			var objectURI string
			var objectURL string
			var isLocalInt int
			var replyCount int
			var likeCount int
			var boostCount int
			var boostedBy string

			if err := rows.Scan(&idStr, &author, &content, &createdAtStr, &objectURI, &objectURL,
				&isLocalInt, &replyCount, &likeCount, &boostCount, &boostedBy); err != nil {
				return err
			}

			postId, _ := uuid.Parse(idStr)
			parsedTime, _ := parseTimestamp(createdAtStr)
			isLocal := isLocalInt == 1

			post := domain.HomePost{
				ID:         postId,
				Author:     author,
				Time:       parsedTime,
				ObjectURI:  objectURI,
				ObjectURL:  objectURL,
				IsLocal:    isLocal,
				ReplyCount: replyCount,
				LikeCount:  likeCount,
				BoostCount: boostCount,
				BoostedBy:  boostedBy,
			}

			if isLocal {
				post.Content = content
				post.NoteID = postId
			} else {
				// For remote posts, extract content from raw JSON
				post.Content = extractContentFromJSON(content)
				if post.Content == "" && objectURL != "" {
					post.Content = objectURL
				}
				post.NoteID = uuid.Nil
				// For relay posts, author is the raw actor_uri — convert it
				if strings.HasPrefix(author, "https://") {
					post.Author = extractAuthorFromActorURI(author)
				}
			}

			posts = append(posts, post)
		}
		return rows.Err()
	}

	// Query A: Non-boost posts (local + remote followed + relay)
	// Params: accountId x3 (local notes user_id, follows account_id x2), limit
	nonBoostRows, err := db.db.Query(sqlSelectHomeNonBoostPosts, aid, aid, aid, limit)
	if err != nil {
		return err, nil
	}
	if err := scanRows(nonBoostRows); err != nil {
		return err, &posts
	}

	// Query B: Boost posts (boosted local + boosted remote local boosters + boosted remote remote boosters)
	// Params: accountId x7 (own boosts x2, followed local boosts, own remote boosts, followed remote boosts, followed remote booster, limit)
	boostRows, err := db.db.Query(sqlSelectHomeBoostPosts, aid, aid, aid, aid, aid, aid, limit)
	if err != nil {
		return err, &posts
	}
	if err := scanRows(boostRows); err != nil {
		return err, &posts
	}

	// Deduplicate posts - prefer non-boosted version (original) over boosted
	seen := make(map[uuid.UUID]int) // maps ID to index in posts slice
	var dedupedPosts []domain.HomePost
	for _, post := range posts {
		if existingIdx, exists := seen[post.ID]; exists {
			// If the new post is the original (no BoostedBy) and existing is boosted, replace
			if post.BoostedBy == "" && dedupedPosts[existingIdx].BoostedBy != "" {
				dedupedPosts[existingIdx] = post
			}
			// Otherwise keep the existing one
		} else {
			seen[post.ID] = len(dedupedPosts)
			dedupedPosts = append(dedupedPosts, post)
		}
	}
	posts = dedupedPosts

	// Sort combined posts by time (newest first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Time.After(posts[j].Time)
	})

	// Limit to requested amount
	if len(posts) > limit {
		posts = posts[:limit]
	}

	return nil, &posts
}

func (db *DB) ReadLocalTimelineNotes(accountId uuid.UUID, limit int) (error, *[]domain.Note) {
	rows, err := db.db.Query(sqlSelectLocalTimelineNotesByFollows, accountId.String(), accountId.String(), limit)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notes []domain.Note
	for rows.Next() {
		var note domain.Note
		var createdAtStr string
		var editedAtStr sql.NullString
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &createdAtStr, &editedAtStr); err != nil {
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

// extractContentFromJSON extracts content from ActivityPub Create activity JSON
func extractContentFromJSON(rawJSON string) string {
	// Properly unmarshal JSON to extract content
	var activityWrapper struct {
		Type   string `json:"type"`
		Object struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"object"`
	}

	if err := json.Unmarshal([]byte(rawJSON), &activityWrapper); err != nil {
		// Fallback to simple string parsing if JSON unmarshal fails
		if idx := strings.Index(rawJSON, `"content":"`); idx >= 0 {
			start := idx + len(`"content":"`)
			end := strings.Index(rawJSON[start:], `"`)
			if end > 0 {
				content := rawJSON[start : start+end]
				return util.StripHTMLTags(content)
			}
		}
		return ""
	}

	// Skip if content is empty
	if activityWrapper.Object.Content == "" {
		return ""
	}

	// Strip HTML tags from content
	return util.StripHTMLTags(activityWrapper.Object.Content)
}

// extractAuthorFromActorURI extracts username@domain from an ActivityPub actor URI
// e.g., "https://mastodon.social/users/alice" -> "@alice@mastodon.social"
func extractAuthorFromActorURI(actorURI string) string {
	// Remove protocol prefix
	uri := strings.TrimPrefix(actorURI, "https://")
	uri = strings.TrimPrefix(uri, "http://")

	// Split into domain and path
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) < 2 {
		return actorURI // Return original if can't parse
	}

	domain := parts[0]
	path := parts[1]

	// Extract username from path (common patterns: /users/X, /@X, /u/X)
	var username string
	if strings.HasPrefix(path, "users/") {
		username = strings.TrimPrefix(path, "users/")
	} else if strings.HasPrefix(path, "@") {
		username = strings.TrimPrefix(path, "@")
	} else if strings.HasPrefix(path, "u/") {
		username = strings.TrimPrefix(path, "u/")
	} else {
		// Just use last path segment as username
		pathParts := strings.Split(path, "/")
		username = pathParts[len(pathParts)-1]
	}

	// Remove any trailing path segments from username
	if idx := strings.Index(username, "/"); idx > 0 {
		username = username[:idx]
	}

	return "@" + username + "@" + domain
}

// convertActivityPubURLToHTML converts an ActivityPub object URI to an HTML post URL
// For Stegodon servers: https://domain/notes/{uuid} -> https://domain/u/{username}/{uuid}
// For other servers: keeps the original URL
func convertActivityPubURLToHTML(objectURI string, actorURI string) string {
	// Check if this is a Stegodon notes URL
	if strings.Contains(objectURI, "/notes/") {
		// Extract username from actor URI
		username := extractUsernameFromActorURI(actorURI)
		if username == "" {
			return objectURI // Fallback to original if can't extract username
		}

		// Extract UUID from object URI
		parts := strings.Split(objectURI, "/notes/")
		if len(parts) != 2 {
			return objectURI
		}
		uuidPart := parts[1]
		// Remove any trailing path segments
		if idx := strings.Index(uuidPart, "/"); idx > 0 {
			uuidPart = uuidPart[:idx]
		}

		// Extract domain from actor URI
		domain := extractDomainFromActorURI(actorURI)
		if domain == "" {
			return objectURI
		}

		// Construct HTML URL
		return fmt.Sprintf("https://%s/u/%s/%s", domain, username, uuidPart)
	}

	// For non-Stegodon servers, return original URL
	return objectURI
}

// extractUsernameFromActorURI extracts just the username from an actor URI
func extractUsernameFromActorURI(actorURI string) string {
	// Remove protocol prefix
	uri := strings.TrimPrefix(actorURI, "https://")
	uri = strings.TrimPrefix(uri, "http://")

	// Split into domain and path
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	path := parts[1]

	// Extract username from path
	var username string
	if strings.HasPrefix(path, "users/") {
		username = strings.TrimPrefix(path, "users/")
	} else if strings.HasPrefix(path, "@") {
		username = strings.TrimPrefix(path, "@")
	} else if strings.HasPrefix(path, "u/") {
		username = strings.TrimPrefix(path, "u/")
	} else {
		// Just use last path segment as username
		pathParts := strings.Split(path, "/")
		username = pathParts[len(pathParts)-1]
	}

	// Remove any trailing path segments from username
	if idx := strings.Index(username, "/"); idx > 0 {
		username = username[:idx]
	}

	return username
}

// extractDomainFromActorURI extracts just the domain from an actor URI
func extractDomainFromActorURI(actorURI string) string {
	// Remove protocol prefix
	uri := strings.TrimPrefix(actorURI, "https://")
	uri = strings.TrimPrefix(uri, "http://")

	// Split into domain and path
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) < 1 {
		return ""
	}

	return parts[0]
}

// ReadGlobalTimelinePosts returns posts for the global timeline (local notes + remote activities)
// excluding replies. Uses UNION ALL for efficient SQL-level sorting and pagination.
func (db *DB) ReadGlobalTimelinePosts(limit, offset int) (error, *[]domain.GlobalTimelinePost) {
	// Use UNION ALL to combine local, remote, and boosted posts with SQL-level sorting and pagination.
	// Each branch has its own ORDER BY/LIMIT to avoid full table scans — per-branch limit is
	// (limit + offset) which guarantees identical results to an unlimited UNION ALL.
	branchLimit := limit + offset
	rows, err := db.db.Query(`
		SELECT
			id, username, user_domain, profile_url, object_uri, object_url,
			is_remote, message, created_at, reply_count, like_count, boost_count, boosted_by
		FROM (
			-- Local posts (excluding replies)
			SELECT * FROM (
				SELECT
					n.id as id,
					a.username as username,
					'' as user_domain,
					'/u/' || a.username as profile_url,
					COALESCE(n.object_uri, '') as object_uri,
					'' as object_url,
					0 as is_remote,
					n.message as message,
					n.created_at as created_at,
					COALESCE(n.reply_count, 0) as reply_count,
					COALESCE(n.like_count, 0) as like_count,
					COALESCE(n.boost_count, 0) as boost_count,
					'' as boosted_by
				FROM notes n
				INNER JOIN accounts a ON a.id = n.user_id
				WHERE (n.in_reply_to_uri IS NULL OR n.in_reply_to_uri = '')
				ORDER BY n.created_at DESC LIMIT ?
			)

			UNION ALL

			-- Remote posts (excluding replies, using indexed in_reply_to column)
			SELECT * FROM (
				SELECT
					act.id as id,
					ra.username as username,
					ra.domain as user_domain,
					ra.actor_uri as profile_url,
					COALESCE(act.object_uri, '') as object_uri,
					COALESCE(act.object_url, '') as object_url,
					1 as is_remote,
					act.raw_json as message,
					act.created_at as created_at,
					COALESCE(act.reply_count, 0) as reply_count,
					COALESCE(act.like_count, 0) as like_count,
					COALESCE(act.boost_count, 0) as boost_count,
					'' as boosted_by
				FROM activities act
				INNER JOIN remote_accounts ra ON ra.actor_uri = act.actor_uri
				WHERE act.activity_type = 'Create'
				AND act.local = 0
				AND (act.in_reply_to IS NULL OR act.in_reply_to = '')
				ORDER BY act.created_at DESC LIMIT ?
			)

			UNION ALL

			-- Boosted local posts (show who boosted them)
			SELECT * FROM (
				SELECT
					n.id as id,
					a.username as username,
					'' as user_domain,
					'/u/' || a.username as profile_url,
					COALESCE(n.object_uri, '') as object_uri,
					'' as object_url,
					0 as is_remote,
					n.message as message,
					b.created_at as created_at,
					COALESCE(n.reply_count, 0) as reply_count,
					COALESCE(n.like_count, 0) as like_count,
					COALESCE(n.boost_count, 0) as boost_count,
					'@' || booster.username as boosted_by
				FROM boosts b
				INNER JOIN accounts booster ON booster.id = b.account_id
				INNER JOIN notes n ON n.id = b.note_id
				INNER JOIN accounts a ON a.id = n.user_id
				WHERE b.account_id != n.user_id
				ORDER BY b.created_at DESC LIMIT ?
			)

			UNION ALL

			-- Boosted remote posts (show who boosted them - local boosters)
			SELECT * FROM (
				SELECT
					act.id as id,
					ra.username as username,
					ra.domain as user_domain,
					ra.actor_uri as profile_url,
					COALESCE(act.object_uri, '') as object_uri,
					COALESCE(act.object_url, '') as object_url,
					1 as is_remote,
					act.raw_json as message,
					b.created_at as created_at,
					COALESCE(act.reply_count, 0) as reply_count,
					COALESCE(act.like_count, 0) as like_count,
					COALESCE(act.boost_count, 0) as boost_count,
					'@' || booster.username as boosted_by
				FROM boosts b
				INNER JOIN accounts booster ON booster.id = b.account_id
				INNER JOIN activities act ON act.object_uri = b.object_uri
				INNER JOIN remote_accounts ra ON ra.actor_uri = act.actor_uri
				WHERE b.object_uri IS NOT NULL AND b.object_uri != ''
				AND (b.account_id IS NOT NULL AND b.account_id != '')
				ORDER BY b.created_at DESC LIMIT ?
			)

			UNION ALL

			-- Boosted remote posts (show who boosted them - remote boosters)
			SELECT * FROM (
				SELECT
					act.id as id,
					ra_author.username as username,
					ra_author.domain as user_domain,
					ra_author.actor_uri as profile_url,
					COALESCE(act.object_uri, '') as object_uri,
					COALESCE(act.object_url, '') as object_url,
					1 as is_remote,
					act.raw_json as message,
					b.created_at as created_at,
					COALESCE(act.reply_count, 0) as reply_count,
					COALESCE(act.like_count, 0) as like_count,
					COALESCE(act.boost_count, 0) as boost_count,
					'@' || ra_booster.username || '@' || ra_booster.domain as boosted_by
				FROM boosts b
				INNER JOIN remote_accounts ra_booster ON ra_booster.id = b.remote_account_id
				INNER JOIN activities act ON act.object_uri = b.object_uri
				INNER JOIN remote_accounts ra_author ON ra_author.actor_uri = act.actor_uri
				WHERE b.remote_account_id IS NOT NULL AND b.remote_account_id != ''
				AND b.object_uri IS NOT NULL AND b.object_uri != ''
				ORDER BY b.created_at DESC LIMIT ?
			)
		) combined
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, branchLimit, branchLimit, branchLimit, branchLimit, branchLimit, limit, offset)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var posts []domain.GlobalTimelinePost
	for rows.Next() {
		var post domain.GlobalTimelinePost
		var createdAtStr, message string
		var isRemoteInt int
		err := rows.Scan(
			&post.NoteId, &post.Username, &post.UserDomain, &post.ProfileURL,
			&post.ObjectURI, &post.ObjectURL, &isRemoteInt, &message, &createdAtStr,
			&post.ReplyCount, &post.LikeCount, &post.BoostCount, &post.BoostedBy,
		)
		if err != nil {
			return err, &posts
		}
		post.IsRemote = isRemoteInt == 1
		post.CreatedAt, _ = parseTimestamp(createdAtStr)

		if post.IsRemote {
			// For remote posts, extract content from raw JSON
			post.Message = extractContentFromJSON(message)
			// Format username as @user@domain for display
			post.Username = fmt.Sprintf("@%s@%s", post.Username, post.UserDomain)
			// Convert ActivityPub object URI to HTML URL for display
			if post.ObjectURL == "" {
				post.ObjectURL = convertActivityPubURLToHTML(post.ObjectURI, post.ProfileURL)
			}
			// If content is empty but we have a URL, show the URL as content
			// This handles posts that are just links (URL-only posts)
			if post.Message == "" && post.ObjectURL != "" {
				post.Message = post.ObjectURL
			}
		} else {
			post.Message = message
		}
		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		return err, &posts
	}

	// Deduplicate posts - prefer boosted version over non-boosted (original)
	// This ensures that when you boost a post, it shows with the boost indicator
	seen := make(map[string]int) // maps NoteId to index in posts slice
	var dedupedPosts []domain.GlobalTimelinePost
	for _, post := range posts {
		if existingIdx, exists := seen[post.NoteId]; exists {
			// If the new post is boosted and existing is not, replace with boosted version
			if post.BoostedBy != "" && dedupedPosts[existingIdx].BoostedBy == "" {
				dedupedPosts[existingIdx] = post
			}
			// Otherwise keep the existing one (first boosted version wins)
		} else {
			seen[post.NoteId] = len(dedupedPosts)
			dedupedPosts = append(dedupedPosts, post)
		}
	}

	return nil, &dedupedPosts
}

// CountGlobalTimelinePosts returns the total count of posts in the global timeline
func (db *DB) CountGlobalTimelinePosts() (int, error) {
	var count int

	// Count both local and remote posts (excluding replies) in a single query
	err := db.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM notes WHERE (in_reply_to_uri IS NULL OR in_reply_to_uri = ''))
			+
			(SELECT COUNT(*) FROM activities WHERE activity_type = 'Create' AND local = 0 AND (in_reply_to IS NULL OR in_reply_to = ''))
	`).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
