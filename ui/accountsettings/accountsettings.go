package accountsettings

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"log"
)

var (
	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SECONDARY))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_ACCENT)).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_ERROR)).
			Bold(true)

	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SECONDARY))

	instructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_DIM))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SUCCESS))

	linkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_LINK)).
			Underline(true)
)

// ViewState represents the current sub-view
type ViewState int

const (
	MenuView ViewState = iota
	EditDisplayNameView
	EditBioView
	AvatarView
	DeleteView
)

// MenuItem represents a menu option
type MenuItem int

const (
	MenuEditDisplayName MenuItem = iota
	MenuEditBio
	MenuChangeAvatar
	MenuDeleteAccount
)

const (
	avatarCols = 12
	avatarRows = 6
)

type Model struct {
	Account        *domain.Account
	ViewState      ViewState
	MenuItem       MenuItem
	ConfirmStep    int // For delete: 0 = initial, 1 = first confirmation, 2 = final
	Status         string
	Error          string
	DeletionStatus string
	ShowByeBye     bool
	Width          int

	// Text inputs for editing
	displayNameInput textinput.Model
	bioInput         textinput.Model

	// Avatar upload
	uploadToken       string
	uploadURL         string
	uploadExpiresAt   time.Time // When the upload token expires
	originalAvatarURL string    // Track original to detect changes
	isPolling         bool      // Whether we're polling for avatar changes
	isActive          bool      // Track if view is visible to prevent goroutine leaks

	// Config for URLs
	conf *util.AppConfig

	// Pre-rendered avatar for display
	avatarRendered string
}

func InitialModel(account *domain.Account) Model {
	// Display name input
	dnInput := textinput.New()
	dnInput.Placeholder = "Display name"
	dnInput.CharLimit = 50
	dnInput.SetWidth(50)
	dnInput.SetValue(account.DisplayName)

	// Bio input
	bioInput := textinput.New()
	bioInput.Placeholder = "Bio/summary"
	bioInput.CharLimit = 200
	bioInput.SetWidth(60)
	bioInput.SetValue(account.Summary)

	conf, _ := util.ReadConf()

	img := util.LoadAvatarImage(account.AvatarURL)
	if img == nil {
		img = util.LoadDefaultAvatarImage()
	}
	var avatarStr string
	if img != nil {
		avatarStr = util.RenderImageToHalfBlocks(img, avatarCols, avatarRows)
	}

	return Model{
		Account:          account,
		ViewState:        MenuView,
		MenuItem:         MenuEditDisplayName,
		ConfirmStep:      0,
		Status:           "",
		Error:            "",
		displayNameInput: dnInput,
		bioInput:         bioInput,
		conf:             conf,
		avatarRendered:   avatarStr,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case common.ActivateAccountSettingsMsg:
		m.isActive = true
		return m, nil

	case common.DeactivateAccountSettingsMsg:
		m.isActive = false
		m.isPolling = false // Stop polling when view is deactivated
		return m, nil

	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case showByeByeMsg:
		m.ShowByeBye = true
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tea.Quit()
		})

	case deleteAccountResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to delete account: %v", msg.err)
			m.ConfirmStep = 0
		} else {
			m.DeletionStatus = "completed"
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return showByeByeMsg{}
			})
		}
		return m, nil

	case updateProfileResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to update: %v", msg.err)
		} else {
			m.Status = "Updated successfully!"
			// Update local account data
			if msg.field == "displayName" {
				m.Account.DisplayName = msg.value
			} else if msg.field == "bio" {
				m.Account.Summary = msg.value
			}
			m.ViewState = MenuView
		}
		return m, clearStatusAfter(3 * time.Second)

	case uploadTokenResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to create upload link: %v", msg.err)
			return m, nil
		}

		m.uploadToken = msg.token
		m.uploadExpiresAt = msg.expiresAt
		m.originalAvatarURL = m.Account.AvatarURL // Track current avatar to detect changes
		m.isPolling = true                        // Start polling for changes
		if m.conf != nil && m.conf.Conf.SslDomain != "" {
			m.uploadURL = fmt.Sprintf("https://%s/upload/%s", m.conf.Conf.SslDomain, msg.token)
		} else {
			m.uploadURL = fmt.Sprintf("http://localhost:%d/upload/%s", m.conf.Conf.HttpPort, msg.token)
		}

		if !msg.isNew {
			m.Status = "Using existing upload link"
		}
		// Just start polling, skip the status clear (it's not critical)
		return m, avatarPollTickCmd()

	case refreshAccountResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to refresh account: %v", msg.err)
		} else {
			m.Account = msg.account
			// Update text inputs with new values
			m.displayNameInput.SetValue(msg.account.DisplayName)
			m.bioInput.SetValue(msg.account.Summary)
			// Re-render avatar
			img := util.LoadAvatarImage(msg.account.AvatarURL)
			if img == nil {
				img = util.LoadDefaultAvatarImage()
			}
			m.avatarRendered = ""
			if img != nil {
				m.avatarRendered = util.RenderImageToHalfBlocks(img, avatarCols, avatarRows)
			}
		}
		// Always clear status after refresh completes (handles both manual refresh and post-upload)
		return m, clearStatusAfter(5 * time.Second)

	case avatarPollTickMsg:
		// Only poll if view is active AND in avatar view AND polling is enabled
		if m.isActive && m.ViewState == AvatarView && m.isPolling && m.uploadToken != "" {
			return m, checkTokenExistsCmd(m.uploadToken)
		}
		return m, nil // Stop ticker chain when not active

	case checkTokenResultMsg:
		if msg.err != nil {
			// Error checking token, continue polling only if still active
			if m.isActive && m.isPolling {
				return m, avatarPollTickCmd()
			}
			return m, nil
		}
		if !msg.tokenExists && m.isPolling {
			// Token was consumed - upload completed!
			m.isPolling = false
			m.uploadToken = ""
			m.uploadURL = ""
			m.Status = "File successfully uploaded!"
			// Refresh account to get new avatar URL (status will be cleared in the result handler)
			return m, refreshAccountCmd(m.Account.Id)
		}
		// Token still exists, continue polling only if still active
		if m.isActive && m.isPolling {
			return m, avatarPollTickCmd()
		}
		return m, nil

	case tea.KeyPressMsg:
		switch m.ViewState {
		case MenuView:
			return m.updateMenu(msg)
		case EditDisplayNameView:
			return m.updateEditDisplayName(msg)
		case EditBioView:
			return m.updateEditBio(msg)
		case AvatarView:
			return m.updateAvatar(msg)
		case DeleteView:
			return m.updateDelete(msg)
		}
	}

	return m, cmd
}

func (m Model) updateMenu(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.MenuItem > MenuEditDisplayName {
			m.MenuItem--
		}
	case "down", "j":
		if m.MenuItem < MenuDeleteAccount {
			m.MenuItem++
		}
	case "enter":
		switch m.MenuItem {
		case MenuEditDisplayName:
			m.ViewState = EditDisplayNameView
			m.displayNameInput.Focus()
			return m, textinput.Blink
		case MenuEditBio:
			m.ViewState = EditBioView
			m.bioInput.Focus()
			return m, textinput.Blink
		case MenuChangeAvatar:
			m.ViewState = AvatarView
			m.uploadToken = ""
			m.uploadURL = ""
			return m, nil
		case MenuDeleteAccount:
			m.ViewState = DeleteView
			m.ConfirmStep = 0
			return m, nil
		}
	case "e":
		m.ViewState = EditDisplayNameView
		m.displayNameInput.Focus()
		return m, textinput.Blink
	case "b":
		m.ViewState = EditBioView
		m.bioInput.Focus()
		return m, textinput.Blink
	case "a":
		m.ViewState = AvatarView
		m.uploadToken = ""
		m.uploadURL = ""
		return m, nil
	case "d":
		m.ViewState = DeleteView
		m.ConfirmStep = 0
		return m, nil
	}
	return m, nil
}

func (m Model) updateEditDisplayName(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ViewState = MenuView
		m.displayNameInput.Blur()
		return m, nil
	case "enter":
		newValue := strings.TrimSpace(m.displayNameInput.Value())
		m.displayNameInput.Blur()
		return m, updateProfileCmd(m.Account.Id, "displayName", newValue)
	}

	var cmd tea.Cmd
	m.displayNameInput, cmd = m.displayNameInput.Update(msg)
	return m, cmd
}

func (m Model) updateEditBio(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ViewState = MenuView
		m.bioInput.Blur()
		return m, nil
	case "enter":
		newValue := strings.TrimSpace(m.bioInput.Value())
		m.bioInput.Blur()
		return m, updateProfileCmd(m.Account.Id, "bio", newValue)
	}

	var cmd tea.Cmd
	m.bioInput, cmd = m.bioInput.Update(msg)
	return m, cmd
}

func (m Model) updateAvatar(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ViewState = MenuView
		m.uploadToken = ""
		m.uploadURL = ""
		m.isPolling = false // Stop polling when leaving avatar view
		return m, nil
	case "g", "G":
		// Generate upload link
		return m, createUploadTokenCmd(m.Account.Id)
	}
	return m, nil
}

func (m Model) updateDelete(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.ConfirmStep == 0 {
			m.ConfirmStep = 1
			return m, nil
		} else if m.ConfirmStep == 1 {
			m.Status = "Deleting account..."
			return m, deleteAccountCmd(m.Account.Id)
		}
	case "n", "N", "esc":
		if m.ConfirmStep > 0 {
			m.ConfirmStep = 0
			m.Status = "Deletion cancelled"
			return m, clearStatusAfter(2 * time.Second)
		}
		m.ViewState = MenuView
		return m, nil
	}
	return m, nil
}

func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render("account settings"))
	s.WriteString("\n\n")

	if m.ShowByeBye {
		byeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SUCCESS)).
			Bold(true).
			Align(lipgloss.Center)
		s.WriteString("\n\n")
		s.WriteString(byeStyle.Render("Bye bye!"))
		s.WriteString("\n\n")
		return tea.NewView(s.String())
	}

	if m.DeletionStatus == "completed" {
		s.WriteString(confirmStyle.Render("Account deleted successfully"))
		s.WriteString("\n\n")
		s.WriteString(instructionStyle.Render("Logging out..."))
		return tea.NewView(s.String())
	}

	switch m.ViewState {
	case MenuView:
		s.WriteString(m.renderMenu())
	case EditDisplayNameView:
		s.WriteString(m.renderEditDisplayName())
	case EditBioView:
		s.WriteString(m.renderEditBio())
	case AvatarView:
		s.WriteString(m.renderAvatar())
	case DeleteView:
		s.WriteString(m.renderDelete())
	}

	// Status and error messages
	if m.Status != "" {
		s.WriteString("\n")
		s.WriteString(successStyle.Render(m.Status))
	}
	if m.Error != "" {
		s.WriteString("\n")
		s.WriteString(warningStyle.Render(m.Error))
	}

	return tea.NewView(s.String())
}

func (m Model) renderMenu() string {
	var s strings.Builder

	// Profile header text (matching profile view style)
	var headerText strings.Builder

	displayNameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(common.COLOR_USERNAME)).
		Bold(true)
	handleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(common.COLOR_SECONDARY))
	bioStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(common.COLOR_WHITE))

	name := m.Account.DisplayName
	if name == "" {
		name = m.Account.Username
	}
	headerText.WriteString(displayNameStyle.Render(name))
	headerText.WriteString("\n")
	headerText.WriteString(handleStyle.Render("@" + m.Account.Username))
	headerText.WriteString("\n")

	if m.Account.Summary != "" {
		headerText.WriteString("\n")
		headerText.WriteString(bioStyle.Render(m.Account.Summary))
		headerText.WriteString("\n")
	}

	if m.avatarRendered != "" {
		leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
		rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
		contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)
		textWidth := contentWidth - avatarCols - 3
		if textWidth < 20 {
			textWidth = 20
		}
		constrainedHeader := lipgloss.NewStyle().Width(textWidth).Render(headerText.String())
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.avatarRendered, "   ", constrainedHeader))
	} else {
		s.WriteString(headerText.String())
	}
	s.WriteString("\n\n")

	items := []struct {
		key   string
		label string
	}{
		{"e", "Edit display name"},
		{"b", "Edit bio"},
		{"a", "Change avatar"},
		{"d", "Delete account"},
	}

	for i, item := range items {
		if MenuItem(i) == m.MenuItem {
			s.WriteString(selectedStyle.Render(fmt.Sprintf("> [%s] %s", item.key, item.label)))
		} else {
			s.WriteString(menuStyle.Render(fmt.Sprintf("  [%s] %s", item.key, item.label)))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(instructionStyle.Render("Use arrow keys or hotkeys to navigate, Enter to select"))

	return s.String()
}

func (m Model) renderEditDisplayName() string {
	var s strings.Builder

	s.WriteString("Edit Display Name\n\n")
	s.WriteString(m.displayNameInput.View())
	s.WriteString("\n\n")
	s.WriteString(instructionStyle.Render("Enter to save, Esc to cancel"))

	return s.String()
}

func (m Model) renderEditBio() string {
	var s strings.Builder

	s.WriteString("Edit Bio\n\n")
	s.WriteString(m.bioInput.View())
	s.WriteString("\n\n")
	s.WriteString(instructionStyle.Render("Enter to save, Esc to cancel"))

	return s.String()
}

func (m Model) renderAvatar() string {
	var s strings.Builder

	s.WriteString("Change Avatar\n\n")

	// Show current avatar
	if m.avatarRendered != "" {
		s.WriteString(m.avatarRendered)
		s.WriteString("\n\n")
	} else if m.Account.AvatarURL != "" {
		s.WriteString(fmt.Sprintf("Current avatar: %s\n\n", m.Account.AvatarURL))
	} else {
		s.WriteString("Current avatar: (default)\n\n")
	}

	if m.uploadURL != "" {
		s.WriteString("Open this link in your browser to upload an image:\n\n")
		// Use OSC 8 hyperlink escape sequence so the entire URL is clickable even when wrapped
		// Format: ESC]8;;URL ST DISPLAY_TEXT ESC]8;; ST (where ST = ESC \ or BEL)
		// Put escape sequences outside lipgloss styling to avoid interference
		s.WriteString("\x1b]8;;")
		s.WriteString(m.uploadURL)
		s.WriteString("\x1b\\")
		s.WriteString(linkStyle.Render(m.uploadURL))
		s.WriteString("\x1b]8;;\x1b\\")
		s.WriteString("\n\n")

		// Show remaining time
		remaining := time.Until(m.uploadExpiresAt)
		if remaining > 0 {
			minutes := int(remaining.Minutes())
			seconds := int(remaining.Seconds()) % 60
			s.WriteString(instructionStyle.Render(fmt.Sprintf("Link expires in %d:%02d", minutes, seconds)))
		} else {
			s.WriteString(warningStyle.Render("Link expired - press 'g' to generate a new one"))
		}
		s.WriteString("\n")
		if m.isPolling && remaining > 0 {
			s.WriteString(successStyle.Render("Waiting for upload... (auto-refreshing)"))
		}
	} else {
		s.WriteString("Press 'g' to generate an upload link.\n")
		s.WriteString("The link will allow you to upload an avatar image from your browser.\n")
	}

	s.WriteString("\n\n")
	s.WriteString(instructionStyle.Render("'g' generate link • Esc go back"))

	return s.String()
}

func (m Model) renderDelete() string {
	var s strings.Builder

	if m.ConfirmStep == 0 {
		s.WriteString(warningStyle.Render("WARNING: This will permanently delete your account!"))
		s.WriteString("\n\n")
		s.WriteString("The following data will be deleted:\n")
		s.WriteString(fmt.Sprintf("  - Your account (@%s)\n", m.Account.Username))
		s.WriteString("  - All your posts and notes\n")
		s.WriteString("  - All follow relationships\n")
		s.WriteString("  - All your activities\n")
		s.WriteString("\n")
		s.WriteString(warningStyle.Render("This action CANNOT be undone!"))
		s.WriteString("\n\n")
		s.WriteString("Are you sure you want to delete your account?\n\n")
		s.WriteString(instructionStyle.Render("Press 'y' to continue or 'n'/'esc' to cancel"))
	} else if m.ConfirmStep == 1 {
		s.WriteString(warningStyle.Render("FINAL WARNING!"))
		s.WriteString("\n\n")
		s.WriteString("You are about to permanently delete account: ")
		s.WriteString(warningStyle.Render("@" + m.Account.Username))
		s.WriteString("\n\n")
		s.WriteString("This is your last chance to cancel.\n")
		s.WriteString("After this, your account and all data will be gone forever.\n\n")
		s.WriteString(instructionStyle.Render("Press 'y' to DELETE PERMANENTLY or 'n'/'esc' to cancel"))
	}

	return s.String()
}

// Message types
type clearStatusMsg struct{}
type showByeByeMsg struct{}

type deleteAccountResultMsg struct {
	err error
}

type updateProfileResultMsg struct {
	field string
	value string
	err   error
}

type uploadTokenResultMsg struct {
	token     string
	expiresAt time.Time
	isNew     bool // true if newly created, false if reusing existing
	err       error
}

type refreshAccountResultMsg struct {
	account *domain.Account
	err     error
}

type avatarPollTickMsg struct{}

type checkTokenResultMsg struct {
	tokenExists bool
	err         error
}

// Commands
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func deleteAccountCmd(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.DeleteAccount(accountId)
		if err != nil {
			log.Printf("Failed to delete account %s: %v", accountId, err)
		} else {
			log.Printf("Successfully deleted account %s", accountId)
		}
		return deleteAccountResultMsg{err: err}
	}
}

func updateProfileCmd(accountId uuid.UUID, field, value string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		var err error

		switch field {
		case "displayName":
			err = database.UpdateAccountDisplayName(accountId, value)
		case "bio":
			err = database.UpdateAccountSummary(accountId, value)
		}

		return updateProfileResultMsg{
			field: field,
			value: value,
			err:   err,
		}
	}
}

func createUploadTokenCmd(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Check for existing valid token first
		existingToken, expiresAt, err := database.GetExistingUploadToken(accountId, "avatar")
		if err != nil {
			return uploadTokenResultMsg{err: err}
		}
		if existingToken != "" {
			// Return existing token
			return uploadTokenResultMsg{token: existingToken, expiresAt: expiresAt, isNew: false}
		}

		// Generate new random token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return uploadTokenResultMsg{err: err}
		}
		token := hex.EncodeToString(tokenBytes)

		// Store in database with expiry
		expiresIn := 10 * time.Minute
		err = database.CreateUploadToken(accountId, token, "avatar", expiresIn)
		if err != nil {
			return uploadTokenResultMsg{err: err}
		}

		return uploadTokenResultMsg{token: token, expiresAt: time.Now().Add(expiresIn), isNew: true}
	}
}

func refreshAccountCmd(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, account := database.ReadAccById(accountId)
		if err != nil {
			return refreshAccountResultMsg{err: err}
		}
		return refreshAccountResultMsg{account: account}
	}
}

func avatarPollTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return avatarPollTickMsg{}
	})
}

func checkTokenExistsCmd(token string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		// Try to validate the token - if it fails, the token was consumed or expired
		_, _, err := database.ValidateUploadToken(token)
		if err != nil {
			// Token doesn't exist or expired - upload likely completed
			return checkTokenResultMsg{tokenExists: false, err: nil}
		}
		return checkTokenResultMsg{tokenExists: true, err: nil}
	}
}
