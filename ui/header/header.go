package header

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/mattn/go-runewidth"
)

type Model struct {
	Width       int
	Acc         *domain.Account
	UnreadCount int
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() tea.View {
	return tea.NewView(GetHeaderStyle(m.Acc, m.Width, m.UnreadCount))
}

func GetHeaderStyle(acc *domain.Account, width int, unreadCount int) string {
	// Single-line header with manual spacing
	elephant := "🦣"

	leftTextPlain := fmt.Sprintf("%s %s", elephant, acc.Username)
	badgePlain := ""

	// Add notification badge if there are unread notifications
	if unreadCount > 0 {
		badgePlain = fmt.Sprintf(" [%d]", unreadCount)
		leftTextPlain += badgePlain
	}
	centerText := fmt.Sprintf("stegodon v%s", util.GetVersion())
	rightText := fmt.Sprintf("joined: %s", acc.CreatedAt.Format("2006-01-02"))

	// Calculate display widths using plain text (without ANSI codes)
	leftLen := runewidth.StringWidth(leftTextPlain)
	centerLen := runewidth.StringWidth(centerText)
	rightLen := runewidth.StringWidth(rightText)

	// Calculate spacing to distribute evenly
	totalTextLen := leftLen + centerLen + rightLen
	totalSpacing := maxInt(
		// Subtract padding for the 2 spaces on each side of header content
		width-totalTextLen-common.HeaderTotalPadding, 2)

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

	// Build leftText - we'll use raw ANSI codes for the badge to avoid breaking the background
	leftText := fmt.Sprintf("%s %s", elephant, acc.Username)
	if unreadCount > 0 {
		// Use raw ANSI escape codes for warning color
		// This avoids lipgloss resetting the background
		leftText += common.ANSI_WARNING_START + badgePlain + common.ANSI_COLOR_RESET
	}

	header := fmt.Sprintf("  %s%s%s%s%s  ",
		leftText,
		spaces(leftSpacing),
		centerText,
		spaces(rightSpacing),
		rightText,
	)

	// Apply background and foreground to the entire header line
	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Background(lipgloss.Color(common.COLOR_ACCENT)).
		Foreground(lipgloss.Color(common.COLOR_WHITE)).
		Bold(true).
		Render(header)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
