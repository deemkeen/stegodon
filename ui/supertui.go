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
	"github.com/deemkeen/stegodon/ui/following"
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
	followingModel     following.Model
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
	followingModel := following.InitialModel(acc.Id, width, height)
	timelineModel := timeline.InitialModel(acc.Id, width, height)
	localTimelineModel := localtimeline.InitialModel(acc.Id, width, height)
	localUsersModel := localusers.InitialModel(acc.Id, width, height)

	m := MainModel{state: common.CreateUserView}
	m.newUserModel = createuser.InitialModel()
	m.createModel = noteModel
	m.listModel = listModel
	m.followModel = followModel
	m.followersModel = followersModel
	m.followingModel = followingModel
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
	var cmds []tea.Cmd

	// Load notes list on startup
	cmds = append(cmds, m.listModel.Init())

	if m.account.FirstTimeLogin == domain.TRUE {
		cmds = append(cmds, func() tea.Msg {
			return common.CreateUserView
		})
	} else {
		cmds = append(cmds, func() tea.Msg {
			return common.CreateNoteView
		})
	}

	return tea.Batch(cmds...)
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Handle window resize
		m.width = msg.Width
		m.height = msg.Height
		m.headerModel.Width = msg.Width
		return m, nil

	case tea.MouseMsg:
		// Handle mouse clicks to switch focus between left and right panels
		if msg.Type == tea.MouseLeft {
			leftPanelWidth := m.width / 3

			// Click on left panel (write note area)
			if msg.X < leftPanelWidth {
				if m.state != common.CreateUserView {
					m.state = common.CreateNoteView
				}
			} else {
				// Click on right panel - switch to the currently displayed view
				// The right panel shows different views depending on current state
				// Don't change state if already in a right-panel view, just ensure focus
				if m.state == common.CreateNoteView {
					// Default to notes list when clicking right from write note
					m.state = common.ListNotesView
				}
				// Otherwise keep the current right-panel view
			}
		}
		return m, nil

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
		case common.FollowingView:
			m.state = common.FollowingView
		case common.FederatedTimelineView:
			m.state = common.FederatedTimelineView
		case common.LocalTimelineView:
			m.state = common.LocalTimelineView
		case common.LocalUsersView:
			m.state = common.LocalUsersView
		case common.UpdateNoteList:
			m.listModel = listnotes.NewPager(m.account.Id, m.width, m.height)
			// Reload the notes after creating a new pager
			return m, m.listModel.Init()
		}

	case common.EditNoteMsg:
		// Route EditNote message to writenote model and switch to CreateNoteView
		m.createModel, cmd = m.createModel.Update(msg)
		m.state = common.CreateNoteView
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case common.DeleteNoteMsg:
		// Note was deleted, reload the list
		m.listModel = listnotes.NewPager(m.account.Id, m.width, m.height)
		return m, m.listModel.Init()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Cycle through main views (excluding create user)
			if m.state == common.CreateUserView {
				return m, nil
			}
			oldState := m.state
			switch m.state {
			case common.CreateNoteView:
				m.state = common.ListNotesView
			case common.ListNotesView:
				m.state = common.FederatedTimelineView
			case common.FederatedTimelineView:
				m.state = common.LocalTimelineView
			case common.LocalTimelineView:
				m.state = common.FollowUserView
			case common.FollowUserView:
				m.state = common.FollowersView
			case common.FollowersView:
				m.state = common.FollowingView
			case common.FollowingView:
				m.state = common.LocalUsersView
			case common.LocalUsersView:
				m.state = common.CreateNoteView
			}
			// Reload data when switching to certain views
			if oldState != m.state {
				cmd = getViewInitCmd(m.state, &m)
				cmds = append(cmds, cmd)
			}
		case "shift+tab":
			// Cycle backwards through views
			if m.state == common.CreateUserView {
				return m, nil
			}
			oldState := m.state
			switch m.state {
			case common.CreateNoteView:
				m.state = common.LocalUsersView
			case common.ListNotesView:
				m.state = common.CreateNoteView
			case common.FederatedTimelineView:
				m.state = common.ListNotesView
			case common.LocalTimelineView:
				m.state = common.FederatedTimelineView
			case common.FollowUserView:
				m.state = common.LocalTimelineView
			case common.FollowersView:
				m.state = common.FollowUserView
			case common.FollowingView:
				m.state = common.FollowersView
			case common.LocalUsersView:
				m.state = common.FollowingView
			}
			// Reload data when switching to certain views
			if oldState != m.state {
				cmd = getViewInitCmd(m.state, &m)
				cmds = append(cmds, cmd)
			}
		case "enter":
			if m.state == common.CreateUserView {
				m.state = common.CreateNoteView
				m.account.Username = m.newUserModel.TextInput.Value()
				m.headerModel = header.Model{Width: m.width, Acc: &m.account}
				return m, updateUserModelCmd(&m.account)
			}
		}
	}

	// Route non-keyboard messages to ALL sub-models
	// This ensures data loading messages like followersLoadedMsg reach their destination
	// But keyboard messages should only go to the active view
	if _, isKeyMsg := msg.(tea.KeyMsg); !isKeyMsg {
		m.headerModel, _ = m.headerModel.Update(msg)
		m.followModel, cmd = m.followModel.Update(msg)
		cmds = append(cmds, cmd)
		m.followersModel, cmd = m.followersModel.Update(msg)
		cmds = append(cmds, cmd)
		m.followingModel, cmd = m.followingModel.Update(msg)
		cmds = append(cmds, cmd)
		m.timelineModel, cmd = m.timelineModel.Update(msg)
		cmds = append(cmds, cmd)
		m.localTimelineModel, cmd = m.localTimelineModel.Update(msg)
		cmds = append(cmds, cmd)
		m.localUsersModel, cmd = m.localUsersModel.Update(msg)
		cmds = append(cmds, cmd)
		m.listModel, cmd = m.listModel.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Route keyboard input ONLY to active model
	if _, ok := msg.(tea.KeyMsg); ok {
		switch m.state {
		case common.CreateUserView:
			m.newUserModel, cmd = m.newUserModel.Update(msg)
		case common.CreateNoteView:
			m.createModel, cmd = m.createModel.Update(msg)
		case common.ListNotesView:
			m.listModel, cmd = m.listModel.Update(msg)
		case common.FollowUserView:
			m.followModel, cmd = m.followModel.Update(msg)
		case common.FollowersView:
			m.followersModel, cmd = m.followersModel.Update(msg)
		case common.FollowingView:
			m.followingModel, cmd = m.followingModel.Update(msg)
		case common.FederatedTimelineView:
			m.timelineModel, cmd = m.timelineModel.Update(msg)
		case common.LocalTimelineView:
			m.localTimelineModel, cmd = m.localTimelineModel.Update(msg)
		case common.LocalUsersView:
			m.localUsersModel, cmd = m.localUsersModel.Update(msg)
		}
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m MainModel) View() string {

	var s string

	model := m.currentFocusedModel()

	// Calculate responsive dimensions
	availableHeight := m.height - 10 // Account for header and help text
	leftPanelWidth := m.width / 3
	rightPanelWidth := m.width - leftPanelWidth - 6 // Account for borders and margins

	createStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(leftPanelWidth).
		MaxWidth(leftPanelWidth).
		Render(m.createModel.View())

	listStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.listModel.View())

	followStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.followModel.View())

	followersStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.followersModel.View())

	followingStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.followingModel.View())

	timelineStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.timelineModel.View())

	localTimelineStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.localTimelineModel.View())

	localUsersStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.localUsersModel.View())

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
		case common.FollowingView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(followingStyleStr))
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
			viewCommands = "↑/↓: select • u: edit • d: delete"
		case common.FollowUserView:
			viewCommands = "enter: follow"
		case common.FollowersView:
			viewCommands = "↑/↓: scroll"
		case common.FollowingView:
			viewCommands = "↑/↓: select • u/enter: unfollow"
		case common.FederatedTimelineView:
			viewCommands = "↑/↓: select • o: open URL"
		case common.LocalTimelineView:
			viewCommands = "↑/↓: scroll"
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
	case common.FollowingView:
		return "following"
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

// getViewInitCmd returns the init command for a view to reload its data
func getViewInitCmd(state common.SessionState, m *MainModel) tea.Cmd {
	switch state {
	case common.FollowersView:
		return m.followersModel.Init()
	case common.FollowingView:
		return m.followingModel.Init()
	case common.FederatedTimelineView:
		return m.timelineModel.Init()
	case common.LocalTimelineView:
		return m.localTimelineModel.Init()
	case common.LocalUsersView:
		return m.localUsersModel.Init()
	case common.ListNotesView:
		return m.listModel.Init()
	default:
		return nil
	}
}
