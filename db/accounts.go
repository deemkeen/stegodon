// accounts.go — Account creation, lookup, updates, admin queries, local follows, and IP tracking.
// Principle: One File, One Mental Model — all account-related database
// operations live together as a cohesive unit.

package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// Account queries
const (
	sqlCreateUserTable = `CREATE TABLE IF NOT EXISTS accounts(
                        id uuid NOT NULL PRIMARY KEY,
                        username varchar(100) UNIQUE NOT NULL,
                        publickey varchar(1000) UNIQUE,
                        created_at timestamp default current_timestamp,
                        first_time_login int default 1,
                        web_public_key text,
                        web_private_key text
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
)

// Admin and stats queries
const (
	sqlSelectAllAccounts        = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted, banned, last_ip FROM accounts WHERE first_time_login = 0 ORDER BY username ASC`
	sqlSelectAllAccountsAdmin   = `SELECT id, username, publickey, created_at, first_time_login, web_public_key, web_private_key, display_name, summary, avatar_url, is_admin, muted, banned, last_ip FROM accounts ORDER BY created_at ASC`
	sqlCountAccounts            = `SELECT COUNT(*) FROM accounts`
	sqlCountLocalPosts          = `SELECT COUNT(*) FROM notes`
	sqlCountActiveUsersMonth    = `SELECT COUNT(DISTINCT user_id) FROM notes WHERE created_at >= datetime('now', '-30 days')`
	sqlCountActiveUsersHalfYear = `SELECT COUNT(DISTINCT user_id) FROM notes WHERE created_at >= datetime('now', '-180 days')`
)

// Outbox collection query - returns public notes for ActivityPub outbox
const (
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

func (db *DB) updateLoginUser(tx *sql.Tx, username string, displayName string, summary string, pkHash string) error {
	_, err := tx.Exec(sqlUpdateLoginUser, username, displayName, summary, pkHash)
	return err
}

func (db *DB) updateLoginUserById(tx *sql.Tx, username string, displayName string, summary string, id uuid.UUID) error {
	_, err := tx.Exec(sqlUpdateLoginUserById, username, displayName, summary, id)
	return err
}

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
