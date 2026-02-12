package globalposts

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

var (
	timeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DIM))

	authorStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_USERNAME)).
			Bold(true)

	// Remote author uses secondary color to differentiate from local
	remoteAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_SECONDARY)).
				Bold(true)

	contentStyle = lipgloss.NewStyle().
			Align(lipgloss.Left)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM)).
			Italic(true)

	// Inverted styles for selected posts
	selectedTimeStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE)).
				Bold(true)

	selectedContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	// Boost indicator style (dim, italic)
	boostIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_DIM)).
				Italic(true)

	selectedBoostIndicatorStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color(common.COLOR_WHITE)).
					Italic(true)
)

type Model struct {
	AccountId          uuid.UUID
	Posts              []domain.GlobalTimelinePost
	Offset             int // Pagination offset
	Selected           int // Currently selected post index
	Width              int
	Height             int
	isActive           bool     // Track if this view is currently visible
	tickerRunning      bool     // Track if refresh ticker is already running (prevents multiple ticker chains)
	showingURL         bool     // Track if URL is displayed instead of content
	showingEngagement  bool     // Track if engagement info (likes/boosts) is displayed
	engagementLikers   []string // List of users who liked the selected post
	engagementBoosters []string // List of users who boosted the selected post
	LocalDomain        string
	spinner            spinner.Model
	initialLoad        bool // true until first postsLoadedMsg arrives
}

func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_SECONDARY))
	return Model{
		AccountId:   accountId,
		Posts:       []domain.GlobalTimelinePost{},
		Offset:      0,
		Selected:    0,
		Width:       width,
		Height:      height,
		isActive:    false,
		LocalDomain: localDomain,
		spinner:     s,
		initialLoad: false, // Set to true on ActivateViewMsg
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// refreshTickMsg is sent periodically to refresh the timeline
type refreshTickMsg struct{}

// tickRefresh returns a command that sends refreshTickMsg
func tickRefresh() tea.Cmd {
	return tea.Tick(common.TimelineRefreshSeconds*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.initialLoad {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case common.DeactivateViewMsg:
		m.isActive = false
		m.tickerRunning = false // Stop ticker chain
		return m, nil

	case common.ActivateViewMsg:
		m.isActive = true
		m.tickerRunning = false // Reset ticker state
		m.initialLoad = true
		m.Selected = 0
		m.Offset = 0
		return m, tea.Batch(loadGlobalPosts(), m.spinner.Tick)

	case common.SessionState:
		if msg == common.UpdateNoteList {
			return m, loadGlobalPosts()
		}
		return m, nil

	case refreshTickMsg:
		if m.isActive {
			return m, loadGlobalPosts()
		}
		return m, nil

	case postsLoadedMsg:
		m.initialLoad = false
		m.Posts = msg.posts
		// Pre-process content for terminal display (avoids re-processing on every View() render)
		for i := range m.Posts {
			m.Posts[i].ProcessedContent = processGlobalPostContent(m.Posts[i], m.LocalDomain)
		}
		if m.Selected >= len(m.Posts) {
			m.Selected = max(0, len(m.Posts)-1)
		}
		m.Offset = m.Selected

		// Only start ticker if active AND no ticker already running
		// This prevents multiple ticker chains from UpdateNoteList reloads
		if m.isActive && !m.tickerRunning {
			m.tickerRunning = true
			return m, tickRefresh()
		}
		return m, nil

	case engagementInfoMsg:
		m.engagementLikers = msg.likers
		m.engagementBoosters = msg.boosters
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected
			}
			m.showingURL = false
			m.showingEngagement = false
		case "down", "j":
			if len(m.Posts) > 0 && m.Selected < len(m.Posts)-1 {
				m.Selected++
				m.Offset = m.Selected
			}
			m.showingURL = false
			m.showingEngagement = false
		case "o":
			// Toggle between showing content and URL
			// Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				displayURL := selectedPost.ObjectURL
				if displayURL == "" {
					displayURL = selectedPost.ObjectURI
				}
				if util.IsURL(displayURL) {
					m.showingURL = !m.showingURL
				}
			}
		case "r":
			// Reply to selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// Construct reply URI - use ObjectURI (ActivityPub canonical ID) for proper federation
				replyURI := selectedPost.ObjectURI
				if !selectedPost.IsRemote && replyURI == "" {
					replyURI = "local:" + selectedPost.NoteId
				}

				if replyURI != "" {
					preview := selectedPost.Message
					if idx := strings.Index(preview, "\n"); idx > 0 {
						preview = preview[:idx]
					}
					return m, func() tea.Msg {
						return common.ReplyToNoteMsg{
							NoteURI: replyURI,
							Author:  selectedPost.Username,
							Preview: preview,
						}
					}
				}
			}
		case "enter":
			// Open thread view for selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// Skip if no replies
				if selectedPost.ReplyCount == 0 {
					return m, nil
				}

				// Use ObjectURI (ActivityPub canonical ID) for thread lookup
				noteURI := selectedPost.ObjectURI
				var noteID uuid.UUID
				if !selectedPost.IsRemote {
					// Parse UUID from NoteId string for local posts
					if id, err := uuid.Parse(selectedPost.NoteId); err == nil {
						noteID = id
						if noteURI == "" {
							noteURI = "local:" + selectedPost.NoteId
						}
					}
				}

				if noteURI != "" {
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   noteURI,
							NoteID:    noteID,
							Author:    selectedPost.Username,
							Content:   selectedPost.Message,
							CreatedAt: selectedPost.CreatedAt,
							IsLocal:   !selectedPost.IsRemote,
						}
					}
				}
			}
		case "l":
			// Like/unlike the selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// Use ObjectURI (ActivityPub canonical ID) for proper federation
				noteURI := selectedPost.ObjectURI
				var noteID uuid.UUID

				// For local posts without ObjectURI, use local: prefix with NoteId
				if !selectedPost.IsRemote && selectedPost.NoteId != "" {
					if id, err := uuid.Parse(selectedPost.NoteId); err == nil {
						noteID = id
						if noteURI == "" {
							noteURI = "local:" + selectedPost.NoteId
						}
					}
				}

				// Send like if we have either a URI or a note ID
				if noteURI != "" || noteID != uuid.Nil {
					return m, func() tea.Msg {
						return common.LikeNoteMsg{
							NoteURI: noteURI,
							NoteID:  noteID,
							IsLocal: !selectedPost.IsRemote,
						}
					}
				}
			}
		case "b":
			// Boost/unboost the selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// Use ObjectURI (ActivityPub canonical ID) for proper federation
				noteURI := selectedPost.ObjectURI
				var noteID uuid.UUID

				// For local posts without ObjectURI, use local: prefix with NoteId
				if !selectedPost.IsRemote && selectedPost.NoteId != "" {
					if id, err := uuid.Parse(selectedPost.NoteId); err == nil {
						noteID = id
						if noteURI == "" {
							noteURI = "local:" + selectedPost.NoteId
						}
					}
				}

				// Send boost if we have either a URI or a note ID
				if noteURI != "" || noteID != uuid.Nil {
					return m, func() tea.Msg {
						return common.BoostNoteMsg{
							NoteURI: noteURI,
							NoteID:  noteID,
							IsLocal: !selectedPost.IsRemote,
						}
					}
				}
			}
		case "i":
			// Toggle engagement info display
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				if selectedPost.LikeCount > 0 || selectedPost.BoostCount > 0 {
					m.showingEngagement = !m.showingEngagement
					if m.showingEngagement {
						return m, loadEngagementInfoGlobal(selectedPost)
					}
				}
			}
		case "f":
			// Follow user (for remote users only) - navigate to follow view
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				if selectedPost.IsRemote && selectedPost.UserDomain != "" {
					// Navigate to follow view where user can enter the handle
					return m, func() tea.Msg {
						return common.SessionState(common.FollowUserView)
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("global (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if m.initialLoad {
		s.WriteString(m.spinner.View() + " Loading...")
	} else if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No posts yet."))
	} else {
		leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
		rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
		contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)

		itemsPerPage := common.DefaultItemsPerPage
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			timeStr := formatTime(post.CreatedAt)
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

			author := post.Username
			if !strings.HasPrefix(author, "@") {
				author = "@" + author
			}

			if i == m.Selected {
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color(common.COLOR_ACCENT)).
					Width(contentWidth)

				timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr))
				authorFormatted := selectedBg.Render(selectedAuthorStyle.Render(author))

				// Show engagement info if toggled
				if m.showingEngagement {
					var engagementText strings.Builder
					hasContent := false

					if len(m.engagementLikers) > 0 {
						hasContent = true
						engagementText.WriteString("⭐ Liked by:\n")
						for idx, liker := range m.engagementLikers {
							if idx < 10 {
								engagementText.WriteString(fmt.Sprintf("  @%s\n", liker))
							}
						}
						if len(m.engagementLikers) > 10 {
							engagementText.WriteString(fmt.Sprintf("  ...and %d more\n", len(m.engagementLikers)-10))
						}
					}

					if len(m.engagementBoosters) > 0 {
						hasContent = true
						if len(m.engagementLikers) > 0 {
							engagementText.WriteString("\n")
						}
						engagementText.WriteString("🔁 Boosted by:\n")
						for idx, booster := range m.engagementBoosters {
							if idx < 10 {
								engagementText.WriteString(fmt.Sprintf("  @%s\n", booster))
							}
						}
						if len(m.engagementBoosters) > 10 {
							engagementText.WriteString(fmt.Sprintf("  ...and %d more\n", len(m.engagementBoosters)-10))
						}
					}

					if !hasContent {
						engagementText.WriteString("No engagement information available yet.\n")
						engagementText.WriteString("(Likes and boosts by local users will appear here)")
					}

					engagementText.WriteString("\n\n(Press 'i' to toggle back)")

					contentStyleBg := lipgloss.NewStyle().
						Background(lipgloss.Color(common.COLOR_ACCENT)).
						Foreground(lipgloss.Color(common.COLOR_WHITE)).
						Width(contentWidth)
					contentFormatted := contentStyleBg.Render(engagementText.String())

					s.WriteString(timeFormatted + "\n")
					s.WriteString(authorFormatted + "\n")
					s.WriteString(contentFormatted)
				} else if m.showingURL {
					// Show URL instead of content
					// Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
					displayURL := post.ObjectURL
					if displayURL == "" {
						displayURL = post.ObjectURI
					}
					if util.IsURL(displayURL) {
						osc8Link := util.FormatClickableURL(displayURL, common.MaxContentTruncateWidth, "🔗 ")
						hintText := "(Cmd+click to open, press 'o' to toggle back)"

						contentStyleBg := lipgloss.NewStyle().
							Background(lipgloss.Color(common.COLOR_ACCENT)).
							Foreground(lipgloss.Color(common.COLOR_WHITE)).
							Width(contentWidth)
						contentFormatted := contentStyleBg.Render(osc8Link + "\n\n" + hintText)

						s.WriteString(timeFormatted + "\n")
						s.WriteString(authorFormatted + "\n")
						s.WriteString(contentFormatted)
					}
				} else {
					// Use pre-processed content (cached in postsLoadedMsg), add URL linkification for selected post
					displayContent := post.ProcessedContent
					if displayContent == "" {
						displayContent = processGlobalPostContent(post, m.LocalDomain)
					}
					highlightedContent := util.LinkifyRawURLsTerminal(displayContent)

					contentFormatted := selectedBg.Render(selectedContentStyle.Render(highlightedContent))
					s.WriteString(timeFormatted + "\n")
					// Show boost indicator if this is a boosted post
					if post.BoostedBy != "" {
						boostLine := fmt.Sprintf("🔁 %s boosted", post.BoostedBy)
						boostFormatted := selectedBg.Render(selectedBoostIndicatorStyle.Render(boostLine))
						s.WriteString(boostFormatted + "\n")
					}
					s.WriteString(authorFormatted + "\n")
					s.WriteString(contentFormatted)
				}
			} else {
				unselectedStyle := lipgloss.NewStyle().
					Width(contentWidth)

				// Use pre-processed content (cached in postsLoadedMsg)
				highlightedContent := post.ProcessedContent
				if highlightedContent == "" {
					highlightedContent = processGlobalPostContent(post, m.LocalDomain)
				}

				var authorFormatted string
				if !post.IsRemote {
					authorFormatted = unselectedStyle.Render(authorStyle.Render(author))
				} else {
					authorFormatted = unselectedStyle.Render(remoteAuthorStyle.Render(author))
				}

				timeFormatted := unselectedStyle.Render(timeStyle.Render(timeStr))
				contentFormatted := unselectedStyle.Render(contentStyle.Render(highlightedContent))

				s.WriteString(timeFormatted + "\n")
				// Show boost indicator if this is a boosted post
				if post.BoostedBy != "" {
					boostLine := fmt.Sprintf("🔁 %s boosted", post.BoostedBy)
					boostFormatted := unselectedStyle.Render(boostIndicatorStyle.Render(boostLine))
					s.WriteString(boostFormatted + "\n")
				}
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			}

			s.WriteString("\n\n")
		}
	}

	return s.String()
}

// processGlobalPostContent applies the full text processing pipeline for terminal display.
// This is called once when posts are loaded (in postsLoadedMsg) and cached in ProcessedContent.
func processGlobalPostContent(post domain.GlobalTimelinePost, localDomain string) string {
	return common.ProcessPostContent(post.Message, !post.IsRemote, localDomain, false)
}

// postsLoadedMsg is sent when posts are loaded
type postsLoadedMsg struct {
	posts []domain.GlobalTimelinePost
}

// engagementInfoMsg is sent when engagement info is loaded
type engagementInfoMsg struct {
	likers   []string
	boosters []string
}

// loadGlobalPosts loads the global timeline
func loadGlobalPosts() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, posts := database.ReadGlobalTimelinePosts(common.HomeTimelinePostLimit, 0)
		if err != nil {
			log.Printf("Failed to load global timeline: %v", err)
			return postsLoadedMsg{posts: []domain.GlobalTimelinePost{}}
		}
		if posts == nil {
			return postsLoadedMsg{posts: []domain.GlobalTimelinePost{}}
		}

		return postsLoadedMsg{posts: *posts}
	}
}

// loadEngagementInfoGlobal loads the list of users who liked and boosted a post
func loadEngagementInfoGlobal(post domain.GlobalTimelinePost) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		var likers []string
		var boosters []string

		// For local posts, use NoteID
		if !post.IsRemote && post.NoteId != "" {
			if noteID, err := uuid.Parse(post.NoteId); err == nil {
				likers, _ = database.ReadLikersInfoByNoteId(noteID)
				boosters, _ = database.ReadBoostersInfoByNoteId(noteID)
			}
		} else if post.ObjectURI != "" {
			// For remote posts, use ObjectURI (ActivityPub canonical ID)
			likers, _ = database.ReadLikersInfoByObjectURI(post.ObjectURI)
			boosters, _ = database.ReadBoostersInfoByObjectURI(post.ObjectURI)
		}

		return engagementInfoMsg{
			likers:   likers,
			boosters: boosters,
		}
	}
}

func max(a, b int) int {
	if a > b {
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
