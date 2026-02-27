package profileview

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deemkeen/stegodon/activitypub"
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

	pendingBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_WARNING))

	remoteBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_SECONDARY))

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

// remotePost represents a post from a remote user (extracted from activity raw_json)
type remotePost struct {
	ObjectURI  string
	ObjectURL  string
	Content    string
	CreatedAt  time.Time
	Author     string
	Domain     string
	LikeCount  int
	BoostCount int
}

type Model struct {
	AccountId         uuid.UUID
	ProfileUser       *domain.Account       // Local user data
	RemoteProfileUser *domain.RemoteAccount // Remote user data
	IsRemoteProfile   bool                  // Which type is active
	Posts             []domain.Note         // Local user posts
	RemotePosts       []remotePost          // Remote user posts
	IsFollowing       bool
	FollowPending     bool // For pending remote follows
	Selected          int
	Offset            int
	Width             int
	Height            int
	loading           bool
	Status            string
	Error             string
	LocalDomain       string
	AvatarRendered    string
	ReturnView        common.SessionState // Where to return on Esc
	spinner           spinner.Model
}

func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_SECONDARY))
	return Model{
		AccountId:         accountId,
		ProfileUser:       nil,
		RemoteProfileUser: nil,
		IsRemoteProfile:   false,
		Posts:             []domain.Note{},
		RemotePosts:       []remotePost{},
		IsFollowing:       false,
		FollowPending:     false,
		Selected:          0,
		Offset:            0,
		Width:             width,
		Height:            height,
		loading:           false,
		Status:            "",
		Error:             "",
		LocalDomain:       localDomain,
		ReturnView:        common.LocalUsersView,
		spinner:           s,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// profileLoadedMsg is sent when local profile data is loaded
type profileLoadedMsg struct {
	account     *domain.Account
	posts       []domain.Note
	isFollowing bool
	avatarStr   string
	err         error
}

// remoteProfileLoadedMsg is sent when remote profile data is loaded
type remoteProfileLoadedMsg struct {
	account       *domain.RemoteAccount
	posts         []remotePost
	isFollowing   bool
	followPending bool
	avatarStr     string
	err           error
}

// clearStatusMsg is sent after a delay to clear status messages
type clearStatusMsg struct{}

// followToggledMsg is sent after follow/unfollow completes
type followToggledMsg struct {
	isFollowing   bool
	followPending bool
	username      string
	err           error
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case common.ViewProfileMsg:
		m.loading = true
		m.Error = ""
		m.Status = ""
		m.Selected = 0
		m.Offset = 0
		m.ProfileUser = nil
		m.RemoteProfileUser = nil
		m.Posts = nil
		m.RemotePosts = nil
		m.AvatarRendered = ""
		m.IsRemoteProfile = msg.IsRemote
		m.FollowPending = false

		if msg.IsRemote {
			return m, tea.Batch(loadRemoteProfile(m.AccountId, msg.ActorURI, msg.Username, msg.Domain), m.spinner.Tick)
		}
		return m, tea.Batch(loadProfile(m.AccountId, msg.Username), m.spinner.Tick)

	case profileLoadedMsg:
		m.loading = false
		m.IsRemoteProfile = false
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

	case remoteProfileLoadedMsg:
		m.loading = false
		m.IsRemoteProfile = true
		if msg.err != nil {
			m.Error = msg.err.Error()
			return m, nil
		}
		m.RemoteProfileUser = msg.account
		m.RemotePosts = msg.posts
		m.IsFollowing = msg.isFollowing
		m.FollowPending = msg.followPending
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
		m.FollowPending = msg.followPending
		if msg.isFollowing {
			if msg.followPending {
				m.Status = fmt.Sprintf("Follow request sent to @%s", msg.username)
			} else {
				m.Status = fmt.Sprintf("Following @%s", msg.username)
			}
		} else {
			m.Status = fmt.Sprintf("Unfollowed @%s", msg.username)
		}
		m.Error = ""
		return m, clearStatusAfter(2 * time.Second)

	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected
			}
		case "down", "j":
			postCount := m.getPostCount()
			if m.Selected < postCount-1 {
				m.Selected++
				m.Offset = m.Selected
			}
		case "enter":
			// View thread for selected post
			if m.IsRemoteProfile {
				if len(m.RemotePosts) > 0 && m.Selected >= 0 && m.Selected < len(m.RemotePosts) {
					post := m.RemotePosts[m.Selected]
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   post.ObjectURI,
							NoteID:    uuid.Nil,
							IsLocal:   false,
							Author:    fmt.Sprintf("%s@%s", post.Author, post.Domain),
							Content:   post.Content,
							CreatedAt: post.CreatedAt,
						}
					}
				}
			} else {
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
			}
		case "f":
			// Toggle follow/unfollow
			if m.IsRemoteProfile && m.RemoteProfileUser != nil {
				return m, toggleRemoteFollow(m.AccountId, m.RemoteProfileUser, m.IsFollowing)
			} else if m.ProfileUser != nil {
				return m, toggleFollow(m.AccountId, m.ProfileUser, m.IsFollowing)
			}
		case "esc":
			return m, func() tea.Msg {
				return m.ReturnView
			}
		}
	}
	return m, nil
}

func (m Model) getPostCount() int {
	if m.IsRemoteProfile {
		return len(m.RemotePosts)
	}
	return len(m.Posts)
}

func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render("profile"))
	s.WriteString("\n")

	if m.loading {
		s.WriteString(m.spinner.View() + " Loading profile...")
		return tea.NewView(s.String())
	}

	if m.Error != "" && m.ProfileUser == nil && m.RemoteProfileUser == nil {
		s.WriteString(emptyStyle.Render("Error: " + m.Error))
		return tea.NewView(s.String())
	}

	if m.ProfileUser == nil && m.RemoteProfileUser == nil {
		s.WriteString(emptyStyle.Render("No profile to display"))
		return tea.NewView(s.String())
	}

	// Calculate content width
	leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
	rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
	contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)

	if m.IsRemoteProfile {
		s.WriteString(m.viewRemoteProfile(contentWidth))
	} else {
		s.WriteString(m.viewLocalProfile(contentWidth))
	}

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

func (m Model) viewLocalProfile(contentWidth int) string {
	var s strings.Builder

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
		bio := util.SanitizeRemoteContent(m.ProfileUser.Summary)
		headerText.WriteString(bioStyle.Render(bio))
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
		textWidth := contentWidth - avatarCols - 3
		if textWidth < 20 {
			textWidth = 20
		}
		constrainedHeader := lipgloss.NewStyle().Width(textWidth).Render(headerText.String())
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.AvatarRendered, "   ", constrainedHeader))
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
			highlightedContent := common.ProcessPostContent(post.Message, true, m.LocalDomain, true)

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

	return s.String()
}

func (m Model) viewRemoteProfile(contentWidth int) string {
	var s strings.Builder

	// Profile header text
	var headerText strings.Builder

	name := m.RemoteProfileUser.DisplayName
	if name == "" {
		name = m.RemoteProfileUser.Username
	}
	headerText.WriteString(displayNameStyle.Render(name))
	headerText.WriteString("\n")
	// Show full federated handle for remote users
	headerText.WriteString(handleStyle.Render(fmt.Sprintf("@%s@%s", m.RemoteProfileUser.Username, m.RemoteProfileUser.Domain)))
	headerText.WriteString("\n")

	if m.RemoteProfileUser.Summary != "" {
		headerText.WriteString("\n")
		// Clean HTML from bio and truncate to local bio limit (200 chars)
		bio := util.StripHTMLTags(m.RemoteProfileUser.Summary)
		bio = util.UnescapeHTML(bio)
		// SanitizeRemoteContent converts complex emojis to simple ones by removing
		// ZWJ, skin tones, and variation selectors (e.g., 🧑‍💻 → 🧑💻, 👋🏽 → 👋)
		bio = util.SanitizeRemoteContent(bio)
		// Collapse newlines and multiple spaces for consistent layout
		bio = strings.ReplaceAll(bio, "\n", " ")
		bio = strings.Join(strings.Fields(bio), " ")
		bio = util.TruncateContent(bio, 200)
		headerText.WriteString(bioStyle.Render(bio))
		headerText.WriteString("\n")
	}

	// Metadata line: remote badge + follow status
	var followBadge string
	if m.IsFollowing {
		if m.FollowPending {
			followBadge = pendingBadgeStyle.Render("pending")
		} else {
			followBadge = followBadgeStyle.Render("following")
		}
	} else {
		followBadge = notFollowBadgeStyle.Render("not following")
	}

	headerText.WriteString("\n")
	headerText.WriteString(remoteBadgeStyle.Render("remote") + metadataStyle.Render(" · ") + followBadge)

	// Compose avatar + header text
	if m.AvatarRendered != "" {
		textWidth := contentWidth - avatarCols - 3
		if textWidth < 20 {
			textWidth = 20
		}
		constrainedHeader := lipgloss.NewStyle().Width(textWidth).Render(headerText.String())
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.AvatarRendered, "   ", constrainedHeader))
	} else {
		s.WriteString(headerText.String())
	}
	s.WriteString("\n\n")

	// Separator
	sep := strings.Repeat("─", contentWidth)
	s.WriteString(separatorStyle.Render(sep))
	s.WriteString("\n")

	// Posts section
	postCount := len(m.RemotePosts)
	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("recent posts (%d)", postCount)))
	s.WriteString("\n")

	if postCount == 0 {
		s.WriteString(emptyStyle.Render("No posts available."))
		s.WriteString("\n")
	} else {
		start := m.Offset
		end := min(start+common.DefaultItemsPerPage, postCount)

		for i := start; i < end; i++ {
			post := m.RemotePosts[i]
			isSelected := i == m.Selected

			// Format timestamp
			timeStr := formatTime(post.CreatedAt)

			// Format author with domain
			author := fmt.Sprintf("@%s@%s", post.Author, post.Domain)

			// Format content
			stripped := util.StripHTMLTags(post.Content)
			highlightedContent := common.ProcessPostContent(stripped, false, m.LocalDomain, true)

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

// loadRemoteProfile fetches a remote user's profile, their activities, and follow status
func loadRemoteProfile(viewerAccountId uuid.UUID, actorURI, username, domain string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Load remote account
		err, remoteAccount := database.ReadRemoteAccountByActorURI(actorURI)
		if err != nil || remoteAccount == nil {
			// Try by ID if actorURI lookup fails
			log.Printf("Failed to load remote account by URI %s: %v", actorURI, err)
			return remoteProfileLoadedMsg{err: fmt.Errorf("remote user not found")}
		}

		// Load activities (posts) from this remote user
		err, activities := database.ReadActivitiesByActorURI(actorURI, maxProfilePosts)
		var remotePosts []remotePost
		if err == nil && activities != nil {
			for _, activity := range *activities {
				post := extractPostFromActivity(&activity, remoteAccount.Username, remoteAccount.Domain)
				if post != nil {
					remotePosts = append(remotePosts, *post)
				}
			}
		}

		// Check follow status - look for a follow where we're following this remote user
		isFollowing := false
		followPending := false
		err, follow := database.ReadFollowByAccountIds(viewerAccountId, remoteAccount.Id)
		if err == nil && follow != nil {
			isFollowing = true
			followPending = !follow.Accepted
		}

		// Render avatar: fetch remote avatar or use default
		var avatarStr string
		img := util.LoadRemoteAvatarImage(remoteAccount.AvatarURL)
		if img == nil {
			img = util.LoadDefaultAvatarImage()
		}
		if img != nil {
			avatarStr = util.RenderImageToHalfBlocks(img, avatarCols, avatarRows)
		}

		return remoteProfileLoadedMsg{
			account:       remoteAccount,
			posts:         remotePosts,
			isFollowing:   isFollowing,
			followPending: followPending,
			avatarStr:     avatarStr,
		}
	}
}

// extractPostFromActivity extracts post content from an Activity's raw JSON
func extractPostFromActivity(activity *domain.Activity, username, domain string) *remotePost {
	if activity.RawJSON == "" {
		return nil
	}

	var activityData map[string]any
	if err := json.Unmarshal([]byte(activity.RawJSON), &activityData); err != nil {
		return nil
	}

	// Get the object from the activity
	objectData, ok := activityData["object"].(map[string]any)
	if !ok {
		return nil
	}

	// Extract content
	content, _ := objectData["content"].(string)
	if content == "" {
		return nil
	}

	return &remotePost{
		ObjectURI:  activity.ObjectURI,
		ObjectURL:  activity.ObjectURL,
		Content:    content,
		CreatedAt:  activity.CreatedAt,
		Author:     username,
		Domain:     domain,
		LikeCount:  activity.LikeCount,
		BoostCount: activity.BoostCount,
	}
}

// toggleFollow follows or unfollows the local profile user
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
				ActorUsername:    follower.Username,
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

// toggleRemoteFollow follows or unfollows a remote user via ActivityPub
func toggleRemoteFollow(viewerAccountId uuid.UUID, remoteUser *domain.RemoteAccount, isFollowing bool) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Get local account
		err, localAccount := database.ReadAccById(viewerAccountId)
		if err != nil {
			return followToggledMsg{err: fmt.Errorf("failed to get local account: %w", err)}
		}

		// Get config
		conf, err := util.ReadConf()
		if err != nil {
			return followToggledMsg{err: fmt.Errorf("failed to read config: %w", err)}
		}

		fullUsername := fmt.Sprintf("%s@%s", remoteUser.Username, remoteUser.Domain)

		if isFollowing {
			// Unfollow - get the existing follow record first
			err, existingFollow := database.ReadFollowByAccountIds(viewerAccountId, remoteUser.Id)
			if err != nil || existingFollow == nil {
				return followToggledMsg{err: fmt.Errorf("follow record not found")}
			}

			// Send Undo activity if AP is enabled
			if conf.Conf.WithAp {
				if err := activitypub.SendUndo(localAccount, existingFollow, remoteUser, conf); err != nil {
					log.Printf("Warning: Failed to send Undo activity: %v", err)
					// Continue with local delete even if remote notification fails
				}
			}

			// Delete from local database
			if existingFollow.URI != "" {
				err = database.DeleteFollowByURI(existingFollow.URI)
			} else {
				err = database.DeleteFollowByAccountIds(viewerAccountId, remoteUser.Id)
			}
			if err != nil {
				return followToggledMsg{err: err}
			}

			return followToggledMsg{isFollowing: false, username: fullUsername}
		}

		// Follow - send Follow activity via ActivityPub
		if !conf.Conf.WithAp {
			return followToggledMsg{err: fmt.Errorf("ActivityPub is not enabled")}
		}

		if err := activitypub.SendFollow(localAccount, remoteUser.ActorURI, conf); err != nil {
			return followToggledMsg{err: err}
		}

		// Follow request sent, it's pending until Accept is received
		return followToggledMsg{isFollowing: true, followPending: true, username: fullUsername}
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
