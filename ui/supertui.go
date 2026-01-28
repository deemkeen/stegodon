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
	"github.com/deemkeen/stegodon/ui/accountsettings"
	"github.com/deemkeen/stegodon/ui/admin"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/ui/createuser"
	"github.com/deemkeen/stegodon/ui/followers"
	"github.com/deemkeen/stegodon/ui/following"
	"github.com/deemkeen/stegodon/ui/followuser"
	"github.com/deemkeen/stegodon/ui/globalposts"
	"github.com/deemkeen/stegodon/ui/header"
	"github.com/deemkeen/stegodon/ui/hometimeline"
	"github.com/deemkeen/stegodon/ui/localusers"
	"github.com/deemkeen/stegodon/ui/myposts"
	"github.com/deemkeen/stegodon/ui/notifications"
	"github.com/deemkeen/stegodon/ui/profileview"
	"github.com/deemkeen/stegodon/ui/relay"
	"github.com/deemkeen/stegodon/ui/terms"
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
	width                int
	height               int
	config               *util.AppConfig
	headerModel          header.Model
	account              domain.Account
	state                common.SessionState
	newUserModel         createuser.Model
	termsModel           terms.Model
	createModel          writenote.Model
	myPostsModel         myposts.Model
	globalPostsModel     globalposts.Model
	followModel          followuser.Model
	followersModel       followers.Model
	followingModel       following.Model
	homeTimelineModel    hometimeline.Model
	localUsersModel      localusers.Model
	adminModel           admin.Model
	relayModel           relay.Model
	accountSettingsModel accountsettings.Model
	threadViewModel      threadview.Model
	profileViewModel     profileview.Model
	notificationsModel   notifications.Model
}

type userUpdateErrorMsg struct {
	err error
}

type termsCheckResultMsg struct {
	needsAcceptance bool
}

func checkTermsAcceptanceCmd(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		needs, err := db.GetDB().UserNeedsToAcceptTerms(userId)
		if err != nil {
			log.Printf("Failed to check terms acceptance: %v", err)
			return termsCheckResultMsg{needsAcceptance: false}
		}
		return termsCheckResultMsg{needsAcceptance: needs}
	}
}

type termsLoadedMsg struct {
	content string
}

func loadTermsCmd() tea.Cmd {
	return func() tea.Msg {
		err, terms := db.GetDB().GetCurrentTermsAndConditions()
		if err != nil {
			log.Printf("Failed to load terms: %v", err)
			return termsLoadedMsg{content: "Terms and conditions could not be loaded."}
		}
		return termsLoadedMsg{content: terms.Content}
	}
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
	globalPostsModel := globalposts.InitialModel(acc.Id, width, height, localDomain)
	followModel := followuser.InitialModel(acc.Id)
	followersModel := followers.InitialModel(acc.Id, width, height)
	followingModel := following.InitialModel(acc.Id, width, height)
	homeTimelineModel := hometimeline.InitialModel(acc.Id, width, height, localDomain)
	localUsersModel := localusers.InitialModel(acc.Id, width, height)
	adminModel := admin.InitialModel(acc.Id, width, height)
	relayModel := relay.InitialModel(acc.Id, &acc, config, width, height)
	accountSettingsModel := accountsettings.InitialModel(&acc)
	threadViewModel := threadview.InitialModel(acc.Id, width, height, localDomain)
	profileViewModel := profileview.InitialModel(acc.Id, width, height, localDomain)
	notificationsModel := notifications.InitialModel(acc.Id, width, height)

	m := MainModel{state: common.CreateUserView}
	m.config = config
	m.newUserModel = createuser.InitialModel()
	m.termsModel = terms.InitialModel(acc.Id)
	m.createModel = noteModel
	m.myPostsModel = myPostsModel
	m.globalPostsModel = globalPostsModel
	m.followModel = followModel
	m.followersModel = followersModel
	m.followingModel = followingModel
	m.homeTimelineModel = homeTimelineModel
	m.localUsersModel = localUsersModel
	m.adminModel = adminModel
	m.relayModel = relayModel
	m.accountSettingsModel = accountSettingsModel
	m.threadViewModel = threadViewModel
	m.profileViewModel = profileViewModel
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
		if m.config.Conf.ShowTos {
			// New users must accept terms first, then create profile
			cmds = append(cmds, func() tea.Msg {
				return common.TermsAcceptanceView
			})
			cmds = append(cmds, loadTermsCmd())
		} else {
			// Terms disabled, go directly to profile creation
			cmds = append(cmds, func() tea.Msg {
				return common.CreateUserView
			})
		}
	} else {
		if m.config.Conf.ShowTos {
			// For existing users, check if they need to accept terms first
			// The termsCheckResultMsg handler will transition to the appropriate state
			cmds = append(cmds, checkTermsAcceptanceCmd(m.account.Id))
		} else {
			// Terms disabled, go directly to main app
			cmds = append(cmds, func() tea.Msg {
				return common.CreateNoteView
			})
			cmds = append(cmds, m.createModel.Init())
		}
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

	case termsCheckResultMsg:
		// Handle terms acceptance check for existing users
		if msg.needsAcceptance {
			m.state = common.TermsAcceptanceView
			return m, loadTermsCmd()
		}
		// No terms acceptance needed, proceed to normal view
		m.state = common.CreateNoteView
		return m, m.createModel.Init()

	case termsLoadedMsg:
		// Update terms model with loaded content
		m.termsModel.TermsContent = msg.content
		return m, nil

	case tea.WindowSizeMsg:
		// Handle window resize - update all models that use width/height for layout
		m.width = msg.Width
		m.height = msg.Height
		m.headerModel.Width = msg.Width
		m.myPostsModel.Width = msg.Width
		m.myPostsModel.Height = msg.Height
		m.globalPostsModel.Width = msg.Width
		m.globalPostsModel.Height = msg.Height
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
		m.profileViewModel.Width = msg.Width
		m.profileViewModel.Height = msg.Height
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
		case common.AccountSettingsView:
			m.state = common.AccountSettingsView
		case common.ThreadView:
			m.state = common.ThreadView
		case common.ProfileView:
			m.state = common.ProfileView
		case common.TermsAcceptanceView:
			m.state = common.TermsAcceptanceView
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
		// Route ReplyToNote message to writenote model
		m.createModel, cmd = m.createModel.Update(msg)
		// Keep thread/profile visible in right panel during reply
		if m.state != common.ThreadView && m.state != common.ProfileView {
			m.state = common.CreateNoteView
		}
		return m, cmd

	case common.ViewThreadMsg:
		// Set return view based on where the thread was opened from
		if m.state == common.ProfileView {
			m.threadViewModel.ReturnView = common.ProfileView
		} else {
			m.threadViewModel.ReturnView = common.HomeTimelineView
		}
		// Route ViewThread message to threadview model and switch to ThreadView
		m.threadViewModel, cmd = m.threadViewModel.Update(msg)
		m.state = common.ThreadView
		return m, cmd

	case common.ViewProfileMsg:
		// Route ViewProfile message to profileview model and switch to ProfileView
		m.profileViewModel, cmd = m.profileViewModel.Update(msg)
		m.state = common.ProfileView
		return m, cmd

	case common.LikeNoteMsg:
		// Handle like/unlike
		return m, likeNoteCmd(m.account.Id, msg.NoteURI, msg.NoteID, msg.IsLocal, &m.account)

	case common.BoostNoteMsg:
		// Handle boost/unboost
		return m, boostNoteCmd(m.account.Id, msg.NoteURI, msg.NoteID, msg.IsLocal, &m.account)

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

				// Deactivate accountsettings if we were in it (stops avatar polling)
				if oldState == common.AccountSettingsView {
					cmds = append(cmds, func() tea.Msg { return common.DeactivateAccountSettingsMsg{} })
				}

				// Note: No need to activate notifications - it's always active
			}
		case "tab":
			// Cycle through main views (excluding create user)
			// Order: write -> home -> my posts -> [global posts] -> [follow] -> followers -> following -> users -> [admin -> relay] -> delete
			// AP-only views: follow remote user, relay management
			// Optional views: global posts (when ShowGlobal is enabled)
			if m.state == common.CreateUserView || m.state == common.TermsAcceptanceView {
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
				if m.config.Conf.ShowGlobal {
					m.state = common.GlobalPostsView
				} else if m.config.Conf.WithAp {
					m.state = common.FollowUserView
				} else {
					m.state = common.FollowersView
				}
			case common.GlobalPostsView:
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
					m.state = common.AccountSettingsView
				}
			case common.AdminPanelView:
				if m.config.Conf.WithAp {
					m.state = common.RelayManagementView
				} else {
					m.state = common.AccountSettingsView
				}
			case common.RelayManagementView:
				m.state = common.AccountSettingsView
			case common.AccountSettingsView:
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

			// Manage accountsettings activation based on visibility (for avatar polling)
			// Uses specific messages to avoid conflicts with timeline activation
			oldAccountSettingsVisible := (oldState == common.AccountSettingsView)
			newAccountSettingsVisible := (m.state == common.AccountSettingsView)

			if !oldAccountSettingsVisible && newAccountSettingsVisible {
				// AccountSettings becoming visible, activate it
				cmds = append(cmds, func() tea.Msg { return common.ActivateAccountSettingsMsg{} })
			} else if oldAccountSettingsVisible && !newAccountSettingsVisible {
				// AccountSettings becoming hidden, deactivate it
				cmds = append(cmds, func() tea.Msg { return common.DeactivateAccountSettingsMsg{} })
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
			// Optional views: global posts (when ShowGlobal is enabled)
			if m.state == common.CreateUserView || m.state == common.TermsAcceptanceView {
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
				m.state = common.AccountSettingsView
			case common.HomeTimelineView:
				m.state = common.CreateNoteView
			case common.MyPostsView:
				m.state = common.HomeTimelineView
			case common.GlobalPostsView:
				m.state = common.MyPostsView
			case common.FollowUserView:
				if m.config.Conf.ShowGlobal {
					m.state = common.GlobalPostsView
				} else {
					m.state = common.MyPostsView
				}
			case common.FollowersView:
				if m.config.Conf.WithAp {
					m.state = common.FollowUserView
				} else if m.config.Conf.ShowGlobal {
					m.state = common.GlobalPostsView
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
			case common.AccountSettingsView:
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

			// Manage accountsettings activation based on visibility (for avatar polling)
			// Uses specific messages to avoid conflicts with timeline activation
			oldAccountSettingsVisible := (oldState == common.AccountSettingsView)
			newAccountSettingsVisible := (m.state == common.AccountSettingsView)

			if !oldAccountSettingsVisible && newAccountSettingsVisible {
				// AccountSettings becoming visible, activate it
				cmds = append(cmds, func() tea.Msg { return common.ActivateAccountSettingsMsg{} })
			} else if oldAccountSettingsVisible && !newAccountSettingsVisible {
				// AccountSettings becoming hidden, deactivate it
				cmds = append(cmds, func() tea.Msg { return common.DeactivateAccountSettingsMsg{} })
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
				// Step 2 (bio) complete - save user and go to main app
				m.state = common.CreateNoteView
				m.account.Username = m.newUserModel.TextInput.Value()
				m.account.DisplayName = m.newUserModel.DisplayName.Value()
				m.account.Summary = m.newUserModel.Bio.Value()

				// Use username as display name if not provided
				if m.account.DisplayName == "" {
					m.account.DisplayName = m.account.Username
				}

				m.headerModel = header.Model{Width: m.width, Acc: &m.account}
				m.accountSettingsModel.Account = &m.account
				m.relayModel.AdminAcct = &m.account
				return m, updateUserModelCmd(&m.account)
			}
		}
	}

	// Route specific message types to appropriate models
	// This is more efficient than routing ALL messages to ALL models
	switch msg.(type) {
	case common.ActivateViewMsg, common.DeactivateViewMsg:
		// Activation/deactivation messages go to home timeline, myposts, globalposts, and notifications models
		// Note: accountsettings uses its own specific messages to avoid conflicts
		m.homeTimelineModel, cmd = m.homeTimelineModel.Update(msg)
		cmds = append(cmds, cmd)
		m.myPostsModel, cmd = m.myPostsModel.Update(msg)
		cmds = append(cmds, cmd)
		m.globalPostsModel, cmd = m.globalPostsModel.Update(msg)
		cmds = append(cmds, cmd)
		m.notificationsModel, cmd = m.notificationsModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.ActivateAccountSettingsMsg, common.DeactivateAccountSettingsMsg:
		// Accountsettings-specific activation messages (for avatar polling)
		m.accountSettingsModel, cmd = m.accountSettingsModel.Update(msg)
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
		// NOTE: accountsettings is NOT included here - it's only routed in the
		// state-based routing below to prevent double-calling Update() which
		// would cause exponential goroutine growth during avatar polling
		m.myPostsModel, cmd = m.myPostsModel.Update(msg)
		cmds = append(cmds, cmd)
		m.createModel, cmd = m.createModel.Update(msg)
		cmds = append(cmds, cmd)
		m.followModel, cmd = m.followModel.Update(msg)
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

		// Only route to admin/relay/thread/profile models when active (leak prevention)
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
		case common.ProfileView:
			m.profileViewModel, cmd = m.profileViewModel.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Route messages to active model
	// This includes keyboard input AND internal messages (like postsLoadedMsg from async commands)
	switch m.state {
	case common.CreateUserView:
		m.newUserModel, cmd = m.newUserModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.TermsAcceptanceView:
		// Route to terms model
		m.termsModel, cmd = m.termsModel.Update(msg)
		cmds = append(cmds, cmd)

		// Handle terms acceptance
		if _, ok := msg.(terms.TermsAcceptedMsg); ok {
			if m.account.FirstTimeLogin == domain.TRUE {
				// New user: go to profile creation (username/display name/bio)
				m.state = common.CreateUserView
				return m, m.newUserModel.Init()
			}
			// Existing user: go to main app
			m.state = common.CreateNoteView
			return m, m.createModel.Init()
		}
	case common.CreateNoteView:
		m.createModel, cmd = m.createModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.HomeTimelineView:
		m.homeTimelineModel, cmd = m.homeTimelineModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.MyPostsView:
		m.myPostsModel, cmd = m.myPostsModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.GlobalPostsView:
		m.globalPostsModel, cmd = m.globalPostsModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.FollowUserView:
		m.followModel, cmd = m.followModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.FollowersView:
		m.followersModel, cmd = m.followersModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.FollowingView:
		m.followingModel, cmd = m.followingModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.LocalUsersView:
		m.localUsersModel, cmd = m.localUsersModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.AdminPanelView:
		m.adminModel, cmd = m.adminModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.RelayManagementView:
		m.relayModel, cmd = m.relayModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.AccountSettingsView:
		m.accountSettingsModel, cmd = m.accountSettingsModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.ThreadView:
		if m.createModel.IsReplying() {
			m.createModel, cmd = m.createModel.Update(msg)
		} else {
			m.threadViewModel, cmd = m.threadViewModel.Update(msg)
		}
		cmds = append(cmds, cmd)
	case common.ProfileView:
		m.profileViewModel, cmd = m.profileViewModel.Update(msg)
		cmds = append(cmds, cmd)
	case common.NotificationsView:
		m.notificationsModel, cmd = m.notificationsModel.Update(msg)
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

	globalPostsStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.globalPostsModel.View())

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

	accountSettingsStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.accountSettingsModel.View())

	threadViewStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.threadViewModel.View())

	profileViewStyleStr := lipgloss.NewStyle().
		MaxHeight(availableHeight).
		Height(availableHeight).
		Width(rightPanelWidth).
		MaxWidth(rightPanelWidth).
		Margin(1).
		Render(m.profileViewModel.View())

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
	} else if m.state == common.TermsAcceptanceView {
		// Show terms acceptance (new users see this first, existing users if terms updated)
		s = m.termsModel.ViewWithWidth(m.width, m.height)
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
		case common.GlobalPostsView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(globalPostsStyleStr))
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
		case common.AccountSettingsView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(accountSettingsStyleStr))
		case common.ThreadView:
			if m.createModel.IsReplying() {
				s += lipgloss.JoinHorizontal(lipgloss.Top,
					focusedModelStyle.Render(createStyleStr),
					modelStyle.Render(threadViewStyleStr))
			} else {
				s += lipgloss.JoinHorizontal(lipgloss.Top,
					modelStyle.Render(createStyleStr),
					focusedModelStyle.Render(threadViewStyleStr))
			}
		case common.ProfileView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(profileViewStyleStr))
		case common.NotificationsView:
			s += lipgloss.JoinHorizontal(lipgloss.Top,
				modelStyle.Render(createStyleStr),
				focusedModelStyle.Render(notificationsStyleStr))
		}

		// Help text
		var viewCommands string
		switch m.state {
		case common.HomeTimelineView:
			viewCommands = "↑/↓ • enter: thread • r: reply • l: ⭐ • b: 🔁 • i: info • o: link"
		case common.MyPostsView:
			viewCommands = "↑/↓ • u: edit • d: delete • l: ⭐ • b: 🔁"
		case common.GlobalPostsView:
			viewCommands = "↑/↓ • enter: thread • r: reply • l: ⭐ • b: 🔁 • i: info • o: link • f: follow"
		case common.FollowUserView:
			viewCommands = "enter: follow"
		case common.FollowersView:
			viewCommands = "↑/↓: scroll • f: follow back"
		case common.FollowingView:
			viewCommands = "↑/↓ • u/enter: unfollow"
		case common.LocalUsersView:
			viewCommands = "↑/↓ • enter: profile • f: follow"
		case common.AdminPanelView:
			// Context-aware help based on admin view state
			switch m.adminModel.CurrentView {
			case 0: // MenuView
				viewCommands = "↑/↓ • enter: select"
			case 1: // UsersView
				viewCommands = "↑/↓ • m: mute • B: ban • U: unban • esc: back"
			case 2: // InfoBoxesView
				if m.adminModel.Editing {
					viewCommands = "tab/shift+tab: switch • ctrl+s: save • esc: cancel"
				} else {
					viewCommands = "↑/↓ • n: add • enter: edit • d: delete • t: toggle • esc: back"
				}
			case 3: // ServerMessageView
				viewCommands = "e: edit • esc: back"
			case 4: // BansView
				viewCommands = "↑/↓ • u: unban • esc: back"
			default:
				viewCommands = "↑/↓ • enter: select"
			}
		case common.RelayManagementView:
			viewCommands = "↑/↓ • a: add • d: delete • r: retry"
		case common.AccountSettingsView:
			viewCommands = "↑/↓ • e: name • b: bio • a: avatar • d: delete"
		case common.ThreadView:
			viewCommands = "↑/↓ • enter: thread • r: reply • l: ⭐ • b: 🔁 • o: URL • esc: back"
		case common.ProfileView:
			viewCommands = "↑/↓ • enter: thread • f: follow • esc: back"
		case common.NotificationsView:
			viewCommands = "j/k: nav • v: view • f: follow • enter: del • a: del all"
		default:
			viewCommands = " "
		}

		var helpText string
		if m.state == common.ThreadView || m.state == common.ProfileView {
			// Thread and profile views don't use tab navigation
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
	case common.GlobalPostsView:
		return "global"
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
	case common.AccountSettingsView:
		return "settings"
	case common.ThreadView:
		return "thread"
	case common.ProfileView:
		return "profile"
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
	case common.GlobalPostsView:
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

// boostNoteCmd handles boosting/unboosting a note
func boostNoteCmd(accountId uuid.UUID, noteURI string, noteID uuid.UUID, isLocal bool, account *domain.Account) tea.Cmd {
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
				log.Printf("Failed to read note for boost: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		} else if noteURI != "" && !strings.HasPrefix(noteURI, "local:") {
			// Remote note - find it by ObjectURI
			err, activity := database.ReadActivityByObjectURI(noteURI)
			if err != nil || activity == nil {
				log.Printf("Failed to find activity for boost: %v", err)
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
				log.Printf("Failed to read note for boost: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		}

		// Check if we already boosted this post
		var hasBoost bool
		var err error
		if isRemotePost {
			hasBoost, err = database.HasBoostByObjectURI(accountId, actualNoteURI)
		} else {
			hasBoost, err = database.HasBoost(accountId, actualNoteID)
		}
		if err != nil {
			log.Printf("Failed to check existing boost: %v", err)
			return common.UpdateNoteList
		}

		if hasBoost {
			// Unboost - remove the boost
			var existingBoost *domain.Boost
			if isRemotePost {
				err, existingBoost = database.ReadBoostByAccountAndObjectURI(accountId, actualNoteURI)
			} else {
				err, existingBoost = database.ReadBoostByAccountAndNote(accountId, actualNoteID)
			}
			if err != nil {
				log.Printf("Failed to read existing boost: %v", err)
				return common.UpdateNoteList
			}

			// Delete the boost
			if isRemotePost {
				if err := database.DeleteBoostByAccountAndObjectURI(accountId, actualNoteURI); err != nil {
					log.Printf("Failed to delete boost: %v", err)
					return common.UpdateNoteList
				}
				// Decrement boost count on the activity
				if err := database.DecrementBoostCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to decrement activity boost count: %v", err)
				}
			} else {
				if err := database.DeleteBoostByAccountAndNote(accountId, actualNoteID); err != nil {
					log.Printf("Failed to delete boost: %v", err)
					return common.UpdateNoteList
				}
				// Decrement boost count on the note
				if err := database.DecrementBoostCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to decrement boost count: %v", err)
				}
			}

			log.Printf("Unboosted post %s", actualNoteURI)

			// Send Undo Announce to remote server (background task)
			if actualNoteURI != "" && existingBoost != nil {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for unboost federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendUndoAnnounce(account, actualNoteURI, existingBoost.URI, conf); err != nil {
						log.Printf("Failed to federate unboost: %v", err)
					} else {
						log.Printf("Unboost federated successfully")
					}
				}()
			}
		} else {
			// Boost - create a new boost
			boostURI := ""
			conf, err := util.ReadConf()
			if err == nil && conf.Conf.WithAp {
				boostURI = fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
			}

			boost := &domain.Boost{
				Id:        uuid.New(),
				AccountId: accountId,
				NoteId:    actualNoteID, // Will be uuid.Nil for remote posts
				URI:       boostURI,
				CreatedAt: time.Now(),
			}

			// Create the boost
			if isRemotePost {
				if err := database.CreateBoostByObjectURI(boost, actualNoteURI); err != nil {
					log.Printf("Failed to create boost: %v", err)
					return common.UpdateNoteList
				}
				// Increment boost count on the activity
				if err := database.IncrementBoostCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to increment activity boost count: %v", err)
				}
			} else {
				if err := database.CreateBoost(boost); err != nil {
					log.Printf("Failed to create boost: %v", err)
					return common.UpdateNoteList
				}
				// Increment boost count on the note
				if err := database.IncrementBoostCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to increment boost count: %v", err)
				}

				// Create notification for local note author
				err, note := database.ReadNoteId(actualNoteID)
				if err == nil && note != nil {
					err, noteAuthor := database.ReadAccByUsername(note.CreatedBy)
					if err == nil && noteAuthor != nil && noteAuthor.Id != accountId {
						// Only notify if booster is not the author
						preview := note.Message
						if len(preview) > 100 {
							preview = preview[:100] + "..."
						}
						notification := &domain.Notification{
							Id:               uuid.New(),
							AccountId:        noteAuthor.Id,
							NotificationType: domain.NotificationBoost,
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
							log.Printf("Failed to create boost notification: %v", err)
						}
					}
				}
			}

			log.Printf("Boosted post %s", actualNoteURI)

			// Send Announce to remote server (background task)
			if actualNoteURI != "" {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for boost federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendAnnounce(account, actualNoteURI, boostURI, conf); err != nil {
						log.Printf("Failed to federate boost: %v", err)
					} else {
						log.Printf("Boost federated successfully")
					}
				}()
			}
		}

		return common.UpdateNoteList
	}
}
