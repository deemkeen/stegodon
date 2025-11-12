package timeline

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/ui/common"
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

	// Inverted styles for selected posts
	selectedTimeStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color("255")) // White

	selectedAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color("255")). // White
				Bold(true)

	selectedContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color("255")) // White
)

type Model struct {
	Posts    []FederatedPost
	Offset   int // Pagination offset
	Selected int // Currently selected post index
	Width    int
	Height   int
}

type FederatedPost struct {
	Actor     string
	Content   string
	Time      time.Time
	ObjectURI string // URL to the original post
}

func InitialModel(width, height int) Model {
	return Model{
		Posts:    []FederatedPost{},
		Offset:   0,
		Selected: 0,
		Width:    width,
		Height:   height,
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
		// Keep selection within bounds after reload
		if m.Selected >= len(m.Posts) {
			m.Selected = max(0, len(m.Posts)-1)
		}
		// Keep Offset in sync
		m.Offset = m.Selected
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected // Keep selected at top
			}
		case "down", "j":
			if len(m.Posts) > 0 && m.Selected < len(m.Posts)-1 {
				m.Selected++
				m.Offset = m.Selected // Keep selected at top
			}
		case "o":
			// Open selected post URL in default browser
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				if selectedPost.ObjectURI != "" {
					return m, openURLCmd(selectedPost.ObjectURI)
				}
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
		// Calculate right panel width for selection background
		leftPanelWidth := m.Width / 3
		rightPanelWidth := m.Width - leftPanelWidth - 6

		itemsPerPage := 5
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			// Format timestamp
			timeStr := formatTime(post.Time)

			// Apply selection highlighting - full width box with inverted colors
			if i == m.Selected {
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color("62")).
					Width(rightPanelWidth - 4)

				timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr))
				authorFormatted := selectedBg.Render(selectedAuthorStyle.Render(post.Actor))
				contentFormatted := selectedBg.Render(selectedContentStyle.Render(truncate(post.Content, 150)))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			} else {
				timeFormatted := timeStyle.Render(timeStr)
				authorStr := authorStyle.Render(post.Actor)
				contentStr := contentStyle.Render(truncate(post.Content, 150))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorStr + "\n")
				s.WriteString(contentStr)
			}

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
					ID      string `json:"id"`
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

			// Use ObjectURI from activity, or extract from raw JSON if empty
			objectURI := activity.ObjectURI
			if objectURI == "" && activityWrapper.Object.ID != "" {
				objectURI = activityWrapper.Object.ID
			}

			posts = append(posts, FederatedPost{
				Actor:     handle,
				Content:   cleanContent,
				Time:      activity.CreatedAt,
				ObjectURI: objectURI,
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// openURLCmd opens a URL in the default browser
func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd

		// Determine command based on OS
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			log.Printf("Unsupported OS for opening URLs: %s", runtime.GOOS)
			return nil
		}

		log.Printf("Opening URL in browser: %s (OS: %s)", url, runtime.GOOS)
		err := cmd.Start()
		if err != nil {
			log.Printf("Failed to open URL: %v", err)
		} else {
			log.Printf("Successfully opened URL: %s", url)
		}
		return nil
	}
}
