package localusers

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
	"log"
)

type Model struct {
	AccountId uuid.UUID
	Users     []domain.Account
	Following map[uuid.UUID]bool
	Selected  int
	Offset    int // Pagination offset
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
		Offset:    0,
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
		m.Selected = 0
		m.Offset = 0
		return m, nil

	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case tea.KeyPressMsg:
		// Get the list of other users (excluding self)
		otherUsers := m.getOtherUsers()

		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				// Scroll up if needed
				if m.Selected < m.Offset {
					m.Offset = m.Selected
				}
			}
		case "down", "j":
			if m.Selected < len(otherUsers)-1 {
				m.Selected++
				// Scroll down if needed
				if m.Selected >= m.Offset+common.DefaultItemsPerPage {
					m.Offset = m.Selected - common.DefaultItemsPerPage + 1
				}
			}
		case "enter":
			if len(otherUsers) > 0 && m.Selected < len(otherUsers) {
				selectedUser := otherUsers[m.Selected]
				return m, func() tea.Msg {
					return common.ViewProfileMsg{
						Username:  selectedUser.Username,
						AccountId: selectedUser.Id,
					}
				}
			}
		case "f":
			if len(otherUsers) > 0 && m.Selected < len(otherUsers) {
				selectedUser := otherUsers[m.Selected]

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
						} else {
							// Create notification for the followed user
							err, follower := database.ReadAccById(m.AccountId)
							if err == nil && follower != nil {
								notification := &domain.Notification{
									Id:               uuid.New(),
									AccountId:        selectedUser.Id,
									NotificationType: domain.NotificationFollow,
									ActorId:          follower.Id,
									ActorUsername:    follower.Username,
									ActorDomain:      "", // Empty for local users
									Read:             false,
									CreatedAt:        time.Now(),
								}
								if err := database.CreateNotification(notification); err != nil {
									log.Printf("Failed to create follow notification: %v", err)
								}
							}
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

// getOtherUsers returns all users except the current user
func (m Model) getOtherUsers() []domain.Account {
	var others []domain.Account
	for _, user := range m.Users {
		if user.Id != m.AccountId {
			others = append(others, user)
		}
	}
	return others
}

func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("local users (%d)", len(m.Users))))
	s.WriteString("\n\n")

	if len(m.Users) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No local users found."))
		return tea.NewView(s.String())
	}

	// Show current user first (not selectable)
	for _, user := range m.Users {
		if user.Id == m.AccountId {
			text := "@" + user.Username + common.ListBadgeStyle.Render(" [you]")
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
			s.WriteString("\n")
			break
		}
	}

	// Get other users and apply pagination
	otherUsers := m.getOtherUsers()

	if len(otherUsers) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No other local users yet."))
		return tea.NewView(s.String())
	}

	start := m.Offset
	end := min(start+common.DefaultItemsPerPage, len(otherUsers))

	for i := start; i < end; i++ {
		user := otherUsers[i]

		username := "@" + user.Username
		badge := ""
		if m.Following[user.Id] {
			badge = " [following]"
		}

		if i == m.Selected {
			// Selected item with arrow prefix
			text := common.ListItemSelectedStyle.Render(username + badge)
			s.WriteString(common.ListSelectedPrefix + text)
		} else {
			// Normal item
			text := username + common.ListBadgeStyle.Render(badge)
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
		}
		s.WriteString("\n")
	}

	// Show pagination info if there are more items
	if len(otherUsers) > common.DefaultItemsPerPage {
		s.WriteString("\n")
		paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(otherUsers))
		s.WriteString(common.ListBadgeStyle.Render(paginationText))
	}

	s.WriteString("\n")

	if m.Status != "" {
		s.WriteString(common.ListStatusStyle.Render(m.Status))
		s.WriteString("\n")
	}

	if m.Error != "" {
		s.WriteString(common.ListErrorStyle.Render(m.Error))
		s.WriteString("\n")
	}

	return tea.NewView(s.String())
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
