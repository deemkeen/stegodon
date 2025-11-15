package deleteaccount

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
	"log"
)

var (
	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_RED)).
			Bold(true)

	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_BLUE))

	instructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_DARK_GREY))
)

type Model struct {
	Account        *domain.Account
	ConfirmStep    int // 0 = initial, 1 = first confirmation, 2 = final confirmation
	Status         string
	Error          string
	DeletionStatus string
	ShowByeBye     bool
}

func InitialModel(account *domain.Account) Model {
	return Model{
		Account:     account,
		ConfirmStep: 0,
		Status:      "",
		Error:       "",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case showByeByeMsg:
		m.ShowByeBye = true
		// After showing "Bye bye!", wait 2 more seconds then quit
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tea.Quit()
		})

	case deleteAccountResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to delete account: %v", msg.err)
			m.ConfirmStep = 0
		} else {
			m.DeletionStatus = "completed"
			// Wait 2 seconds then show "Bye bye!"
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return showByeByeMsg{}
			})
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			if m.ConfirmStep == 0 {
				// First confirmation
				m.ConfirmStep = 1
				m.Status = ""
				return m, nil
			} else if m.ConfirmStep == 1 {
				// Final confirmation - delete account
				m.Status = "Deleting account..."
				return m, deleteAccountCmd(m.Account.Id)
			}
		case "n", "N", "esc":
			// Cancel at any step
			m.ConfirmStep = 0
			m.Status = "Deletion cancelled"
			m.Error = ""
			return m, clearStatusAfter(2 * time.Second)
		}
	}

	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render("delete account"))
	s.WriteString("\n\n")

	if m.ShowByeBye {
		// Show goodbye message
		byeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Align(lipgloss.Center)
		s.WriteString("\n\n")
		s.WriteString(byeStyle.Render("Bye bye!"))
		s.WriteString("\n\n")
		return s.String()
	}

	if m.DeletionStatus == "completed" {
		s.WriteString(confirmStyle.Render("✓ Account deleted successfully"))
		s.WriteString("\n\n")
		s.WriteString(instructionStyle.Render("Logging out..."))
		return s.String()
	}

	if m.ConfirmStep == 0 {
		// Initial warning
		s.WriteString(warningStyle.Render("⚠ WARNING: This will permanently delete your account!"))
		s.WriteString("\n\n")
		s.WriteString("The following data will be deleted:\n")
		s.WriteString("  • Your account (@" + m.Account.Username + ")\n")
		s.WriteString("  • All your posts and notes\n")
		s.WriteString("  • All follow relationships\n")
		s.WriteString("  • All your activities\n")
		s.WriteString("\n")
		s.WriteString(warningStyle.Render("This action CANNOT be undone!"))
		s.WriteString("\n\n")
		s.WriteString("Are you sure you want to delete your account?\n\n")
		s.WriteString(instructionStyle.Render("Press 'y' to continue or 'n'/'esc' to cancel"))
	} else if m.ConfirmStep == 1 {
		// Final confirmation
		s.WriteString(warningStyle.Render("⚠ FINAL WARNING!"))
		s.WriteString("\n\n")
		s.WriteString("You are about to permanently delete account: ")
		s.WriteString(warningStyle.Render("@" + m.Account.Username))
		s.WriteString("\n\n")
		s.WriteString("This is your last chance to cancel.\n")
		s.WriteString("After this, your account and all data will be gone forever.\n\n")
		s.WriteString(instructionStyle.Render("Press 'y' to DELETE PERMANENTLY or 'n'/'esc' to cancel"))
	}

	s.WriteString("\n\n")

	if m.Status != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.Status))
		s.WriteString("\n")
	}

	if m.Error != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.Error))
		s.WriteString("\n")
	}

	return s.String()
}

// clearStatusMsg is sent after a delay to clear status/error messages
type clearStatusMsg struct{}

// showByeByeMsg is sent after deletion to show goodbye message
type showByeByeMsg struct{}

// deleteAccountResultMsg is sent when the delete operation completes
type deleteAccountResultMsg struct {
	err error
}

// clearStatusAfter returns a command that sends clearStatusMsg after a duration
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// deleteAccountCmd returns a command that deletes an account and sends the result
func deleteAccountCmd(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		err := deleteAccount(accountId)
		return deleteAccountResultMsg{
			err: err,
		}
	}
}

// deleteAccount deletes the account and all associated data
func deleteAccount(accountId uuid.UUID) error {
	database := db.GetDB()
	err := database.DeleteAccount(accountId)
	if err != nil {
		log.Printf("Failed to delete account %s: %v", accountId, err)
		return err
	}

	log.Printf("Successfully deleted account %s", accountId)
	return nil
}
