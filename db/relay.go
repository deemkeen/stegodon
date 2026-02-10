// relay.go — ActivityPub relay subscription management.
// Principle: One File, One Mental Model — relay CRUD and content cleanup in one place.
package db

import (
	"database/sql"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

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
	// Clean up FTS index before deleting relay activities
	db.DeleteRelayActivitiesFTS()

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
