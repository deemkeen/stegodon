package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/ssh"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"log"
	"modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
	"time"
)

// DB is the database struct.
type DB struct {
	db *sql.DB
}

var (
	dbInstance *DB
	dbOnce     sync.Once
)

const (
	//TODO add indices

	//Accounts
	sqlCreateUserTable = `CREATE TABLE IF NOT EXISTS accounts(
                        id uuid NOT NULL PRIMARY KEY,
                        username varchar(100) UNIQUE NOT NULL,
                        publickey varchar(1000) UNIQUE,
                        created_at timestamp default current_timestamp,
                        first_time_login int default 1,
                        web_public_key text,
                        web_private_key text
                        )`

	// Terms and Conditions
	sqlCreateTermsAndConditionsTable = `CREATE TABLE IF NOT EXISTS terms_and_conditions(
                        id INTEGER PRIMARY KEY,
                        content TEXT NOT NULL,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                        )`

	sqlCreateUserTermsAcceptanceTable = `CREATE TABLE IF NOT EXISTS user_terms_acceptance(
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        user_id uuid NOT NULL,
                        terms_id INTEGER NOT NULL,
                        accepted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        FOREIGN KEY (user_id) REFERENCES accounts(id),
                        FOREIGN KEY (terms_id) REFERENCES terms_and_conditions(id)
                        )`
	sqlInsertUser               = `INSERT INTO accounts(id, username, publickey, web_public_key, web_private_key, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlUpdateLoginUser          = `UPDATE accounts SET first_time_login = 0, username = ?, display_name = ?, summary = ? WHERE publickey = ?`
	sqlUpdateLoginUserById      = `UPDATE accounts SET first_time_login = 0, username = ?, display_name = ?, summary = ? WHERE id = ?`
	sqlUpdateAccountDisplayName = `UPDATE accounts SET display_name = ? WHERE id = ?`
	sqlUpdateAccountSummary     = `UPDATE accounts SET summary = ? WHERE id = ?`
	sqlUpdateAccountAvatar      = `UPDATE accounts SET avatar_url = ? WHERE id = ?`
	sqlSelectUserByPublicKey    = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted, banned, last_ip FROM accounts WHERE publickey = ?`
	sqlSelectUserById           = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted, banned, last_ip FROM accounts WHERE id = ?`
	sqlSelectUserByUsername     = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted, banned, last_ip FROM accounts WHERE username = ?`

	//Notes
	sqlCreateNotesTable = `CREATE TABLE IF NOT EXISTS notes(
                        id uuid NOT NULL PRIMARY KEY,
                        user_id uuid NOT NULL,
                        message varchar(1000),
                        created_at timestamp default current_timestamp
                        )`
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

	// Local users and local timeline queries
	sqlSelectAllAccounts        = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted, banned, last_ip FROM accounts WHERE first_time_login = 0 ORDER BY username ASC`
	sqlSelectAllAccountsAdmin   = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted, banned, last_ip FROM accounts ORDER BY created_at ASC`
	sqlCountAccounts            = `SELECT COUNT(*) FROM accounts`
	sqlCountLocalPosts          = `SELECT COUNT(*) FROM notes`
	sqlCountActiveUsersMonth    = `SELECT COUNT(DISTINCT user_id) FROM notes WHERE created_at >= datetime('now', '-30 days')`
	sqlCountActiveUsersHalfYear = `SELECT COUNT(DISTINCT user_id) FROM notes WHERE created_at >= datetime('now', '-180 days')`
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

	// Outbox collection query - returns public notes for ActivityPub outbox
	sqlSelectPublicNotesByUsername = `SELECT notes.id, notes.user_id, notes.message, notes.created_at, notes.edited_at, notes.visibility, notes.object_uri
														FROM notes
														INNER JOIN accounts ON accounts.id = notes.user_id
														WHERE accounts.username = ? AND notes.visibility = 'public'
														ORDER BY notes.created_at DESC
														LIMIT ? OFFSET ?`
)

func (db *DB) CreateAccount(s ssh.Session, username string) (error, bool) {
	err, found := db.ReadAccBySession(s)
	if err != nil {
		log.Printf("No records for %s found, creating new user..", username)
	}

	if found != nil {
		return nil, true
	}

	keypair := util.GeneratePemKeypair()
	err2 := db.CreateAccByUsername(s, username, keypair)
	if err2 != nil {
		log.Println("Creating new user failed: ", err2)
		return err2, false
	}
	return nil, true
}

func (db *DB) CreateAccByUsername(s ssh.Session, username string, webKeyPair *util.RsaKeyPair) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.insertUser(tx, username, util.PublicKeyToString(s.PublicKey()), webKeyPair)
		if err != nil {
			return err
		}
		return nil
	})
}

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
	return noteId, err
}

func (db *DB) UpdateNote(noteId uuid.UUID, message string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.updateNote(tx, noteId, message)
		if err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) DeleteNoteById(noteId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.deleteNote(tx, noteId)
		if err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) UpdateLoginByPkHash(username string, displayName string, summary string, pkHash string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.updateLoginUser(tx, username, displayName, summary, pkHash)
		if err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) UpdateLoginById(username string, displayName string, summary string, id uuid.UUID) error {
	// Check if username is already taken by another user (before transaction)
	err, existingAcc := db.ReadAccByUsername(username)
	if err == nil && existingAcc != nil && existingAcc.Id != id {
		return fmt.Errorf("username '%s' is already taken", username)
	}

	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.updateLoginUserById(tx, username, displayName, summary, id)
		if err != nil {
			return err
		}
		return nil
	})
}

// UpdateAccountDisplayName updates only the display name for an account
func (db *DB) UpdateAccountDisplayName(accountId uuid.UUID, displayName string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateAccountDisplayName, displayName, accountId.String())
		return err
	})
}

// UpdateAccountSummary updates only the bio/summary for an account
func (db *DB) UpdateAccountSummary(accountId uuid.UUID, summary string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateAccountSummary, summary, accountId.String())
		return err
	})
}

// UpdateAccountAvatar updates only the avatar URL for an account
func (db *DB) UpdateAccountAvatar(accountId uuid.UUID, avatarURL string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateAccountAvatar, avatarURL, accountId.String())
		return err
	})
}

// CreateUploadToken creates a new one-time upload token
func (db *DB) CreateUploadToken(accountId uuid.UUID, token string, tokenType string, expiresIn time.Duration) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		expiresAt := time.Now().Add(expiresIn)
		_, err := tx.Exec(`INSERT INTO upload_tokens (token, account_id, token_type, expires_at) VALUES (?, ?, ?, ?)`,
			token, accountId.String(), tokenType, expiresAt.Format(time.RFC3339))
		return err
	})
}

// ValidateUploadToken validates a token and returns the account ID if valid
func (db *DB) ValidateUploadToken(token string) (uuid.UUID, string, error) {
	var accountIdStr, tokenType string
	var expiresAtStr string
	row := db.db.QueryRow(`SELECT account_id, token_type, expires_at FROM upload_tokens WHERE token = ?`, token)
	err := row.Scan(&accountIdStr, &tokenType, &expiresAtStr)
	if err != nil {
		return uuid.Nil, "", err
	}

	// Parse expiry time
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid expiry time: %w", err)
	}

	// Check if expired
	if time.Now().After(expiresAt) {
		return uuid.Nil, "", fmt.Errorf("token expired")
	}

	accountId, err := uuid.Parse(accountIdStr)
	if err != nil {
		return uuid.Nil, "", err
	}

	return accountId, tokenType, nil
}

// DeleteUploadToken removes a used or expired token
func (db *DB) DeleteUploadToken(token string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM upload_tokens WHERE token = ?`, token)
		return err
	})
}

// CleanupExpiredUploadTokens removes all expired tokens
func (db *DB) CleanupExpiredUploadTokens() error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM upload_tokens WHERE expires_at < ?`, time.Now().Format(time.RFC3339))
		return err
	})
}

// GetExistingUploadToken returns an existing valid token for an account, or empty string if none exists
func (db *DB) GetExistingUploadToken(accountId uuid.UUID, tokenType string) (string, time.Time, error) {
	var token, expiresAtStr string
	row := db.db.QueryRow(`SELECT token, expires_at FROM upload_tokens WHERE account_id = ? AND token_type = ? AND expires_at > ?`,
		accountId.String(), tokenType, time.Now().Format(time.RFC3339))
	err := row.Scan(&token, &expiresAtStr)
	if err == sql.ErrNoRows {
		return "", time.Time{}, nil // No existing token
	}
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

func (db *DB) ReadAccBySession(s ssh.Session) (error, *domain.Account) {
	publicKeyToString := util.PublicKeyToString(s.PublicKey())
	var tempAcc domain.Account
	var displayName, summary, avatarURL, lastIP sql.NullString
	var isAdmin, muted, banned sql.NullInt64
	row := db.db.QueryRow(sqlSelectUserByPublicKey, util.PkToHash(publicKeyToString))
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey, &displayName, &summary, &avatarURL, &isAdmin, &muted, &banned, &lastIP)
	if err == sql.ErrNoRows {
		return err, nil
	}
	tempAcc.DisplayName = displayName.String
	tempAcc.Summary = summary.String
	tempAcc.AvatarURL = avatarURL.String
	tempAcc.IsAdmin = isAdmin.Int64 == 1
	tempAcc.Muted = muted.Int64 == 1
	tempAcc.Banned = banned.Int64 == 1
	tempAcc.LastIP = lastIP.String
	return err, &tempAcc
}

func (db *DB) ReadAccByPkHash(pkHash string) (error, *domain.Account) {
	row := db.db.QueryRow(sqlSelectUserByPublicKey, pkHash)
	var tempAcc domain.Account
	var displayName, summary, avatarURL, lastIP sql.NullString
	var isAdmin, muted, banned sql.NullInt64
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey, &displayName, &summary, &avatarURL, &isAdmin, &muted, &banned, &lastIP)
	if err == sql.ErrNoRows {
		return err, nil
	}
	tempAcc.DisplayName = displayName.String
	tempAcc.Summary = summary.String
	tempAcc.AvatarURL = avatarURL.String
	tempAcc.IsAdmin = isAdmin.Int64 == 1
	tempAcc.Muted = muted.Int64 == 1
	tempAcc.Banned = banned.Int64 == 1
	tempAcc.LastIP = lastIP.String
	return err, &tempAcc
}

func (db *DB) ReadAccById(id uuid.UUID) (error, *domain.Account) {
	row := db.db.QueryRow(sqlSelectUserById, id)
	var tempAcc domain.Account
	var displayName, summary, avatarURL, lastIP sql.NullString
	var isAdmin, muted, banned sql.NullInt64
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey, &displayName, &summary, &avatarURL, &isAdmin, &muted, &banned, &lastIP)
	if err == sql.ErrNoRows {
		return err, nil
	}
	tempAcc.DisplayName = displayName.String
	tempAcc.Summary = summary.String
	tempAcc.AvatarURL = avatarURL.String
	tempAcc.IsAdmin = isAdmin.Int64 == 1
	tempAcc.Muted = muted.Int64 == 1
	tempAcc.Banned = banned.Int64 == 1
	tempAcc.LastIP = lastIP.String
	return err, &tempAcc
}

func (db *DB) ReadAccByUsername(username string) (error, *domain.Account) {
	row := db.db.QueryRow(sqlSelectUserByUsername, username)
	var tempAcc domain.Account
	var displayName, summary, avatarURL, lastIP sql.NullString
	var isAdmin, muted, banned sql.NullInt64
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey, &displayName, &summary, &avatarURL, &isAdmin, &muted, &banned, &lastIP)
	if err == sql.ErrNoRows {
		return err, nil
	}
	tempAcc.DisplayName = displayName.String
	tempAcc.Summary = summary.String
	tempAcc.AvatarURL = avatarURL.String
	tempAcc.IsAdmin = isAdmin.Int64 == 1
	tempAcc.Muted = muted.Int64 == 1
	tempAcc.Banned = banned.Int64 == 1
	tempAcc.LastIP = lastIP.String
	return err, &tempAcc
}

// parseTimestamp parses a timestamp string from SQLite, handling both ISO 8601 and space-separated formats
// SQLite driver returns timestamps with Z suffix even though they're stored in local time
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

// CreateDB creates the database.
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

		// Create terms and conditions tables
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
func (db *DB) createUserTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateUserTable)
	return err
}

func (db *DB) createNotesTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateNotesTable)
	return err
}

func (db *DB) createTermsAndConditionsTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateTermsAndConditionsTable)
	return err
}

func (db *DB) createUserTermsAcceptanceTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateUserTermsAcceptanceTable)
	return err
}

// RunActivityPubMigrations runs ActivityPub-specific migrations
func (db *DB) RunActivityPubMigrations() error {
	log.Println("Running ActivityPub migrations...")
	return db.RunMigrations()
}

func (db *DB) insertUser(tx *sql.Tx, username string, publicKey string, webKeyPair *util.RsaKeyPair) error {
	// Check if this is the first user
	var count int
	err := tx.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	if err != nil {
		return err
	}

	// Set is_admin to 1 for first user, 0 for others
	isAdmin := 0
	if count == 0 {
		isAdmin = 1
		log.Println("Creating first user as admin:", username)
	}

	_, err = tx.Exec(sqlInsertUser, uuid.New(), username, util.PkToHash(publicKey), webKeyPair.Public, webKeyPair.Private, time.Now())
	if err != nil {
		return err
	}

	// Update is_admin for the newly created user
	_, err = tx.Exec("UPDATE accounts SET is_admin = ? WHERE username = ?", isAdmin, username)
	return err
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

func (db *DB) updateLoginUser(tx *sql.Tx, username string, displayName string, summary string, pkHash string) error {
	_, err := tx.Exec(sqlUpdateLoginUser, username, displayName, summary, pkHash)
	return err
}

func (db *DB) updateLoginUserById(tx *sql.Tx, username string, displayName string, summary string, id uuid.UUID) error {
	_, err := tx.Exec(sqlUpdateLoginUserById, username, displayName, summary, id)
	return err
}

// wrapTransaction runs the given function within a transaction.
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

// Remote Accounts queries
const (
	sqlInsertRemoteAccount      = `INSERT INTO remote_accounts(id, username, domain, actor_uri, display_name, summary, inbox_uri, outbox_uri, public_key_pem, avatar_url, last_fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	sqlSelectRemoteAccountByURI = `SELECT id, username, domain, actor_uri, display_name, summary, inbox_uri, outbox_uri, public_key_pem, avatar_url, last_fetched_at FROM remote_accounts WHERE actor_uri = ?`
	sqlSelectRemoteAccountById  = `SELECT id, username, domain, actor_uri, display_name, summary, inbox_uri, outbox_uri, public_key_pem, avatar_url, last_fetched_at FROM remote_accounts WHERE id = ?`
	sqlUpdateRemoteAccount      = `UPDATE remote_accounts SET display_name = ?, summary = ?, inbox_uri = ?, outbox_uri = ?, public_key_pem = ?, avatar_url = ?, last_fetched_at = ? WHERE actor_uri = ?`
)

func (db *DB) CreateRemoteAccount(acc *domain.RemoteAccount) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertRemoteAccount,
			acc.Id.String(),
			acc.Username,
			acc.Domain,
			acc.ActorURI,
			acc.DisplayName,
			acc.Summary,
			acc.InboxURI,
			acc.OutboxURI,
			acc.PublicKeyPem,
			acc.AvatarURL,
			acc.LastFetchedAt,
		)
		return err
	})
}

func (db *DB) ReadRemoteAccountByURI(uri string) (error, *domain.RemoteAccount) {
	row := db.db.QueryRow(sqlSelectRemoteAccountByURI, uri)
	var acc domain.RemoteAccount
	var idStr string
	err := row.Scan(
		&idStr,
		&acc.Username,
		&acc.Domain,
		&acc.ActorURI,
		&acc.DisplayName,
		&acc.Summary,
		&acc.InboxURI,
		&acc.OutboxURI,
		&acc.PublicKeyPem,
		&acc.AvatarURL,
		&acc.LastFetchedAt,
	)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	acc.Id, _ = uuid.Parse(idStr)
	return nil, &acc
}

func (db *DB) ReadRemoteAccountById(id uuid.UUID) (error, *domain.RemoteAccount) {
	row := db.db.QueryRow(sqlSelectRemoteAccountById, id.String())
	var acc domain.RemoteAccount
	var idStr string
	err := row.Scan(
		&idStr,
		&acc.Username,
		&acc.Domain,
		&acc.ActorURI,
		&acc.DisplayName,
		&acc.Summary,
		&acc.InboxURI,
		&acc.OutboxURI,
		&acc.PublicKeyPem,
		&acc.AvatarURL,
		&acc.LastFetchedAt,
	)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	acc.Id, _ = uuid.Parse(idStr)
	return nil, &acc
}

func (db *DB) UpdateRemoteAccount(acc *domain.RemoteAccount) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateRemoteAccount,
			acc.DisplayName,
			acc.Summary,
			acc.InboxURI,
			acc.OutboxURI,
			acc.PublicKeyPem,
			acc.AvatarURL,
			acc.LastFetchedAt,
			acc.ActorURI,
		)
		return err
	})
}

// ReadAllRemoteAccounts returns all cached remote accounts for autocomplete
func (db *DB) ReadAllRemoteAccounts() (error, []domain.RemoteAccount) {
	rows, err := db.db.Query(`SELECT id, username, domain, actor_uri, display_name, summary, inbox_uri, outbox_uri, public_key_pem, avatar_url, last_fetched_at FROM remote_accounts ORDER BY username`)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var accounts []domain.RemoteAccount
	for rows.Next() {
		var acc domain.RemoteAccount
		var idStr string
		err := rows.Scan(
			&idStr,
			&acc.Username,
			&acc.Domain,
			&acc.ActorURI,
			&acc.DisplayName,
			&acc.Summary,
			&acc.InboxURI,
			&acc.OutboxURI,
			&acc.PublicKeyPem,
			&acc.AvatarURL,
			&acc.LastFetchedAt,
		)
		if err != nil {
			return err, nil
		}
		acc.Id, _ = uuid.Parse(idStr)
		accounts = append(accounts, acc)
	}
	return nil, accounts
}

// Follow queries
const (
	sqlInsertFollow                  = `INSERT INTO follows(id, account_id, target_account_id, uri, accepted, created_at, is_local) VALUES (?, ?, ?, ?, ?, ?, ?)`
	sqlSelectFollowByURI             = `SELECT id, account_id, target_account_id, uri, accepted, created_at FROM follows WHERE uri = ?`
	sqlDeleteFollowByURI             = `DELETE FROM follows WHERE uri = ?`
	sqlSelectLocalFollowsByAccountId = `SELECT id, account_id, target_account_id, uri, accepted, created_at FROM follows WHERE account_id = ? AND is_local = 1 AND accepted = 1`
	sqlDeleteLocalFollow             = `DELETE FROM follows WHERE account_id = ? AND target_account_id = ? AND is_local = 1`
	sqlCheckLocalFollow              = `SELECT COUNT(*) FROM follows WHERE account_id = ? AND target_account_id = ? AND is_local = 1`
	sqlSelectFollowByAccountIds      = `SELECT id, account_id, target_account_id, uri, accepted, created_at FROM follows WHERE account_id = ? AND target_account_id = ?`
)

func (db *DB) CreateFollow(follow *domain.Follow) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		isLocal := 0
		if follow.IsLocal {
			isLocal = 1
		}
		_, err := tx.Exec(sqlInsertFollow,
			follow.Id.String(),
			follow.AccountId.String(),
			follow.TargetAccountId.String(),
			follow.URI,
			follow.Accepted,
			follow.CreatedAt,
			isLocal,
		)
		return err
	})
}

func (db *DB) ReadFollowByURI(uri string) (error, *domain.Follow) {
	row := db.db.QueryRow(sqlSelectFollowByURI, uri)
	var follow domain.Follow
	var idStr, accountIdStr, targetIdStr string
	err := row.Scan(
		&idStr,
		&accountIdStr,
		&targetIdStr,
		&follow.URI,
		&follow.Accepted,
		&follow.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	follow.Id, _ = uuid.Parse(idStr)
	follow.AccountId, _ = uuid.Parse(accountIdStr)
	follow.TargetAccountId, _ = uuid.Parse(targetIdStr)
	return nil, &follow
}

func (db *DB) ReadFollowByAccountIds(accountId, targetAccountId uuid.UUID) (error, *domain.Follow) {
	row := db.db.QueryRow(sqlSelectFollowByAccountIds, accountId.String(), targetAccountId.String())
	var follow domain.Follow
	var idStr, accountIdStr, targetIdStr string
	err := row.Scan(
		&idStr,
		&accountIdStr,
		&targetIdStr,
		&follow.URI,
		&follow.Accepted,
		&follow.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	follow.Id, _ = uuid.Parse(idStr)
	follow.AccountId, _ = uuid.Parse(accountIdStr)
	follow.TargetAccountId, _ = uuid.Parse(targetIdStr)
	return nil, &follow
}

func (db *DB) DeleteFollowByURI(uri string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteFollowByURI, uri)
		return err
	})
}

func (db *DB) DeleteFollowByAccountIds(accountId, targetAccountId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteLocalFollow, accountId.String(), targetAccountId.String())
		return err
	})
}

func (db *DB) AcceptFollowByURI(uri string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE follows SET accepted = 1 WHERE uri = ?", uri)
		return err
	})
}

// CleanupOrphanedFollows removes follow records that point to deleted remote accounts
func (db *DB) CleanupOrphanedFollows() error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Delete remote follows where the remote account (either follower or followee) doesn't exist
		// For "Following" (local follows remote): target_account_id should be in remote_accounts
		// For "Followers" (remote follows local): account_id should be in remote_accounts
		result, err := tx.Exec(`
			DELETE FROM follows
			WHERE is_local = 0
			AND (
				(account_id NOT IN (SELECT id FROM remote_accounts)
				 AND account_id NOT IN (SELECT id FROM accounts))
				OR
				(target_account_id NOT IN (SELECT id FROM remote_accounts)
				 AND target_account_id NOT IN (SELECT id FROM accounts))
			)
		`)
		if err != nil {
			return err
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("Cleaned up %d orphaned follow records", rowsAffected)
		}
		return nil
	})
}

// Activity queries
const (
	sqlInsertActivity      = `INSERT INTO activities(id, activity_uri, activity_type, actor_uri, object_uri, object_url, in_reply_to, raw_json, processed, local, created_at, from_relay) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	sqlUpdateActivity      = `UPDATE activities SET raw_json = ?, processed = ?, object_uri = ? WHERE id = ?`
	sqlSelectActivityByURI = `SELECT id, activity_uri, activity_type, actor_uri, object_uri, raw_json, processed, local, created_at FROM activities WHERE activity_uri = ?`
)

func (db *DB) CreateActivity(activity *domain.Activity) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertActivity,
			activity.Id.String(),
			activity.ActivityURI,
			activity.ActivityType,
			activity.ActorURI,
			activity.ObjectURI,
			activity.ObjectURL,
			activity.InReplyTo,
			activity.RawJSON,
			activity.Processed,
			activity.Local,
			activity.CreatedAt.Format("2006-01-02 15:04:05"),
			activity.FromRelay,
		)
		return err
	})
}

func (db *DB) UpdateActivity(activity *domain.Activity) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateActivity,
			activity.RawJSON,
			activity.Processed,
			activity.ObjectURI,
			activity.Id.String(),
		)
		return err
	})
}

func (db *DB) ReadActivityByURI(uri string) (error, *domain.Activity) {
	row := db.db.QueryRow(sqlSelectActivityByURI, uri)
	var activity domain.Activity
	var idStr string
	err := row.Scan(
		&idStr,
		&activity.ActivityURI,
		&activity.ActivityType,
		&activity.ActorURI,
		&activity.ObjectURI,
		&activity.RawJSON,
		&activity.Processed,
		&activity.Local,
		&activity.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	activity.Id, _ = uuid.Parse(idStr)
	return nil, &activity
}

// ReadActivityByObjectURI reads an activity by the object URI
// First tries exact match on object_uri column, falls back to searching raw_json for older activities
func (db *DB) ReadActivityByObjectURI(objectURI string) (error, *domain.Activity) {
	var activity domain.Activity
	var idStr, actorURIStr string

	// First try exact match on object_uri column (faster and more reliable)
	err := db.db.QueryRow(
		`SELECT id, activity_uri, activity_type, actor_uri, raw_json, processed, local, created_at, COALESCE(like_count, 0), COALESCE(boost_count, 0)
		 FROM activities
		 WHERE activity_type = 'Create' AND object_uri = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		objectURI,
	).Scan(&idStr, &activity.ActivityURI, &activity.ActivityType, &actorURIStr,
		&activity.RawJSON, &activity.Processed, &activity.Local, &activity.CreatedAt, &activity.LikeCount, &activity.BoostCount)

	if err == nil {
		activity.Id, _ = uuid.Parse(idStr)
		activity.ActorURI = actorURIStr
		activity.ObjectURI = objectURI
		return nil, &activity
	}

	// If not found by column, fall back to LIKE search in raw_json for older activities
	if err.Error() != "sql: no rows in result set" {
		return err, nil
	}

	// Escape LIKE special characters to prevent wildcard injection
	escapedURI := strings.ReplaceAll(objectURI, "\\", "\\\\") // Escape backslash first
	escapedURI = strings.ReplaceAll(escapedURI, "%", "\\%")
	escapedURI = strings.ReplaceAll(escapedURI, "_", "\\_")

	// Search for CREATE activities where the raw JSON contains the object URI
	// Filter by activity_type='Create' to avoid finding Update/Delete activities
	err = db.db.QueryRow(
		`SELECT id, activity_uri, activity_type, actor_uri, raw_json, processed, local, created_at, COALESCE(like_count, 0), COALESCE(boost_count, 0)
		 FROM activities
		 WHERE activity_type = 'Create' AND raw_json LIKE ? ESCAPE '\'
		 ORDER BY created_at DESC
		 LIMIT 1`,
		"%\"id\":\""+escapedURI+"\"%",
	).Scan(&idStr, &activity.ActivityURI, &activity.ActivityType, &actorURIStr,
		&activity.RawJSON, &activity.Processed, &activity.Local, &activity.CreatedAt, &activity.LikeCount, &activity.BoostCount)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return err, nil
	}

	activity.Id, _ = uuid.Parse(idStr)
	activity.ActorURI = actorURIStr

	return nil, &activity
}

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
	// Local notes for home timeline: own posts + posts from followed local users (excluding replies)
	// Includes reply_count, like_count, and boost_count for denormalized counts
	sqlSelectHomeLocalNotes = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.object_uri, COALESCE(notes.reply_count, 0), COALESCE(notes.like_count, 0), COALESCE(notes.boost_count, 0) FROM notes
		INNER JOIN accounts ON accounts.id = notes.user_id
		WHERE (notes.in_reply_to_uri IS NULL OR notes.in_reply_to_uri = '')
		AND (notes.user_id = ? OR notes.user_id IN (
			SELECT target_account_id FROM follows
			WHERE account_id = ? AND accepted = 1 AND is_local = 1
		))
		ORDER BY notes.created_at DESC LIMIT ?`

	// Remote activities for home timeline: posts from followed remote users
	// Excludes replies (activities where inReplyTo has a URL value, not null)
	// Top-level posts have "inReplyTo":null, replies have "inReplyTo":"https://..."
	// Includes reply_count for denormalized reply counting
	sqlSelectHomeRemoteActivities = `SELECT a.id, a.actor_uri, a.object_uri, COALESCE(a.object_url, ''), a.raw_json, a.created_at, ra.username, ra.domain, COALESCE(a.reply_count, 0), COALESCE(a.like_count, 0), COALESCE(a.boost_count, 0)
		FROM activities a
		INNER JOIN remote_accounts ra ON ra.actor_uri = a.actor_uri
		INNER JOIN follows f ON f.target_account_id = ra.id
		WHERE a.activity_type = 'Create' AND a.local = 0 AND f.account_id = ? AND f.accepted = 1 AND f.is_local = 0
		AND a.raw_json NOT LIKE '%"inReplyTo":"http%'
		ORDER BY a.created_at DESC LIMIT ?`
)

// ReadHomeTimelinePosts returns a unified home timeline combining local and remote posts
func (db *DB) ReadHomeTimelinePosts(accountId uuid.UUID, limit int) (error, *[]domain.HomePost) {
	var posts []domain.HomePost

	// Fetch local notes (already excludes replies via sqlSelectHomeLocalNotes WHERE clause)
	localRows, err := db.db.Query(sqlSelectHomeLocalNotes, accountId.String(), accountId.String(), limit)
	if err != nil {
		return err, nil
	}
	defer localRows.Close()

	for localRows.Next() {
		var idStr string
		var username string
		var message string
		var createdAtStr string
		var objectURI sql.NullString
		var replyCount int
		var likeCount int
		var boostCount int

		if err := localRows.Scan(&idStr, &username, &message, &createdAtStr, &objectURI, &replyCount, &likeCount, &boostCount); err != nil {
			return err, &posts
		}

		noteId, _ := uuid.Parse(idStr)
		parsedTime, _ := parseTimestamp(createdAtStr)

		uri := ""
		if objectURI.Valid {
			uri = objectURI.String
		}

		posts = append(posts, domain.HomePost{
			ID:         noteId,
			Author:     username,
			Content:    message,
			Time:       parsedTime,
			ObjectURI:  uri,
			IsLocal:    true,
			NoteID:     noteId,
			ReplyCount: replyCount,
			LikeCount:  likeCount,
			BoostCount: boostCount,
		})
	}
	if err = localRows.Err(); err != nil {
		return err, &posts
	}

	// Fetch remote activities (query excludes all replies - only top-level posts)
	remoteRows, err := db.db.Query(sqlSelectHomeRemoteActivities, accountId.String(), limit)
	if err != nil {
		return err, &posts
	}
	defer remoteRows.Close()

	for remoteRows.Next() {
		var idStr string
		var actorURI string
		var objectURI string
		var objectURL string
		var rawJSON string
		var createdAtStr string
		var username string
		var remDomain string
		var replyCount int
		var likeCount int
		var boostCount int

		if err := remoteRows.Scan(&idStr, &actorURI, &objectURI, &objectURL, &rawJSON, &createdAtStr, &username, &remDomain, &replyCount, &likeCount, &boostCount); err != nil {
			return err, &posts
		}

		activityId, _ := uuid.Parse(idStr)
		parsedTime, _ := parseTimestamp(createdAtStr)

		// Extract content from raw JSON
		content := extractContentFromJSON(rawJSON)
		// If content is empty but we have a URL, show the URL as content
		if content == "" && objectURL != "" {
			content = objectURL
		}

		posts = append(posts, domain.HomePost{
			ID:         activityId,
			Author:     "@" + username + "@" + remDomain,
			Content:    content,
			Time:       parsedTime,
			ObjectURI:  objectURI,
			ObjectURL:  objectURL,
			IsLocal:    false,
			NoteID:     uuid.Nil,
			ReplyCount: replyCount,
			LikeCount:  likeCount,
			BoostCount: boostCount,
		})
	}
	if err = remoteRows.Err(); err != nil {
		return err, &posts
	}

	// Fetch relay-forwarded activities (marked with from_relay = 1)
	// These come from both FediBuzz (Announce-wrapped) and YUKIMOCHI (raw Create) relays
	relayRows, err := db.db.Query(`
		SELECT a.id, a.actor_uri, a.object_uri, COALESCE(a.object_url, ''), a.raw_json, a.created_at, COALESCE(a.reply_count, 0), COALESCE(a.like_count, 0), COALESCE(a.boost_count, 0)
		FROM activities a
		WHERE a.activity_type = 'Create' AND a.local = 0 AND a.from_relay = 1
		AND a.raw_json NOT LIKE '%"inReplyTo":"http%'
		ORDER BY a.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return err, &posts
	}
	defer relayRows.Close()

	for relayRows.Next() {
		var idStr string
		var actorURI string
		var objectURI string
		var objectURL string
		var rawJSON string
		var createdAtStr string
		var replyCount int
		var likeCount int
		var boostCount int

		if err := relayRows.Scan(&idStr, &actorURI, &objectURI, &objectURL, &rawJSON, &createdAtStr, &replyCount, &likeCount, &boostCount); err != nil {
			return err, &posts
		}

		activityId, _ := uuid.Parse(idStr)
		parsedTime, _ := parseTimestamp(createdAtStr)

		// Extract content from raw JSON
		content := extractContentFromJSON(rawJSON)
		// If content is empty but we have a URL, show the URL as content
		if content == "" && objectURL != "" {
			content = objectURL
		}

		// Extract author info from actorURI (format: https://domain/users/username)
		author := extractAuthorFromActorURI(actorURI)

		posts = append(posts, domain.HomePost{
			ID:         activityId,
			Author:     author,
			Content:    content,
			Time:       parsedTime,
			ObjectURI:  objectURI,
			ObjectURL:  objectURL,
			IsLocal:    false,
			NoteID:     uuid.Nil,
			ReplyCount: replyCount,
			LikeCount:  likeCount,
			BoostCount: boostCount,
		})
	}
	if err = relayRows.Err(); err != nil {
		return err, &posts
	}

	// Fetch posts boosted by the current user or by local users that the current user follows
	// Uses UNION to allow index usage (OR prevents index optimization)
	// Excludes self-boosts of your own posts (they already appear as original posts)
	boostedLocalRows, err := db.db.Query(`
		SELECT id, username, message, boost_time, object_uri, reply_count, like_count, boost_count, booster_username
		FROM (
			-- Own boosts of other users' posts
			SELECT n.id, a.username, n.message, b.created_at as boost_time, n.object_uri,
			       COALESCE(n.reply_count, 0) as reply_count, COALESCE(n.like_count, 0) as like_count,
			       COALESCE(n.boost_count, 0) as boost_count, booster.username as booster_username
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN notes n ON n.id = b.note_id
			INNER JOIN accounts a ON a.id = n.user_id
			WHERE b.account_id = ? AND n.user_id != ?

			UNION

			-- Boosts from followed users
			SELECT n.id, a.username, n.message, b.created_at as boost_time, n.object_uri,
			       COALESCE(n.reply_count, 0) as reply_count, COALESCE(n.like_count, 0) as like_count,
			       COALESCE(n.boost_count, 0) as boost_count, booster.username as booster_username
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN notes n ON n.id = b.note_id
			INNER JOIN accounts a ON a.id = n.user_id
			INNER JOIN follows f ON f.target_account_id = b.account_id AND f.account_id = ? AND f.accepted = 1
		)
		ORDER BY boost_time DESC LIMIT ?`,
		accountId.String(), accountId.String(), accountId.String(), limit)
	if err != nil {
		return err, &posts
	}
	defer boostedLocalRows.Close()

	for boostedLocalRows.Next() {
		var idStr string
		var username string
		var message string
		var createdAtStr string
		var objectURI sql.NullString
		var replyCount int
		var likeCount int
		var boostCount int
		var boosterUsername string

		if err := boostedLocalRows.Scan(&idStr, &username, &message, &createdAtStr, &objectURI, &replyCount, &likeCount, &boostCount, &boosterUsername); err != nil {
			return err, &posts
		}

		noteId, _ := uuid.Parse(idStr)
		parsedTime, _ := parseTimestamp(createdAtStr)

		uri := ""
		if objectURI.Valid {
			uri = objectURI.String
		}

		posts = append(posts, domain.HomePost{
			ID:         noteId,
			Author:     username,
			Content:    message,
			Time:       parsedTime,
			ObjectURI:  uri,
			IsLocal:    true,
			NoteID:     noteId,
			ReplyCount: replyCount,
			LikeCount:  likeCount,
			BoostCount: boostCount,
			BoostedBy:  "@" + boosterUsername,
		})
	}
	if err = boostedLocalRows.Err(); err != nil {
		return err, &posts
	}

	// Fetch remote posts boosted by the current user or by local users that the current user follows
	// Uses UNION to allow index usage (OR prevents index optimization)
	boostedRemoteRows, err := db.db.Query(`
		SELECT id, actor_uri, object_uri, object_url, raw_json, boost_time, username, domain,
		       reply_count, like_count, boost_count, booster_username
		FROM (
			-- Own boosts of remote posts
			SELECT act.id, act.actor_uri, act.object_uri, COALESCE(act.object_url, '') as object_url,
			       act.raw_json, b.created_at as boost_time, ra.username, ra.domain,
			       COALESCE(act.reply_count, 0) as reply_count, COALESCE(act.like_count, 0) as like_count,
			       COALESCE(act.boost_count, 0) as boost_count, booster.username as booster_username
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN activities act ON act.object_uri = b.object_uri
			INNER JOIN remote_accounts ra ON ra.actor_uri = act.actor_uri
			WHERE b.account_id = ? AND b.object_uri IS NOT NULL AND b.object_uri != ''

			UNION

			-- Boosts from followed users of remote posts
			SELECT act.id, act.actor_uri, act.object_uri, COALESCE(act.object_url, '') as object_url,
			       act.raw_json, b.created_at as boost_time, ra.username, ra.domain,
			       COALESCE(act.reply_count, 0) as reply_count, COALESCE(act.like_count, 0) as like_count,
			       COALESCE(act.boost_count, 0) as boost_count, booster.username as booster_username
			FROM boosts b
			INNER JOIN accounts booster ON booster.id = b.account_id
			INNER JOIN activities act ON act.object_uri = b.object_uri
			INNER JOIN remote_accounts ra ON ra.actor_uri = act.actor_uri
			INNER JOIN follows f ON f.target_account_id = b.account_id AND f.account_id = ? AND f.accepted = 1
			WHERE b.object_uri IS NOT NULL AND b.object_uri != ''
		)
		ORDER BY boost_time DESC LIMIT ?`,
		accountId.String(), accountId.String(), limit)
	if err != nil {
		return err, &posts
	}
	defer boostedRemoteRows.Close()

	for boostedRemoteRows.Next() {
		var idStr string
		var actorURI string
		var objectURI string
		var objectURL string
		var rawJSON string
		var createdAtStr string
		var username string
		var remDomain string
		var replyCount int
		var likeCount int
		var boostCount int
		var boosterUsername string

		if err := boostedRemoteRows.Scan(&idStr, &actorURI, &objectURI, &objectURL, &rawJSON, &createdAtStr, &username, &remDomain, &replyCount, &likeCount, &boostCount, &boosterUsername); err != nil {
			return err, &posts
		}

		activityId, _ := uuid.Parse(idStr)
		parsedTime, _ := parseTimestamp(createdAtStr)

		// Extract content from raw JSON
		content := extractContentFromJSON(rawJSON)
		// If content is empty but we have a URL, show the URL as content
		if content == "" && objectURL != "" {
			content = objectURL
		}

		posts = append(posts, domain.HomePost{
			ID:         activityId,
			Author:     "@" + username + "@" + remDomain,
			Content:    content,
			Time:       parsedTime,
			ObjectURI:  objectURI,
			ObjectURL:  objectURL,
			IsLocal:    false,
			NoteID:     uuid.Nil,
			ReplyCount: replyCount,
			LikeCount:  likeCount,
			BoostCount: boostCount,
			BoostedBy:  "@" + boosterUsername,
		})
	}
	if err = boostedRemoteRows.Err(); err != nil {
		return err, &posts
	}

	// Fetch boosts from followed REMOTE users (remote_account_id is set)
	// These are boosts where the booster is a remote user that the current user follows
	remoteBoosterRows, err := db.db.Query(`
		SELECT act.id, act.actor_uri, act.object_uri, COALESCE(act.object_url, '') as object_url,
		       act.raw_json, b.created_at as boost_time, ra_author.username, ra_author.domain,
		       COALESCE(act.reply_count, 0) as reply_count, COALESCE(act.like_count, 0) as like_count,
		       COALESCE(act.boost_count, 0) as boost_count,
		       ra_booster.username as booster_username, ra_booster.domain as booster_domain
		FROM boosts b
		INNER JOIN remote_accounts ra_booster ON ra_booster.id = b.remote_account_id
		INNER JOIN activities act ON act.object_uri = b.object_uri
		INNER JOIN remote_accounts ra_author ON ra_author.actor_uri = act.actor_uri
		INNER JOIN follows f ON f.target_account_id = b.remote_account_id AND f.account_id = ? AND f.accepted = 1
		WHERE b.remote_account_id IS NOT NULL AND b.remote_account_id != ''
		AND b.object_uri IS NOT NULL AND b.object_uri != ''
		ORDER BY b.created_at DESC LIMIT ?`,
		accountId.String(), limit)
	if err != nil {
		return err, &posts
	}
	defer remoteBoosterRows.Close()

	for remoteBoosterRows.Next() {
		var idStr string
		var actorURI string
		var objectURI string
		var objectURL string
		var rawJSON string
		var createdAtStr string
		var username string
		var remDomain string
		var replyCount int
		var likeCount int
		var boostCount int
		var boosterUsername string
		var boosterDomain string

		if err := remoteBoosterRows.Scan(&idStr, &actorURI, &objectURI, &objectURL, &rawJSON, &createdAtStr, &username, &remDomain, &replyCount, &likeCount, &boostCount, &boosterUsername, &boosterDomain); err != nil {
			return err, &posts
		}

		activityId, _ := uuid.Parse(idStr)
		parsedTime, _ := parseTimestamp(createdAtStr)

		// Extract content from raw JSON
		content := extractContentFromJSON(rawJSON)

		posts = append(posts, domain.HomePost{
			ID:         activityId,
			Author:     "@" + username + "@" + remDomain,
			Content:    content,
			Time:       parsedTime,
			ObjectURI:  objectURI,
			ObjectURL:  objectURL,
			IsLocal:    false,
			NoteID:     uuid.Nil,
			ReplyCount: replyCount,
			LikeCount:  likeCount,
			BoostCount: boostCount,
			BoostedBy:  "@" + boosterUsername + "@" + boosterDomain,
		})
	}
	if err = remoteBoosterRows.Err(); err != nil {
		return err, &posts
	}

	// Deduplicate posts - prefer non-boosted version (original) over boosted
	// Use a map to track seen posts by ID
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

	// Sort combined posts by time (newest first) using efficient sort
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Time.After(posts[j].Time)
	})

	// Limit to requested amount
	if len(posts) > limit {
		posts = posts[:limit]
	}

	return nil, &posts
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

// Delivery Queue queries
const (
	sqlInsertDeliveryQueue     = `INSERT INTO delivery_queue(id, inbox_uri, activity_json, attempts, next_retry_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlSelectPendingDeliveries = `SELECT id, inbox_uri, activity_json, attempts, next_retry_at, created_at FROM delivery_queue WHERE next_retry_at <= ? ORDER BY created_at ASC LIMIT ?`
	sqlUpdateDeliveryAttempt   = `UPDATE delivery_queue SET attempts = ?, next_retry_at = ? WHERE id = ?`
	sqlDeleteDelivery          = `DELETE FROM delivery_queue WHERE id = ?`
)

func (db *DB) EnqueueDelivery(item *domain.DeliveryQueueItem) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertDeliveryQueue,
			item.Id.String(),
			item.InboxURI,
			item.ActivityJSON,
			item.Attempts,
			item.NextRetryAt,
			item.CreatedAt,
		)
		return err
	})
}

func (db *DB) ReadPendingDeliveries(limit int) (error, *[]domain.DeliveryQueueItem) {
	rows, err := db.db.Query(sqlSelectPendingDeliveries, time.Now(), limit)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var items []domain.DeliveryQueueItem
	for rows.Next() {
		var item domain.DeliveryQueueItem
		var idStr string
		if err := rows.Scan(&idStr, &item.InboxURI, &item.ActivityJSON, &item.Attempts, &item.NextRetryAt, &item.CreatedAt); err != nil {
			return err, &items
		}
		item.Id, _ = uuid.Parse(idStr)
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return err, &items
	}
	return nil, &items
}

func (db *DB) UpdateDeliveryAttempt(id uuid.UUID, attempts int, nextRetry time.Time) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateDeliveryAttempt, attempts, nextRetry, id.String())
		return err
	})
}

func (db *DB) DeleteDelivery(id uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteDelivery, id.String())
		return err
	})
}

// Follower queries
const (
	sqlSelectFollowersByAccountId = `SELECT id, account_id, target_account_id, uri, accepted, created_at, is_local FROM follows WHERE target_account_id = ? AND accepted = 1`
	// Select following with LEFT JOIN to filter out orphaned remote follows
	sqlSelectFollowingByAccountId = `
		SELECT f.id, f.account_id, f.target_account_id, f.uri, f.accepted, f.created_at, f.is_local
		FROM follows f
		LEFT JOIN remote_accounts ra ON f.target_account_id = ra.id AND f.is_local = 0
		WHERE f.account_id = ?
		AND (f.is_local = 1 OR ra.id IS NOT NULL)
	`
)

func (db *DB) ReadFollowersByAccountId(accountId uuid.UUID) (error, *[]domain.Follow) {
	rows, err := db.db.Query(sqlSelectFollowersByAccountId, accountId.String())
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var followers []domain.Follow
	for rows.Next() {
		var follow domain.Follow
		var idStr, accountIdStr, targetIdStr string
		var isLocal int
		if err := rows.Scan(&idStr, &accountIdStr, &targetIdStr, &follow.URI, &follow.Accepted, &follow.CreatedAt, &isLocal); err != nil {
			return err, &followers
		}
		follow.Id, _ = uuid.Parse(idStr)
		follow.AccountId, _ = uuid.Parse(accountIdStr)
		follow.TargetAccountId, _ = uuid.Parse(targetIdStr)
		follow.IsLocal = isLocal == 1
		followers = append(followers, follow)
	}
	if err = rows.Err(); err != nil {
		return err, &followers
	}
	return nil, &followers
}

// ReadFollowingByAccountId returns all accounts that the given account is following (remote accounts)
func (db *DB) ReadFollowingByAccountId(accountId uuid.UUID) (error, *[]domain.Follow) {
	rows, err := db.db.Query(sqlSelectFollowingByAccountId, accountId.String())
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var following []domain.Follow
	for rows.Next() {
		var follow domain.Follow
		var idStr, accountIdStr, targetIdStr string
		var isLocal int
		if err := rows.Scan(&idStr, &accountIdStr, &targetIdStr, &follow.URI, &follow.Accepted, &follow.CreatedAt, &isLocal); err != nil {
			return err, &following
		}
		follow.Id, _ = uuid.Parse(idStr)
		follow.AccountId, _ = uuid.Parse(accountIdStr)
		follow.TargetAccountId, _ = uuid.Parse(targetIdStr)
		follow.IsLocal = isLocal == 1
		following = append(following, follow)
	}
	if err = rows.Err(); err != nil {
		return err, &following
	}
	return nil, &following
}

// ReadAllAccounts returns all local user accounts (excluding first-time login users)
func (db *DB) ReadAllAccounts() (error, *[]domain.Account) {
	rows, err := db.db.Query(sqlSelectAllAccounts)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var acc domain.Account
		var displayName, summary, avatarURL, lastIP sql.NullString
		var isAdmin, muted, banned sql.NullInt64
		if err := rows.Scan(&acc.Id, &acc.Username, &acc.Publickey, &acc.CreatedAt, &acc.FirstTimeLogin, &acc.WebPublicKey, &acc.WebPrivateKey, &displayName, &summary, &avatarURL, &isAdmin, &muted, &banned, &lastIP); err != nil {
			return err, &accounts
		}
		acc.DisplayName = displayName.String
		acc.Summary = summary.String
		acc.AvatarURL = avatarURL.String
		acc.IsAdmin = isAdmin.Int64 == 1
		acc.Muted = muted.Int64 == 1
		acc.Banned = banned.Int64 == 1
		acc.LastIP = lastIP.String
		accounts = append(accounts, acc)
	}
	if err = rows.Err(); err != nil {
		return err, &accounts
	}
	return nil, &accounts
}

// ReadAllAccountsAdmin returns all local user accounts including first-time login users (for admin panel)
func (db *DB) ReadAllAccountsAdmin() (error, *[]domain.Account) {
	rows, err := db.db.Query(sqlSelectAllAccountsAdmin)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var acc domain.Account
		var displayName, summary, avatarURL, lastIP sql.NullString
		var isAdmin, muted, banned sql.NullInt64
		if err := rows.Scan(&acc.Id, &acc.Username, &acc.Publickey, &acc.CreatedAt, &acc.FirstTimeLogin, &acc.WebPublicKey, &acc.WebPrivateKey, &displayName, &summary, &avatarURL, &isAdmin, &muted, &banned, &lastIP); err != nil {
			return err, &accounts
		}
		acc.DisplayName = displayName.String
		acc.Summary = summary.String
		acc.AvatarURL = avatarURL.String
		acc.IsAdmin = isAdmin.Int64 == 1
		acc.Muted = muted.Int64 == 1
		acc.Banned = banned.Int64 == 1
		acc.LastIP = lastIP.String
		accounts = append(accounts, acc)
	}
	if err = rows.Err(); err != nil {
		return err, &accounts
	}
	return nil, &accounts
}

// CountAccounts returns the total number of accounts in the database
func (db *DB) CountAccounts() (int, error) {
	var count int
	err := db.db.QueryRow(sqlCountAccounts).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountLocalPosts returns the total number of local posts (notes) in the database
func (db *DB) CountLocalPosts() (int, error) {
	var count int
	err := db.db.QueryRow(sqlCountLocalPosts).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountActiveUsersMonth returns the number of users who posted in the last 30 days
func (db *DB) CountActiveUsersMonth() (int, error) {
	var count int
	err := db.db.QueryRow(sqlCountActiveUsersMonth).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountActiveUsersHalfYear returns the number of users who posted in the last 180 days
func (db *DB) CountActiveUsersHalfYear() (int, error) {
	var count int
	err := db.db.QueryRow(sqlCountActiveUsersHalfYear).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteAccount deletes a local account and all associated data (notes, follows, activities)
func (db *DB) DeleteAccount(accountId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Delete all notes by this user
		_, err := tx.Exec("DELETE FROM notes WHERE user_id = ?", accountId.String())
		if err != nil {
			return fmt.Errorf("failed to delete notes: %w", err)
		}

		// Delete all follows where this user is the follower or target
		_, err = tx.Exec("DELETE FROM follows WHERE account_id = ? OR target_account_id = ?",
			accountId.String(), accountId.String())
		if err != nil {
			return fmt.Errorf("failed to delete follows: %w", err)
		}

		// Delete all likes by this user
		_, err = tx.Exec("DELETE FROM likes WHERE account_id = ?", accountId.String())
		if err != nil {
			return fmt.Errorf("failed to delete likes: %w", err)
		}

		// Delete all delivery queue items for this user (if table exists)
		_, err = tx.Exec("DELETE FROM delivery_queue WHERE account_id = ?", accountId.String())
		if err != nil {
			// Table might not exist in older schemas, log but don't fail
			log.Printf("Warning: failed to delete delivery queue items (table may not exist): %v", err)
		}

		// Note: We don't delete activities because they're linked by actor_uri (string) not account_id
		// Activities will remain as a historical record even after account deletion
		// This matches ActivityPub behavior where activities persist after account deletion

		// Finally, delete the account itself
		_, err = tx.Exec("DELETE FROM accounts WHERE id = ?", accountId.String())
		if err != nil {
			return fmt.Errorf("failed to delete account: %w", err)
		}

		return nil
	})
}

// MuteUser mutes a user and deletes all their posts
func (db *DB) MuteUser(accountId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Delete all notes by this user
		_, err := tx.Exec("DELETE FROM notes WHERE user_id = ?", accountId.String())
		if err != nil {
			return fmt.Errorf("failed to delete notes: %w", err)
		}

		// Set muted flag
		_, err = tx.Exec("UPDATE accounts SET muted = 1 WHERE id = ?", accountId.String())
		if err != nil {
			return fmt.Errorf("failed to mute user: %w", err)
		}

		log.Printf("Muted user %s and deleted their posts", accountId.String())
		return nil
	})
}

// UnmuteUser unmutes a user
func (db *DB) UnmuteUser(accountId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE accounts SET muted = 0 WHERE id = ?", accountId.String())
		if err != nil {
			return fmt.Errorf("failed to unmute user: %w", err)
		}
		log.Printf("Unmuted user %s", accountId.String())
		return nil
	})
}

// ReadLocalTimelineNotes returns recent notes from local users that the given account follows (plus their own posts)
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

// CreateLocalFollow creates a local-only follow relationship
func (db *DB) CreateLocalFollow(followerAccountId, targetAccountId uuid.UUID) error {
	follow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       followerAccountId,
		TargetAccountId: targetAccountId,
		URI:             "",   // No URI for local follows
		Accepted:        true, // Auto-accept for local follows
		IsLocal:         true,
		CreatedAt:       time.Now(),
	}
	return db.CreateFollow(follow)
}

// DeleteLocalFollow removes a local follow relationship
func (db *DB) DeleteLocalFollow(followerAccountId, targetAccountId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteLocalFollow, followerAccountId.String(), targetAccountId.String())
		return err
	})
}

// ReadPublicNotesByUsername returns public notes for a user's ActivityPub outbox with pagination
// Returns notes with full metadata including object_uri for ActivityPub compatibility
func (db *DB) ReadPublicNotesByUsername(username string, limit, offset int) (error, *[]domain.Note) {
	rows, err := db.db.Query(sqlSelectPublicNotesByUsername, username, limit, offset)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notes []domain.Note
	for rows.Next() {
		var note domain.Note
		var userId, visibility, objectURI sql.NullString
		var editedAt sql.NullTime

		err := rows.Scan(&note.Id, &userId, &note.Message, &note.CreatedAt, &editedAt, &visibility, &objectURI)
		if err != nil {
			return err, &notes
		}

		// Set username for note (we already know it from the query parameter)
		note.CreatedBy = username

		// Handle nullable fields
		if editedAt.Valid {
			note.EditedAt = &editedAt.Time
		}
		note.Visibility = visibility.String
		note.ObjectURI = objectURI.String

		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return err, &notes
	}
	return nil, &notes
}

// IsFollowingLocal checks if a user is following another local user
func (db *DB) IsFollowingLocal(followerAccountId, targetAccountId uuid.UUID) (bool, error) {
	var count int
	err := db.db.QueryRow(sqlCheckLocalFollow, followerAccountId.String(), targetAccountId.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ReadLocalFollowsByAccountId returns all local users that an account is following
func (db *DB) ReadLocalFollowsByAccountId(accountId uuid.UUID) (error, *[]domain.Follow) {
	rows, err := db.db.Query(sqlSelectLocalFollowsByAccountId, accountId.String())
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var follows []domain.Follow
	for rows.Next() {
		var follow domain.Follow
		var idStr, accountIdStr, targetIdStr string
		if err := rows.Scan(&idStr, &accountIdStr, &targetIdStr, &follow.URI, &follow.Accepted, &follow.CreatedAt); err != nil {
			return err, &follows
		}
		follow.Id, _ = uuid.Parse(idStr)
		follow.AccountId, _ = uuid.Parse(accountIdStr)
		follow.TargetAccountId, _ = uuid.Parse(targetIdStr)
		follow.IsLocal = true
		follows = append(follows, follow)
	}
	if err = rows.Err(); err != nil {
		return err, &follows
	}
	return nil, &follows
}

// DeleteActivity deletes an activity by ID
func (db *DB) DeleteActivity(id uuid.UUID) error {
	_, err := db.db.Exec("DELETE FROM activities WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("failed to delete activity: %w", err)
	}
	return nil
}

// ReadRemoteAccountByActorURI reads a remote account by its ActivityPub actor URI
func (db *DB) ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount) {
	var account domain.RemoteAccount
	var idStr string

	err := db.db.QueryRow(
		`SELECT id, actor_uri, username, domain, display_name, summary, avatar_url,
		 public_key_pem, inbox_uri, outbox_uri, last_fetched_at
		 FROM remote_accounts WHERE actor_uri = ?`,
		actorURI,
	).Scan(
		&idStr, &account.ActorURI, &account.Username, &account.Domain,
		&account.DisplayName, &account.Summary, &account.AvatarURL,
		&account.PublicKeyPem, &account.InboxURI, &account.OutboxURI,
		&account.LastFetchedAt,
	)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return err, nil
	}

	account.Id, _ = uuid.Parse(idStr)
	return nil, &account
}

// DeleteRemoteAccount deletes a remote account by ID
func (db *DB) DeleteRemoteAccount(id uuid.UUID) error {
	_, err := db.db.Exec("DELETE FROM remote_accounts WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("failed to delete remote account: %w", err)
	}
	return nil
}

// DeleteFollowsByRemoteAccountId deletes all follows to/from a remote account
func (db *DB) DeleteFollowsByRemoteAccountId(remoteAccountId uuid.UUID) error {
	// Delete follows where this account is the follower (account_id)
	_, err := db.db.Exec("DELETE FROM follows WHERE account_id = ? OR target_account_id = ?",
		remoteAccountId.String(), remoteAccountId.String())
	if err != nil {
		return fmt.Errorf("failed to delete follows: %w", err)
	}
	return nil
}

// MigrateKeysToPKCS8 converts all existing PKCS#1 keys to PKCS#8 format
// This is a one-time migration that preserves the cryptographic key material
func (db *DB) MigrateKeysToPKCS8() error {
	log.Println("Starting PKCS#1 to PKCS#8 key migration...")

	// Get all accounts
	rows, err := db.db.Query("SELECT id, username, web_private_key, web_public_key FROM accounts WHERE web_private_key IS NOT NULL")
	if err != nil {
		return fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	migratedCount := 0
	skippedCount := 0
	errorCount := 0

	for rows.Next() {
		var idStr, username, privateKeyPEM, publicKeyPEM string
		if err := rows.Scan(&idStr, &username, &privateKeyPEM, &publicKeyPEM); err != nil {
			log.Printf("Failed to scan account row: %v", err)
			errorCount++
			continue
		}

		// Convert private key
		newPrivateKey, err := util.ConvertPrivateKeyToPKCS8(privateKeyPEM)
		if err != nil {
			log.Printf("Failed to convert private key for user %s: %v", username, err)
			errorCount++
			continue
		}

		// Convert public key
		newPublicKey, err := util.ConvertPublicKeyToPKIX(publicKeyPEM)
		if err != nil {
			log.Printf("Failed to convert public key for user %s: %v", username, err)
			errorCount++
			continue
		}

		// Check if conversion was needed (keys already in new format)
		if newPrivateKey == privateKeyPEM && newPublicKey == publicKeyPEM {
			log.Printf("User %s: Keys already in PKCS#8 format, skipping", username)
			skippedCount++
			continue
		}

		// Update database
		_, err = db.db.Exec("UPDATE accounts SET web_private_key = ?, web_public_key = ? WHERE id = ?",
			newPrivateKey, newPublicKey, idStr)
		if err != nil {
			log.Printf("Failed to update keys for user %s: %v", username, err)
			errorCount++
			continue
		}

		log.Printf("User %s: Successfully migrated keys to PKCS#8 format", username)
		migratedCount++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating account rows: %w", err)
	}

	log.Printf("Key migration complete: %d migrated, %d skipped, %d errors", migratedCount, skippedCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("migration completed with %d errors", errorCount)
	}

	return nil
}

// MigrateDuplicateFollows removes duplicate follow relationships and adds UNIQUE constraint
// This is a one-time migration to fix the issue where multiple Follow activities
// from the same actor could create duplicate entries
func (db *DB) MigrateDuplicateFollows() error {
	log.Println("Starting duplicate follows cleanup migration...")

	// First, check if we already have the UNIQUE constraint
	// If the constraint exists, we can skip this migration
	rows, err := db.db.Query(`SELECT sql FROM sqlite_master WHERE type='table' AND name='follows'`)
	if err != nil {
		return fmt.Errorf("failed to check table schema: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			return fmt.Errorf("failed to read table schema: %w", err)
		}
		// Check if UNIQUE constraint already exists
		if strings.Contains(sql, "UNIQUE") && strings.Contains(sql, "account_id") && strings.Contains(sql, "target_account_id") {
			log.Println("UNIQUE constraint already exists on follows table, skipping migration")
			return nil
		}
	}

	// Find and count duplicate follows
	duplicateQuery := `
		SELECT account_id, target_account_id, COUNT(*) as count
		FROM follows
		GROUP BY account_id, target_account_id
		HAVING count > 1
	`
	dupRows, err := db.db.Query(duplicateQuery)
	if err != nil {
		return fmt.Errorf("failed to query duplicates: %w", err)
	}

	type duplicatePair struct {
		accountId       string
		targetAccountId string
		count           int
	}
	var duplicates []duplicatePair

	for dupRows.Next() {
		var dp duplicatePair
		if err := dupRows.Scan(&dp.accountId, &dp.targetAccountId, &dp.count); err != nil {
			log.Printf("Failed to scan duplicate row: %v", err)
			continue
		}
		duplicates = append(duplicates, dp)
	}
	dupRows.Close()

	if len(duplicates) == 0 {
		log.Println("No duplicate follows found")
	} else {
		log.Printf("Found %d pairs with duplicate follows", len(duplicates))

		// Remove duplicates, keeping only the oldest one (first created)
		removedCount := 0
		for _, dp := range duplicates {
			// Keep the oldest follow (MIN(created_at)), delete the rest
			deleteQuery := `
				DELETE FROM follows
				WHERE account_id = ? AND target_account_id = ?
				AND id NOT IN (
					SELECT id FROM follows
					WHERE account_id = ? AND target_account_id = ?
					ORDER BY created_at ASC
					LIMIT 1
				)
			`
			result, err := db.db.Exec(deleteQuery, dp.accountId, dp.targetAccountId, dp.accountId, dp.targetAccountId)
			if err != nil {
				log.Printf("Failed to delete duplicates for %s -> %s: %v", dp.accountId, dp.targetAccountId, err)
				continue
			}

			affected, _ := result.RowsAffected()
			removedCount += int(affected)
			log.Printf("Removed %d duplicate(s) for relationship %s -> %s", affected, dp.accountId, dp.targetAccountId)
		}

		log.Printf("Duplicate cleanup complete: removed %d duplicate follows", removedCount)
	}

	// Now add the UNIQUE constraint by recreating the table
	log.Println("Adding UNIQUE constraint to follows table...")

	// SQLite doesn't support ALTER TABLE to add constraints, so we need to recreate the table
	// https://www.sqlite.org/lang_altertable.html
	err = db.wrapTransaction(func(tx *sql.Tx) error {
		// Create new table with UNIQUE constraint
		_, err := tx.Exec(`
			CREATE TABLE follows_new (
				id TEXT NOT NULL PRIMARY KEY,
				account_id TEXT NOT NULL,
				target_account_id TEXT NOT NULL,
				uri TEXT NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				accepted INTEGER DEFAULT 0,
				is_local INTEGER DEFAULT 0,
				UNIQUE(account_id, target_account_id)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create new follows table: %w", err)
		}

		// Copy data from old table to new table
		_, err = tx.Exec(`
			INSERT INTO follows_new (id, account_id, target_account_id, uri, created_at, accepted, is_local)
			SELECT id, account_id, target_account_id, uri, created_at, accepted,
				   COALESCE(is_local, 0) as is_local
			FROM follows
		`)
		if err != nil {
			return fmt.Errorf("failed to copy data to new table: %w", err)
		}

		// Drop old table
		_, err = tx.Exec(`DROP TABLE follows`)
		if err != nil {
			return fmt.Errorf("failed to drop old table: %w", err)
		}

		// Rename new table to old name
		_, err = tx.Exec(`ALTER TABLE follows_new RENAME TO follows`)
		if err != nil {
			return fmt.Errorf("failed to rename new table: %w", err)
		}

		// Recreate indices
		_, err = tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_follows_account_id ON follows(account_id);
			CREATE INDEX IF NOT EXISTS idx_follows_target_account_id ON follows(target_account_id);
			CREATE INDEX IF NOT EXISTS idx_follows_uri ON follows(uri);
		`)
		if err != nil {
			return fmt.Errorf("failed to recreate indices: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to add UNIQUE constraint: %w", err)
	}

	log.Println("Successfully added UNIQUE constraint to follows table")
	return nil
}

// MigrateLocalReplyCounts recalculates reply_count for notes with local replies
// This fixes the issue where local-only replies (with "local:" prefix URIs) weren't
// being counted, causing threads not to open in the TUI
func (db *DB) MigrateLocalReplyCounts() error {
	log.Println("Starting local reply counts migration...")

	// Find all notes that have replies with "local:" prefix in_reply_to_uri
	// but have reply_count = 0
	query := `
		SELECT DISTINCT n.id
		FROM notes n
		WHERE n.reply_count = 0
		AND EXISTS (
			SELECT 1 FROM notes r
			WHERE r.in_reply_to_uri LIKE 'local:' || n.id || '%'
		)
	`
	rows, err := db.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query notes with local replies: %w", err)
	}
	defer rows.Close()

	var noteIds []string
	for rows.Next() {
		var noteId string
		if err := rows.Scan(&noteId); err != nil {
			log.Printf("Failed to scan note id: %v", err)
			continue
		}
		noteIds = append(noteIds, noteId)
	}

	if len(noteIds) == 0 {
		log.Println("No notes with uncounted local replies found")
		return nil
	}

	log.Printf("Found %d notes with uncounted local replies", len(noteIds))

	// For each note, count all replies (including nested) and update reply_count
	updatedCount := 0
	for _, noteId := range noteIds {
		// Count direct and indirect replies
		countQuery := `
			WITH RECURSIVE reply_chain(id) AS (
				-- Direct replies to this note
				SELECT id FROM notes WHERE in_reply_to_uri LIKE 'local:' || ? || '%'
				UNION ALL
				-- Replies to replies (recursive)
				SELECT n.id FROM notes n
				INNER JOIN reply_chain rc ON n.in_reply_to_uri LIKE 'local:' || rc.id || '%'
			)
			SELECT COUNT(*) FROM reply_chain
		`
		var replyCount int
		err := db.db.QueryRow(countQuery, noteId).Scan(&replyCount)
		if err != nil {
			log.Printf("Failed to count replies for note %s: %v", noteId, err)
			continue
		}

		if replyCount > 0 {
			_, err = db.db.Exec(`UPDATE notes SET reply_count = ? WHERE id = ?`, replyCount, noteId)
			if err != nil {
				log.Printf("Failed to update reply_count for note %s: %v", noteId, err)
				continue
			}
			log.Printf("Updated note %s: reply_count = %d", noteId, replyCount)
			updatedCount++
		}
	}

	log.Printf("Local reply counts migration complete: updated %d notes", updatedCount)
	return nil
}

// MigrateOrphanActivities removes Create activities that were incorrectly stored
// because the inbox handler stored them before checking if the user follows the actor.
// This is a one-time cleanup migration for the bug fix in handleCreateActivityWithDeps.
func (db *DB) MigrateOrphanActivities() error {
	log.Println("Starting orphan activities cleanup migration...")

	// Delete Create activities where:
	// 1. activity_type = 'Create'
	// 2. local = 0 (not local)
	// 3. from_relay = 0 (not from a relay)
	// 4. The actor is NOT followed by any local user
	// 5. The activity is NOT a reply to a local note
	//
	// Note: Activities from relays (from_relay = 1) are valid even without follow relationship
	// Note: Replies to local posts are valid even without follow relationship
	deleteQuery := `
		DELETE FROM activities
		WHERE id IN (
			SELECT a.id FROM activities a
			WHERE a.activity_type = 'Create'
			AND a.local = 0
			AND a.from_relay = 0
			-- Not followed by any local user
			AND NOT EXISTS (
				SELECT 1 FROM follows f
				INNER JOIN remote_accounts ra ON ra.id = f.target_account_id
				WHERE ra.actor_uri = a.actor_uri
				AND f.accepted = 1
			)
			-- Not a reply to a local note
			AND NOT EXISTS (
				SELECT 1 FROM notes n
				WHERE n.object_uri = a.in_reply_to
				AND a.in_reply_to IS NOT NULL
				AND a.in_reply_to != ''
			)
			-- Not a reply to another activity (local or remote)
			AND NOT EXISTS (
				SELECT 1 FROM activities parent
				WHERE parent.object_uri = a.in_reply_to
				AND a.in_reply_to IS NOT NULL
				AND a.in_reply_to != ''
			)
		)
	`

	result, err := db.db.Exec(deleteQuery)
	if err != nil {
		return fmt.Errorf("failed to delete orphan activities: %w", err)
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		log.Printf("Orphan activities migration: deleted %d orphan activities", deleted)
	} else {
		log.Println("Orphan activities migration: no orphan activities found")
	}

	return nil
}

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

// Mention queries
const (
	sqlInsertNoteMention        = `INSERT INTO note_mentions(id, note_id, mentioned_actor_uri, mentioned_username, mentioned_domain, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlSelectMentionsByNoteId   = `SELECT id, note_id, mentioned_actor_uri, mentioned_username, mentioned_domain, created_at FROM note_mentions WHERE note_id = ?`
	sqlSelectMentionsByActorURI = `SELECT nm.id, nm.note_id, nm.mentioned_actor_uri, nm.mentioned_username, nm.mentioned_domain, nm.created_at
								   FROM note_mentions nm ORDER BY nm.created_at DESC LIMIT ? OFFSET ?`
	sqlDeleteMentionsByNoteId = `DELETE FROM note_mentions WHERE note_id = ?`
)

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

// ReadActivitiesByInReplyTo finds all Create activities that are replies to the given URI
// This searches the raw_json field for "inReplyTo":"<uri>" patterns
// It supports both exact URI matches and partial matches (for notes without stored object_uri)
func (db *DB) ReadActivitiesByInReplyTo(parentURI string) (error, *[]domain.Activity) {
	// Search for activities where in_reply_to matches the parentURI (using indexed column)
	rows, err := db.db.Query(`
		SELECT id, activity_uri, activity_type, actor_uri, object_uri, COALESCE(object_url, ''), raw_json, processed, local, created_at, COALESCE(like_count, 0), COALESCE(boost_count, 0)
		FROM activities
		WHERE activity_type = 'Create'
		AND in_reply_to = ?
		ORDER BY created_at ASC`,
		parentURI)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var activities []domain.Activity
	for rows.Next() {
		var a domain.Activity
		var idStr string
		err := rows.Scan(&idStr, &a.ActivityURI, &a.ActivityType, &a.ActorURI, &a.ObjectURI, &a.ObjectURL, &a.RawJSON, &a.Processed, &a.Local, &a.CreatedAt, &a.LikeCount, &a.BoostCount)
		if err != nil {
			continue
		}
		a.Id, _ = uuid.Parse(idStr)
		activities = append(activities, a)
	}
	return nil, &activities
}

// CountActivitiesByInReplyTo counts Create activities that are replies to the given URI
func (db *DB) CountActivitiesByInReplyTo(parentURI string) (int, error) {
	var count int
	// Count activities that reply to parentURI using indexed in_reply_to column
	// Excludes duplicates of local notes (activities whose object_uri matches a local note
	// either by exact object_uri match or by /notes/{uuid} pattern in the object_uri)
	err := db.db.QueryRow(`
		SELECT COUNT(*)
		FROM activities a
		WHERE a.activity_type = 'Create'
		AND a.in_reply_to = ?
		AND NOT EXISTS (
			SELECT 1 FROM notes n
			WHERE (n.object_uri = a.object_uri AND n.object_uri IS NOT NULL AND n.object_uri != '')
			   OR (a.object_uri LIKE '%/notes/' || n.id || '%')
		)`,
		parentURI).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ========== Relay Functions ==========

// CreateRelay creates a new relay subscription
func (db *DB) CreateRelay(relay *domain.Relay) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO relays(id, actor_uri, inbox_uri, follow_uri, name, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			relay.Id.String(),
			relay.ActorURI,
			relay.InboxURI,
			relay.FollowURI,
			relay.Name,
			relay.Status,
			relay.CreatedAt.Format(time.RFC3339))
		return err
	})
}

// ReadAllRelays returns all relay subscriptions
func (db *DB) ReadAllRelays() (error, *[]domain.Relay) {
	rows, err := db.db.Query(`SELECT id, actor_uri, inbox_uri, COALESCE(follow_uri, ''), name, status, COALESCE(paused, 0), created_at, accepted_at FROM relays ORDER BY created_at DESC`)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var relays []domain.Relay
	for rows.Next() {
		var relay domain.Relay
		var idStr, createdAtStr string
		var acceptedAtStr sql.NullString
		var paused int
		if err := rows.Scan(&idStr, &relay.ActorURI, &relay.InboxURI, &relay.FollowURI, &relay.Name, &relay.Status, &paused, &createdAtStr, &acceptedAtStr); err != nil {
			return err, nil
		}
		relay.Id, _ = uuid.Parse(idStr)
		relay.Paused = paused == 1
		relay.CreatedAt, _ = parseTimestamp(createdAtStr)
		if acceptedAtStr.Valid {
			t, _ := parseTimestamp(acceptedAtStr.String)
			relay.AcceptedAt = &t
		}
		relays = append(relays, relay)
	}
	return nil, &relays
}

// ReadActiveRelays returns all relay subscriptions with status='active'
func (db *DB) ReadActiveRelays() (error, *[]domain.Relay) {
	rows, err := db.db.Query(`SELECT id, actor_uri, inbox_uri, COALESCE(follow_uri, ''), name, status, COALESCE(paused, 0), created_at, accepted_at FROM relays WHERE status = 'active'`)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var relays []domain.Relay
	for rows.Next() {
		var relay domain.Relay
		var idStr, createdAtStr string
		var acceptedAtStr sql.NullString
		var paused int
		if err := rows.Scan(&idStr, &relay.ActorURI, &relay.InboxURI, &relay.FollowURI, &relay.Name, &relay.Status, &paused, &createdAtStr, &acceptedAtStr); err != nil {
			return err, nil
		}
		relay.Id, _ = uuid.Parse(idStr)
		relay.Paused = paused == 1
		relay.CreatedAt, _ = parseTimestamp(createdAtStr)
		if acceptedAtStr.Valid {
			t, _ := parseTimestamp(acceptedAtStr.String)
			relay.AcceptedAt = &t
		}
		relays = append(relays, relay)
	}
	return nil, &relays
}

// ReadActiveUnpausedRelays returns all relay subscriptions with status='active' and paused=0
func (db *DB) ReadActiveUnpausedRelays() (error, *[]domain.Relay) {
	rows, err := db.db.Query(`SELECT id, actor_uri, inbox_uri, COALESCE(follow_uri, ''), name, status, COALESCE(paused, 0), created_at, accepted_at FROM relays WHERE status = 'active' AND COALESCE(paused, 0) = 0`)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var relays []domain.Relay
	for rows.Next() {
		var relay domain.Relay
		var idStr, createdAtStr string
		var acceptedAtStr sql.NullString
		var paused int
		if err := rows.Scan(&idStr, &relay.ActorURI, &relay.InboxURI, &relay.FollowURI, &relay.Name, &relay.Status, &paused, &createdAtStr, &acceptedAtStr); err != nil {
			return err, nil
		}
		relay.Id, _ = uuid.Parse(idStr)
		relay.Paused = paused == 1
		relay.CreatedAt, _ = parseTimestamp(createdAtStr)
		if acceptedAtStr.Valid {
			t, _ := parseTimestamp(acceptedAtStr.String)
			relay.AcceptedAt = &t
		}
		relays = append(relays, relay)
	}
	return nil, &relays
}

// ReadRelayByActorURI returns a relay by its actor URI
func (db *DB) ReadRelayByActorURI(actorURI string) (error, *domain.Relay) {
	var relay domain.Relay
	var idStr, createdAtStr string
	var acceptedAtStr, followURI sql.NullString
	var paused int

	err := db.db.QueryRow(`SELECT id, actor_uri, inbox_uri, follow_uri, name, status, COALESCE(paused, 0), created_at, accepted_at FROM relays WHERE actor_uri = ?`, actorURI).
		Scan(&idStr, &relay.ActorURI, &relay.InboxURI, &followURI, &relay.Name, &relay.Status, &paused, &createdAtStr, &acceptedAtStr)
	if err != nil {
		return err, nil
	}

	relay.Id, _ = uuid.Parse(idStr)
	relay.Paused = paused == 1
	relay.CreatedAt, _ = parseTimestamp(createdAtStr)
	if acceptedAtStr.Valid {
		t, _ := parseTimestamp(acceptedAtStr.String)
		relay.AcceptedAt = &t
	}
	if followURI.Valid {
		relay.FollowURI = followURI.String
	}
	return nil, &relay
}

// ReadRelayById returns a relay by its ID
func (db *DB) ReadRelayById(id uuid.UUID) (error, *domain.Relay) {
	var relay domain.Relay
	var idStr, createdAtStr string
	var acceptedAtStr, followURI sql.NullString
	var paused int

	err := db.db.QueryRow(`SELECT id, actor_uri, inbox_uri, follow_uri, name, status, COALESCE(paused, 0), created_at, accepted_at FROM relays WHERE id = ?`, id.String()).
		Scan(&idStr, &relay.ActorURI, &relay.InboxURI, &followURI, &relay.Name, &relay.Status, &paused, &createdAtStr, &acceptedAtStr)
	if err != nil {
		return err, nil
	}

	relay.Id, _ = uuid.Parse(idStr)
	relay.Paused = paused == 1
	relay.CreatedAt, _ = parseTimestamp(createdAtStr)
	if acceptedAtStr.Valid {
		t, _ := parseTimestamp(acceptedAtStr.String)
		relay.AcceptedAt = &t
	}
	if followURI.Valid {
		relay.FollowURI = followURI.String
	}
	return nil, &relay
}

// UpdateRelayStatus updates a relay's status and optionally sets accepted_at
func (db *DB) UpdateRelayStatus(id uuid.UUID, status string, acceptedAt *time.Time) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		if acceptedAt != nil {
			_, err := tx.Exec(`UPDATE relays SET status = ?, accepted_at = ? WHERE id = ?`,
				status, acceptedAt.Format(time.RFC3339), id.String())
			return err
		}
		_, err := tx.Exec(`UPDATE relays SET status = ? WHERE id = ?`, status, id.String())
		return err
	})
}

// DeleteRelay deletes a relay subscription
func (db *DB) DeleteRelay(id uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM relays WHERE id = ?`, id.String())
		return err
	})
}

// UpdateRelayPaused updates a relay's paused status
func (db *DB) UpdateRelayPaused(id uuid.UUID, paused bool) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		pausedInt := 0
		if paused {
			pausedInt = 1
		}
		_, err := tx.Exec(`UPDATE relays SET paused = ? WHERE id = ?`, pausedInt, id.String())
		return err
	})
}

// DeleteRelayActivities deletes all activities that were forwarded by relays (from_relay=1)
func (db *DB) DeleteRelayActivities() (int64, error) {
	var count int64
	err := db.wrapTransaction(func(tx *sql.Tx) error {
		result, err := tx.Exec(`DELETE FROM activities WHERE from_relay = 1`)
		if err != nil {
			return err
		}
		count, _ = result.RowsAffected()
		return nil
	})
	return count, err
}

// ============================================================================
// Notifications
// ============================================================================

const (
	sqlInsertNotification = `INSERT INTO notifications(id, account_id, notification_type, actor_id, actor_username, actor_domain, note_id, note_uri, note_preview, read, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	sqlSelectNotificationsByAccountId = `SELECT id, account_id, notification_type, actor_id, actor_username, actor_domain, note_id, note_uri, note_preview, read, created_at
		FROM notifications
		WHERE account_id = ?
		ORDER BY created_at DESC
		LIMIT ?`

	sqlSelectUnreadCountByAccountId = `SELECT COUNT(*) FROM notifications WHERE account_id = ? AND read = 0`

	sqlMarkNotificationRead = `UPDATE notifications SET read = 1 WHERE id = ?`

	sqlMarkAllNotificationsRead = `UPDATE notifications SET read = 1 WHERE account_id = ?`

	sqlDeleteNotification     = `DELETE FROM notifications WHERE id = ?`
	sqlDeleteAllNotifications = `DELETE FROM notifications WHERE account_id = ?`
)

// CreateNotification creates a new notification
func (db *DB) CreateNotification(notification *domain.Notification) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		readInt := 0
		if notification.Read {
			readInt = 1
		}

		// Handle optional note fields (NULL for follow notifications)
		var noteIdStr interface{}
		if notification.NoteId != uuid.Nil {
			noteIdStr = notification.NoteId.String()
		} else {
			noteIdStr = nil
		}

		var noteURI interface{}
		if notification.NoteURI != "" {
			noteURI = notification.NoteURI
		} else {
			noteURI = nil
		}

		var notePreview interface{}
		if notification.NotePreview != "" {
			notePreview = notification.NotePreview
		} else {
			notePreview = nil
		}

		_, err := tx.Exec(sqlInsertNotification,
			notification.Id.String(),
			notification.AccountId.String(),
			string(notification.NotificationType),
			notification.ActorId.String(),
			notification.ActorUsername,
			notification.ActorDomain,
			noteIdStr,
			noteURI,
			notePreview,
			readInt,
			notification.CreatedAt.Format(time.RFC3339))
		return err
	})
}

// ReadNotificationsByAccountId retrieves notifications for an account
func (db *DB) ReadNotificationsByAccountId(accountId uuid.UUID, limit int) (error, *[]domain.Notification) {
	rows, err := db.db.Query(sqlSelectNotificationsByAccountId, accountId.String(), limit)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notifications []domain.Notification
	for rows.Next() {
		var n domain.Notification
		var idStr, accountIdStr, notificationTypeStr, actorIdStr string
		var actorUsername, actorDomain, createdAtStr string
		var noteIdStr, noteURI, notePreview sql.NullString
		var readInt int

		if err := rows.Scan(&idStr, &accountIdStr, &notificationTypeStr, &actorIdStr,
			&actorUsername, &actorDomain, &noteIdStr, &noteURI, &notePreview,
			&readInt, &createdAtStr); err != nil {
			return err, &notifications
		}

		// Parse UUIDs
		n.Id, _ = uuid.Parse(idStr)
		n.AccountId, _ = uuid.Parse(accountIdStr)
		n.ActorId, _ = uuid.Parse(actorIdStr)
		if noteIdStr.Valid {
			n.NoteId, _ = uuid.Parse(noteIdStr.String)
		}

		n.NotificationType = domain.NotificationType(notificationTypeStr)
		n.ActorUsername = actorUsername
		n.ActorDomain = actorDomain
		n.NoteURI = noteURI.String
		n.NotePreview = notePreview.String
		n.Read = readInt == 1

		// Parse timestamp
		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			log.Printf("Failed to parse notification created_at: %v", err)
			createdAt = time.Now()
		}
		n.CreatedAt = createdAt

		notifications = append(notifications, n)
	}

	if err = rows.Err(); err != nil {
		return err, &notifications
	}

	return nil, &notifications
}

// ReadUnreadNotificationCount returns the count of unread notifications for an account
func (db *DB) ReadUnreadNotificationCount(accountId uuid.UUID) (int, error) {
	var count int
	err := db.db.QueryRow(sqlSelectUnreadCountByAccountId, accountId.String()).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// MarkNotificationRead marks a notification as read
func (db *DB) MarkNotificationRead(notificationId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlMarkNotificationRead, notificationId.String())
		return err
	})
}

// MarkAllNotificationsRead marks all notifications for an account as read
func (db *DB) MarkAllNotificationsRead(accountId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlMarkAllNotificationsRead, accountId.String())
		return err
	})
}

// DeleteNotification deletes a notification
func (db *DB) DeleteNotification(notificationId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteNotification, notificationId.String())
		return err
	})
}

// DeleteAllNotifications deletes all notifications for an account
func (db *DB) DeleteAllNotifications(accountId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteAllNotifications, accountId.String())
		return err
	})
}

// ============================================================================
// Info Boxes
// ============================================================================

const (
	sqlSelectAllInfoBoxes     = `SELECT id, title, content, order_num, enabled, created_at, updated_at FROM info_boxes ORDER BY order_num ASC`
	sqlSelectEnabledInfoBoxes = `SELECT id, title, content, order_num, enabled, created_at, updated_at FROM info_boxes WHERE enabled = 1 ORDER BY order_num ASC`
	sqlSelectInfoBoxById      = `SELECT id, title, content, order_num, enabled, created_at, updated_at FROM info_boxes WHERE id = ?`
	sqlInsertInfoBox          = `INSERT INTO info_boxes(id, title, content, order_num, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	sqlUpdateInfoBox          = `UPDATE info_boxes SET title = ?, content = ?, order_num = ?, enabled = ?, updated_at = ? WHERE id = ?`
	sqlDeleteInfoBox          = `DELETE FROM info_boxes WHERE id = ?`
	sqlToggleInfoBoxEnabled   = `UPDATE info_boxes SET enabled = NOT enabled, updated_at = ? WHERE id = ?`
)

// ReadAllInfoBoxes returns all info boxes ordered by order_num
func (db *DB) ReadAllInfoBoxes() (error, *[]domain.InfoBox) {
	rows, err := db.db.Query(sqlSelectAllInfoBoxes)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var boxes []domain.InfoBox
	for rows.Next() {
		var box domain.InfoBox
		var idStr, createdAtStr, updatedAtStr string
		var enabled int
		if err := rows.Scan(&idStr, &box.Title, &box.Content, &box.OrderNum, &enabled, &createdAtStr, &updatedAtStr); err != nil {
			return err, &boxes
		}
		box.Id, _ = uuid.Parse(idStr)
		box.Enabled = enabled == 1
		box.CreatedAt, _ = parseTimestamp(createdAtStr)
		box.UpdatedAt, _ = parseTimestamp(updatedAtStr)
		boxes = append(boxes, box)
	}
	if err = rows.Err(); err != nil {
		return err, &boxes
	}
	return nil, &boxes
}

// ReadEnabledInfoBoxes returns only enabled info boxes ordered by order_num
func (db *DB) ReadEnabledInfoBoxes() (error, *[]domain.InfoBox) {
	rows, err := db.db.Query(sqlSelectEnabledInfoBoxes)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var boxes []domain.InfoBox
	for rows.Next() {
		var box domain.InfoBox
		var idStr, createdAtStr, updatedAtStr string
		var enabled int
		if err := rows.Scan(&idStr, &box.Title, &box.Content, &box.OrderNum, &enabled, &createdAtStr, &updatedAtStr); err != nil {
			return err, &boxes
		}
		box.Id, _ = uuid.Parse(idStr)
		box.Enabled = enabled == 1
		box.CreatedAt, _ = parseTimestamp(createdAtStr)
		box.UpdatedAt, _ = parseTimestamp(updatedAtStr)
		boxes = append(boxes, box)
	}
	if err = rows.Err(); err != nil {
		return err, &boxes
	}
	return nil, &boxes
}

// ReadInfoBoxById returns a single info box by ID
func (db *DB) ReadInfoBoxById(id uuid.UUID) (error, *domain.InfoBox) {
	row := db.db.QueryRow(sqlSelectInfoBoxById, id.String())
	var box domain.InfoBox
	var idStr, createdAtStr, updatedAtStr string
	var enabled int
	err := row.Scan(&idStr, &box.Title, &box.Content, &box.OrderNum, &enabled, &createdAtStr, &updatedAtStr)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	box.Id, _ = uuid.Parse(idStr)
	box.Enabled = enabled == 1
	box.CreatedAt, _ = parseTimestamp(createdAtStr)
	box.UpdatedAt, _ = parseTimestamp(updatedAtStr)
	return nil, &box
}

// CreateInfoBox creates a new info box
func (db *DB) CreateInfoBox(box *domain.InfoBox) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		enabledInt := 0
		if box.Enabled {
			enabledInt = 1
		}
		_, err := tx.Exec(sqlInsertInfoBox,
			box.Id.String(),
			box.Title,
			box.Content,
			box.OrderNum,
			enabledInt,
			box.CreatedAt.Format(time.RFC3339),
			box.UpdatedAt.Format(time.RFC3339))
		return err
	})
}

// UpdateInfoBox updates an existing info box
func (db *DB) UpdateInfoBox(box *domain.InfoBox) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		enabledInt := 0
		if box.Enabled {
			enabledInt = 1
		}
		box.UpdatedAt = time.Now()
		_, err := tx.Exec(sqlUpdateInfoBox,
			box.Title,
			box.Content,
			box.OrderNum,
			enabledInt,
			box.UpdatedAt.Format(time.RFC3339),
			box.Id.String())
		return err
	})
}

// DeleteInfoBox deletes an info box
func (db *DB) DeleteInfoBox(id uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteInfoBox, id.String())
		return err
	})
}

// ToggleInfoBoxEnabled toggles the enabled status of an info box
func (db *DB) ToggleInfoBoxEnabled(id uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlToggleInfoBoxEnabled, time.Now().Format(time.RFC3339), id.String())
		return err
	})
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

// ============================================================================
// Server Message
// ============================================================================

const (
	sqlSelectServerMessage = `SELECT id, message, enabled, web_enabled, updated_at FROM server_message WHERE id = 1`
	sqlInsertServerMessage = `INSERT OR REPLACE INTO server_message(id, message, enabled, web_enabled, updated_at) VALUES (1, ?, ?, ?, ?)`
	sqlUpdateServerMessage = `UPDATE server_message SET message = ?, enabled = ?, web_enabled = ?, updated_at = ? WHERE id = 1`
)

// ReadServerMessage returns the current server message (single row)
func (db *DB) ReadServerMessage() (error, *domain.ServerMessage) {
	row := db.db.QueryRow(sqlSelectServerMessage)
	var msg domain.ServerMessage
	var updatedAtStr string
	var enabled, webEnabled int
	err := row.Scan(&msg.Id, &msg.Message, &enabled, &webEnabled, &updatedAtStr)
	if err == sql.ErrNoRows {
		// No message exists yet, return empty disabled message
		return nil, &domain.ServerMessage{
			Id:         1,
			Message:    "",
			Enabled:    false,
			WebEnabled: true, // Default to enabled for web
			UpdatedAt:  time.Now(),
		}
	}
	if err != nil {
		return err, nil
	}
	msg.Enabled = enabled == 1
	msg.WebEnabled = webEnabled == 1
	msg.UpdatedAt, _ = parseTimestamp(updatedAtStr)
	return nil, &msg
}

// UpdateServerMessage updates the server message (creates if doesn't exist)
func (db *DB) UpdateServerMessage(message string, enabled bool, webEnabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	webEnabledInt := 0
	if webEnabled {
		webEnabledInt = 1
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	result, err := db.db.Exec(sqlUpdateServerMessage, message, enabledInt, webEnabledInt, timestamp)
	if err != nil {
		return err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows affected, insert new row
	if rowsAffected == 0 {
		_, err = db.db.Exec(sqlInsertServerMessage, message, enabledInt, webEnabledInt, timestamp)
	}
	return err
}

// ============================================================================
// Global Timeline (Local + Federated Posts)
// ============================================================================

// ReadGlobalTimelinePosts returns posts for the global timeline (local notes + remote activities)
// excluding replies. Uses UNION ALL for efficient SQL-level sorting and pagination.
func (db *DB) ReadGlobalTimelinePosts(limit, offset int) (error, *[]domain.GlobalTimelinePost) {
	// Use UNION ALL to combine local, remote, and boosted posts with SQL-level sorting and pagination
	rows, err := db.db.Query(`
		SELECT
			id, username, user_domain, profile_url, object_uri, object_url,
			is_remote, message, created_at, reply_count, like_count, boost_count, boosted_by
		FROM (
			-- Local posts (excluding replies)
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

			UNION ALL

			-- Remote posts (excluding replies, using indexed in_reply_to column)
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

			UNION ALL

			-- Boosted local posts (show who boosted them)
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

			UNION ALL

			-- Boosted remote posts (show who boosted them - local boosters)
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

			UNION ALL

			-- Boosted remote posts (show who boosted them - remote boosters)
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
		) combined
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
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

// ============================================================================
// Ban Management
// ============================================================================

const (
	sqlCreateBan      = `INSERT INTO bans(id, username, ip_address, public_key_hash, reason, banned_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlReadAllBans    = `SELECT id, username, ip_address, public_key_hash, reason, banned_at FROM bans ORDER BY banned_at DESC`
	sqlDeleteBan      = `DELETE FROM bans WHERE id = ?`
	// IP bans expire after 60 days - only check recent bans
	sqlCheckIPBanned  = `SELECT COUNT(*) FROM bans WHERE ip_address = ? AND ip_address != '' AND banned_at >= datetime('now', '-60 days')`
	sqlCheckKeyBanned = `SELECT COUNT(*) FROM bans WHERE public_key_hash = ?`
	// Cleanup query to clear expired IP addresses (older than 60 days)
	sqlClearExpiredIPBans = `UPDATE bans SET ip_address = '' WHERE ip_address != '' AND banned_at < datetime('now', '-60 days')`
)

// CreateBan adds a new ban record with IP address and public key hash
func (db *DB) CreateBan(id, username, ipAddress, publicKeyHash, reason string) error {
	_, err := db.db.Exec(sqlCreateBan, id, username, ipAddress, publicKeyHash, reason, time.Now())
	return err
}

// ReadAllBans returns all ban records
func (db *DB) ReadAllBans() (error, *[]domain.Ban) {
	rows, err := db.db.Query(sqlReadAllBans)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var bans []domain.Ban
	for rows.Next() {
		var ban domain.Ban
		var bannedAtStr string
		err := rows.Scan(&ban.Id, &ban.Username, &ban.IPAddress, &ban.PublicKeyHash, &ban.Reason, &bannedAtStr)
		if err != nil {
			log.Printf("Error scanning ban: %v", err)
			continue
		}
		ban.BannedAt, _ = parseTimestamp(bannedAtStr)
		bans = append(bans, ban)
	}

	return nil, &bans
}

// DeleteBan removes a ban record by ID
func (db *DB) DeleteBan(id string) error {
	_, err := db.db.Exec(sqlDeleteBan, id)
	return err
}

// IsIPBanned checks if an IP address is banned
func (db *DB) IsIPBanned(ipAddress string) bool {
	var count int
	err := db.db.QueryRow(sqlCheckIPBanned, ipAddress).Scan(&count)
	if err != nil {
		log.Printf("Error checking IP ban: %v", err)
		return false
	}
	return count > 0
}

// IsPublicKeyBanned checks if a public key hash is banned
func (db *DB) IsPublicKeyBanned(publicKeyHash string) bool {
	var count int
	err := db.db.QueryRow(sqlCheckKeyBanned, publicKeyHash).Scan(&count)
	if err != nil {
		log.Printf("Error checking public key ban: %v", err)
		return false
	}
	return count > 0
}

// BanAccount sets the banned flag on an account and creates a ban record
func (db *DB) BanAccount(accountId uuid.UUID) error {
	_, err := db.db.Exec(`UPDATE accounts SET banned = 1 WHERE id = ?`, accountId.String())
	return err
}

// UnbanAccount clears the banned flag on an account
func (db *DB) UnbanAccount(accountId uuid.UUID) error {
	_, err := db.db.Exec(`UPDATE accounts SET banned = 0 WHERE id = ?`, accountId.String())
	return err
}

// UpdateAccountLastIP updates the last_ip field for an account
func (db *DB) UpdateAccountLastIP(accountId uuid.UUID, ipAddress string) error {
	_, err := db.db.Exec(`UPDATE accounts SET last_ip = ? WHERE id = ?`, ipAddress, accountId.String())
	return err
}

// UpdateAccountLastIPByPkHash updates the last_ip field for an account by public key hash
func (db *DB) UpdateAccountLastIPByPkHash(pkHash string, ipAddress string) error {
	_, err := db.db.Exec(`UPDATE accounts SET last_ip = ? WHERE publickey = ?`, ipAddress, pkHash)
	return err
}

// CleanupExpiredIPBans clears IP addresses from bans older than 60 days
// The ban record is kept (for public key blocking) but the IP is cleared
func (db *DB) CleanupExpiredIPBans() (int64, error) {
	result, err := db.db.Exec(sqlClearExpiredIPBans)
	if err != nil {
		log.Printf("Failed to cleanup expired IP bans: %v", err)
		return 0, err
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		log.Printf("Cleared %d expired IP addresses from ban records", affected)
	}
	return affected, nil
}

// ============================================================================
// Terms and Conditions
// ============================================================================

const (
	sqlInsertTermsAndConditions = `INSERT OR REPLACE INTO terms_and_conditions(id, content, updated_at) VALUES (1, ?, ?)`
	sqlSelectTermsAndConditions = `SELECT id, content, updated_at FROM terms_and_conditions WHERE id = 1`
	sqlInsertUserTermsAcceptance = `INSERT INTO user_terms_acceptance(user_id, terms_id, accepted_at) VALUES (?, 1, ?)`
	sqlSelectUserTermsAcceptance = `SELECT id, user_id, terms_id, accepted_at FROM user_terms_acceptance WHERE user_id = ?`
)

// GetCurrentTermsAndConditions returns the current terms and conditions
func (db *DB) GetCurrentTermsAndConditions() (error, *domain.TermsAndConditions) {
	row := db.db.QueryRow(sqlSelectTermsAndConditions)
	var terms domain.TermsAndConditions
	var updatedAtStr string
	err := row.Scan(&terms.Id, &terms.Content, &updatedAtStr)
	if err == sql.ErrNoRows {
		// No terms exist yet, return default terms
		return nil, &domain.TermsAndConditions{
			Id:      1,
			Content: "Default Terms and Conditions",
			UpdatedAt: time.Now(),
		}
	}
	if err != nil {
		return err, nil
	}
	terms.UpdatedAt, _ = parseTimestamp(updatedAtStr)
	return nil, &terms
}

// UpdateTermsAndConditions updates the terms and conditions
func (db *DB) UpdateTermsAndConditions(content string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertTermsAndConditions, content, time.Now().Format(time.RFC3339))
		return err
	})
}

// RecordUserTermsAcceptance records that a user has accepted the terms and conditions
func (db *DB) RecordUserTermsAcceptance(userId uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlInsertUserTermsAcceptance, userId.String(), time.Now().Format(time.RFC3339))
		return err
	})
}

// GetUserTermsAcceptance returns the terms acceptance record for a user
func (db *DB) GetUserTermsAcceptance(userId uuid.UUID) (error, *domain.UserTermsAcceptance) {
	row := db.db.QueryRow(sqlSelectUserTermsAcceptance, userId.String())
	var acceptance domain.UserTermsAcceptance
	var acceptedAtStr string
	err := row.Scan(&acceptance.Id, &acceptance.UserId, &acceptance.TermsId, &acceptedAtStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return err, nil
	}
	acceptance.AcceptedAt, _ = parseTimestamp(acceptedAtStr)
	return nil, &acceptance
}

// UserNeedsToAcceptTerms checks if a user needs to accept terms and conditions.
// Returns true if:
// - Terms exist and user has never accepted them
// - Terms were updated after user's last acceptance
func (db *DB) UserNeedsToAcceptTerms(userId uuid.UUID) (bool, error) {
	// Get current terms
	err, terms := db.GetCurrentTermsAndConditions()
	if err != nil {
		return false, err
	}

	// Check if terms are just the default (no real terms set)
	if terms.Content == "Default Terms and Conditions" {
		return false, nil
	}

	// Get user's acceptance record
	err, acceptance := db.GetUserTermsAcceptance(userId)
	if err != nil {
		return false, err
	}

	// User has never accepted
	if acceptance == nil {
		return true, nil
	}

	// Check if terms were updated after acceptance
	if terms.UpdatedAt.After(acceptance.AcceptedAt) {
		return true, nil
	}

	return false, nil
}
