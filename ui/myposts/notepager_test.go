package myposts

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

func TestNewPager(t *testing.T) {
	userId := uuid.New()
	width := 120
	height := 40

	m := NewPager(userId, width, height, "")

	if m.userId != userId {
		t.Errorf("Expected userId %v, got %v", userId, m.userId)
	}
	if m.Width != width {
		t.Errorf("Expected Width %d, got %d", width, m.Width)
	}
	if m.Height != height {
		t.Errorf("Expected Height %d, got %d", height, m.Height)
	}
	if len(m.Notes) != 0 {
		t.Errorf("Expected empty Notes, got %d", len(m.Notes))
	}
	if m.Offset != 0 {
		t.Errorf("Expected Offset 0, got %d", m.Offset)
	}
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0, got %d", m.Selected)
	}
	if m.confirmingDelete {
		t.Error("Expected confirmingDelete to be false")
	}
	if m.deleteTargetId != uuid.Nil {
		t.Errorf("Expected deleteTargetId to be Nil, got %v", m.deleteTargetId)
	}
}

func TestUpdate_NotesLoaded(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")

	now := time.Now()
	notes := []domain.Note{
		{
			Id:        uuid.New(),
			CreatedBy: "testuser",
			Message:   "First note",
			CreatedAt: now,
		},
		{
			Id:        uuid.New(),
			CreatedBy: "testuser",
			Message:   "Second note",
			CreatedAt: now.Add(-time.Hour),
		},
	}

	m, _ = m.Update(notesLoadedMsg{notes: notes})

	if len(m.Notes) != 2 {
		t.Errorf("Expected 2 notes, got %d", len(m.Notes))
	}
	if m.Notes[0].Message != "First note" {
		t.Errorf("Expected first note 'First note', got '%s'", m.Notes[0].Message)
	}
}

func TestUpdate_NotesLoaded_SelectionClamp(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	m.Selected = 10 // Out of bounds

	notes := []domain.Note{
		{Id: uuid.New(), CreatedBy: "user1", Message: "Note 1"},
		{Id: uuid.New(), CreatedBy: "user2", Message: "Note 2"},
	}

	m, _ = m.Update(notesLoadedMsg{notes: notes})

	// Selected should be clamped to last valid index
	if m.Selected != 1 {
		t.Errorf("Expected Selected clamped to 1, got %d", m.Selected)
	}
}

func TestUpdate_Navigation(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	m.Notes = []domain.Note{
		{Id: uuid.New(), CreatedBy: "user1", Message: "Note 1"},
		{Id: uuid.New(), CreatedBy: "user2", Message: "Note 2"},
		{Id: uuid.New(), CreatedBy: "user3", Message: "Note 3"},
	}
	m.Selected = 0

	// Move down with 'j'
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after 'j', got %d", m.Selected)
	}
	if m.Offset != 1 {
		t.Errorf("Expected Offset 1 after 'j', got %d", m.Offset)
	}

	// Move down with down arrow
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 after down, got %d", m.Selected)
	}

	// Try to move past last (should stay)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 (stay at last), got %d", m.Selected)
	}

	// Move up with 'k'
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after 'k', got %d", m.Selected)
	}

	// Move up with up arrow
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 after up, got %d", m.Selected)
	}

	// Try to move before first (should stay)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 (stay at first), got %d", m.Selected)
	}
}

func TestUpdate_EditNote(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	noteId := uuid.New()
	createdAt := time.Now()
	m.Notes = []domain.Note{
		{
			Id:        noteId,
			CreatedBy: "testuser",
			Message:   "Edit me",
			CreatedAt: createdAt,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})

	if cmd == nil {
		t.Fatal("Expected command for edit")
	}

	msg := cmd()
	editMsg, ok := msg.(common.EditNoteMsg)
	if !ok {
		t.Fatalf("Expected EditNoteMsg, got %T", msg)
	}

	if editMsg.NoteId != noteId {
		t.Errorf("Expected NoteId %v, got %v", noteId, editMsg.NoteId)
	}
	if editMsg.Message != "Edit me" {
		t.Errorf("Expected Message 'Edit me', got '%s'", editMsg.Message)
	}
}

func TestUpdate_DeleteConfirmation(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	noteId := uuid.New()
	m.Notes = []domain.Note{
		{Id: noteId, CreatedBy: "testuser", Message: "Delete me"},
	}
	m.Selected = 0

	// Press 'd' to start delete confirmation
	m, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})

	if !m.confirmingDelete {
		t.Error("Expected confirmingDelete true after 'd'")
	}
	if m.deleteTargetId != noteId {
		t.Errorf("Expected deleteTargetId %v, got %v", noteId, m.deleteTargetId)
	}
}

func TestUpdate_DeleteConfirmYes(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	noteId := uuid.New()
	m.Notes = []domain.Note{
		{Id: noteId, CreatedBy: "testuser", Message: "Delete me"},
	}
	m.Selected = 0
	m.confirmingDelete = true
	m.deleteTargetId = noteId

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	if m.confirmingDelete {
		t.Error("Expected confirmingDelete false after 'y'")
	}
	if m.deleteTargetId != uuid.Nil {
		t.Errorf("Expected deleteTargetId Nil, got %v", m.deleteTargetId)
	}
	if cmd == nil {
		t.Error("Expected delete command")
	}
}

func TestUpdate_DeleteConfirmNo(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	noteId := uuid.New()
	m.Notes = []domain.Note{
		{Id: noteId, CreatedBy: "testuser", Message: "Keep me"},
	}
	m.Selected = 0
	m.confirmingDelete = true
	m.deleteTargetId = noteId

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})

	if m.confirmingDelete {
		t.Error("Expected confirmingDelete false after 'n'")
	}
	if m.deleteTargetId != uuid.Nil {
		t.Errorf("Expected deleteTargetId Nil, got %v", m.deleteTargetId)
	}
	if cmd != nil {
		t.Error("Expected no command after cancel")
	}
}

func TestUpdate_DeleteConfirmEscape(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	noteId := uuid.New()
	m.Notes = []domain.Note{
		{Id: noteId, CreatedBy: "testuser", Message: "Keep me"},
	}
	m.confirmingDelete = true
	m.deleteTargetId = noteId

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.confirmingDelete {
		t.Error("Expected confirmingDelete false after escape")
	}
	if cmd != nil {
		t.Error("Expected no command after escape")
	}
}

func TestUpdate_NavigationBlockedDuringConfirm(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	m.Notes = []domain.Note{
		{Id: uuid.New(), CreatedBy: "user1", Message: "Note 1"},
		{Id: uuid.New(), CreatedBy: "user2", Message: "Note 2"},
	}
	m.Selected = 0
	m.confirmingDelete = true
	m.deleteTargetId = m.Notes[0].Id

	// Try to navigate while confirming - should be blocked
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 0 {
		t.Errorf("Expected navigation blocked during confirm, Selected changed to %d", m.Selected)
	}

	// Other keys should also be blocked
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if cmd != nil {
		t.Error("Expected edit blocked during confirm")
	}
}

func TestView_EmptyNotes(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	m.Notes = []domain.Note{}

	view := m.View()

	if !strings.Contains(view.Content, "No notes yet") {
		t.Error("Expected 'No notes yet' message")
	}
	if !strings.Contains(view.Content, "Create your first note") {
		t.Error("Expected 'Create your first note' message")
	}
}

func TestView_NoteCount(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	m.Notes = []domain.Note{
		{Id: uuid.New(), CreatedBy: "user1", Message: "Note 1", CreatedAt: time.Now()},
		{Id: uuid.New(), CreatedBy: "user2", Message: "Note 2", CreatedAt: time.Now()},
		{Id: uuid.New(), CreatedBy: "user3", Message: "Note 3", CreatedAt: time.Now()},
	}

	view := m.View()

	if !strings.Contains(view.Content, "3 notes") {
		t.Error("Expected '3 notes' in header")
	}
}

func TestView_EditedIndicator(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	editedTime := time.Now()
	m.Notes = []domain.Note{
		{
			Id:        uuid.New(),
			CreatedBy: "testuser",
			Message:   "Edited note",
			CreatedAt: time.Now().Add(-time.Hour),
			EditedAt:  &editedTime,
		},
	}

	view := m.View()

	if !strings.Contains(view.Content, "(edited)") {
		t.Error("Expected '(edited)' indicator")
	}
}

func TestView_DeleteConfirmation(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	noteId := uuid.New()
	m.Notes = []domain.Note{
		{Id: noteId, CreatedBy: "testuser", Message: "Delete me", CreatedAt: time.Now()},
	}
	m.Selected = 0
	m.confirmingDelete = true
	m.deleteTargetId = noteId

	view := m.View()

	if !strings.Contains(view.Content, "Delete this note?") {
		t.Error("Expected delete confirmation message")
	}
	if !strings.Contains(view.Content, "y to confirm") {
		t.Error("Expected 'y to confirm' in delete message")
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
			time:     now.Add(-10 * time.Minute),
			expected: "10m ago",
		},
		{
			name:     "hours ago",
			time:     now.Add(-4 * time.Hour),
			expected: "4h ago",
		},
		{
			name:     "days ago",
			time:     now.Add(-5 * 24 * time.Hour),
			expected: "5d ago",
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

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "short",
			maxLen:   10,
			expected: "short",
		},
		{
			name:     "exact length",
			input:    "exact",
			maxLen:   5,
			expected: "exact",
		},
		{
			name:     "needs truncation",
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestMax(t *testing.T) {
	if max(5, 10) != 10 {
		t.Error("max(5, 10) should be 10")
	}
	if max(10, 5) != 10 {
		t.Error("max(10, 5) should be 10")
	}
	if max(5, 5) != 5 {
		t.Error("max(5, 5) should be 5")
	}
	if max(-1, 0) != 0 {
		t.Error("max(-1, 0) should be 0")
	}
}

func TestUpdate_EmptyNotes_NoCrash(t *testing.T) {
	m := NewPager(uuid.New(), 120, 40, "")
	m.Notes = []domain.Note{}

	// Navigation on empty notes should not crash
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})

	// Should complete without panic
	if m.confirmingDelete {
		t.Error("Expected confirmingDelete false on empty list")
	}
}

func TestModel_Fields(t *testing.T) {
	userId := uuid.New()
	deleteTargetId := uuid.New()

	m := Model{
		Notes:            []domain.Note{{Id: uuid.New(), Message: "Test"}},
		Offset:           5,
		Selected:         3,
		Width:            100,
		Height:           50,
		userId:           userId,
		confirmingDelete: true,
		deleteTargetId:   deleteTargetId,
	}

	if len(m.Notes) != 1 {
		t.Errorf("Expected 1 note, got %d", len(m.Notes))
	}
	if m.Offset != 5 {
		t.Errorf("Expected Offset 5, got %d", m.Offset)
	}
	if m.Selected != 3 {
		t.Errorf("Expected Selected 3, got %d", m.Selected)
	}
	if m.userId != userId {
		t.Errorf("Expected userId %v, got %v", userId, m.userId)
	}
	if !m.confirmingDelete {
		t.Error("Expected confirmingDelete true")
	}
	if m.deleteTargetId != deleteTargetId {
		t.Errorf("Expected deleteTargetId %v, got %v", deleteTargetId, m.deleteTargetId)
	}
}
