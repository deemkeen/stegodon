package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

func TestFollow_LocalUser_TextMode(t *testing.T) {
	targetId := uuid.New()
	db := &mockDatabase{
		localAccounts: map[string]*domain.Account{
			"alice": {Id: targetId, Username: "alice"},
		},
	}
	// Override ReadAccByUsername for this test
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"follow", "@alice"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Now following @alice") {
		t.Errorf("Expected 'Now following @alice' in output, got: %s", result)
	}
	if !db.followCreated {
		t.Error("Expected follow to be created")
	}
}

func TestFollow_LocalUser_JSONMode(t *testing.T) {
	targetId := uuid.New()
	db := &mockDatabase{
		localAccounts: map[string]*domain.Account{
			"alice": {Id: targetId, Username: "alice"},
		},
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"follow", "@alice", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp FollowResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.Username != "alice" {
		t.Errorf("Expected username 'alice', got %s", resp.Username)
	}
	if !resp.Following {
		t.Error("Expected following to be true")
	}
	if resp.Domain != "" {
		t.Errorf("Expected empty domain for local user, got %s", resp.Domain)
	}
}

func TestFollow_LocalUser_Unfollow(t *testing.T) {
	targetId := uuid.New()
	db := &mockDatabase{
		localAccounts: map[string]*domain.Account{
			"alice": {Id: targetId, Username: "alice"},
		},
		isFollowing: true, // Already following
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"follow", "@alice", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp FollowResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.Following {
		t.Error("Expected following to be false (unfollowed)")
	}
	if !db.followDeleted {
		t.Error("Expected follow to be deleted")
	}
}

func TestFollow_LocalUser_NotFound(t *testing.T) {
	db := &mockDatabase{
		localAccounts: map[string]*domain.Account{},
	}
	session := newMockSession("")
	account := &domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}
	conf := &util.AppConfig{}

	// Create a handler that returns "not found" for unknown users
	handler := &Handler{
		session: session,
		db:      &mockDatabaseUserNotFound{mockDatabase: db},
		account: account,
		conf:    conf,
	}

	err := handler.Execute([]string{"follow", "@nonexistent"})
	if err == nil {
		t.Error("Expected error for nonexistent user")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestFollow_SelfFollow(t *testing.T) {
	db := &mockDatabase{}
	session := newMockSession("")
	account := &domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}
	conf := &util.AppConfig{}

	// Create handler that returns the same account for "testuser"
	handler := &Handler{
		session: session,
		db:      &mockDatabaseSelfFollow{mockDatabase: db, selfAccount: account},
		account: account,
		conf:    conf,
	}

	err := handler.Execute([]string{"follow", "@testuser"})
	if err == nil {
		t.Error("Expected error for self-follow")
	}
	if !strings.Contains(err.Error(), "cannot follow yourself") {
		t.Errorf("Expected 'cannot follow yourself' error, got: %v", err)
	}
}

func TestFollow_EmptyUser(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"follow"})
	if err == nil {
		t.Error("Expected error for empty user")
	}
}

func TestFollow_InvalidFormat(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"follow", "@"})
	if err == nil {
		t.Error("Expected error for invalid format")
	}
}

func TestFollow_RemoteUser_NoActivityPub(t *testing.T) {
	db := &mockDatabase{}
	session := newMockSession("")
	account := &domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}
	conf := &util.AppConfig{}
	conf.Conf.WithAp = false // ActivityPub disabled

	handler := &Handler{
		session: session,
		db:      db,
		account: account,
		conf:    conf,
	}

	err := handler.Execute([]string{"follow", "user@mastodon.social"})
	if err == nil {
		t.Error("Expected error when ActivityPub is disabled")
	}
	if !strings.Contains(err.Error(), "ActivityPub is not enabled") {
		t.Errorf("Expected 'ActivityPub is not enabled' error, got: %v", err)
	}
}

// mockDatabaseUserNotFound wraps mockDatabase to return "not found" for unknown users
type mockDatabaseUserNotFound struct {
	*mockDatabase
}

func (m *mockDatabaseUserNotFound) ReadAccByUsername(username string) (error, *domain.Account) {
	return nil, nil // User not found
}

// mockDatabaseSelfFollow wraps mockDatabase to return the same account
type mockDatabaseSelfFollow struct {
	*mockDatabase
	selfAccount *domain.Account
}

func (m *mockDatabaseSelfFollow) ReadAccByUsername(username string) (error, *domain.Account) {
	if username == m.selfAccount.Username {
		return nil, m.selfAccount
	}
	return nil, nil
}
