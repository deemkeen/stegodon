package createuser

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/util"
)

var (
	Style = lipgloss.NewStyle().Height(35).
		Align(lipgloss.Center, lipgloss.Center).
		BorderStyle(lipgloss.ThickBorder())
)

type Model struct {
	TextInput textinput.Model
	Err       util.ErrMsg
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case util.ErrMsg:
		m.Err = msg
		return m, nil
	}

	m.TextInput, cmd = m.TextInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return fmt.Sprintf(
		"Logging into STEGODON v%s\n\nYou don't have a username yet, please choose wisely!\n\n%s\n\n%s",
		util.GetVersion(),
		m.TextInput.View(),
		"(enter to save, ctrl-c to quit)",
	) + "\n"
}

func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "ElonMusk666"
	ti.Focus()
	ti.CharLimit = 15
	ti.Width = 20

	return Model{
		TextInput: ti,
		Err:       nil,
	}
}
