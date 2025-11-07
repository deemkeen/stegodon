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
	// Simpler approach: calculate fixed widths for username, @, version
	// and give all remaining space to the created section

	usernameWidth := 15  // Fixed width for username
	atWidth := 3         // Fixed width for @ symbol with padding
	versionWidth := width / 3  // ~33% for version

	// Calculate remaining width for created section
	// Account for borders and padding: each section has border on left/right (2 chars) + padding left/right (2 chars) = 4 chars overhead
	// But we're setting the content Width, so lipgloss will add the 4 chars
	// Total rendered width will be: usernameWidth+4 + atWidth+4 + versionWidth+4 + createdWidth+4

	overhead := 16  // 4 sections * 4 chars each
	usedWidth := usernameWidth + atWidth + versionWidth + overhead - 4 // Subtract 4 because we'll calculate created differently
	createdWidth := width - usedWidth - 4  // The remaining space minus its own overhead

	if createdWidth < 20 {
		createdWidth = 20  // Minimum width
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

	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		username,
		at,
		version,
		created,
	)

	// Wrap in a container that fills to exact width
	return lipgloss.NewStyle().
		Width(width).
		Inline(true).
		Render(header)
}
