package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
)

// SQL for new ActivityPub tables
const (
	// Follow relationships table
	sqlCreateFollowsTable = `CREATE TABLE IF NOT EXISTS follows (
		id TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		target_account_id TEXT NOT NULL,
		uri TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accepted INTEGER DEFAULT 0
	)`

	sqlCreateFollowsIndices = `
		CREATE INDEX IF NOT EXISTS idx_follows_account_id ON follows(account_id);
		CREATE INDEX IF NOT EXISTS idx_follows_target_account_id ON follows(target_account_id);
		CREATE INDEX IF NOT EXISTS idx_follows_uri ON follows(uri);
	`

	// Remote accounts cache table
	sqlCreateRemoteAccountsTable = `CREATE TABLE IF NOT EXISTS remote_accounts (
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
	)`

	sqlCreateRemoteAccountsIndices = `
		CREATE INDEX IF NOT EXISTS idx_remote_accounts_actor_uri ON remote_accounts(actor_uri);
		CREATE INDEX IF NOT EXISTS idx_remote_accounts_domain ON remote_accounts(domain);
	`

	// Activities log table (for deduplication & debugging)
	sqlCreateActivitiesTable = `CREATE TABLE IF NOT EXISTS activities (
		id TEXT NOT NULL PRIMARY KEY,
		activity_uri TEXT UNIQUE NOT NULL,
		activity_type TEXT NOT NULL,
		actor_uri TEXT NOT NULL,
		object_uri TEXT,
		object_url TEXT,
		in_reply_to TEXT,
		raw_json TEXT NOT NULL,
		processed INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		local INTEGER DEFAULT 0
	)`

	sqlCreateActivitiesIndices = `
		CREATE INDEX IF NOT EXISTS idx_activities_uri ON activities(activity_uri);
		CREATE INDEX IF NOT EXISTS idx_activities_processed ON activities(processed);
		CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(activity_type);
		CREATE INDEX IF NOT EXISTS idx_activities_created_at ON activities(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_activities_object_uri ON activities(object_uri);
		CREATE INDEX IF NOT EXISTS idx_activities_from_relay ON activities(from_relay);
		CREATE INDEX IF NOT EXISTS idx_activities_in_reply_to ON activities(in_reply_to);
	`

	// Likes/favorites table
	sqlCreateLikesTable = `CREATE TABLE IF NOT EXISTS likes (
		id TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		note_id TEXT NOT NULL,
		uri TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(account_id, note_id)
	)`

	sqlCreateLikesIndices = `
		CREATE INDEX IF NOT EXISTS idx_likes_note_id ON likes(note_id);
		CREATE INDEX IF NOT EXISTS idx_likes_account_id ON likes(account_id);
		CREATE INDEX IF NOT EXISTS idx_likes_object_uri ON likes(object_uri);
	`

	// Boosts/announces table
	sqlCreateBoostsTable = `CREATE TABLE IF NOT EXISTS boosts (
		id TEXT NOT NULL PRIMARY KEY,
		account_id TEXT,
		remote_account_id TEXT,
		note_id TEXT NOT NULL,
		object_uri TEXT,
		uri TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(account_id, note_id)
	)`

	sqlCreateBoostsIndices = `
		CREATE INDEX IF NOT EXISTS idx_boosts_note_id ON boosts(note_id);
		CREATE INDEX IF NOT EXISTS idx_boosts_account_id ON boosts(account_id);
		CREATE INDEX IF NOT EXISTS idx_boosts_object_uri ON boosts(object_uri);
		CREATE INDEX IF NOT EXISTS idx_boosts_remote_account_id ON boosts(remote_account_id);
		CREATE INDEX IF NOT EXISTS idx_boosts_remote_created ON boosts(remote_account_id, created_at DESC);
	`

	// Delivery queue table
	sqlCreateDeliveryQueueTable = `CREATE TABLE IF NOT EXISTS delivery_queue (
		id TEXT NOT NULL PRIMARY KEY,
		inbox_uri TEXT NOT NULL,
		activity_json TEXT NOT NULL,
		attempts INTEGER DEFAULT 0,
		next_retry_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	sqlCreateDeliveryQueueIndices = `
		CREATE INDEX IF NOT EXISTS idx_delivery_queue_next_retry ON delivery_queue(next_retry_at);
	`

	// Hashtags table
	sqlCreateHashtagsTable = `CREATE TABLE IF NOT EXISTS hashtags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		usage_count INTEGER DEFAULT 0,
		last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	sqlCreateHashtagsIndices = `
		CREATE INDEX IF NOT EXISTS idx_hashtags_name ON hashtags(name);
		CREATE INDEX IF NOT EXISTS idx_hashtags_usage ON hashtags(usage_count DESC);
	`

	// Note-hashtag relationship table
	sqlCreateNoteHashtagsTable = `CREATE TABLE IF NOT EXISTS note_hashtags (
		note_id TEXT NOT NULL,
		hashtag_id INTEGER NOT NULL,
		PRIMARY KEY (note_id, hashtag_id),
		FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
		FOREIGN KEY (hashtag_id) REFERENCES hashtags(id) ON DELETE CASCADE
	)`

	sqlCreateNoteHashtagsIndices = `
		CREATE INDEX IF NOT EXISTS idx_note_hashtags_note_id ON note_hashtags(note_id);
		CREATE INDEX IF NOT EXISTS idx_note_hashtags_hashtag_id ON note_hashtags(hashtag_id);
	`

	// Note-mention relationship table (stores @user@domain mentions in notes)
	sqlCreateNoteMentionsTable = `CREATE TABLE IF NOT EXISTS note_mentions (
		id TEXT PRIMARY KEY,
		note_id TEXT NOT NULL,
		mentioned_actor_uri TEXT NOT NULL,
		mentioned_username TEXT NOT NULL,
		mentioned_domain TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
	)`

	sqlCreateNoteMentionsIndices = `
		CREATE INDEX IF NOT EXISTS idx_note_mentions_note_id ON note_mentions(note_id);
		CREATE INDEX IF NOT EXISTS idx_note_mentions_actor_uri ON note_mentions(mentioned_actor_uri);
	`

	// Relays table for ActivityPub relay subscriptions
	sqlCreateRelaysTable = `CREATE TABLE IF NOT EXISTS relays (
		id TEXT NOT NULL PRIMARY KEY,
		actor_uri TEXT UNIQUE NOT NULL,
		inbox_uri TEXT NOT NULL,
		follow_uri TEXT,
		name TEXT,
		status TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accepted_at TIMESTAMP
	)`

	sqlCreateRelaysIndices = `
		CREATE INDEX IF NOT EXISTS idx_relays_status ON relays(status);
	`

	// Notifications table for user notifications
	sqlCreateNotificationsTable = `CREATE TABLE IF NOT EXISTS notifications (
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
	)`

	sqlCreateNotificationsIndices = `
		CREATE INDEX IF NOT EXISTS idx_notifications_account_id ON notifications(account_id);
		CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_notifications_account_read ON notifications(account_id, read);
	`

	// InfoBoxes table for customizable website information boxes
	sqlCreateInfoBoxesTable = `CREATE TABLE IF NOT EXISTS info_boxes (
		id TEXT NOT NULL PRIMARY KEY,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		order_num INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	sqlCreateInfoBoxesIndices = `
		CREATE INDEX IF NOT EXISTS idx_info_boxes_order ON info_boxes(order_num);
		CREATE INDEX IF NOT EXISTS idx_info_boxes_enabled ON info_boxes(enabled);
	`

	// Upload tokens table for one-time upload links (avatar, etc.)
	sqlCreateUploadTokensTable = `CREATE TABLE IF NOT EXISTS upload_tokens (
		token TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		token_type TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
	)`

	sqlCreateUploadTokensIndices = `
		CREATE INDEX IF NOT EXISTS idx_upload_tokens_account_id ON upload_tokens(account_id);
		CREATE INDEX IF NOT EXISTS idx_upload_tokens_expires_at ON upload_tokens(expires_at);
	`

	// Server message table - single row for admin announcements
	sqlCreateServerMessageTable = `CREATE TABLE IF NOT EXISTS server_message (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		message TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 0,
		web_enabled INTEGER NOT NULL DEFAULT 1,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	// Bans table for storing banned users' IP addresses and public keys
	sqlCreateBansTable = `CREATE TABLE IF NOT EXISTS bans (
		id TEXT NOT NULL PRIMARY KEY,
		username TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		public_key_hash TEXT NOT NULL,
		reason TEXT NOT NULL DEFAULT 'Banned by administrator',
		banned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	sqlCreateBansIndices = `
		CREATE INDEX IF NOT EXISTS idx_bans_ip_address ON bans(ip_address);
		CREATE INDEX IF NOT EXISTS idx_bans_public_key_hash ON bans(public_key_hash);
	`

	// Extend existing tables with new columns
	sqlExtendAccountsTable = `
		ALTER TABLE accounts ADD COLUMN display_name TEXT;
		ALTER TABLE accounts ADD COLUMN summary TEXT;
		ALTER TABLE accounts ADD COLUMN avatar_url TEXT;
	`

	sqlExtendNotesTable = `
		ALTER TABLE notes ADD COLUMN visibility TEXT DEFAULT 'public';
		ALTER TABLE notes ADD COLUMN in_reply_to_uri TEXT;
		ALTER TABLE notes ADD COLUMN object_uri TEXT;
		ALTER TABLE notes ADD COLUMN federated INTEGER DEFAULT 1;
		ALTER TABLE notes ADD COLUMN sensitive INTEGER DEFAULT 0;
		ALTER TABLE notes ADD COLUMN content_warning TEXT;
	`

	sqlCreateNotesIndices = `
		CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes(user_id);
		CREATE INDEX IF NOT EXISTS idx_notes_created_at ON notes(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_notes_object_uri ON notes(object_uri);
		CREATE INDEX IF NOT EXISTS idx_notes_in_reply_to_uri ON notes(in_reply_to_uri);
	`

	sqlExtendServerMessageTable = `
		ALTER TABLE server_message ADD COLUMN web_enabled INTEGER NOT NULL DEFAULT 1;
	`
)

// RunMigrations executes all database migrations
func (db *DB) RunMigrations() error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Create new tables
		if err := db.createTableIfNotExists(tx, sqlCreateFollowsTable, "follows"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateRemoteAccountsTable, "remote_accounts"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateActivitiesTable, "activities"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateLikesTable, "likes"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateBoostsTable, "boosts"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateDeliveryQueueTable, "delivery_queue"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateHashtagsTable, "hashtags"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateNoteHashtagsTable, "note_hashtags"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateNoteMentionsTable, "note_mentions"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateRelaysTable, "relays"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateNotificationsTable, "notifications"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateInfoBoxesTable, "info_boxes"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateUploadTokensTable, "upload_tokens"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateServerMessageTable, "server_message"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateBansTable, "bans"); err != nil {
			return err
		}

		// Create indices
		if _, err := tx.Exec(sqlCreateFollowsIndices); err != nil {
			log.Printf("Warning: Failed to create follows indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateRemoteAccountsIndices); err != nil {
			log.Printf("Warning: Failed to create remote_accounts indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateActivitiesIndices); err != nil {
			log.Printf("Warning: Failed to create activities indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateLikesIndices); err != nil {
			log.Printf("Warning: Failed to create likes indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateBoostsIndices); err != nil {
			log.Printf("Warning: Failed to create boosts indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateDeliveryQueueIndices); err != nil {
			log.Printf("Warning: Failed to create delivery_queue indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateHashtagsIndices); err != nil {
			log.Printf("Warning: Failed to create hashtags indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNoteHashtagsIndices); err != nil {
			log.Printf("Warning: Failed to create note_hashtags indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNoteMentionsIndices); err != nil {
			log.Printf("Warning: Failed to create note_mentions indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateRelaysIndices); err != nil {
			log.Printf("Warning: Failed to create relays indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNotificationsIndices); err != nil {
			log.Printf("Warning: Failed to create notifications indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateInfoBoxesIndices); err != nil {
			log.Printf("Warning: Failed to create info_boxes indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateUploadTokensIndices); err != nil {
			log.Printf("Warning: Failed to create upload_tokens indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateBansIndices); err != nil {
			log.Printf("Warning: Failed to create bans indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNotesIndices); err != nil {
			log.Printf("Warning: Failed to create notes indices: %v", err)
		}

		// Extend existing tables (ignore errors if columns already exist)
		db.extendExistingTables(tx)

		// Backfill object_uri for existing activities
		if err := db.backfillActivityObjectURIs(tx); err != nil {
			log.Printf("Warning: Failed to backfill activity object_uri: %v", err)
		}

		// Add username uniqueness constraint (handles duplicates gracefully)
		if err := db.addUsernameUniqueConstraint(tx); err != nil {
			log.Printf("Warning: Failed to add username unique constraint: %v", err)
		}

		// Backfill reply counts for existing notes and activities
		if err := db.backfillReplyCounts(tx); err != nil {
			log.Printf("Warning: Failed to backfill reply counts: %v", err)
		}

		// Fix orphaned Update activities (convert to Create so they show in timeline)
		if err := db.fixOrphanedUpdateActivities(tx); err != nil {
			log.Printf("Warning: Failed to fix orphaned Update activities: %v", err)
		}

		// Seed default info boxes if none exist
		if err := db.seedDefaultInfoBoxes(tx); err != nil {
			log.Printf("Warning: Failed to seed default info boxes: %v", err)
		}

		return nil
	})
}

func (db *DB) createTableIfNotExists(tx *sql.Tx, createSQL string, tableName string) error {
	_, err := tx.Exec(createSQL)
	if err != nil {
		log.Printf("Error creating table %s: %v", tableName, err)
		return err
	}
	log.Printf("Table %s created or already exists", tableName)
	return nil
}

func (db *DB) extendExistingTables(tx *sql.Tx) {
	// Try to add columns to accounts table (ignore errors if they exist)
	tx.Exec("ALTER TABLE accounts ADD COLUMN display_name TEXT")
	tx.Exec("ALTER TABLE accounts ADD COLUMN summary TEXT")
	tx.Exec("ALTER TABLE accounts ADD COLUMN avatar_url TEXT")
	tx.Exec("ALTER TABLE accounts ADD COLUMN is_admin INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE accounts ADD COLUMN muted INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE accounts ADD COLUMN banned INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE accounts ADD COLUMN last_ip TEXT")

	// Try to add columns to notes table (ignore errors if they exist)
	tx.Exec("ALTER TABLE notes ADD COLUMN visibility TEXT DEFAULT 'public'")
	tx.Exec("ALTER TABLE notes ADD COLUMN in_reply_to_uri TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN object_uri TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN federated INTEGER DEFAULT 1")
	tx.Exec("ALTER TABLE notes ADD COLUMN sensitive INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE notes ADD COLUMN content_warning TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN edited_at TIMESTAMP")

	// Engagement count columns for notes (denormalized for performance)
	tx.Exec("ALTER TABLE notes ADD COLUMN reply_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE notes ADD COLUMN like_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE notes ADD COLUMN boost_count INTEGER DEFAULT 0")

	// Engagement count columns for activities (remote posts)
	tx.Exec("ALTER TABLE activities ADD COLUMN reply_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE activities ADD COLUMN like_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE activities ADD COLUMN boost_count INTEGER DEFAULT 0")

	// Add is_local column to follows table to support local follows
	tx.Exec("ALTER TABLE follows ADD COLUMN is_local INTEGER DEFAULT 0")

	// Add account_id column to delivery_queue table to support account-based cleanup
	tx.Exec("ALTER TABLE delivery_queue ADD COLUMN account_id TEXT")

	// Add object_uri column to likes table for remote post likes
	tx.Exec("ALTER TABLE likes ADD COLUMN object_uri TEXT")

	// Add unique index for remote post likes (account_id + object_uri)
	// This allows one like per account per remote post (identified by object_uri)
	tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_likes_account_object_uri ON likes(account_id, object_uri) WHERE object_uri IS NOT NULL AND object_uri != ''")

	// Add object_uri column to boosts table for remote post boosts
	tx.Exec("ALTER TABLE boosts ADD COLUMN object_uri TEXT")

	// Add unique index for remote post boosts (account_id + object_uri)
	tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_boosts_account_object_uri ON boosts(account_id, object_uri) WHERE object_uri IS NOT NULL AND object_uri != ''")

	// Add follow_uri column to relays table for proper Undo Follow
	tx.Exec("ALTER TABLE relays ADD COLUMN follow_uri TEXT")

	// Add paused column to relays table for pause/resume functionality
	tx.Exec("ALTER TABLE relays ADD COLUMN paused INTEGER DEFAULT 0")

	// Add from_relay column to activities table to track relay-forwarded content
	tx.Exec("ALTER TABLE activities ADD COLUMN from_relay INTEGER DEFAULT 0")

	// Add web_enabled column to server_message table for separate web UI toggle
	tx.Exec("ALTER TABLE server_message ADD COLUMN web_enabled INTEGER NOT NULL DEFAULT 1")

	// Add remote_account_id column to boosts table for tracking boosts from remote followed users
	tx.Exec("ALTER TABLE boosts ADD COLUMN remote_account_id TEXT")
	tx.Exec("CREATE INDEX IF NOT EXISTS idx_boosts_remote_account_id ON boosts(remote_account_id)")
	tx.Exec("CREATE INDEX IF NOT EXISTS idx_boosts_remote_created ON boosts(remote_account_id, created_at DESC)")

	log.Println("Extended existing tables with new columns")
}

// backfillActivityObjectURIs extracts object_uri from raw_json for activities that are missing it
func (db *DB) backfillActivityObjectURIs(tx *sql.Tx) error {
	// Find activities with empty object_uri
	rows, err := tx.Query(`SELECT id, raw_json FROM activities WHERE object_uri IS NULL OR object_uri = ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id, rawJSON string
		if err := rows.Scan(&id, &rawJSON); err != nil {
			log.Printf("Warning: Failed to scan activity: %v", err)
			continue
		}

		// Parse the raw JSON to extract object ID
		var activity struct {
			Object any `json:"object"`
		}
		if err := json.Unmarshal([]byte(rawJSON), &activity); err != nil {
			log.Printf("Warning: Failed to parse activity JSON for ID %s: %v", id, err)
			continue
		}

		// Extract object URI
		var objectURI string
		if activity.Object != nil {
			switch obj := activity.Object.(type) {
			case string:
				objectURI = obj
			case map[string]any:
				if idVal, ok := obj["id"].(string); ok {
					objectURI = idVal
				}
			}
		}

		// Update the activity if we found an object URI
		if objectURI != "" {
			_, err := tx.Exec(`UPDATE activities SET object_uri = ? WHERE id = ?`, objectURI, id)
			if err != nil {
				log.Printf("Warning: Failed to update activity %s: %v", id, err)
			} else {
				updated++
			}
		}
	}

	if updated > 0 {
		log.Printf("Backfilled object_uri for %d activities", updated)
	}

	return nil
}

// addUsernameUniqueConstraint renames duplicate usernames and adds UNIQUE constraint
func (db *DB) addUsernameUniqueConstraint(tx *sql.Tx) error {
	// Find duplicate usernames (case-insensitive)
	rows, err := tx.Query(`
		SELECT username, COUNT(*) as count
		FROM accounts
		GROUP BY LOWER(username)
		HAVING count > 1
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect duplicate usernames
	var duplicates []string
	for rows.Next() {
		var username string
		var count int
		if err := rows.Scan(&username, &count); err != nil {
			log.Printf("Warning: Failed to scan duplicate username: %v", err)
			continue
		}
		duplicates = append(duplicates, username)
	}

	// Process each duplicate username
	for _, username := range duplicates {
		// Get all accounts with this username, ordered by creation time
		accountRows, err := tx.Query(`
			SELECT id, username, created_at
			FROM accounts
			WHERE LOWER(username) = LOWER(?)
			ORDER BY created_at ASC
		`, username)
		if err != nil {
			log.Printf("Warning: Failed to query accounts for username '%s': %v", username, err)
			continue
		}

		var accounts []struct {
			id        string
			username  string
			createdAt string
		}

		for accountRows.Next() {
			var acc struct {
				id        string
				username  string
				createdAt string
			}
			if err := accountRows.Scan(&acc.id, &acc.username, &acc.createdAt); err != nil {
				log.Printf("Warning: Failed to scan account: %v", err)
				continue
			}
			accounts = append(accounts, acc)
		}
		accountRows.Close()

		// Keep the first (oldest) account, rename the rest
		for i := 1; i < len(accounts); i++ {
			newUsername := accounts[i].username + "_" + fmt.Sprintf("%d", i+1)

			// Ensure new username doesn't exceed any length limits and is valid
			if len(newUsername) > 50 {
				newUsername = accounts[i].username[:45] + "_" + fmt.Sprintf("%d", i+1)
			}

			_, err := tx.Exec(`UPDATE accounts SET username = ? WHERE id = ?`, newUsername, accounts[i].id)
			if err != nil {
				log.Printf("Warning: Failed to rename duplicate username '%s' (id: %s) to '%s': %v",
					accounts[i].username, accounts[i].id, newUsername, err)
			} else {
				log.Printf("Renamed duplicate username '%s' (id: %s, created: %s) to '%s'",
					accounts[i].username, accounts[i].id, accounts[i].createdAt, newUsername)
			}
		}
	}

	// Add UNIQUE constraint (case-insensitive)
	_, err = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_username ON accounts(username COLLATE NOCASE)`)
	if err != nil {
		return fmt.Errorf("failed to create unique index on username: %v", err)
	}

	log.Println("Added UNIQUE constraint to accounts.username column")
	return nil
}

// backfillReplyCounts recalculates reply_count for all notes and activities
// This runs once during migration to populate the denormalized counts
// It uses recursive counting to get the total of all nested replies
func (db *DB) backfillReplyCounts(tx *sql.Tx) error {
	// Check if we've already backfilled (if any reply_count > 0, skip)
	var hasData int
	err := tx.QueryRow(`SELECT COUNT(*) FROM notes WHERE reply_count > 0`).Scan(&hasData)
	if err == nil && hasData > 0 {
		log.Println("Reply counts already backfilled, skipping")
		return nil
	}

	log.Println("Backfilling reply counts for notes and activities (recursive)...")

	// Reset all counts to 0
	tx.Exec(`UPDATE notes SET reply_count = 0`)
	tx.Exec(`UPDATE activities SET reply_count = 0`)

	// Get all notes with in_reply_to_uri (these are replies)
	rows, err := tx.Query(`SELECT in_reply_to_uri FROM notes WHERE in_reply_to_uri IS NOT NULL AND in_reply_to_uri != ''`)
	if err != nil {
		log.Printf("Warning: Failed to query note replies: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var inReplyTo string
			if err := rows.Scan(&inReplyTo); err == nil && inReplyTo != "" {
				// Increment all ancestors
				db.incrementReplyCountRecursive(tx, inReplyTo, make(map[string]bool))
			}
		}
	}

	// Get all Create activities with inReplyTo (remote replies)
	// Skip activities that are duplicates of local notes:
	// 1. Same object_uri exists in notes table, OR
	// 2. Activity object_uri contains a local note ID pattern (/notes/{uuid})
	rows2, err := tx.Query(`
		SELECT a.object_uri, a.raw_json
		FROM activities a
		WHERE a.activity_type = 'Create'
		AND a.raw_json LIKE '%"inReplyTo":"http%'
		AND NOT EXISTS (
			SELECT 1 FROM notes n
			WHERE (n.object_uri = a.object_uri AND n.object_uri IS NOT NULL AND n.object_uri != '')
			   OR (a.object_uri LIKE '%/notes/' || n.id || '%')
		)
	`)
	if err != nil {
		log.Printf("Warning: Failed to query activity replies: %v", err)
	} else {
		defer rows2.Close()
		for rows2.Next() {
			var objectURI, rawJSON string
			if err := rows2.Scan(&objectURI, &rawJSON); err == nil {
				inReplyTo := extractInReplyToFromJSON(rawJSON)
				if inReplyTo != "" {
					// Increment all ancestors
					db.incrementReplyCountRecursive(tx, inReplyTo, make(map[string]bool))
				}
			}
		}
	}

	log.Println("Completed backfilling reply counts")
	return nil
}

// fixOrphanedUpdateActivities converts Update activities that have no corresponding Create
// to Create activities so they show up in the timeline.
// This happens when we followed a user after their original post, and only received the Update.
func (db *DB) fixOrphanedUpdateActivities(tx *sql.Tx) error {
	// Find Update activities for Notes where we don't have a Create activity for the same object_uri
	// Group by object_uri and pick the first Update (oldest) to convert to Create
	rows, err := tx.Query(`
		SELECT u.id, u.activity_uri, u.actor_uri, u.object_uri, u.raw_json, u.created_at
		FROM activities u
		WHERE u.activity_type = 'Update'
		AND u.object_uri IS NOT NULL
		AND u.object_uri != ''
		AND NOT EXISTS (
			SELECT 1 FROM activities c
			WHERE c.object_uri = u.object_uri
			AND c.activity_type = 'Create'
		)
		AND u.id = (
			SELECT MIN(u2.id) FROM activities u2
			WHERE u2.object_uri = u.object_uri
			AND u2.activity_type = 'Update'
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to query orphaned Update activities: %w", err)
	}
	defer rows.Close()

	converted := 0
	for rows.Next() {
		var id, activityURI, actorURI, objectURI, rawJSON, createdAt string
		if err := rows.Scan(&id, &activityURI, &actorURI, &objectURI, &rawJSON, &createdAt); err != nil {
			log.Printf("Warning: Failed to scan Update activity: %v", err)
			continue
		}

		// Update the activity_type to 'Create' so it shows in timeline
		_, err := tx.Exec(`UPDATE activities SET activity_type = 'Create' WHERE id = ?`, id)
		if err != nil {
			log.Printf("Warning: Failed to convert Update %s to Create: %v", id, err)
		} else {
			converted++
			log.Printf("Converted orphaned Update to Create: %s (object: %s)", activityURI, objectURI)
		}
	}

	if converted > 0 {
		log.Printf("Converted %d orphaned Update activities to Create", converted)
	}

	return nil
}

// MigratePerformanceIndexes adds performance-critical indexes that were missing
// These indexes speed up threading queries and relay content filtering
func (db *DB) MigratePerformanceIndexes() error {
	log.Println("Checking for missing performance indexes...")

	// Add index on notes.in_reply_to_uri for faster threading queries
	_, err := db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notes_in_reply_to_uri ON notes(in_reply_to_uri)`)
	if err != nil {
		log.Printf("Warning: Failed to create idx_notes_in_reply_to_uri: %v", err)
	}

	// Add index on activities.object_uri for faster deduplication checks
	_, err = db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_object_uri ON activities(object_uri)`)
	if err != nil {
		log.Printf("Warning: Failed to create idx_activities_object_uri: %v", err)
	}

	// Add index on activities.from_relay for faster relay content filtering
	_, err = db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_from_relay ON activities(from_relay)`)
	if err != nil {
		log.Printf("Warning: Failed to create idx_activities_from_relay: %v", err)
	}

	// Add in_reply_to column to activities for faster reply queries (avoids LIKE on raw_json)
	_, err = db.db.Exec(`ALTER TABLE activities ADD COLUMN in_reply_to TEXT`)
	if err != nil {
		// Column likely already exists, which is fine
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("Note: in_reply_to column may already exist: %v", err)
		}
	} else {
		log.Println("Added in_reply_to column to activities table")
	}

	// Create index on in_reply_to for fast reply lookups
	_, err = db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_in_reply_to ON activities(in_reply_to)`)
	if err != nil {
		log.Printf("Warning: Failed to create idx_activities_in_reply_to: %v", err)
	}

	// Backfill in_reply_to from raw_json for existing activities
	db.backfillActivitiesInReplyTo()

	// Add object_url column to activities for human-readable web links (vs object_uri which is the AP id)
	_, err = db.db.Exec(`ALTER TABLE activities ADD COLUMN object_url TEXT`)
	if err != nil {
		// Column likely already exists, which is fine
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("Note: object_url column may already exist: %v", err)
		}
	} else {
		log.Println("Added object_url column to activities table")
	}

	// Backfill object_url from raw_json for existing activities
	db.backfillActivitiesObjectURL()

	// Backfill object_uri from raw_json for existing activities that are missing it.
	// This eliminates the need for the slow LIKE fallback in ReadActivityByObjectURI.
	db.backfillActivitiesObjectURI()

	log.Println("Performance indexes migration complete")
	return nil
}

// backfillActivitiesInReplyTo extracts inReplyTo from raw_json and populates the in_reply_to column
func (db *DB) backfillActivitiesInReplyTo() {
	// Only backfill rows where in_reply_to is NULL but raw_json contains inReplyTo
	rows, err := db.db.Query(`
		SELECT id, raw_json FROM activities
		WHERE in_reply_to IS NULL
		AND activity_type = 'Create'
		AND (raw_json LIKE '%"inReplyTo":%' OR raw_json LIKE '%"inReplyTo" :%')
	`)
	if err != nil {
		log.Printf("Warning: Failed to query activities for in_reply_to backfill: %v", err)
		return
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id, rawJSON string
		if err := rows.Scan(&id, &rawJSON); err != nil {
			continue
		}

		// Extract inReplyTo from JSON
		inReplyTo := extractInReplyToFromJSON(rawJSON)
		if inReplyTo != "" {
			_, err := db.db.Exec(`UPDATE activities SET in_reply_to = ? WHERE id = ?`, inReplyTo, id)
			if err == nil {
				updated++
			}
		}
	}

	if updated > 0 {
		log.Printf("Backfilled in_reply_to for %d activities", updated)
	}
}

// backfillActivitiesObjectURL extracts url from raw_json and populates the object_url column
func (db *DB) backfillActivitiesObjectURL() {
	// Only backfill rows where object_url is NULL but raw_json contains url
	rows, err := db.db.Query(`
		SELECT id, raw_json FROM activities
		WHERE object_url IS NULL
		AND activity_type = 'Create'
		AND raw_json LIKE '%"url":%'
	`)
	if err != nil {
		log.Printf("Warning: Failed to query activities for object_url backfill: %v", err)
		return
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id, rawJSON string
		if err := rows.Scan(&id, &rawJSON); err != nil {
			continue
		}

		// Extract url from JSON
		objectURL := extractObjectURLFromJSON(rawJSON)
		if objectURL != "" {
			_, err := db.db.Exec(`UPDATE activities SET object_url = ? WHERE id = ?`, objectURL, id)
			if err == nil {
				updated++
			}
		}
	}

	if updated > 0 {
		log.Printf("Backfilled object_url for %d activities", updated)
	}
}

// backfillActivitiesObjectURI extracts the object id from raw_json and populates the object_uri column.
// After this migration, ReadActivityByObjectURI's exact-match query always finds existing activities,
// making the slow LIKE fallback on raw_json unreachable.
func (db *DB) backfillActivitiesObjectURI() {
	// Only backfill rows where object_uri is NULL or empty but raw_json contains an id
	rows, err := db.db.Query(`
		SELECT id, raw_json FROM activities
		WHERE (object_uri IS NULL OR object_uri = '')
		AND activity_type = 'Create'
		AND raw_json IS NOT NULL AND raw_json != ''
	`)
	if err != nil {
		log.Printf("Warning: Failed to query activities for object_uri backfill: %v", err)
		return
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id, rawJSON string
		if err := rows.Scan(&id, &rawJSON); err != nil {
			continue
		}

		// Extract the object id from JSON (this is the object_uri)
		objectURI := extractObjectURIFromJSON(rawJSON)
		if objectURI != "" {
			_, err := db.db.Exec(`UPDATE activities SET object_uri = ? WHERE id = ?`, objectURI, id)
			if err == nil {
				updated++
			}
		}
	}

	if updated > 0 {
		log.Printf("Backfilled object_uri for %d activities", updated)
	}
}

// extractObjectURIFromJSON extracts the object.id field from ActivityPub JSON
func extractObjectURIFromJSON(rawJSON string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &data); err != nil {
		return ""
	}

	// The raw_json may contain the Note/Article directly (with top-level "id")
	// or may be wrapped in a Create activity (with object.id)
	if obj, ok := data["object"].(map[string]any); ok {
		if id, ok := obj["id"].(string); ok {
			return id
		}
	}

	// Top-level id (when raw_json is the Note itself, not the wrapping activity)
	if id, ok := data["id"].(string); ok {
		return id
	}

	return ""
}

// extractObjectURLFromJSON extracts the object.url field from ActivityPub JSON
func extractObjectURLFromJSON(rawJSON string) string {
	var activity map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &activity); err != nil {
		return ""
	}

	// Check if object is a map with a url field
	if obj, ok := activity["object"].(map[string]any); ok {
		if url, ok := obj["url"].(string); ok {
			return url
		}
	}
	return ""
}

// seedDefaultInfoBoxes creates default info boxes on first run
func (db *DB) seedDefaultInfoBoxes(tx *sql.Tx) error {
	// Check if any info boxes already exist
	var count int
	err := tx.QueryRow(`SELECT COUNT(*) FROM info_boxes`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing info boxes: %w", err)
	}

	if count > 0 {
		log.Println("Info boxes already exist, skipping seed")
		return nil
	}

	log.Println("Seeding default info boxes...")

	// Default info boxes based on current template
	defaultBoxes := []struct {
		title    string
		content  string
		orderNum int
	}{
		{
			title: "ssh-first fediverse blog",
			content: `Connect via SSH to start posting:

` + "```" + `
ssh -p {{SSH_PORT}} YourIpOrDomain
` + "```" + `

On first connection, you'll be prompted to choose a username. After that, you can create posts, follow users, and explore the federated timeline.

On federated services like Mastodon people now can follow you, when searching for:

` + "```" + `
@YourUser@YourSslDomain.com
` + "```" + ``,
			orderNum: 1,
		},
		{
			title: "features",
			content: `- Create and read posts
- Follow local and remote users
- ActivityPub federation
- [RSS feeds](/feed)`,
			orderNum: 2,
		},
		{
			title:    `<svg style="width: 1.2em; height: 1.2em; vertical-align: middle; margin-right: 8px;" viewBox="0 0 16 16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>github`,
			content:  `Source and Documentation available on [Github](https://github.com/deemkeen/stegodon)`,
			orderNum: 3,
		},
	}

	for _, box := range defaultBoxes {
		id := uuid.New()
		_, err := tx.Exec(`
			INSERT INTO info_boxes (id, title, content, order_num, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, id.String(), box.title, box.content, box.orderNum)
		if err != nil {
			return fmt.Errorf("failed to seed info box '%s': %w", box.title, err)
		}
	}

	log.Printf("Seeded %d default info boxes", len(defaultBoxes))
	return nil
}

// MigrateFTS5Search creates the FTS5 virtual table for full-text search and backfills existing data
func (db *DB) MigrateFTS5Search() error {
	log.Println("Checking FTS5 search index...")

	// Create the FTS5 virtual table (standalone, not external-content)
	_, err := db.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
		content,
		author,
		source_type UNINDEXED,
		created_at UNINDEXED,
		object_uri UNINDEXED,
		object_url UNINDEXED,
		tokenize='unicode61'
	)`)
	if err != nil {
		return fmt.Errorf("failed to create posts_fts table: %w", err)
	}

	// Lookup table maps source IDs to FTS rowids (needed for delete/update)
	_, err = db.db.Exec(`CREATE TABLE IF NOT EXISTS posts_fts_lookup(
		source_id TEXT PRIMARY KEY,
		fts_rowid INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("failed to create posts_fts_lookup table: %w", err)
	}

	// Index on fts_rowid for efficient JOIN in search queries
	db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_posts_fts_lookup_rowid ON posts_fts_lookup(fts_rowid)`)

	// Check if backfill is needed
	var count int
	err = db.db.QueryRow(`SELECT COUNT(*) FROM posts_fts`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check posts_fts count: %w", err)
	}
	if count > 0 {
		log.Printf("FTS5 index already populated (%d entries), skipping backfill", count)
		return nil
	}

	log.Println("Backfilling FTS5 search index...")

	// Backfill local notes
	notesBackfilled, err := db.backfillNotesFTS()
	if err != nil {
		log.Printf("Warning: Failed to backfill notes FTS: %v", err)
	}

	// Backfill remote activities
	activitiesBackfilled, err := db.backfillActivitiesFTS()
	if err != nil {
		log.Printf("Warning: Failed to backfill activities FTS: %v", err)
	}

	log.Printf("FTS5 backfill complete: %d notes, %d activities", notesBackfilled, activitiesBackfilled)
	return nil
}

// backfillNotesFTS inserts all existing local notes into the FTS index
func (db *DB) backfillNotesFTS() (int, error) {
	rows, err := db.db.Query(`
		SELECT n.id, a.username, n.message, n.created_at, COALESCE(n.object_uri, '')
		FROM notes n
		JOIN accounts a ON a.id = n.user_id
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	inserted := 0
	for rows.Next() {
		var id, username, message, createdAt, objectURI string
		if err := rows.Scan(&id, &username, &message, &createdAt, &objectURI); err != nil {
			log.Printf("Warning: Failed to scan note for FTS backfill: %v", err)
			continue
		}

		author := "@" + username
		result, err := db.db.Exec(
			`INSERT INTO posts_fts(content, author, source_type, created_at, object_uri, object_url) VALUES (?, ?, 'note', ?, ?, '')`,
			message, author, createdAt, objectURI,
		)
		if err != nil {
			log.Printf("Warning: Failed to insert note %s into FTS: %v", id, err)
			continue
		}
		if ftsRowid, err := result.LastInsertId(); err == nil {
			db.db.Exec(`INSERT OR REPLACE INTO posts_fts_lookup(source_id, fts_rowid) VALUES (?, ?)`, id, ftsRowid)
		}
		inserted++
	}

	return inserted, rows.Err()
}

// backfillActivitiesFTS inserts all existing remote Create activities into the FTS index
func (db *DB) backfillActivitiesFTS() (int, error) {
	rows, err := db.db.Query(`
		SELECT id, actor_uri, raw_json, created_at, COALESCE(object_uri, ''), COALESCE(object_url, '')
		FROM activities
		WHERE activity_type = 'Create' AND local = 0
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	inserted := 0
	for rows.Next() {
		var id, actorURI, rawJSON, createdAt, objectURI, objectURL string
		if err := rows.Scan(&id, &actorURI, &rawJSON, &createdAt, &objectURI, &objectURL); err != nil {
			log.Printf("Warning: Failed to scan activity for FTS backfill: %v", err)
			continue
		}

		content := extractContentFromJSON(rawJSON)
		if content == "" {
			continue
		}

		author := extractAuthorFromActorURI(actorURI)

		result, err := db.db.Exec(
			`INSERT INTO posts_fts(content, author, source_type, created_at, object_uri, object_url) VALUES (?, ?, 'activity', ?, ?, ?)`,
			content, author, createdAt, objectURI, objectURL,
		)
		if err != nil {
			log.Printf("Warning: Failed to insert activity %s into FTS: %v", id, err)
			continue
		}
		if ftsRowid, err := result.LastInsertId(); err == nil {
			db.db.Exec(`INSERT OR REPLACE INTO posts_fts_lookup(source_id, fts_rowid) VALUES (?, ?)`, id, ftsRowid)
		}
		inserted++
	}

	return inserted, rows.Err()
}
