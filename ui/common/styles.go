package common

import "github.com/charmbracelet/lipgloss"

const (
	COLOR_GREY      = "241"
	COLOR_MAGENTA   = "170"
	COLOR_LIGHTBLUE = "69"
	COLOR_PURPLE    = "#7D56F4"
)

var (
	HelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_GREY)).Padding(0, 2)
	CaptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_MAGENTA)).Padding(2)
)

func DefaultWindowWidth(width int) int {
	return width - 10
}

func DefaultWindowHeight(heigth int) int {
	return heigth - 10
}

func DefaultCreateNoteWidth(width int) int {
	return width / 4
}

func DefaultListWidth(width int) int {
	return width - DefaultCreateNoteWidth(width)
}

func DefaultListHeight(height int) int {
	return height
}
