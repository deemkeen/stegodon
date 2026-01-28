package terms

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func TestInitialModel(t *testing.T) {
	userId := uuid.New()
	m := InitialModel(userId)

	if m.UserId != userId {
		t.Errorf("Expected UserId %v, got %v", userId, m.UserId)
	}
	if m.Accepted {
		t.Error("Expected Accepted to be false initially")
	}
	if m.TermsContent != "" {
		t.Error("Expected TermsContent to be empty initially")
	}
	if m.Error != "" {
		t.Error("Expected Error to be empty initially")
	}
}

func TestUpdate_TermsLoadedMsg(t *testing.T) {
	m := InitialModel(uuid.New())
	content := "Test terms content"

	newM, _ := m.Update(termsLoadedMsg{content: content})

	if newM.TermsContent != content {
		t.Errorf("Expected TermsContent %q, got %q", content, newM.TermsContent)
	}
}

func TestUpdate_TermsAcceptedMsg(t *testing.T) {
	m := InitialModel(uuid.New())

	newM, _ := m.Update(TermsAcceptedMsg{})

	if !newM.Accepted {
		t.Error("Expected Accepted to be true after TermsAcceptedMsg")
	}
}

func TestUpdate_TermsAcceptanceErrorMsg(t *testing.T) {
	m := InitialModel(uuid.New())

	newM, _ := m.Update(termsAcceptanceErrorMsg{err: nil})

	if newM.Error == "" {
		t.Error("Expected Error to be set after termsAcceptanceErrorMsg")
	}
}

func TestUpdate_EscapeKey(t *testing.T) {
	m := InitialModel(uuid.New())
	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}

	newM, cmd := m.Update(keyMsg)

	if newM.Accepted {
		t.Error("Expected Accepted to remain false after escape")
	}
	// Should return tea.Quit command
	if cmd == nil {
		t.Error("Expected quit command after escape")
	}
}

func TestView_ContainsTitle(t *testing.T) {
	m := InitialModel(uuid.New())
	m.TermsContent = "Test terms"

	view := m.View()

	if !strings.Contains(view, "Terms and Conditions") {
		t.Error("Expected view to contain 'Terms and Conditions'")
	}
}

func TestView_ContainsContent(t *testing.T) {
	m := InitialModel(uuid.New())
	m.TermsContent = "Test terms content here"

	view := m.View()

	if !strings.Contains(view, "Test terms content here") {
		t.Error("Expected view to contain the terms content")
	}
}

func TestView_ContainsInstructions(t *testing.T) {
	m := InitialModel(uuid.New())

	view := m.View()

	if !strings.Contains(view, "ENTER") {
		t.Error("Expected view to contain ENTER instruction")
	}
	if !strings.Contains(view, "ESC") {
		t.Error("Expected view to contain ESC instruction")
	}
}

func TestView_ShowsError(t *testing.T) {
	m := InitialModel(uuid.New())
	m.Error = "Test error message"

	view := m.View()

	if !strings.Contains(view, "Test error message") {
		t.Error("Expected view to contain error message")
	}
}

func TestViewWithWidth(t *testing.T) {
	m := InitialModel(uuid.New())
	m.TermsContent = "Test content"

	view := m.ViewWithWidth(120, 40)

	// Just verify it doesn't panic and produces output
	if len(view) == 0 {
		t.Error("Expected ViewWithWidth to produce output")
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{4, 4, 4},
		{-1, -5, -1},
	}

	for _, tt := range tests {
		result := max(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("max(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}
