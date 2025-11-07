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
	// Calculate proportional widths
	usernameWidth := width / 6      // ~16% of screen
	versionWidth := width / 2       // ~50% of screen
	createdWidth := width - usernameWidth - versionWidth - 10 // Remaining space minus borders

	// Ensure minimum widths
	if usernameWidth < 10 {
		usernameWidth = 10
	}
	if versionWidth < 20 {
		versionWidth = 20
	}
	if createdWidth < 20 {
		createdWidth = 20
	}

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
		lipgloss.NewStyle().
			SetString(util.GetNameAndVersion()).
			Width(versionWidth).
			Height(2).
			Background(lipgloss.Color(common.COLOR_GREY)).
			Padding(1).
			Border(lipgloss.NormalBorder(), true, false, true, false).
			BorderForeground(lipgloss.Color(common.COLOR_MAGENTA)).
			String(),
		created,
	)
}
