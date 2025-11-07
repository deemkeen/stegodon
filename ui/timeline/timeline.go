package timeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"log"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			MarginBottom(1)

	postStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			MarginBottom(1)

	authorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	contentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Faint(true)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

type Model struct {
	Posts  []FederatedPost
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
		Width:  width,
		Height: height,
	}
}

func (m Model) Init() tea.Cmd {
	return loadFederatedPosts()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case postsLoadedMsg:
		m.Posts = msg.posts
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render(fmt.Sprintf("Federated Timeline (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No federated posts yet.\nFollow some accounts to see their posts here!"))
	} else {
		displayCount := min(len(m.Posts), 5) // Show max 5 posts
		for i := 0; i < displayCount; i++ {
			post := m.Posts[i]

			postContent := fmt.Sprintf("%s\n%s\n%s",
				authorStyle.Render(post.Actor),
				contentStyle.Render(truncate(post.Content, 80)),
				timeStyle.Render(formatTime(post.Time)),
			)

			s.WriteString(postStyle.Render(postContent))
			s.WriteString("\n")
		}

		if len(m.Posts) > 5 {
			s.WriteString(emptyStyle.Render(fmt.Sprintf("... and %d more posts", len(m.Posts)-5)))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Faint(true).Render("tab: switch view • ctrl-c: exit"))

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
			var create struct {
				Object struct {
					Content string `json:"content"`
				} `json:"object"`
			}

			if err := json.Unmarshal([]byte(activity.RawJSON), &create); err != nil {
				log.Printf("Failed to parse activity JSON: %v", err)
				continue
			}

			posts = append(posts, FederatedPost{
				Actor:   activity.ActorURI,
				Content: create.Object.Content,
				Time:    activity.CreatedAt,
			})
		}

		return postsLoadedMsg{posts: posts}
	}
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
