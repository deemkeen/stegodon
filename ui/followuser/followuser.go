package followuser

import (
	"fmt"
	"strings"
	"time"

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
	ti.Placeholder = "user@domain or @user@domain"
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
	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		m.TextInput.SetValue("")
		return m, nil

	case followResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed: %v", msg.err)
			m.Status = ""
		} else {
			m.Status = fmt.Sprintf("✓ Sent follow request to %s", msg.username)
		}
		return m, clearStatusAfter(2 * time.Second)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Parse user@domain format
			input := strings.TrimSpace(m.TextInput.Value())
			if input == "" {
				m.Error = "Please enter a user@domain"
				return m, clearStatusAfter(2 * time.Second)
			}

			// Remove leading @ if present
			input = strings.TrimPrefix(input, "@")

			parts := strings.Split(input, "@")
			if len(parts) != 2 {
				m.Error = "Invalid format. Use: user@domain.com or @user@domain.com"
				return m, clearStatusAfter(2 * time.Second)
			}

			username := parts[0]
			domain := parts[1]

			if username == "" || domain == "" {
				m.Error = "Invalid format. Use: user@domain.com or @user@domain.com"
				return m, clearStatusAfter(2 * time.Second)
			}

			// Attempt to follow
			m.Status = fmt.Sprintf("Following %s@%s...", username, domain)
			m.Error = ""

			return m, followRemoteUserCmd(m.AccountId, username, domain, fmt.Sprintf("%s@%s", username, domain))
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
	s.WriteString("Enter ActivityPub address:\n")
	s.WriteString("(e.g., user@mastodon.social or @user@mastodon.social)\n\n")
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

	return s.String()
}

// clearStatusMsg is sent after a delay to clear status/error messages
type clearStatusMsg struct{}

// followResultMsg is sent when the follow operation completes
type followResultMsg struct {
	username string
	err      error
}

// clearStatusAfter returns a command that sends clearStatusMsg after a duration
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// followRemoteUserCmd returns a command that follows a remote user and sends the result
func followRemoteUserCmd(accountId uuid.UUID, username, domain, fullUsername string) tea.Cmd {
	return func() tea.Msg {
		err := followRemoteUser(accountId, username, domain)
		return followResultMsg{
			username: fullUsername,
			err:      err,
		}
	}
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

	// Check if already following this user
	err, following := database.ReadFollowingByAccountId(accountId)
	if err == nil && following != nil {
		for _, follow := range *following {
			err, remoteAcc := database.ReadRemoteAccountById(follow.TargetAccountId)
			if err == nil && remoteAcc != nil {
				if remoteAcc.ActorURI == actorURI {
					return fmt.Errorf("already following %s@%s", username, domain)
				}
			}
		}
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
