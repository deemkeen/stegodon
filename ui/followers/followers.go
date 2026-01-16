package followers

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"log"
)

type Model struct {
	AccountId uuid.UUID
	Followers []domain.Follow
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
		Followers: []domain.Follow{},
		Selected:  0,
		Offset:    0,
		Width:     width,
		Height:    height,
		Status:    "",
		Error:     "",
	}
}

func (m Model) Init() tea.Cmd {
	return loadFollowers(m.AccountId)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case followersLoadedMsg:
		m.Followers = msg.followers
		m.Offset = 0
		m.Selected = 0
		return m, nil

	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case followResultMsg:
		if msg.err != nil {
			// Check error type (case-insensitive)
			errMsg := strings.ToLower(msg.err.Error())
			if strings.Contains(errMsg, "already following") {
				m.Status = fmt.Sprintf("ℹ Already following %s", msg.username)
				m.Error = ""
			} else if strings.Contains(errMsg, "follow pending") {
				m.Status = fmt.Sprintf("ℹ Follow request pending for %s", msg.username)
				m.Error = ""
			} else if strings.Contains(errMsg, "self-follow not allowed") {
				m.Status = "ℹ Self-follow not allowed"
				m.Error = ""
			} else {
				m.Error = fmt.Sprintf("Failed to follow %s: %v", msg.username, msg.err)
				m.Status = ""
			}
		} else {
			m.Status = fmt.Sprintf("✓ Following %s", msg.username)
			m.Error = ""
		}
		return m, clearStatusAfter(3 * time.Second)

	case tea.KeyMsg:
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
			if m.Selected < len(m.Followers)-1 {
				m.Selected++
				// Scroll down if needed
				if m.Selected >= m.Offset+common.DefaultItemsPerPage {
					m.Offset = m.Selected - common.DefaultItemsPerPage + 1
				}
			}
		case "f":
			// Follow user back
			if len(m.Followers) > 0 && m.Selected < len(m.Followers) {
				follower := m.Followers[m.Selected]
				if follower.IsLocal {
					m.Status = "Following local user..."
					return m, followLocalUserCmd(m.AccountId, follower.AccountId)
				} else {
					m.Status = "Requesting to follow remote user..."
					return m, followRemoteUserByIdCmd(m.AccountId, follower.AccountId)
				}
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
		s.WriteString(common.ListEmptyStyle.Render("No followers yet. Share your profile to get followers!"))
		return s.String()
	}

	start := m.Offset
	end := min(start+common.DefaultItemsPerPage, len(m.Followers))

	for i := start; i < end; i++ {
		follow := m.Followers[i]
		database := db.GetDB()

		var username, badge string

		if follow.IsLocal {
			// Local follower - look up in accounts table
			err, localAcc := database.ReadAccById(follow.AccountId)
			if err != nil {
				log.Printf("Failed to read local account: %v", err)
				continue
			}
			username = "@" + localAcc.Username
			badge = " [local]"
		} else {
			// Remote follower - look up in remote_accounts table
			err, remoteAcc := database.ReadRemoteAccountById(follow.AccountId)
			if err != nil {
				log.Printf("Failed to read remote account: %v", err)
				continue
			}
			username = fmt.Sprintf("@%s@%s", remoteAcc.Username, remoteAcc.Domain)
			badge = ""
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
	if len(m.Followers) > common.DefaultItemsPerPage {
		s.WriteString("\n")
		paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.Followers))
		s.WriteString(common.ListBadgeStyle.Render(paginationText))
	}

	if m.Status != "" {
		s.WriteString("\n")
		s.WriteString(common.ListStatusStyle.Render(m.Status))
	}

	if m.Error != "" {
		s.WriteString("\n")
		s.WriteString(common.ListErrorStyle.Render(m.Error))
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

		// Clean up any orphaned follows before loading
		if err := database.CleanupOrphanedFollows(); err != nil {
			log.Printf("Warning: Failed to cleanup orphaned follows: %v", err)
		}

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

// clearStatusMsg is sent after a delay to clear status/error messages
type clearStatusMsg struct{}

// clearStatusAfter returns a command that sends clearStatusMsg after a duration
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// followResultMsg is sent when the follow operation completes
type followResultMsg struct {
	username string
	err      error
}

// followLocalUserCmd returns a command that follows a local user
func followLocalUserCmd(followerId, targetId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		
		// Check if already following
		isFollowing, err := database.IsFollowingLocal(followerId, targetId)
		if err != nil {
			return followResultMsg{username: "user", err: err}
		}
		
		if isFollowing {
			return followResultMsg{username: "user", err: fmt.Errorf("already following")}
		}
		
		// Create follow
		err = database.CreateLocalFollow(followerId, targetId)
		if err != nil {
			return followResultMsg{username: "user", err: err}
		}
		
		// Get target username for success message
		err, targetUser := database.ReadAccById(targetId)
		username := "user"
		if err == nil && targetUser != nil {
			username = "@" + targetUser.Username
			
			// Create notification for the followed user
			err, follower := database.ReadAccById(followerId)
			if err == nil && follower != nil {
				notification := &domain.Notification{
					Id:               uuid.New(),
					AccountId:        targetId,
					NotificationType: domain.NotificationFollow,
					ActorId:          followerId,
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
		
		return followResultMsg{username: username, err: nil}
	}
}

// followRemoteUserByIdCmd looks up a remote account and follows it
func followRemoteUserByIdCmd(localAccountId, remoteAccountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		
		// Get remote account details
		err, remoteAcc := database.ReadRemoteAccountById(remoteAccountId)
		if err != nil {
			return followResultMsg{username: "remote user", err: err}
		}
		
		fullUsername := fmt.Sprintf("%s@%s", remoteAcc.Username, remoteAcc.Domain)
		
		// Get local account
		err, localAccount := database.ReadAccById(localAccountId)
		if err != nil {
			return followResultMsg{username: fullUsername, err: err}
		}

		// Get config
		conf, err := util.ReadConf()
		if err != nil {
			return followResultMsg{username: fullUsername, err: err}
		}

		// Send Follow activity
		if err := activitypub.SendFollow(localAccount, remoteAcc.ActorURI, conf); err != nil {
			return followResultMsg{username: fullUsername, err: err}
		}

		return followResultMsg{username: fullUsername, err: nil}
	}
}
