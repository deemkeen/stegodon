package following

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
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
		m.Selected = 0
		m.Offset = 0
		return m, nil

	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case tea.KeyPressMsg:
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
			if m.Selected < len(m.Following)-1 {
				m.Selected++
				// Scroll down if needed
				if m.Selected >= m.Offset+common.DefaultItemsPerPage {
					m.Offset = m.Selected - common.DefaultItemsPerPage + 1
				}
			}
		case "f":
			// Unfollow the selected account
			if len(m.Following) > 0 && m.Selected < len(m.Following) {
				selectedFollow := m.Following[m.Selected]
				database := db.GetDB()

				var displayName string

				if selectedFollow.IsLocal {
					// Local follow - get local account details
					err, localAcc := database.ReadAccById(selectedFollow.TargetAccountId)
					if err == nil && localAcc != nil {
						displayName = "@" + localAcc.Username
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

				// Delete the follow and send Undo activity for remote follows
				go func() {
					var err error
					if selectedFollow.IsLocal {
						// For local follows, just delete by account IDs
						err = database.DeleteFollowByAccountIds(m.AccountId, selectedFollow.TargetAccountId)
					} else {
						// For remote follows, send Undo activity first, then delete
						// Get local account
						localAccErr, localAccount := database.ReadAccById(m.AccountId)
						if localAccErr != nil {
							log.Printf("Unfollow failed: failed to get local account: %v", localAccErr)
							return
						}

						// Get remote account
						remoteAccErr, remoteAccount := database.ReadRemoteAccountById(selectedFollow.TargetAccountId)
						if remoteAccErr != nil {
							log.Printf("Unfollow failed: failed to get remote account: %v", remoteAccErr)
							return
						}

						// Get config for SendUndo
						conf, confErr := util.ReadConf()
						if confErr != nil {
							log.Printf("Unfollow failed: failed to read config: %v", confErr)
							return
						}

						// Send Undo activity to remote server
						if conf.Conf.WithAp {
							if undoErr := activitypub.SendUndo(localAccount, &selectedFollow, remoteAccount, conf); undoErr != nil {
								log.Printf("Warning: Failed to send Undo activity: %v", undoErr)
								// Continue with local delete even if remote notification fails
							}
						}

						// Delete from local database
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
		case "enter":
			// View profile of selected user
			if len(m.Following) > 0 && m.Selected < len(m.Following) {
				selectedFollow := m.Following[m.Selected]
				database := db.GetDB()

				if selectedFollow.IsLocal {
					err, localAcc := database.ReadAccById(selectedFollow.TargetAccountId)
					if err == nil && localAcc != nil {
						return m, func() tea.Msg {
							return common.ViewProfileMsg{
								Username:  localAcc.Username,
								AccountId: localAcc.Id,
								IsRemote:  false,
							}
						}
					}
				} else {
					err, remoteAcc := database.ReadRemoteAccountById(selectedFollow.TargetAccountId)
					if err == nil && remoteAcc != nil {
						return m, func() tea.Msg {
							return common.ViewProfileMsg{
								Username:  remoteAcc.Username,
								AccountId: remoteAcc.Id,
								IsRemote:  true,
								ActorURI:  remoteAcc.ActorURI,
								Domain:    remoteAcc.Domain,
							}
						}
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("following (%d)", len(m.Following))))
	s.WriteString("\n\n")

	if len(m.Following) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("You're not following anyone yet.\nUse the follow user view to start following!"))
		return tea.NewView(s.String())
	}

	start := m.Offset
	end := min(start+common.DefaultItemsPerPage, len(m.Following))

	for i := start; i < end; i++ {
		follow := m.Following[i]
		database := db.GetDB()

		var username, badge string

		if follow.IsLocal {
			// Local follow - look up in accounts table
			err, localAcc := database.ReadAccById(follow.TargetAccountId)
			if err != nil {
				log.Printf("Failed to read local account: %v", err)
				continue
			}
			username = "@" + localAcc.Username
			badge = " [local]"
			if !follow.Accepted {
				badge += " [pending]"
			}
		} else {
			// Remote follow - look up in remote_accounts table
			err, remoteAcc := database.ReadRemoteAccountById(follow.TargetAccountId)
			if err != nil {
				log.Printf("Failed to read remote account: %v", err)
				continue
			}
			username = fmt.Sprintf("@%s@%s", remoteAcc.Username, remoteAcc.Domain)
			badge = ""
			if !follow.Accepted {
				badge = " [pending]"
			}
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
	if len(m.Following) > common.DefaultItemsPerPage {
		s.WriteString("\n")
		paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.Following))
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

		// Clean up any orphaned follows before loading
		if err := database.CleanupOrphanedFollows(); err != nil {
			log.Printf("Warning: Failed to cleanup orphaned follows: %v", err)
		}

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
