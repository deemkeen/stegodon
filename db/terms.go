// terms.go — Terms and conditions management and user acceptance tracking.
// Principle: One File, One Mental Model — terms lifecycle in one place.
package db

import (
	"database/sql"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

const (
	sqlInsertTermsAndConditions  = `INSERT OR REPLACE INTO terms_and_conditions(id, content, updated_at) VALUES (1, ?, ?)`
	sqlSelectTermsAndConditions  = `SELECT id, content, updated_at FROM terms_and_conditions WHERE id = 1`
	sqlInsertUserTermsAcceptance = `INSERT INTO user_terms_acceptance(user_id, terms_id, accepted_at) VALUES (?, 1, ?)`
	sqlSelectUserTermsAcceptance = `SELECT id, user_id, terms_id, accepted_at FROM user_terms_acceptance WHERE user_id = ?`
)

const (
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
			Id:        1,
			Content:   "Default Terms and Conditions",
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

func (db *DB) createTermsAndConditionsTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateTermsAndConditionsTable)
	return err
}

func (db *DB) createUserTermsAcceptanceTable(tx *sql.Tx) error {
	_, err := tx.Exec(sqlCreateUserTermsAcceptanceTable)
	return err
}
