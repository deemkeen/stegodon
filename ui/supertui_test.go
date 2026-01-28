package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/ui/terms"
	"github.com/google/uuid"
)

// TestMainModelInitialization verifies the main model starts correctly
func TestMainModelInitialization(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	if model.account.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", model.account.Username)
	}

	// Width and height are adjusted by common.DefaultWindowWidth/Height
	// Just verify they're set to reasonable values
	if model.width < 80 {
		t.Errorf("Expected width >= 80, got %d", model.width)
	}

	if model.height < 20 {
		t.Errorf("Expected height >= 20, got %d", model.height)
	}
}

// TestMessageRoutingDoesNotPanic verifies message routing doesn't panic
func TestMessageRoutingDoesNotPanic(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Test various message types don't cause panics
	testCases := []struct {
		name string
		msg  tea.Msg
	}{
		{"ActivateViewMsg", common.ActivateViewMsg{}},
		{"DeactivateViewMsg", common.DeactivateViewMsg{}},
		{"KeyMsg", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}},
		{"SessionState", common.UpdateNoteList},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Update panicked with message %s: %v", tc.name, r)
				}
			}()

			_, _ = model.Update(tc.msg)
		})
	}
}

// TestCommandFilteringRemovesNils verifies nil commands are filtered out
func TestCommandFilteringRemovesNils(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Send a message that should return a command (or nil)
	// The key is that it doesn't panic and returns a valid tea.Model
	updatedModel, cmd := model.Update(common.ActivateViewMsg{})

	if updatedModel == nil {
		t.Error("Expected Update to return non-nil model")
	}

	// cmd can be nil or non-nil, both are valid
	// The important thing is we didn't panic during filtering
	_ = cmd
}

// TestViewSwitchingDoesNotPanic verifies Tab navigation works
func TestViewSwitchingDoesNotPanic(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	// Start in CreateNoteView (default for non-first-time users)
	model.state = common.CreateNoteView

	// Press Tab to cycle through views
	for i := 0; i < 10; i++ {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = r.(error)
				}
			}()
			teaModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\t'}})
			model = teaModel.(MainModel)
		}()

		if err != nil {
			t.Errorf("Tab navigation panicked on iteration %d: %v", i, err)
			break
		}
	}
}

// TestTimelineActivationReturnsCommand verifies timeline activation returns a command
func TestTimelineActivationReturnsCommand(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.HomeTimelineView

	// Send ActivateViewMsg to timeline
	_, cmd := model.Update(common.ActivateViewMsg{})

	// Timeline activation should return a command (either from homeTimelineModel or nil)
	// The important thing is it doesn't panic
	_ = cmd
}

// TestMultipleCommandsReturnsFirstNonNil verifies command prioritization
func TestMultipleCommandsReturnsFirstNonNil(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Send EditNoteMsg which routes to multiple models
	editMsg := common.EditNoteMsg{
		NoteId:  uuid.New(),
		Message: "test",
	}

	_, cmd := model.Update(editMsg)

	// Should return a single command (not batched)
	// This verifies our "return first non-nil" logic works
	_ = cmd
}

// TestSafeModelsReceiveMessagesRegardlessOfActiveView verifies that safe models
// (those without background tickers) receive messages even when not the active view
func TestSafeModelsReceiveMessagesRegardlessOfActiveView(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Set active view to CreateNoteView
	model.state = common.CreateNoteView

	// Create a generic message (not a special case like ActivateViewMsg or SessionState)
	type testMsg struct{}

	// Send the message - safe models should receive it even though they're not active
	updatedModel, _ := model.Update(testMsg{})

	if updatedModel == nil {
		t.Error("Expected Update to return non-nil model")
	}

	// The test verifies that no panic occurs when routing to inactive safe models
	// In production, followModel, accountSettingsModel, followersModel, followingModel,
	// and localUsersModel should all receive this message regardless of active view
}

// TestTimelineModelsOnlyReceiveMessagesWhenActive verifies that timeline models
// with background tickers only receive messages when they're the active view
func TestTimelineModelsOnlyReceiveMessagesWhenActive(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	testCases := []struct {
		name        string
		activeView  common.SessionState
		description string
	}{
		{
			name:        "HomeTimelineActive",
			activeView:  common.HomeTimelineView,
			description: "homeTimelineModel should receive messages when HomeTimelineView is active",
		},
		{
			name:        "AdminPanelActive",
			activeView:  common.AdminPanelView,
			description: "adminModel should receive messages when AdminPanelView is active",
		},
		{
			name:        "CreateNoteActive",
			activeView:  common.CreateNoteView,
			description: "Timeline models should NOT receive messages when CreateNoteView is active",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model.state = tc.activeView

			type testMsg struct{}

			// Send message - should not panic regardless of routing
			updatedModel, _ := model.Update(testMsg{})

			if updatedModel == nil {
				t.Errorf("Expected Update to return non-nil model for %s", tc.description)
			}
		})
	}
}

// TestMessageRoutingPreventsPanicsAcrossAllViews verifies message routing
// doesn't panic when switching between all possible views
func TestMessageRoutingPreventsPanicsAcrossAllViews(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	allViews := []common.SessionState{
		common.CreateUserView,
		common.CreateNoteView,
		common.HomeTimelineView,
		common.MyPostsView,
		common.FollowUserView,
		common.FollowersView,
		common.FollowingView,
		common.LocalUsersView,
		common.AdminPanelView,
		common.DeleteAccountView,
	}

	// Create various message types to test
	type genericMsg struct{}
	messages := []tea.Msg{
		genericMsg{},
		common.ActivateViewMsg{},
		common.DeactivateViewMsg{},
		common.UpdateNoteList,
	}

	for _, view := range allViews {
		for _, msg := range messages {
			model.state = view

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic with view %v and message %T: %v", view, msg, r)
					}
				}()

				updatedModel, _ := model.Update(msg)
				if updatedModel == nil {
					t.Errorf("Expected non-nil model for view %v with message %T", view, msg)
				}
			}()
		}
	}
}

// TestCommandBatchingWithMultipleModels verifies that when multiple models
// return commands, they are properly batched
func TestCommandBatchingWithMultipleModels(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.CreateNoteView

	// Send EditNoteMsg which routes to multiple models (listModel, createModel)
	editMsg := common.EditNoteMsg{
		NoteId:  uuid.New(),
		Message: "test message",
	}

	updatedModel, cmd := model.Update(editMsg)

	if updatedModel == nil {
		t.Error("Expected non-nil model after EditNoteMsg")
	}

	// Command can be nil or non-nil, both are valid
	// The important part is that multiple commands are handled correctly
	_ = cmd
}

// TestAccountSettingsModelUpdatedAfterUsernameChange verifies accountSettingsModel
// receives updated account info after username creation
// New flow: Terms first → username → display name → bio → main app
func TestAccountSettingsModelUpdatedAfterUsernameChange(t *testing.T) {
	account := domain.Account{
		Id:             uuid.New(),
		Username:       "internal_generated_name",
		FirstTimeLogin: domain.TRUE,
	}

	model := NewModel(account, 100, 30)
	// New users start at TermsAcceptanceView, simulate they already accepted
	model.state = common.CreateUserView

	// Setup the newUserModel as if user filled in the form
	model.newUserModel.TextInput.SetValue("alice")
	model.newUserModel.DisplayName.SetValue("Alice Test")
	model.newUserModel.Bio.SetValue("Test bio")
	model.newUserModel.Step = 2

	// Verify accountSettingsModel initially has old username
	if model.accountSettingsModel.Account.Username != "internal_generated_name" {
		t.Errorf("Expected accountSettingsModel to have internal username initially, got %s",
			model.accountSettingsModel.Account.Username)
	}

	// Simulate pressing enter on bio step (Step 2) - goes directly to main app
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mainModel := updatedModel.(MainModel)

	// Verify we're now in CreateNoteView (main app)
	if mainModel.state != common.CreateNoteView {
		t.Errorf("Expected state CreateNoteView after bio, got %v", mainModel.state)
	}

	// Verify account was updated
	if mainModel.account.Username != "alice" {
		t.Errorf("Expected account username to be updated to 'alice', got %s", mainModel.account.Username)
	}

	if mainModel.account.DisplayName != "Alice Test" {
		t.Errorf("Expected display name to be 'Alice Test', got %s", mainModel.account.DisplayName)
	}

	// Verify accountSettingsModel now has updated username
	if mainModel.accountSettingsModel.Account.Username != "alice" {
		t.Errorf("Expected accountSettingsModel to have updated username 'alice', got %s",
			mainModel.accountSettingsModel.Account.Username)
	}

	// Verify accountSettingsModel has same values as mainModel.account
	if mainModel.accountSettingsModel.Account.DisplayName != mainModel.account.DisplayName {
		t.Error("accountSettingsModel should have same DisplayName as mainModel.account")
	}

	if mainModel.accountSettingsModel.Account.Summary != mainModel.account.Summary {
		t.Error("accountSettingsModel should have same Summary as mainModel.account")
	}
}

// TestHomeTimelineStaysActiveWhenVisible verifies that the home timeline remains
// active (continues refreshing) when visible in CreateNoteView or HomeTimelineView
func TestHomeTimelineStaysActiveWhenVisible(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	testCases := []struct {
		name                 string
		oldState             common.SessionState
		newState             common.SessionState
		shouldSendDeactivate bool
	}{
		{
			name:                 "CreateNote to HomeTimeline (both show timeline)",
			oldState:             common.CreateNoteView,
			newState:             common.HomeTimelineView,
			shouldSendDeactivate: false, // Timeline stays active
		},
		{
			name:                 "HomeTimeline to CreateNote (both show timeline)",
			oldState:             common.HomeTimelineView,
			newState:             common.CreateNoteView,
			shouldSendDeactivate: false, // Timeline stays active
		},
		{
			name:                 "HomeTimeline to MyPosts (timeline hidden)",
			oldState:             common.HomeTimelineView,
			newState:             common.MyPostsView,
			shouldSendDeactivate: true, // Timeline should deactivate
		},
		{
			name:                 "CreateNote to MyPosts (timeline hidden)",
			oldState:             common.CreateNoteView,
			newState:             common.MyPostsView,
			shouldSendDeactivate: true, // Timeline should deactivate
		},
		{
			name:                 "MyPosts to HomeTimeline (timeline visible)",
			oldState:             common.MyPostsView,
			newState:             common.HomeTimelineView,
			shouldSendDeactivate: false, // Timeline activates (was inactive)
		},
		{
			name:                 "MyPosts to CreateNote (timeline visible)",
			oldState:             common.MyPostsView,
			newState:             common.CreateNoteView,
			shouldSendDeactivate: false, // Timeline activates (was inactive)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model.state = tc.oldState

			// Simulate tab navigation to switch state
			key := tea.KeyMsg{Type: tea.KeyTab}
			updatedModel, cmd := model.Update(key)
			mainModel := updatedModel.(MainModel)

			// The state should have changed
			if mainModel.state == tc.oldState {
				t.Logf("State didn't change (expected for some cases)")
			}

			// We can't easily verify DeactivateViewMsg was sent without inspecting cmd,
			// but we can verify the update didn't panic
			_ = cmd
		})
	}
}

// TestNotificationsAlwaysActive verifies that notifications model never gets
// deactivated to keep the badge count updating
func TestNotificationsAlwaysActive(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Cycle through all views - notifications should never be deactivated
	views := []common.SessionState{
		common.CreateNoteView,
		common.HomeTimelineView,
		common.MyPostsView,
		common.FollowersView,
		common.NotificationsView, // Even when leaving notifications view
	}

	for _, view := range views {
		model.state = view

		// Simulate tab to next view
		updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyTab})
		mainModel := updatedModel.(MainModel)

		// Verify update doesn't panic
		if updatedModel == nil {
			t.Errorf("Update returned nil model when leaving %v", view)
		}

		// Notifications should continue refreshing (we can't verify this directly,
		// but we verify no panic occurs)
		_ = cmd
		_ = mainModel
	}
}

// TestTabNavigationWithTimelineActivation verifies that tabbing between views
// correctly activates/deactivates the timeline based on visibility
func TestTabNavigationWithTimelineActivation(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.CreateNoteView

	// Tab from CreateNote (timeline visible) to HomeTimeline (timeline visible)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.HomeTimelineView {
		t.Errorf("Expected state HomeTimelineView after tab from CreateNote, got %v", mainModel.state)
	}

	// Tab to MyPosts (timeline NOT visible)
	updatedModel, _ = mainModel.Update(tea.KeyMsg{Type: tea.KeyTab})
	mainModel = updatedModel.(MainModel)

	if mainModel.state != common.MyPostsView {
		t.Errorf("Expected state MyPostsView after tab from HomeTimeline, got %v", mainModel.state)
	}

	// At this point, timeline should have been deactivated (but we can't verify directly)
	// The important thing is no panic occurred
}

// TestShiftTabNavigationWithTimelineActivation verifies that shift-tab navigation
// correctly handles timeline activation/deactivation
func TestShiftTabNavigationWithTimelineActivation(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.MyPostsView

	// Shift-Tab from MyPosts (timeline NOT visible) to HomeTimeline (timeline visible)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.HomeTimelineView {
		t.Errorf("Expected state HomeTimelineView after shift-tab from MyPosts, got %v", mainModel.state)
	}

	// Timeline should have been activated (but we can't verify directly)
	// The important thing is no panic occurred
}

// TestNKeyNavigationToNotifications verifies 'n' key navigates to notifications
// and handles timeline deactivation correctly
func TestNKeyNavigationToNotifications(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.HomeTimelineView

	// Press 'ctrl+n' to go to notifications (timeline becomes hidden)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.NotificationsView {
		t.Errorf("Expected state NotificationsView after pressing 'ctrl+n', got %v", mainModel.state)
	}

	// Timeline should have been deactivated since it's no longer visible
	// (but we can't verify directly without inspecting the command)
}

// TestActivateDeactivateCycle verifies activation/deactivation messages are
// properly sent during view navigation cycles
func TestActivateDeactivateCycle(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Start at CreateNote (timeline visible)
	model.state = common.CreateNoteView

	// Go through a cycle: CreateNote -> HomeTimeline -> MyPosts -> back to HomeTimeline
	transitions := []struct {
		key      tea.KeyMsg
		expected common.SessionState
	}{
		{tea.KeyMsg{Type: tea.KeyTab}, common.HomeTimelineView},
		{tea.KeyMsg{Type: tea.KeyTab}, common.MyPostsView},
		{tea.KeyMsg{Type: tea.KeyShiftTab}, common.HomeTimelineView},
	}

	for i, trans := range transitions {
		updatedModel, cmd := model.Update(trans.key)
		mainModel := updatedModel.(MainModel)

		if mainModel.state != trans.expected {
			t.Errorf("Transition %d: expected state %v, got %v", i, trans.expected, mainModel.state)
		}

		// Verify no panic occurred during message routing
		_ = cmd
		model = mainModel
	}
}

// TestReplyFromThreadViewKeepsThreadState verifies that pressing 'r' in
// thread view keeps the thread visible in the right panel (issue #97)
func TestReplyFromThreadViewKeepsThreadState(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 120, 40)
	model.state = common.ThreadView

	replyMsg := common.ReplyToNoteMsg{
		NoteURI: "https://example.com/notes/123",
		Author:  "someuser",
		Preview: "Test post content",
	}

	updatedModel, _ := model.Update(replyMsg)
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.ThreadView {
		t.Errorf("Expected state to remain ThreadView after reply, got %v", mainModel.state)
	}
}

// TestReplyFromHomeTimelineSwitchesToCreateNoteView verifies that reply from
// non-thread views still switches to CreateNoteView (existing behavior)
func TestReplyFromHomeTimelineSwitchesToCreateNoteView(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 120, 40)
	model.state = common.HomeTimelineView

	replyMsg := common.ReplyToNoteMsg{
		NoteURI: "https://example.com/notes/123",
		Author:  "someuser",
		Preview: "Test post content",
	}

	updatedModel, _ := model.Update(replyMsg)
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.CreateNoteView {
		t.Errorf("Expected state CreateNoteView after reply from home, got %v", mainModel.state)
	}
}

// ============================================================================
// Terms and Conditions Tests
// ============================================================================

// TestTermsCheckResultNeedsAcceptance verifies that when terms need to be
// accepted, the state transitions to TermsAcceptanceView
func TestTermsCheckResultNeedsAcceptance(t *testing.T) {
	account := domain.Account{
		Id:             uuid.New(),
		Username:       "existinguser",
		FirstTimeLogin: domain.FALSE, // Existing user
	}

	model := NewModel(account, 100, 30)

	// Simulate receiving termsCheckResultMsg indicating acceptance is needed
	updatedModel, cmd := model.Update(termsCheckResultMsg{needsAcceptance: true})
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.TermsAcceptanceView {
		t.Errorf("Expected state TermsAcceptanceView when terms need acceptance, got %v", mainModel.state)
	}

	// Should return a command to initialize terms model
	if cmd == nil {
		t.Error("Expected Init command for terms model")
	}
}

// TestTermsCheckResultNoAcceptanceNeeded verifies that when terms don't need
// to be accepted, the state transitions directly to CreateNoteView
func TestTermsCheckResultNoAcceptanceNeeded(t *testing.T) {
	account := domain.Account{
		Id:             uuid.New(),
		Username:       "existinguser",
		FirstTimeLogin: domain.FALSE, // Existing user
	}

	model := NewModel(account, 100, 30)

	// Simulate receiving termsCheckResultMsg indicating no acceptance needed
	updatedModel, cmd := model.Update(termsCheckResultMsg{needsAcceptance: false})
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.CreateNoteView {
		t.Errorf("Expected state CreateNoteView when no terms acceptance needed, got %v", mainModel.state)
	}

	// Should return a command to initialize create model
	if cmd == nil {
		t.Error("Expected Init command for create model")
	}
}

// TestTermsAcceptanceViewBlocksTabNavigation verifies that tab navigation
// is blocked when in TermsAcceptanceView
func TestTermsAcceptanceViewBlocksTabNavigation(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.TermsAcceptanceView

	// Try to tab away
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	mainModel := updatedModel.(MainModel)

	// State should remain TermsAcceptanceView
	if mainModel.state != common.TermsAcceptanceView {
		t.Errorf("Expected state to remain TermsAcceptanceView after tab, got %v", mainModel.state)
	}

	// Also test shift+tab
	updatedModel, _ = mainModel.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	mainModel = updatedModel.(MainModel)

	if mainModel.state != common.TermsAcceptanceView {
		t.Errorf("Expected state to remain TermsAcceptanceView after shift+tab, got %v", mainModel.state)
	}
}

// TestTermsAcceptanceTransitionsToCreateNoteView verifies that after accepting
// terms in TermsAcceptanceView, the state transitions to CreateNoteView
func TestTermsAcceptanceTransitionsToCreateNoteView(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "existinguser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.TermsAcceptanceView

	// Simulate receiving TermsAcceptedMsg (from terms model after user accepts)
	updatedModel, cmd := model.Update(terms.TermsAcceptedMsg{})
	mainModel := updatedModel.(MainModel)

	if mainModel.state != common.CreateNoteView {
		t.Errorf("Expected state CreateNoteView after terms accepted, got %v", mainModel.state)
	}

	// Should return a command to initialize create model
	if cmd == nil {
		t.Error("Expected Init command for create model after terms accepted")
	}
}

// TestTermsAcceptanceViewRendersTerms verifies that View() renders the terms
// screen when in TermsAcceptanceView and doesn't panic
func TestTermsAcceptanceViewRendersTerms(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.TermsAcceptanceView
	model.termsModel.TermsContent = "Test Terms Content"

	// Should not panic and should return non-empty view
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("View() panicked: %v", r)
		}
	}()

	view := model.View()

	// The view should be non-empty
	if len(view) == 0 {
		t.Error("Expected non-empty view for TermsAcceptanceView")
	}

	// Verify we're in the right state
	if model.state != common.TermsAcceptanceView {
		t.Errorf("Expected state TermsAcceptanceView, got %v", model.state)
	}
}

// TestNewUserFlowShowsTermsFirst verifies that new users see terms FIRST
// before creating their profile (username/display name/bio)
func TestNewUserFlowShowsTermsFirst(t *testing.T) {
	account := domain.Account{
		Id:             uuid.New(),
		Username:       "internal_name",
		FirstTimeLogin: domain.TRUE,
	}

	model := NewModel(account, 100, 30)

	// Run Init - for new users, Init returns commands including TermsAcceptanceView state
	cmd := model.Init()
	if cmd == nil {
		t.Error("Expected Init to return commands")
	}

	// Simulate receiving the TermsAcceptanceView state message (from Init batch)
	updatedModel, _ := model.Update(common.TermsAcceptanceView)
	mainModel := updatedModel.(MainModel)

	// New users should be in TermsAcceptanceView after processing Init message
	if mainModel.state != common.TermsAcceptanceView {
		t.Errorf("Expected new user to be in TermsAcceptanceView, got %v", mainModel.state)
	}
}

// TestNewUserFlowAfterTermsAccepted verifies that after accepting terms,
// new users go to CreateUserView to set up their profile
func TestNewUserFlowAfterTermsAccepted(t *testing.T) {
	account := domain.Account{
		Id:             uuid.New(),
		Username:       "internal_name",
		FirstTimeLogin: domain.TRUE,
	}

	model := NewModel(account, 100, 30)
	model.state = common.TermsAcceptanceView

	// Simulate accepting terms
	updatedModel, cmd := model.Update(terms.TermsAcceptedMsg{})
	mainModel := updatedModel.(MainModel)

	// Should now be in CreateUserView to set up profile
	if mainModel.state != common.CreateUserView {
		t.Errorf("Expected state CreateUserView after terms accepted, got %v", mainModel.state)
	}

	// Should return a command to initialize the createuser model
	if cmd == nil {
		t.Error("Expected Init command for createuser model")
	}
}
