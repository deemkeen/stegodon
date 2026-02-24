package writenote

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

func TestEmptyNoteValidation(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Test 1: Try to save empty note
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	if m.Error == "" {
		t.Error("Expected error message when saving empty note")
	}
	if m.Error != "Cannot save an empty note" {
		t.Errorf("Expected error 'Cannot save an empty note', got '%s'", m.Error)
	}

	// Test 2: Type something and verify error clears
	m, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})

	if m.Error != "" {
		t.Errorf("Expected error to clear when typing, but still has: '%s'", m.Error)
	}
}

func TestEmptyNoteWithWhitespaceValidation(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set textarea to only whitespace
	m.Textarea.SetValue("   \n\n  \t  ")

	// Try to save (should fail because NormalizeInput will trim to empty)
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	if m.Error == "" {
		t.Error("Expected error message when saving whitespace-only note")
	}
	if m.Error != "Cannot save an empty note" {
		t.Errorf("Expected error 'Cannot save an empty note', got '%s'", m.Error)
	}
}

func TestValidNoteDoesNotShowError(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set valid content
	m.Textarea.SetValue("This is a valid note")

	// Try to save (should succeed and clear any previous errors)
	oldValue := m.Textarea.Value()
	m, cmd := m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	if m.Error != "" {
		t.Errorf("Expected no error for valid note, got: '%s'", m.Error)
	}

	// Textarea should be cleared after save
	if m.Textarea.Value() != "" {
		t.Errorf("Expected textarea to be cleared after save, got: '%s'", m.Textarea.Value())
	}

	// Should return a command (the save command)
	if cmd == nil {
		t.Error("Expected save command to be returned")
	}

	// Verify the original content was valid
	if oldValue == "" {
		t.Error("Test setup error: should have had valid content")
	}
}

func TestErrorClearsOnBackspace(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Try to save empty note to trigger error
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	if m.Error == "" {
		t.Fatal("Setup error: expected error from empty save")
	}

	// Press backspace
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	if m.Error != "" {
		t.Errorf("Expected error to clear on backspace, but still has: '%s'", m.Error)
	}
}

func TestErrorInEditMode(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Enter edit mode
	noteId := uuid.New()
	m, _ = m.Update(common.EditNoteMsg{
		NoteId:    noteId,
		Message:   "Original message",
		CreatedAt: time.Now(),
	})

	if !m.isEditing {
		t.Fatal("Setup error: should be in edit mode")
	}

	// Clear the textarea to make it empty
	m.Textarea.SetValue("")

	// Try to save empty note in edit mode
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	if m.Error == "" {
		t.Error("Expected error when saving empty note in edit mode")
	}
	if m.Error != "Cannot save an empty note" {
		t.Errorf("Expected error 'Cannot save an empty note', got '%s'", m.Error)
	}

	// Should still be in edit mode because save failed
	if !m.isEditing {
		t.Error("Should still be in edit mode after failed save")
	}
}

func TestViewShowsError(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set an error
	m.Error = "Cannot save an empty note"

	// Render the view
	view := m.View()

	// Check that error message appears in view
	if !strings.Contains(view.Content, "Cannot save an empty note") {
		t.Error("Expected error message to appear in view")
	}
}

func TestViewWithoutError(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Render the view without error
	view := m.View()

	// Check that no error styling appears
	if strings.Contains(view.Content, "Cannot save") {
		t.Error("Should not show error message when no error exists")
	}
}

func TestCharCountIsCorrect(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Initial count should be maxLetters
	if m.CharCount() != maxLetters {
		t.Errorf("Expected initial char count to be %d, got %d", maxLetters, m.CharCount())
	}

	// Add some text
	testText := "Hello"
	m.Textarea.SetValue(testText)
	m.lettersLeft = m.CharCount()

	expectedLeft := maxLetters - len(testText)
	if m.lettersLeft != expectedLeft {
		t.Errorf("Expected %d characters left after typing '%s', got %d",
			expectedLeft, testText, m.lettersLeft)
	}
}

func TestCharCountWithMarkdownLinks(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set text with markdown link
	// [stegodon](https://github.com/deemkeen/stegodon) = 8 visible chars (just "stegodon")
	testText := "[stegodon](https://github.com/deemkeen/stegodon)"
	m.Textarea.SetValue(testText)
	m.lettersLeft = m.CharCount()

	// Should only count "stegodon" (8 chars), not the URL
	expectedLeft := maxLetters - 8
	if m.lettersLeft != expectedLeft {
		t.Errorf("Expected %d characters left (only counting visible text), got %d",
			expectedLeft, m.lettersLeft)
	}
}

func Test1000CharacterValidation(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Create a note that exceeds 1000 characters
	longText := strings.Repeat("a", 1001)
	m.Textarea.SetValue(longText)

	// Try to save
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	// Should show error
	if m.Error == "" {
		t.Error("Expected error for note exceeding 1000 characters")
	}

	if !strings.Contains(m.Error, "too long") {
		t.Errorf("Expected error about note being too long, got: %s", m.Error)
	}
}

func Test1000CharacterValidationWithMarkdown(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Create a note with markdown that exceeds 1000 chars total
	// Visible: "Check link" (10 chars) but full markdown is 1007 chars
	longURL := strings.Repeat("x", 990)
	testText := fmt.Sprintf("Check [link](%s)", longURL)
	m.Textarea.SetValue(testText)

	// Try to save
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	// Should show error because full text > 1000
	if m.Error == "" {
		t.Error("Expected error for note with markdown exceeding 1000 characters")
	}
}

func TestURLAutoPasteConversion(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Simulate pasting a URL (textarea contains only the URL)
	testURL := "https://github.com/deemkeen/stegodon"
	m.Textarea.SetValue(testURL)

	// Trigger update to process auto-conversion
	m, _ = m.Update(tea.KeyPressMsg{Code: 0, Text: ""})

	// Should have been converted to markdown format with "Link" as text
	value := m.Textarea.Value()
	expectedMarkdown := fmt.Sprintf("[Link](%s)", testURL)

	if value != expectedMarkdown {
		t.Errorf("Expected URL to be auto-converted to %q, got %q", expectedMarkdown, value)
	}
}

func TestURLAutoPasteDoesNotConvertPartialText(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set text that contains a URL but is not ONLY a URL
	testText := "Check out https://github.com/deemkeen/stegodon project"
	m.Textarea.SetValue(testText)

	// Trigger update
	m, _ = m.Update(tea.KeyPressMsg{Code: 0, Text: ""})

	// Should NOT be auto-converted (not a pure URL paste)
	value := m.Textarea.Value()
	if value != testText {
		t.Errorf("Expected text to remain unchanged, got %q", value)
	}
}

func TestGetMarkdownLinkCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "no links",
			input: "Just plain text",
			want:  0,
		},
		{
			name:  "single link",
			input: "[stegodon](https://github.com/deemkeen/stegodon)",
			want:  1,
		},
		{
			name:  "multiple links",
			input: "Visit [site1](https://example.com) and [site2](https://test.com)",
			want:  2,
		},
		{
			name:  "incomplete link missing closing bracket",
			input: "[text](https://example.com",
			want:  0,
		},
		{
			name:  "incomplete link missing url",
			input: "[text]()",
			want:  0, // Empty URL doesn't match the regex pattern
		},
		{
			name:  "text with brackets but not links",
			input: "Some [text] with (parentheses)",
			want:  0,
		},
		{
			name:  "link at start",
			input: "[Link](https://example.com) and more text",
			want:  1,
		},
		{
			name:  "link at end",
			input: "Check this [Link](https://example.com)",
			want:  1,
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.GetMarkdownLinkCount(tt.input)
			if got != tt.want {
				t.Errorf("GetMarkdownLinkCount(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestViewShowsLinkIndicator(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Test 1: No links - should not show indicator
	m.Textarea.SetValue("Just plain text")
	view := m.View()
	if strings.Contains(view.Content, "markdown link") {
		t.Error("Should not show link indicator when no links present")
	}

	// Test 2: Single link - should show indicator with singular
	m.Textarea.SetValue("[Link](https://example.com)")
	view = m.View()
	if !strings.Contains(view.Content, "✓ 1 markdown link detected") {
		t.Error("Should show '1 markdown link detected' for single link")
	}
	if strings.Contains(view.Content, "links detected") {
		t.Error("Should use singular 'link' not plural 'links' for single link")
	}

	// Test 3: Multiple links - should show indicator with plural
	m.Textarea.SetValue("[site1](https://example.com) and [site2](https://test.com)")
	view = m.View()
	if !strings.Contains(view.Content, "✓ 2 markdown links detected") {
		t.Error("Should show '2 markdown links detected' for multiple links")
	}
	if !strings.Contains(view.Content, "links detected") {
		t.Error("Should use plural 'links' for multiple links")
	}

	// Test 4: Indicator should appear with green checkmark
	if !strings.Contains(view.Content, "✓") {
		t.Error("Link indicator should contain checkmark symbol")
	}
}

func TestViewLinkIndicatorWithError(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set a link and an error
	m.Textarea.SetValue("[Link](https://example.com)")
	m.Error = "Some error message"

	view := m.View()

	// Both should be present in the view
	if !strings.Contains(view.Content, "✓ 1 markdown link detected") {
		t.Error("Link indicator should be present even when error exists")
	}
	if !strings.Contains(view.Content, "Some error message") {
		t.Error("Error message should be present")
	}

	// Link indicator should come before error (based on View() order)
	linkPos := strings.Index(view.Content, "✓ 1 markdown link")
	errorPos := strings.Index(view.Content, "Some error message")
	if linkPos > errorPos {
		t.Error("Link indicator should appear before error message in view")
	}
}

// Tests for MentionCandidate

func TestMentionCandidateFullMention(t *testing.T) {
	tests := []struct {
		name     string
		username string
		domain   string
		isLocal  bool
		want     string
	}{
		{
			name:     "local user",
			username: "alice",
			domain:   "example.com",
			isLocal:  true,
			want:     "@alice@example.com",
		},
		{
			name:     "remote user",
			username: "bob",
			domain:   "mastodon.social",
			isLocal:  false,
			want:     "@bob@mastodon.social",
		},
		{
			name:     "username with underscore",
			username: "user_name",
			domain:   "instance.org",
			isLocal:  false,
			want:     "@user_name@instance.org",
		},
		{
			name:     "username with numbers",
			username: "user123",
			domain:   "test.io",
			isLocal:  true,
			want:     "@user123@test.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MentionCandidate{
				Username: tt.username,
				Domain:   tt.domain,
				IsLocal:  tt.isLocal,
			}
			got := m.FullMention()
			if got != tt.want {
				t.Errorf("FullMention() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMentionCandidateDisplayMention(t *testing.T) {
	tests := []struct {
		name     string
		username string
		domain   string
		isLocal  bool
		want     string
	}{
		{
			name:     "local user shows without domain",
			username: "alice",
			domain:   "example.com",
			isLocal:  true,
			want:     "@alice",
		},
		{
			name:     "remote user shows with domain",
			username: "bob",
			domain:   "mastodon.social",
			isLocal:  false,
			want:     "@bob@mastodon.social",
		},
		{
			name:     "local user with underscore",
			username: "user_name",
			domain:   "instance.org",
			isLocal:  true,
			want:     "@user_name",
		},
		{
			name:     "remote user with numbers",
			username: "user123",
			domain:   "test.io",
			isLocal:  false,
			want:     "@user123@test.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MentionCandidate{
				Username: tt.username,
				Domain:   tt.domain,
				IsLocal:  tt.isLocal,
			}
			got := m.DisplayMention()
			if got != tt.want {
				t.Errorf("DisplayMention() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMentionCandidateDisplayVsFullMention(t *testing.T) {
	// For local users, DisplayMention should differ from FullMention
	local := MentionCandidate{
		Username: "alice",
		Domain:   "example.com",
		IsLocal:  true,
	}

	if local.DisplayMention() == local.FullMention() {
		t.Error("Local user's DisplayMention should differ from FullMention")
	}
	if local.DisplayMention() != "@alice" {
		t.Errorf("Local user's DisplayMention should be @username, got %q", local.DisplayMention())
	}

	// For remote users, DisplayMention should equal FullMention
	remote := MentionCandidate{
		Username: "bob",
		Domain:   "mastodon.social",
		IsLocal:  false,
	}

	if remote.DisplayMention() != remote.FullMention() {
		t.Error("Remote user's DisplayMention should equal FullMention")
	}
}
