package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/ui/createuser"
	"github.com/deemkeen/stegodon/ui/header"
	"github.com/deemkeen/stegodon/ui/listnotes"
	"github.com/deemkeen/stegodon/ui/writenote"
	"log"
)

var (
	modelStyle = lipgloss.NewStyle().
			Align(lipgloss.Top, lipgloss.Top).
			BorderStyle(lipgloss.HiddenBorder()).MarginLeft(1)
	focusedModelStyle = lipgloss.NewStyle().
				Align(lipgloss.Top, lipgloss.Top).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(common.COLOR_LIGHTBLUE)).MarginLeft(1)
)

type MainModel struct {
	width        int
	height       int
	headerModel  header.Model
	account      domain.Account
	state        common.SessionState
	newUserModel createuser.Model
	createModel  writenote.Model
	listModel    listnotes.Model
}

func updateUserModelCmd(acc *domain.Account) tea.Cmd {
	return func() tea.Msg {
		acc.FirstTimeLogin = domain.FALSE
		err := db.GetDB().UpdateLoginById(acc.Username, acc.Id)
		if err != nil {
			log.Println(fmt.Sprintf("User %s could not be updated!", acc.Username))
		}
		return nil
	}
}

func NewModel(acc domain.Account, width int, height int) MainModel {

	width = common.DefaultWindowWidth(width)
	height = common.DefaultWindowHeight(height)

	noteModel := writenote.InitialNote(width, acc.Id)
	headerModel := header.Model{Width: width, Acc: &acc}
	listModel := listnotes.NewPager(acc.Id, width, height)

	m := MainModel{state: common.CreateUserView}
	m.newUserModel = createuser.InitialModel()
	m.createModel = noteModel
	m.listModel = listModel
	m.headerModel = headerModel
	m.account = acc
	m.width = width
	m.height = height
	return m
}

func (m MainModel) Init() tea.Cmd {
	if m.account.FirstTimeLogin == domain.TRUE {
		return func() tea.Msg {
			return common.CreateUserView
		}
	}

	return func() tea.Msg {
		return common.CreateNoteView
	}
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case common.SessionState:
		switch msg {
		case common.CreateUserView:
			{
				m.state = common.CreateUserView
			}
		case common.ListNotesView:
			{
				m.state = common.ListNotesView
			}
		case common.CreateNoteView:
			{
				m.state = common.CreateNoteView
			}
		case common.UpdateNoteList:
			{
				m.listModel = listnotes.
					NewPager(m.account.Id, m.width, m.height)
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.state == common.CreateNoteView {
				m.state = common.ListNotesView
			} else {
				m.state = common.CreateNoteView
			}
		case "enter":
			if m.state == common.CreateUserView {
				m.state = common.CreateNoteView
				m.account.Username = m.newUserModel.TextInput.Value()
				m.headerModel = header.Model{Width: m.width, Acc: &m.account}
				return m, updateUserModelCmd(&m.account)
			}
		}
		switch m.state {
		//switch between models
		case common.CreateUserView:
			m.newUserModel, cmd = m.newUserModel.Update(msg)
		case common.ListNotesView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.listModel, cmd = m.listModel.Update(msg)
		case common.CreateNoteView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.createModel, cmd = m.createModel.Update(msg)
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m MainModel) View() string {

	var s string

	model := m.currentFocusedModel()

	createStyleStr := lipgloss.NewStyle().MaxHeight(25).Height(25).Width(44).MaxWidth(44).Render(m.createModel.View())
	listStyleStr := lipgloss.NewStyle().MaxHeight(35).Height(35).MaxWidth(88).Margin(1).Render(m.listModel.View())

	if m.state == common.CreateUserView {
		s = createuser.Style.Width(m.width).Render(m.newUserModel.View())
		return s
	} else {
		navContainer := lipgloss.NewStyle().Render(m.headerModel.View())

		s += navContainer + "\n"

		if m.state == common.CreateNoteView {
			s += lipgloss.
				JoinHorizontal(lipgloss.Top, focusedModelStyle.Render(createStyleStr),
					modelStyle.Render(listStyleStr))
		} else {
			s += lipgloss.
				JoinHorizontal(lipgloss.Top, modelStyle.Render(createStyleStr),
					focusedModelStyle.Render(listStyleStr))
		}

		var listCommands string

		if m.state == common.ListNotesView {
			listCommands = "• arrows: scroll pages"
		} else {
			listCommands = " "
		}

		s += common.HelpStyle.
			Render(fmt.Sprintf("focused > %s\t\tkeys > tab: focus next %s • ctrl-c: exit", model, listCommands))
		return lipgloss.NewStyle().Render(s)
	}

}

func (m MainModel) currentFocusedModel() string {
	if m.state == common.CreateNoteView {
		return "new note"
	} else if m.state == common.ListNotesView {
		return "notes list"
	} else {
		return "newUserModel"
	}
}
