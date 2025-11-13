package following

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
	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0)

	selectedStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(0).
			Foreground(lipgloss.Color("86")).
			Bold(true)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)

type Model struct {
	AccountId uuid.UUID
	Following []domain.Follow
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
		Following: []domain.Follow{},
		Selected:  0,
		Offset:    0,
		Width:     width,
		Height:    height,
		Status:    "",
		Error:     "",
	}
}

func (m Model) Init() tea.Cmd {
	return loadFollowing(m.AccountId)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case followingLoadedMsg:
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
			if m.Selected < len(m.Following)-1 {
				m.Selected++
			}
		case "u", "enter":
			// Unfollow the selected account
			if len(m.Following) > 0 && m.Selected < len(m.Following) {
				selectedFollow := m.Following[m.Selected]
				database := db.GetDB()

				var displayName string

				if selectedFollow.IsLocal {
					// Local follow - get local account details
					err, localAcc := database.ReadAccById(selectedFollow.TargetAccountId)
					if err == nil && localAcc != nil {
						displayName = localAcc.Username
					} else {
						displayName = "user"
					}
				} else {
					// Remote follow - get remote account details
					err, remoteAcc := database.ReadRemoteAccountById(selectedFollow.TargetAccountId)
					if err == nil && remoteAcc != nil {
						displayName = fmt.Sprintf("@%s@%s", remoteAcc.Username, remoteAcc.Domain)
					} else {
						displayName = "user"
					}
				}

				// Delete the follow
				go func() {
					var err error
					if selectedFollow.IsLocal {
						// For local follows, delete by account IDs
						err = database.DeleteFollowByAccountIds(m.AccountId, selectedFollow.TargetAccountId)
					} else {
						// For remote follows, delete by URI
						err = database.DeleteFollowByURI(selectedFollow.URI)
					}
					if err != nil {
						log.Printf("Unfollow failed: %v", err)
					}
				}()

				// Remove from local list
				m.Following = append(m.Following[:m.Selected], m.Following[m.Selected+1:]...)
				if m.Selected >= len(m.Following) && m.Selected > 0 {
					m.Selected--
				}

				m.Status = fmt.Sprintf("Unfollowed %s", displayName)
				m.Error = ""
				return m, clearStatusAfter(2 * time.Second)
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("following (%d)", len(m.Following))))
	s.WriteString("\n\n")

	if len(m.Following) == 0 {
		s.WriteString(emptyStyle.Render("You're not following anyone yet.\nUse the follow user view to start following!"))
	} else {
		displayCount := min(len(m.Following), 10)
		for i := 0; i < displayCount; i++ {
			follow := m.Following[i]
			database := db.GetDB()

			var userText string

			if follow.IsLocal {
				// Local follow - look up in accounts table
				err, localAcc := database.ReadAccById(follow.TargetAccountId)
				if err != nil {
					log.Printf("Failed to read local account: %v", err)
					continue
				}

				userText = fmt.Sprintf("• %s (local)", localAcc.Username)
			} else {
				// Remote follow - look up in remote_accounts table
				err, remoteAcc := database.ReadRemoteAccountById(follow.TargetAccountId)
				if err != nil {
					log.Printf("Failed to read remote account: %v", err)
					continue
				}

				displayName := remoteAcc.DisplayName
				if displayName == "" {
					displayName = remoteAcc.Username
				}

				userText = fmt.Sprintf("• %s (@%s@%s)",
					displayName,
					remoteAcc.Username,
					remoteAcc.Domain,
				)
			}

			if i == m.Selected {
				s.WriteString("→ " + selectedStyle.Render(userText))
			} else {
				s.WriteString("  " + itemStyle.Render(userText))
			}
			s.WriteString("\n")
		}

		if len(m.Following) > 10 {
			s.WriteString(itemStyle.Render(fmt.Sprintf("... and %d more", len(m.Following)-10)))
			s.WriteString("\n")
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

// followingLoadedMsg is sent when following list is loaded
type followingLoadedMsg struct {
	following []domain.Follow
}

// clearStatusMsg is sent after a delay to clear status/error messages
type clearStatusMsg struct{}

// clearStatusAfter returns a command that sends clearStatusMsg after a duration
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// loadFollowing loads the accounts that the user is following
func loadFollowing(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, following := database.ReadFollowingByAccountId(accountId)
		if err != nil {
			log.Printf("Failed to load following: %v", err)
			return followingLoadedMsg{following: []domain.Follow{}}
		}

		if following == nil {
			return followingLoadedMsg{following: []domain.Follow{}}
		}

		return followingLoadedMsg{following: *following}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
