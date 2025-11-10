package followers

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
	"log"
)

var (
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
	Offset    int // Pagination offset
	Width     int
	Height    int
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId: accountId,
		Followers: []domain.Follow{},
		Offset:    0,
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
		m.Offset = 0 // Reset offset on reload
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "left":
			if m.Offset > 0 {
				m.Offset--
			}
		case "down", "j", "right":
			// Allow scrolling if we have any followers at all
			if len(m.Followers) > 0 && m.Offset < len(m.Followers)-1 {
				m.Offset++
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("followers (%d)", len(m.Followers))))
	s.WriteString("\n\n")

	if len(m.Followers) == 0 {
		s.WriteString(emptyStyle.Render("No followers yet. Share your account to get followers!"))
	} else {
		itemsPerPage := 10
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Followers) {
			end = len(m.Followers)
		}

		for i := start; i < end; i++ {
			follow := m.Followers[i]

			// Get remote account details
			database := db.GetDB()
			err, remoteAcc := database.ReadRemoteAccountById(follow.AccountId)
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
