package listnotes

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
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

	confirmDeleteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Inverted styles for selected notes (light text on dark background)
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
	Notes           []domain.Note
	Offset          int
	Selected        int  // Currently selected note index
	width           int
	height          int
	userId          uuid.UUID
	confirmingDelete bool      // True when showing delete confirmation
	deleteTargetId   uuid.UUID // ID of note pending deletion
}

func (m Model) Init() tea.Cmd {
	return loadNotes(m.userId)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case notesLoadedMsg:
		m.Notes = msg.notes
		// Restore selection after reload, but make sure it's within bounds
		if m.Selected >= len(m.Notes) {
			m.Selected = max(0, len(m.Notes)-1)
		}
		// Keep Offset in sync - Selected should always be at top of visible area
		m.Offset = m.Selected
		return m, nil

	case tea.KeyMsg:
		// If confirming delete, only handle y/n
		if m.confirmingDelete {
			switch msg.String() {
			case "y", "Y":
				// Delete confirmed
				noteId := m.deleteTargetId
				m.confirmingDelete = false
				m.deleteTargetId = uuid.Nil
				return m, deleteNoteCmd(noteId)
			case "n", "N", "esc":
				// Delete cancelled
				m.confirmingDelete = false
				m.deleteTargetId = uuid.Nil
			}
			return m, nil
		}

		// Normal key handling - like federated timeline
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected // Keep selected at top
			}
		case "down", "j":
			if len(m.Notes) > 0 && m.Selected < len(m.Notes)-1 {
				m.Selected++
				m.Offset = m.Selected // Keep selected at top
			}
		case "u":
			// Edit selected note
			if len(m.Notes) > 0 && m.Selected < len(m.Notes) {
				selectedNote := m.Notes[m.Selected]
				return m, func() tea.Msg {
					return common.EditNoteMsg{
						NoteId:    selectedNote.Id,
						Message:   selectedNote.Message,
						CreatedAt: selectedNote.CreatedAt,
					}
				}
			}
		case "d":
			// Delete selected note (show confirmation)
			if len(m.Notes) > 0 && m.Selected < len(m.Notes) {
				m.confirmingDelete = true
				m.deleteTargetId = m.Notes[m.Selected].Id
			}
		}
	}
	return m, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("notes list (%d notes)", len(m.Notes))))
	s.WriteString("\n\n")

	if len(m.Notes) == 0 {
		s.WriteString(emptyStyle.Render("No notes yet.\nCreate your first note!"))
	} else {
		// Calculate right panel width (same as supertui calculation)
		leftPanelWidth := m.width / 3
		rightPanelWidth := m.width - leftPanelWidth - 6 // Account for borders and margins

		itemsPerPage := 10
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Notes) {
			end = len(m.Notes)
		}

		for i := start; i < end; i++ {
			note := m.Notes[i]

			// Format timestamp with edited indicator
			timeStr := formatTime(note.CreatedAt)
			if note.EditedAt != nil {
				timeStr += " (edited)"
			}

			// Apply selection highlighting - full width box with proper spacing
			if i == m.Selected {
				// Create a style that fills the full width
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color("62")).
					Width(rightPanelWidth - 4)

				// Render each line with the background and inverted text colors
				timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr))
				authorFormatted := selectedBg.Render(selectedAuthorStyle.Render("@" + note.CreatedBy))
				contentFormatted := selectedBg.Render(selectedContentStyle.Render(truncate(note.Message, 150)))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			} else {
				timeFormatted := timeStyle.Render(timeStr)
				authorStr := authorStyle.Render("@" + note.CreatedBy)
				contentStr := contentStyle.Render(truncate(note.Message, 150))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorStr + "\n")
				s.WriteString(contentStr)
			}

			s.WriteString("\n\n")

			// Show delete confirmation below selected note
			if m.confirmingDelete && i == m.Selected && m.deleteTargetId == note.Id {
				s.WriteString(confirmDeleteStyle.Render("Delete this note? Press y to confirm, n to cancel"))
				s.WriteString("\n\n")
			}
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

// deleteNoteCmd deletes a note by ID and federates the deletion
func deleteNoteCmd(noteId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Get note details before deletion for federation
		err, note := database.ReadNoteId(noteId)
		var accountUsername string
		if err == nil && note != nil {
			accountUsername = note.CreatedBy
		}

		// Delete the note from database
		err = database.DeleteNoteById(noteId)
		if err != nil {
			log.Printf("Failed to delete note: %v", err)
			return common.DeleteNoteMsg{NoteId: noteId}
		}

		// Federate the deletion via ActivityPub (background task)
		if accountUsername != "" {
			go func() {
				// Get the account
				err, account := database.ReadAccByUsername(accountUsername)
				if err != nil {
					log.Printf("Failed to get account for delete federation: %v", err)
					return
				}

				// Get config
				conf, err := util.ReadConf()
				if err != nil {
					log.Printf("Failed to read config for delete federation: %v", err)
					return
				}

				// Only federate if ActivityPub is enabled
				if !conf.Conf.WithAp {
					return
				}

				// Send Delete activity to all followers
				if err := activitypub.SendDelete(noteId, account, conf); err != nil {
					log.Printf("Failed to federate note deletion: %v", err)
				} else {
					log.Printf("Note deletion federated successfully for %s", account.Username)
				}
			}()
		}

		return common.DeleteNoteMsg{NoteId: noteId}
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
		Notes:           []domain.Note{},
		Offset:          0,
		Selected:        0,
		width:           width,
		height:          height,
		userId:          userId,
		confirmingDelete: false,
		deleteTargetId:   uuid.Nil,
	}
}
