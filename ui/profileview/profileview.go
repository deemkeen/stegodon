package profileview

import (
	"fmt"
	"log"
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
	displayNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_USERNAME)).
				Bold(true)

	handleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SECONDARY))

	bioStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_WHITE))

	metadataStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM))

	followBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_SUCCESS))

	notFollowBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_DIM))

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM))

	postTimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM))

	postAuthorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_USERNAME)).
			Bold(true)

	postContentStyle = lipgloss.NewStyle()

	selectedPostTimeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedPostAuthorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_WHITE)).
				Bold(true)

	selectedPostContentStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color(common.COLOR_WHITE))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM)).
			Italic(true)
)

const (
	maxProfilePosts = 10
	avatarCols      = 12 // character width (= pixel width)
	avatarRows      = 6  // character height (= pixel height / 2, so 12px tall)
)

type Model struct {
	AccountId      uuid.UUID
	ProfileUser    *domain.Account
	Posts          []domain.Note
	IsFollowing    bool
	Selected       int
	Offset         int
	Width          int
	Height         int
	loading        bool
	Status         string
	Error          string
	LocalDomain    string
	AvatarRendered string
}

func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
	return Model{
		AccountId:   accountId,
		ProfileUser: nil,
		Posts:       []domain.Note{},
		IsFollowing: false,
		Selected:    0,
		Offset:      0,
		Width:       width,
		Height:      height,
		loading:     false,
		Status:      "",
		Error:       "",
		LocalDomain: localDomain,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// profileLoadedMsg is sent when profile data is loaded
type profileLoadedMsg struct {
	account     *domain.Account
	posts       []domain.Note
	isFollowing bool
	avatarStr   string
	err         error
}

// clearStatusMsg is sent after a delay to clear status messages
type clearStatusMsg struct{}

// followToggledMsg is sent after follow/unfollow completes
type followToggledMsg struct {
	isFollowing bool
	username    string
	err         error
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.ViewProfileMsg:
		m.loading = true
		m.Error = ""
		m.Status = ""
		m.Selected = 0
		m.Offset = 0
		m.ProfileUser = nil
		m.Posts = nil
		m.AvatarRendered = ""
		return m, loadProfile(m.AccountId, msg.Username)

	case profileLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.Error = msg.err.Error()
			return m, nil
		}
		m.ProfileUser = msg.account
		m.Posts = msg.posts
		m.IsFollowing = msg.isFollowing
		m.AvatarRendered = msg.avatarStr
		m.Selected = 0
		m.Offset = 0
		return m, nil

	case followToggledMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to toggle follow: %v", msg.err)
			return m, clearStatusAfter(2 * time.Second)
		}
		m.IsFollowing = msg.isFollowing
		if msg.isFollowing {
			m.Status = fmt.Sprintf("Following @%s", msg.username)
		} else {
			m.Status = fmt.Sprintf("Unfollowed @%s", msg.username)
		}
		m.Error = ""
		return m, clearStatusAfter(2 * time.Second)

	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected
			}
		case "down", "j":
			if m.Selected < len(m.Posts)-1 {
				m.Selected++
				m.Offset = m.Selected
			}
		case "enter":
			// View thread for selected post
			if len(m.Posts) > 0 && m.Selected >= 0 && m.Selected < len(m.Posts) {
				post := m.Posts[m.Selected]
				noteURI := post.ObjectURI
				if noteURI == "" {
					noteURI = "local:" + post.Id.String()
				}
				return m, func() tea.Msg {
					return common.ViewThreadMsg{
						NoteURI:   noteURI,
						NoteID:    post.Id,
						IsLocal:   true,
						Author:    post.CreatedBy,
						Content:   post.Message,
						CreatedAt: post.CreatedAt,
					}
				}
			}
		case "f":
			// Toggle follow/unfollow
			if m.ProfileUser != nil {
				return m, toggleFollow(m.AccountId, m.ProfileUser, m.IsFollowing)
			}
		case "esc":
			return m, func() tea.Msg {
				return common.LocalUsersView
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render("profile"))
	s.WriteString("\n")

	if m.loading {
		s.WriteString(emptyStyle.Render("Loading profile..."))
		return s.String()
	}

	if m.Error != "" && m.ProfileUser == nil {
		s.WriteString(emptyStyle.Render("Error: " + m.Error))
		return s.String()
	}

	if m.ProfileUser == nil {
		s.WriteString(emptyStyle.Render("No profile to display"))
		return s.String()
	}

	// Calculate content width
	leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
	rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
	contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)

	// Profile header text
	var headerText strings.Builder

	name := m.ProfileUser.DisplayName
	if name == "" {
		name = m.ProfileUser.Username
	}
	headerText.WriteString(displayNameStyle.Render(name))
	headerText.WriteString("\n")
	headerText.WriteString(handleStyle.Render("@" + m.ProfileUser.Username))
	headerText.WriteString("\n")

	if m.ProfileUser.Summary != "" {
		headerText.WriteString("\n")
		headerText.WriteString(bioStyle.Render(m.ProfileUser.Summary))
		headerText.WriteString("\n")
	}

	// Metadata line: join date + follow status
	joinDuration := time.Since(m.ProfileUser.CreatedAt)
	var joinStr string
	if joinDuration < common.HoursPerDay*time.Hour {
		joinStr = fmt.Sprintf("joined %dh ago", int(joinDuration.Hours()))
	} else {
		joinStr = fmt.Sprintf("joined %dd ago", int(joinDuration.Hours()/common.HoursPerDay))
	}

	var followBadge string
	if m.IsFollowing {
		followBadge = followBadgeStyle.Render("following")
	} else {
		followBadge = notFollowBadgeStyle.Render("not following")
	}

	headerText.WriteString("\n")
	headerText.WriteString(metadataStyle.Render(joinStr+" · ") + followBadge)

	// Compose avatar + header text
	if m.AvatarRendered != "" {
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.AvatarRendered, "   ", headerText.String()))
	} else {
		s.WriteString(headerText.String())
	}
	s.WriteString("\n\n")

	// Separator
	sep := strings.Repeat("─", contentWidth)
	s.WriteString(separatorStyle.Render(sep))
	s.WriteString("\n")

	// Posts section
	postCount := len(m.Posts)
	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("recent posts (%d)", postCount)))
	s.WriteString("\n")

	if postCount == 0 {
		s.WriteString(emptyStyle.Render("No posts yet."))
		s.WriteString("\n")
	} else {
		start := m.Offset
		end := min(start+common.DefaultItemsPerPage, postCount)

		for i := start; i < end; i++ {
			post := m.Posts[i]
			isSelected := i == m.Selected

			// Format timestamp
			timeStr := formatTime(post.CreatedAt)

			// Format author
			author := "@" + post.CreatedBy

			// Format content
			processedContent := post.Message
			processedContent = util.TruncateContent(processedContent, common.MaxDisplayContentLength)
			processedContent = util.UnescapeHTML(processedContent)
			processedContent = util.MarkdownLinksToTerminal(processedContent)
			processedContent = util.LinkifyRawURLsTerminal(processedContent)
			highlightedContent := util.HighlightHashtagsTerminal(processedContent)
			highlightedContent = util.HighlightMentionsTerminal(highlightedContent, m.LocalDomain)

			if isSelected {
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color(common.COLOR_ACCENT)).
					Width(contentWidth)

				timeFormatted := selectedBg.Render(selectedPostTimeStyle.Render(timeStr))
				authorFormatted := selectedBg.Render(selectedPostAuthorStyle.Render(author))
				contentFormatted := selectedBg.Render(selectedPostContentStyle.Render(highlightedContent))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			} else {
				unselectedStyle := lipgloss.NewStyle().Width(contentWidth)

				timeFormatted := unselectedStyle.Render(postTimeStyle.Render(timeStr))
				authorFormatted := unselectedStyle.Render(postAuthorStyle.Render(author))
				contentFormatted := unselectedStyle.Render(postContentStyle.Render(highlightedContent))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			}
			s.WriteString("\n\n")
		}

		// Pagination info
		if postCount > common.DefaultItemsPerPage {
			paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, postCount)
			s.WriteString(common.ListBadgeStyle.Render(paginationText))
			s.WriteString("\n")
		}
	}

	if m.Status != "" {
		s.WriteString(common.ListStatusStyle.Render(m.Status))
		s.WriteString("\n")
	}

	if m.Error != "" {
		s.WriteString(common.ListErrorStyle.Render(m.Error))
		s.WriteString("\n")
	}

	return s.String()
}

// loadProfile fetches the profile user's account, their top-level posts, and follow status
func loadProfile(viewerAccountId uuid.UUID, username string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Load account
		err, account := database.ReadAccByUsername(username)
		if err != nil || account == nil {
			return profileLoadedMsg{err: fmt.Errorf("user not found")}
		}

		// Load notes
		err, allNotes := database.ReadNotesByUserId(account.Id)
		if err != nil {
			log.Printf("Failed to load notes for profile %s: %v", username, err)
			return profileLoadedMsg{account: account, posts: []domain.Note{}, isFollowing: false}
		}

		// Filter out replies (only show top-level posts) and take first maxProfilePosts
		var topLevelPosts []domain.Note
		if allNotes != nil {
			for _, note := range *allNotes {
				if note.InReplyToURI == "" {
					topLevelPosts = append(topLevelPosts, note)
					if len(topLevelPosts) >= maxProfilePosts {
						break
					}
				}
			}
		}

		// Check follow status
		isFollowing, err := database.IsFollowingLocal(viewerAccountId, account.Id)
		if err != nil {
			log.Printf("Failed to check follow status: %v", err)
			isFollowing = false
		}

		// Render avatar: custom if available, otherwise default logo
		var avatarStr string
		img := util.LoadAvatarImage(account.AvatarURL)
		if img == nil {
			img = util.LoadDefaultAvatarImage()
		}
		if img != nil {
			avatarStr = util.RenderImageToHalfBlocks(img, avatarCols, avatarRows)
		}

		return profileLoadedMsg{
			account:     account,
			posts:       topLevelPosts,
			isFollowing: isFollowing,
			avatarStr:   avatarStr,
		}
	}
}

// toggleFollow follows or unfollows the profile user
func toggleFollow(viewerAccountId uuid.UUID, profileUser *domain.Account, isFollowing bool) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		if isFollowing {
			err := database.DeleteLocalFollow(viewerAccountId, profileUser.Id)
			if err != nil {
				return followToggledMsg{err: err}
			}
			return followToggledMsg{isFollowing: false, username: profileUser.Username}
		}

		err := database.CreateLocalFollow(viewerAccountId, profileUser.Id)
		if err != nil {
			return followToggledMsg{err: err}
		}

		// Create notification for the followed user
		err, follower := database.ReadAccById(viewerAccountId)
		if err == nil && follower != nil {
			notification := &domain.Notification{
				Id:               uuid.New(),
				AccountId:        profileUser.Id,
				NotificationType: domain.NotificationFollow,
				ActorId:          follower.Id,
				ActorUsername:     follower.Username,
				ActorDomain:      "",
				Read:             false,
				CreatedAt:        time.Now(),
			}
			if err := database.CreateNotification(notification); err != nil {
				log.Printf("Failed to create follow notification: %v", err)
			}
		}

		return followToggledMsg{isFollowing: true, username: profileUser.Username}
	}
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
