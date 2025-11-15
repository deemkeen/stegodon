package admin

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
	userStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0)

	selectedStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0).
			Foreground(lipgloss.Color(common.COLOR_GREEN)).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0).
			Foreground(lipgloss.Color(common.COLOR_RED))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DARK_GREY)).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_BLUE))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_RED))
)

type Model struct {
	AdminId  uuid.UUID
	Users    []domain.Account
	Selected int
	Width    int
	Height   int
	Status   string
	Error    string
}

func InitialModel(adminId uuid.UUID, width, height int) Model {
	return Model{
		AdminId:  adminId,
		Users:    []domain.Account{},
		Selected: 0,
		Width:    width,
		Height:   height,
		Status:   "",
		Error:    "",
	}
}

func (m Model) Init() tea.Cmd {
	return loadUsers()
}

type usersLoadedMsg struct {
	users []domain.Account
}

type muteUserMsg struct {
	userId uuid.UUID
}

type kickUserMsg struct {
	userId uuid.UUID
}

func loadUsers() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, users := database.ReadAllAccountsAdmin()
		if err != nil {
			log.Printf("Admin panel: Failed to load users: %v", err)
			return usersLoadedMsg{users: []domain.Account{}}
		}
		if users == nil {
			log.Printf("Admin panel: Users is nil")
			return usersLoadedMsg{users: []domain.Account{}}
		}
		log.Printf("Admin panel: Loaded %d users", len(*users))
		return usersLoadedMsg{users: *users}
	}
}

func muteUser(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.MuteUser(userId)
		if err != nil {
			log.Printf("Failed to mute user: %v", err)
		}
		return muteUserMsg{userId: userId}
	}
}

func kickUser(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.DeleteAccount(userId)
		if err != nil {
			log.Printf("Failed to kick user: %v", err)
		}
		return kickUserMsg{userId: userId}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case usersLoadedMsg:
		log.Printf("Admin panel: Received usersLoadedMsg with %d users", len(msg.users))
		m.Users = msg.users
		if m.Selected >= len(m.Users) {
			m.Selected = max(0, len(m.Users)-1)
		}
		return m, nil

	case muteUserMsg:
		m.Status = "User muted and posts deleted"
		m.Error = ""
		return m, loadUsers()

	case kickUserMsg:
		m.Status = "User kicked successfully"
		m.Error = ""
		return m, loadUsers()

	case tea.KeyMsg:
		m.Status = ""
		m.Error = ""

		switch msg.String() {
		case "up":
			if m.Selected > 0 {
				m.Selected--
			}
		case "down":
			if len(m.Users) > 0 && m.Selected < len(m.Users)-1 {
				m.Selected++
			}
		case "m":
			// Mute selected user
			if len(m.Users) > 0 && m.Selected < len(m.Users) {
				selectedUser := m.Users[m.Selected]
				// Can't mute admin or yourself
				if selectedUser.IsAdmin {
					m.Error = "Cannot mute admin user"
					return m, nil
				}
				if selectedUser.Id == m.AdminId {
					m.Error = "Cannot mute yourself"
					return m, nil
				}
				if selectedUser.Muted {
					m.Error = "User is already muted"
					return m, nil
				}
				return m, muteUser(selectedUser.Id)
			}
		case "k":
			// Kick selected user
			if len(m.Users) > 0 && m.Selected < len(m.Users) {
				selectedUser := m.Users[m.Selected]
				// Can't kick admin or yourself
				if selectedUser.IsAdmin {
					m.Error = "Cannot kick admin user"
					return m, nil
				}
				if selectedUser.Id == m.AdminId {
					m.Error = "Cannot kick yourself"
					return m, nil
				}
				return m, kickUser(selectedUser.Id)
			}
		}
	}

	return m, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("admin panel (%d users)", len(m.Users))))
	s.WriteString("\n\n")

	if len(m.Users) == 0 {
		s.WriteString(emptyStyle.Render("No users found."))
	} else {
		for i, user := range m.Users {
			prefix := "  "
			style := userStyle
			suffix := ""

			if i == m.Selected {
				prefix = "> "
				style = selectedStyle
			}

			if user.Muted {
				style = mutedStyle
				suffix = " [MUTED]"
			}

			if user.IsAdmin {
				suffix += " [ADMIN]"
			}

			s.WriteString(style.Render(fmt.Sprintf("%s%s%s", prefix, user.Username, suffix)))
			s.WriteString("\n")
		}

		// Help text
		s.WriteString("\n")
		s.WriteString(common.HelpStyle.Render("m: mute  k: kick  ↑/↓: navigate"))
		s.WriteString("\n")
	}

	if m.Status != "" {
		s.WriteString("\n")
		s.WriteString(statusStyle.Render(m.Status))
	}

	if m.Error != "" {
		s.WriteString("\n")
		s.WriteString(errorStyle.Render("Error: " + m.Error))
	}

	return s.String()
}
