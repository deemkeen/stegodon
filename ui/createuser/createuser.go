package createuser

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/util"
)

var (
	Style = lipgloss.NewStyle().Height(25).Width(80).
		Align(lipgloss.Center, lipgloss.Center).
		BorderStyle(lipgloss.ThickBorder()).
		Margin(0, 3)
)

type Model struct {
	TextInput   textinput.Model
	DisplayName textinput.Model
	Bio         textinput.Model
	Step        int // 0=username, 1=display name, 2=bio
	Err         util.ErrMsg
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
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.Step == 0 {
				// Move to display name
				m.Step = 1
				m.DisplayName.Focus()
				m.TextInput.Blur()
				return m, nil
			} else if m.Step == 1 {
				// Move to bio
				m.Step = 2
				m.Bio.Focus()
				m.DisplayName.Blur()
				return m, nil
			}
			// Step 2 (bio) - form submission handled by parent
		}
	}

	// Update the active input
	switch m.Step {
	case 0:
		m.TextInput, cmd = m.TextInput.Update(msg)
	case 1:
		m.DisplayName, cmd = m.DisplayName.Update(msg)
	case 2:
		m.Bio, cmd = m.Bio.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	var prompt string
	var input string
	var help string

	switch m.Step {
	case 0:
		prompt = "You don't have a username yet, please choose wisely!"
		input = m.TextInput.View()
		help = "(enter to continue, ctrl-c to quit)"
	case 1:
		prompt = fmt.Sprintf("Username: %s\n\nChoose your display name (optional):", m.TextInput.Value())
		input = m.DisplayName.View()
		help = "(enter to continue, leave empty to skip)"
	case 2:
		prompt = fmt.Sprintf("Username: %s\nDisplay name: %s\n\nWrite a short bio (optional):",
			m.TextInput.Value(),
			m.DisplayName.Value())
		input = m.Bio.View()
		help = "(enter to save profile, ctrl-c to quit)"
	}

	return fmt.Sprintf(
		"Logging into STEGODON v%s\n\n%s\n\n%s\n\n%s",
		util.GetVersion(),
		prompt,
		input,
		help,
	) + "\n"
}

// ViewWithWidth renders the view with proper width accounting for border and margins
func (m Model) ViewWithWidth(termWidth, termHeight int) string {
	// Account for border (2 chars) and margins already defined in Style (6 chars total)
	// Total to subtract: 2 (border) + 6 (margins) = 8
	contentWidth := termWidth - 8
	if contentWidth < 40 {
		contentWidth = 40 // Minimum width
	}

	bordered := Style.Width(contentWidth).Render(m.View())
	return lipgloss.Place(termWidth, termHeight, lipgloss.Center, lipgloss.Center, bordered)
}

func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "ElonMusk666"
	ti.Focus()
	ti.CharLimit = 15
	ti.Width = 20

	displayName := textinput.New()
	displayName.Placeholder = "John Doe"
	displayName.CharLimit = 50
	displayName.Width = 50

	bio := textinput.New()
	bio.Placeholder = "CEO of X, Tesla, SpaceX..."
	bio.CharLimit = 200
	bio.Width = 60

	return Model{
		TextInput:   ti,
		DisplayName: displayName,
		Bio:         bio,
		Step:        0,
		Err:         nil,
	}
}
