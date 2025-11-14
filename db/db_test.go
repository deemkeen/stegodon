package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *DB {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	db := &DB{db: sqlDB}

	// Create tables
	if _, err := db.db.Exec(sqlCreateUserTable); err != nil {
		t.Fatalf("Failed to create accounts table: %v", err)
	}

	if _, err := db.db.Exec(sqlCreateNotesTable); err != nil {
		t.Fatalf("Failed to create notes table: %v", err)
	}

	// Add edited_at column which might be missing from base table
	db.db.Exec(`ALTER TABLE notes ADD COLUMN edited_at timestamp`)

	// Add ActivityPub profile fields to accounts table
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN display_name varchar(255)`)
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN summary text`)
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN avatar_url text`)

	// Create ActivityPub tables
	db.db.Exec(`CREATE TABLE IF NOT EXISTS remote_accounts(
		id uuid NOT NULL PRIMARY KEY,
		username varchar(100) NOT NULL,
		domain varchar(255) NOT NULL,
		actor_uri varchar(500) UNIQUE NOT NULL,
		display_name varchar(255),
		summary text,
		inbox_uri varchar(500),
		outbox_uri varchar(500),
		public_key_pem text,
		avatar_url varchar(500),
		last_fetched_at timestamp default current_timestamp,
		UNIQUE(username, domain)
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS follows(
		id uuid NOT NULL PRIMARY KEY,
		account_id uuid NOT NULL,
		target_account_id uuid NOT NULL,
		uri varchar(500),
		created_at timestamp default current_timestamp,
		accepted int default 0,
		is_local int default 0,
		UNIQUE(account_id, target_account_id)
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS activities(
		id uuid NOT NULL PRIMARY KEY,
		activity_uri varchar(500) UNIQUE NOT NULL,
		activity_type varchar(50) NOT NULL,
		actor_uri varchar(500),
		object_uri varchar(500),
		raw_json text,
		processed int default 0,
		created_at timestamp default current_timestamp,
		local int default 0
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS likes(
		id uuid NOT NULL PRIMARY KEY,
		account_id uuid NOT NULL,
		note_id uuid NOT NULL,
		uri varchar(500),
		created_at timestamp default current_timestamp,
		UNIQUE(account_id, note_id)
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS delivery_queue(
		id uuid NOT NULL PRIMARY KEY,
		inbox_uri varchar(500) NOT NULL,
		activity_json text NOT NULL,
		attempts int default 0,
		next_retry_at timestamp default current_timestamp,
		created_at timestamp default current_timestamp
	)`)

	return db
}

// createTestAccount is a helper to create accounts directly via SQL
func createTestAccount(t *testing.T, db *DB, id uuid.UUID, username, pubkey, webPubKey, webPrivKey string) {
	_, err := db.db.Exec(sqlInsertUser, id, username, pubkey, webPubKey, webPrivKey, time.Now())
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}
}

func TestReadAccById(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create account directly
	id := uuid.New()
	username := "testuser"
	pubkey := "ssh-rsa AAAAB3..."
	createTestAccount(t, db, id, username, pubkey, "webpub", "webpriv")

	// Read account
	err, acc := db.ReadAccById(id)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}

	if acc.Id != id {
		t.Errorf("Expected Id %s, got %s", id, acc.Id)
	}
	if acc.Username != username {
		t.Errorf("Expected Username %s, got %s", username, acc.Username)
	}
	if acc.Publickey != pubkey {
		t.Errorf("Expected Publickey %s, got %s", pubkey, acc.Publickey)
	}
}

func TestReadAccByIdNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Test non-existent account
	randomId := uuid.New()
	err, acc := db.ReadAccById(randomId)
	if err == nil {
		t.Error("Expected error for non-existent account")
	}
	if acc != nil {
		t.Error("Expected nil account for non-existent ID")
	}
}

func TestReadAccByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create account
	id := uuid.New()
	username := "alice"
	createTestAccount(t, db, id, username, "pubkey", "webpub", "webpriv")

	// Read by username
	err, acc := db.ReadAccByUsername(username)
	if err != nil {
		t.Fatalf("ReadAccByUsername failed: %v", err)
	}

	if acc.Username != username {
		t.Errorf("Expected username %s, got %s", username, acc.Username)
	}
	if acc.Id != id {
		t.Errorf("Expected ID %s, got %s", id, acc.Id)
	}
}

func TestReadAccByUsernameNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	err, acc := db.ReadAccByUsername("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent username")
	}
	if acc != nil {
		t.Error("Expected nil account")
	}
}

func TestUpdateLoginById(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	id := uuid.New()
	oldUsername := "oldname"
	newUsername := "newname"

	// Create account
	createTestAccount(t, db, id, oldUsername, "pubkey", "webpub", "webpriv")

	// Update username
	err := db.UpdateLoginById(newUsername, "Alice Test", "Test bio", id)
	if err != nil {
		t.Fatalf("UpdateLoginById failed: %v", err)
	}

	// Verify update
	err, acc := db.ReadAccById(id)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}

	if acc.Username != newUsername {
		t.Errorf("Expected username %s, got %s", newUsername, acc.Username)
	}
	if acc.FirstTimeLogin != domain.FALSE {
		t.Error("Expected FirstTimeLogin to be FALSE after update")
	}
}

func TestCreateNote(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user first
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create note
	message := "Test message"
	noteId, err := db.CreateNote(userId, message)
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	if noteId == uuid.Nil {
		t.Error("Expected valid note ID")
	}

	// Verify note exists
	err, note := db.ReadNoteId(noteId)
	if err != nil {
		t.Fatalf("ReadNoteId failed: %v", err)
	}

	if note.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, note.Message)
	}
	if note.CreatedBy != "testuser" {
		t.Errorf("Expected CreatedBy 'testuser', got '%s'", note.CreatedBy)
	}
}

func TestReadNoteIdNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Test non-existent note
	randomId := uuid.New()
	err, note := db.ReadNoteId(randomId)
	if err == nil {
		t.Error("Expected error for non-existent note")
	}
	if note != nil {
		t.Error("Expected nil note")
	}
}

func TestReadNotesByUserId(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create multiple notes
	for i := 0; i < 3; i++ {
		_, err := db.CreateNote(userId, "Test message")
		if err != nil {
			t.Fatalf("Failed to create note %d: %v", i, err)
		}
	}

	// Read notes
	err, notes := db.ReadNotesByUserId(userId)
	if err != nil {
		t.Fatalf("ReadNotesByUserId failed: %v", err)
	}

	if len(*notes) != 3 {
		t.Errorf("Expected 3 notes, got %d", len(*notes))
	}
}

func TestReadNotesByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	username := "alice"
	userId := uuid.New()

	// Create user
	createTestAccount(t, db, userId, username, "pubkey", "webpub", "webpriv")

	// Create note
	db.CreateNote(userId, "Alice's note")

	// Read notes by username
	err, notes := db.ReadNotesByUsername(username)
	if err != nil {
		t.Fatalf("ReadNotesByUsername failed: %v", err)
	}

	if len(*notes) == 0 {
		t.Error("Expected at least one note")
	}

	if (*notes)[0].CreatedBy != username {
		t.Errorf("Expected CreatedBy '%s', got '%s'", username, (*notes)[0].CreatedBy)
	}
}

func TestReadAllNotes(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create two users
	user1Id := uuid.New()
	user2Id := uuid.New()
	createTestAccount(t, db, user1Id, "user1", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, user2Id, "user2", "pubkey2", "webpub2", "webpriv2")

	// Create notes for both users
	db.CreateNote(user1Id, "User1 note")
	db.CreateNote(user2Id, "User2 note")

	// Read all notes
	err, notes := db.ReadAllNotes()
	if err != nil {
		t.Fatalf("ReadAllNotes failed: %v", err)
	}

	if len(*notes) < 2 {
		t.Errorf("Expected at least 2 notes, got %d", len(*notes))
	}
}

func TestUpdateNote(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	noteId, err := db.CreateNote(userId, "Original message")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Update note
	newMessage := "Updated message"
	err = db.UpdateNote(noteId, newMessage)
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}

	// Verify update
	err, note := db.ReadNoteId(noteId)
	if err != nil {
		t.Fatalf("ReadNoteId failed: %v", err)
	}

	if note.Message != newMessage {
		t.Errorf("Expected message '%s', got '%s'", newMessage, note.Message)
	}

	if note.EditedAt == nil {
		t.Error("Expected EditedAt to be set after update")
	}
}

func TestDeleteNoteById(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	noteId, err := db.CreateNote(userId, "To be deleted")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Delete note
	err = db.DeleteNoteById(noteId)
	if err != nil {
		t.Fatalf("DeleteNoteById failed: %v", err)
	}

	// Verify deletion
	err, note := db.ReadNoteId(noteId)
	if err == nil {
		t.Error("Expected error when reading deleted note")
	}
	if note != nil {
		t.Error("Expected nil note after deletion")
	}
}

func TestReadAllAccounts(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create accounts
	user1Id := uuid.New()
	user2Id := uuid.New()

	createTestAccount(t, db, user1Id, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, user2Id, "bob", "pubkey2", "webpub2", "webpriv2")

	// Update to set first_time_login = 0
	db.UpdateLoginById("alice", "Alice", "Alice's bio", user1Id)
	db.UpdateLoginById("bob", "Bob", "Bob's bio", user2Id)

	// Read all accounts
	err, accounts := db.ReadAllAccounts()
	if err != nil {
		t.Fatalf("ReadAllAccounts failed: %v", err)
	}

	if len(*accounts) < 2 {
		t.Errorf("Expected at least 2 accounts, got %d", len(*accounts))
	}
}

func TestNoteTimestamps(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	noteId, err := db.CreateNote(userId, "Timestamp test")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Verify timestamp
	err, note := db.ReadNoteId(noteId)
	if err != nil {
		t.Fatalf("ReadNoteId failed: %v", err)
	}

	// Just verify that CreatedAt is set (not zero)
	if note.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if note.EditedAt != nil {
		t.Error("EditedAt should be nil for new note")
	}
}

func TestAccountFirstTimeLogin(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	id := uuid.New()
	username := "newuser"

	// Create account
	createTestAccount(t, db, id, username, "pubkey", "webpub", "webpriv")

	// Check initial state
	err, acc := db.ReadAccById(id)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}

	if acc.FirstTimeLogin != domain.TRUE {
		t.Error("Expected FirstTimeLogin to be TRUE for new account")
	}

	// Update username (which sets FirstTimeLogin to FALSE)
	err = db.UpdateLoginById("updateduser", "Updated User", "Updated bio", id)
	if err != nil {
		t.Fatalf("UpdateLoginById failed: %v", err)
	}

	// Verify FirstTimeLogin is now FALSE
	err, acc = db.ReadAccById(id)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}

	if acc.FirstTimeLogin != domain.FALSE {
		t.Error("Expected FirstTimeLogin to be FALSE after update")
	}
}

// ActivityPub-related tests

func TestCreateRemoteAccount(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	remoteAcc := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "example.com",
		ActorURI:      "https://example.com/users/bob",
		DisplayName:   "Bob Smith",
		Summary:       "Test user",
		InboxURI:      "https://example.com/users/bob/inbox",
		OutboxURI:     "https://example.com/users/bob/outbox",
		PublicKeyPem:  "-----BEGIN PUBLIC KEY-----",
		AvatarURL:     "https://example.com/avatar.png",
		LastFetchedAt: time.Now(),
	}

	err := db.CreateRemoteAccount(remoteAcc)
	if err != nil {
		t.Fatalf("CreateRemoteAccount failed: %v", err)
	}

	// Verify
	err, acc := db.ReadRemoteAccountByURI(remoteAcc.ActorURI)
	if err != nil {
		t.Fatalf("ReadRemoteAccountByURI failed: %v", err)
	}

	if acc.Username != remoteAcc.Username {
		t.Errorf("Expected username %s, got %s", remoteAcc.Username, acc.Username)
	}
}

func TestCreateLocalFollow(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create two local accounts
	follower := uuid.New()
	target := uuid.New()
	createTestAccount(t, db, follower, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, target, "bob", "pubkey2", "webpub2", "webpriv2")

	// Create local follow
	err := db.CreateLocalFollow(follower, target)
	if err != nil {
		t.Fatalf("CreateLocalFollow failed: %v", err)
	}

	// Verify follow exists
	isFollowing, err := db.IsFollowingLocal(follower, target)
	if err != nil {
		t.Fatalf("IsFollowingLocal failed: %v", err)
	}

	if !isFollowing {
		t.Error("Expected isFollowing to be true")
	}
}

func TestDeleteLocalFollow(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create two local accounts
	follower := uuid.New()
	target := uuid.New()
	createTestAccount(t, db, follower, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, target, "bob", "pubkey2", "webpub2", "webpriv2")

	// Create and then delete follow
	db.CreateLocalFollow(follower, target)

	err := db.DeleteLocalFollow(follower, target)
	if err != nil {
		t.Fatalf("DeleteLocalFollow failed: %v", err)
	}

	// Verify follow doesn't exist
	isFollowing, err := db.IsFollowingLocal(follower, target)
	if err != nil {
		t.Fatalf("IsFollowingLocal failed: %v", err)
	}

	if isFollowing {
		t.Error("Expected isFollowing to be false after deletion")
	}
}

func TestCreateActivity(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://example.com/activities/123",
		ActivityType: "Create",
		ActorURI:     "https://example.com/users/bob",
		ObjectURI:    "https://example.com/notes/456",
		RawJSON:      `{"type":"Create"}`,
		Processed:    false,
		CreatedAt:    time.Now(),
		Local:        false,
	}

	err := db.CreateActivity(activity)
	if err != nil {
		t.Fatalf("CreateActivity failed: %v", err)
	}

	// Verify
	err, act := db.ReadActivityByURI(activity.ActivityURI)
	if err != nil {
		t.Fatalf("ReadActivityByURI failed: %v", err)
	}

	if act.ActivityType != activity.ActivityType {
		t.Errorf("Expected ActivityType %s, got %s", activity.ActivityType, act.ActivityType)
	}
}

func TestReadLocalTimelineNotes(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and notes
	userId := uuid.New()
	createTestAccount(t, db, userId, "alice", "pubkey", "webpub", "webpriv")

	// Create some notes
	for i := 0; i < 5; i++ {
		db.CreateNote(userId, "Note "+string(rune('A'+i)))
	}

	// Read timeline with limit
	err, notes := db.ReadLocalTimelineNotes(userId, 3)
	if err != nil {
		t.Fatalf("ReadLocalTimelineNotes failed: %v", err)
	}

	if len(*notes) != 3 {
		t.Errorf("Expected 3 notes (limited), got %d", len(*notes))
	}
}
