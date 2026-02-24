package myposts

import (
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
	timeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DIM))

	authorStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_USERNAME)).
			Bold(true)

	contentStyle = lipgloss.NewStyle().
			Align(lipgloss.Left)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM)).
			Italic(true)

	confirmDeleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_ERROR)).
				Bold(true)

	// Inverted styles for selected notes (light text on dark background)
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
	Notes            []domain.Note
	Offset           int
	Selected         int // Currently selected note index
	Width            int
	Height           int
	userId           uuid.UUID
	confirmingDelete bool      // True when showing delete confirmation
	deleteTargetId   uuid.UUID // ID of note pending deletion
	LocalDomain      string    // Cached local domain for mention highlighting
	spinner          spinner.Model
	initialLoad      bool // true until first notesLoadedMsg arrives
}

func (m Model) Init() tea.Cmd {
	return loadNotes(m.userId)
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

	case common.ActivateViewMsg:
		// View is becoming active (user navigated here)
		// Reset scroll position to top when switching to this view
		m.Selected = 0
		m.Offset = 0
		m.initialLoad = true
		m.confirmingDelete = false
		m.deleteTargetId = uuid.Nil
		return m, tea.Batch(loadNotes(m.userId), m.spinner.Tick)

	case common.SessionState:
		// Handle UpdateNoteList to refresh when notes are created/updated/liked
		if msg == common.UpdateNoteList {
			return m, loadNotes(m.userId)
		}
		return m, nil

	case notesLoadedMsg:
		m.initialLoad = false
		m.Notes = msg.notes
		// Restore selection after reload, but make sure it's within bounds
		if m.Selected >= len(m.Notes) {
			m.Selected = max(0, len(m.Notes)-1)
		}
		// Keep Offset in sync - Selected should always be at top of visible area
		m.Offset = m.Selected
		return m, nil

	case tea.KeyPressMsg:
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
		case "l":
			// Like/unlike selected note
			if len(m.Notes) > 0 && m.Selected < len(m.Notes) {
				selectedNote := m.Notes[m.Selected]
				noteURI := selectedNote.ObjectURI
				// For local notes without ObjectURI, use local: prefix
				if noteURI == "" {
					noteURI = "local:" + selectedNote.Id.String()
				}
				return m, func() tea.Msg {
					return common.LikeNoteMsg{
						NoteURI: noteURI,
						NoteID:  selectedNote.Id,
						IsLocal: true, // myposts only shows local notes
					}
				}
			}
		case "b":
			// Boost/unboost selected note
			if len(m.Notes) > 0 && m.Selected < len(m.Notes) {
				selectedNote := m.Notes[m.Selected]
				noteURI := selectedNote.ObjectURI
				// For local notes without ObjectURI, use local: prefix
				if noteURI == "" {
					noteURI = "local:" + selectedNote.Id.String()
				}
				return m, func() tea.Msg {
					return common.BoostNoteMsg{
						NoteURI: noteURI,
						NoteID:  selectedNote.Id,
						IsLocal: true, // myposts only shows local notes
					}
				}
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

func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("my posts (%d notes)", len(m.Notes))))
	s.WriteString("\n\n")

	if m.initialLoad {
		s.WriteString(m.spinner.View() + " Loading...")
	} else if len(m.Notes) == 0 {
		s.WriteString(emptyStyle.Render("No notes yet.\nCreate your first note!"))
	} else {
		// Calculate right panel width using layout helpers
		leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
		rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
		contentWidth := common.CalculateContentWidth(rightPanelWidth, 2) // 2 padding on each side

		itemsPerPage := common.DefaultItemsPerPage
		start := m.Offset
		end := min(start+itemsPerPage, len(m.Notes))

		for i := start; i < end; i++ {
			note := m.Notes[i]

			// Format timestamp with edited indicator
			timeStr := formatTime(note.CreatedAt)
			if note.EditedAt != nil {
				timeStr += " (edited)"
			}

			// Format engagement stats
			engagementStr := ""
			if note.LikeCount > 0 {
				engagementStr = fmt.Sprintf(" · ⭐ %d", note.LikeCount)
			}
			if note.BoostCount > 0 {
				engagementStr += fmt.Sprintf(" · 🔁 %d", note.BoostCount)
			}

			// Unescape HTML entities, convert Markdown links and raw URLs to OSC 8 hyperlinks, and highlight hashtags and mentions
			unescapedMessage := util.UnescapeHTML(note.Message)
			messageWithLinks := util.MarkdownLinksToTerminal(unescapedMessage)
			messageWithLinks = util.LinkifyRawURLsTerminal(messageWithLinks)
			messageWithLinksAndHashtags := util.HighlightHashtagsTerminal(messageWithLinks)
			messageWithLinksAndHashtags = util.HighlightMentionsTerminal(messageWithLinksAndHashtags, m.LocalDomain)

			// Apply selection highlighting - full width box with proper spacing
			if i == m.Selected {
				// Create a style that fills the full width
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color(common.COLOR_ACCENT)).
					Width(contentWidth)

				// Render each line with the background and inverted text colors
				timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr + engagementStr))
				authorFormatted := selectedBg.Render(selectedAuthorStyle.Render("@" + note.CreatedBy))
				contentFormatted := selectedBg.Render(selectedContentStyle.Render(messageWithLinksAndHashtags))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			} else {
				// Apply same width to unselected items for consistent wrapping
				unselectedStyle := lipgloss.NewStyle().
					Width(contentWidth)

				timeFormatted := unselectedStyle.Render(timeStyle.Render(timeStr + engagementStr))
				authorFormatted := unselectedStyle.Render(authorStyle.Render("@" + note.CreatedBy))
				contentFormatted := unselectedStyle.Render(contentStyle.Render(messageWithLinksAndHashtags))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			}

			s.WriteString("\n\n")

			// Show delete confirmation below selected note
			if m.confirmingDelete && i == m.Selected && m.deleteTargetId == note.Id {
				s.WriteString(confirmDeleteStyle.Render("Delete this note? Press y to confirm, n to cancel"))
				s.WriteString("\n\n")
			}
		}
	}

	return tea.NewView(s.String())
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

func NewPager(userId uuid.UUID, width int, height int, localDomain string) Model {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_SECONDARY))
	return Model{
		Notes:            []domain.Note{},
		Offset:           0,
		Selected:         0,
		Width:            width,
		Height:           height,
		userId:           userId,
		confirmingDelete: false,
		deleteTargetId:   uuid.Nil,
		LocalDomain:      localDomain,
		spinner:          s,
		initialLoad:      false, // Set to true on ActivateViewMsg
	}
}
