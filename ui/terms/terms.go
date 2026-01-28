package terms

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

var (
	Style = lipgloss.NewStyle().Height(25).Width(80).
		Align(lipgloss.Center, lipgloss.Center).
		BorderStyle(lipgloss.ThickBorder()).
		Margin(0, 3)
)

type Model struct {
	TermsContent string
	UserId       uuid.UUID
	Accepted    bool
	Error       string
}

func (m Model) Init() tea.Cmd {
	return loadTermsAndConditions()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case termsLoadedMsg:
		m.TermsContent = msg.content
		return m, nil

	case termsAcceptanceErrorMsg:
		m.Error = "Failed to save acceptance. Please try again."
		return m, nil

	case TermsAcceptedMsg:
		m.Accepted = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// User accepts terms and conditions
			return m, acceptTermsAndConditions(m.UserId)
		case "esc", "ctrl+c":
			// User cancels
			m.Accepted = false
			return m, tea.Quit
		}
	}

	return m, cmd
}

func (m Model) View() string {
	var s string

	s += fmt.Sprintf("Terms and Conditions\n\n")
	s += fmt.Sprintf("Please read the following terms and conditions carefully:\n\n")

	// Display terms content with word wrapping
	termsStyle := lipgloss.NewStyle().Width(70).Height(15).Border(lipgloss.RoundedBorder())
	s += termsStyle.Render(m.TermsContent)

	s += "\n\n"
	s += fmt.Sprintf("Press [ENTER] to accept or [ESC] to cancel")

	// Add error message if present
	if m.Error != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_ERROR)).Bold(true)
		s += "\n\n" + errorStyle.Render(m.Error)
	}

	return s
}

// ViewWithWidth renders the view with proper width accounting for border and margins
func (m Model) ViewWithWidth(termWidth, termHeight int) string {
	// Account for border (2 chars) and margins already defined in Style (6 chars total)
	// Total to subtract: 2 (border) + 6 (margins) = 8
	contentWidth := max(termWidth-common.TermsDialogBorderAndMargin,
		// Minimum width
		common.TermsDialogMinWidth)

	bordered := Style.Width(contentWidth).Render(m.View())
	return lipgloss.Place(termWidth, termHeight, lipgloss.Center, lipgloss.Center, bordered)
}

func InitialModel(userId uuid.UUID) Model {
	return Model{
		UserId:    userId,
		Accepted: false,
	}
}

// Message types
type termsLoadedMsg struct {
	content string
}

type termsAcceptanceErrorMsg struct {
	err error
}

// TermsAcceptedMsg is exported for the parent view to detect acceptance
type TermsAcceptedMsg struct{}

// loadTermsAndConditions loads the current terms and conditions from the database
func loadTermsAndConditions() tea.Cmd {
	return func() tea.Msg {
		err, terms := db.GetDB().GetCurrentTermsAndConditions()
		if err != nil {
			log.Printf("Failed to load terms and conditions: %v", err)
			return termsLoadedMsg{content: "Terms and conditions could not be loaded."}
		}
		return termsLoadedMsg{content: terms.Content}
	}
}

// acceptTermsAndConditions records that the user has accepted the terms and conditions
func acceptTermsAndConditions(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		err := db.GetDB().RecordUserTermsAcceptance(userId)
		if err != nil {
			log.Printf("Failed to record terms acceptance: %v", err)
			return termsAcceptanceErrorMsg{err: err}
		}
		return TermsAcceptedMsg{}
	}
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
