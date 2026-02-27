package hometimeline

import (
	"fmt"
	"log"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
)

type Model struct {
	AccountId          uuid.UUID
	Posts              []domain.HomePost
	Offset             int // Pagination offset
	Selected           int // Currently selected post index
	Width              int
	Height             int
	isActive           bool     // Track if this view is currently visible (prevents ticker leaks)
	tickerRunning      bool     // Track if refresh ticker is already running (prevents multiple ticker chains)
	showingURL         bool     // Track if URL is displayed instead of content for selected post
	showingEngagement  bool     // Track if engagement info (likes/boosts) is displayed
	engagementLikers   []string // List of users who liked the selected post
	engagementBoosters []string // List of users who boosted the selected post
	LocalDomain        string   // Cached local domain for mention highlighting
	spinner            spinner.Model
	initialLoad        bool // true until first postsLoadedMsg arrives
}

func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_SECONDARY))
	return Model{
		AccountId:   accountId,
		Posts:       []domain.HomePost{},
		Offset:      0,
		Selected:    0,
		Width:       width,
		Height:      height,
		isActive:    false, // Start inactive, will be activated when view is shown
		showingURL:  false, // Start in content mode
		LocalDomain: localDomain,
		spinner:     s,
		initialLoad: false, // Set to true on ActivateViewMsg
	}
}

func (m Model) Init() tea.Cmd {
	// Don't start any commands here - model starts inactive
	// ActivateViewMsg handler will load data and start ticker when view becomes active
	return nil
}

// refreshTickMsg is sent periodically to refresh the timeline
type refreshTickMsg struct{}

// tickRefresh returns a command that sends refreshTickMsg every TimelineRefreshSeconds
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
		// View is becoming inactive (user navigated away)
		m.isActive = false
		m.tickerRunning = false // Stop ticker chain
		return m, nil

	case common.ActivateViewMsg:
		// View is becoming active (user navigated here)
		m.isActive = true
		m.showingURL = false
		m.showingEngagement = false
		if len(m.Posts) > 0 {
			// Posts already loaded (view was visible but unfocused) — just start refresh ticker
			m.tickerRunning = false
			return m, tickRefresh()
		}
		// First activation or no data yet — full load with spinner
		m.tickerRunning = false
		m.initialLoad = true
		m.Selected = 0
		m.Offset = 0
		return m, tea.Batch(loadHomePosts(m.AccountId), m.spinner.Tick)

	case common.SessionState:
		// Handle UpdateNoteList to refresh when notes are created/updated
		// Always reload data when notes change, regardless of active state
		// The isActive flag only controls the ticker chain, not one-time reloads
		if msg == common.UpdateNoteList {
			return m, loadHomePosts(m.AccountId)
		}
		return m, nil

	case refreshTickMsg:
		// Only schedule next refresh if view is still active
		if m.isActive {
			return m, loadHomePosts(m.AccountId)
		}
		// View is inactive, stop the ticker chain
		return m, nil

	case postsLoadedMsg:
		m.initialLoad = false
		m.Posts = msg.posts
		// Pre-process content for terminal display (avoids re-processing on every View() render)
		for i := range m.Posts {
			m.Posts[i].ProcessedContent = processPostContent(m.Posts[i], m.LocalDomain)
		}
		// Keep selection within bounds after reload
		if m.Selected >= len(m.Posts) {
			m.Selected = max(0, len(m.Posts)-1)
		}
		// Keep Offset in sync
		m.Offset = m.Selected

		// Schedule next tick AFTER data loads (only if still active AND no ticker running)
		// This prevents multiple ticker chains from UpdateNoteList reloads
		if m.isActive && !m.tickerRunning {
			m.tickerRunning = true
			return m, tickRefresh()
		}
		return m, nil

	case engagementInfoMsg:
		// Store the engagement info
		m.engagementLikers = msg.likers
		m.engagementBoosters = msg.boosters
		return m, nil

	case tea.KeyPressMsg:
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
			// Toggle between showing content and URL (only for posts with valid HTTP/HTTPS URLs)
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
				replyURI := selectedPost.ObjectURI

				// For local posts without object_uri, we can still reply using the note ID
				// The writenote component will handle constructing the proper URI
				if replyURI == "" && selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
					// Use a placeholder URI that writenote can resolve
					replyURI = "local:" + selectedPost.NoteID.String()
				}

				if replyURI != "" {
					preview := selectedPost.Content
					if idx := strings.Index(preview, "\n"); idx > 0 {
						preview = preview[:idx]
					}
					return m, func() tea.Msg {
						return common.ReplyToNoteMsg{
							NoteURI: replyURI,
							Author:  selectedPost.Author,
							Preview: preview,
						}
					}
				}
			}
		case "enter":
			// Open thread view for selected post (only if it has replies)
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// Skip if no replies
				if selectedPost.ReplyCount == 0 {
					return m, nil
				}
				// For local posts, we can use NoteID even without ObjectURI
				if selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
					noteURI := selectedPost.ObjectURI
					if noteURI == "" {
						// Use local: prefix for local notes without ObjectURI
						noteURI = "local:" + selectedPost.NoteID.String()
					}
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   noteURI,
							NoteID:    selectedPost.NoteID,
							Author:    selectedPost.Author,
							Content:   selectedPost.Content,
							CreatedAt: selectedPost.Time,
							IsLocal:   true,
						}
					}
				} else if selectedPost.ObjectURI != "" {
					// Remote posts must have ObjectURI
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   selectedPost.ObjectURI,
							NoteID:    selectedPost.NoteID,
							Author:    selectedPost.Author,
							Content:   selectedPost.Content,
							CreatedAt: selectedPost.Time,
							IsLocal:   selectedPost.IsLocal,
						}
					}
				}
			}
		case "l":
			// Like/unlike the selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				noteURI := selectedPost.ObjectURI
				// For local posts without ObjectURI, use local: prefix
				if noteURI == "" && selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
					noteURI = "local:" + selectedPost.NoteID.String()
				}
				if noteURI != "" || selectedPost.NoteID != uuid.Nil {
					return m, func() tea.Msg {
						return common.LikeNoteMsg{
							NoteURI: noteURI,
							NoteID:  selectedPost.NoteID,
							IsLocal: selectedPost.IsLocal,
						}
					}
				}
			}
		case "b":
			// Boost/unboost the selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				noteURI := selectedPost.ObjectURI
				// For local posts without ObjectURI, use local: prefix
				if noteURI == "" && selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
					noteURI = "local:" + selectedPost.NoteID.String()
				}
				if noteURI != "" || selectedPost.NoteID != uuid.Nil {
					return m, func() tea.Msg {
						return common.BoostNoteMsg{
							NoteURI: noteURI,
							NoteID:  selectedPost.NoteID,
							IsLocal: selectedPost.IsLocal,
						}
					}
				}
			}
		case "i":
			// Toggle engagement info display (who liked/boosted)
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// Only show engagement if there are likes or boosts
				if selectedPost.LikeCount > 0 || selectedPost.BoostCount > 0 {
					m.showingEngagement = !m.showingEngagement
					// If showing engagement, fetch the data
					if m.showingEngagement {
						return m, loadEngagementInfo(selectedPost)
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("home (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if m.initialLoad {
		s.WriteString(m.spinner.View() + " Loading...")
	} else if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No posts yet.\nFollow some accounts to see their posts here!"))
	} else {
		// Calculate right panel width using layout helpers
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

			// Format boost indicator if this is a boosted post
			boostedByLine := ""
			if post.BoostedBy != "" {
				boostedByLine = fmt.Sprintf("🔁 %s boosted", post.BoostedBy)
			}

			// Apply selection highlighting
			if i == m.Selected {
				// Create a style that fills the full width (same approach as myposts)
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
							if idx < 10 { // Limit to first 10
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
							if idx < 10 { // Limit to first 10
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
					// Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
					displayURL := post.ObjectURL
					if displayURL == "" {
						displayURL = post.ObjectURI
					}
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
				} else {
					// Use pre-processed content (cached in postsLoadedMsg), add URL linkification for selected post
					displayContent := post.ProcessedContent
					if displayContent == "" {
						displayContent = processPostContent(post, m.LocalDomain)
					}
					highlightedContent := util.LinkifyRawURLsTerminal(displayContent)

					contentFormatted := selectedBg.Render(selectedContentStyle.Render(highlightedContent))
					s.WriteString(timeFormatted + "\n")
					if boostedByLine != "" {
						boostedByFormatted := selectedBg.Render(lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_GREY)).Render(boostedByLine))
						s.WriteString(boostedByFormatted + "\n")
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
					highlightedContent = processPostContent(post, m.LocalDomain)
				}

				// Use different author color for local vs remote
				var authorFormatted string
				if post.IsLocal {
					authorFormatted = unselectedStyle.Render(authorStyle.Render(author))
				} else {
					authorFormatted = unselectedStyle.Render(remoteAuthorStyle.Render(author))
				}

				timeFormatted := unselectedStyle.Render(timeStyle.Render(timeStr))
				contentFormatted := unselectedStyle.Render(contentStyle.Render(highlightedContent))

				s.WriteString(timeFormatted + "\n")
				if boostedByLine != "" {
					boostedByFormatted := unselectedStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_GREY)).Render(boostedByLine))
					s.WriteString(boostedByFormatted + "\n")
				}
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			}

			s.WriteString("\n\n")
		}
	}

	return tea.NewView(s.String())
}

// processPostContent applies the full text processing pipeline for terminal display.
// This is called once when posts are loaded (in postsLoadedMsg) and cached in ProcessedContent.
func processPostContent(post domain.HomePost, localDomain string) string {
	return common.ProcessPostContent(post.Content, post.IsLocal, localDomain, false)
}

// postsLoadedMsg is sent when posts are loaded
type postsLoadedMsg struct {
	posts []domain.HomePost
}

// engagementInfoMsg is sent when engagement info is loaded
type engagementInfoMsg struct {
	likers   []string
	boosters []string
}

// loadHomePosts loads the unified home timeline
func loadHomePosts(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, posts := database.ReadHomeTimelinePosts(accountId, common.HomeTimelinePostLimit)
		if err != nil {
			log.Printf("Failed to load home timeline: %v", err)
			return postsLoadedMsg{posts: []domain.HomePost{}}
		}

		if posts == nil {
			return postsLoadedMsg{posts: []domain.HomePost{}}
		}

		return postsLoadedMsg{posts: *posts}
	}
}

// loadEngagementInfo loads the list of users who liked and boosted a post
func loadEngagementInfo(post domain.HomePost) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		var likers []string
		var boosters []string

		// Fetch likers
		if post.IsLocal && post.NoteID != uuid.Nil {
			likers, _ = database.ReadLikersInfoByNoteId(post.NoteID)
		} else if post.ObjectURI != "" {
			likers, _ = database.ReadLikersInfoByObjectURI(post.ObjectURI)
		}

		// Fetch boosters
		if post.IsLocal && post.NoteID != uuid.Nil {
			boosters, _ = database.ReadBoostersInfoByNoteId(post.NoteID)
		} else if post.ObjectURI != "" {
			boosters, _ = database.ReadBoostersInfoByObjectURI(post.ObjectURI)
		}

		return engagementInfoMsg{
			likers:   likers,
			boosters: boosters,
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
