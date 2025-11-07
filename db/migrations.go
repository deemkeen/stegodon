package db

import (
	"database/sql"
	"log"
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
	`

	// Delivery queue table
	sqlCreateDeliveryQueueTable = `CREATE TABLE IF NOT EXISTS delivery_queue (
		id TEXT NOT NULL PRIMARY KEY,
		activity_id TEXT NOT NULL,
		inbox_uri TEXT NOT NULL,
		attempts INTEGER DEFAULT 0,
		next_retry_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_error TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	sqlCreateDeliveryQueueIndices = `
		CREATE INDEX IF NOT EXISTS idx_delivery_queue_next_retry ON delivery_queue(next_retry_at);
		CREATE INDEX IF NOT EXISTS idx_delivery_queue_activity_id ON delivery_queue(activity_id);
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
		if err := db.createTableIfNotExists(tx, sqlCreateDeliveryQueueTable, "delivery_queue"); err != nil {
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
		if _, err := tx.Exec(sqlCreateDeliveryQueueIndices); err != nil {
			log.Printf("Warning: Failed to create delivery_queue indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNotesIndices); err != nil {
			log.Printf("Warning: Failed to create notes indices: %v", err)
		}

		// Extend existing tables (ignore errors if columns already exist)
		db.extendExistingTables(tx)

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

	// Try to add columns to notes table (ignore errors if they exist)
	tx.Exec("ALTER TABLE notes ADD COLUMN visibility TEXT DEFAULT 'public'")
	tx.Exec("ALTER TABLE notes ADD COLUMN in_reply_to_uri TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN object_uri TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN federated INTEGER DEFAULT 1")
	tx.Exec("ALTER TABLE notes ADD COLUMN sensitive INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE notes ADD COLUMN content_warning TEXT")

	log.Println("Extended existing tables with new columns")
}
