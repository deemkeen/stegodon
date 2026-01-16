package followers

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

func TestInitialModel(t *testing.T) {
	accountId := uuid.New()
	width := 100
	height := 40

	model := InitialModel(accountId, width, height)

	if model.AccountId != accountId {
		t.Errorf("Expected AccountId %v, got %v", accountId, model.AccountId)
	}
	if model.Width != width {
		t.Errorf("Expected Width %d, got %d", width, model.Width)
	}
	if model.Height != height {
		t.Errorf("Expected Height %d, got %d", height, model.Height)
	}
	if len(model.Followers) != 0 {
		t.Errorf("Expected empty followers list initially")
	}
	if model.Selected != 0 {
		t.Errorf("Expected Selected to be 0 initially, got %d", model.Selected)
	}
	if model.Offset != 0 {
		t.Errorf("Expected Offset to be 0 initially, got %d", model.Offset)
	}
}

func TestUpdate_FollowersLoadedMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Selected = 5
	model.Offset = 10

	followers := []domain.Follow{
		{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			IsLocal:   true,
		},
		{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			IsLocal:   false,
		},
	}

	msg := followersLoadedMsg{followers: followers}
	newModel, cmd := model.Update(msg)

	if len(newModel.Followers) != 2 {
		t.Errorf("Expected 2 followers, got %d", len(newModel.Followers))
	}
	// Should reset selection and offset when loading
	if newModel.Selected != 0 {
		t.Errorf("Expected Selected to be reset to 0, got %d", newModel.Selected)
	}
	if newModel.Offset != 0 {
		t.Errorf("Expected Offset to be reset to 0, got %d", newModel.Offset)
	}
	if cmd != nil {
		t.Errorf("Expected no command after loading followers, got %v", cmd)
	}
}

func TestUpdate_KeyboardNavigation(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Followers = []domain.Follow{
		{Id: uuid.New(), AccountId: uuid.New(), IsLocal: true},
		{Id: uuid.New(), AccountId: uuid.New(), IsLocal: true},
		{Id: uuid.New(), AccountId: uuid.New(), IsLocal: false},
	}
	model.Selected = 0

	// Test down navigation with 'j'
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newModel.Selected != 1 {
		t.Errorf("Expected selected 1 after 'j', got %d", newModel.Selected)
	}

	// Test up navigation with 'k'
	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if newModel.Selected != 0 {
		t.Errorf("Expected selected 0 after 'k', got %d", newModel.Selected)
	}

	// Test down with arrow key
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newModel.Selected != 1 {
		t.Errorf("Expected selected 1 after down arrow, got %d", newModel.Selected)
	}

	// Test up with arrow key
	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newModel.Selected != 0 {
		t.Errorf("Expected selected 0 after up arrow, got %d", newModel.Selected)
	}
}

func TestUpdate_SelectionBounds(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Followers = []domain.Follow{
		{Id: uuid.New(), AccountId: uuid.New(), IsLocal: true},
		{Id: uuid.New(), AccountId: uuid.New(), IsLocal: true},
	}
	model.Selected = 0

	// Try to go up from 0 (should stay at 0)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if newModel.Selected != 0 {
		t.Errorf("Expected selected to stay at 0 when at top")
	}

	// Go to last item
	model.Selected = 1
	// Try to go down from last (should stay at last)
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newModel.Selected != 1 {
		t.Errorf("Expected selected to stay at 1 when at bottom")
	}
}

func TestUpdate_FollowBack_LocalFollower(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	followerId := uuid.New()

	model.Followers = []domain.Follow{
		{
			Id:        uuid.New(),
			AccountId: followerId,
			IsLocal:   true,
		},
	}
	model.Selected = 0

	// Press 'f' to follow back
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if cmd == nil {
		t.Fatal("Expected follow command to be returned")
	}

	// Status should be set
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
	}
	expectedStatus := "Following local user..."
	if newModel.Status != expectedStatus {
		t.Errorf("Expected status '%s', got '%s'", expectedStatus, newModel.Status)
	}
}

func TestUpdate_FollowBack_RemoteFollower(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	followerId := uuid.New()

	model.Followers = []domain.Follow{
		{
			Id:        uuid.New(),
			AccountId: followerId,
			IsLocal:   false, // Remote follower
		},
	}
	model.Selected = 0

	// Press 'f' to follow back
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if cmd == nil {
		t.Fatal("Expected follow command to be returned")
	}

	// Status should be set
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
	}
	expectedStatus := "Requesting to follow remote user..."
	if newModel.Status != expectedStatus {
		t.Errorf("Expected status '%s', got '%s'", expectedStatus, newModel.Status)
	}
}

func TestUpdate_FollowBack_EmptyList_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Followers = []domain.Follow{} // Empty list
	model.Selected = 0

	// Press 'f' when no followers exist
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Should not return a command (no followers to follow back)
	if cmd != nil {
		t.Errorf("Expected no command when followers list is empty, got %v", cmd)
	}
}

func TestUpdate_FollowBack_OutOfBounds_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	followerId := uuid.New()

	model.Followers = []domain.Follow{
		{
			Id:        uuid.New(),
			AccountId: followerId,
			IsLocal:   true,
		},
	}
	model.Selected = 5 // Out of bounds

	// Press 'f' when selection is out of bounds
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Should not return a command (selection is invalid)
	if cmd != nil {
		t.Errorf("Expected no command when selection is out of bounds, got %v", cmd)
	}
}

func TestUpdate_FollowResultMsg_Success(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Simulate successful follow
	msg := followResultMsg{
		username: "@testuser",
		err:      nil,
	}

	newModel, cmd := model.Update(msg)

	// Should show success status
	expectedStatus := "✓ Following @testuser"
	if newModel.Status != expectedStatus {
		t.Errorf("Expected status '%s', got '%s'", expectedStatus, newModel.Status)
	}
	if newModel.Error != "" {
		t.Errorf("Expected no error, got '%s'", newModel.Error)
	}

	// Should schedule status clear
	if cmd == nil {
		t.Errorf("Expected clear status command")
	}
}

func TestUpdate_FollowResultMsg_AlreadyFollowing(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Simulate already following error
	msg := followResultMsg{
		username: "@testuser",
		err:      &testError{msg: "already following"},
	}

	newModel, cmd := model.Update(msg)

	// Should show info status (not error)
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
	}
	if newModel.Error != "" {
		t.Errorf("Expected no error for 'already following', got '%s'", newModel.Error)
	}

	// Should schedule status clear
	if cmd == nil {
		t.Errorf("Expected clear status command")
	}
}

func TestUpdate_FollowResultMsg_FollowPending(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Simulate follow pending error
	msg := followResultMsg{
		username: "@testuser@remote.social",
		err:      &testError{msg: "follow pending"},
	}

	newModel, cmd := model.Update(msg)

	// Should show info status (not error)
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
	}
	if newModel.Error != "" {
		t.Errorf("Expected no error for 'follow pending', got '%s'", newModel.Error)
	}

	// Should schedule status clear
	if cmd == nil {
		t.Errorf("Expected clear status command")
	}
}

func TestUpdate_FollowResultMsg_SelfFollowNotAllowed(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Simulate self-follow error
	msg := followResultMsg{
		username: "@me",
		err:      &testError{msg: "self-follow not allowed"},
	}

	newModel, _ := model.Update(msg)

	// Should show info status (not error)
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
	}
	expectedStatus := "ℹ Self-follow not allowed"
	if newModel.Status != expectedStatus {
		t.Errorf("Expected status '%s', got '%s'", expectedStatus, newModel.Status)
	}
	if newModel.Error != "" {
		t.Errorf("Expected no error for 'self-follow', got '%s'", newModel.Error)
	}
}

func TestUpdate_FollowResultMsg_GenericError(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Simulate generic follow error
	msg := followResultMsg{
		username: "@testuser",
		err:      &testError{msg: "network error"},
	}

	newModel, cmd := model.Update(msg)

	// Should show error
	if newModel.Error == "" {
		t.Errorf("Expected error message to be set")
	}
	if newModel.Status != "" {
		t.Errorf("Expected no status for generic error, got '%s'", newModel.Status)
	}

	// Should schedule status clear
	if cmd == nil {
		t.Errorf("Expected clear status command")
	}
}

func TestUpdate_ClearStatusMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Status = "Test status"
	model.Error = "Test error"

	newModel, _ := model.Update(clearStatusMsg{})

	// Should clear both status and error
	if newModel.Status != "" {
		t.Errorf("Expected status to be cleared, got '%s'", newModel.Status)
	}
	if newModel.Error != "" {
		t.Errorf("Expected error to be cleared, got '%s'", newModel.Error)
	}
}

func TestView_EmptyFollowers(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Followers = []domain.Follow{}

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty")
	}
	// Should show empty message about no followers
	if len(model.Followers) == 0 && view == "" {
		t.Errorf("Should render something even with no followers")
	}
}

func TestView_WithFollowers(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Followers = []domain.Follow{
		{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			IsLocal:   true,
		},
		{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			IsLocal:   false,
		},
	}

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty with followers")
	}
}

func TestView_WithStatusMessage(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Followers = []domain.Follow{
		{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			IsLocal:   true,
		},
	}
	model.Status = "Test status message"

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty")
	}
	// Note: We can't easily test if the status is actually rendered
	// without more complex view parsing, but we verify the view renders
}

func TestView_WithErrorMessage(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Followers = []domain.Follow{
		{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			IsLocal:   true,
		},
	}
	model.Error = "Test error message"

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty")
	}
	// Note: We can't easily test if the error is actually rendered
	// without more complex view parsing, but we verify the view renders
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
