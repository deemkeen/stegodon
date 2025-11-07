package localtimeline

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"log"
)

var (
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
	Posts  []domain.Note
	Width  int
	Height int
}

func InitialModel(width, height int) Model {
	return Model{
		Posts:  []domain.Note{},
		Width:  width,
		Height: height,
	}
}

func (m Model) Init() tea.Cmd {
	return loadLocalPosts()
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

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("local timeline (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No local posts yet.\nCreate some notes or invite others to join!"))
	} else {
		displayCount := min(len(m.Posts), 10) // Show max 10 posts
		for i := 0; i < displayCount; i++ {
			post := m.Posts[i]

			postContent := fmt.Sprintf("%s\n%s\n%s",
				authorStyle.Render("@"+post.CreatedBy),
				contentStyle.Render(truncate(post.Message, 80)),
				timeStyle.Render(formatTime(post.CreatedAt)),
			)

			s.WriteString(postStyle.Render(postContent))
			s.WriteString("\n")
		}

		if len(m.Posts) > 10 {
			s.WriteString(emptyStyle.Render(fmt.Sprintf("... and %d more posts", len(m.Posts)-10)))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(common.HelpStyle.Render("tab: switch view • shift+tab: prev view • ctrl-c: exit"))

	return s.String()
}

// postsLoadedMsg is sent when posts are loaded
type postsLoadedMsg struct {
	posts []domain.Note
}

// loadLocalPosts loads recent posts from all local users
func loadLocalPosts() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, notes := database.ReadLocalTimelineNotes(50)
		if err != nil {
			log.Printf("Failed to load local timeline: %v", err)
			return postsLoadedMsg{posts: []domain.Note{}}
		}

		if notes == nil {
			return postsLoadedMsg{posts: []domain.Note{}}
		}

		return postsLoadedMsg{posts: *notes}
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
