package notifications

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
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
	if model.isActive != false {
		t.Errorf("Expected isActive false initially, got %v", model.isActive)
	}
	if len(model.Notifications) != 0 {
		t.Errorf("Expected empty notifications list initially")
	}
}

func TestUpdate_ActivateViewMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.isActive = false

	newModel, cmd := model.Update(common.ActivateViewMsg{})

	if !newModel.isActive {
		t.Errorf("Expected isActive true after ActivateViewMsg")
	}
	if cmd == nil {
		t.Errorf("Expected cmd to load notifications")
	}
}

func TestUpdate_DeactivateViewMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.isActive = true

	newModel, cmd := model.Update(common.DeactivateViewMsg{})

	// Note: After our changes, deactivation doesn't actually stop the ticker
	// It just marks the view as not actively viewing
	if newModel.isActive != false {
		t.Errorf("Expected isActive false after DeactivateViewMsg")
	}
	// Cmd should be nil (we don't stop the ticker anymore)
	if cmd != nil {
		t.Errorf("Expected no cmd after DeactivateViewMsg")
	}
}

func TestUpdate_NotificationsLoadedMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	notifications := []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationLike,
			ActorUsername:    "user1",
			Read:             false,
			CreatedAt:        time.Now(),
		},
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationFollow,
			ActorUsername:    "user2",
			Read:             true,
			CreatedAt:        time.Now(),
		},
	}

	msg := notificationsLoadedMsg{
		notifications: notifications,
		unreadCount:   1,
	}

	newModel, cmd := model.Update(msg)

	if len(newModel.Notifications) != 2 {
		t.Errorf("Expected 2 notifications, got %d", len(newModel.Notifications))
	}
	if newModel.UnreadCount != 1 {
		t.Errorf("Expected unread count 1, got %d", newModel.UnreadCount)
	}
	// Should return ticker command to keep refreshing
	if cmd == nil {
		t.Errorf("Expected ticker cmd after loading notifications")
	}
}

func TestUpdate_RefreshTickMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Ticker should always trigger refresh (even if not active)
	_, cmd := model.Update(refreshTickMsg{})

	if cmd == nil {
		t.Errorf("Expected load notifications cmd after refresh tick")
	}

	// Same behavior whether active or not (this is the key change)
	model.isActive = false
	_, cmd = model.Update(refreshTickMsg{})

	if cmd == nil {
		t.Errorf("Expected load notifications cmd even when inactive (for badge updates)")
	}
}

func TestUpdate_KeyboardNavigation(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Add some notifications
	model.Notifications = []domain.Notification{
		{Id: uuid.New(), ActorUsername: "user1"},
		{Id: uuid.New(), ActorUsername: "user2"},
		{Id: uuid.New(), ActorUsername: "user3"},
	}
	model.Selected = 0

	// Test down navigation
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newModel.Selected != 1 {
		t.Errorf("Expected selected 1 after 'j', got %d", newModel.Selected)
	}

	// Test up navigation
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

	// Add some notifications
	model.Notifications = []domain.Notification{
		{Id: uuid.New(), ActorUsername: "user1"},
		{Id: uuid.New(), ActorUsername: "user2"},
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

func TestView_EmptyNotifications(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.UnreadCount = 0

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty")
	}
	// Should show empty message
	if len(model.Notifications) == 0 && view == "" {
		t.Errorf("Should render something even with no notifications")
	}
}

func TestView_WithNotifications(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			NotificationType: domain.NotificationLike,
			ActorUsername:    "alice",
			ActorDomain:      "",
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.UnreadCount = 1

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty with notifications")
	}
}

func TestUpdate_ViewNotification_WithLocalNote(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	noteId := uuid.New()
	createdAt := time.Now()

	// Add a notification with a local note (has NoteId)
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationLike,
			ActorUsername:    "alice",
			ActorDomain:      "", // Local user (no domain)
			NoteId:           noteId,
			NoteURI:          "https://example.com/note/123",
			NotePreview:      "This is a test note",
			Read:             false,
			CreatedAt:        createdAt,
		},
	}
	model.Selected = 0

	// Press 'v' to view the notification
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if cmd == nil {
		t.Fatal("Expected ViewThreadMsg command to be returned")
	}

	// Execute the command to get the message
	msg := cmd()

	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	// Verify the ViewThreadMsg contains correct data
	if viewMsg.NoteURI != "https://example.com/note/123" {
		t.Errorf("Expected NoteURI 'https://example.com/note/123', got %s", viewMsg.NoteURI)
	}
	if viewMsg.NoteID != noteId {
		t.Errorf("Expected NoteID %v, got %v", noteId, viewMsg.NoteID)
	}
	if !viewMsg.IsLocal {
		t.Errorf("Expected IsLocal true for local note")
	}
	if viewMsg.Author != "alice" {
		t.Errorf("Expected Author 'alice', got %s", viewMsg.Author)
	}
	if viewMsg.Content != "This is a test note" {
		t.Errorf("Expected Content 'This is a test note', got %s", viewMsg.Content)
	}
	if viewMsg.CreatedAt != createdAt {
		t.Errorf("Expected CreatedAt %v, got %v", createdAt, viewMsg.CreatedAt)
	}

	// Model should remain unchanged (no side effects)
	if newModel.Selected != model.Selected {
		t.Errorf("Selection should not change when viewing notification")
	}
}

func TestUpdate_ViewNotification_WithRemoteNote(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	noteId := uuid.New()
	createdAt := time.Now()

	// Add a notification from a remote user
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationMention,
			ActorUsername:    "bob",
			ActorDomain:      "remote.social", // Remote user
			NoteId:           noteId,
			NoteURI:          "https://remote.social/note/456",
			NotePreview:      "Mentioning you here!",
			Read:             false,
			CreatedAt:        createdAt,
		},
	}
	model.Selected = 0

	// Press 'v' to view the notification
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if cmd == nil {
		t.Fatal("Expected ViewThreadMsg command to be returned")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	// Verify remote user format includes domain
	expectedAuthor := "bob@remote.social"
	if viewMsg.Author != expectedAuthor {
		t.Errorf("Expected Author '%s', got %s", expectedAuthor, viewMsg.Author)
	}
	if viewMsg.IsLocal {
		t.Errorf("Expected IsLocal false for remote note")
	}
	if viewMsg.NoteURI != "https://remote.social/note/456" {
		t.Errorf("Expected NoteURI 'https://remote.social/note/456', got %s", viewMsg.NoteURI)
	}
}

func TestUpdate_ViewNotification_ReplyType(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	noteId := uuid.New()

	// Add a reply notification
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationReply,
			ActorUsername:    "charlie",
			ActorDomain:      "another.instance",
			NoteId:           noteId,
			NoteURI:          "https://another.instance/note/789",
			NotePreview:      "Replying to your post",
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.Selected = 0

	// Press 'v' to view the notification
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if cmd == nil {
		t.Fatal("Expected ViewThreadMsg command for reply notification")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	if viewMsg.Content != "Replying to your post" {
		t.Errorf("Expected Content 'Replying to your post', got %s", viewMsg.Content)
	}
}

func TestUpdate_ViewNotification_FollowType_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Add a follow notification (no associated note)
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationFollow,
			ActorUsername:    "dave",
			ActorDomain:      "example.com",
			NoteId:           uuid.Nil,
			NoteURI:          "",
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.Selected = 0

	// Press 'v' on a follow notification
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	// Should not return a command (follow notifications don't have notes)
	if cmd != nil {
		t.Errorf("Expected no command for follow notification, got %v", cmd)
	}
}

func TestUpdate_ViewNotification_NoNoteData_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Add a like notification without note data
func TestUpdate_ViewNotification_EmptyList_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Notifications = []domain.Notification{} // Empty list
	model.Selected = 0

	// Press 'v' when no notifications exist
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	// Should not return a command (no notifications to view)
	if cmd != nil {
		t.Errorf("Expected no command when notifications list is empty, got %v", cmd)
	}
}

func TestUpdate_FollowBack_LocalUser(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	actorId := uuid.New()

	// Add a follow notification from a local user
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationLike,
			ActorUsername:    "eve",
			ActorDomain:      "",
			NoteId:           uuid.Nil,   // No note ID
			NoteURI:          "",          // No note URI
			NotificationType: domain.NotificationFollow,
			ActorId:          actorId,
			ActorUsername:    "localuser",
			ActorDomain:      "", // Local user (no domain)
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.Selected = 0

	// Press 'v' on a notification without note data
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	// Should not return a command (no note to view)
	if cmd != nil {
		t.Errorf("Expected no command when notification has no note data, got %v", cmd)
	}
}

func TestUpdate_ViewNotification_EmptyPreview(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	noteId := uuid.New()

	// Add a notification with empty preview
	// Press 'f' to follow back
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if cmd == nil {
		t.Fatal("Expected follow command to be returned")
	}

	// Status should be set indicating follow in progress
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
	}
	if newModel.Status != "Following @localuser..." {
		t.Errorf("Expected status 'Following @localuser...', got '%s'", newModel.Status)
	}

	// Model selection should remain unchanged
	if newModel.Selected != 0 {
		t.Errorf("Expected selection to remain at 0, got %d", newModel.Selected)
	}
}

func TestUpdate_FollowBack_RemoteUser(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	actorId := uuid.New()

	// Add a follow notification from a remote user
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationLike,
			ActorUsername:    "frank",
			ActorDomain:      "",
			NoteId:           noteId,
			NoteURI:          "https://example.com/note/999",
			NotePreview:      "", // Empty preview
			NotificationType: domain.NotificationFollow,
			ActorId:          actorId,
			ActorUsername:    "remoteuser",
			ActorDomain:      "mastodon.social", // Remote user
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.Selected = 0

	// Press 'v' to view the notification
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if cmd == nil {
		t.Fatal("Expected ViewThreadMsg command even with empty preview")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	// Should use placeholder text for empty preview
	expectedContent := "[No preview available]"
	if viewMsg.Content != expectedContent {
		t.Errorf("Expected Content '%s' for empty preview, got %s", expectedContent, viewMsg.Content)
	}
}

func TestUpdate_ViewNotification_EmptyList_NoAction(t *testing.T) {
	// Press 'f' to follow back
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if cmd == nil {
		t.Fatal("Expected follow command to be returned")
	}

	// Status should be set indicating follow request in progress
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
	}
	expectedStatus := "Requesting to follow @remoteuser@mastodon.social..."
	if newModel.Status != expectedStatus {
		t.Errorf("Expected status '%s', got '%s'", expectedStatus, newModel.Status)
	}
}

func TestUpdate_FollowBack_EmptyList_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Notifications = []domain.Notification{} // Empty list
	model.Selected = 0

	// Press 'v' when no notifications exist
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	// Should not return a command (no notifications to view)
	// Press 'f' when no notifications exist
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Should not return a command (no notifications to follow)
	if cmd != nil {
		t.Errorf("Expected no command when notifications list is empty, got %v", cmd)
	}
}

func TestUpdate_ViewNotification_OutOfBounds_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	noteId := uuid.New()
func TestUpdate_FollowBack_OutOfBounds_NoAction(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	actorId := uuid.New()

	// Add one notification
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationLike,
			ActorUsername:    "grace",
			ActorDomain:      "",
			NoteId:           noteId,
			NoteURI:          "https://example.com/note/111",
			NotePreview:      "Test note",
			NotificationType: domain.NotificationFollow,
			ActorId:          actorId,
			ActorUsername:    "alice",
			ActorDomain:      "",
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.Selected = 5 // Out of bounds

	// Press 'v' when selection is out of bounds
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	// Press 'f' when selection is out of bounds
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Should not return a command (selection is invalid)
	if cmd != nil {
		t.Errorf("Expected no command when selection is out of bounds, got %v", cmd)
	}
}

func TestUpdate_ViewNotification_WithOnlyNoteId(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	noteId := uuid.New()

	// Add a notification with only NoteId (no URI)
func TestUpdate_FollowBack_NonFollowNotification(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	actorId := uuid.New()

	// Add a like notification (not a follow)
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationLike,
			ActorUsername:    "henry",
			ActorDomain:      "",
			NoteId:           noteId,
			NoteURI:          "", // No URI, only ID
			NotePreview:      "Local note only",
			ActorId:          actorId,
			ActorUsername:    "bob",
			ActorDomain:      "",
			NoteId:           uuid.New(),
			NotePreview:      "Liked your post",
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.Selected = 0

	// Press 'v' to view the notification
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if cmd == nil {
		t.Fatal("Expected ViewThreadMsg command when NoteId is present")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	// Should still work with just NoteId
	if viewMsg.NoteID != noteId {
		t.Errorf("Expected NoteID %v, got %v", noteId, viewMsg.NoteID)
	}
	if viewMsg.NoteURI != "" {
		t.Errorf("Expected empty NoteURI, got %s", viewMsg.NoteURI)
	}
}

func TestUpdate_ViewNotification_WithOnlyNoteUri(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Add a notification with only NoteURI (no local ID)
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationMention,
			ActorUsername:    "iris",
			ActorDomain:      "remote.example",
			NoteId:           uuid.Nil, // No local ID
			NoteURI:          "https://remote.example/note/222",
			NotePreview:      "Remote note only",
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.Selected = 0

	// Press 'v' to view the notification
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if cmd == nil {
		t.Fatal("Expected ViewThreadMsg command when NoteURI is present")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	// Should work with just NoteURI
	if viewMsg.NoteURI != "https://remote.example/note/222" {
		t.Errorf("Expected NoteURI 'https://remote.example/note/222', got %s", viewMsg.NoteURI)
	}
	if viewMsg.NoteID != uuid.Nil {
		t.Errorf("Expected Nil NoteID, got %v", viewMsg.NoteID)
	}
	// Press 'f' on a like notification
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Should still allow following (f works on any notification)
	if cmd == nil {
		t.Fatal("Expected follow command to be returned")
	}

	// Status should be set
	if newModel.Status == "" {
		t.Errorf("Expected status message to be set")
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

	newModel, cmd := model.Update(clearStatusMsg{})

	// Should clear both status and error
	if newModel.Status != "" {
		t.Errorf("Expected status to be cleared, got '%s'", newModel.Status)
	}
	if newModel.Error != "" {
		t.Errorf("Expected error to be cleared, got '%s'", newModel.Error)
	}
	if cmd != nil {
		t.Errorf("Expected no command, got %v", cmd)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
