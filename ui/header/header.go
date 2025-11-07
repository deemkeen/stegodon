package header

import (
	"fmt"
	"strings"

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
	// Single bar design that fills entire width
	// Format: "username @ stegodon/version | registered: date"

	leftText := fmt.Sprintf("%s @ %s", acc.Username, util.GetNameAndVersion())
	rightText := fmt.Sprintf("registered: %s", acc.CreatedAt.Format(util.DateTimeFormat()))

	// Calculate spacing needed to fill the width
	// Account for padding on both sides (2 chars each = 4 total)
	textLength := len(leftText) + len(rightText) + 3 // +3 for " | "
	paddingTotal := 4
	spacesNeeded := width - textLength - paddingTotal

	if spacesNeeded < 1 {
		spacesNeeded = 1
	}

	// Build the header text with spacing
	headerText := fmt.Sprintf("%s%s%s", leftText, strings.Repeat(" ", spacesNeeded), rightText)

	return lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color(common.COLOR_PURPLE)).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Bold(true).
		Render(headerText)
}
