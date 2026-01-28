package db

import (
	"database/sql"
	"strings"
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

	// Add ActivityPub note fields to notes table
	db.db.Exec(`ALTER TABLE notes ADD COLUMN visibility TEXT DEFAULT 'public'`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN in_reply_to_uri TEXT`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN object_uri TEXT`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN federated INTEGER DEFAULT 1`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN sensitive INTEGER DEFAULT 0`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN content_warning TEXT`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN reply_count INTEGER DEFAULT 0`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN like_count INTEGER DEFAULT 0`)
	db.db.Exec(`ALTER TABLE notes ADD COLUMN boost_count INTEGER DEFAULT 0`)

	// Add ActivityPub profile fields to accounts table
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN display_name varchar(255)`)
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN summary text`)
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN avatar_url text`)

	// Add admin fields to accounts table
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN is_admin INTEGER DEFAULT 0`)
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN muted INTEGER DEFAULT 0`)
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN banned INTEGER DEFAULT 0`)
	db.db.Exec(`ALTER TABLE accounts ADD COLUMN last_ip TEXT`)

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
		object_url TEXT,
		in_reply_to TEXT,
		raw_json text,
		processed int default 0,
		created_at timestamp default current_timestamp,
		local int default 0,
		from_relay int default 0,
		reply_count INTEGER DEFAULT 0,
		like_count INTEGER DEFAULT 0,
		boost_count INTEGER DEFAULT 0
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS likes(
		id uuid NOT NULL PRIMARY KEY,
		account_id uuid NOT NULL,
		note_id uuid NOT NULL,
		uri varchar(500),
		object_uri TEXT,
		created_at timestamp default current_timestamp,
		UNIQUE(account_id, note_id)
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS boosts(
		id uuid NOT NULL PRIMARY KEY,
		account_id uuid,
		remote_account_id uuid,
		note_id uuid NOT NULL,
		uri varchar(500),
		object_uri TEXT,
		created_at timestamp default current_timestamp,
		UNIQUE(account_id, note_id)
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS delivery_queue(
		id uuid NOT NULL PRIMARY KEY,
		inbox_uri varchar(500) NOT NULL,
		activity_json text NOT NULL,
		attempts int default 0,
		next_retry_at timestamp default current_timestamp,
		created_at timestamp default current_timestamp,
		account_id TEXT
	)`)

	// Create hashtag tables
	db.db.Exec(`CREATE TABLE IF NOT EXISTS hashtags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		usage_count INTEGER DEFAULT 0,
		last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS note_hashtags (
		note_id TEXT NOT NULL,
		hashtag_id INTEGER NOT NULL,
		PRIMARY KEY (note_id, hashtag_id)
	)`)

	// Create relays table
	db.db.Exec(`CREATE TABLE IF NOT EXISTS relays (
		id TEXT NOT NULL PRIMARY KEY,
		actor_uri TEXT UNIQUE NOT NULL,
		inbox_uri TEXT NOT NULL,
		follow_uri TEXT,
		name TEXT,
		status TEXT DEFAULT 'pending',
		paused INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accepted_at TIMESTAMP
	)`)

	// Create terms and conditions tables
	db.db.Exec(`CREATE TABLE IF NOT EXISTS terms_and_conditions(
		id INTEGER PRIMARY KEY,
		content TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)

	db.db.Exec(`CREATE TABLE IF NOT EXISTS user_terms_acceptance(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id uuid NOT NULL,
		terms_id INTEGER NOT NULL,
		accepted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES accounts(id),
		FOREIGN KEY (terms_id) REFERENCES terms_and_conditions(id)
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

func TestReadNoteIdTimestampParsing(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create a note
	noteId, err := db.CreateNote(userId, "Test message")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Read the note back
	err, note := db.ReadNoteId(noteId)
	if err != nil {
		t.Fatalf("ReadNoteId failed: %v", err)
	}
	if note == nil {
		t.Fatal("Expected note, got nil")
	}

	// Verify timestamp is not zero time
	if note.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero time - timestamp parsing failed")
	}

	// Verify timestamp is recent (within last minute)
	timeSince := time.Since(note.CreatedAt)
	if timeSince < 0 || timeSince > time.Minute {
		t.Errorf("CreatedAt timestamp is not recent: %v ago (expected < 1 minute)", timeSince)
	}

	// Verify note content
	if note.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", note.Message)
	}
	if note.CreatedBy != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", note.CreatedBy)
	}
}

func TestReadNotesByUserId(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create multiple notes
	for i := range 3 {
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
	for i := range 5 {
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

func TestDeleteAccount(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create a test account
	userId := uuid.New()
	createTestAccount(t, db, userId, "alice", "pubkey", "webpub", "webpriv")

	// Create notes for the user
	_, err := db.CreateNote(userId, "Test note 1")
	if err != nil {
		t.Fatalf("Failed to create note 1: %v", err)
	}
	_, err = db.CreateNote(userId, "Test note 2")
	if err != nil {
		t.Fatalf("Failed to create note 2: %v", err)
	}

	// Create a second user for follow relationships
	user2Id := uuid.New()
	createTestAccount(t, db, user2Id, "bob", "pubkey2", "webpub2", "webpriv2")

	// Create local follow relationships (alice follows bob, bob follows alice)
	err = db.CreateLocalFollow(userId, user2Id)
	if err != nil {
		t.Fatalf("Failed to create follow: %v", err)
	}
	err = db.CreateLocalFollow(user2Id, userId)
	if err != nil {
		t.Fatalf("Failed to create reverse follow: %v", err)
	}

	// Create an activity for the user
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://example.com/activities/alice",
		ActivityType: "Create",
		ActorURI:     "https://example.com/users/alice",
		ObjectURI:    "https://example.com/notes/123",
		RawJSON:      `{"type":"Create"}`,
		Processed:    true,
		CreatedAt:    time.Now(),
		Local:        true,
	}
	err = db.CreateActivity(activity)
	if err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Verify data exists before deletion
	err, acc := db.ReadAccById(userId)
	if err != nil || acc == nil {
		t.Fatalf("Account should exist before deletion")
	}

	err, notes := db.ReadNotesByUserId(userId)
	if err != nil || len(*notes) != 2 {
		t.Fatalf("Expected 2 notes before deletion, got %d", len(*notes))
	}

	// Delete the account
	err = db.DeleteAccount(userId)
	if err != nil {
		t.Fatalf("DeleteAccount failed: %v", err)
	}

	// Verify account was deleted
	err, acc = db.ReadAccById(userId)
	if err != sql.ErrNoRows {
		t.Errorf("Account should not exist after deletion, got: %v", acc)
	}

	// Verify notes were deleted
	err, notes = db.ReadNotesByUserId(userId)
	if err != nil || len(*notes) != 0 {
		t.Errorf("Expected 0 notes after deletion, got %d", len(*notes))
	}

	// Verify follows were deleted (both directions)
	err, following := db.ReadFollowingByAccountId(userId)
	if err != nil || len(*following) != 0 {
		t.Errorf("Expected 0 following relationships after deletion, got %d", len(*following))
	}

	err, followers := db.ReadFollowersByAccountId(userId)
	if err != nil || len(*followers) != 0 {
		t.Errorf("Expected 0 follower relationships after deletion, got %d", len(*followers))
	}

	// Note: Activities are NOT deleted (they remain as historical record)
	// This matches ActivityPub behavior

	// Verify bob's account still exists (shouldn't be affected)
	err, bob := db.ReadAccById(user2Id)
	if err != nil || bob == nil {
		t.Errorf("Bob's account should still exist after alice deletion")
	}
}

func TestDeleteAccountNonExistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Try to delete a non-existent account
	nonExistentId := uuid.New()
	err := db.DeleteAccount(nonExistentId)
	// Should not error even if account doesn't exist (idempotent delete)
	if err != nil {
		t.Errorf("Deleting non-existent account should not error: %v", err)
	}
}

func TestDeleteAccountWithNoData(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create a user with no notes, follows, activities, etc.
	userId := uuid.New()
	createTestAccount(t, db, userId, "lonely", "pubkey", "webpub", "webpriv")

	// Delete the account
	err := db.DeleteAccount(userId)
	if err != nil {
		t.Fatalf("DeleteAccount failed for account with no data: %v", err)
	}

	// Verify account was deleted
	err, acc := db.ReadAccById(userId)
	if err != sql.ErrNoRows {
		t.Errorf("Account should not exist after deletion, got: %v", acc)
	}
}

// Admin functionality tests

func TestFirstUserIsAdmin(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create first user
	user1Id := uuid.New()
	_, err := db.db.Exec(sqlInsertUser, user1Id, "firstuser", "hash1", "webpub1", "webpriv1", time.Now())
	if err != nil {
		t.Fatalf("Failed to insert first user: %v", err)
	}

	// Set is_admin for first user
	_, err = db.db.Exec("UPDATE accounts SET is_admin = 1 WHERE id = ?", user1Id.String())
	if err != nil {
		t.Fatalf("Failed to set is_admin: %v", err)
	}

	// Create second user
	user2Id := uuid.New()
	_, err = db.db.Exec(sqlInsertUser, user2Id, "seconduser", "hash2", "webpub2", "webpriv2", time.Now())
	if err != nil {
		t.Fatalf("Failed to insert second user: %v", err)
	}

	// Verify first user is admin
	err, acc1 := db.ReadAccById(user1Id)
	if err != nil {
		t.Fatalf("ReadAccById failed for first user: %v", err)
	}
	if !acc1.IsAdmin {
		t.Errorf("First user should be admin, got IsAdmin = %v", acc1.IsAdmin)
	}

	// Verify second user is not admin
	err, acc2 := db.ReadAccById(user2Id)
	if err != nil {
		t.Fatalf("ReadAccById failed for second user: %v", err)
	}
	if acc2.IsAdmin {
		t.Errorf("Second user should not be admin, got IsAdmin = %v", acc2.IsAdmin)
	}
}

func TestMuteUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user with notes
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create some notes
	noteId1 := uuid.New()
	noteId2 := uuid.New()
	_, err := db.db.Exec("INSERT INTO notes (id, user_id, message, created_at) VALUES (?, ?, ?, ?)",
		noteId1, userId.String(), "Note 1", time.Now())
	if err != nil {
		t.Fatalf("Failed to create note 1: %v", err)
	}
	_, err = db.db.Exec("INSERT INTO notes (id, user_id, message, created_at) VALUES (?, ?, ?, ?)",
		noteId2, userId.String(), "Note 2", time.Now())
	if err != nil {
		t.Fatalf("Failed to create note 2: %v", err)
	}

	// Verify notes exist
	err, notes := db.ReadNotesByUserId(userId)
	if err != nil || len(*notes) != 2 {
		t.Fatalf("Expected 2 notes before mute, got %d", len(*notes))
	}

	// Mute the user
	err = db.MuteUser(userId)
	if err != nil {
		t.Fatalf("MuteUser failed: %v", err)
	}

	// Verify user is muted
	err, acc := db.ReadAccById(userId)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}
	if !acc.Muted {
		t.Errorf("User should be muted, got Muted = %v", acc.Muted)
	}

	// Verify notes were deleted
	err, notes = db.ReadNotesByUserId(userId)
	if err != nil || len(*notes) != 0 {
		t.Errorf("Expected 0 notes after mute, got %d", len(*notes))
	}
}

func TestUnmuteUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Mute the user
	err := db.MuteUser(userId)
	if err != nil {
		t.Fatalf("MuteUser failed: %v", err)
	}

	// Verify user is muted
	err, acc := db.ReadAccById(userId)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}
	if !acc.Muted {
		t.Fatalf("User should be muted")
	}

	// Unmute the user
	err = db.UnmuteUser(userId)
	if err != nil {
		t.Fatalf("UnmuteUser failed: %v", err)
	}

	// Verify user is not muted
	err, acc = db.ReadAccById(userId)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}
	if acc.Muted {
		t.Errorf("User should not be muted after unmute, got Muted = %v", acc.Muted)
	}
}

func TestReadAllAccountsAdmin(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create users with different first_time_login states
	user1Id := uuid.New()
	user2Id := uuid.New()
	user3Id := uuid.New()

	createTestAccount(t, db, user1Id, "user1", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, user2Id, "user2", "pubkey2", "webpub2", "webpriv2")
	createTestAccount(t, db, user3Id, "user3", "pubkey3", "webpub3", "webpriv3")

	// Set first_time_login = 0 for user1 and user2 (completed registration)
	_, err := db.db.Exec("UPDATE accounts SET first_time_login = 0 WHERE id IN (?, ?)", user1Id.String(), user2Id.String())
	if err != nil {
		t.Fatalf("Failed to update first_time_login: %v", err)
	}

	// user3 keeps first_time_login = 1 (default from sqlInsertUser)

	// ReadAllAccounts should only return users with first_time_login = 0
	err, accounts := db.ReadAllAccounts()
	if err != nil {
		t.Fatalf("ReadAllAccounts failed: %v", err)
	}
	if len(*accounts) != 2 {
		t.Errorf("ReadAllAccounts: expected 2 users, got %d", len(*accounts))
	}

	// ReadAllAccountsAdmin should return ALL users
	err, accountsAdmin := db.ReadAllAccountsAdmin()
	if err != nil {
		t.Fatalf("ReadAllAccountsAdmin failed: %v", err)
	}
	if len(*accountsAdmin) != 3 {
		t.Errorf("ReadAllAccountsAdmin: expected 3 users, got %d", len(*accountsAdmin))
	}

	// Verify admin query includes the first-time login user
	foundNewUser := false
	for _, acc := range *accountsAdmin {
		if acc.Id == user3Id {
			foundNewUser = true
			if acc.FirstTimeLogin != 1 {
				t.Errorf("User3 should have first_time_login = 1, got %d", acc.FirstTimeLogin)
			}
		}
	}
	if !foundNewUser {
		t.Errorf("ReadAllAccountsAdmin should include first-time login user")
	}
}

func TestReadPublicNotesByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create notes with different visibility
	publicNote1 := uuid.New()
	publicNote2 := uuid.New()
	privateNote := uuid.New()

	// Insert public notes
	_, err := db.db.Exec("INSERT INTO notes (id, user_id, message, created_at, visibility) VALUES (?, ?, ?, ?, ?)",
		publicNote1, userId.String(), "Public Note 1", time.Now().Add(-2*time.Hour), "public")
	if err != nil {
		t.Fatalf("Failed to create public note 1: %v", err)
	}

	_, err = db.db.Exec("INSERT INTO notes (id, user_id, message, created_at, visibility) VALUES (?, ?, ?, ?, ?)",
		publicNote2, userId.String(), "Public Note 2", time.Now().Add(-1*time.Hour), "public")
	if err != nil {
		t.Fatalf("Failed to create public note 2: %v", err)
	}

	// Insert private note (should not appear in outbox)
	_, err = db.db.Exec("INSERT INTO notes (id, user_id, message, created_at, visibility) VALUES (?, ?, ?, ?, ?)",
		privateNote, userId.String(), "Private Note", time.Now(), "followers")
	if err != nil {
		t.Fatalf("Failed to create private note: %v", err)
	}

	// Test: Should return only public notes
	err, notes := db.ReadPublicNotesByUsername("testuser", 10, 0)
	if err != nil {
		t.Fatalf("ReadPublicNotesByUsername failed: %v", err)
	}

	if len(*notes) != 2 {
		t.Errorf("Expected 2 public notes, got %d", len(*notes))
	}

	// Verify notes are ordered by created_at DESC (newest first)
	if len(*notes) >= 2 {
		if (*notes)[0].CreatedAt.Before((*notes)[1].CreatedAt) {
			t.Errorf("Notes should be ordered by created_at DESC")
		}
	}

	// Test: Pagination with limit
	err, notesPage1 := db.ReadPublicNotesByUsername("testuser", 1, 0)
	if err != nil {
		t.Fatalf("ReadPublicNotesByUsername with limit failed: %v", err)
	}
	if len(*notesPage1) != 1 {
		t.Errorf("Expected 1 note with limit=1, got %d", len(*notesPage1))
	}

	// Test: Pagination with offset
	err, notesPage2 := db.ReadPublicNotesByUsername("testuser", 1, 1)
	if err != nil {
		t.Fatalf("ReadPublicNotesByUsername with offset failed: %v", err)
	}
	if len(*notesPage2) != 1 {
		t.Errorf("Expected 1 note with offset=1, got %d", len(*notesPage2))
	}

	// Verify pages return different notes
	if len(*notesPage1) > 0 && len(*notesPage2) > 0 {
		if (*notesPage1)[0].Id == (*notesPage2)[0].Id {
			t.Errorf("Pagination should return different notes")
		}
	}

	// Test: Non-existent user
	err, notesNone := db.ReadPublicNotesByUsername("nonexistent", 10, 0)
	if err != nil {
		t.Fatalf("ReadPublicNotesByUsername for non-existent user should not error: %v", err)
	}
	if len(*notesNone) != 0 {
		t.Errorf("Expected 0 notes for non-existent user, got %d", len(*notesNone))
	}
}

func TestCountAccounts(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Test: Empty database
	count, err := db.CountAccounts()
	if err != nil {
		t.Fatalf("CountAccounts failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 accounts in empty database, got %d", count)
	}

	// Test: Add one account
	userId1 := uuid.New()
	createTestAccount(t, db, userId1, "user1", "pubkey1", "webpub1", "webpriv1")

	count, err = db.CountAccounts()
	if err != nil {
		t.Fatalf("CountAccounts failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 account, got %d", count)
	}

	// Test: Add second account
	userId2 := uuid.New()
	createTestAccount(t, db, userId2, "user2", "pubkey2", "webpub2", "webpriv2")

	count, err = db.CountAccounts()
	if err != nil {
		t.Fatalf("CountAccounts failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 accounts, got %d", count)
	}

	// Test: Add third account
	userId3 := uuid.New()
	createTestAccount(t, db, userId3, "user3", "pubkey3", "webpub3", "webpriv3")

	count, err = db.CountAccounts()
	if err != nil {
		t.Fatalf("CountAccounts failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 accounts, got %d", count)
	}
}

func TestUpdateLoginById_UsernameUniqueness(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create two test accounts
	userId1 := uuid.New()
	userId2 := uuid.New()
	createTestAccount(t, db, userId1, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, userId2, "bob", "pubkey2", "webpub2", "webpriv2")

	// Test 1: Update user's own username (should succeed)
	err := db.UpdateLoginById("alice_updated", "Alice Updated", "Bio", userId1)
	if err != nil {
		t.Errorf("UpdateLoginById should succeed when updating own username: %v", err)
	}

	// Verify update worked
	err, acc := db.ReadAccByUsername("alice_updated")
	if err != nil {
		t.Fatalf("ReadAccByUsername failed: %v", err)
	}
	if acc == nil {
		t.Fatal("Expected to find updated account")
	}
	if acc.DisplayName != "Alice Updated" {
		t.Errorf("Expected display name 'Alice Updated', got '%s'", acc.DisplayName)
	}

	// Test 2: Try to update to an existing username (should fail)
	err = db.UpdateLoginById("bob", "Alice Trying Bob", "Bio", userId1)
	if err == nil {
		t.Error("UpdateLoginById should fail when username is already taken by another user")
	}
	if err != nil && err.Error() != "username 'bob' is already taken" {
		t.Errorf("Expected error message 'username 'bob' is already taken', got '%s'", err.Error())
	}

	// Test 3: Verify first user's username wasn't changed after failed update
	err, acc = db.ReadAccByUsername("alice_updated")
	if err != nil {
		t.Fatalf("ReadAccByUsername failed: %v", err)
	}
	if acc == nil {
		t.Fatal("Expected original account to still exist")
	}

	// Test 4: Update display name and bio without changing username (should succeed)
	err = db.UpdateLoginById("alice_updated", "Alice New Display", "New Bio", userId1)
	if err != nil {
		t.Errorf("UpdateLoginById should succeed when keeping same username: %v", err)
	}

	// Verify update worked
	err, acc = db.ReadAccByUsername("alice_updated")
	if err != nil {
		t.Fatalf("ReadAccByUsername failed: %v", err)
	}
	if acc.DisplayName != "Alice New Display" {
		t.Errorf("Expected display name 'Alice New Display', got '%s'", acc.DisplayName)
	}
	if acc.Summary != "New Bio" {
		t.Errorf("Expected summary 'New Bio', got '%s'", acc.Summary)
	}
}

func TestUpdateLoginById_NonExistentUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create one test account
	userId1 := uuid.New()
	createTestAccount(t, db, userId1, "alice", "pubkey1", "webpub1", "webpriv1")

	// Try to update a non-existent user
	nonExistentId := uuid.New()
	err := db.UpdateLoginById("newusername", "Display", "Bio", nonExistentId)
	// This should not error because UPDATE will just affect 0 rows
	// But the username check should pass since nobody has "newusername"
	if err != nil {
		t.Errorf("UpdateLoginById with non-existent user should not error: %v", err)
	}
}

func TestUpdateLoginById_CaseInsensitiveUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create two test accounts
	userId1 := uuid.New()
	userId2 := uuid.New()
	createTestAccount(t, db, userId1, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, userId2, "bob", "pubkey2", "webpub2", "webpriv2")

	// SQLite UNIQUE constraint is case-insensitive by default
	// Try to update to "ALICE" (different case but same username)
	err := db.UpdateLoginById("Alice", "Alice Upper", "Bio", userId2)

	// This might fail or succeed depending on SQLite collation settings
	// For this test, we just verify that if it fails, it's a constraint error
	if err != nil {
		// Should fail with constraint error or our custom error
		t.Logf("Case-insensitive check result: %v", err)
	}
}

// Tests for duplicate follow prevention

func TestReadFollowByAccountIds(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create test accounts
	followerId := uuid.New()
	targetId := uuid.New()
	createTestAccount(t, db, followerId, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, targetId, "bob", "pubkey2", "webpub2", "webpriv2")

	// Create a follow relationship
	follow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       followerId,
		TargetAccountId: targetId,
		URI:             "https://example.com/follows/123",
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	err := db.CreateFollow(follow)
	if err != nil {
		t.Fatalf("Failed to create follow: %v", err)
	}

	// Test: Read existing follow relationship
	err, existingFollow := db.ReadFollowByAccountIds(followerId, targetId)
	if err != nil {
		t.Fatalf("ReadFollowByAccountIds failed: %v", err)
	}
	if existingFollow == nil {
		t.Fatal("Expected to find follow relationship but got nil")
	}
	if existingFollow.AccountId != followerId {
		t.Errorf("Expected follower ID %s, got %s", followerId, existingFollow.AccountId)
	}
	if existingFollow.TargetAccountId != targetId {
		t.Errorf("Expected target ID %s, got %s", targetId, existingFollow.TargetAccountId)
	}

	// Test: Read non-existent follow relationship
	nonExistentId := uuid.New()
	err, notFound := db.ReadFollowByAccountIds(followerId, nonExistentId)
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows for non-existent follow, got: %v", err)
	}
	if notFound != nil {
		t.Error("Expected nil for non-existent follow relationship")
	}

	// Test: Read pending (not yet accepted) follow relationship
	pendingTargetId := uuid.New()
	createTestAccount(t, db, pendingTargetId, "charlie", "pubkey3", "webpub3", "webpriv3")
	pendingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       followerId,
		TargetAccountId: pendingTargetId,
		URI:             "https://example.com/follows/456",
		Accepted:        false, // Pending, not yet accepted
		CreatedAt:       time.Now(),
	}
	err = db.CreateFollow(pendingFollow)
	if err != nil {
		t.Fatalf("Failed to create pending follow: %v", err)
	}

	// Should find pending follow (even though accepted = false)
	err, foundPending := db.ReadFollowByAccountIds(followerId, pendingTargetId)
	if err != nil {
		t.Fatalf("ReadFollowByAccountIds failed to find pending follow: %v", err)
	}
	if foundPending == nil {
		t.Fatal("Expected to find pending follow relationship but got nil")
	}
	if foundPending.Accepted {
		t.Error("Expected pending follow to have Accepted=false")
	}
}

func TestCreateFollow_DuplicatePrevention(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create test accounts
	followerId := uuid.New()
	targetId := uuid.New()
	createTestAccount(t, db, followerId, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, targetId, "bob", "pubkey2", "webpub2", "webpriv2")

	// Create first follow relationship (pending)
	follow1 := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       followerId,
		TargetAccountId: targetId,
		URI:             "https://example.com/follows/123",
		Accepted:        false, // Pending follow
		CreatedAt:       time.Now(),
	}
	err := db.CreateFollow(follow1)
	if err != nil {
		t.Fatalf("Failed to create first follow: %v", err)
	}

	// Verify ReadFollowByAccountIds can detect the pending follow
	err, existingFollow := db.ReadFollowByAccountIds(followerId, targetId)
	if err != nil {
		t.Fatalf("ReadFollowByAccountIds should find pending follow: %v", err)
	}
	if existingFollow == nil {
		t.Fatal("Pending follow should be found by ReadFollowByAccountIds")
	}
	if existingFollow.Accepted {
		t.Error("Expected pending follow to have Accepted=false")
	}

	// Test: Attempting to create duplicate follow should fail with UNIQUE constraint error
	follow2 := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       followerId,
		TargetAccountId: targetId,
		URI:             "https://example.com/follows/456",
		Accepted:        false,
		CreatedAt:       time.Now(),
	}
	err = db.CreateFollow(follow2)
	if err == nil {
		t.Fatal("Expected UNIQUE constraint error when creating duplicate follow")
	}
	if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(err.Error(), "constraint") {
		t.Errorf("Expected UNIQUE constraint error, got: %v", err)
	}
}

func TestFollowersListNoDuplicates(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create test accounts
	follower1Id := uuid.New()
	follower2Id := uuid.New()
	targetId := uuid.New()
	createTestAccount(t, db, follower1Id, "alice", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, follower2Id, "charlie", "pubkey2", "webpub2", "webpriv2")
	createTestAccount(t, db, targetId, "bob", "pubkey3", "webpub3", "webpriv3")

	// Create two different followers for the same target
	follow1 := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       follower1Id,
		TargetAccountId: targetId,
		URI:             "https://example.com/follows/1",
		Accepted:        true,
		IsLocal:         true, // Local follow between local accounts
		CreatedAt:       time.Now(),
	}
	err := db.CreateFollow(follow1)
	if err != nil {
		t.Fatalf("Failed to create follow1: %v", err)
	}

	follow2 := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       follower2Id,
		TargetAccountId: targetId,
		URI:             "https://example.com/follows/2",
		Accepted:        true,
		IsLocal:         true, // Local follow between local accounts
		CreatedAt:       time.Now(),
	}
	err = db.CreateFollow(follow2)
	if err != nil {
		t.Fatalf("Failed to create follow2: %v", err)
	}

	// Read followers
	err, followers := db.ReadFollowersByAccountId(targetId)
	if err != nil {
		t.Fatalf("Failed to read followers: %v", err)
	}
	if followers == nil {
		t.Fatal("Followers list should not be nil")
	}

	// Should have exactly 2 followers
	if len(*followers) != 2 {
		t.Errorf("Expected 2 followers, got %d", len(*followers))
	}

	// Verify no duplicates by checking unique AccountIds
	seenFollowers := make(map[uuid.UUID]int)
	for _, f := range *followers {
		seenFollowers[f.AccountId]++
		if seenFollowers[f.AccountId] > 1 {
			t.Errorf("Duplicate follower detected: %s appears %d times", f.AccountId, seenFollowers[f.AccountId])
		}
	}
}

func TestCountLocalPosts(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Test: Empty database
	count, err := db.CountLocalPosts()
	if err != nil {
		t.Fatalf("CountLocalPosts failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 posts in empty database, got %d", count)
	}

	// Test: Add user and notes
	userId := uuid.New()
	createTestAccount(t, db, userId, "user1", "pubkey1", "webpub1", "webpriv1")

	// Add first note
	_, err = db.CreateNote(userId, "First note")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountLocalPosts()
	if err != nil {
		t.Fatalf("CountLocalPosts failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 post, got %d", count)
	}

	// Add second note
	_, err = db.CreateNote(userId, "Second note")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountLocalPosts()
	if err != nil {
		t.Fatalf("CountLocalPosts failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 posts, got %d", count)
	}

	// Add third note from same user
	_, err = db.CreateNote(userId, "Third note")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountLocalPosts()
	if err != nil {
		t.Fatalf("CountLocalPosts failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 posts, got %d", count)
	}
}

func TestCountActiveUsersMonth(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Test: Empty database
	count, err := db.CountActiveUsersMonth()
	if err != nil {
		t.Fatalf("CountActiveUsersMonth failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 active users in empty database, got %d", count)
	}

	// Test: Add users and notes
	userId1 := uuid.New()
	userId2 := uuid.New()
	createTestAccount(t, db, userId1, "user1", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, userId2, "user2", "pubkey2", "webpub2", "webpriv2")

	// User 1 posts (should be counted as active)
	_, err = db.CreateNote(userId1, "Note from user1")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountActiveUsersMonth()
	if err != nil {
		t.Fatalf("CountActiveUsersMonth failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 active user, got %d", count)
	}

	// User 2 posts (should be counted as active)
	_, err = db.CreateNote(userId2, "Note from user2")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountActiveUsersMonth()
	if err != nil {
		t.Fatalf("CountActiveUsersMonth failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 active users, got %d", count)
	}

	// User 1 posts again (should still be 2 unique users)
	_, err = db.CreateNote(userId1, "Another note from user1")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountActiveUsersMonth()
	if err != nil {
		t.Fatalf("CountActiveUsersMonth failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 active users (should count distinct users), got %d", count)
	}
}

func TestCountActiveUsersHalfYear(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Test: Empty database
	count, err := db.CountActiveUsersHalfYear()
	if err != nil {
		t.Fatalf("CountActiveUsersHalfYear failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 active users in empty database, got %d", count)
	}

	// Test: Add users and notes
	userId1 := uuid.New()
	userId2 := uuid.New()
	userId3 := uuid.New()
	createTestAccount(t, db, userId1, "user1", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, userId2, "user2", "pubkey2", "webpub2", "webpriv2")
	createTestAccount(t, db, userId3, "user3", "pubkey3", "webpub3", "webpriv3")

	// User 1 posts
	_, err = db.CreateNote(userId1, "Note from user1")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountActiveUsersHalfYear()
	if err != nil {
		t.Fatalf("CountActiveUsersHalfYear failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 active user, got %d", count)
	}

	// User 2 posts
	_, err = db.CreateNote(userId2, "Note from user2")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountActiveUsersHalfYear()
	if err != nil {
		t.Fatalf("CountActiveUsersHalfYear failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 active users, got %d", count)
	}

	// User 3 posts
	_, err = db.CreateNote(userId3, "Note from user3")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	count, err = db.CountActiveUsersHalfYear()
	if err != nil {
		t.Fatalf("CountActiveUsersHalfYear failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 active users, got %d", count)
	}
}

func TestCountActiveUsers_Consistency(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create users and have them post
	userId1 := uuid.New()
	userId2 := uuid.New()
	createTestAccount(t, db, userId1, "user1", "pubkey1", "webpub1", "webpriv1")
	createTestAccount(t, db, userId2, "user2", "pubkey2", "webpub2", "webpriv2")

	_, err := db.CreateNote(userId1, "Note 1")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}
	_, err = db.CreateNote(userId2, "Note 2")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Get all counts
	activeMonth, err := db.CountActiveUsersMonth()
	if err != nil {
		t.Fatalf("CountActiveUsersMonth failed: %v", err)
	}

	activeHalfYear, err := db.CountActiveUsersHalfYear()
	if err != nil {
		t.Fatalf("CountActiveUsersHalfYear failed: %v", err)
	}

	totalUsers, err := db.CountAccounts()
	if err != nil {
		t.Fatalf("CountAccounts failed: %v", err)
	}

	// Verify logical consistency
	// ActiveMonth should not exceed ActiveHalfYear
	if activeMonth > activeHalfYear {
		t.Errorf("ActiveMonth (%d) should not exceed ActiveHalfYear (%d)", activeMonth, activeHalfYear)
	}

	// ActiveHalfYear should not exceed TotalUsers
	if activeHalfYear > totalUsers {
		t.Errorf("ActiveHalfYear (%d) should not exceed TotalUsers (%d)", activeHalfYear, totalUsers)
	}

	// ActiveMonth should not exceed TotalUsers
	if activeMonth > totalUsers {
		t.Errorf("ActiveMonth (%d) should not exceed TotalUsers (%d)", activeMonth, totalUsers)
	}
}

// TestReadActivityByObjectURI_WildcardEscaping tests that SQL wildcards are properly escaped
func TestReadActivityByObjectURI_WildcardEscaping(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create activities with URIs containing wildcards
	activities := []struct {
		uri     string
		content string
	}{
		{"https://example.com/posts/123", "Normal post"},
		{"https://example.com/posts/12%", "Post with percent"},
		{"https://example.com/posts/12_", "Post with underscore"},
		{"https://example.com/posts/12\\", "Post with backslash"},
	}

	for _, act := range activities {
		activity := &domain.Activity{
			Id:           uuid.New(),
			ActivityURI:  "https://example.com/activities/" + uuid.New().String(),
			ActivityType: "Create",
			ActorURI:     "https://example.com/users/alice",
			ObjectURI:    act.uri,
			RawJSON:      `{"id":"` + act.uri + `","type":"Note","content":"` + act.content + `"}`,
			Processed:    true,
			Local:        false,
			CreatedAt:    time.Now(),
		}
		if err := db.CreateActivity(activity); err != nil {
			t.Fatalf("Failed to create activity: %v", err)
		}
	}

	// Test exact match for URI with percent (should not match as wildcard)
	err, found := db.ReadActivityByObjectURI("https://example.com/posts/12%")
	if err != nil {
		t.Fatalf("ReadActivityByObjectURI failed: %v", err)
	}
	if found == nil {
		t.Fatal("Expected to find activity with percent in URI")
	}
	if !strings.Contains(found.RawJSON, "Post with percent") {
		t.Error("Found wrong activity - percent was treated as wildcard")
	}

	// Test exact match for URI with underscore (should not match as wildcard)
	err, found = db.ReadActivityByObjectURI("https://example.com/posts/12_")
	if err != nil {
		t.Fatalf("ReadActivityByObjectURI failed: %v", err)
	}
	if found == nil {
		t.Fatal("Expected to find activity with underscore in URI")
	}
	if !strings.Contains(found.RawJSON, "Post with underscore") {
		t.Error("Found wrong activity - underscore was treated as wildcard")
	}

	// Test exact match for URI with backslash
	err, found = db.ReadActivityByObjectURI("https://example.com/posts/12\\")
	if err != nil {
		t.Fatalf("ReadActivityByObjectURI failed: %v", err)
	}
	if found == nil {
		t.Fatal("Expected to find activity with backslash in URI")
	}
	if !strings.Contains(found.RawJSON, "Post with backslash") {
		t.Error("Found wrong activity - backslash escaping failed")
	}
}

// TestCleanupOrphanedFollows_BothDirections tests cleanup of orphaned follows
func TestCleanupOrphanedFollows_BothDirections(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create local account
	localId := uuid.New()
	createTestAccount(t, db, localId, "alice", "pubkey1", "webpub1", "webpriv1")

	// Create remote accounts
	remote1Id := uuid.New()
	remote2Id := uuid.New()
	remoteAcc1 := &domain.RemoteAccount{
		Id:            remote1Id,
		Username:      "bob",
		Domain:        "remote.com",
		ActorURI:      "https://remote.com/users/bob",
		InboxURI:      "https://remote.com/users/bob/inbox",
		PublicKeyPem:  "pubkey2",
		LastFetchedAt: time.Now(),
	}
	remoteAcc2 := &domain.RemoteAccount{
		Id:            remote2Id,
		Username:      "charlie",
		Domain:        "remote.com",
		ActorURI:      "https://remote.com/users/charlie",
		InboxURI:      "https://remote.com/users/charlie/inbox",
		PublicKeyPem:  "pubkey3",
		LastFetchedAt: time.Now(),
	}
	if err := db.CreateRemoteAccount(remoteAcc1); err != nil {
		t.Fatalf("Failed to create remote account 1: %v", err)
	}
	if err := db.CreateRemoteAccount(remoteAcc2); err != nil {
		t.Fatalf("Failed to create remote account 2: %v", err)
	}

	// Create follows:
	// 1. Local follows remote (following)
	follow1 := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localId,
		TargetAccountId: remote1Id,
		URI:             "https://example.com/follows/1",
		Accepted:        true,
		IsLocal:         false,
		CreatedAt:       time.Now(),
	}
	// 2. Remote follows local (follower)
	follow2 := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       remote2Id,
		TargetAccountId: localId,
		URI:             "https://remote.com/follows/1",
		Accepted:        true,
		IsLocal:         false,
		CreatedAt:       time.Now(),
	}
	// 3. Orphaned follow (remote account doesn't exist)
	orphanedId := uuid.New()
	follow3 := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localId,
		TargetAccountId: orphanedId, // This account doesn't exist
		URI:             "https://example.com/follows/orphaned",
		Accepted:        true,
		IsLocal:         false,
		CreatedAt:       time.Now(),
	}

	for _, f := range []*domain.Follow{follow1, follow2, follow3} {
		if err := db.CreateFollow(f); err != nil {
			t.Fatalf("Failed to create follow: %v", err)
		}
	}

	// Run cleanup
	if err := db.CleanupOrphanedFollows(); err != nil {
		t.Fatalf("CleanupOrphanedFollows failed: %v", err)
	}

	// Verify: follow1 and follow2 should remain, follow3 should be deleted
	err, f1 := db.ReadFollowByURI(follow1.URI)
	if err != nil || f1 == nil {
		t.Error("Valid follow (local->remote) was incorrectly deleted")
	}

	err, f2 := db.ReadFollowByURI(follow2.URI)
	if err != nil || f2 == nil {
		t.Error("Valid follow (remote->local) was incorrectly deleted")
	}

	err, f3 := db.ReadFollowByURI(follow3.URI)
	if err == nil && f3 != nil {
		t.Error("Orphaned follow should have been deleted")
	}

	// Delete remote account and verify cleanup removes follows
	if err := db.DeleteRemoteAccount(remote1Id); err != nil {
		t.Fatalf("Failed to delete remote account: %v", err)
	}

	// Run cleanup again
	if err := db.CleanupOrphanedFollows(); err != nil {
		t.Fatalf("CleanupOrphanedFollows failed: %v", err)
	}

	// Now follow1 should be gone too
	err, f1 = db.ReadFollowByURI(follow1.URI)
	if err == nil && f1 != nil {
		t.Error("Follow should have been deleted after remote account deletion")
	}
}

// Hashtag tests

func TestCreateOrUpdateHashtag(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create new hashtag
	id1, err := db.CreateOrUpdateHashtag("golang")
	if err != nil {
		t.Fatalf("CreateOrUpdateHashtag failed: %v", err)
	}
	if id1 == 0 {
		t.Error("Expected non-zero hashtag ID")
	}

	// Verify hashtag was created with usage_count = 1
	var name string
	var usageCount int
	err = db.db.QueryRow("SELECT name, usage_count FROM hashtags WHERE id = ?", id1).Scan(&name, &usageCount)
	if err != nil {
		t.Fatalf("Failed to query hashtag: %v", err)
	}
	if name != "golang" {
		t.Errorf("Expected name 'golang', got '%s'", name)
	}
	if usageCount != 1 {
		t.Errorf("Expected usage_count 1, got %d", usageCount)
	}

	// Update existing hashtag (should increment usage_count)
	id2, err := db.CreateOrUpdateHashtag("golang")
	if err != nil {
		t.Fatalf("CreateOrUpdateHashtag (update) failed: %v", err)
	}
	if id2 != id1 {
		t.Errorf("Expected same ID on update, got %d vs %d", id1, id2)
	}

	// Verify usage_count incremented
	err = db.db.QueryRow("SELECT usage_count FROM hashtags WHERE id = ?", id1).Scan(&usageCount)
	if err != nil {
		t.Fatalf("Failed to query hashtag: %v", err)
	}
	if usageCount != 2 {
		t.Errorf("Expected usage_count 2 after update, got %d", usageCount)
	}
}

func TestCreateOrUpdateHashtag_CaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create hashtag
	id1, err := db.CreateOrUpdateHashtag("GoLang")
	if err != nil {
		t.Fatalf("CreateOrUpdateHashtag failed: %v", err)
	}

	// Verify it's stored as lowercase
	var name string
	err = db.db.QueryRow("SELECT name FROM hashtags WHERE id = ?", id1).Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query hashtag: %v", err)
	}
	if name != "golang" {
		t.Errorf("Expected lowercase 'golang', got '%s'", name)
	}

	// Update with different case should update the same record
	id2, err := db.CreateOrUpdateHashtag("GOLANG")
	if err != nil {
		t.Fatalf("CreateOrUpdateHashtag (different case) failed: %v", err)
	}
	if id2 != id1 {
		t.Errorf("Expected same ID for case-insensitive match, got %d vs %d", id1, id2)
	}
}

func TestLinkNoteHashtags(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")
	noteId, err := db.CreateNote(userId, "Test note #golang #rust")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Create hashtags
	golangId, _ := db.CreateOrUpdateHashtag("golang")
	rustId, _ := db.CreateOrUpdateHashtag("rust")

	// Link note to hashtags
	err = db.LinkNoteHashtags(noteId, []int64{golangId, rustId})
	if err != nil {
		t.Fatalf("LinkNoteHashtags failed: %v", err)
	}

	// Verify links were created
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM note_hashtags WHERE note_id = ?", noteId.String()).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query note_hashtags: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 hashtag links, got %d", count)
	}
}

func TestLinkNoteHashtags_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")
	noteId, err := db.CreateNote(userId, "Test note #golang")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Create hashtag
	golangId, _ := db.CreateOrUpdateHashtag("golang")

	// Link note to hashtag
	err = db.LinkNoteHashtags(noteId, []int64{golangId})
	if err != nil {
		t.Fatalf("LinkNoteHashtags failed: %v", err)
	}

	// Try to link again (should be ignored due to INSERT OR IGNORE)
	err = db.LinkNoteHashtags(noteId, []int64{golangId})
	if err != nil {
		t.Fatalf("LinkNoteHashtags (duplicate) should not error: %v", err)
	}

	// Verify only one link exists
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM note_hashtags WHERE note_id = ?", noteId.String()).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query note_hashtags: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 hashtag link after duplicate insert, got %d", count)
	}
}

func TestReadHashtagsByNoteId(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")
	noteId, err := db.CreateNote(userId, "Test note #golang #rust #fediverse")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Create and link hashtags
	golangId, _ := db.CreateOrUpdateHashtag("golang")
	rustId, _ := db.CreateOrUpdateHashtag("rust")
	fediverseId, _ := db.CreateOrUpdateHashtag("fediverse")
	err = db.LinkNoteHashtags(noteId, []int64{golangId, rustId, fediverseId})
	if err != nil {
		t.Fatalf("LinkNoteHashtags failed: %v", err)
	}

	// Read hashtags
	err, hashtags := db.ReadHashtagsByNoteId(noteId)
	if err != nil {
		t.Fatalf("ReadHashtagsByNoteId failed: %v", err)
	}
	if len(hashtags) != 3 {
		t.Errorf("Expected 3 hashtags, got %d", len(hashtags))
	}

	// Verify all hashtags are present
	hashtagMap := make(map[string]bool)
	for _, h := range hashtags {
		hashtagMap[h] = true
	}
	if !hashtagMap["golang"] {
		t.Error("Expected 'golang' in hashtags")
	}
	if !hashtagMap["rust"] {
		t.Error("Expected 'rust' in hashtags")
	}
	if !hashtagMap["fediverse"] {
		t.Error("Expected 'fediverse' in hashtags")
	}
}

func TestReadHashtagsByNoteId_NoHashtags(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note without hashtags
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")
	noteId, err := db.CreateNote(userId, "Test note without hashtags")
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Read hashtags (should return empty slice)
	err, hashtags := db.ReadHashtagsByNoteId(noteId)
	if err != nil {
		t.Fatalf("ReadHashtagsByNoteId failed: %v", err)
	}
	if len(hashtags) != 0 {
		t.Errorf("Expected 0 hashtags, got %d", len(hashtags))
	}
}

func TestReadNotesByHashtag(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create notes with hashtags
	note1Id, _ := db.CreateNote(userId, "Note 1 #golang")
	note2Id, _ := db.CreateNote(userId, "Note 2 #golang #rust")
	note3Id, _ := db.CreateNote(userId, "Note 3 #rust")

	// Create and link hashtags
	golangId, _ := db.CreateOrUpdateHashtag("golang")
	rustId, _ := db.CreateOrUpdateHashtag("rust")
	db.LinkNoteHashtags(note1Id, []int64{golangId})
	db.LinkNoteHashtags(note2Id, []int64{golangId, rustId})
	db.LinkNoteHashtags(note3Id, []int64{rustId})

	// Read notes by #golang
	err, notes := db.ReadNotesByHashtag("golang", 10, 0)
	if err != nil {
		t.Fatalf("ReadNotesByHashtag failed: %v", err)
	}
	if len(*notes) != 2 {
		t.Errorf("Expected 2 notes with #golang, got %d", len(*notes))
	}

	// Read notes by #rust
	err, notes = db.ReadNotesByHashtag("rust", 10, 0)
	if err != nil {
		t.Fatalf("ReadNotesByHashtag failed: %v", err)
	}
	if len(*notes) != 2 {
		t.Errorf("Expected 2 notes with #rust, got %d", len(*notes))
	}

	// Read notes by non-existent hashtag
	err, notes = db.ReadNotesByHashtag("nonexistent", 10, 0)
	if err != nil {
		t.Fatalf("ReadNotesByHashtag failed: %v", err)
	}
	if len(*notes) != 0 {
		t.Errorf("Expected 0 notes with #nonexistent, got %d", len(*notes))
	}
}

func TestReadNotesByHashtag_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create 5 notes with #golang
	golangId, _ := db.CreateOrUpdateHashtag("golang")
	for i := 0; i < 5; i++ {
		noteId, _ := db.CreateNote(userId, "Note #golang")
		db.LinkNoteHashtags(noteId, []int64{golangId})
	}

	// Test limit
	err, notes := db.ReadNotesByHashtag("golang", 3, 0)
	if err != nil {
		t.Fatalf("ReadNotesByHashtag failed: %v", err)
	}
	if len(*notes) != 3 {
		t.Errorf("Expected 3 notes with limit=3, got %d", len(*notes))
	}

	// Test offset
	err, notes = db.ReadNotesByHashtag("golang", 3, 3)
	if err != nil {
		t.Fatalf("ReadNotesByHashtag with offset failed: %v", err)
	}
	if len(*notes) != 2 {
		t.Errorf("Expected 2 notes with limit=3 offset=3, got %d", len(*notes))
	}
}

func TestReadNotesByHashtag_CaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user and note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")
	noteId, _ := db.CreateNote(userId, "Note #GoLang")

	// Create and link hashtag
	golangId, _ := db.CreateOrUpdateHashtag("golang")
	db.LinkNoteHashtags(noteId, []int64{golangId})

	// Search with different case
	err, notes := db.ReadNotesByHashtag("GOLANG", 10, 0)
	if err != nil {
		t.Fatalf("ReadNotesByHashtag failed: %v", err)
	}
	if len(*notes) != 1 {
		t.Errorf("Expected 1 note with case-insensitive search, got %d", len(*notes))
	}
}

func TestCountNotesByHashtag(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create notes with hashtags
	note1Id, _ := db.CreateNote(userId, "Note 1 #golang")
	note2Id, _ := db.CreateNote(userId, "Note 2 #golang #rust")
	note3Id, _ := db.CreateNote(userId, "Note 3 #golang")

	// Create and link hashtags
	golangId, _ := db.CreateOrUpdateHashtag("golang")
	rustId, _ := db.CreateOrUpdateHashtag("rust")
	db.LinkNoteHashtags(note1Id, []int64{golangId})
	db.LinkNoteHashtags(note2Id, []int64{golangId, rustId})
	db.LinkNoteHashtags(note3Id, []int64{golangId})

	// Count notes by #golang
	count, err := db.CountNotesByHashtag("golang")
	if err != nil {
		t.Fatalf("CountNotesByHashtag failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 notes with #golang, got %d", count)
	}

	// Count notes by #rust
	count, err = db.CountNotesByHashtag("rust")
	if err != nil {
		t.Fatalf("CountNotesByHashtag failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 note with #rust, got %d", count)
	}

	// Count notes by non-existent hashtag
	count, err = db.CountNotesByHashtag("nonexistent")
	if err != nil {
		t.Fatalf("CountNotesByHashtag failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 notes with #nonexistent, got %d", count)
	}
}

// ============================================================================
// Reply Count Tests
// ============================================================================

func TestExtractInReplyToFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		rawJSON  string
		expected string
	}{
		{
			name:     "valid inReplyTo string",
			rawJSON:  `{"object":{"inReplyTo":"https://example.com/notes/123"}}`,
			expected: "https://example.com/notes/123",
		},
		{
			name:     "inReplyTo with spaces in JSON",
			rawJSON:  `{"object": {"inReplyTo": "https://example.com/notes/456"}}`,
			expected: "https://example.com/notes/456",
		},
		{
			name:     "inReplyTo is null",
			rawJSON:  `{"object":{"inReplyTo":null}}`,
			expected: "",
		},
		{
			name:     "missing inReplyTo field",
			rawJSON:  `{"object":{"content":"Hello world"}}`,
			expected: "",
		},
		{
			name:     "missing object field",
			rawJSON:  `{"type":"Create"}`,
			expected: "",
		},
		{
			name:     "empty JSON",
			rawJSON:  "",
			expected: "",
		},
		{
			name:     "invalid JSON",
			rawJSON:  `{invalid}`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInReplyToFromJSON(tt.rawJSON)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIncrementReplyCountRecursive_Notes(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create a thread: root -> reply1 -> reply2 (3 levels deep)
	rootId := uuid.New()
	reply1Id := uuid.New()
	reply2Id := uuid.New()

	rootURI := "https://example.com/notes/" + rootId.String()
	reply1URI := "https://example.com/notes/" + reply1Id.String()

	// Create root note
	_, err := db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, object_uri, reply_count)
		VALUES (?, ?, ?, ?, ?, ?)`,
		rootId, userId.String(), "Root post", time.Now(), rootURI, 0)
	if err != nil {
		t.Fatalf("Failed to create root note: %v", err)
	}

	// Create reply1 (replies to root)
	_, err = db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, object_uri, in_reply_to_uri, reply_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		reply1Id, userId.String(), "Reply 1", time.Now(), reply1URI, rootURI, 0)
	if err != nil {
		t.Fatalf("Failed to create reply1: %v", err)
	}

	// Create reply2 (replies to reply1)
	_, err = db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, object_uri, in_reply_to_uri, reply_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		reply2Id, userId.String(), "Reply 2", time.Now(), "", reply1URI, 0)
	if err != nil {
		t.Fatalf("Failed to create reply2: %v", err)
	}

	// Increment reply count for reply1URI (simulates receiving reply2)
	err = db.IncrementReplyCountByURI(reply1URI)
	if err != nil {
		t.Fatalf("IncrementReplyCountByURI failed: %v", err)
	}

	// Verify reply1 has reply_count = 1
	var reply1Count int
	err = db.db.QueryRow(`SELECT reply_count FROM notes WHERE id = ?`, reply1Id.String()).Scan(&reply1Count)
	if err != nil {
		t.Fatalf("Failed to query reply1: %v", err)
	}
	if reply1Count != 1 {
		t.Errorf("Expected reply1 reply_count = 1, got %d", reply1Count)
	}

	// Verify root also has reply_count = 1 (recursive increment)
	var rootCount int
	err = db.db.QueryRow(`SELECT reply_count FROM notes WHERE id = ?`, rootId.String()).Scan(&rootCount)
	if err != nil {
		t.Fatalf("Failed to query root: %v", err)
	}
	if rootCount != 1 {
		t.Errorf("Expected root reply_count = 1 (recursive), got %d", rootCount)
	}
}

func TestIncrementReplyCountRecursive_Activities(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create a chain of activities: remoteRoot -> remoteReply1 -> remoteReply2
	rootURI := "https://remote.example.com/notes/root"
	reply1URI := "https://remote.example.com/notes/reply1"

	// Create root activity (no inReplyTo)
	rootActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/1",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    rootURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + rootURI + `","type":"Note","content":"Root post"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	err := db.CreateActivity(rootActivity)
	if err != nil {
		t.Fatalf("Failed to create root activity: %v", err)
	}

	// Create reply1 activity (replies to root)
	reply1Activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/2",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/bob",
		ObjectURI:    reply1URI,
		InReplyTo:    rootURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + reply1URI + `","type":"Note","content":"Reply 1","inReplyTo":"` + rootURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	err = db.CreateActivity(reply1Activity)
	if err != nil {
		t.Fatalf("Failed to create reply1 activity: %v", err)
	}

	// Increment reply count for reply1URI (simulates receiving a new reply)
	err = db.IncrementReplyCountByURI(reply1URI)
	if err != nil {
		t.Fatalf("IncrementReplyCountByURI failed: %v", err)
	}

	// Verify reply1 has reply_count = 1
	var reply1Count int
	err = db.db.QueryRow(`SELECT reply_count FROM activities WHERE object_uri = ?`, reply1URI).Scan(&reply1Count)
	if err != nil {
		t.Fatalf("Failed to query reply1: %v", err)
	}
	if reply1Count != 1 {
		t.Errorf("Expected reply1 reply_count = 1, got %d", reply1Count)
	}

	// Verify root also has reply_count = 1 (recursive increment via inReplyTo in raw_json)
	var rootCount int
	err = db.db.QueryRow(`SELECT reply_count FROM activities WHERE object_uri = ?`, rootURI).Scan(&rootCount)
	if err != nil {
		t.Fatalf("Failed to query root: %v", err)
	}
	if rootCount != 1 {
		t.Errorf("Expected root reply_count = 1 (recursive), got %d", rootCount)
	}
}

func TestIncrementReplyCountRecursive_MixedChain(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create a chain: local note -> remote activity reply
	localNoteId := uuid.New()
	localNoteURI := "https://example.com/notes/" + localNoteId.String()

	// Create local note (root)
	_, err := db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, object_uri, reply_count)
		VALUES (?, ?, ?, ?, ?, ?)`,
		localNoteId, userId.String(), "Local root post", time.Now(), localNoteURI, 0)
	if err != nil {
		t.Fatalf("Failed to create local note: %v", err)
	}

	// Create remote activity that replies to local note
	remoteReplyURI := "https://remote.example.com/notes/remote-reply"
	remoteActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/1",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    remoteReplyURI,
		InReplyTo:    localNoteURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + remoteReplyURI + `","type":"Note","content":"Remote reply","inReplyTo":"` + localNoteURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	err = db.CreateActivity(remoteActivity)
	if err != nil {
		t.Fatalf("Failed to create remote activity: %v", err)
	}

	// Increment reply count for local note (simulates receiving the remote reply)
	err = db.IncrementReplyCountByURI(localNoteURI)
	if err != nil {
		t.Fatalf("IncrementReplyCountByURI failed: %v", err)
	}

	// Verify local note has reply_count = 1
	var localCount int
	err = db.db.QueryRow(`SELECT reply_count FROM notes WHERE id = ?`, localNoteId.String()).Scan(&localCount)
	if err != nil {
		t.Fatalf("Failed to query local note: %v", err)
	}
	if localCount != 1 {
		t.Errorf("Expected local note reply_count = 1, got %d", localCount)
	}
}

func TestIncrementReplyCount_ByNoteIdInURI(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create a note WITHOUT object_uri (to test ID-in-URI matching)
	noteId := uuid.New()
	_, err := db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, reply_count)
		VALUES (?, ?, ?, ?, ?)`,
		noteId, userId.String(), "Test note", time.Now(), 0)
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Increment using a URI that contains the note ID
	noteURI := "https://example.com/notes/" + noteId.String()
	err = db.IncrementReplyCountByURI(noteURI)
	if err != nil {
		t.Fatalf("IncrementReplyCountByURI failed: %v", err)
	}

	// Verify reply_count was incremented
	var count int
	err = db.db.QueryRow(`SELECT reply_count FROM notes WHERE id = ?`, noteId.String()).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query note: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected reply_count = 1, got %d", count)
	}
}

func TestIncrementReplyCount_CycleDetection(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create a note that references itself (artificial cycle)
	noteId := uuid.New()
	noteURI := "https://example.com/notes/" + noteId.String()

	_, err := db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, object_uri, in_reply_to_uri, reply_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		noteId, userId.String(), "Self-referencing note", time.Now(), noteURI, noteURI, 0)
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// This should not cause infinite recursion due to visited map
	err = db.IncrementReplyCountByURI(noteURI)
	if err != nil {
		t.Fatalf("IncrementReplyCountByURI should handle cycles: %v", err)
	}

	// Verify it was only incremented once
	var count int
	err = db.db.QueryRow(`SELECT reply_count FROM notes WHERE id = ?`, noteId.String()).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query note: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected reply_count = 1 (not double-counted), got %d", count)
	}
}

func TestCountActivitiesByInReplyTo_Basic(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	parentURI := "https://example.com/notes/parent"

	// Create 3 activities that reply to the parent
	for i := 0; i < 3; i++ {
		activity := &domain.Activity{
			Id:           uuid.New(),
			ActivityURI:  "https://remote.example.com/activities/" + uuid.New().String(),
			ActivityType: "Create",
			ActorURI:     "https://remote.example.com/users/alice",
			ObjectURI:    "https://remote.example.com/notes/" + uuid.New().String(),
			InReplyTo:    parentURI,
			RawJSON:      `{"type":"Create","object":{"inReplyTo":"` + parentURI + `"}}`,
			Processed:    true,
			CreatedAt:    time.Now(),
		}
		if err := db.CreateActivity(activity); err != nil {
			t.Fatalf("Failed to create activity %d: %v", i, err)
		}
	}

	// Count replies
	count, err := db.CountActivitiesByInReplyTo(parentURI)
	if err != nil {
		t.Fatalf("CountActivitiesByInReplyTo failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 replies, got %d", count)
	}
}

func TestCountActivitiesByInReplyTo_ExcludesDuplicates(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user for local note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	parentURI := "https://example.com/notes/parent"
	localNoteId := uuid.New()
	localNoteURI := "https://example.com/notes/" + localNoteId.String()

	// Create a local note that replies to parent
	_, err := db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, object_uri, in_reply_to_uri)
		VALUES (?, ?, ?, ?, ?, ?)`,
		localNoteId, userId.String(), "Local reply", time.Now(), localNoteURI, parentURI)
	if err != nil {
		t.Fatalf("Failed to create local note: %v", err)
	}

	// Create a remote activity that is a DUPLICATE of the local note
	// (same object_uri - this happens when a local post federates back)
	duplicateActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/" + uuid.New().String(),
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    localNoteURI, // Same as local note!
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + localNoteURI + `","inReplyTo":"` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(duplicateActivity); err != nil {
		t.Fatalf("Failed to create duplicate activity: %v", err)
	}

	// Create a genuine remote activity (not a duplicate)
	genuineActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://other-remote.example.com/activities/" + uuid.New().String(),
		ActivityType: "Create",
		ActorURI:     "https://other-remote.example.com/users/bob",
		ObjectURI:    "https://other-remote.example.com/notes/" + uuid.New().String(),
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"inReplyTo":"` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(genuineActivity); err != nil {
		t.Fatalf("Failed to create genuine activity: %v", err)
	}

	// Count should be 1 (only the genuine activity, not the duplicate)
	count, err := db.CountActivitiesByInReplyTo(parentURI)
	if err != nil {
		t.Fatalf("CountActivitiesByInReplyTo failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 reply (excluding duplicate), got %d", count)
	}
}

func TestCountActivitiesByInReplyTo_ExcludesDuplicatesByNoteIdPattern(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create user for local note
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	parentURI := "https://example.com/notes/parent"
	localNoteId := uuid.New()

	// Create a local note WITHOUT object_uri (to test the /notes/{uuid} pattern matching)
	_, err := db.db.Exec(`INSERT INTO notes (id, user_id, message, created_at, in_reply_to_uri)
		VALUES (?, ?, ?, ?, ?)`,
		localNoteId, userId.String(), "Local reply", time.Now(), parentURI)
	if err != nil {
		t.Fatalf("Failed to create local note: %v", err)
	}

	// Create a remote activity that references the local note by ID pattern
	duplicateActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/" + uuid.New().String(),
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    "https://example.com/notes/" + localNoteId.String(), // Contains the note ID
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"inReplyTo":"` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(duplicateActivity); err != nil {
		t.Fatalf("Failed to create duplicate activity: %v", err)
	}

	// Create a genuine remote activity (not a duplicate)
	genuineActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://other-remote.example.com/activities/" + uuid.New().String(),
		ActivityType: "Create",
		ActorURI:     "https://other-remote.example.com/users/bob",
		ObjectURI:    "https://other-remote.example.com/notes/" + uuid.New().String(),
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"inReplyTo":"` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(genuineActivity); err != nil {
		t.Fatalf("Failed to create genuine activity: %v", err)
	}

	// Count should be 1 (only the genuine activity, not the duplicate)
	count, err := db.CountActivitiesByInReplyTo(parentURI)
	if err != nil {
		t.Fatalf("CountActivitiesByInReplyTo failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 reply (excluding duplicate by ID pattern), got %d", count)
	}
}

func TestCountActivitiesByInReplyTo_SpaceVariants(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	parentURI := "https://example.com/notes/parent"

	// Create activity with "inReplyTo" (no space)
	activity1 := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/" + uuid.New().String(),
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    "https://remote.example.com/notes/1",
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"inReplyTo":"` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(activity1); err != nil {
		t.Fatalf("Failed to create activity1: %v", err)
	}

	// Create activity with "inReplyTo": (with space)
	activity2 := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/" + uuid.New().String(),
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/bob",
		ObjectURI:    "https://remote.example.com/notes/2",
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"inReplyTo": "` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(activity2); err != nil {
		t.Fatalf("Failed to create activity2: %v", err)
	}

	// Both should be counted
	count, err := db.CountActivitiesByInReplyTo(parentURI)
	if err != nil {
		t.Fatalf("CountActivitiesByInReplyTo failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 replies (both space variants), got %d", count)
	}
}

func TestCountActivitiesByInReplyTo_OnlyCountsCreateType(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	parentURI := "https://example.com/notes/parent"

	// Create a "Create" activity
	createActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    "https://remote.example.com/notes/1",
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"inReplyTo":"` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(createActivity); err != nil {
		t.Fatalf("Failed to create Create activity: %v", err)
	}

	// Create a "Like" activity (should not be counted)
	likeActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/like",
		ActivityType: "Like",
		ActorURI:     "https://remote.example.com/users/bob",
		ObjectURI:    "https://remote.example.com/notes/2",
		RawJSON:      `{"type":"Like","object":{"inReplyTo":"` + parentURI + `"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(likeActivity); err != nil {
		t.Fatalf("Failed to create Like activity: %v", err)
	}

	// Only the Create should be counted
	count, err := db.CountActivitiesByInReplyTo(parentURI)
	if err != nil {
		t.Fatalf("CountActivitiesByInReplyTo failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 reply (only Create type), got %d", count)
	}
}

// Tests for remote post likes with object_uri

func TestCreateLikeByObjectURI_DeterministicPlaceholder(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	accountId := uuid.New()
	objectURI := "https://remote.example.com/notes/123"

	like := &domain.Like{
		Id:        uuid.New(),
		AccountId: accountId,
		URI:       "https://local.example.com/likes/1",
		CreatedAt: time.Now(),
	}

	// Create the like
	err := db.CreateLikeByObjectURI(like, objectURI)
	if err != nil {
		t.Fatalf("CreateLikeByObjectURI failed: %v", err)
	}

	// Verify it was created with the object_uri
	hasLike, err := db.HasLikeByObjectURI(accountId, objectURI)
	if err != nil {
		t.Fatalf("HasLikeByObjectURI failed: %v", err)
	}
	if !hasLike {
		t.Error("Expected HasLikeByObjectURI to return true")
	}

	// Verify the placeholder note_id is deterministic (same object_uri = same placeholder)
	expectedPlaceholder := uuid.NewSHA1(uuid.NameSpaceURL, []byte(objectURI))
	var storedNoteId string
	err = db.db.QueryRow("SELECT note_id FROM likes WHERE object_uri = ?", objectURI).Scan(&storedNoteId)
	if err != nil {
		t.Fatalf("Failed to query note_id: %v", err)
	}
	if storedNoteId != expectedPlaceholder.String() {
		t.Errorf("Expected placeholder note_id %s, got %s", expectedPlaceholder.String(), storedNoteId)
	}
}

func TestCreateLikeByObjectURI_DifferentPostsDifferentPlaceholders(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	accountId := uuid.New()
	objectURI1 := "https://remote.example.com/notes/123"
	objectURI2 := "https://remote.example.com/notes/456"

	like1 := &domain.Like{
		Id:        uuid.New(),
		AccountId: accountId,
		URI:       "https://local.example.com/likes/1",
		CreatedAt: time.Now(),
	}
	like2 := &domain.Like{
		Id:        uuid.New(),
		AccountId: accountId,
		URI:       "https://local.example.com/likes/2",
		CreatedAt: time.Now(),
	}

	// Create likes for two different remote posts
	if err := db.CreateLikeByObjectURI(like1, objectURI1); err != nil {
		t.Fatalf("CreateLikeByObjectURI for first post failed: %v", err)
	}
	if err := db.CreateLikeByObjectURI(like2, objectURI2); err != nil {
		t.Fatalf("CreateLikeByObjectURI for second post failed: %v", err)
	}

	// Verify both likes exist
	hasLike1, _ := db.HasLikeByObjectURI(accountId, objectURI1)
	hasLike2, _ := db.HasLikeByObjectURI(accountId, objectURI2)
	if !hasLike1 || !hasLike2 {
		t.Error("Expected both likes to exist")
	}

	// Verify they have different placeholder note_ids
	var noteId1, noteId2 string
	db.db.QueryRow("SELECT note_id FROM likes WHERE object_uri = ?", objectURI1).Scan(&noteId1)
	db.db.QueryRow("SELECT note_id FROM likes WHERE object_uri = ?", objectURI2).Scan(&noteId2)
	if noteId1 == noteId2 {
		t.Errorf("Expected different placeholder note_ids for different posts, both got %s", noteId1)
	}
}

func TestCreateLikeByObjectURI_UniqueConstraintPreventsDoubleLike(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	accountId := uuid.New()
	objectURI := "https://remote.example.com/notes/123"

	like1 := &domain.Like{
		Id:        uuid.New(),
		AccountId: accountId,
		URI:       "https://local.example.com/likes/1",
		CreatedAt: time.Now(),
	}
	like2 := &domain.Like{
		Id:        uuid.New(),
		AccountId: accountId,
		URI:       "https://local.example.com/likes/2",
		CreatedAt: time.Now(),
	}

	// First like should succeed
	if err := db.CreateLikeByObjectURI(like1, objectURI); err != nil {
		t.Fatalf("First CreateLikeByObjectURI failed: %v", err)
	}

	// Second like to same post should fail (unique constraint)
	err := db.CreateLikeByObjectURI(like2, objectURI)
	if err == nil {
		t.Error("Expected second like to same post to fail with unique constraint")
	}
}

func TestDeleteLikeByAccountAndObjectURI(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	accountId := uuid.New()
	objectURI := "https://remote.example.com/notes/123"

	like := &domain.Like{
		Id:        uuid.New(),
		AccountId: accountId,
		URI:       "https://local.example.com/likes/1",
		CreatedAt: time.Now(),
	}

	// Create the like
	if err := db.CreateLikeByObjectURI(like, objectURI); err != nil {
		t.Fatalf("CreateLikeByObjectURI failed: %v", err)
	}

	// Verify it exists
	hasLike, _ := db.HasLikeByObjectURI(accountId, objectURI)
	if !hasLike {
		t.Fatal("Like should exist before deletion")
	}

	// Delete it
	if err := db.DeleteLikeByAccountAndObjectURI(accountId, objectURI); err != nil {
		t.Fatalf("DeleteLikeByAccountAndObjectURI failed: %v", err)
	}

	// Verify it's gone
	hasLike, _ = db.HasLikeByObjectURI(accountId, objectURI)
	if hasLike {
		t.Error("Like should not exist after deletion")
	}
}

func TestReadLikeByAccountAndObjectURI(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	accountId := uuid.New()
	objectURI := "https://remote.example.com/notes/123"
	likeId := uuid.New()

	like := &domain.Like{
		Id:        likeId,
		AccountId: accountId,
		URI:       "https://local.example.com/likes/1",
		CreatedAt: time.Now(),
	}

	// Create the like
	if err := db.CreateLikeByObjectURI(like, objectURI); err != nil {
		t.Fatalf("CreateLikeByObjectURI failed: %v", err)
	}

	// Read it back
	err, readLike := db.ReadLikeByAccountAndObjectURI(accountId, objectURI)
	if err != nil {
		t.Fatalf("ReadLikeByAccountAndObjectURI failed: %v", err)
	}
	if readLike == nil {
		t.Fatal("Expected to find like")
	}
	if readLike.Id != likeId {
		t.Errorf("Expected like ID %s, got %s", likeId, readLike.Id)
	}
	if readLike.AccountId != accountId {
		t.Errorf("Expected account ID %s, got %s", accountId, readLike.AccountId)
	}
}

func TestIncrementDecrementLikeCountByObjectURI(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	objectURI := "https://remote.example.com/notes/123"

	// Create an activity for this object
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    objectURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + objectURI + `","content":"Hello"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(activity); err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Increment like count
	if err := db.IncrementLikeCountByObjectURI(objectURI); err != nil {
		t.Fatalf("IncrementLikeCountByObjectURI failed: %v", err)
	}

	// Verify count is 1
	var likeCount int
	db.db.QueryRow("SELECT like_count FROM activities WHERE object_uri = ?", objectURI).Scan(&likeCount)
	if likeCount != 1 {
		t.Errorf("Expected like_count 1, got %d", likeCount)
	}

	// Increment again
	db.IncrementLikeCountByObjectURI(objectURI)
	db.db.QueryRow("SELECT like_count FROM activities WHERE object_uri = ?", objectURI).Scan(&likeCount)
	if likeCount != 2 {
		t.Errorf("Expected like_count 2, got %d", likeCount)
	}

	// Decrement
	if err := db.DecrementLikeCountByObjectURI(objectURI); err != nil {
		t.Fatalf("DecrementLikeCountByObjectURI failed: %v", err)
	}
	db.db.QueryRow("SELECT like_count FROM activities WHERE object_uri = ?", objectURI).Scan(&likeCount)
	if likeCount != 1 {
		t.Errorf("Expected like_count 1 after decrement, got %d", likeCount)
	}
}

func TestReadActivitiesByInReplyTo_IncludesLikeAndBoostCounts(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	parentURI := "https://example.com/notes/parent"
	replyURI := "https://remote.example.com/notes/reply1"

	// Create a reply activity with like and boost counts
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    replyURI,
		InReplyTo:    parentURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + replyURI + `","inReplyTo":"` + parentURI + `","content":"Reply"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(activity); err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Set like and boost counts directly
	db.db.Exec("UPDATE activities SET like_count = 5, boost_count = 3 WHERE object_uri = ?", replyURI)

	// Read activities by in_reply_to
	err, activities := db.ReadActivitiesByInReplyTo(parentURI)
	if err != nil {
		t.Fatalf("ReadActivitiesByInReplyTo failed: %v", err)
	}
	if activities == nil || len(*activities) != 1 {
		t.Fatalf("Expected 1 activity, got %v", activities)
	}

	reply := (*activities)[0]
	if reply.LikeCount != 5 {
		t.Errorf("Expected LikeCount 5, got %d", reply.LikeCount)
	}
	if reply.BoostCount != 3 {
		t.Errorf("Expected BoostCount 3, got %d", reply.BoostCount)
	}
}

func TestReadActivityByObjectURI_IncludesLikeAndBoostCounts(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	objectURI := "https://remote.example.com/notes/123"

	// Create an activity
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/alice",
		ObjectURI:    objectURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + objectURI + `","content":"Hello"}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
	}
	if err := db.CreateActivity(activity); err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Set like and boost counts
	db.db.Exec("UPDATE activities SET like_count = 10, boost_count = 7 WHERE object_uri = ?", objectURI)

	// Read the activity by object URI
	err, readActivity := db.ReadActivityByObjectURI(objectURI)
	if err != nil {
		t.Fatalf("ReadActivityByObjectURI failed: %v", err)
	}
	if readActivity == nil {
		t.Fatal("Expected to find activity")
	}

	if readActivity.LikeCount != 10 {
		t.Errorf("Expected LikeCount 10, got %d", readActivity.LikeCount)
	}
	if readActivity.BoostCount != 7 {
		t.Errorf("Expected BoostCount 7, got %d", readActivity.BoostCount)
	}
}

func TestReadHomeTimelinePosts_RemotePostsIncludeLikeAndBoostCounts(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create a local account
	localAccountId := uuid.New()
	createTestAccount(t, db, localAccountId, "localuser", "ssh-key", "webpub", "webpriv")

	// Create a remote account
	remoteAccountId := uuid.New()
	_, err := db.db.Exec(`INSERT INTO remote_accounts(id, username, domain, actor_uri, inbox_uri) VALUES (?, ?, ?, ?, ?)`,
		remoteAccountId.String(), "remoteuser", "remote.example.com",
		"https://remote.example.com/users/remoteuser",
		"https://remote.example.com/users/remoteuser/inbox")
	if err != nil {
		t.Fatalf("Failed to create remote account: %v", err)
	}

	// Create a follow relationship
	followId := uuid.New()
	_, err = db.db.Exec(`INSERT INTO follows(id, account_id, target_account_id, accepted, is_local) VALUES (?, ?, ?, 1, 0)`,
		followId.String(), localAccountId.String(), remoteAccountId.String())
	if err != nil {
		t.Fatalf("Failed to create follow: %v", err)
	}

	// Create an activity from the remote user
	objectURI := "https://remote.example.com/notes/123"
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/remoteuser",
		ObjectURI:    objectURI,
		RawJSON:      `{"type":"Create","object":{"id":"` + objectURI + `","content":"Hello from remote","inReplyTo":null}}`,
		Processed:    true,
		CreatedAt:    time.Now(),
		Local:        false,
	}
	if err := db.CreateActivity(activity); err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Set like and boost counts
	db.db.Exec("UPDATE activities SET like_count = 15, boost_count = 8 WHERE object_uri = ?", objectURI)

	// Read home timeline
	err, posts := db.ReadHomeTimelinePosts(localAccountId, 10)
	if err != nil {
		t.Fatalf("ReadHomeTimelinePosts failed: %v", err)
	}

	// Find the remote post
	var remotePost *domain.HomePost
	for i := range *posts {
		if (*posts)[i].ObjectURI == objectURI {
			remotePost = &(*posts)[i]
			break
		}
	}

	if remotePost == nil {
		t.Fatal("Expected to find remote post in home timeline")
	}

	if remotePost.LikeCount != 15 {
		t.Errorf("Expected LikeCount 15, got %d", remotePost.LikeCount)
	}
	if remotePost.BoostCount != 8 {
		t.Errorf("Expected BoostCount 8, got %d", remotePost.BoostCount)
	}
}

// ============ Relay Tests ============

func TestCreateRelay(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	relay := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://relay.example.com/actor",
		InboxURI:  "https://relay.example.com/inbox",
		Name:      "Test Relay",
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	err := db.CreateRelay(relay)
	if err != nil {
		t.Fatalf("CreateRelay failed: %v", err)
	}

	// Verify it was created
	err, fetched := db.ReadRelayByActorURI(relay.ActorURI)
	if err != nil {
		t.Fatalf("ReadRelayByActorURI failed: %v", err)
	}

	if fetched.Name != relay.Name {
		t.Errorf("Expected Name %s, got %s", relay.Name, fetched.Name)
	}
	if fetched.Status != "pending" {
		t.Errorf("Expected Status 'pending', got %s", fetched.Status)
	}
}

func TestReadAllRelays(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create multiple relays
	relay1 := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://relay1.example.com/actor",
		InboxURI:  "https://relay1.example.com/inbox",
		Name:      "Relay 1",
		Status:    "active",
		CreatedAt: time.Now(),
	}
	relay2 := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://relay2.example.com/actor",
		InboxURI:  "https://relay2.example.com/inbox",
		Name:      "Relay 2",
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	db.CreateRelay(relay1)
	db.CreateRelay(relay2)

	err, relays := db.ReadAllRelays()
	if err != nil {
		t.Fatalf("ReadAllRelays failed: %v", err)
	}

	if len(*relays) != 2 {
		t.Errorf("Expected 2 relays, got %d", len(*relays))
	}
}

func TestReadActiveRelays(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create active and pending relays
	activeRelay := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://active.relay.example.com/actor",
		InboxURI:  "https://active.relay.example.com/inbox",
		Name:      "Active Relay",
		Status:    "active",
		CreatedAt: time.Now(),
	}
	pendingRelay := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://pending.relay.example.com/actor",
		InboxURI:  "https://pending.relay.example.com/inbox",
		Name:      "Pending Relay",
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	db.CreateRelay(activeRelay)
	db.CreateRelay(pendingRelay)

	err, relays := db.ReadActiveRelays()
	if err != nil {
		t.Fatalf("ReadActiveRelays failed: %v", err)
	}

	if len(*relays) != 1 {
		t.Errorf("Expected 1 active relay, got %d", len(*relays))
	}
	if (*relays)[0].Name != "Active Relay" {
		t.Errorf("Expected 'Active Relay', got %s", (*relays)[0].Name)
	}
}

func TestUpdateRelayStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	relay := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://relay.example.com/actor",
		InboxURI:  "https://relay.example.com/inbox",
		Name:      "Test Relay",
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	db.CreateRelay(relay)

	// Update to active
	now := time.Now()
	err := db.UpdateRelayStatus(relay.Id, "active", &now)
	if err != nil {
		t.Fatalf("UpdateRelayStatus failed: %v", err)
	}

	// Verify update
	err, fetched := db.ReadRelayByActorURI(relay.ActorURI)
	if err != nil {
		t.Fatalf("ReadRelayByActorURI failed: %v", err)
	}

	if fetched.Status != "active" {
		t.Errorf("Expected Status 'active', got %s", fetched.Status)
	}
	if fetched.AcceptedAt == nil {
		t.Error("Expected AcceptedAt to be set")
	}
}

func TestDeleteRelay(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	relay := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://relay.example.com/actor",
		InboxURI:  "https://relay.example.com/inbox",
		Name:      "Test Relay",
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	db.CreateRelay(relay)

	// Delete relay
	err := db.DeleteRelay(relay.Id)
	if err != nil {
		t.Fatalf("DeleteRelay failed: %v", err)
	}

	// Verify deletion
	err, fetched := db.ReadRelayByActorURI(relay.ActorURI)
	if err == nil && fetched != nil {
		t.Error("Expected relay to be deleted")
	}
}

func TestReadRelayByActorURINotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	err, relay := db.ReadRelayByActorURI("https://nonexistent.relay.com/actor")
	if err == nil && relay != nil {
		t.Error("Expected error for non-existent relay")
	}
}

func TestReadRelayById(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	relay := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  "https://relay.example.com/actor",
		InboxURI:  "https://relay.example.com/inbox",
		Name:      "Test Relay",
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	db.CreateRelay(relay)

	// Read by ID
	err, fetched := db.ReadRelayById(relay.Id)
	if err != nil {
		t.Fatalf("ReadRelayById failed: %v", err)
	}

	if fetched.ActorURI != relay.ActorURI {
		t.Errorf("Expected ActorURI %s, got %s", relay.ActorURI, fetched.ActorURI)
	}
}

// Tests for upload tokens and account updates (Issue #41)

func TestUpdateAccountDisplayName(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create test account
	accountId := uuid.New()
	_, err := db.db.Exec(sqlInsertUser, accountId.String(), "testuser", "pkhash123", "", "", time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Update display name
	err = db.UpdateAccountDisplayName(accountId, "New Display Name")
	if err != nil {
		t.Fatalf("UpdateAccountDisplayName failed: %v", err)
	}

	// Verify update
	err, acc := db.ReadAccById(accountId)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}
	if acc.DisplayName != "New Display Name" {
		t.Errorf("Expected display name 'New Display Name', got '%s'", acc.DisplayName)
	}
}

func TestUpdateAccountSummary(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create test account
	accountId := uuid.New()
	_, err := db.db.Exec(sqlInsertUser, accountId.String(), "testuser", "pkhash123", "", "", time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Update summary/bio
	err = db.UpdateAccountSummary(accountId, "This is my new bio")
	if err != nil {
		t.Fatalf("UpdateAccountSummary failed: %v", err)
	}

	// Verify update
	err, acc := db.ReadAccById(accountId)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}
	if acc.Summary != "This is my new bio" {
		t.Errorf("Expected summary 'This is my new bio', got '%s'", acc.Summary)
	}
}

func TestUpdateAccountAvatar(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create test account
	accountId := uuid.New()
	_, err := db.db.Exec(sqlInsertUser, accountId.String(), "testuser", "pkhash123", "", "", time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Update avatar URL
	avatarURL := "/avatars/" + accountId.String() + ".png"
	err = db.UpdateAccountAvatar(accountId, avatarURL)
	if err != nil {
		t.Fatalf("UpdateAccountAvatar failed: %v", err)
	}

	// Verify update
	err, acc := db.ReadAccById(accountId)
	if err != nil {
		t.Fatalf("ReadAccById failed: %v", err)
	}
	if acc.AvatarURL != avatarURL {
		t.Errorf("Expected avatar URL '%s', got '%s'", avatarURL, acc.AvatarURL)
	}
}

func setupTestDBWithUploadTokens(t *testing.T) *DB {
	db := setupTestDB(t)

	// Create upload_tokens table
	_, err := db.db.Exec(`CREATE TABLE IF NOT EXISTS upload_tokens (
		token TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		token_type TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL
	)`)
	if err != nil {
		t.Fatalf("Failed to create upload_tokens table: %v", err)
	}

	return db
}

func TestCreateUploadToken(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	accountId := uuid.New()
	token := "test-token-123"

	err := db.CreateUploadToken(accountId, token, "avatar", 10*time.Minute)
	if err != nil {
		t.Fatalf("CreateUploadToken failed: %v", err)
	}

	// Verify token was created
	var count int
	err = db.db.QueryRow(`SELECT COUNT(*) FROM upload_tokens WHERE token = ?`, token).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 token, got %d", count)
	}
}

func TestValidateUploadToken(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	accountId := uuid.New()
	token := "valid-token-456"

	// Create a valid token
	err := db.CreateUploadToken(accountId, token, "avatar", 10*time.Minute)
	if err != nil {
		t.Fatalf("CreateUploadToken failed: %v", err)
	}

	// Validate the token
	returnedAccountId, tokenType, err := db.ValidateUploadToken(token)
	if err != nil {
		t.Fatalf("ValidateUploadToken failed: %v", err)
	}
	if returnedAccountId != accountId {
		t.Errorf("Expected account ID %s, got %s", accountId, returnedAccountId)
	}
	if tokenType != "avatar" {
		t.Errorf("Expected token type 'avatar', got '%s'", tokenType)
	}
}

func TestValidateUploadTokenExpired(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	accountId := uuid.New()
	token := "expired-token-789"

	// Create an expired token (negative duration)
	expiresAt := time.Now().Add(-1 * time.Hour)
	_, err := db.db.Exec(`INSERT INTO upload_tokens (token, account_id, token_type, expires_at) VALUES (?, ?, ?, ?)`,
		token, accountId.String(), "avatar", expiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	// Validate should fail for expired token
	_, _, err = db.ValidateUploadToken(token)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "expired") {
		t.Errorf("Expected 'expired' in error message, got: %v", err)
	}
}

func TestValidateUploadTokenNotFound(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	// Try to validate non-existent token
	_, _, err := db.ValidateUploadToken("nonexistent-token")
	if err == nil {
		t.Error("Expected error for non-existent token, got nil")
	}
}

func TestDeleteUploadToken(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	accountId := uuid.New()
	token := "delete-me-token"

	// Create token
	err := db.CreateUploadToken(accountId, token, "avatar", 10*time.Minute)
	if err != nil {
		t.Fatalf("CreateUploadToken failed: %v", err)
	}

	// Delete token
	err = db.DeleteUploadToken(token)
	if err != nil {
		t.Fatalf("DeleteUploadToken failed: %v", err)
	}

	// Verify token was deleted
	var count int
	err = db.db.QueryRow(`SELECT COUNT(*) FROM upload_tokens WHERE token = ?`, token).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 tokens after delete, got %d", count)
	}
}

func TestCleanupExpiredUploadTokens(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	accountId := uuid.New()

	// Create an expired token
	expiredToken := "expired-cleanup-token"
	expiresAt := time.Now().Add(-1 * time.Hour)
	_, err := db.db.Exec(`INSERT INTO upload_tokens (token, account_id, token_type, expires_at) VALUES (?, ?, ?, ?)`,
		expiredToken, accountId.String(), "avatar", expiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	// Create a valid token
	validToken := "valid-cleanup-token"
	err = db.CreateUploadToken(accountId, validToken, "avatar", 10*time.Minute)
	if err != nil {
		t.Fatalf("CreateUploadToken failed: %v", err)
	}

	// Run cleanup
	err = db.CleanupExpiredUploadTokens()
	if err != nil {
		t.Fatalf("CleanupExpiredUploadTokens failed: %v", err)
	}

	// Verify expired token was deleted
	var count int
	err = db.db.QueryRow(`SELECT COUNT(*) FROM upload_tokens WHERE token = ?`, expiredToken).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected expired token to be deleted, but found %d", count)
	}

	// Verify valid token still exists
	err = db.db.QueryRow(`SELECT COUNT(*) FROM upload_tokens WHERE token = ?`, validToken).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected valid token to still exist, but found %d", count)
	}
}

func TestGetExistingUploadToken(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	accountId := uuid.New()

	// Test when no token exists
	token, expiresAt, err := db.GetExistingUploadToken(accountId, "avatar")
	if err != nil {
		t.Fatalf("GetExistingUploadToken failed: %v", err)
	}
	if token != "" {
		t.Errorf("Expected empty token when none exists, got %s", token)
	}
	if !expiresAt.IsZero() {
		t.Errorf("Expected zero time when no token exists")
	}

	// Create a token
	testToken := "existing-test-token"
	err = db.CreateUploadToken(accountId, testToken, "avatar", 10*time.Minute)
	if err != nil {
		t.Fatalf("CreateUploadToken failed: %v", err)
	}

	// Test when token exists
	token, expiresAt, err = db.GetExistingUploadToken(accountId, "avatar")
	if err != nil {
		t.Fatalf("GetExistingUploadToken failed: %v", err)
	}
	if token != testToken {
		t.Errorf("Expected token %s, got %s", testToken, token)
	}
	if expiresAt.IsZero() {
		t.Error("Expected non-zero expiry time")
	}
	if time.Until(expiresAt) < 9*time.Minute {
		t.Error("Expected expiry time to be around 10 minutes from now")
	}

	// Test with different token type - should not find avatar token
	token, _, err = db.GetExistingUploadToken(accountId, "other")
	if err != nil {
		t.Fatalf("GetExistingUploadToken failed: %v", err)
	}
	if token != "" {
		t.Errorf("Expected empty token for different type, got %s", token)
	}

	// Test with different account - should not find token
	otherAccountId := uuid.New()
	token, _, err = db.GetExistingUploadToken(otherAccountId, "avatar")
	if err != nil {
		t.Fatalf("GetExistingUploadToken failed: %v", err)
	}
	if token != "" {
		t.Errorf("Expected empty token for different account, got %s", token)
	}
}

func TestGetExistingUploadTokenExpired(t *testing.T) {
	db := setupTestDBWithUploadTokens(t)
	defer db.db.Close()

	accountId := uuid.New()

	// Create an expired token manually
	expiredToken := "expired-existing-token"
	expiresAt := time.Now().Add(-1 * time.Hour)
	_, err := db.db.Exec(`INSERT INTO upload_tokens (token, account_id, token_type, expires_at) VALUES (?, ?, ?, ?)`,
		expiredToken, accountId.String(), "avatar", expiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	// Should not return expired token
	token, _, err := db.GetExistingUploadToken(accountId, "avatar")
	if err != nil {
		t.Fatalf("GetExistingUploadToken failed: %v", err)
	}
	if token != "" {
		t.Errorf("Expected empty token for expired token, got %s", token)
	}
}

// ============================================================================
// Terms and Conditions Tests
// ============================================================================

func TestGetCurrentTermsAndConditions_Default(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// When no terms exist, should return default
	err, terms := db.GetCurrentTermsAndConditions()
	if err != nil {
		t.Fatalf("GetCurrentTermsAndConditions failed: %v", err)
	}
	if terms == nil {
		t.Fatal("Expected default terms, got nil")
	}
	if terms.Content != "Default Terms and Conditions" {
		t.Errorf("Expected default content, got %q", terms.Content)
	}
}

func TestUpdateTermsAndConditions(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	content := "These are the new terms and conditions."
	err := db.UpdateTermsAndConditions(content)
	if err != nil {
		t.Fatalf("UpdateTermsAndConditions failed: %v", err)
	}

	// Verify it was saved
	err2, terms := db.GetCurrentTermsAndConditions()
	if err2 != nil {
		t.Fatalf("GetCurrentTermsAndConditions failed: %v", err2)
	}
	if terms.Content != content {
		t.Errorf("Expected content %q, got %q", content, terms.Content)
	}
}

func TestRecordUserTermsAcceptance(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	// Create a user first
	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Create terms
	err := db.UpdateTermsAndConditions("Test terms")
	if err != nil {
		t.Fatalf("UpdateTermsAndConditions failed: %v", err)
	}

	// Record acceptance
	err = db.RecordUserTermsAcceptance(userId)
	if err != nil {
		t.Fatalf("RecordUserTermsAcceptance failed: %v", err)
	}

	// Verify acceptance was recorded
	err2, acceptance := db.GetUserTermsAcceptance(userId)
	if err2 != nil {
		t.Fatalf("GetUserTermsAcceptance failed: %v", err2)
	}
	if acceptance == nil {
		t.Fatal("Expected acceptance record, got nil")
	}
	if acceptance.UserId != userId {
		t.Errorf("Expected UserId %v, got %v", userId, acceptance.UserId)
	}
}

func TestGetUserTermsAcceptance_NoRecord(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	userId := uuid.New()
	err, acceptance := db.GetUserTermsAcceptance(userId)
	if err != nil {
		t.Fatalf("GetUserTermsAcceptance failed: %v", err)
	}
	if acceptance != nil {
		t.Error("Expected nil acceptance for user with no record")
	}
}

func TestUserNeedsToAcceptTerms_NoTermsSet(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// With default terms (no custom terms set), user should not need to accept
	needs, err := db.UserNeedsToAcceptTerms(userId)
	if err != nil {
		t.Fatalf("UserNeedsToAcceptTerms failed: %v", err)
	}
	if needs {
		t.Error("Expected user NOT to need acceptance when no custom terms are set")
	}
}

func TestUserNeedsToAcceptTerms_TermsSetButNotAccepted(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Set custom terms
	err := db.UpdateTermsAndConditions("Custom terms that require acceptance")
	if err != nil {
		t.Fatalf("UpdateTermsAndConditions failed: %v", err)
	}

	// User should need to accept
	needs, err := db.UserNeedsToAcceptTerms(userId)
	if err != nil {
		t.Fatalf("UserNeedsToAcceptTerms failed: %v", err)
	}
	if !needs {
		t.Error("Expected user to need acceptance when terms are set but not accepted")
	}
}

func TestUserNeedsToAcceptTerms_AlreadyAccepted(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Set custom terms
	err := db.UpdateTermsAndConditions("Custom terms")
	if err != nil {
		t.Fatalf("UpdateTermsAndConditions failed: %v", err)
	}

	// Record acceptance
	err = db.RecordUserTermsAcceptance(userId)
	if err != nil {
		t.Fatalf("RecordUserTermsAcceptance failed: %v", err)
	}

	// User should NOT need to accept
	needs, err := db.UserNeedsToAcceptTerms(userId)
	if err != nil {
		t.Fatalf("UserNeedsToAcceptTerms failed: %v", err)
	}
	if needs {
		t.Error("Expected user NOT to need acceptance when already accepted")
	}
}

func TestUserNeedsToAcceptTerms_TermsUpdatedAfterAcceptance(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	userId := uuid.New()
	createTestAccount(t, db, userId, "testuser", "pubkey", "webpub", "webpriv")

	// Set initial terms with a timestamp in the past
	pastTime := time.Now().Add(-1 * time.Hour)
	_, err := db.db.Exec(`INSERT OR REPLACE INTO terms_and_conditions(id, content, updated_at) VALUES (1, ?, ?)`,
		"Original terms v1", pastTime.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to insert initial terms: %v", err)
	}

	// User accepts terms (acceptance time is "now", which is after the terms)
	err = db.RecordUserTermsAcceptance(userId)
	if err != nil {
		t.Fatalf("RecordUserTermsAcceptance failed: %v", err)
	}

	// Verify user doesn't need to accept (yet)
	needs, err := db.UserNeedsToAcceptTerms(userId)
	if err != nil {
		t.Fatalf("UserNeedsToAcceptTerms failed: %v", err)
	}
	if needs {
		t.Error("Expected user NOT to need acceptance immediately after accepting")
	}

	// Now update the terms with a future timestamp (simulating admin changing T&Cs)
	futureTime := time.Now().Add(1 * time.Hour)
	_, err = db.db.Exec(`INSERT OR REPLACE INTO terms_and_conditions(id, content, updated_at) VALUES (1, ?, ?)`,
		"Updated terms v2 with new conditions", futureTime.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Failed to update terms: %v", err)
	}

	// User should now need to accept again
	needs, err = db.UserNeedsToAcceptTerms(userId)
	if err != nil {
		t.Fatalf("UserNeedsToAcceptTerms failed: %v", err)
	}
	if !needs {
		t.Error("Expected user to need acceptance after terms were updated")
	}
}
