package db

import (
	"context"
	"database/sql"
	"fmt"
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
	sqlInsertUser            = `INSERT INTO accounts(id, username, publickey, web_public_key, web_private_key, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlUpdateLoginUser       = `UPDATE accounts SET first_time_login = 0, username = ? WHERE publickey = ?`
	sqlUpdateLoginUserById   = `UPDATE accounts SET first_time_login = 0, username = ? WHERE id = ?`
	sqlSelectUserByPublicKey = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key FROM accounts WHERE publickey = ?`
	sqlSelectUserById        = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key FROM accounts WHERE id = ?`
	sqlSelectUserByUsername  = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key FROM accounts WHERE username = ?`

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
	sqlSelectNoteById = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id
                                                            WHERE notes.id = ?`
	sqlSelectNotesByUserId = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id
                                                            WHERE notes.user_id = ?
                                                            ORDER BY notes.created_at DESC`
	sqlSelectNotesByUsername = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id
                                                            WHERE accounts.username = ?
                                                            ORDER BY notes.created_at DESC`
	sqlSelectAllNotes = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id
                                                            ORDER BY notes.created_at DESC`

	// Local users and local timeline queries
	sqlSelectAllAccounts = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key FROM accounts WHERE first_time_login = 0 ORDER BY username ASC`
	sqlSelectLocalTimelineNotes = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
														INNER JOIN accounts ON accounts.id = notes.user_id
														ORDER BY notes.created_at DESC LIMIT ?`
	sqlSelectLocalTimelineNotesByFollows = `SELECT notes.id, accounts.username, notes.message, notes.created_at, notes.edited_at FROM notes
														INNER JOIN accounts ON accounts.id = notes.user_id
														WHERE notes.user_id = ? OR notes.user_id IN (
															SELECT target_account_id FROM follows
															WHERE account_id = ? AND accepted = 1 AND is_local = 1
														)
														ORDER BY notes.created_at DESC LIMIT ?`
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
	var noteId uuid.UUID
	err := db.wrapTransaction(func(tx *sql.Tx) error {
		id, err := db.insertNote(tx, userId, message)
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

func (db *DB) UpdateLoginByPkHash(username string, pkHash string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.updateLoginUser(tx, username, pkHash)
		if err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) UpdateLoginById(username string, id uuid.UUID) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.updateLoginUserById(tx, username, id)
		if err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) ReadAccBySession(s ssh.Session) (error, *domain.Account) {
	publicKeyToString := util.PublicKeyToString(s.PublicKey())
	var tempAcc domain.Account
	row := db.db.QueryRow(sqlSelectUserByPublicKey, util.PkToHash(publicKeyToString))
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey)
	if err == sql.ErrNoRows {
		return err, nil
	}
	return err, &tempAcc
}

func (db *DB) ReadAccByPkHash(pkHash string) (error, *domain.Account) {
	row := db.db.QueryRow(sqlSelectUserByPublicKey, pkHash)
	var tempAcc domain.Account
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey)
	if err == sql.ErrNoRows {
		return err, nil
	}
	return err, &tempAcc
}

func (db *DB) ReadAccById(id uuid.UUID) (error, *domain.Account) {
	row := db.db.QueryRow(sqlSelectUserById, id)
	var tempAcc domain.Account
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey)
	if err == sql.ErrNoRows {
		return err, nil
	}
	return err, &tempAcc
}

func (db *DB) ReadAccByUsername(username string) (error, *domain.Account) {
	row := db.db.QueryRow(sqlSelectUserByUsername, username)
	var tempAcc domain.Account
	err := row.Scan(&tempAcc.Id, &tempAcc.Username, &tempAcc.Publickey, &tempAcc.CreatedAt, &tempAcc.FirstTimeLogin, &tempAcc.WebPublicKey, &tempAcc.WebPrivateKey)
	if err == sql.ErrNoRows {
		return err, nil
	}
	return err, &tempAcc
}

// parseTimestamp parses a timestamp string from SQLite, handling both ISO 8601 and space-separated formats
// SQLite driver returns timestamps with Z suffix even though they're stored in local time
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

func (db *DB) ReadNoteId(id uuid.UUID) (error, *domain.Note) {
	row := db.db.QueryRow(sqlSelectNoteById, id)
	var note domain.Note
	var editedAtStr sql.NullString
	err := row.Scan(&note.Id, &note.CreatedBy, &note.Message, &note.CreatedAt, &editedAtStr)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if editedAtStr.Valid {
		if parsedTime, err := parseTimestamp(editedAtStr.String); err == nil {
			note.EditedAt = &parsedTime
		}
	}
	return err, &note
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

func GetDB() *DB {
	dbOnce.Do(func() {
		// Open database connection
		db, err := sql.Open("sqlite", "database.db")
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

		return nil
	})
}

// RunActivityPubMigrations runs ActivityPub-specific migrations
func (db *DB) RunActivityPubMigrations() error {
	log.Println("Running ActivityPub migrations...")
	return db.RunMigrations()
}

func (db *DB) createUserTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateUserTable)
	return err
}

func (db *DB) createNotesTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateNotesTable)
	return err
}

func (db *DB) insertUser(tx *sql.Tx, username string, publicKey string, webKeyPair *util.RsaKeyPair) error {
	_, err := tx.Exec(sqlInsertUser, uuid.New(), username, util.PkToHash(publicKey), webKeyPair.Public, webKeyPair.Private, time.Now())
	return err
}

func (db *DB) insertNote(tx *sql.Tx, userId uuid.UUID, message string) (uuid.UUID, error) {
	noteId := uuid.New()
	_, err := tx.Exec(sqlInsertNote, noteId, userId, message, time.Now().Format("2006-01-02 15:04:05"))
	return noteId, err
}

func (db *DB) updateNote(tx *sql.Tx, noteId uuid.UUID, message string) error {
	_, err := tx.Exec(sqlUpdateNote, message, time.Now().Format("2006-01-02 15:04:05"), noteId)
	return err
}

func (db *DB) deleteNote(tx *sql.Tx, noteId uuid.UUID) error {
	_, err := tx.Exec(sqlDeleteNote, noteId)
	return err
}

func (db *DB) updateLoginUser(tx *sql.Tx, username string, pkHash string) error {
	_, err := tx.Exec(sqlUpdateLoginUser, username, pkHash)
	return err
}

func (db *DB) updateLoginUserById(tx *sql.Tx, username string, id uuid.UUID) error {
	_, err := tx.Exec(sqlUpdateLoginUserById, username, id)
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
	sqlInsertRemoteAccount       = `INSERT INTO remote_accounts(id, username, domain, actor_uri, display_name, summary, inbox_uri, outbox_uri, public_key_pem, avatar_url, last_fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	sqlSelectRemoteAccountByURI  = `SELECT id, username, domain, actor_uri, display_name, summary, inbox_uri, outbox_uri, public_key_pem, avatar_url, last_fetched_at FROM remote_accounts WHERE actor_uri = ?`
	sqlSelectRemoteAccountById   = `SELECT id, username, domain, actor_uri, display_name, summary, inbox_uri, outbox_uri, public_key_pem, avatar_url, last_fetched_at FROM remote_accounts WHERE id = ?`
	sqlUpdateRemoteAccount       = `UPDATE remote_accounts SET display_name = ?, summary = ?, inbox_uri = ?, outbox_uri = ?, public_key_pem = ?, avatar_url = ?, last_fetched_at = ? WHERE actor_uri = ?`
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

// Follow queries
const (
	sqlInsertFollow         = `INSERT INTO follows(id, account_id, target_account_id, uri, accepted, created_at, is_local) VALUES (?, ?, ?, ?, ?, ?, ?)`
	sqlSelectFollowByURI    = `SELECT id, account_id, target_account_id, uri, accepted, created_at FROM follows WHERE uri = ?`
	sqlDeleteFollowByURI    = `DELETE FROM follows WHERE uri = ?`
	sqlSelectLocalFollowsByAccountId = `SELECT id, account_id, target_account_id, uri, accepted, created_at FROM follows WHERE account_id = ? AND is_local = 1 AND accepted = 1`
	sqlDeleteLocalFollow    = `DELETE FROM follows WHERE account_id = ? AND target_account_id = ? AND is_local = 1`
	sqlCheckLocalFollow     = `SELECT COUNT(*) FROM follows WHERE account_id = ? AND target_account_id = ? AND is_local = 1`
	sqlSelectFollowByAccountIds = `SELECT id, account_id, target_account_id, uri, accepted, created_at FROM follows WHERE account_id = ? AND target_account_id = ? AND accepted = 1`
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

// Activity queries
const (
	sqlInsertActivity = `INSERT INTO activities(id, activity_uri, activity_type, actor_uri, object_uri, raw_json, processed, local, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	sqlUpdateActivity = `UPDATE activities SET raw_json = ?, processed = ?, object_uri = ? WHERE id = ?`
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
			activity.RawJSON,
			activity.Processed,
			activity.Local,
			activity.CreatedAt.Format("2006-01-02 15:04:05"),
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

// ReadActivityByObjectURI reads an activity by searching for the object URI in the raw JSON
// This is needed because Create activities are stored with their activity ID, not the Note ID
func (db *DB) ReadActivityByObjectURI(objectURI string) (error, *domain.Activity) {
	var activity domain.Activity
	var idStr, actorURIStr string

	// Search for CREATE activities where the raw JSON contains the object URI
	// Filter by activity_type='Create' to avoid finding Update/Delete activities
	err := db.db.QueryRow(
		`SELECT id, activity_uri, activity_type, actor_uri, raw_json, processed, local, created_at
		 FROM activities
		 WHERE activity_type = 'Create' AND raw_json LIKE ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		"%\"id\":\""+objectURI+"\"%",
	).Scan(&idStr, &activity.ActivityURI, &activity.ActivityType, &actorURIStr,
		&activity.RawJSON, &activity.Processed, &activity.Local, &activity.CreatedAt)

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
	sqlSelectFederatedActivities = `SELECT id, activity_uri, activity_type, actor_uri, object_uri, raw_json, processed, local, created_at FROM activities WHERE activity_type = 'Create' AND local = 0 ORDER BY created_at DESC LIMIT ?`
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

// Delivery Queue queries
const (
	sqlInsertDeliveryQueue = `INSERT INTO delivery_queue(id, inbox_uri, activity_json, attempts, next_retry_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlSelectPendingDeliveries = `SELECT id, inbox_uri, activity_json, attempts, next_retry_at, created_at FROM delivery_queue WHERE next_retry_at <= ? ORDER BY created_at ASC LIMIT ?`
	sqlUpdateDeliveryAttempt = `UPDATE delivery_queue SET attempts = ?, next_retry_at = ? WHERE id = ?`
	sqlDeleteDelivery = `DELETE FROM delivery_queue WHERE id = ?`
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
	sqlSelectFollowersByAccountId  = `SELECT id, account_id, target_account_id, uri, accepted, created_at, is_local FROM follows WHERE target_account_id = ? AND accepted = 1`
	sqlSelectFollowingByAccountId = `SELECT id, account_id, target_account_id, uri, accepted, created_at, is_local FROM follows WHERE account_id = ? AND accepted = 1`
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
		if err := rows.Scan(&acc.Id, &acc.Username, &acc.Publickey, &acc.CreatedAt, &acc.FirstTimeLogin, &acc.WebPublicKey, &acc.WebPrivateKey); err != nil {
			return err, &accounts
		}
		accounts = append(accounts, acc)
	}
	if err = rows.Err(); err != nil {
		return err, &accounts
	}
	return nil, &accounts
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
		URI:             "", // No URI for local follows
		Accepted:        true,  // Auto-accept for local follows
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

