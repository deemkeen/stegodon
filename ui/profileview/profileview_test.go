package profileview

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

func TestInitialModel(t *testing.T) {
	accountId := uuid.New()
	width := 120
	height := 40

	m := InitialModel(accountId, width, height, "example.com")

	if m.AccountId != accountId {
		t.Errorf("Expected AccountId %v, got %v", accountId, m.AccountId)
	}
	if m.Width != width {
		t.Errorf("Expected Width %d, got %d", width, m.Width)
	}
	if m.Height != height {
		t.Errorf("Expected Height %d, got %d", height, m.Height)
	}
	if m.ProfileUser != nil {
		t.Error("Expected nil ProfileUser")
	}
	if len(m.Posts) != 0 {
		t.Errorf("Expected empty Posts, got %d", len(m.Posts))
	}
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0, got %d", m.Selected)
	}
	if m.IsFollowing {
		t.Error("Expected IsFollowing to be false")
	}
	if m.loading {
		t.Error("Expected loading to be false")
	}
	if m.LocalDomain != "example.com" {
		t.Errorf("Expected LocalDomain 'example.com', got '%s'", m.LocalDomain)
	}
	if m.AvatarRendered != "" {
		t.Errorf("Expected empty AvatarRendered, got '%s'", m.AvatarRendered)
	}
}

func TestUpdate_ViewProfileMsg(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	profileUserId := uuid.New()
	m, cmd := m.Update(common.ViewProfileMsg{
		Username:  "alice",
		AccountId: profileUserId,
	})

	if !m.loading {
		t.Error("Expected loading to be true after ViewProfileMsg")
	}
	if m.Error != "" {
		t.Error("Expected Error to be cleared")
	}
	if m.Selected != 0 {
		t.Errorf("Expected Selected to be 0, got %d", m.Selected)
	}
	if cmd == nil {
		t.Error("Expected a command to be returned")
	}
}

func TestUpdate_ProfileLoaded(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.loading = true

	account := &domain.Account{
		Id:          uuid.New(),
		Username:    "alice",
		DisplayName: "Alice in Wonderland",
		Summary:     "Exploring the fediverse",
		CreatedAt:   time.Now().Add(-42 * 24 * time.Hour),
	}

	posts := []domain.Note{
		{
			Id:        uuid.New(),
			CreatedBy: "alice",
			Message:   "First post",
			CreatedAt: time.Now().Add(-3 * time.Hour),
		},
		{
			Id:        uuid.New(),
			CreatedBy: "alice",
			Message:   "Second post",
			CreatedAt: time.Now().Add(-1 * 24 * time.Hour),
		},
	}

	m, _ = m.Update(profileLoadedMsg{
		account:     account,
		posts:       posts,
		isFollowing: true,
		err:         nil,
	})

	if m.loading {
		t.Error("Expected loading to be false after profileLoadedMsg")
	}
	if m.ProfileUser == nil {
		t.Fatal("Expected ProfileUser to be set")
	}
	if m.ProfileUser.Username != "alice" {
		t.Errorf("Expected username 'alice', got '%s'", m.ProfileUser.Username)
	}
	if len(m.Posts) != 2 {
		t.Errorf("Expected 2 posts, got %d", len(m.Posts))
	}
	if !m.IsFollowing {
		t.Error("Expected IsFollowing to be true")
	}
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 after load, got %d", m.Selected)
	}
}

func TestUpdate_ProfileLoadedError(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.loading = true

	m, _ = m.Update(profileLoadedMsg{
		err: &testError{"user not found"},
	})

	if m.loading {
		t.Error("Expected loading to be false after error")
	}
	if m.Error != "user not found" {
		t.Errorf("Expected error 'user not found', got '%s'", m.Error)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestUpdate_Navigation(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	m.Posts = []domain.Note{
		{Id: uuid.New(), CreatedBy: "alice", Message: "Post 1"},
		{Id: uuid.New(), CreatedBy: "alice", Message: "Post 2"},
		{Id: uuid.New(), CreatedBy: "alice", Message: "Post 3"},
	}
	m.Selected = 0

	// Move down
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after down, got %d", m.Selected)
	}
	if m.Offset != 1 {
		t.Errorf("Expected Offset 1 after down, got %d", m.Offset)
	}

	// Move down again
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 after down, got %d", m.Selected)
	}
	if m.Offset != 2 {
		t.Errorf("Expected Offset 2 after down, got %d", m.Offset)
	}

	// Try to move past end
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 (stay at end), got %d", m.Selected)
	}

	// Move up
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after up, got %d", m.Selected)
	}
	if m.Offset != 1 {
		t.Errorf("Expected Offset 1 after up, got %d", m.Offset)
	}

	// Move up again
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 after up, got %d", m.Selected)
	}
	if m.Offset != 0 {
		t.Errorf("Expected Offset 0 after up, got %d", m.Offset)
	}

	// Try to move past start
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 (stay at start), got %d", m.Selected)
	}
}

func TestUpdate_EscapeReturnsToLocalUsers(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if cmd == nil {
		t.Fatal("Expected command for escape")
	}

	msg := cmd()
	if msg != common.LocalUsersView {
		t.Errorf("Expected LocalUsersView, got %v", msg)
	}
}

func TestUpdate_EnterEmitsViewThreadMsg(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}

	noteId := uuid.New()
	m.Posts = []domain.Note{
		{
			Id:        noteId,
			CreatedBy: "alice",
			Message:   "A test post",
			CreatedAt: time.Now(),
			ObjectURI: "https://example.com/notes/123",
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command for enter")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	if viewMsg.NoteURI != "https://example.com/notes/123" {
		t.Errorf("Expected NoteURI 'https://example.com/notes/123', got '%s'", viewMsg.NoteURI)
	}
	if viewMsg.NoteID != noteId {
		t.Errorf("Expected NoteID %v, got %v", noteId, viewMsg.NoteID)
	}
	if !viewMsg.IsLocal {
		t.Error("Expected IsLocal to be true")
	}
}

func TestUpdate_EnterWithNoObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}

	noteId := uuid.New()
	m.Posts = []domain.Note{
		{
			Id:        noteId,
			CreatedBy: "alice",
			Message:   "A test post",
			CreatedAt: time.Now(),
			ObjectURI: "", // No ObjectURI
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command for enter")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	expectedURI := "local:" + noteId.String()
	if viewMsg.NoteURI != expectedURI {
		t.Errorf("Expected NoteURI '%s', got '%s'", expectedURI, viewMsg.NoteURI)
	}
}

func TestUpdate_EnterWithNoPosts(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	m.Posts = []domain.Note{} // No posts
	m.Selected = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("Expected no command when there are no posts")
	}
}

func TestUpdate_FollowToggle(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	m.IsFollowing = false

	// Press f to toggle follow - should return a command
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})

	if cmd == nil {
		t.Error("Expected command for follow toggle")
	}
}

func TestUpdate_FollowToggleNoProfile(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = nil

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})

	if cmd != nil {
		t.Error("Expected no command when profile is nil")
	}
}

func TestUpdate_FollowToggledMsg(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}

	// Follow success
	m, cmd := m.Update(followToggledMsg{isFollowing: true, username: "alice"})
	if !m.IsFollowing {
		t.Error("Expected IsFollowing to be true")
	}
	if m.Status != "Following @alice" {
		t.Errorf("Expected status 'Following @alice', got '%s'", m.Status)
	}
	if cmd == nil {
		t.Error("Expected clearStatus command")
	}

	// Unfollow success
	m, cmd = m.Update(followToggledMsg{isFollowing: false, username: "alice"})
	if m.IsFollowing {
		t.Error("Expected IsFollowing to be false")
	}
	if m.Status != "Unfollowed @alice" {
		t.Errorf("Expected status 'Unfollowed @alice', got '%s'", m.Status)
	}
	if cmd == nil {
		t.Error("Expected clearStatus command")
	}
}

func TestUpdate_FollowToggledMsgError(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	m, _ = m.Update(followToggledMsg{err: &testError{"db error"}})

	if !strings.Contains(m.Error, "db error") {
		t.Errorf("Expected error to contain 'db error', got '%s'", m.Error)
	}
}

func TestUpdate_ClearStatusMsg(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Status = "some status"
	m.Error = "some error"

	m, _ = m.Update(clearStatusMsg{})

	if m.Status != "" {
		t.Errorf("Expected empty status, got '%s'", m.Status)
	}
	if m.Error != "" {
		t.Errorf("Expected empty error, got '%s'", m.Error)
	}
}

func TestView_Loading(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.loading = true

	view := m.View()

	if !strings.Contains(view.Content, "Loading profile") {
		t.Error("Expected loading message in view")
	}
}

func TestView_Error(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Error = "user not found"
	m.ProfileUser = nil

	view := m.View()

	if !strings.Contains(view.Content, "user not found") {
		t.Error("Expected error message in view")
	}
}

func TestView_NoProfile(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	view := m.View()

	if !strings.Contains(view.Content, "No profile to display") {
		t.Error("Expected 'No profile to display' message")
	}
}

func TestView_ProfileWithPosts(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "example.com")
	m.ProfileUser = &domain.Account{
		Id:          uuid.New(),
		Username:    "alice",
		DisplayName: "Alice in Wonderland",
		Summary:     "Exploring the fediverse",
		CreatedAt:   time.Now().Add(-42 * 24 * time.Hour),
	}
	m.IsFollowing = true
	m.Posts = []domain.Note{
		{
			Id:        uuid.New(),
			CreatedBy: "alice",
			Message:   "A wonderful test post",
			CreatedAt: time.Now().Add(-3 * time.Hour),
		},
	}
	m.Selected = 0

	view := m.View()

	if !strings.Contains(view.Content, "Alice in Wonderland") {
		t.Error("Expected display name in view")
	}
	if !strings.Contains(view.Content, "@alice") {
		t.Error("Expected handle in view")
	}
	if !strings.Contains(view.Content, "Exploring the fediverse") {
		t.Error("Expected bio in view")
	}
	if !strings.Contains(view.Content, "following") {
		t.Error("Expected follow status in view")
	}
	if !strings.Contains(view.Content, "recent posts (1)") {
		t.Error("Expected recent posts count in view")
	}
	if !strings.Contains(view.Content, "A wonderful test post") {
		t.Error("Expected post content in view")
	}
}

func TestView_ProfileNotFollowing(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:        uuid.New(),
		Username:  "bob",
		CreatedAt: time.Now().Add(-10 * 24 * time.Hour),
	}
	m.IsFollowing = false
	m.Posts = []domain.Note{}

	view := m.View()

	if !strings.Contains(view.Content, "not following") {
		t.Error("Expected 'not following' in view")
	}
	if !strings.Contains(view.Content, "No posts yet") {
		t.Error("Expected 'No posts yet' message")
	}
}

func TestView_DisplayNameFallback(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:          uuid.New(),
		Username:    "bob",
		DisplayName: "", // No display name
		CreatedAt:   time.Now(),
	}
	m.Posts = []domain.Note{}

	view := m.View()

	// Should fall back to username
	if !strings.Contains(view.Content, "bob") {
		t.Error("Expected username as fallback for empty display name")
	}
}

func TestView_StatusMessage(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:        uuid.New(),
		Username:  "alice",
		CreatedAt: time.Now(),
	}
	m.Posts = []domain.Note{}
	m.Status = "Following @alice"

	view := m.View()

	if !strings.Contains(view.Content, "Following @alice") {
		t.Error("Expected status message in view")
	}
}

func TestView_ErrorMessage(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ProfileUser = &domain.Account{
		Id:        uuid.New(),
		Username:  "alice",
		CreatedAt: time.Now(),
	}
	m.Posts = []domain.Note{}
	m.Error = "Failed to toggle follow"

	view := m.View()

	if !strings.Contains(view.Content, "Failed to toggle follow") {
		t.Error("Expected error message in view")
	}
}

func TestFormatTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5m ago",
		},
		{
			name:     "hours ago",
			time:     now.Add(-3 * time.Hour),
			expected: "3h ago",
		},
		{
			name:     "days ago",
			time:     now.Add(-2 * 24 * time.Hour),
			expected: "2d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.time)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
