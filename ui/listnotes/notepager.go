package listnotes

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
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
	Notes  []domain.Note
	Offset int
	width  int
	height int
	userId uuid.UUID
}

func (m Model) Init() tea.Cmd {
	return loadNotes(m.userId)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case notesLoadedMsg:
		m.Notes = msg.notes
		m.Offset = 0
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "left":
			if m.Offset > 0 {
				m.Offset--
			}
		case "down", "j", "right":
			if len(m.Notes) > 0 && m.Offset < len(m.Notes)-1 {
				m.Offset++
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("notes list (%d notes)", len(m.Notes))))
	s.WriteString("\n\n")

	if len(m.Notes) == 0 {
		s.WriteString(emptyStyle.Render("No notes yet.\nCreate your first note!"))
	} else {
		itemsPerPage := 10
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Notes) {
			end = len(m.Notes)
		}

		for i := start; i < end; i++ {
			note := m.Notes[i]

			// Render in vertical layout like timeline
			timeStr := timeStyle.Render(formatTime(note.CreatedAt))
			authorStr := authorStyle.Render("@" + note.CreatedBy)
			contentStr := contentStyle.Render(truncate(note.Message, 150))

			noteContent := lipgloss.JoinVertical(lipgloss.Left, timeStr, authorStr, contentStr)
			s.WriteString(noteContent)
			s.WriteString("\n\n")
		}
	}

	return s.String()
}

// notesLoadedMsg is sent when notes are loaded
type notesLoadedMsg struct {
	notes []domain.Note
}

// loadNotes loads notes for the given user
func loadNotes(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, notes := database.ReadNotesByUserId(userId)
		if err != nil {
			log.Printf("Failed to load notes: %v", err)
			return notesLoadedMsg{notes: []domain.Note{}}
		}

		if notes == nil {
			return notesLoadedMsg{notes: []domain.Note{}}
		}

		return notesLoadedMsg{notes: *notes}
	}
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func NewPager(userId uuid.UUID, width int, height int) Model {
	return Model{
		Notes:  []domain.Note{},
		Offset: 0,
		width:  width,
		height: height,
		userId: userId,
	}
}
