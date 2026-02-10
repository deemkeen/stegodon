// admin.go — Admin panel operations: info boxes, server messages, and bans.
// Principle: One File, One Mental Model — all admin management operations in one place.
package db

import (
	"database/sql"
	"log"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

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
// Ban Management
// ============================================================================

const (
	sqlCreateBan   = `INSERT INTO bans(id, username, ip_address, public_key_hash, reason, banned_at) VALUES (?, ?, ?, ?, ?, ?)`
	sqlReadAllBans = `SELECT id, username, ip_address, public_key_hash, reason, banned_at FROM bans ORDER BY banned_at DESC`
	sqlDeleteBan   = `DELETE FROM bans WHERE id = ?`
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
