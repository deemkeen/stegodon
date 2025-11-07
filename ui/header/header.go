package header

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
)

type Model struct {
	Width int
	Acc   *domain.Account
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	return GetHeaderStyle(m.Acc, m.Width)
}

func GetHeaderStyle(acc *domain.Account, width int) string {
	// Each lipgloss border (NormalBorder) with sides adds 2 chars to width
	// Each padding(1) adds 2 chars to width (left + right)
	// So each styled box adds 4 chars total to the content width

	// We have 4 boxes, each adding 4 chars overhead = 16 chars total
	// But we need to be more precise...

	// Calculate widths accounting for lipgloss rendering
	// Username box: content + padding(2) + border(2) = content + 4
	// At box: content + padding(2) + border(2) = content + 4
	// Version box: content + padding(2) + border(2) = content + 4
	// Created box: content + padding(2) + border(2) = content + 4

	overhead := 16 // Total for all 4 boxes
	availableWidth := width - overhead

	if availableWidth < 40 {
		availableWidth = 40
	}

	// Distribute available width
	usernameWidth := availableWidth / 6
	atWidth := 1
	versionWidth := availableWidth / 2
	createdWidth := availableWidth - usernameWidth - atWidth - versionWidth

	username := lipgloss.
		NewStyle().
		SetString(acc.Username).
		Align(lipgloss.Left).
		Background(lipgloss.Color(common.COLOR_PURPLE)).
		Padding(1).
		Height(2).
		Width(usernameWidth).
		Border(lipgloss.NormalBorder(), true, false, true, false).
		BorderForeground(lipgloss.Color(common.COLOR_MAGENTA)).
		String()

	at := lipgloss.
		NewStyle().
		SetString("@").
		Background(lipgloss.NoColor{}).
		Foreground(lipgloss.Color(common.COLOR_MAGENTA)).
		Padding(1).
		Height(2).
		Width(atWidth).
		Border(lipgloss.NormalBorder(), true, false, true, false).
		BorderForeground(lipgloss.Color(common.COLOR_MAGENTA)).
		String()

	version := lipgloss.
		NewStyle().
		SetString(util.GetNameAndVersion()).
		Width(versionWidth).
		Height(2).
		Background(lipgloss.Color(common.COLOR_GREY)).
		Padding(1).
		Border(lipgloss.NormalBorder(), true, false, true, false).
		BorderForeground(lipgloss.Color(common.COLOR_MAGENTA)).
		String()

	created := lipgloss.
		NewStyle().
		SetString("registered: "+acc.CreatedAt.Format(util.DateTimeFormat())).
		Background(lipgloss.Color(common.COLOR_MAGENTA)).
		Padding(1).
		Align(lipgloss.Left).
		Height(2).
		Width(createdWidth).
		Border(lipgloss.NormalBorder(), true, false, true, false).
		BorderForeground(lipgloss.Color(common.COLOR_MAGENTA)).
		String()

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		username,
		at,
		version,
		created,
	)
}
