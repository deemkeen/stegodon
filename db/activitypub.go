// activitypub.go — Remote accounts, follows, activities, and federation queries.
// Principle: One File, One Mental Model — all ActivityPub data persistence in one place.

package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

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
	err := db.wrapTransaction(func(tx *sql.Tx) error {
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
	if err == nil && activity.ActivityType == "Create" && !activity.Local {
		// Index in FTS (best-effort)
		db.InsertActivityFTS(
			activity.Id,
			activity.ActorURI,
			activity.RawJSON,
			activity.CreatedAt.Format("2006-01-02 15:04:05"),
			activity.ObjectURI,
			activity.ObjectURL,
		)
	}
	return err
}

func (db *DB) UpdateActivity(activity *domain.Activity) error {
	err := db.wrapTransaction(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlUpdateActivity,
			activity.RawJSON,
			activity.Processed,
			activity.ObjectURI,
			activity.Id.String(),
		)
		return err
	})
	if err == nil && activity.ActivityType == "Create" {
		// Update FTS index (best-effort): delete old + insert new
		db.DeleteFromFTS(activity.Id.String())
		db.InsertActivityFTS(
			activity.Id,
			activity.ActorURI,
			activity.RawJSON,
			activity.CreatedAt.Format("2006-01-02 15:04:05"),
			activity.ObjectURI,
			activity.ObjectURL,
		)
	}
	return err
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

// ReadActivityById reads an activity by its UUID
func (db *DB) ReadActivityById(id uuid.UUID) (error, *domain.Activity) {
	row := db.db.QueryRow(
		`SELECT id, activity_uri, activity_type, actor_uri, object_uri, COALESCE(object_url, ''), raw_json, processed, local, created_at, COALESCE(like_count, 0), COALESCE(boost_count, 0)
		FROM activities WHERE id = ?`,
		id.String(),
	)
	var activity domain.Activity
	var idStr string
	var createdAtStr string
	err := row.Scan(
		&idStr,
		&activity.ActivityURI,
		&activity.ActivityType,
		&activity.ActorURI,
		&activity.ObjectURI,
		&activity.ObjectURL,
		&activity.RawJSON,
		&activity.Processed,
		&activity.Local,
		&createdAtStr,
		&activity.LikeCount,
		&activity.BoostCount,
	)
	if err == sql.ErrNoRows {
		return err, nil
	}
	if err != nil {
		return err, nil
	}
	activity.Id, _ = uuid.Parse(idStr)
	if parsedTime, parseErr := parseTimestamp(createdAtStr); parseErr == nil {
		activity.CreatedAt = parsedTime
	}
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

// DeleteActivity deletes an activity by ID
func (db *DB) DeleteActivity(id uuid.UUID) error {
	// Delete from FTS before deleting from DB (need row data for standalone FTS delete)
	db.DeleteFromFTS(id.String())

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

// ReadActivitiesByActorURI returns top-level posts (Create activities) from a specific remote actor
// Used to display posts on a remote user's profile
func (db *DB) ReadActivitiesByActorURI(actorURI string, limit int) (error, *[]domain.Activity) {
	rows, err := db.db.Query(`
		SELECT id, activity_uri, activity_type, actor_uri, object_uri, COALESCE(object_url, ''), raw_json, processed, local, created_at, COALESCE(like_count, 0), COALESCE(boost_count, 0)
		FROM activities
		WHERE actor_uri = ? AND activity_type = 'Create' AND local = 0
		AND (in_reply_to IS NULL OR in_reply_to = '')
		ORDER BY created_at DESC LIMIT ?`,
		actorURI, limit)
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
