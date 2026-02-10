// delivery.go — ActivityPub delivery queue operations.
// Principle: One File, One Mental Model — delivery lifecycle from enqueue to completion.

package db

import (
	"database/sql"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

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
