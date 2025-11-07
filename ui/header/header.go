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
	// Account for borders: each section has left+right borders (2 chars each)
	// We have 4 sections, so 4*2 = 8 border chars total
	// Also account for padding: each section has padding left+right (2 chars each) = 4*2 = 8 padding chars
	totalBorderAndPadding := 16

	availableWidth := width - totalBorderAndPadding
	if availableWidth < 40 {
		availableWidth = 40 // Minimum to prevent overflow
	}

	// Distribute available width proportionally
	usernameWidth := availableWidth / 6      // ~16%
	atWidth := 1                              // Just the @ symbol
	versionWidth := availableWidth / 2       // ~50%
	createdWidth := availableWidth - usernameWidth - atWidth - versionWidth // Remaining

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

	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		username,
		at,
		version,
		created,
	)

	// Wrap in a container that's exactly the terminal width
	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Render(header)
}
