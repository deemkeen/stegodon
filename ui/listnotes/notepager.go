package listnotes

import (
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"log"
	"strings"
	"time"
)

type Model struct {
	items     []string
	paginator paginator.Model
	width     int
	height    int
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	m.paginator, cmd = m.paginator.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(common.COLOR_MAGENTA)).
		PaddingBottom(1).Render("notes list"))

	start, end := m.paginator.GetSliceBounds(len(m.items))
	for _, item := range m.items[start:end] {
		b.WriteString(item + "\n\n")
	}
	if m.paginator.TotalPages > 1 {
		pages := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_MAGENTA)).Align(lipgloss.Bottom).
			Render("\n\npages:  " + m.paginator.View() + "\n\n")
		b.WriteString(pages)
	}

	return lipgloss.NewStyle().
		Height(common.DefaultListHeight(m.height)).
		Width(common.DefaultListWidth(m.width)).Margin(1).
		SetString(b.String()).String()

}

type item struct {
	createdBy string
	message   string
	createdAt time.Time
}

func LoadItems(userId uuid.UUID) []item {

	var items []item
	err, notes := db.GetDB().ReadNotesByUserId(userId)
	if err != nil {
		log.Fatalln("Could not get notes!", err)
		return nil
	}

	for _, note := range *notes {
		it := item{note.CreatedBy, note.Message, note.CreatedAt}
		items = append(items, it)
	}

	return items
}

func renderNote(item item, width int) string {
	created := lipgloss.NewStyle().
		Align(lipgloss.Left).
		Foreground(lipgloss.Color(common.COLOR_PURPLE)).
		PaddingTop(2).
		PaddingBottom(1).
		MaxWidth(100).
		Width(width).
		SetString(item.createdAt.Format(util.DateTimeFormat()))
	message := lipgloss.NewStyle().
		Align(lipgloss.Left).
		MaxWidth(150).
		Width(75).
		SetString(item.message)
	return lipgloss.JoinVertical(lipgloss.Left, created.String(), message.String())
}

func NewPager(userId uuid.UUID, width int, height int) Model {

	width = common.DefaultListWidth(width)
	height = common.DefaultWindowHeight(height)

	loadedItems := LoadItems(userId)
	var styledItems []string

	for _, item := range loadedItems {
		var styledItem = renderNote(item, width)
		styledItems = append(styledItems, styledItem)
	}

	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = 4
	p.SetTotalPages(len(loadedItems))

	return Model{
		paginator: p,
		items:     styledItems,
		width:     width,
		height:    height,
	}
}
