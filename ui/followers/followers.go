package followers

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
	"log"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			MarginBottom(1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

type Model struct {
	AccountId uuid.UUID
	Followers []domain.Follow
	Width     int
	Height    int
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId: accountId,
		Followers: []domain.Follow{},
		Width:     width,
		Height:    height,
	}
}

func (m Model) Init() tea.Cmd {
	return loadFollowers(m.AccountId)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case followersLoadedMsg:
		m.Followers = msg.followers
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render(fmt.Sprintf("Followers (%d)", len(m.Followers))))
	s.WriteString("\n\n")

	if len(m.Followers) == 0 {
		s.WriteString(emptyStyle.Render("No followers yet. Share your account to get followers!"))
	} else {
		for i, follow := range m.Followers {
			if i >= 10 { // Limit display to 10 for now
				s.WriteString(itemStyle.Render(fmt.Sprintf("... and %d more", len(m.Followers)-10)))
				break
			}

			// Get remote account details
			database := db.GetDB()
			err, remoteAcc := database.ReadRemoteAccountById(follow.TargetAccountId)
			if err != nil {
				log.Printf("Failed to read remote account: %v", err)
				continue
			}

			displayName := remoteAcc.DisplayName
			if displayName == "" {
				displayName = remoteAcc.Username
			}

			s.WriteString(itemStyle.Render(fmt.Sprintf(
				"• %s (@%s@%s)",
				displayName,
				remoteAcc.Username,
				remoteAcc.Domain,
			)))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Faint(true).Render("tab: switch view • ctrl-c: exit"))

	return s.String()
}

// followersLoadedMsg is sent when followers are loaded
type followersLoadedMsg struct {
	followers []domain.Follow
}

// loadFollowers loads the followers for the given account
func loadFollowers(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, followers := database.ReadFollowersByAccountId(accountId)
		if err != nil {
			log.Printf("Failed to load followers: %v", err)
			return followersLoadedMsg{followers: []domain.Follow{}}
		}

		if followers == nil {
			return followersLoadedMsg{followers: []domain.Follow{}}
		}

		return followersLoadedMsg{followers: *followers}
	}
}
