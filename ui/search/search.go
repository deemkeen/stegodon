package search

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
)

const maxSearchResults = 20

var (
	searchBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_WHITE)).
			Bold(true)

	resultAuthorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_USERNAME)).
				Bold(true)

	resultRemoteAuthorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_SECONDARY)).
				Bold(true)

	resultTimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM))

	resultContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	noResultsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM)).
			Italic(true)

	highlightStart = "\033[1m" // ANSI bold
	highlightEnd   = "\033[22m"
)

const searchDebounceMs = 200

// searchResultsMsg carries search results back from an async search
type searchResultsMsg struct {
	query   string
	results []domain.SearchResult
}

// searchDebounceMsg is sent after the debounce timer expires
type searchDebounceMsg struct {
	query string
}

// Model is the search overlay model
type Model struct {
	Active      bool
	Input       textinput.Model
	Results     []domain.SearchResult
	Selected    int
	Offset      int
	Width       int
	Height      int
	LocalDomain string
	lastQuery   string
}

// InitialModel creates a new search model
func InitialModel(localDomain string) Model {
	ti := textinput.New()
	ti.Placeholder = "search posts..."
	ti.CharLimit = 100

	return Model{
		Active:      false,
		Input:       ti,
		Results:     nil,
		Selected:    0,
		Offset:      0,
		LocalDomain: localDomain,
	}
}

// Activate enables the search overlay
func (m *Model) Activate() {
	m.Active = true
	m.Input.SetValue("")
	m.Input.Focus()
	m.Results = nil
	m.Selected = 0
	m.Offset = 0
	m.lastQuery = ""
}

// Deactivate disables the search overlay
func (m *Model) Deactivate() {
	m.Active = false
	m.Input.Blur()
	m.Results = nil
	m.lastQuery = ""
}

// Update handles messages for the search overlay
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case searchDebounceMsg:
		// Debounce timer expired — run search if query still matches
		if msg.query == m.lastQuery && m.Active {
			return m, performSearch(msg.query)
		}
		return m, nil

	case searchResultsMsg:
		// Only apply results if query still matches (prevents stale results)
		if msg.query == m.lastQuery {
			m.Results = msg.results
			m.Selected = 0
			m.Offset = 0
		}
		return m, nil

	case tea.KeyMsg:
		if !m.Active {
			return m, nil
		}

		switch msg.String() {
		case "esc":
			m.Deactivate()
			return m, nil

		case "up", "ctrl+p":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected
			}
			return m, nil

		case "down", "ctrl+n":
			if len(m.Results) > 0 && m.Selected < len(m.Results)-1 {
				m.Selected++
				m.Offset = m.Selected
			}
			return m, nil

		case "enter":
			if len(m.Results) > 0 && m.Selected < len(m.Results) {
				result := m.Results[m.Selected]
				m.Deactivate()

				return m, func() tea.Msg {
					return common.ViewThreadMsg{
						NoteURI:   result.ObjectURI,
						NoteID:    result.NoteID,
						IsLocal:   result.IsLocal,
						Author:    result.Author,
						Content:   result.Content,
						CreatedAt: result.Time,
					}
				}
			}
			return m, nil
		}

		// Forward other keys to textinput
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)

		// Check if query changed — debounce search
		currentQuery := m.Input.Value()
		if currentQuery != m.lastQuery {
			m.lastQuery = currentQuery
			if len(currentQuery) >= 2 {
				q := currentQuery
				debounceCmd := tea.Tick(time.Millisecond*searchDebounceMs, func(t time.Time) tea.Msg {
					return searchDebounceMsg{query: q}
				})
				return m, tea.Batch(cmd, debounceCmd)
			}
			// Clear results for short queries
			m.Results = nil
			m.Selected = 0
			m.Offset = 0
		}

		return m, cmd

	default:
		if !m.Active {
			return m, nil
		}
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd
	}
}

// performSearch runs the FTS5 search asynchronously
func performSearch(query string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, results := database.SearchPosts(query, maxSearchResults)
		if err != nil {
			return searchResultsMsg{query: query, results: nil}
		}
		return searchResultsMsg{query: query, results: results}
	}
}

// View renders the search overlay
func (m Model) View() string {
	if !m.Active {
		return ""
	}

	var s strings.Builder

	// Search bar
	s.WriteString(searchBarStyle.Render("/ search"))
	s.WriteString("\n")
	s.WriteString(m.Input.View())
	s.WriteString("\n\n")

	if m.lastQuery == "" || len(m.lastQuery) < 2 {
		s.WriteString(noResultsStyle.Render("type at least 2 characters to search"))
		return s.String()
	}

	if len(m.Results) == 0 {
		s.WriteString(noResultsStyle.Render("no results found"))
		return s.String()
	}

	// Calculate layout
	leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
	rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
	contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)

	s.WriteString(resultTimeStyle.Render(fmt.Sprintf("%d results", len(m.Results))))
	s.WriteString("\n\n")

	// Render results with pagination
	itemsPerPage := m.itemsPerPage()
	start := m.Offset
	end := start + itemsPerPage
	if end > len(m.Results) {
		end = len(m.Results)
	}

	for i := start; i < end; i++ {
		result := m.Results[i]

		// Format time
		timeStr := formatTime(result.Time)

		// Format author
		author := result.Author
		if !strings.HasPrefix(author, "@") {
			author = "@" + author
		}

		// Render snippet with highlights (replace <<>> markers with ANSI bold)
		snippet := renderSnippet(result.Snippet)

		// Truncate snippet to fit content width
		if len(snippet) > contentWidth*2 {
			snippet = snippet[:contentWidth*2] + "..."
		}

		if i == m.Selected {
			// Selected item with background highlight
			bgStyle := lipgloss.NewStyle().
				Background(lipgloss.Color(common.COLOR_ACCENT)).
				Width(contentWidth)

			selectedContent := lipgloss.NewStyle().
				Background(lipgloss.Color(common.COLOR_ACCENT)).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

			s.WriteString(bgStyle.Render(selectedContent.Render(timeStr)))
			s.WriteString("\n")
			s.WriteString(bgStyle.Render(selectedContent.Bold(true).Render(author)))
			s.WriteString("\n")
			s.WriteString(bgStyle.Render(selectedContent.Bold(false).Render(snippet)))
			s.WriteString("\n\n")
		} else {
			// Unselected item
			s.WriteString(resultTimeStyle.Render(timeStr))
			s.WriteString("\n")
			if result.IsLocal {
				s.WriteString(resultAuthorStyle.Render(author))
			} else {
				s.WriteString(resultRemoteAuthorStyle.Render(author))
			}
			s.WriteString("\n")
			s.WriteString(resultContentStyle.Render(snippet))
			s.WriteString("\n\n")
		}
	}

	return s.String()
}

// itemsPerPage calculates how many results fit on screen
func (m Model) itemsPerPage() int {
	// Each result takes ~4 lines (time + author + content + blank)
	// Reserve 5 lines for search bar header
	available := m.Height - 5
	items := available / 4
	if items < 3 {
		return 3
	}
	return items
}

// renderSnippet converts FTS5 <<>> highlight markers to ANSI bold
func renderSnippet(snippet string) string {
	snippet = strings.ReplaceAll(snippet, "<<", highlightStart)
	snippet = strings.ReplaceAll(snippet, ">>", highlightEnd)
	return snippet
}

// formatTime formats a timestamp to a relative time string
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
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
