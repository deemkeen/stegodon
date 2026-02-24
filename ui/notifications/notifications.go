package notifications

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/deemkeen/stegodon/web"
	"github.com/google/uuid"
)

const (
	notificationsLimit = 50
	refreshInterval    = 30 * time.Second
)

type Model struct {
	AccountId     uuid.UUID
	Notifications []domain.Notification
	Selected      int
	Offset        int
	Width         int
	Height        int
	isActive      bool
	tickerRunning bool // Track if refresh ticker is already running
	UnreadCount   int
	Status        string
	Error         string
}

type notificationsLoadedMsg struct {
	notifications []domain.Notification
	unreadCount   int
}

type refreshTickMsg struct{}

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

// followRemoteUserCmd returns a command that follows a remote user
func followRemoteUserCmd(accountId uuid.UUID, username, domain string) tea.Cmd {
	return func() tea.Msg {
		fullUsername := fmt.Sprintf("%s@%s", username, domain)

		// Get local account
		database := db.GetDB()
		err, localAccount := database.ReadAccById(accountId)
		if err != nil {
			return followResultMsg{username: fullUsername, err: err}
		}

		// Resolve WebFinger to get actor URI
		actorURI, err := web.ResolveWebFinger(username, domain)
		if err != nil {
			return followResultMsg{username: fullUsername, err: fmt.Errorf("webfinger failed: %w", err)}
		}

		// Get config
		conf, err := util.ReadConf()
		if err != nil {
			return followResultMsg{username: fullUsername, err: err}
		}

		// Send Follow activity
		if err := activitypub.SendFollow(localAccount, actorURI, conf); err != nil {
			return followResultMsg{username: fullUsername, err: err}
		}

		return followResultMsg{username: fullUsername, err: nil}
	}
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId:     accountId,
		Notifications: []domain.Notification{},
		Selected:      0,
		Offset:        0,
		Width:         width,
		Height:        height,
		isActive:      false,
		UnreadCount:   0,
		Status:        "",
		Error:         "",
	}
}

func (m Model) Init() tea.Cmd {
	return nil // Don't start commands - model starts inactive
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.ActivateViewMsg:
		// Notifications model is always active for badge updates
		// Just load data when view becomes focused
		m.isActive = true
		return m, loadNotifications(m.AccountId)

	case common.DeactivateViewMsg:
		// Don't actually deactivate - keep refreshing for badge
		// Just mark as not actively viewing
		m.isActive = false
		return m, nil

	case notificationsLoadedMsg:
		m.Notifications = msg.notifications
		m.UnreadCount = msg.unreadCount
		// Keep selection within bounds
		if m.Selected >= len(m.Notifications) {
			m.Selected = len(m.Notifications) - 1
		}
		if m.Selected < 0 {
			m.Selected = 0
		}
		// Schedule next tick to keep badge updated (only if not already running)
		if !m.tickerRunning {
			m.tickerRunning = true
			return m, tickRefresh()
		}
		return m, nil

	case refreshTickMsg:
		// Always refresh to keep badge count updated
		return m, loadNotifications(m.AccountId)

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
			if m.Selected < len(m.Notifications)-1 {
				m.Selected++
				// Scroll down if needed
				itemsPerPage := common.DefaultItemsPerPage
				if m.Selected >= m.Offset+itemsPerPage {
					m.Offset = m.Selected - itemsPerPage + 1
				}
			}
		case "v":
			// View notification source (the post/reply)
			if m.Selected < len(m.Notifications) {
				notif := m.Notifications[m.Selected]
				// Only view if notification has an associated note (either URI or ID)
				if notif.NotificationType != domain.NotificationFollow && (notif.NoteURI != "" || notif.NoteId != uuid.Nil) {
					// Get the actual note creation time from database
					database := db.GetDB()
					var noteCreatedAt time.Time
					var noteContent string
					var noteAuthor string
					var noteURI string
					var noteID uuid.UUID
					var isLocal bool

					if notif.NoteId != uuid.Nil {
						// Local note - get from notes table
						isLocal = true
						err, note := database.ReadNoteId(notif.NoteId)
						if err == nil && note != nil {
							noteCreatedAt = note.CreatedAt
							noteContent = note.Message
							noteAuthor = note.CreatedBy
							noteURI = note.ObjectURI
							if noteURI == "" {
								// Use local: prefix for notes without ObjectURI
								noteURI = "local:" + note.Id.String()
							}
							noteID = note.Id
							log.Printf("[Notifications] Loaded local note %s by %s", noteID, noteAuthor)
						} else {
							log.Printf("[Notifications] Failed to load local note %s: %v", notif.NoteId, err)
						}
					} else if notif.NoteURI != "" {
						// Remote note - try to find in activities table
						err, activity := database.ReadActivityByObjectURI(notif.NoteURI)
						if err == nil && activity != nil {
							noteCreatedAt = activity.CreatedAt
							noteURI = activity.ObjectURI
							noteID = activity.Id
							isLocal = false
							// Parse activity content
							var activityWrapper struct {
								Object struct {
									Content string `json:"content"`
								} `json:"object"`
							}
							if err := json.Unmarshal([]byte(activity.RawJSON), &activityWrapper); err == nil {
								noteContent = util.StripHTMLTags(activityWrapper.Object.Content)
							}
							// Get author from notification
							noteAuthor = notif.ActorUsername
							if notif.ActorDomain != "" {
								noteAuthor = fmt.Sprintf("%s@%s", notif.ActorUsername, notif.ActorDomain)
							}
							log.Printf("[Notifications] Loaded remote activity %s by %s", noteURI, noteAuthor)
						} else {
							log.Printf("[Notifications] Failed to load remote activity %s: %v", notif.NoteURI, err)
						}
					}

					// Fallback to notification data if database lookup failed
					if noteCreatedAt.IsZero() {
						noteCreatedAt = notif.CreatedAt
						log.Printf("[Notifications] Using fallback timestamp from notification")
					}
					if noteContent == "" {
						noteContent = notif.NotePreview
						if noteContent == "" {
							noteContent = "[No preview available]"
						}
						log.Printf("[Notifications] Using fallback content from notification preview")
					}
					if noteAuthor == "" {
						noteAuthor = notif.ActorUsername
						if notif.ActorDomain != "" {
							noteAuthor = fmt.Sprintf("%s@%s", notif.ActorUsername, notif.ActorDomain)
						}
						log.Printf("[Notifications] Using fallback author from notification")
					}
					if noteURI == "" {
						noteURI = notif.NoteURI
						log.Printf("[Notifications] Using notification NoteURI: %s", noteURI)
					}
					if noteID == uuid.Nil {
						noteID = notif.NoteId
						log.Printf("[Notifications] Using notification NoteId: %s", noteID)
					}

					log.Printf("[Notifications] ViewThreadMsg: URI=%s, ID=%s, IsLocal=%v, Author=%s",
						noteURI, noteID, isLocal, noteAuthor)

					// Send ViewThreadMsg to navigate to the post
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   noteURI,
							NoteID:    noteID,
							IsLocal:   isLocal,
							Author:    noteAuthor,
							Content:   noteContent,
							CreatedAt: noteCreatedAt,
						}
					}
				}
			}
		case "f":
			// Follow user back
			if m.Selected < len(m.Notifications) {
				notif := m.Notifications[m.Selected]

				// Determine if local or remote
				if notif.ActorDomain == "" {
					// Local user
					m.Status = fmt.Sprintf("Following @%s...", notif.ActorUsername)
					return m, followLocalUserCmd(m.AccountId, notif.ActorId)
				} else {
					// Remote user
					m.Status = fmt.Sprintf("Requesting to follow @%s@%s...", notif.ActorUsername, notif.ActorDomain)
					return m, followRemoteUserCmd(m.AccountId, notif.ActorUsername, notif.ActorDomain)
				}
			}
		case "enter":
			// Delete notification (mark as read by removing it)
			if m.Selected < len(m.Notifications) {
				notif := m.Notifications[m.Selected]
				return m, deleteNotification(notif.Id, m.AccountId)
			}
		case "a":
			// Delete all notifications (mark all as read by removing them)
			return m, deleteAllNotifications(m.AccountId)
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var s strings.Builder

	// Header
	title := fmt.Sprintf("🔔 Notifications (%d unread)", m.UnreadCount)
	s.WriteString(common.CaptionStyle.Render(title))
	s.WriteString("\n\n")

	if len(m.Notifications) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No notifications yet."))
		return tea.NewView(s.String())
	}

	// Calculate visible range
	itemsPerPage := common.DefaultItemsPerPage
	start := m.Offset
	end := start + itemsPerPage
	if end > len(m.Notifications) {
		end = len(m.Notifications)
	}

	// Render notifications
	for i := start; i < end; i++ {
		notif := m.Notifications[i]
		selected := i == m.Selected

		// Format notification
		line1 := fmt.Sprintf("%s %s %s", notif.TypeIcon(), notif.ActorHandle(), notif.TypeLabel())
		timeAgo := formatTimeAgo(notif.CreatedAt)

		if selected {
			// Selected styling
			if !notif.Read {
				s.WriteString(common.ListSelectedPrefix +
					lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(common.COLOR_USERNAME)).Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			} else {
				s.WriteString(common.ListSelectedPrefix +
					common.ListItemSelectedStyle.Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			}
		} else {
			// Normal styling
			if !notif.Read {
				s.WriteString(common.ListUnselectedPrefix +
					lipgloss.NewStyle().Bold(true).Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			} else {
				s.WriteString(common.ListUnselectedPrefix +
					common.ListItemStyle.Render(line1) +
					"  " + common.ListBadgeStyle.Render(timeAgo))
			}
		}
		s.WriteString("\n")

		// Show preview for like/reply/mention (indented)
		if notif.NotePreview != "" && notif.NotificationType != domain.NotificationFollow {
			preview := truncate(notif.NotePreview, 60)
			s.WriteString("  " + common.ListBadgeStyle.Render("\""+preview+"\""))
			s.WriteString("\n")
		}
	}

	// Pagination info
	if len(m.Notifications) > itemsPerPage {
		pageInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.Notifications))
		s.WriteString("\n" + common.ListBadgeStyle.Render(pageInfo))
	}

	if m.Status != "" {
		s.WriteString("\n")
		s.WriteString(common.ListStatusStyle.Render(m.Status))
	}

	if m.Error != "" {
		s.WriteString("\n")
		s.WriteString(common.ListErrorStyle.Render(m.Error))
	}

	return tea.NewView(s.String())
}

// loadNotifications loads notifications for an account
func loadNotifications(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, notifications := database.ReadNotificationsByAccountId(accountId, notificationsLimit)
		if err != nil {
			log.Printf("Failed to load notifications: %v", err)
			return notificationsLoadedMsg{notifications: []domain.Notification{}, unreadCount: 0}
		}

		// Get unread count
		unreadCount, err := database.ReadUnreadNotificationCount(accountId)
		if err != nil {
			log.Printf("Failed to get unread count: %v", err)
			unreadCount = 0
		}

		return notificationsLoadedMsg{
			notifications: *notifications,
			unreadCount:   unreadCount,
		}
	}
}

// deleteNotification deletes a single notification
func deleteNotification(notificationId uuid.UUID, accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		if err := database.DeleteNotification(notificationId); err != nil {
			log.Printf("Failed to delete notification: %v", err)
		}
		// Reload notifications to update the view
		return loadNotifications(accountId)()
	}
}

// deleteAllNotifications deletes all notifications for an account
func deleteAllNotifications(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		if err := database.DeleteAllNotifications(accountId); err != nil {
			log.Printf("Failed to delete all notifications: %v", err)
		}
		// Reload notifications to update the view
		return loadNotifications(accountId)()
	}
}

// tickRefresh returns a command that triggers a refresh after a delay
func tickRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

// formatTimeAgo formats a time as a relative string (e.g., "2h ago")
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / 24 / 7)
		return fmt.Sprintf("%dw ago", weeks)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		return fmt.Sprintf("%dmo ago", months)
	} else {
		years := int(duration.Hours() / 24 / 365)
		return fmt.Sprintf("%dy ago", years)
	}
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
