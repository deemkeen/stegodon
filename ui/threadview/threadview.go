package threadview

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

var (
	// Parent post styles (highlighted)
	parentTimeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DIM))

	parentAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_USERNAME)).
				Bold(true)

	parentContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left)

	// Reply styles (indented)
	replyTimeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DIM))

	replyAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_SECONDARY)).
				Bold(true)

	replyContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left)

	// Selected reply styles
	selectedReplyTimeStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedReplyAuthorStyle = lipgloss.NewStyle().
					Align(lipgloss.Left).
					Foreground(lipgloss.Color(common.COLOR_WHITE)).
					Bold(true)

	selectedReplyContentStyle = lipgloss.NewStyle().
					Align(lipgloss.Left).
					Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedBgStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(common.COLOR_ACCENT))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM)).
			Italic(true)

	replyIndent = strings.Repeat(" ", common.ReplyIndentWidth) // Indent for replies
)

// ThreadPost represents a post in the thread (either parent or reply)
type ThreadPost struct {
	ID         uuid.UUID
	Author     string
	Content    string
	Time       time.Time
	ObjectURI  string // ActivityPub object id (canonical URI, returns JSON)
	ObjectURL  string // ActivityPub object url (human-readable web UI link, preferred for display)
	IsLocal    bool   // Whether this is a local post
	IsParent   bool   // Whether this is the parent post
	IsDeleted  bool   // Whether this post was deleted (placeholder)
	ReplyCount int    // Number of replies to this post
	LikeCount  int    // Number of likes on this post
	BoostCount int    // Number of boosts on this post
}

// Model represents the thread view state
type Model struct {
	AccountId    uuid.UUID
	ParentURI    string       // URI of the parent post being viewed
	ParentPost   *ThreadPost  // The parent post
	Replies      []ThreadPost // Replies to the parent
	Selected     int          // Currently selected reply index (-1 = parent selected)
	Offset       int          // Scroll offset for pagination
	Width        int
	Height       int
	isActive     bool
	loading      bool
	errorMessage string
	showingURL   bool // Track if URL is displayed instead of content for selected post
	// Fields to support reloading
	parentNoteID    uuid.UUID // Local note ID (for local notes)
	parentIsLocal   bool      // Whether the parent is a local note
	parentAuthor    string    // Original author (for reload)
	parentContent   string    // Original content (for reload)
	parentCreatedAt time.Time // Original timestamp (for reload)
	// Fields to restore selection after reload
	pendingSelection int                 // Selection to restore after reload (-2 means no pending restore)
	pendingOffset    int                 // Offset to restore after reload
	LocalDomain      string              // Cached local domain for mention highlighting
	ReturnView       common.SessionState // View to return to on Esc (default: HomeTimelineView)
}

// InitialModel creates a new thread view model
func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
	return Model{
		AccountId:        accountId,
		ParentURI:        "",
		ParentPost:       nil,
		Replies:          []ThreadPost{},
		Selected:         -1, // Start with parent selected
		Offset:           -1, // Start at parent
		Width:            width,
		Height:           height,
		isActive:         false,
		loading:          false,
		errorMessage:     "",
		pendingSelection: -2, // -2 means no pending restore
		pendingOffset:    -2,
		LocalDomain:      localDomain,
		ReturnView:       common.HomeTimelineView,
	}
}

// SetThread sets the thread to display
func (m *Model) SetThread(parentURI string) {
	m.ParentURI = parentURI
	m.ParentPost = nil
	m.Replies = []ThreadPost{}
	m.Selected = -1
	m.Offset = -1
	m.loading = true
	m.errorMessage = ""
}

func (m Model) Init() tea.Cmd {
	return nil
}

// threadLoadedMsg is sent when thread data is loaded
type threadLoadedMsg struct {
	parent  *ThreadPost
	replies []ThreadPost
	err     error
}

// loadThread loads the parent post and its replies
func loadThread(parentURI string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Get config for building URIs
		conf, confErr := util.ReadConf()
		var domain string
		if confErr == nil {
			domain = conf.Conf.SslDomain
		}

		var parent *ThreadPost
		var replies []ThreadPost

		// Try to find the parent post
		// First check if it's a local note by URI
		err, localNote := database.ReadNoteByURI(parentURI)
		if err == nil && localNote != nil {
			replyCount, _ := database.CountRepliesByNoteId(localNote.Id)
			parent = &ThreadPost{
				ID:         localNote.Id,
				Author:     localNote.CreatedBy,
				Content:    localNote.Message,
				Time:       localNote.CreatedAt,
				ObjectURI:  localNote.ObjectURI,
				IsLocal:    true,
				IsParent:   true,
				ReplyCount: replyCount,
				LikeCount:  localNote.LikeCount,
				BoostCount: localNote.BoostCount,
			}
		} else {
			// Check if it's a stored activity (federated post)
			err, activity := database.ReadActivityByObjectURI(parentURI)
			if err == nil && activity != nil {
				// Parse activity to get content
				content, author := parseActivityContent(activity)
				// Count local replies to this remote post
				replyCount, _ := database.CountRepliesByURI(parentURI)
				parent = &ThreadPost{
					ID:         activity.Id,
					Author:     author,
					Content:    content,
					Time:       activity.CreatedAt,
					ObjectURI:  activity.ObjectURI,
					ObjectURL:  activity.ObjectURL,
					IsLocal:    false,
					IsParent:   true,
					ReplyCount: replyCount,
					LikeCount:  activity.LikeCount,
					BoostCount: activity.BoostCount,
				}
			}
		}

		// Load replies using the parentURI (which matches in_reply_to_uri)
		err, localReplies := database.ReadRepliesByURI(parentURI)
		if err == nil && localReplies != nil {
			for _, note := range *localReplies {
				// Count local replies for each reply
				replyCount, _ := database.CountRepliesByNoteId(note.Id)

				// Also count remote replies to this local reply
				if domain != "" {
					replyURI := fmt.Sprintf("https://%s/notes/%s", domain, note.Id.String())
					remoteReplyCount, _ := database.CountActivitiesByInReplyTo(replyURI)
					replyCount += remoteReplyCount
				}

				replies = append(replies, ThreadPost{
					ID:         note.Id,
					Author:     note.CreatedBy,
					Content:    note.Message,
					Time:       note.CreatedAt,
					ObjectURI:  note.ObjectURI,
					IsLocal:    true,
					IsParent:   false,
					ReplyCount: replyCount,
					LikeCount:  note.LikeCount,
					BoostCount: note.BoostCount,
				})
			}
		}

		// Also load remote replies from activities table
		err, remoteReplies := database.ReadActivitiesByInReplyTo(parentURI)
		if err == nil && remoteReplies != nil {
			// Build local actor prefix to filter out local users
			localActorPrefix := ""
			if domain != "" {
				localActorPrefix = fmt.Sprintf("https://%s/users/", domain)
			}

			for _, activity := range *remoteReplies {
				// Skip if this is from a local user (already shown as local reply)
				if localActorPrefix != "" && strings.HasPrefix(activity.ActorURI, localActorPrefix) {
					continue
				}
				// Skip if this activity is a duplicate of a local note (federated copy of local post)
				if activity.ObjectURI != "" {
					dupErr, existingNote := database.ReadNoteByURI(activity.ObjectURI)
					if dupErr == nil && existingNote != nil {
						// This is a duplicate, skip it
						continue
					}
				}
				replyContent, replyAuthor := parseActivityContent(&activity)
				// Count replies to this remote reply (could be local notes replying to it)
				replyCount, _ := database.CountRepliesByURI(activity.ObjectURI)
				replies = append(replies, ThreadPost{
					ID:         activity.Id,
					Author:     replyAuthor,
					Content:    replyContent,
					Time:       activity.CreatedAt,
					ObjectURI:  activity.ObjectURI,
					ObjectURL:  activity.ObjectURL,
					IsLocal:    false,
					IsParent:   false,
					ReplyCount: replyCount,
					LikeCount:  activity.LikeCount,
					BoostCount: activity.BoostCount,
				})
			}
		}

		// Sort replies by time
		sort.Slice(replies, func(i, j int) bool {
			return replies[i].Time.Before(replies[j].Time)
		})

		// Update parent reply count to include all replies found
		if parent != nil {
			parent.ReplyCount = len(replies)
		}

		// If parent not found but we have replies, create a deleted placeholder
		if parent == nil {
			if len(replies) > 0 {
				parent = &ThreadPost{
					Author:     "[deleted]",
					Content:    "This post has been deleted",
					IsParent:   true,
					IsDeleted:  true,
					ReplyCount: len(replies),
				}
			} else {
				return threadLoadedMsg{
					parent:  nil,
					replies: nil,
					err:     fmt.Errorf("post not found"),
				}
			}
		}

		return threadLoadedMsg{
			parent:  parent,
			replies: replies,
			err:     nil,
		}
	}
}

// loadThreadByID loads the thread for a local note by its UUID
func loadThreadByID(noteID uuid.UUID, noteURI string, author string, content string, createdAt time.Time) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Get config for building URIs
		conf, confErr := util.ReadConf()
		var domain string
		if confErr == nil {
			domain = conf.Conf.SslDomain
		}

		// Count replies for parent (local + remote)
		parentReplyCount, _ := database.CountRepliesByNoteId(noteID)

		// Get like count, boost count, and content from database
		parentLikeCount := 0
		parentBoostCount := 0
		if err, note := database.ReadNoteId(noteID); err == nil && note != nil {
			parentLikeCount = note.LikeCount
			parentBoostCount = note.BoostCount
			if content == "" {
				content = note.Message
			}
		}

		// Create parent from the provided data
		parent := &ThreadPost{
			ID:         noteID,
			Author:     author,
			Content:    content,
			Time:       createdAt,
			ObjectURI:  noteURI,
			IsLocal:    true,
			IsParent:   true,
			ReplyCount: parentReplyCount,
			LikeCount:  parentLikeCount,
			BoostCount: parentBoostCount,
		}

		// Load local replies using the note ID - this searches for any in_reply_to_uri
		// that contains the note ID (handles various URI formats)
		var replies []ThreadPost
		err, localReplies := database.ReadRepliesByNoteId(noteID)
		if err == nil && localReplies != nil {
			for _, note := range *localReplies {
				// Count local replies for each reply
				replyCount, _ := database.CountRepliesByNoteId(note.Id)

				// Also count remote replies to this local reply
				if domain != "" {
					replyURI := fmt.Sprintf("https://%s/notes/%s", domain, note.Id.String())
					remoteReplyCount, _ := database.CountActivitiesByInReplyTo(replyURI)
					replyCount += remoteReplyCount
				}

				replies = append(replies, ThreadPost{
					ID:         note.Id,
					Author:     note.CreatedBy,
					Content:    note.Message,
					Time:       note.CreatedAt,
					ObjectURI:  note.ObjectURI,
					IsLocal:    true,
					IsParent:   false,
					ReplyCount: replyCount,
					LikeCount:  note.LikeCount,
					BoostCount: note.BoostCount,
				})
			}
		}

		// Also load remote replies from activities table
		// Build the possible URIs that remote servers might use as inReplyTo
		if domain != "" {
			// The canonical URI for this note
			canonicalURI := fmt.Sprintf("https://%s/notes/%s", domain, noteID.String())
			localActorPrefix := fmt.Sprintf("https://%s/users/", domain)

			// Search for remote activities that reply to this note
			err, remoteReplies := database.ReadActivitiesByInReplyTo(canonicalURI)
			if err == nil && remoteReplies != nil {
				for _, activity := range *remoteReplies {
					// Skip if this is from a local user (already shown as local reply)
					if strings.HasPrefix(activity.ActorURI, localActorPrefix) {
						continue
					}
					// Skip if this activity is a duplicate of a local note (federated copy of local post)
					if activity.ObjectURI != "" {
						dupErr, existingNote := database.ReadNoteByURI(activity.ObjectURI)
						if dupErr == nil && existingNote != nil {
							// This is a duplicate, skip it
							continue
						}
					}
					replyContent, replyAuthor := parseActivityContent(&activity)
					// Count replies to this remote reply (could be local notes replying to it)
					replyCount, _ := database.CountRepliesByURI(activity.ObjectURI)
					replies = append(replies, ThreadPost{
						ID:         activity.Id,
						Author:     replyAuthor,
						Content:    replyContent,
						Time:       activity.CreatedAt,
						ObjectURI:  activity.ObjectURI,
						ObjectURL:  activity.ObjectURL,
						IsLocal:    false,
						IsParent:   false,
						ReplyCount: replyCount,
						LikeCount:  activity.LikeCount,
						BoostCount: activity.BoostCount,
					})
				}
			}
		}

		// Sort replies by time
		sort.Slice(replies, func(i, j int) bool {
			return replies[i].Time.Before(replies[j].Time)
		})

		// Update parent reply count to include remote replies
		parent.ReplyCount = len(replies)

		return threadLoadedMsg{
			parent:  parent,
			replies: replies,
			err:     nil,
		}
	}
}

// parseActivityContent extracts content and author from an activity's raw JSON
func parseActivityContent(activity *domain.Activity) (string, string) {
	content := ""
	author := activity.ActorURI

	// Try to get a better author name from the database
	database := db.GetDB()
	err, remoteAcc := database.ReadRemoteAccountByActorURI(activity.ActorURI)
	if err == nil && remoteAcc != nil {
		author = "@" + remoteAcc.Username + "@" + remoteAcc.Domain
	}

	// Parse the raw JSON to extract content using proper JSON unmarshaling
	if activity.RawJSON != "" {
		var activityWrapper struct {
			Type   string `json:"type"`
			Object struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"object"`
		}

		if err := json.Unmarshal([]byte(activity.RawJSON), &activityWrapper); err == nil {
			content = util.StripHTMLTags(activityWrapper.Object.Content)
		}
	}

	return content, author
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.DeactivateViewMsg:
		m.isActive = false
		return m, nil

	case common.ActivateViewMsg:
		m.isActive = true
		// Note: Thread loading is already started by ViewThreadMsg, so we don't need to reload here
		// Only reload if we somehow got back to this view with stale data
		return m, nil

	case common.SessionState:
		// Handle UpdateNoteList to refresh when notes are liked
		if msg == common.UpdateNoteList && m.isActive && m.ParentURI != "" {
			// Store current selection to restore after reload
			m.pendingSelection = m.Selected
			m.pendingOffset = m.Offset
			// Reload the thread to get updated like counts
			if m.parentIsLocal && m.parentNoteID != uuid.Nil {
				return m, loadThreadByID(m.parentNoteID, m.ParentURI, m.parentAuthor, m.parentContent, m.parentCreatedAt)
			}
			return m, loadThread(m.ParentURI)
		}
		return m, nil

	case common.ViewThreadMsg:
		// Load a new thread
		m.SetThread(msg.NoteURI)
		m.isActive = true
		// Store info needed for reload
		m.parentNoteID = msg.NoteID
		m.parentIsLocal = msg.IsLocal
		m.parentAuthor = msg.Author
		m.parentContent = msg.Content
		m.parentCreatedAt = msg.CreatedAt
		// For local notes, use loadThreadByID which doesn't rely on object_uri in DB
		if msg.IsLocal && msg.NoteID != uuid.Nil {
			return m, loadThreadByID(msg.NoteID, msg.NoteURI, msg.Author, msg.Content, msg.CreatedAt)
		}
		return m, loadThread(msg.NoteURI)

	case threadLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			log.Printf("Thread load error: %v", msg.err)
		} else {
			m.ParentPost = msg.parent
			m.Replies = msg.replies
			// Check if we have a pending selection to restore (from reload after like)
			if m.pendingSelection != -2 {
				// Restore selection, making sure it's within bounds
				if m.pendingSelection >= -1 && m.pendingSelection < len(m.Replies) {
					m.Selected = m.pendingSelection
					m.Offset = m.pendingOffset
				} else {
					m.Selected = -1
					m.Offset = -1
				}
				// Clear pending restore
				m.pendingSelection = -2
				m.pendingOffset = -2
			} else {
				// Initial load - select parent
				m.Selected = -1
				m.Offset = -1
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > -1 {
				m.Selected--
				m.Offset = m.Selected
			}
			m.showingURL = false // Reset URL view on navigation
		case "down", "j":
			if m.Selected < len(m.Replies)-1 {
				m.Selected++
				m.Offset = m.Selected
			}
			m.showingURL = false // Reset URL view on navigation
		case "r":
			// Reply to selected post
			if m.Selected == -1 && m.ParentPost != nil && !m.ParentPost.IsDeleted {
				// Reply to parent (only if not deleted)
				preview := m.ParentPost.Content
				if idx := strings.Index(preview, "\n"); idx > 0 {
					preview = preview[:idx]
				}

				// Determine the reply URI
				replyURI := m.ParentPost.ObjectURI
				// For local parents without ObjectURI, use local: prefix with note ID
				if replyURI == "" && m.ParentPost.IsLocal && m.ParentPost.ID != uuid.Nil {
					replyURI = "local:" + m.ParentPost.ID.String()
				}

				if replyURI != "" {
					return m, func() tea.Msg {
						return common.ReplyToNoteMsg{
							NoteURI: replyURI,
							Author:  m.ParentPost.Author,
							Preview: preview,
						}
					}
				}
			} else if m.Selected >= 0 && m.Selected < len(m.Replies) {
				// Reply to selected reply
				reply := m.Replies[m.Selected]
				preview := reply.Content
				if idx := strings.Index(preview, "\n"); idx > 0 {
					preview = preview[:idx]
				}

				// Determine the reply URI
				replyURI := reply.ObjectURI
				// For local replies without ObjectURI, use local: prefix with note ID
				if replyURI == "" && reply.IsLocal && reply.ID != uuid.Nil {
					replyURI = "local:" + reply.ID.String()
				}

				if replyURI != "" {
					return m, func() tea.Msg {
						return common.ReplyToNoteMsg{
							NoteURI: replyURI,
							Author:  reply.Author,
							Preview: preview,
						}
					}
				}
			}
		case "enter":
			// Open selected reply as a new thread (only if it has replies)
			if m.Selected >= 0 && m.Selected < len(m.Replies) {
				reply := m.Replies[m.Selected]

				// Skip if no replies
				if reply.ReplyCount == 0 {
					return m, nil
				}

				// Determine the note URI
				noteURI := reply.ObjectURI
				if noteURI == "" && reply.IsLocal && reply.ID != uuid.Nil {
					noteURI = "local:" + reply.ID.String()
				}

				if noteURI != "" || (reply.IsLocal && reply.ID != uuid.Nil) {
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   noteURI,
							NoteID:    reply.ID,
							Author:    reply.Author,
							Content:   reply.Content,
							CreatedAt: reply.Time,
							IsLocal:   reply.IsLocal,
						}
					}
				}
			}
		case "esc", "q":
			// Go back to the view that opened this thread
			returnView := m.ReturnView
			return m, func() tea.Msg {
				return returnView
			}
		case "l":
			// Like/unlike selected post
			if m.Selected == -1 && m.ParentPost != nil && !m.ParentPost.IsDeleted {
				// Like the parent post
				noteURI := m.ParentPost.ObjectURI
				if noteURI == "" && m.ParentPost.IsLocal && m.ParentPost.ID != uuid.Nil {
					noteURI = "local:" + m.ParentPost.ID.String()
				}
				if noteURI != "" || m.ParentPost.ID != uuid.Nil {
					return m, func() tea.Msg {
						return common.LikeNoteMsg{
							NoteURI: noteURI,
							NoteID:  m.ParentPost.ID,
							IsLocal: m.ParentPost.IsLocal,
						}
					}
				}
			} else if m.Selected >= 0 && m.Selected < len(m.Replies) {
				// Like a reply
				reply := m.Replies[m.Selected]
				noteURI := reply.ObjectURI
				if noteURI == "" && reply.IsLocal && reply.ID != uuid.Nil {
					noteURI = "local:" + reply.ID.String()
				}
				if noteURI != "" || reply.ID != uuid.Nil {
					return m, func() tea.Msg {
						return common.LikeNoteMsg{
							NoteURI: noteURI,
							NoteID:  reply.ID,
							IsLocal: reply.IsLocal,
						}
					}
				}
			}
		case "b":
			// Boost/unboost selected post
			if m.Selected == -1 && m.ParentPost != nil && !m.ParentPost.IsDeleted {
				// Boost the parent post
				noteURI := m.ParentPost.ObjectURI
				if noteURI == "" && m.ParentPost.IsLocal && m.ParentPost.ID != uuid.Nil {
					noteURI = "local:" + m.ParentPost.ID.String()
				}
				if noteURI != "" || m.ParentPost.ID != uuid.Nil {
					return m, func() tea.Msg {
						return common.BoostNoteMsg{
							NoteURI: noteURI,
							NoteID:  m.ParentPost.ID,
							IsLocal: m.ParentPost.IsLocal,
						}
					}
				}
			} else if m.Selected >= 0 && m.Selected < len(m.Replies) {
				// Boost a reply
				reply := m.Replies[m.Selected]
				noteURI := reply.ObjectURI
				if noteURI == "" && reply.IsLocal && reply.ID != uuid.Nil {
					noteURI = "local:" + reply.ID.String()
				}
				if noteURI != "" || reply.ID != uuid.Nil {
					return m, func() tea.Msg {
						return common.BoostNoteMsg{
							NoteURI: noteURI,
							NoteID:  reply.ID,
							IsLocal: reply.IsLocal,
						}
					}
				}
			}
		case "o":
			// Toggle between showing content and URL (only for posts with valid HTTP/HTTPS URLs)
			// Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
			if m.Selected == -1 && m.ParentPost != nil {
				// Toggle URL for parent post
				displayURL := m.ParentPost.ObjectURL
				if displayURL == "" {
					displayURL = m.ParentPost.ObjectURI
				}
				if util.IsURL(displayURL) {
					m.showingURL = !m.showingURL
				}
			} else if m.Selected >= 0 && m.Selected < len(m.Replies) {
				// Toggle URL for selected reply
				reply := m.Replies[m.Selected]
				displayURL := reply.ObjectURL
				if displayURL == "" {
					displayURL = reply.ObjectURI
				}
				if util.IsURL(displayURL) {
					m.showingURL = !m.showingURL
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	// Header
	replyCount := len(m.Replies)
	if replyCount == 1 {
		s.WriteString(common.CaptionStyle.Render("thread (1 reply)"))
	} else {
		s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("thread (%d replies)", replyCount)))
	}
	s.WriteString("\n\n")

	if m.loading {
		s.WriteString(emptyStyle.Render("Loading thread..."))
		return s.String()
	}

	if m.errorMessage != "" {
		s.WriteString(emptyStyle.Render("Error: " + m.errorMessage))
		s.WriteString("\n\n")
		s.WriteString(common.HelpStyle.Render("esc: back"))
		return s.String()
	}

	if m.ParentPost == nil {
		s.WriteString(emptyStyle.Render("No thread to display"))
		s.WriteString("\n\n")
		s.WriteString(common.HelpStyle.Render("esc: back"))
		return s.String()
	}

	// Calculate content width using layout helpers (same as home timeline)
	leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
	rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
	contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)

	itemsPerPage := common.DefaultItemsPerPage
	// Offset is -1 for parent, 0+ for replies
	start := m.Offset
	end := start + itemsPerPage

	// Total items: parent (-1) + replies (0 to len-1)
	totalItems := len(m.Replies) // -1 to len(Replies)-1
	if end > totalItems {
		end = totalItems
	}

	// Render items from start to end
	for i := start; i < end; i++ {
		var post *ThreadPost
		var isParent bool

		if i == -1 {
			post = m.ParentPost
			isParent = true
		} else if i >= 0 && i < len(m.Replies) {
			post = &m.Replies[i]
			isParent = false
		} else {
			continue
		}

		isSelected := i == m.Selected

		// Determine indent for replies: use PaddingLeft so all wrapped lines are indented
		indentWidth := 0
		itemWidth := contentWidth
		if !isParent {
			indentWidth = len(replyIndent)
			itemWidth = contentWidth - indentWidth
		}

		// Format timestamp with engagement indicators
		timeStr := formatTime(post.Time)
		if post.ReplyCount == 1 {
			timeStr = fmt.Sprintf("%s · 1 reply", timeStr)
		} else if post.ReplyCount > 1 {
			timeStr = fmt.Sprintf("%s · %d replies", timeStr, post.ReplyCount)
		}
		if post.LikeCount > 0 {
			timeStr = fmt.Sprintf("%s · ⭐ %d", timeStr, post.LikeCount)
		}
		if post.BoostCount > 0 {
			timeStr = fmt.Sprintf("%s · 🔁 %d", timeStr, post.BoostCount)
		}

		// Format author with @ prefix for all users
		author := post.Author
		if !strings.HasPrefix(author, "@") {
			author = "@" + author
		}

		// Format content - Unescape HTML entities, convert Markdown links, then highlight hashtags and mentions (same order as myposts)
		processedContent := post.Content
		processedContent = util.TruncateContent(processedContent, common.MaxDisplayContentLength)
		if post.IsLocal {
			processedContent = util.UnescapeHTML(processedContent)
			processedContent = util.MarkdownLinksToTerminal(processedContent)
		} else {
			// Normalize emojis for remote posts to fix terminal width calculation issues
			processedContent = util.NormalizeEmojis(processedContent)
		}
		processedContent = util.LinkifyRawURLsTerminal(processedContent)
		highlightedContent := util.HighlightHashtagsTerminal(processedContent)
		highlightedContent = util.HighlightMentionsTerminal(highlightedContent, m.LocalDomain)

		if isSelected {
			// Create a style that fills the full width (same approach as myposts/hometimeline)
			selectedBg := lipgloss.NewStyle().
				Background(lipgloss.Color(common.COLOR_ACCENT)).
				Width(itemWidth)

			timeFormatted := selectedBg.Render(selectedReplyTimeStyle.Render(timeStr))
			authorFormatted := selectedBg.Render(selectedReplyAuthorStyle.Render(author))

			var contentFormatted string
			// Toggle between content and URL
			// Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
			displayURL := post.ObjectURL
			if displayURL == "" {
				displayURL = post.ObjectURI
			}
			if m.showingURL && displayURL != "" {
				osc8Link := util.FormatClickableURL(displayURL, common.MaxContentTruncateWidth, "🔗 ")
				hintText := "(Cmd+click to open, press 'o' to toggle back)"

				contentStyleBg := lipgloss.NewStyle().
					Background(lipgloss.Color(common.COLOR_ACCENT)).
					Foreground(lipgloss.Color(common.COLOR_WHITE)).
					Width(itemWidth)
				contentFormatted = contentStyleBg.Render(osc8Link + "\n\n" + hintText)
			} else {
				contentFormatted = selectedBg.Render(selectedReplyContentStyle.Render(highlightedContent))
			}

			// Build the post block
			postBlock := timeFormatted + "\n" + authorFormatted + "\n" + contentFormatted

			// Apply left padding for replies (affects all lines including wrapped)
			if indentWidth > 0 {
				postBlock = lipgloss.NewStyle().PaddingLeft(indentWidth).Render(postBlock)
			}
			s.WriteString(postBlock)
		} else {
			unselectedStyle := lipgloss.NewStyle().Width(itemWidth)

			// Use different author color for local vs remote
			var authorFormatted string
			if post.IsLocal {
				authorFormatted = unselectedStyle.Render(parentAuthorStyle.Render(author))
			} else {
				authorFormatted = unselectedStyle.Render(replyAuthorStyle.Render(author))
			}

			timeFormatted := unselectedStyle.Render(parentTimeStyle.Render(timeStr))
			contentFormatted := unselectedStyle.Render(parentContentStyle.Render(highlightedContent))

			// Build the post block
			postBlock := timeFormatted + "\n" + authorFormatted + "\n" + contentFormatted

			// Apply left padding for replies (affects all lines including wrapped)
			if indentWidth > 0 {
				postBlock = lipgloss.NewStyle().PaddingLeft(indentWidth).Render(postBlock)
			}
			s.WriteString(postBlock)
		}

		s.WriteString("\n\n")
	}

	return s.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if duration < common.HoursPerDay*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(duration.Hours() / common.HoursPerDay)
		return fmt.Sprintf("%dd ago", days)
	}
}
