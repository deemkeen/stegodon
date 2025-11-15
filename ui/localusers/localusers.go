package localusers

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
	userStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0)

	selectedStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0).
			Foreground(lipgloss.Color(common.COLOR_GREEN)).
			Bold(true)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DARK_GREY)).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_BLUE))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_RED))
)

type Model struct {
	AccountId uuid.UUID
	Users     []domain.Account
	Following map[uuid.UUID]bool
	Selected  int
	Width     int
	Height    int
	Status    string
	Error     string
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId: accountId,
		Users:     []domain.Account{},
		Following: make(map[uuid.UUID]bool),
		Selected:  0,
		Width:     width,
		Height:    height,
		Status:    "",
		Error:     "",
	}
}

func (m Model) Init() tea.Cmd {
	return loadLocalUsers(m.AccountId)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case usersLoadedMsg:
		m.Users = msg.users
		m.Following = msg.following
		return m, nil

	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
			}
		case "down", "j":
			// Count non-current-user entries
			maxSelection := 0
			for _, user := range m.Users {
				if user.Id != m.AccountId {
					maxSelection++
				}
			}
			if m.Selected < maxSelection-1 {
				m.Selected++
			}
		case "enter", "f":
			if len(m.Users) > 0 {
				// Find the actual user at the selected display position
				// Skip the current user when counting
				displayIndex := 0
				var selectedUser *domain.Account
				for i := range m.Users {
					if m.Users[i].Id == m.AccountId {
						continue
					}
					if displayIndex == m.Selected {
						selectedUser = &m.Users[i]
						break
					}
					displayIndex++
				}

				if selectedUser == nil {
					return m, nil
				}

				// Don't allow following yourself (shouldn't happen but double-check)
				if selectedUser.Id == m.AccountId {
					m.Error = "You can't follow yourself!"
					m.Status = ""
					return m, clearStatusAfter(2 * time.Second)
				}

				// Toggle follow/unfollow
				isFollowing := m.Following[selectedUser.Id]

				go func() {
					database := db.GetDB()
					var err error
					if isFollowing {
						err = database.DeleteLocalFollow(m.AccountId, selectedUser.Id)
						if err != nil {
							log.Printf("Unfollow failed: %v", err)
						}
					} else {
						err = database.CreateLocalFollow(m.AccountId, selectedUser.Id)
						if err != nil {
							log.Printf("Follow failed: %v", err)
						}
					}
				}()

				// Update local state immediately
				if isFollowing {
					delete(m.Following, selectedUser.Id)
					m.Status = fmt.Sprintf("Unfollowed @%s", selectedUser.Username)
				} else {
					m.Following[selectedUser.Id] = true
					m.Status = fmt.Sprintf("Following @%s", selectedUser.Username)
				}
				m.Error = ""
				return m, clearStatusAfter(2 * time.Second)
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("local users (%d)", len(m.Users))))
	s.WriteString("\n\n")

	if len(m.Users) == 0 {
		s.WriteString(emptyStyle.Render("No other local users yet."))
	} else {
		// First, show the current user at the top
		for _, user := range m.Users {
			if user.Id == m.AccountId {
				s.WriteString("  " + userStyle.Render(fmt.Sprintf("@%s (you)", user.Username)))
				s.WriteString("\n")
				break
			}
		}

		// Then show all other users
		displayIndex := 0
		for _, user := range m.Users {
			// Skip the current user as we already displayed them
			if user.Id == m.AccountId {
				continue
			}

			followStatus := ""
			if m.Following[user.Id] {
				followStatus = " [following]"
			}

			userText := fmt.Sprintf("@%s%s", user.Username, followStatus)

			if displayIndex == m.Selected {
				s.WriteString("→ " + selectedStyle.Render(userText))
			} else {
				s.WriteString("  " + userStyle.Render(userText))
			}
			s.WriteString("\n")
			displayIndex++
		}
	}

	s.WriteString("\n")

	if m.Status != "" {
		s.WriteString(statusStyle.Render(m.Status))
		s.WriteString("\n\n")
	}

	if m.Error != "" {
		s.WriteString(errorStyle.Render(m.Error))
		s.WriteString("\n\n")
	}

	return s.String()
}

// usersLoadedMsg is sent when users are loaded
type usersLoadedMsg struct {
	users     []domain.Account
	following map[uuid.UUID]bool
}

// clearStatusMsg is sent after a delay to clear status/error messages
type clearStatusMsg struct{}

// clearStatusAfter returns a command that sends clearStatusMsg after a duration
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// loadLocalUsers loads all local users and checks which ones are being followed
func loadLocalUsers(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Load all local users
		err, users := database.ReadAllAccounts()
		if err != nil {
			log.Printf("Failed to load local users: %v", err)
			return usersLoadedMsg{users: []domain.Account{}, following: make(map[uuid.UUID]bool)}
		}

		if users == nil {
			return usersLoadedMsg{users: []domain.Account{}, following: make(map[uuid.UUID]bool)}
		}

		// Load local follows to see who we're following
		err, follows := database.ReadLocalFollowsByAccountId(accountId)
		following := make(map[uuid.UUID]bool)
		if err == nil && follows != nil {
			for _, follow := range *follows {
				following[follow.TargetAccountId] = true
			}
		}

		return usersLoadedMsg{users: *users, following: following}
	}
}
