package timeline

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/ui/common"
	"log"
)

var (
	timeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_PURPLE))

	authorStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_LIGHTBLUE)).
			Bold(true)

	contentStyle = lipgloss.NewStyle().
			Align(lipgloss.Left)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

type Model struct {
	Posts  []FederatedPost
	Offset int // Pagination offset
	Width  int
	Height int
}

type FederatedPost struct {
	Actor   string
	Content string
	Time    time.Time
}

func InitialModel(width, height int) Model {
	return Model{
		Posts:  []FederatedPost{},
		Offset: 0,
		Width:  width,
		Height: height,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadFederatedPosts(),
		tickRefresh(),
	)
}

// refreshTickMsg is sent periodically to refresh the timeline
type refreshTickMsg struct{}

// tickRefresh returns a command that sends refreshTickMsg every 10 seconds
func tickRefresh() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshTickMsg:
		// Reload posts and schedule next refresh
		return m, tea.Batch(loadFederatedPosts(), tickRefresh())

	case postsLoadedMsg:
		m.Posts = msg.posts
		// Don't reset offset on auto-refresh, only on manual Init
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "left":
			if m.Offset > 0 {
				m.Offset--
			}
		case "down", "j", "right":
			// Allow scrolling if we have any posts at all
			if len(m.Posts) > 0 && m.Offset < len(m.Posts)-1 {
				m.Offset++
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("federated timeline (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No federated posts yet.\nFollow some accounts to see their posts here!"))
	} else {
		itemsPerPage := 5
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			// Render in vertical layout like notes list
			timeStr := timeStyle.Render(formatTime(post.Time))
			authorStr := authorStyle.Render(post.Actor)
			contentStr := contentStyle.Render(truncate(post.Content, 150))

			postContent := lipgloss.JoinVertical(lipgloss.Left, timeStr, authorStr, contentStr)
			s.WriteString(postContent)
			s.WriteString("\n\n")
		}
	}

	return s.String()
}

// postsLoadedMsg is sent when posts are loaded
type postsLoadedMsg struct {
	posts []FederatedPost
}

// loadFederatedPosts loads recent federated activities
func loadFederatedPosts() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, activities := database.ReadFederatedActivities(20)
		if err != nil {
			log.Printf("Failed to load federated activities: %v", err)
			return postsLoadedMsg{posts: []FederatedPost{}}
		}

		if activities == nil {
			return postsLoadedMsg{posts: []FederatedPost{}}
		}

		// Parse activities into posts
		posts := make([]FederatedPost, 0, len(*activities))
		for _, activity := range *activities {
			// Parse the raw JSON to extract content
			// Handle both Create and Update activities (Update is stored in Create activities)
			var activityWrapper struct {
				Type   string `json:"type"`
				Object struct {
					Content string `json:"content"`
				} `json:"object"`
			}

			if err := json.Unmarshal([]byte(activity.RawJSON), &activityWrapper); err != nil {
				log.Printf("Failed to parse activity JSON: %v", err)
				continue
			}

			// Skip if content is empty
			if activityWrapper.Object.Content == "" {
				continue
			}

			// Strip HTML tags from content
			cleanContent := stripHTMLTags(activityWrapper.Object.Content)

			// Get remote account to format handle as username@domain
			handle := activity.ActorURI // fallback to URI
			err, remoteAcc := database.ReadRemoteAccountByActorURI(activity.ActorURI)
			if err == nil && remoteAcc != nil {
				handle = "@" + remoteAcc.Username + "@" + remoteAcc.Domain
			}

			posts = append(posts, FederatedPost{
				Actor:   handle,
				Content: cleanContent,
				Time:    activity.CreatedAt,
			})
		}

		return postsLoadedMsg{posts: posts}
	}
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// stripHTMLTags removes HTML tags from a string and converts common HTML entities
func stripHTMLTags(html string) string {
	// Remove all HTML tags
	text := htmlTagRegex.ReplaceAllString(html, "")

	// Convert common HTML entities
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Clean up extra whitespace
	text = strings.TrimSpace(text)

	return text
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}
