// notifications.go — User notification CRUD operations.
// Principle: One File, One Mental Model — notification lifecycle in one place.

package db

import (
	"database/sql"
	"log"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

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
