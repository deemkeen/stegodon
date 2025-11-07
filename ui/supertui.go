package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/ui/createuser"
	"github.com/deemkeen/stegodon/ui/followers"
	"github.com/deemkeen/stegodon/ui/followuser"
	"github.com/deemkeen/stegodon/ui/header"
	"github.com/deemkeen/stegodon/ui/listnotes"
	"github.com/deemkeen/stegodon/ui/localtimeline"
	"github.com/deemkeen/stegodon/ui/localusers"
	"github.com/deemkeen/stegodon/ui/timeline"
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
	width              int
	height             int
	headerModel        header.Model
	account            domain.Account
	state              common.SessionState
	newUserModel       createuser.Model
	createModel        writenote.Model
	listModel          listnotes.Model
	followModel        followuser.Model
	followersModel     followers.Model
	timelineModel      timeline.Model
	localTimelineModel localtimeline.Model
	localUsersModel    localusers.Model
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
	followModel := followuser.InitialModel(acc.Id)
	followersModel := followers.InitialModel(acc.Id, width, height)
	timelineModel := timeline.InitialModel(width, height)
	localTimelineModel := localtimeline.InitialModel(width, height)
	localUsersModel := localusers.InitialModel(acc.Id, width, height)

	m := MainModel{state: common.CreateUserView}
	m.newUserModel = createuser.InitialModel()
	m.createModel = noteModel
	m.listModel = listModel
	m.followModel = followModel
	m.followersModel = followersModel
	m.timelineModel = timelineModel
	m.localTimelineModel = localTimelineModel
	m.localUsersModel = localUsersModel
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
			m.state = common.CreateUserView
		case common.ListNotesView:
			m.state = common.ListNotesView
		case common.CreateNoteView:
			m.state = common.CreateNoteView
		case common.FollowUserView:
			m.state = common.FollowUserView
		case common.FollowersView:
			m.state = common.FollowersView
		case common.FederatedTimelineView:
			m.state = common.FederatedTimelineView
		case common.LocalTimelineView:
			m.state = common.LocalTimelineView
		case common.LocalUsersView:
			m.state = common.LocalUsersView
		case common.UpdateNoteList:
			m.listModel = listnotes.NewPager(m.account.Id, m.width, m.height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Cycle through main views (excluding create user)
			if m.state == common.CreateUserView {
				return m, nil
			}
			switch m.state {
			case common.CreateNoteView:
				m.state = common.ListNotesView
			case common.ListNotesView:
				m.state = common.FollowUserView
			case common.FollowUserView:
				m.state = common.FollowersView
			case common.FollowersView:
				m.state = common.FederatedTimelineView
			case common.FederatedTimelineView:
				m.state = common.LocalTimelineView
			case common.LocalTimelineView:
				m.state = common.LocalUsersView
			case common.LocalUsersView:
				m.state = common.CreateNoteView
			}
		case "shift+tab":
			// Cycle backwards through views
			if m.state == common.CreateUserView {
				return m, nil
			}
			switch m.state {
			case common.CreateNoteView:
				m.state = common.LocalUsersView
			case common.ListNotesView:
				m.state = common.CreateNoteView
			case common.FollowUserView:
				m.state = common.ListNotesView
			case common.FollowersView:
				m.state = common.FollowUserView
			case common.FederatedTimelineView:
				m.state = common.FollowersView
			case common.LocalTimelineView:
				m.state = common.FederatedTimelineView
			case common.LocalUsersView:
				m.state = common.LocalTimelineView
			}
		case "enter":
			if m.state == common.CreateUserView {
				m.state = common.CreateNoteView
				m.account.Username = m.newUserModel.TextInput.Value()
				m.headerModel = header.Model{Width: m.width, Acc: &m.account}
				return m, updateUserModelCmd(&m.account)
			}
		}

		// Route to appropriate sub-model
		switch m.state {
		case common.CreateUserView:
			m.newUserModel, cmd = m.newUserModel.Update(msg)
		case common.ListNotesView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.listModel, cmd = m.listModel.Update(msg)
		case common.CreateNoteView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.createModel, cmd = m.createModel.Update(msg)
		case common.FollowUserView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.followModel, cmd = m.followModel.Update(msg)
		case common.FollowersView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.followersModel, cmd = m.followersModel.Update(msg)
		case common.FederatedTimelineView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.timelineModel, cmd = m.timelineModel.Update(msg)
		case common.LocalTimelineView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.localTimelineModel, cmd = m.localTimelineModel.Update(msg)
		case common.LocalUsersView:
			m.headerModel, cmd = m.headerModel.Update(msg)
			m.localUsersModel, cmd = m.localUsersModel.Update(msg)
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
	followStyleStr := lipgloss.NewStyle().MaxHeight(35).Height(35).Width(60).MaxWidth(60).Margin(1).Render(m.followModel.View())
	followersStyleStr := lipgloss.NewStyle().MaxHeight(35).Height(35).Width(60).MaxWidth(60).Margin(1).Render(m.followersModel.View())
	timelineStyleStr := lipgloss.NewStyle().MaxHeight(35).Height(35).Width(88).MaxWidth(88).Margin(1).Render(m.timelineModel.View())
	localTimelineStyleStr := lipgloss.NewStyle().MaxHeight(35).Height(35).Width(88).MaxWidth(88).Margin(1).Render(m.localTimelineModel.View())
	localUsersStyleStr := lipgloss.NewStyle().MaxHeight(35).Height(35).Width(60).MaxWidth(60).Margin(1).Render(m.localUsersModel.View())

	if m.state == common.CreateUserView {
		s = createuser.Style.Width(m.width).Render(m.newUserModel.View())
		return s
	} else {
		navContainer := lipgloss.NewStyle().Render(m.headerModel.View())
		s += navContainer + "\n"

		// Render current view
		switch m.state {
		case common.CreateNoteView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				focusedModelStyle.Render(createStyleStr),
				modelStyle.Render(listStyleStr))
		case common.ListNotesView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(listStyleStr))
		case common.FollowUserView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(followStyleStr))
		case common.FollowersView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(followersStyleStr))
		case common.FederatedTimelineView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(timelineStyleStr))
		case common.LocalTimelineView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(localTimelineStyleStr))
		case common.LocalUsersView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(localUsersStyleStr))
		}

		// Help text
		var viewCommands string
		switch m.state {
		case common.ListNotesView:
			viewCommands = "arrows: scroll"
		case common.FollowUserView:
			viewCommands = "enter: follow"
		case common.LocalUsersView:
			viewCommands = "↑/↓: select • enter: toggle follow"
		default:
			viewCommands = " "
		}

		s += common.HelpStyle.Render(fmt.Sprintf(
			"focused > %s\t\tkeys > tab: next • shift+tab: prev • %s • ctrl-c: exit",
			model, viewCommands))
		return lipgloss.NewStyle().Render(s)
	}
}

func (m MainModel) currentFocusedModel() string {
	switch m.state {
	case common.CreateNoteView:
		return "new note"
	case common.ListNotesView:
		return "notes list"
	case common.FollowUserView:
		return "follow user"
	case common.FollowersView:
		return "followers"
	case common.FederatedTimelineView:
		return "federated timeline"
	case common.LocalTimelineView:
		return "local timeline"
	case common.LocalUsersView:
		return "local users"
	default:
		return "create user"
	}
}
