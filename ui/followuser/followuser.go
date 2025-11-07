package followuser

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/deemkeen/stegodon/web"
	"github.com/google/uuid"
	"log"
)

var (
	Style = lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63"))
)

type Model struct {
	TextInput textinput.Model
	AccountId uuid.UUID
	Status    string
	Error     string
}

func InitialModel(accountId uuid.UUID) Model {
	ti := textinput.New()
	ti.Placeholder = "user@mastodon.social"
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	return Model{
		TextInput: ti,
		AccountId: accountId,
		Status:    "",
		Error:     "",
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Parse user@domain format
			input := strings.TrimSpace(m.TextInput.Value())
			if input == "" {
				m.Error = "Please enter a user@domain"
				return m, nil
			}

			parts := strings.Split(input, "@")
			if len(parts) != 2 {
				m.Error = "Invalid format. Use: user@domain.com"
				return m, nil
			}

			username := parts[0]
			domain := parts[1]

			// Attempt to follow
			m.Status = fmt.Sprintf("Following %s...", input)
			m.Error = ""

			go func() {
				if err := followRemoteUser(m.AccountId, username, domain); err != nil {
					log.Printf("Follow failed: %v", err)
					m.Error = fmt.Sprintf("Failed: %v", err)
					m.Status = ""
				} else {
					m.Status = fmt.Sprintf("✓ Sent follow request to %s", input)
					m.TextInput.SetValue("")
				}
			}()

			return m, nil
		case "esc":
			m.TextInput.SetValue("")
			m.Status = ""
			m.Error = ""
			return m, nil
		}
	}

	m.TextInput, cmd = m.TextInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render("follow remote user"))
	s.WriteString("\n\n")
	s.WriteString("Enter ActivityPub address (e.g., user@mastodon.social):\n\n")
	s.WriteString(m.TextInput.View())
	s.WriteString("\n\n")

	if m.Status != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.Status))
		s.WriteString("\n")
	}

	if m.Error != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.Error))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(common.HelpStyle.Render("enter: follow • esc: clear • tab: switch view • shift+tab: prev view"))

	return s.String()
}

// followRemoteUser resolves and follows a remote ActivityPub user
func followRemoteUser(accountId uuid.UUID, username, domain string) error {
	// Get local account
	database := db.GetDB()
	err, localAccount := database.ReadAccById(accountId)
	if err != nil {
		return fmt.Errorf("failed to get local account: %w", err)
	}

	// Resolve WebFinger to get actor URI
	actorURI, err := web.ResolveWebFinger(username, domain)
	if err != nil {
		return fmt.Errorf("webfinger resolution failed: %w", err)
	}

	// Get config (TODO: pass from main)
	conf, err := util.ReadConf()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Send Follow activity
	if err := activitypub.SendFollow(localAccount, actorURI, conf); err != nil {
		return fmt.Errorf("failed to send follow: %w", err)
	}

	log.Printf("Successfully sent follow request from %s to %s@%s (%s)",
		localAccount.Username, username, domain, actorURI)

	return nil
}
