package db

import (
	"context"
	"database/sql"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/gliderlabs/ssh"
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
	sqlSelectNoteById = `SELECT notes.id, accounts.username, notes.message, notes.created_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id 
                                                            WHERE notes.id = ?`
	sqlSelectNotesByUserId = `SELECT notes.id, accounts.username, notes.message, notes.created_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id 
                                                            WHERE notes.user_id = ?
                                                            ORDER BY notes.created_at DESC`
	sqlSelectNotesByUsername = `SELECT notes.id, accounts.username, notes.message, notes.created_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id 
                                                            WHERE accounts.username = ?
                                                            ORDER BY notes.created_at DESC`
	sqlSelectAllNotes = `SELECT notes.id, accounts.username, notes.message, notes.created_at FROM notes
    														INNER JOIN accounts ON accounts.id = notes.user_id 
                                                            ORDER BY notes.created_at DESC`
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

func (db *DB) CreateNote(userId uuid.UUID, message string) error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		err := db.insertNote(tx, userId, message)
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

func (db *DB) ReadNotesByUserId(userId uuid.UUID) (error, *[]domain.Note) {
	rows, err := db.db.Query(sqlSelectNotesByUserId, userId)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	var notes []domain.Note

	for rows.Next() {
		var note domain.Note
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &note.CreatedAt); err != nil {
			return err, &notes
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
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &note.CreatedAt); err != nil {
			return err, &notes
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
	err := row.Scan(&note.Id, &note.CreatedBy, &note.Message, &note.CreatedAt)
	if err == sql.ErrNoRows {
		return err, nil
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
		if err := rows.Scan(&note.Id, &note.CreatedBy, &note.Message, &note.CreatedAt); err != nil {
			return err, &notes
		}
		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return err, &notes
	}

	return nil, &notes
}

func GetDB() *DB {
	//TODO optimize db access
	var err error
	db, err := sql.Open("sqlite", "database.db")
	if err != nil {
		panic(err)
	}

	log.Printf("new db operation")

	d := &DB{db: db}

	err2 := d.CreateDB()
	if err2 != nil {
		panic(err2)
	}

	return d
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

func (db *DB) insertNote(tx *sql.Tx, userId uuid.UUID, message string) error {
	_, err := tx.Exec(sqlInsertNote, uuid.New(), userId, message, time.Now())
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
