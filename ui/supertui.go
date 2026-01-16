package ui

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/admin"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/ui/createuser"
	"github.com/deemkeen/stegodon/ui/deleteaccount"
	"github.com/deemkeen/stegodon/ui/followers"
	"github.com/deemkeen/stegodon/ui/following"
	"github.com/deemkeen/stegodon/ui/followuser"
	"github.com/deemkeen/stegodon/ui/header"
	"github.com/deemkeen/stegodon/ui/hometimeline"
	"github.com/deemkeen/stegodon/ui/localusers"
	"github.com/deemkeen/stegodon/ui/myposts"
	"github.com/deemkeen/stegodon/ui/notifications"
	"github.com/deemkeen/stegodon/ui/relay"
	"github.com/deemkeen/stegodon/ui/threadview"
	"github.com/deemkeen/stegodon/ui/writenote"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

var (
	modelStyle = lipgloss.NewStyle().
			Align(lipgloss.Top, lipgloss.Top).
			BorderStyle(lipgloss.HiddenBorder()).MarginLeft(1)
	focusedModelStyle = lipgloss.NewStyle().
				Align(lipgloss.Top, lipgloss.Top).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(common.COLOR_ACCENT)).MarginLeft(1)
)

type MainModel struct {
	width              int
	height             int
	config             *util.AppConfig
	headerModel        header.Model
	account            domain.Account
	state              common.SessionState
	newUserModel       createuser.Model
	createModel        writenote.Model
	myPostsModel       myposts.Model
	followModel        followuser.Model
	followersModel     followers.Model
	followingModel     following.Model
	homeTimelineModel  hometimeline.Model
	localUsersModel    localusers.Model
	adminModel         admin.Model
	relayModel         relay.Model
	deleteAccountModel deleteaccount.Model
	threadViewModel    threadview.Model
	notificationsModel notifications.Model
}

type userUpdateErrorMsg struct {
	err error
}

func updateUserModelCmd(acc *domain.Account) tea.Cmd {
	return func() tea.Msg {
		acc.FirstTimeLogin = domain.FALSE
		err := db.GetDB().UpdateLoginById(acc.Username, acc.DisplayName, acc.Summary, acc.Id)
		if err != nil {
			log.Printf("User %s could not be updated: %v", acc.Username, err)
			return userUpdateErrorMsg{err: err}
		}
		return nil
	}
}

func NewModel(acc domain.Account, width int, height int) MainModel {

	width = common.DefaultWindowWidth(width)
	height = common.DefaultWindowHeight(height)

	// Load config for relay management and local domain caching
	config, err := util.ReadConf()
	if err != nil {
		log.Printf("Failed to read config: %v", err)
	}

	// Cache local domain for mention highlighting (avoids re-reading config on every render)
	localDomain := ""
	if config != nil {
		localDomain = config.Conf.SslDomain
	}

	noteModel := writenote.InitialNote(width, acc.Id)
	headerModel := header.Model{Width: width, Acc: &acc}
	myPostsModel := myposts.NewPager(acc.Id, width, height, localDomain)
	followModel := followuser.InitialModel(acc.Id)
	followersModel := followers.InitialModel(acc.Id, width, height)
	followingModel := following.InitialModel(acc.Id, width, height)
	homeTimelineModel := hometimeline.InitialModel(acc.Id, width, height, localDomain)
	localUsersModel := localusers.InitialModel(acc.Id, width, height)
	adminModel := admin.InitialModel(acc.Id, width, height)
	relayModel := relay.InitialModel(acc.Id, &acc, config, width, height)
	deleteAccountModel := deleteaccount.InitialModel(&acc)
	threadViewModel := threadview.InitialModel(acc.Id, width, height, localDomain)
	notificationsModel := notifications.InitialModel(acc.Id, width, height)

	m := MainModel{state: common.CreateUserView}
	m.config = config
	m.newUserModel = createuser.InitialModel()
	m.createModel = noteModel
	m.myPostsModel = myPostsModel
	m.followModel = followModel
	m.followersModel = followersModel
	m.followingModel = followingModel
	m.homeTimelineModel = homeTimelineModel
	m.localUsersModel = localUsersModel
	m.adminModel = adminModel
	m.relayModel = relayModel
	m.deleteAccountModel = deleteAccountModel
	m.threadViewModel = threadViewModel
	m.notificationsModel = notificationsModel
	m.headerModel = headerModel
	m.account = acc
	m.width = width
	m.height = height
	return m
}

func (m MainModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Load my posts list on startup
	cmds = append(cmds, m.myPostsModel.Init())

	// Load home timeline on startup (shown in right panel)
	// Also activates notifications model to start badge refresh
	cmds = append(cmds, func() tea.Msg { return common.ActivateViewMsg{} })

	if m.account.FirstTimeLogin == domain.TRUE {
		cmds = append(cmds, func() tea.Msg {
			return common.CreateUserView
		})
	} else {
		cmds = append(cmds, func() tea.Msg {
			return common.CreateNoteView
		})
		// Initialize writenote model to start cursor blinking
		cmds = append(cmds, m.createModel.Init())
	}

	return tea.Batch(cmds...)
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case userUpdateErrorMsg:
		// Handle username validation error
		if m.state == common.CreateUserView {
			m.newUserModel.Error = msg.err.Error()
			m.newUserModel.Step = 0 // Reset to username step
			m.newUserModel.TextInput.Focus()
			return m, nil
		}

	case tea.WindowSizeMsg:
		// Handle window resize - update all models that use width/height for layout
		m.width = msg.Width
		m.height = msg.Height
		m.headerModel.Width = msg.Width
		m.myPostsModel.Width = msg.Width
		m.myPostsModel.Height = msg.Height
		m.homeTimelineModel.Width = msg.Width
		m.homeTimelineModel.Height = msg.Height
		m.followersModel.Width = msg.Width
		m.followersModel.Height = msg.Height
		m.followingModel.Width = msg.Width
		m.followingModel.Height = msg.Height
		m.localUsersModel.Width = msg.Width
		m.localUsersModel.Height = msg.Height
		m.threadViewModel.Width = msg.Width
		m.threadViewModel.Height = msg.Height
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
					// Default to home timeline when clicking right from write note
					m.state = common.HomeTimelineView
				}
				// Otherwise keep the current right-panel view
			}
		}
		return m, nil

	case common.SessionState:
		switch msg {
		case common.CreateUserView:
			m.state = common.CreateUserView
		case common.HomeTimelineView:
			m.state = common.HomeTimelineView
		case common.MyPostsView:
			m.state = common.MyPostsView
		case common.CreateNoteView:
			m.state = common.CreateNoteView
		case common.FollowUserView:
			m.state = common.FollowUserView
		case common.FollowersView:
			m.state = common.FollowersView
		case common.FollowingView:
			m.state = common.FollowingView
		case common.LocalUsersView:
			m.state = common.LocalUsersView
		case common.DeleteAccountView:
			m.state = common.DeleteAccountView
		case common.ThreadView:
			m.state = common.ThreadView
		case common.UpdateNoteList:
			// Route to models that need to refresh (handled by SessionState routing below)
			// Note: This message is also a SessionState, so it will trigger reloads
			// in myposts and hometimeline via the SessionState routing
		}

	case common.EditNoteMsg:
		// Route EditNote message to writenote model and switch to CreateNoteView
		m.createModel, cmd = m.createModel.Update(msg)
		m.state = common.CreateNoteView
		// Return single command directly instead of batching
		return m, cmd

	case common.DeleteNoteMsg:
		// Note was deleted, reload the list
		localDomain := ""
		if m.config != nil {
			localDomain = m.config.Conf.SslDomain
		}
		m.myPostsModel = myposts.NewPager(m.account.Id, m.width, m.height, localDomain)
		return m, m.myPostsModel.Init()

	case common.ReplyToNoteMsg:
		// Route ReplyToNote message to writenote model and switch to CreateNoteView
		m.createModel, cmd = m.createModel.Update(msg)
		m.state = common.CreateNoteView
		return m, cmd

	case common.ViewThreadMsg:
		// Route ViewThread message to threadview model and switch to ThreadView
		m.threadViewModel, cmd = m.threadViewModel.Update(msg)
		m.state = common.ThreadView
		return m, cmd

	case common.LikeNoteMsg:
		// Handle like/unlike
		return m, likeNoteCmd(m.account.Id, msg.NoteURI, msg.NoteID, msg.IsLocal, &m.account)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+n":
			// Navigate to notifications (global shortcut, works from any view)
			if m.state != common.NotificationsView {
				oldState := m.state
				m.state = common.NotificationsView

				// Manage home timeline activation
				oldTimelineVisible := (oldState == common.CreateNoteView || oldState == common.HomeTimelineView)
				// Notifications view doesn't show timeline
				if oldTimelineVisible {
					// Timeline becoming hidden, deactivate it
					cmds = append(cmds, func() tea.Msg { return common.DeactivateViewMsg{} })
				}

				// Note: No need to activate notifications - it's always active
			}
		case "tab":
			// Cycle through main views (excluding create user)
			// Order: write -> home -> my posts -> [follow] -> followers -> following -> users -> [admin -> relay] -> delete
			// AP-only views: follow remote user, relay management
			if m.state == common.CreateUserView {
				return m, nil
			}
			// Block tab navigation when in admin submenus (users/info boxes management)
			if m.state == common.AdminPanelView && m.adminModel.CurrentView != 0 {
				// Tab is blocked in submenus
				return m, nil
			}
			oldState := m.state
			switch m.state {
			case common.CreateNoteView:
				m.state = common.HomeTimelineView
			case common.HomeTimelineView:
				m.state = common.MyPostsView
			case common.MyPostsView:
				if m.config.Conf.WithAp {
					m.state = common.FollowUserView
				} else {
					m.state = common.FollowersView
				}
			case common.FollowUserView:
				m.state = common.FollowersView
			case common.FollowersView:
				m.state = common.FollowingView
			case common.FollowingView:
				m.state = common.LocalUsersView
			case common.LocalUsersView:
				if m.account.IsAdmin {
					m.state = common.AdminPanelView
				} else {
					m.state = common.DeleteAccountView
				}
			case common.AdminPanelView:
				if m.config.Conf.WithAp {
					m.state = common.RelayManagementView
				} else {
					m.state = common.DeleteAccountView
				}
			case common.RelayManagementView:
				m.state = common.DeleteAccountView
			case common.DeleteAccountView:
				m.state = common.NotificationsView
			case common.NotificationsView:
				m.state = common.CreateNoteView
			}
			// Handle focus changes for writenote textarea
			if oldState == common.CreateNoteView {
				m.createModel.Blur()
			}
			if m.state == common.CreateNoteView {
				m.createModel.Focus()
			}
			// Manage home timeline activation based on visibility
			// Home timeline is visible when in CreateNoteView or HomeTimelineView
			oldTimelineVisible := (oldState == common.CreateNoteView || oldState == common.HomeTimelineView)
			newTimelineVisible := (m.state == common.CreateNoteView || m.state == common.HomeTimelineView)

			if oldTimelineVisible && !newTimelineVisible {
				// Timeline becoming hidden, deactivate it
				cmds = append(cmds, func() tea.Msg { return common.DeactivateViewMsg{} })
			} else if !oldTimelineVisible && newTimelineVisible {
				// Timeline becoming visible, activate it
				cmds = append(cmds, func() tea.Msg { return common.ActivateViewMsg{} })
			}

			// Note: Notifications model is never deactivated because the badge
			// in the header needs to show real-time unread count

			// Reload data when switching to certain views
			if oldState != m.state {
				cmd = getViewInitCmd(m.state, &m)
				cmds = append(cmds, cmd)
			}
		case "shift+tab":
			// Cycle backwards through views
			// AP-only views: follow remote user, relay management
			if m.state == common.CreateUserView {
				return m, nil
			}
			// Block shift+tab navigation when in admin submenus (users/info boxes management)
			if m.state == common.AdminPanelView && m.adminModel.CurrentView != 0 {
				// Shift+tab is blocked in submenus
				return m, nil
			}
			oldState := m.state
			switch m.state {
			case common.CreateNoteView:
				m.state = common.NotificationsView
			case common.NotificationsView:
				m.state = common.DeleteAccountView
			case common.HomeTimelineView:
				m.state = common.CreateNoteView
			case common.MyPostsView:
				m.state = common.HomeTimelineView
			case common.FollowUserView:
				m.state = common.MyPostsView
			case common.FollowersView:
				if m.config.Conf.WithAp {
					m.state = common.FollowUserView
				} else {
					m.state = common.MyPostsView
				}
			case common.FollowingView:
				m.state = common.FollowersView
			case common.LocalUsersView:
				m.state = common.FollowingView
			case common.AdminPanelView:
				m.state = common.LocalUsersView
			case common.RelayManagementView:
				m.state = common.AdminPanelView
			case common.DeleteAccountView:
				if m.account.IsAdmin {
					if m.config.Conf.WithAp {
						m.state = common.RelayManagementView
					} else {
						m.state = common.AdminPanelView
					}
				} else {
					m.state = common.LocalUsersView
				}
			}
			// Handle focus changes for writenote textarea
			if oldState == common.CreateNoteView {
				m.createModel.Blur()
			}
			if m.state == common.CreateNoteView {
				m.createModel.Focus()
			}
			// Manage home timeline activation based on visibility
			// Home timeline is visible when in CreateNoteView or HomeTimelineView
			oldTimelineVisible := (oldState == common.CreateNoteView || oldState == common.HomeTimelineView)
			newTimelineVisible := (m.state == common.CreateNoteView || m.state == common.HomeTimelineView)

			if oldTimelineVisible && !newTimelineVisible {
				// Timeline becoming hidden, deactivate it
				cmds = append(cmds, func() tea.Msg { return common.DeactivateViewMsg{} })
			} else if !oldTimelineVisible && newTimelineVisible {
				// Timeline becoming visible, activate it
				cmds = append(cmds, func() tea.Msg { return common.ActivateViewMsg{} })
			}

			// Note: Notifications model is never deactivated because the badge
			// in the header needs to show real-time unread count

			// Reload data when switching to certain views
			if oldState != m.state {
				cmd = getViewInitCmd(m.state, &m)
				cmds = append(cmds, cmd)
			}
		case "enter":
			if m.state == common.CreateUserView {
				// Check which step we're on
				if m.newUserModel.Step < 2 {
					// Still in username or display name step, let createuser handle it
					m.newUserModel, cmd = m.newUserModel.Update(msg)
					return m, cmd
				}
				// Step 2 (bio) - save all info
				m.state = common.CreateNoteView
				m.account.Username = m.newUserModel.TextInput.Value()
				m.account.DisplayName = m.newUserModel.DisplayName.Value()
				m.account.Summary = m.newUserModel.Bio.Value()

				// Use username as display name if not provided
				if m.account.DisplayName == "" {
					m.account.DisplayName = m.account.Username
				}

				m.headerModel = header.Model{Width: m.width, Acc: &m.account}
				// Update deleteAccountModel and relayModel with the new account info
				m.deleteAccountModel.Account = &m.account
				m.relayModel.AdminAcct = &m.account
				return m, updateUserModelCmd(&m.account)
			}
		}
	}

	// Route specific message types to appropriate models
	// This is more efficient than routing ALL messages to ALL models
	switch msg.(type) {
	case common.ActivateViewMsg, common.DeactivateViewMsg:
		// Activation/deactivation messages go to home timeline, myposts, and notifications models
		m.homeTimelineModel, cmd = m.homeTimelineModel.Update(msg)
		cmds = append(cmds, cmd)
		m.myPostsModel, cmd = m.myPostsModel.Update(msg)
		cmds = append(cmds, cmd)
		m.notificationsModel, cmd = m.notificationsModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.EditNoteMsg, common.DeleteNoteMsg, common.SessionState:
		// Note-related messages go to note models
		m.myPostsModel, cmd = m.myPostsModel.Update(msg)
		cmds = append(cmds, cmd)
		m.createModel, cmd = m.createModel.Update(msg)
		cmds = append(cmds, cmd)
		// Also route SessionState to home timeline for UpdateNoteList handling
		m.homeTimelineModel, cmd = m.homeTimelineModel.Update(msg)
		cmds = append(cmds, cmd)
		// Route SessionState to threadview for like count updates
		m.threadViewModel, cmd = m.threadViewModel.Update(msg)
		cmds = append(cmds, cmd)
	case tea.KeyMsg:
		// Keyboard input handled below in separate switch
	default:
		// For other messages (data loaded messages, feedback messages, etc.),
		// route based on model safety regarding goroutine leaks

		// Always route to models that need feedback and don't spawn tickers
		// These are safe from goroutine accumulation
		m.myPostsModel, cmd = m.myPostsModel.Update(msg)
		cmds = append(cmds, cmd)
		m.followModel, cmd = m.followModel.Update(msg)
		cmds = append(cmds, cmd)
		m.deleteAccountModel, cmd = m.deleteAccountModel.Update(msg)
		cmds = append(cmds, cmd)
		m.followersModel, cmd = m.followersModel.Update(msg)
		cmds = append(cmds, cmd)
		m.followingModel, cmd = m.followingModel.Update(msg)
		cmds = append(cmds, cmd)
		m.localUsersModel, cmd = m.localUsersModel.Update(msg)
		cmds = append(cmds, cmd)

		// Always route to home timeline and notifications - they have internal isActive state
		// that controls whether they process messages (prevents ticker leaks)
		m.homeTimelineModel, cmd = m.homeTimelineModel.Update(msg)
		cmds = append(cmds, cmd)
		m.notificationsModel, cmd = m.notificationsModel.Update(msg)
		cmds = append(cmds, cmd)

		// Only route to admin/relay/thread models when active (leak prevention)
		switch m.state {
		case common.AdminPanelView:
			m.adminModel, cmd = m.adminModel.Update(msg)
			cmds = append(cmds, cmd)
		case common.RelayManagementView:
			m.relayModel, cmd = m.relayModel.Update(msg)
			cmds = append(cmds, cmd)
		case common.ThreadView:
			m.threadViewModel, cmd = m.threadViewModel.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Route keyboard input ONLY to active model
	if _, ok := msg.(tea.KeyMsg); ok {
		switch m.state {
		case common.CreateUserView:
			m.newUserModel, cmd = m.newUserModel.Update(msg)
		case common.CreateNoteView:
			m.createModel, cmd = m.createModel.Update(msg)
		case common.HomeTimelineView:
			m.homeTimelineModel, cmd = m.homeTimelineModel.Update(msg)
		case common.MyPostsView:
			m.myPostsModel, cmd = m.myPostsModel.Update(msg)
		case common.FollowUserView:
			m.followModel, cmd = m.followModel.Update(msg)
		case common.FollowersView:
			m.followersModel, cmd = m.followersModel.Update(msg)
		case common.FollowingView:
			m.followingModel, cmd = m.followingModel.Update(msg)
		case common.LocalUsersView:
			m.localUsersModel, cmd = m.localUsersModel.Update(msg)
		case common.AdminPanelView:
			m.adminModel, cmd = m.adminModel.Update(msg)
		case common.RelayManagementView:
			m.relayModel, cmd = m.relayModel.Update(msg)
		case common.DeleteAccountView:
			m.deleteAccountModel, cmd = m.deleteAccountModel.Update(msg)
		case common.ThreadView:
			m.threadViewModel, cmd = m.threadViewModel.Update(msg)
		case common.NotificationsView:
			m.notificationsModel, cmd = m.notificationsModel.Update(msg)
		}
		cmds = append(cmds, cmd)
	}

	//  Filter out nil commands to minimize tea.Batch() goroutine accumulation
	var nonNilCmds []tea.Cmd
	for _, cmd := range cmds {
		if cmd != nil {
			nonNilCmds = append(nonNilCmds, cmd)
		}
	}

	// Handle command execution strategy to balance goroutine leak prevention
	// with proper initialization:
	// - For 0-1 commands: Execute directly without batching
	// - For 2+ commands: Use tea.Batch() only when necessary (view switches, etc.)
	// This is acceptable during transitions but avoided during normal operation
	switch len(nonNilCmds) {
	case 0:
		return m, nil
	case 1:
		return m, nonNilCmds[0]
	default:
		// Multiple commands - batch them
		// This happens during view initialization/switching which is infrequent
		return m, tea.Batch(nonNilCmds...)
	}
}

func (m MainModel) View() string {

	// Check minimum terminal size
	minWidth := 115
	minHeight := 28

	if m.width < minWidth || m.height < minHeight {
		message := fmt.Sprintf(
			"Terminal too small!\n\nMinimum required: %dx%d\nCurrent size: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(lipgloss.Color(common.COLOR_CRITICAL)).
			Bold(true).
			Render(message)
	}

	var s string

	model := m.currentFocusedModel()

	// Calculate responsive dimensions
	availableHeight := common.CalculateAvailableHeight(m.height)
	leftPanelWidth := common.TextInputDefaultWidth + 10 // Fixed width for left panel (textarea + padding)
	rightPanelWidth := common.CalculateRightPanelWidth(m.width, leftPanelWidth)

	createStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(leftPanelWidth).
		MaxWidth(leftPanelWidth).
		Margin(1).
		Render(m.createModel.View())

	homeTimelineStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.homeTimelineModel.View())

	myPostsStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.myPostsModel.View())

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

	localUsersStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.localUsersModel.View())

	adminStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.adminModel.View())

	relayStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.relayModel.View())

	deleteAccountStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.deleteAccountModel.View())

	threadViewStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.threadViewModel.View())

	notificationsStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.notificationsModel.View())

	if m.state == common.CreateUserView {
		s = m.newUserModel.ViewWithWidth(m.width, m.height)
		return s
	} else {
		// Update header with current unread notification count
		m.headerModel.UnreadCount = m.notificationsModel.UnreadCount
		navContainer := lipgloss.NewStyle().Render(m.headerModel.View())
		s += navContainer + "\n"

		// Render current view
		switch m.state {
		case common.CreateNoteView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				focusedModelStyle.Render(createStyleStr),
				modelStyle.Render(homeTimelineStyleStr))
		case common.HomeTimelineView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(homeTimelineStyleStr))
		case common.MyPostsView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(myPostsStyleStr))
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
		case common.LocalUsersView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(localUsersStyleStr))
		case common.AdminPanelView:
			// Always show both panels (write note + admin)
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(adminStyleStr))
		case common.RelayManagementView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(relayStyleStr))
		case common.DeleteAccountView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(deleteAccountStyleStr))
		case common.ThreadView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(threadViewStyleStr))
		case common.NotificationsView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(notificationsStyleStr))
		}

		// Help text
		var viewCommands string
		switch m.state {
		case common.HomeTimelineView:
			viewCommands = "↑/↓ • enter: thread • r: reply • l: ⭐ • i: info • o: link"
		case common.MyPostsView:
			viewCommands = "↑/↓ • u: edit • d: delete • l: ⭐"
		case common.FollowUserView:
			viewCommands = "enter: follow"
		case common.FollowersView:
			viewCommands = "↑/↓: scroll • f: follow back"
		case common.FollowingView:
			viewCommands = "↑/↓ • u/enter: unfollow"
		case common.LocalUsersView:
			viewCommands = "↑/↓ • enter: toggle follow"
		case common.AdminPanelView:
			// Context-aware help based on admin view state
			switch m.adminModel.CurrentView {
			case 0: // MenuView
				viewCommands = "↑/↓ • enter: select"
			case 1: // UsersView
				viewCommands = "↑/↓ • m: mute • K: kick • esc: back"
			case 2: // InfoBoxesView
				if m.adminModel.Editing {
					viewCommands = "tab/shift+tab: switch • ctrl+s: save • esc: cancel"
				} else {
					viewCommands = "↑/↓ • n: add • enter: edit • d: delete • t: toggle • esc: back"
				}
			default:
				viewCommands = "↑/↓ • enter: select"
			}
		case common.RelayManagementView:
			viewCommands = "↑/↓ • a: add • d: delete • r: retry"
		case common.DeleteAccountView:
			viewCommands = "y: confirm • n/esc: cancel"
		case common.ThreadView:
			viewCommands = "↑/↓ • enter: thread • r: reply • l: ⭐ • o: URL • esc: back"
		case common.NotificationsView:
			viewCommands = "j/k: nav • v: view • f: follow • enter: del • a: del all"
		default:
			viewCommands = " "
		}

		var helpText string
		if m.state == common.ThreadView {
			// Thread view doesn't use tab navigation
			helpText = fmt.Sprintf(
				"focused > %s\t\tkeys > %s • ctrl-c: exit",
				model, viewCommands)
		} else {
			helpText = fmt.Sprintf(
				"focused > %s\t\tkeys > tab: next • shift+tab: prev • %s • ctrl-c: exit",
				model, viewCommands)
		}

		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_HELP)).
			Width(m.width).
			Align(lipgloss.Center)

		// Calculate remaining vertical space and add it before footer
		// The panel takes availableHeight + margin (2), and we need space for footer (1)
		// Original: currentContentHeight = availableHeight + 2, remainingHeight = m.height - currentContentHeight - 1
		currentContentHeight := availableHeight + common.PanelMarginVertical
		remainingHeight := m.height - currentContentHeight - common.FooterHeight

		if remainingHeight > 0 {
			s += strings.Repeat("\n", remainingHeight)
		}

		s += helpStyle.Render(helpText)
		return lipgloss.NewStyle().Render(s)
	}
}

func (m MainModel) currentFocusedModel() string {
	switch m.state {
	case common.CreateNoteView:
		return "write"
	case common.HomeTimelineView:
		return "home"
	case common.MyPostsView:
		return "my posts"
	case common.FollowUserView:
		return "follow"
	case common.FollowersView:
		return "followers"
	case common.FollowingView:
		return "following"
	case common.LocalUsersView:
		return "users"
	case common.AdminPanelView:
		return "admin"
	case common.RelayManagementView:
		return "relays"
	case common.DeleteAccountView:
		return "delete"
	case common.ThreadView:
		return "thread"
	case common.NotificationsView:
		return "notifications"
	default:
		return "create user"
	}
}

// getViewInitCmd returns the init command for a view to reload its data
func getViewInitCmd(state common.SessionState, m *MainModel) tea.Cmd {
	switch state {
	case common.CreateNoteView:
		return m.createModel.Init()
	case common.HomeTimelineView:
		// Timeline Init() returns nil now, just send activation message
		return func() tea.Msg { return common.ActivateViewMsg{} }
	case common.MyPostsView:
		// Send activation message to reset scroll and reload data
		return func() tea.Msg { return common.ActivateViewMsg{} }
	case common.FollowersView:
		return m.followersModel.Init()
	case common.FollowingView:
		return m.followingModel.Init()
	case common.LocalUsersView:
		return m.localUsersModel.Init()
	case common.AdminPanelView:
		return m.adminModel.Init()
	case common.RelayManagementView:
		return m.relayModel.Init()
	case common.ThreadView:
		// Thread view activation message
		return func() tea.Msg { return common.ActivateViewMsg{} }
	case common.NotificationsView:
		// Notifications view activation message
		return func() tea.Msg { return common.ActivateViewMsg{} }
	default:
		return nil
	}
}

// likeNoteCmd handles liking/unliking a note
func likeNoteCmd(accountId uuid.UUID, noteURI string, noteID uuid.UUID, isLocal bool, account *domain.Account) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Determine the actual note ID and URI to use
		var actualNoteID uuid.UUID
		var actualNoteURI string
		var isRemotePost bool

		if isLocal && noteID != uuid.Nil {
			actualNoteID = noteID
			// Get the note's ObjectURI for federation
			err, note := database.ReadNoteId(noteID)
			if err != nil {
				log.Printf("Failed to read note for like: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		} else if noteURI != "" && !strings.HasPrefix(noteURI, "local:") {
			// Remote note - find it by ObjectURI
			err, activity := database.ReadActivityByObjectURI(noteURI)
			if err != nil || activity == nil {
				log.Printf("Failed to find activity for like: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = noteURI
			isRemotePost = true
			// Try to find a local note with this URI (federated back)
			err, localNote := database.ReadNoteByURI(noteURI)
			if err == nil && localNote != nil {
				actualNoteID = localNote.Id
				isRemotePost = false // It's actually a local post that was federated back
			}
		} else if strings.HasPrefix(noteURI, "local:") {
			// Parse local: prefix
			idStr := strings.TrimPrefix(noteURI, "local:")
			parsedID, err := uuid.Parse(idStr)
			if err != nil {
				log.Printf("Failed to parse local note ID: %v", err)
				return common.UpdateNoteList
			}
			actualNoteID = parsedID
			// Get the note's ObjectURI for federation
			err, note := database.ReadNoteId(parsedID)
			if err != nil {
				log.Printf("Failed to read note for like: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		}

		// Check if we already liked this post
		var hasLike bool
		var err error
		if isRemotePost {
			hasLike, err = database.HasLikeByObjectURI(accountId, actualNoteURI)
		} else {
			hasLike, err = database.HasLike(accountId, actualNoteID)
		}
		if err != nil {
			log.Printf("Failed to check existing like: %v", err)
			return common.UpdateNoteList
		}

		if hasLike {
			// Unlike - remove the like
			var existingLike *domain.Like
			if isRemotePost {
				err, existingLike = database.ReadLikeByAccountAndObjectURI(accountId, actualNoteURI)
			} else {
				err, existingLike = database.ReadLikeByAccountAndNote(accountId, actualNoteID)
			}
			if err != nil {
				log.Printf("Failed to read existing like: %v", err)
				return common.UpdateNoteList
			}

			// Delete the like
			if isRemotePost {
				if err := database.DeleteLikeByAccountAndObjectURI(accountId, actualNoteURI); err != nil {
					log.Printf("Failed to delete like: %v", err)
					return common.UpdateNoteList
				}
				// Decrement like count on the activity
				if err := database.DecrementLikeCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to decrement activity like count: %v", err)
				}
			} else {
				if err := database.DeleteLikeByAccountAndNote(accountId, actualNoteID); err != nil {
					log.Printf("Failed to delete like: %v", err)
					return common.UpdateNoteList
				}
				// Decrement like count on the note
				if err := database.DecrementLikeCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to decrement like count: %v", err)
				}
			}

			log.Printf("Unliked post %s", actualNoteURI)

			// Send Undo Like to remote server (background task)
			if actualNoteURI != "" && existingLike != nil {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for unlike federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendUndoLike(account, actualNoteURI, existingLike.URI, conf); err != nil {
						log.Printf("Failed to federate unlike: %v", err)
					} else {
						log.Printf("Unlike federated successfully")
					}
				}()
			}
		} else {
			// Like - create a new like
			likeURI := ""
			conf, err := util.ReadConf()
			if err == nil && conf.Conf.WithAp {
				likeURI = fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
			}

			like := &domain.Like{
				Id:        uuid.New(),
				AccountId: accountId,
				NoteId:    actualNoteID, // Will be uuid.Nil for remote posts
				URI:       likeURI,
				CreatedAt: time.Now(),
			}

			// Create the like
			if isRemotePost {
				if err := database.CreateLikeByObjectURI(like, actualNoteURI); err != nil {
					log.Printf("Failed to create like: %v", err)
					return common.UpdateNoteList
				}
				// Increment like count on the activity
				if err := database.IncrementLikeCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to increment activity like count: %v", err)
				}
			} else {
				if err := database.CreateLike(like); err != nil {
					log.Printf("Failed to create like: %v", err)
					return common.UpdateNoteList
				}
				// Increment like count on the note
				if err := database.IncrementLikeCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to increment like count: %v", err)
				}

				// Create notification for local note author
				err, note := database.ReadNoteId(actualNoteID)
				if err == nil && note != nil {
					err, noteAuthor := database.ReadAccByUsername(note.CreatedBy)
					if err == nil && noteAuthor != nil && noteAuthor.Id != accountId {
						// Only notify if liker is not the author
						preview := note.Message
						if len(preview) > 100 {
							preview = preview[:100] + "..."
						}
						notification := &domain.Notification{
							Id:               uuid.New(),
							AccountId:        noteAuthor.Id,
							NotificationType: domain.NotificationLike,
							ActorId:          accountId,
							ActorUsername:    account.Username,
							ActorDomain:      "", // Empty for local users
							NoteId:           note.Id,
							NoteURI:          note.ObjectURI,
							NotePreview:      preview,
							Read:             false,
							CreatedAt:        time.Now(),
						}
						if err := database.CreateNotification(notification); err != nil {
							log.Printf("Failed to create like notification: %v", err)
						}
					}
				}
			}

			log.Printf("Liked post %s", actualNoteURI)

			// Send Like to remote server (background task)
			if actualNoteURI != "" {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for like federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendLike(account, actualNoteURI, conf); err != nil {
						log.Printf("Failed to federate like: %v", err)
					} else {
						log.Printf("Like federated successfully")
					}
				}()
			}
		}

		return common.UpdateNoteList
	}
}
