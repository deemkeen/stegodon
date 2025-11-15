package header

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/mattn/go-runewidth"
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
	// Single-line header with manual spacing
	elephant := "🦣"

	leftText := fmt.Sprintf("%s %s", elephant, acc.Username)
	centerText := fmt.Sprintf("stegodon v%s", util.GetVersion())
	rightText := fmt.Sprintf("joined: %s", acc.CreatedAt.Format("2006-01-02"))

	// Calculate display widths
	leftLen := runewidth.StringWidth(leftText)
	centerLen := runewidth.StringWidth(centerText)
	rightLen := runewidth.StringWidth(rightText)

	// Calculate spacing to distribute evenly
	totalTextLen := leftLen + centerLen + rightLen
	totalSpacing := width - totalTextLen - 4 // -4 for side padding

	if totalSpacing < 2 {
		totalSpacing = 2
	}

	// Split spacing: half before center, half after
	leftSpacing := totalSpacing / 2
	rightSpacing := totalSpacing - leftSpacing

	// Build the header as a single string with spaces
	spaces := func(n int) string {
		if n < 0 {
			n = 0
		}
		result := ""
		for i := 0; i < n; i++ {
			result += " "
		}
		return result
	}

	header := fmt.Sprintf("  %s%s%s%s%s  ",
		leftText,
		spaces(leftSpacing),
		centerText,
		spaces(rightSpacing),
		rightText,
	)

	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Background(lipgloss.Color(common.COLOR_LIGHTBLUE)).
		Foreground(lipgloss.Color(common.COLOR_WHITE)).
		Bold(true).
		Inline(true).
		Render(header)
}
