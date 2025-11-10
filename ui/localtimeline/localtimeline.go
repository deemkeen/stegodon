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
	Posts  []domain.Note
	Offset int // Pagination offset
	Width  int
	Height int
}

func InitialModel(width, height int) Model {
	return Model{
		Posts:  []domain.Note{},
		Offset: 0,
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
		m.Offset = 0 // Reset offset on reload
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

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("local timeline (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No local posts yet.\nCreate some notes or invite others to join!"))
	} else {
		itemsPerPage := 10
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			// Render in vertical layout like notes list
			timeStr := timeStyle.Render(formatTime(post.CreatedAt))
			authorStr := authorStyle.Render("@" + post.CreatedBy)
			contentStr := contentStyle.Render(truncate(post.Message, 150))

			postContent := lipgloss.JoinVertical(lipgloss.Left, timeStr, authorStr, contentStr)
			s.WriteString(postContent)
			s.WriteString("\n\n")
		}
	}

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
