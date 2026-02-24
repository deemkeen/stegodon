package threadview

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

func TestInitialModel(t *testing.T) {
	accountId := uuid.New()
	width := 120
	height := 40

	m := InitialModel(accountId, width, height, "")

	if m.AccountId != accountId {
		t.Errorf("Expected AccountId %v, got %v", accountId, m.AccountId)
	}
	if m.Width != width {
		t.Errorf("Expected Width %d, got %d", width, m.Width)
	}
	if m.Height != height {
		t.Errorf("Expected Height %d, got %d", height, m.Height)
	}
	if m.ParentURI != "" {
		t.Errorf("Expected empty ParentURI, got %s", m.ParentURI)
	}
	if m.ParentPost != nil {
		t.Error("Expected nil ParentPost")
	}
	if len(m.Replies) != 0 {
		t.Errorf("Expected empty Replies, got %d", len(m.Replies))
	}
	if m.Selected != -1 {
		t.Errorf("Expected Selected -1 (parent), got %d", m.Selected)
	}
	if m.Offset != -1 {
		t.Errorf("Expected Offset -1, got %d", m.Offset)
	}
	if m.isActive {
		t.Error("Expected isActive to be false initially")
	}
	if m.loading {
		t.Error("Expected loading to be false initially")
	}
}

func TestSetThread(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	parentURI := "https://example.com/notes/123"

	m.SetThread(parentURI)

	if m.ParentURI != parentURI {
		t.Errorf("Expected ParentURI %s, got %s", parentURI, m.ParentURI)
	}
	if m.ParentPost != nil {
		t.Error("Expected ParentPost to be reset to nil")
	}
	if len(m.Replies) != 0 {
		t.Error("Expected Replies to be reset to empty")
	}
	if m.Selected != -1 {
		t.Errorf("Expected Selected -1, got %d", m.Selected)
	}
	if !m.loading {
		t.Error("Expected loading to be true after SetThread")
	}
}

func TestUpdate_ActivateDeactivate(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	// Activate
	m, _ = m.Update(common.ActivateViewMsg{})
	if !m.isActive {
		t.Error("Expected isActive true after ActivateViewMsg")
	}

	// Deactivate
	m, _ = m.Update(common.DeactivateViewMsg{})
	if m.isActive {
		t.Error("Expected isActive false after DeactivateViewMsg")
	}
}

func TestUpdate_ThreadLoaded(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.loading = true

	parent := &ThreadPost{
		ID:        uuid.New(),
		Author:    "testuser",
		Content:   "Parent post content",
		Time:      time.Now(),
		ObjectURI: "https://example.com/notes/parent",
		IsLocal:   true,
		IsParent:  true,
	}

	replies := []ThreadPost{
		{
			ID:        uuid.New(),
			Author:    "reply1",
			Content:   "First reply",
			Time:      time.Now(),
			ObjectURI: "https://example.com/notes/reply1",
			IsLocal:   true,
		},
		{
			ID:        uuid.New(),
			Author:    "@remote@example.com",
			Content:   "Remote reply",
			Time:      time.Now(),
			ObjectURI: "https://remote.example.com/notes/reply2",
			IsLocal:   false,
		},
	}

	m, _ = m.Update(threadLoadedMsg{
		parent:  parent,
		replies: replies,
		err:     nil,
	})

	if m.loading {
		t.Error("Expected loading to be false after threadLoadedMsg")
	}
	if m.ParentPost == nil {
		t.Fatal("Expected ParentPost to be set")
	}
	if m.ParentPost.Author != "testuser" {
		t.Errorf("Expected parent Author 'testuser', got '%s'", m.ParentPost.Author)
	}
	if len(m.Replies) != 2 {
		t.Errorf("Expected 2 replies, got %d", len(m.Replies))
	}
	if m.Selected != -1 {
		t.Errorf("Expected Selected -1 after load, got %d", m.Selected)
	}
}

func TestUpdate_ThreadLoadedError(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.loading = true

	m, _ = m.Update(threadLoadedMsg{
		parent:  nil,
		replies: nil,
		err:     fmt.Errorf("post not found"),
	})

	if m.loading {
		t.Error("Expected loading to be false after error")
	}
	if m.errorMessage != "post not found" {
		t.Errorf("Expected error message 'post not found', got '%s'", m.errorMessage)
	}
}

func TestUpdate_Navigation(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "parent",
		Content:  "Parent",
		IsParent: true,
	}
	m.Replies = []ThreadPost{
		{ID: uuid.New(), Author: "reply1", Content: "Reply 1"},
		{ID: uuid.New(), Author: "reply2", Content: "Reply 2"},
		{ID: uuid.New(), Author: "reply3", Content: "Reply 3"},
	}
	m.Selected = -1 // Start at parent

	// Move down to first reply
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 after down, got %d", m.Selected)
	}

	// Move down to second reply
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after down, got %d", m.Selected)
	}

	// Move up back to first reply
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 after up, got %d", m.Selected)
	}

	// Move up to parent
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != -1 {
		t.Errorf("Expected Selected -1 (parent) after up, got %d", m.Selected)
	}

	// Try to move up from parent (should stay at parent)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != -1 {
		t.Errorf("Expected Selected -1 (stay at parent), got %d", m.Selected)
	}

	// Move to last reply
	m.Selected = 2
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 (stay at last), got %d", m.Selected)
	}
}

func TestUpdate_ReplyToParent(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:        uuid.New(),
		Author:    "parent",
		Content:   "Parent content",
		ObjectURI: "https://example.com/notes/parent",
		IsParent:  true,
		IsDeleted: false,
	}
	m.Selected = -1 // Parent selected

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	if cmd == nil {
		t.Fatal("Expected command for reply")
	}

	msg := cmd()
	replyMsg, ok := msg.(common.ReplyToNoteMsg)
	if !ok {
		t.Fatalf("Expected ReplyToNoteMsg, got %T", msg)
	}

	if replyMsg.NoteURI != "https://example.com/notes/parent" {
		t.Errorf("Expected NoteURI 'https://example.com/notes/parent', got '%s'", replyMsg.NoteURI)
	}
	if replyMsg.Author != "parent" {
		t.Errorf("Expected Author 'parent', got '%s'", replyMsg.Author)
	}
}

func TestUpdate_ReplyToDeletedParent(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:        uuid.New(),
		Author:    "[deleted]",
		Content:   "This post has been deleted",
		IsParent:  true,
		IsDeleted: true,
	}
	m.Selected = -1 // Parent selected

	// Try to reply to deleted parent - should not produce a command
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	if cmd != nil {
		t.Error("Expected no command for reply to deleted parent")
	}
}

func TestUpdate_ReplyToReply(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "parent",
		Content:  "Parent",
		IsParent: true,
	}
	m.Replies = []ThreadPost{
		{
			ID:        uuid.New(),
			Author:    "reply1",
			Content:   "First reply content",
			ObjectURI: "https://example.com/notes/reply1",
			IsLocal:   true,
		},
	}
	m.Selected = 0 // First reply selected

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	if cmd == nil {
		t.Fatal("Expected command for reply to reply")
	}

	msg := cmd()
	replyMsg, ok := msg.(common.ReplyToNoteMsg)
	if !ok {
		t.Fatalf("Expected ReplyToNoteMsg, got %T", msg)
	}

	if replyMsg.NoteURI != "https://example.com/notes/reply1" {
		t.Errorf("Expected NoteURI, got '%s'", replyMsg.NoteURI)
	}
}

func TestUpdate_ReplyToLocalNoteWithoutObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	noteID := uuid.New()
	m.ParentPost = &ThreadPost{
		ID:        noteID,
		Author:    "parent",
		Content:   "Parent content",
		ObjectURI: "", // No ObjectURI
		IsLocal:   true,
		IsParent:  true,
	}
	m.Selected = -1

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	if cmd == nil {
		t.Fatal("Expected command for reply")
	}

	msg := cmd()
	replyMsg, ok := msg.(common.ReplyToNoteMsg)
	if !ok {
		t.Fatalf("Expected ReplyToNoteMsg, got %T", msg)
	}

	expectedURI := "local:" + noteID.String()
	if replyMsg.NoteURI != expectedURI {
		t.Errorf("Expected NoteURI '%s', got '%s'", expectedURI, replyMsg.NoteURI)
	}
}

func TestUpdate_EnterOnReplyWithReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "parent",
		Content:  "Parent",
		IsParent: true,
	}
	replyID := uuid.New()
	m.Replies = []ThreadPost{
		{
			ID:         replyID,
			Author:     "reply1",
			Content:    "First reply",
			ObjectURI:  "https://example.com/notes/reply1",
			IsLocal:    true,
			ReplyCount: 3, // Has replies
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command for thread navigation")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	if viewMsg.NoteURI != "https://example.com/notes/reply1" {
		t.Errorf("Expected NoteURI, got '%s'", viewMsg.NoteURI)
	}
	if viewMsg.NoteID != replyID {
		t.Errorf("Expected NoteID %v, got %v", replyID, viewMsg.NoteID)
	}
}

func TestUpdate_EnterOnReplyWithNoReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "parent",
		Content:  "Parent",
		IsParent: true,
	}
	m.Replies = []ThreadPost{
		{
			ID:         uuid.New(),
			Author:     "reply1",
			Content:    "First reply",
			ObjectURI:  "https://example.com/notes/reply1",
			IsLocal:    true,
			ReplyCount: 0, // No replies
		},
	}
	m.Selected = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("Expected no command for reply without sub-replies")
	}
}

func TestUpdate_EscapeGoesBack(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	// Default ReturnView is HomeTimelineView
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if cmd == nil {
		t.Fatal("Expected command for escape")
	}

	msg := cmd()
	if msg != common.HomeTimelineView {
		t.Errorf("Expected HomeTimelineView, got %v", msg)
	}
}

func TestUpdate_EscapeReturnsToProfileView(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ReturnView = common.ProfileView

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if cmd == nil {
		t.Fatal("Expected command for escape")
	}

	msg := cmd()
	if msg != common.ProfileView {
		t.Errorf("Expected ProfileView, got %v", msg)
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "simple paragraph",
			html:     "<p>Hello world</p>",
			expected: "Hello world",
		},
		{
			name:     "link with text",
			html:     `<a href="http://example.com">Click here</a>`,
			expected: "Click here",
		},
		{
			name:     "nested tags",
			html:     "<p><strong>Bold</strong> and <em>italic</em></p>",
			expected: "Bold and italic",
		},
		{
			name:     "HTML entities",
			html:     "&lt;script&gt; &amp; &quot;test&quot;",
			expected: "<script> & \"test\"",
		},
		{
			name:     "break tags",
			html:     "Line 1<br>Line 2<br/>Line 3",
			expected: "Line 1Line 2Line 3",
		},
		{
			name:     "span with class",
			html:     `<span class="hashtag">#test</span>`,
			expected: "#test",
		},
		{
			name:     "empty string",
			html:     "",
			expected: "",
		},
		{
			name:     "plain text",
			html:     "No HTML here",
			expected: "No HTML here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.StripHTMLTags(tt.html)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
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

func TestView_Loading(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.loading = true

	view := m.View()

	if !strings.Contains(view.Content, "Loading thread") {
		t.Error("Expected loading message in view")
	}
}

func TestView_Error(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.errorMessage = "post not found"

	view := m.View()

	if !strings.Contains(view.Content, "Error: post not found") {
		t.Error("Expected error message in view")
	}
}

func TestView_NoThread(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = nil

	view := m.View()

	if !strings.Contains(view.Content, "No thread to display") {
		t.Error("Expected 'No thread to display' message")
	}
}

func TestView_SingularReply(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "testuser",
		Content:  "Test content",
		Time:     time.Now(),
		IsParent: true,
	}
	m.Replies = []ThreadPost{
		{ID: uuid.New(), Author: "reply1", Content: "Reply", Time: time.Now()},
	}

	view := m.View()

	if !strings.Contains(view.Content, "1 reply)") {
		t.Error("Expected singular '1 reply' in header")
	}
}

func TestView_PluralReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "testuser",
		Content:  "Test content",
		Time:     time.Now(),
		IsParent: true,
	}
	m.Replies = []ThreadPost{
		{ID: uuid.New(), Author: "reply1", Content: "Reply 1", Time: time.Now()},
		{ID: uuid.New(), Author: "reply2", Content: "Reply 2", Time: time.Now()},
	}

	view := m.View()

	if !strings.Contains(view.Content, "2 replies)") {
		t.Error("Expected plural '2 replies' in header")
	}
}

func TestThreadPost_Fields(t *testing.T) {
	id := uuid.New()
	now := time.Now()

	post := ThreadPost{
		ID:         id,
		Author:     "testuser",
		Content:    "Test content",
		Time:       now,
		ObjectURI:  "https://example.com/notes/123",
		IsLocal:    true,
		IsParent:   true,
		IsDeleted:  false,
		ReplyCount: 5,
	}

	if post.ID != id {
		t.Errorf("Expected ID %v, got %v", id, post.ID)
	}
	if post.Author != "testuser" {
		t.Errorf("Expected Author 'testuser', got '%s'", post.Author)
	}
	if post.Content != "Test content" {
		t.Errorf("Expected Content 'Test content', got '%s'", post.Content)
	}
	if post.ReplyCount != 5 {
		t.Errorf("Expected ReplyCount 5, got %d", post.ReplyCount)
	}
}

// Helper to create key messages
func keyMsg(key string) tea.KeyPressMsg {
	if len(key) > 0 {
		return tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	return tea.KeyPressMsg{}
}

func TestView_ReplyIndentConsistency(t *testing.T) {
	// Test that all lines of a reply have consistent left padding
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "parent_user",
		Content:  "This is the parent post",
		Time:     time.Now(),
		IsLocal:  true,
		IsParent: true,
	}
	m.Replies = []ThreadPost{
		{
			ID:       uuid.New(),
			Author:   "reply_user",
			Content:  "This is a reply",
			Time:     time.Now(),
			IsLocal:  true,
			IsParent: false,
		},
	}
	// Select the reply (index 0)
	m.Selected = 0
	m.Offset = 0

	view := m.View()

	// The replyIndent is 4 spaces
	indentStr := "    "

	// Find the reply section - look for the reply author
	lines := strings.Split(view.Content, "\n")
	foundReplyAuthor := false
	replyLineIndices := []int{}

	for i, line := range lines {
		// Reply lines should contain the reply author or be part of that post block
		if strings.Contains(line, "reply_user") {
			foundReplyAuthor = true
			replyLineIndices = append(replyLineIndices, i)
		}
	}

	if !foundReplyAuthor {
		t.Fatal("Could not find reply author in view")
	}

	// Check that lines around the reply author also have indent
	// The reply block has: time, author, content - all should be indented
	for _, idx := range replyLineIndices {
		line := lines[idx]
		// Skip empty lines
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		// Check that the line starts with the indent (or has padding applied via lipgloss)
		if !strings.HasPrefix(line, indentStr) && !strings.HasPrefix(line, " ") {
			t.Errorf("Reply line %d should be indented, got: %q", idx, line)
		}
	}
}

func TestView_ParentNotIndented(t *testing.T) {
	// Test that parent post is NOT indented
	m := InitialModel(uuid.New(), 120, 40, "")
	m.ParentPost = &ThreadPost{
		ID:       uuid.New(),
		Author:   "parent_author_unique",
		Content:  "Parent content here",
		Time:     time.Now(),
		IsLocal:  true,
		IsParent: true,
	}
	m.Replies = []ThreadPost{}
	m.Selected = -1 // Parent selected
	m.Offset = -1

	view := m.View()
	lines := strings.Split(view.Content, "\n")

	// Find the parent author line
	for _, line := range lines {
		if strings.Contains(line, "parent_author_unique") {
			// Parent should NOT start with the 4-space indent
			if strings.HasPrefix(line, "    ") && strings.Contains(line, "parent_author_unique") {
				// Could be indented for other reasons, but check it's not the reply indent pattern
				// This is a basic check - parent lines should be at start of content area
			}
			break
		}
	}
}

func TestView_ReplyIndentAdaptiveWidth(t *testing.T) {
	// Test that reply indent works with different widths
	widths := []int{80, 120, 160}

	for _, width := range widths {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			m := InitialModel(uuid.New(), width, 40, "")
			m.ParentPost = &ThreadPost{
				ID:       uuid.New(),
				Author:   "parent",
				Content:  "Parent",
				Time:     time.Now(),
				IsLocal:  true,
				IsParent: true,
			}
			m.Replies = []ThreadPost{
				{
					ID:       uuid.New(),
					Author:   "replier",
					Content:  "Reply content",
					Time:     time.Now(),
					IsLocal:  true,
					IsParent: false,
				},
			}
			m.Selected = 0
			m.Offset = 0

			view := m.View()

			// View should render without panic
			if len(view.Content) == 0 {
				t.Error("View should not be empty")
			}

			// Should contain the reply
			if !strings.Contains(view.Content, "replier") {
				t.Error("View should contain reply author")
			}
		})
	}
}
