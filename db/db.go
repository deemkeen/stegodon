// db.go — Core database infrastructure: connection management, transactions, and shared helpers.
// Principle: Boundaries over Boundaries — this file defines the single database entry point
// (GetDB singleton) and shared primitives. Feature-specific operations live in their own files:
//   accounts.go, activitypub.go, admin.go, delivery.go, engagement.go,
//   notes.go, notifications.go, relay.go, search.go, terms.go, timeline.go

package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/deemkeen/stegodon/util"
	"modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

// DB is the database struct.
type DB struct {
	db *sql.DB
}

var (
	dbInstance *DB
	dbOnce     sync.Once
)

// SQL for core table creation (called by CreateDB during initial setup).
// Account table SQL is defined in accounts.go (sqlCreateUserTable).
const (
	sqlCreateNotesTable = `CREATE TABLE IF NOT EXISTS notes(
                        id uuid NOT NULL PRIMARY KEY,
                        user_id uuid NOT NULL,
                        message varchar(1000),
                        created_at timestamp default current_timestamp
                        )`
)

// GetDB returns the singleton database instance, initializing it on first call.
// It opens the SQLite connection, configures WAL mode and connection pooling,
// and runs initial schema creation (accounts, notes, terms tables).
func GetDB() *DB {
	dbOnce.Do(func() {
		// Resolve database path (local first, then user config dir)
		dbPath := util.ResolveFilePath("database.db")
		log.Printf("Using database at: %s", dbPath)

		// Open database connection
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			panic(err)
		}

		// Configure connection pool for concurrent access
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Hour)

		// Try to enable WAL2 mode, fall back to WAL if not supported
		var journalMode string
		err = db.QueryRow("PRAGMA journal_mode=WAL2").Scan(&journalMode)
		if err != nil || journalMode == "delete" {
			// WAL2 not supported, try regular WAL
			err = db.QueryRow("PRAGMA journal_mode=WAL").Scan(&journalMode)
			if err != nil {
				log.Printf("Warning: Failed to enable WAL mode: %v", err)
			} else {
				log.Printf("Database journal mode: %s (WAL2 not supported, using WAL)", journalMode)
			}
		} else {
			log.Printf("Database journal mode: %s", journalMode)
		}

		// Optimize PRAGMAs for concurrent ActivityPub workload
		// These need to be set as connection defaults
		db.Exec("PRAGMA synchronous = NORMAL")      // Reduces fsync calls
		db.Exec("PRAGMA cache_size = -64000")       // 64MB cache per connection
		db.Exec("PRAGMA temp_store = MEMORY")       // Store temp tables in RAM
		db.Exec("PRAGMA busy_timeout = 5000")       // Wait up to 5s for locks
		db.Exec("PRAGMA foreign_keys = ON")         // Enable FK constraints
		db.Exec("PRAGMA auto_vacuum = INCREMENTAL") // Better performance than FULL

		log.Printf("Database initialized with connection pooling (max 25 connections)")

		dbInstance = &DB{db: db}

		// Run initial schema setup
		err2 := dbInstance.CreateDB()
		if err2 != nil {
			panic(err2)
		}
	})

	return dbInstance
}

// CreateDB creates the core database tables (accounts, notes, terms).
// Called once during GetDB initialization.
func (db *DB) CreateDB() error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.createUserTable(tx)
		if err != nil {
			return err
		}

		err2 := db.createNotesTable(tx)
		if err2 != nil {
			return err2
		}

		// Create terms and conditions tables (defined in terms.go)
		err3 := db.createTermsAndConditionsTable(tx)
		if err3 != nil {
			return err3
		}

		err4 := db.createUserTermsAcceptanceTable(tx)
		if err4 != nil {
			return err4
		}

		return nil
	})
}

// createUserTable creates the accounts table if it does not exist.
// Uses sqlCreateUserTable defined in accounts.go.
func (db *DB) createUserTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateUserTable)
	return err
}

// createNotesTable creates the notes table if it does not exist.
func (db *DB) createNotesTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateNotesTable)
	return err
}

// RunActivityPubMigrations runs ActivityPub-specific migrations.
// Called from app.go during application initialization.
func (db *DB) RunActivityPubMigrations() error {
	log.Println("Running ActivityPub migrations...")
	return db.RunMigrations()
}

// parseTimestamp parses a timestamp string from SQLite into a time.Time.
//
// SQLite stores timestamps without timezone info. When reading back, the Go
// SQLite driver adds a "Z" suffix, but these are local time, NOT UTC.
// This function handles the Z suffix BEFORE RFC3339 parsing to avoid
// misinterpreting local timestamps as UTC.
//
// Used by many files across the db package.
func parseTimestamp(timestampStr string) (time.Time, error) {
	if timestampStr == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	// Handle Z suffix FIRST - SQLite adds Z but timestamps are actually local time
	// Must check before RFC3339 because RFC3339 would parse Z as UTC
	if strings.HasSuffix(timestampStr, "Z") {
		timestampStr = strings.TrimSuffix(timestampStr, "Z")
		timestampStr = strings.Replace(timestampStr, "T", " ", 1)
		return time.ParseInLocation("2006-01-02 15:04:05", timestampStr, time.Local)
	}

	// Try RFC3339 for timestamps with real timezone offsets (e.g., "2026-01-21T00:38:20+01:00")
	if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
		return t, nil
	}

	// Parse as local time (space-separated format)
	return time.ParseInLocation("2006-01-02 15:04:05", timestampStr, time.Local)
}

// wrapTransaction runs the given function within a database transaction.
// It handles SQLITE_BUSY retries automatically.
//
// Used by many files across the db package for all write operations.
func (db *DB) wrapTransaction(f func(tx *sql.Tx) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("error starting transaction: %s", err)
		return err
	}
	for {
		err = f(tx)
		if err != nil {
			serr, ok := err.(*sqlite.Error)
			if ok && serr.Code() == sqlitelib.SQLITE_BUSY {
				continue
			}
			log.Printf("error in transaction: %s", err)
			return err
		}
		err = tx.Commit()
		if err != nil {
			log.Printf("error committing transaction: %s", err)
			return err
		}
		break
	}
	return nil
}
