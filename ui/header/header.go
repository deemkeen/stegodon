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
	username := lipgloss.
		NewStyle().
		SetString(acc.Username).
		Align(lipgloss.Left).
		Background(lipgloss.Color(common.COLOR_PURPLE)).
		Padding(1).
		Height(2).
		Width(width).
		MaxWidth(15).
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
		Width(width).
		MaxWidth(75).
		Border(lipgloss.NormalBorder(), true, false, true, false).
		BorderForeground(lipgloss.Color(common.COLOR_MAGENTA)).
		String()

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		username,
		at,
		lipgloss.NewStyle().
			SetString(util.GetNameAndVersion()).
			Width(65).
			MaxWidth(45).
			Height(2).
			Background(lipgloss.Color(common.COLOR_GREY)).
			Padding(1).
			Border(lipgloss.NormalBorder(), true, false, true, false).
			BorderForeground(lipgloss.Color(common.COLOR_MAGENTA)).
			String(),
		created,
	)
}
