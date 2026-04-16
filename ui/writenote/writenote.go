package writenote

import (
	"fmt"
	"log"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

const maxAutocompleteSuggestions = 5

// maxLetters is the maximum number of visible characters allowed in a note
// This value is loaded from configuration (with STEGODON_MAX_CHARS env var support)
// and stored here for efficient access without repeated function calls
var maxLetters int

// MentionCandidate represents a user that can be mentioned
type MentionCandidate struct {
	Username string
	Domain   string
	IsLocal  bool
}

func (m MentionCandidate) FullMention() string {
	return fmt.Sprintf("@%s@%s", m.Username, m.Domain)
}

func (m MentionCandidate) DisplayMention() string {
	if m.IsLocal {
		return fmt.Sprintf("@%s", m.Username)
	}
	return fmt.Sprintf("@%s@%s", m.Username, m.Domain)
}

type Model struct {
	Textarea          textarea.Model
	Err               util.ErrMsg
	Error             string // Error message to display
	userId            uuid.UUID
	lettersLeft       int
	width             int
	isEditing         bool      // True when editing an existing note
	editingNoteId     uuid.UUID // ID of note being edited
	originalCreatedAt time.Time // Original creation time (preserved during edit)
	// Reply mode fields
	isReplying     bool   // True when replying to a post
	replyToURI     string // URI of the post being replied to
	replyToAuthor  string // Author of the post being replied to
	replyToPreview string // Preview of the post being replied to
	// Autocomplete fields
	showAutocomplete       bool               // True when autocomplete popup is visible
	autocompleteCandidates []MentionCandidate // All available candidates
	filteredCandidates     []MentionCandidate // Filtered based on current input
	autocompleteIndex      int                // Currently selected suggestion
	lastMentionQuery       string             // Last seen mention query, used to preserve selection across no-op updates
	mentionStartPos        int                // Position where @ was typed
	localDomain            string             // Local domain for identifying local users
	// Server message
	serverMessage *domain.ServerMessage // Message from server admin to display
}

func InitialNote(contentWidth int, userId uuid.UUID) Model {
	width := common.DefaultCreateNoteWidth(contentWidth)
	ti := textarea.New()
	ti.Placeholder = "enter your message"
	ti.CharLimit = common.MaxNoteDBLength // Set to DB limit, we'll validate visible chars separately
	ti.ShowLineNumbers = false
	ti.SetWidth(common.TextInputDefaultWidth)
	ti.SetHeight(10) // Double the default height
	ti.Focus()

	// Load configuration to get max characters setting
	if conf, err := util.ReadConf(); err == nil {
		maxLetters = conf.Conf.MaxChars
	} else {
		// Fallback to default if config can't be read
		maxLetters = 150
	}

	// Get local domain for autocomplete
	localDomain := "example.com"
	if conf, err := util.ReadConf(); err == nil {
		localDomain = conf.Conf.SslDomain
	}

	// Load autocomplete candidates
	candidates := loadAutocompleteCandidates(localDomain)

	return Model{
		Textarea:               ti,
		Err:                    nil,
		Error:                  "",
		userId:                 userId,
		lettersLeft:            maxLetters,
		width:                  width,
		isEditing:              false,
		editingNoteId:          uuid.Nil,
		originalCreatedAt:      time.Time{},
		isReplying:             false,
		replyToURI:             "",
		replyToAuthor:          "",
		replyToPreview:         "",
		showAutocomplete:       false,
		autocompleteCandidates: candidates,
		filteredCandidates:     nil,
		autocompleteIndex:      0,
		mentionStartPos:        -1,
		localDomain:            localDomain,
	}
}

// loadAutocompleteCandidates loads all local and remote accounts for autocomplete
func loadAutocompleteCandidates(localDomain string) []MentionCandidate {
	var candidates []MentionCandidate
	database := db.GetDB()

	// Load local accounts
	err, localAccounts := database.ReadAllAccounts()
	if err == nil && localAccounts != nil {
		for _, acc := range *localAccounts {
			candidates = append(candidates, MentionCandidate{
				Username: acc.Username,
				Domain:   localDomain,
				IsLocal:  true,
			})
		}
	}

	// Load remote accounts
	err, remoteAccounts := database.ReadAllRemoteAccounts()
	if err == nil && remoteAccounts != nil {
		for _, acc := range remoteAccounts {
			candidates = append(candidates, MentionCandidate{
				Username: acc.Username,
				Domain:   acc.Domain,
				IsLocal:  false,
			})
		}
	}

	return candidates
}

func createNoteModelCmd(note *domain.SaveNote) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Create note in database and get the created note ID
		// Use CreateNoteWithReply to support replies
		noteId, err := database.CreateNoteWithReply(note.UserId, note.Message, note.InReplyToURI)
		if err != nil {
			log.Println("Note could not be saved!")
			return common.UpdateNoteList
		}

		// Create reply notification if replying to a local note
		if note.InReplyToURI != "" {
			// Check if this is a local note URI
			if strings.HasPrefix(note.InReplyToURI, "local:") || strings.HasPrefix(note.InReplyToURI, "https://") {
				var parentNote *domain.Note
				var readErr error

				// Try to read parent note
				if strings.HasPrefix(note.InReplyToURI, "local:") {
					noteIdStr := strings.TrimPrefix(note.InReplyToURI, "local:")
					parentNoteId, parseErr := uuid.Parse(noteIdStr)
					if parseErr == nil {
						readErr, parentNote = database.ReadNoteId(parentNoteId)
					}
				} else {
					readErr, parentNote = database.ReadNoteByURI(note.InReplyToURI)
				}

				// Create notification if parent note exists and is local
				if readErr == nil && parentNote != nil {
					readErr, parentAuthor := database.ReadAccByUsername(parentNote.CreatedBy)
					if readErr == nil && parentAuthor != nil && parentAuthor.Id != note.UserId {
						// Only notify if replier is not the parent author
						readErr, replier := database.ReadAccById(note.UserId)
						if readErr == nil && replier != nil {
							preview := util.StripHTMLTags(note.Message)
							if len(preview) > 100 {
								preview = preview[:100] + "..."
							}
							notification := &domain.Notification{
								Id:               uuid.New(),
								AccountId:        parentAuthor.Id,
								NotificationType: domain.NotificationReply,
								ActorId:          replier.Id,
								ActorUsername:    replier.Username,
								ActorDomain:      "", // Empty for local users
								NoteId:           noteId,
								NotePreview:      preview,
								Read:             false,
								CreatedAt:        time.Now(),
							}
							if err := database.CreateNotification(notification); err != nil {
								log.Printf("Failed to create reply notification: %v", err)
							}
						}
					}
				}
			}
		}

		// Create mention notifications for local users
		mentions := util.ParseMentions(note.Message)
		if len(mentions) > 0 {
			conf, confErr := util.ReadConf()
			if confErr == nil && conf != nil {
				readErr, author := database.ReadAccById(note.UserId)
				if readErr == nil && author != nil {
					preview := util.StripHTMLTags(note.Message)
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}

					for _, mention := range mentions {
						// Check if this is a local user
						if mention.Domain == "" || mention.Domain == conf.Conf.SslDomain {
							readErr, mentionedUser := database.ReadAccByUsername(mention.Username)
							if readErr == nil && mentionedUser != nil && mentionedUser.Id != note.UserId {
								// Only notify if mentioner is not the mentioned user
								notification := &domain.Notification{
									Id:               uuid.New(),
									AccountId:        mentionedUser.Id,
									NotificationType: domain.NotificationMention,
									ActorId:          author.Id,
									ActorUsername:    author.Username,
									ActorDomain:      "", // Empty for local users
									NoteId:           noteId,
									NotePreview:      preview,
									Read:             false,
									CreatedAt:        time.Now(),
								}
								if err := database.CreateNotification(notification); err != nil {
									log.Printf("Failed to create mention notification: %v", err)
								}
							}
						}
					}
				}
			}
		}

		// Link hashtags to the note
		hashtags := util.ParseHashtags(note.Message)
		if len(hashtags) > 0 {
			hashtagIds := make([]int64, 0, len(hashtags))
			for _, tag := range hashtags {
				hashtagId, err := database.CreateOrUpdateHashtag(tag)
				if err != nil {
					log.Printf("Failed to create/update hashtag %s: %v", tag, err)
					continue
				}
				hashtagIds = append(hashtagIds, hashtagId)
			}
			if len(hashtagIds) > 0 {
				if err := database.LinkNoteHashtags(noteId, hashtagIds); err != nil {
					log.Printf("Failed to link hashtags to note: %v", err)
				}
			}
		}

		// Federate the note via ActivityPub (background task)
		go func() {
			// Get the created note from database with actual ID, timestamps, and reply info
			err, createdNote := database.ReadNoteIdWithReplyInfo(noteId)
			if err != nil {
				log.Printf("Failed to read created note for federation: %v", err)
				return
			}

			// Get the account
			err, account := database.ReadAccById(note.UserId)
			if err != nil {
				log.Printf("Failed to get account for federation: %v", err)
				return
			}

			// Get config
			conf, err := util.ReadConf()
			if err != nil {
				log.Printf("Failed to read config for federation: %v", err)
				return
			}

			// Only federate if ActivityPub is enabled
			if !conf.Conf.WithAp {
				return
			}

			// Send Create activity to all followers with the actual note from database
			if err := activitypub.SendCreate(createdNote, account, conf); err != nil {
				log.Printf("Failed to federate note: %v", err)
			} else {
				log.Printf("Note federated successfully for %s", account.Username)
			}
		}()

		return common.UpdateNoteList
	}
}

func updateNoteModelCmd(noteId uuid.UUID, message string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Update note in database
		err := database.UpdateNote(noteId, message)
		if err != nil {
			log.Printf("Note could not be updated: %v", err)
			return common.UpdateNoteList
		}

		log.Printf("Note %s updated successfully", noteId)

		// Update hashtags for the note
		// First, we need to unlink old hashtags and link new ones
		// For simplicity, we'll just create/update new hashtags and link them
		// The old links will remain (could be cleaned up with a separate function)
		hashtags := util.ParseHashtags(message)
		if len(hashtags) > 0 {
			hashtagIds := make([]int64, 0, len(hashtags))
			for _, tag := range hashtags {
				hashtagId, err := database.CreateOrUpdateHashtag(tag)
				if err != nil {
					log.Printf("Failed to create/update hashtag %s: %v", tag, err)
					continue
				}
				hashtagIds = append(hashtagIds, hashtagId)
			}
			if len(hashtagIds) > 0 {
				if err := database.LinkNoteHashtags(noteId, hashtagIds); err != nil {
					log.Printf("Failed to link hashtags to note: %v", err)
				}
			}
		}

		// Federate the update via ActivityPub (background task)
		go func() {
			// Get the updated note
			err, note := database.ReadNoteId(noteId)
			if err != nil {
				log.Printf("Failed to get note for federation: %v", err)
				return
			}

			// Get the account
			err, account := database.ReadAccByUsername(note.CreatedBy)
			if err != nil {
				log.Printf("Failed to get account for federation: %v", err)
				return
			}

			// Get config
			conf, err := util.ReadConf()
			if err != nil {
				log.Printf("Failed to read config for federation: %v", err)
				return
			}

			// Only federate if ActivityPub is enabled
			if !conf.Conf.WithAp {
				return
			}

			// Send Update activity to all followers
			if err := activitypub.SendUpdate(note, account, conf); err != nil {
				log.Printf("Failed to federate note update: %v", err)
			} else {
				log.Printf("Note update federated successfully for %s", account.Username)
			}
		}()

		return common.UpdateNoteList
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, loadServerMessage())
}

type serverMessageLoadedMsg struct {
	message *domain.ServerMessage
}

func loadServerMessage() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, msg := database.ReadServerMessage()
		if err != nil {
			log.Printf("Failed to load server message: %v", err)
			return serverMessageLoadedMsg{message: nil}
		}
		return serverMessageLoadedMsg{message: msg}
	}
}

func (m *Model) Focus() {
	m.Textarea.Focus()
}

func (m *Model) Blur() {
	m.Textarea.Blur()
}

func (m Model) IsReplying() bool {
	return m.isReplying
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case serverMessageLoadedMsg:
		m.serverMessage = msg.message
		return m, nil

	case common.EditNoteMsg:
		// Enter edit mode: populate textarea with existing note
		m.isEditing = true
		m.editingNoteId = msg.NoteId
		m.originalCreatedAt = msg.CreatedAt
		m.Textarea.SetValue(msg.Message)
		m.Textarea.Focus()
		// Clear reply mode if active
		m.isReplying = false
		m.replyToURI = ""
		m.replyToAuthor = ""
		m.replyToPreview = ""
		// Clear autocomplete
		m.showAutocomplete = false
		return m, nil

	case common.ReplyToNoteMsg:
		// Enter reply mode
		m.isReplying = true
		m.replyToURI = msg.NoteURI
		m.replyToAuthor = msg.Author
		m.replyToPreview = msg.Preview
		// Clear edit mode if active
		m.isEditing = false
		m.editingNoteId = uuid.Nil
		m.originalCreatedAt = time.Time{}
		// Pre-fill textarea with @user@server mention (user can delete if desired)
		mention := msg.Author
		if !strings.HasPrefix(mention, "@") {
			mention = "@" + mention
		}
		m.Textarea.SetValue(mention + " ")
		m.Textarea.CursorEnd()
		m.Textarea.Focus()
		// Clear autocomplete
		m.showAutocomplete = false
		return m, nil

	case tea.KeyPressMsg:
		// Clear error when user starts typing
		if msg.String() != "ctrl+s" && msg.String() != "ctrl+a" && msg.String() != "ctrl+c" && msg.String() != "esc" {
			m.Error = ""
		}

		// Handle autocomplete navigation when popup is visible
		if m.showAutocomplete {
			switch msg.String() {
			case "up":
				if m.autocompleteIndex > 0 {
					m.autocompleteIndex--
				}
				return m, nil
			case "down":
				if m.autocompleteIndex < len(m.filteredCandidates)-1 {
					m.autocompleteIndex++
				}
				return m, nil
			case "tab", "enter":
				// Insert selected suggestion
				if len(m.filteredCandidates) > 0 {
					m.insertAutocompleteSuggestion()
				}
				m.showAutocomplete = false
				return m, nil
			case "esc":
				// Close autocomplete
				m.showAutocomplete = false
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+a":
			if m.Textarea.Focused() {
				m.Textarea.Blur()
			}
		case "ctrl+s":
			rawValue := m.Textarea.Value()

			// Validate that note is not empty (trim whitespace for validation)
			if len(strings.TrimSpace(rawValue)) == 0 {
				m.Error = "Cannot save an empty note"
				return m, nil
			}

			// Validate that visible characters don't exceed maxChars
			visibleChars := util.CountVisibleChars(rawValue)
			if visibleChars > maxLetters {
				m.Error = fmt.Sprintf("Note too long (%d visible characters, max %d)", visibleChars, maxLetters)
				return m, nil
			}

			// Validate that full message (including markdown) doesn't exceed MaxNoteDBLength
			// Check BEFORE normalizing, as normalization might change length
			if err := util.ValidateNoteLength(rawValue); err != nil {
				m.Error = err.Error()
				return m, nil
			}

			// Normalize input after validation
			value := util.NormalizeInput(rawValue)

			if m.isEditing {
				// Update existing note
				noteId := m.editingNoteId
				m.Textarea.SetValue("")
				m.Error = ""
				// Exit edit mode
				m.isEditing = false
				m.editingNoteId = uuid.Nil
				m.originalCreatedAt = time.Time{}
				return m, updateNoteModelCmd(noteId, value)
			} else if m.isReplying {
				// Create reply note with inReplyTo
				replyURI := m.replyToURI

				// If this is a local: URI, resolve it to a proper ActivityPub URI
				if strings.HasPrefix(replyURI, "local:") {
					noteIdStr := strings.TrimPrefix(replyURI, "local:")
					if conf, err := util.ReadConf(); err == nil && conf.Conf.SslDomain != "" && conf.Conf.SslDomain != "example.com" {
						// Construct proper ActivityPub URI
						replyURI = fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, noteIdStr)
					} else {
						// No valid domain configured, keep local: prefix
						// This will only work for local thread views
						replyURI = "local:" + noteIdStr
					}
				}

				note := domain.SaveNote{
					UserId:       m.userId,
					Message:      value,
					InReplyToURI: replyURI,
				}
				m.Textarea.SetValue("")
				m.Error = ""
				// Exit reply mode
				m.isReplying = false
				m.replyToURI = ""
				m.replyToAuthor = ""
				m.replyToPreview = ""
				return m, createNoteModelCmd(&note)
			} else {
				// Create new note
				note := domain.SaveNote{
					UserId:  m.userId,
					Message: value,
				}
				m.Textarea.SetValue("")
				m.Error = ""
				return m, createNoteModelCmd(&note)
			}
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			// Cancel edit mode or reply mode
			if m.isEditing {
				m.isEditing = false
				m.editingNoteId = uuid.Nil
				m.originalCreatedAt = time.Time{}
				m.Textarea.SetValue("")
				return m, nil
			}
			if m.isReplying {
				m.isReplying = false
				m.replyToURI = ""
				m.replyToAuthor = ""
				m.replyToPreview = ""
				m.Textarea.SetValue("")
				return m, nil
			}
		default:
			if !m.Textarea.Focused() {
				cmd = m.Textarea.Focus()
				cmds = append(cmds, cmd)
			}
		}

	// We handle errors just like any other message
	case util.ErrMsg:
		m.Err = msg
		return m, nil

	case tea.WindowSizeMsg:
		// Ignore window resize - keep textarea at fixed width
		return m, nil
	}

	m.Textarea, cmd = m.Textarea.Update(msg)

	// Check if visible character count exceeds maxChars
	visibleChars := util.CountVisibleChars(m.Textarea.Value())
	if visibleChars > maxLetters {
		// Revert the last change by not allowing more visible chars
		// Note: This is a simple check, ideally we'd prevent the input
		// For now, the character counter will show negative and save will fail
	}

	// Auto-convert pasted URLs to markdown format
	// Check if the entire content is a URL (user just pasted a URL)
	currentValue := m.Textarea.Value()
	if util.IsURL(strings.TrimSpace(currentValue)) {
		url := strings.TrimSpace(currentValue)
		// Convert to markdown with "Link" as the text: [Link](url)
		markdown := fmt.Sprintf("[Link](%s)", url)
		m.Textarea.SetValue(markdown)
		// Move cursor to end
		m.Textarea.CursorEnd()
	}

	// Handle autocomplete trigger and filtering
	m.updateAutocomplete()

	m.lettersLeft = m.CharCount()
	cmds = append(cmds, cmd)

	// Avoid tea.Batch() when possible to prevent goroutine leaks
	switch len(cmds) {
	case 0:
		return m, nil
	case 1:
		return m, cmds[0]
	default:
		return m, tea.Batch(cmds...)
	}
}

// updateAutocomplete checks if we should show/hide/update autocomplete suggestions
func (m *Model) updateAutocomplete() {
	value := m.Textarea.Value()

	// We assume cursor is at end of text for simplicity
	// This works because users typically type at the end
	cursorPos := len([]rune(value))

	// Find if we're currently typing a mention
	mentionStart, query := m.findCurrentMention(value, cursorPos)

	if mentionStart >= 0 {
		// We're typing a mention
		m.mentionStartPos = mentionStart
		m.filterCandidates(query)
		m.showAutocomplete = len(m.filteredCandidates) > 0
		// Only reset selection when the query actually changes, so background
		// messages (cursor blink, ticks) don't clobber the user's scroll position.
		if query != m.lastMentionQuery {
			m.autocompleteIndex = 0
			m.lastMentionQuery = query
		} else if m.autocompleteIndex >= len(m.filteredCandidates) {
			// Clamp if candidates shrank
			m.autocompleteIndex = 0
		}
	} else {
		// Not typing a mention
		m.showAutocomplete = false
		m.mentionStartPos = -1
		m.lastMentionQuery = ""
	}
}

// findCurrentMention finds if cursor is within a mention being typed
// Returns the start position of @ and the query text after it
func (m *Model) findCurrentMention(text string, cursorPos int) (int, string) {
	if cursorPos == 0 || len(text) == 0 {
		return -1, ""
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(text)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	// Search backwards from cursor for @
	for i := cursorPos - 1; i >= 0; i-- {
		r := runes[i]

		// Stop at whitespace or start of text
		if r == ' ' || r == '\n' || r == '\t' {
			return -1, ""
		}

		// Found @
		if r == '@' {
			// Check if this is at start or after whitespace
			if i == 0 || runes[i-1] == ' ' || runes[i-1] == '\n' || runes[i-1] == '\t' {
				query := string(runes[i+1 : cursorPos])
				return i, query
			}
			return -1, ""
		}
	}

	return -1, ""
}

// filterCandidates filters autocomplete candidates based on query
func (m *Model) filterCandidates(query string) {
	query = strings.ToLower(query)
	m.filteredCandidates = nil

	for _, candidate := range m.autocompleteCandidates {
		// Match against username or full mention
		username := strings.ToLower(candidate.Username)
		fullMention := strings.ToLower(candidate.FullMention())

		if strings.HasPrefix(username, query) || strings.Contains(fullMention, query) {
			m.filteredCandidates = append(m.filteredCandidates, candidate)
			if len(m.filteredCandidates) >= maxAutocompleteSuggestions {
				break
			}
		}
	}
}

// insertAutocompleteSuggestion inserts the selected mention into the textarea
func (m *Model) insertAutocompleteSuggestion() {
	if m.autocompleteIndex >= len(m.filteredCandidates) {
		return
	}

	selected := m.filteredCandidates[m.autocompleteIndex]
	value := m.Textarea.Value()

	// Convert to runes for proper Unicode handling
	runes := []rune(value)
	cursorPos := len(runes) // Assume cursor at end

	// Build new text: everything before @, the mention, then continue typing
	before := string(runes[:m.mentionStartPos])
	mention := selected.FullMention() + " "
	after := ""
	if cursorPos < len(runes) {
		after = string(runes[cursorPos:])
	}

	newValue := before + mention + after
	m.Textarea.SetValue(newValue)

	// Move cursor to end
	m.Textarea.CursorEnd()

	// Update character count after insertion
	m.lettersLeft = m.CharCount()
}

func (m Model) CharCount() int {
	// Use CountVisibleChars to only count visible text, not markdown URLs
	visibleChars := util.CountVisibleChars(m.Textarea.Value())
	return maxLetters - visibleChars
}

func (m Model) View() tea.View {
	styledTextarea := lipgloss.NewStyle().PaddingLeft(5).PaddingRight(5).Render(m.Textarea.View())

	// Show autocomplete popup if active
	autocompletePopup := ""
	if m.showAutocomplete && len(m.filteredCandidates) > 0 {
		autocompletePopup = "\n" + m.renderAutocompletePopup()
	}

	// Show markdown link indicator if any are detected
	linkIndicator := ""
	if linkCount := util.GetMarkdownLinkCount(m.Textarea.Value()); linkCount > 0 {
		linkStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SUCCESS)).
			PaddingLeft(5)
		plural := ""
		if linkCount > 1 {
			plural = "s"
		}
		linkIndicator = "\n" + linkStyle.Render(fmt.Sprintf("✓ %d markdown link%s detected", linkCount, plural))
	}

	helpText := "post message: ctrl+s"
	if m.isEditing {
		helpText = "save changes: ctrl+s\ncancel: esc"
	} else if m.isReplying {
		helpText = "post reply: ctrl+s\ncancel: esc"
	}
	if m.showAutocomplete {
		helpText += "\n↑/↓: navigate, enter: select, esc: close"
	}

	// Build the help section with proper formatting
	helpLines := fmt.Sprintf("characters left: %d\n\n%s", m.lettersLeft, helpText)
	charsLeft := common.HelpStyle.Render(lipgloss.NewStyle().PaddingLeft(5).Render(helpLines))

	captionText := "new note"
	if m.isEditing {
		captionText = "edit note"
	} else if m.isReplying {
		// replyToAuthor already has @ prefix for remote users (@user@domain)
		// but not for local users, so we need to check
		if strings.HasPrefix(m.replyToAuthor, "@") {
			captionText = "reply to " + m.replyToAuthor
		} else {
			captionText = "reply to @" + m.replyToAuthor
		}
	}
	caption := common.CaptionStyle.PaddingLeft(5).Render(captionText)

	// Show reply context if replying
	replyContext := ""
	if m.isReplying && m.replyToPreview != "" {
		replyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_MUTED)).
			Italic(true).
			PaddingLeft(5)
		// Truncate preview if too long
		preview := m.replyToPreview
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		replyContext = replyStyle.Render("\""+preview+"\"") + "\n\n"
	}

	// Add error message if present
	errorSection := ""
	if m.Error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_ERROR)).
			Bold(true).
			PaddingLeft(5)
		errorSection = "\n" + errorStyle.Render(m.Error)
	}

	// Add server message if enabled
	serverMessageSection := ""
	if m.serverMessage != nil && m.serverMessage.Enabled && m.serverMessage.Message != "" {
		serverMsgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_WARNING)).
			Bold(true).
			PaddingLeft(5).
			PaddingTop(1)
		serverMessageSection = "\n\n" + serverMsgStyle.Render("📢 Server Message:") + "\n" +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_MUTED)).
				PaddingLeft(5).
				Render(m.serverMessage.Message)
	}

	return tea.NewView(fmt.Sprintf("%s\n\n%s%s%s%s%s\n\n%s%s", caption, replyContext, styledTextarea, autocompletePopup, linkIndicator, errorSection, charsLeft, serverMessageSection))
}

// renderAutocompletePopup renders the autocomplete suggestion list
func (m Model) renderAutocompletePopup() string {
	if len(m.filteredCandidates) == 0 {
		return ""
	}

	// Styles for the popup
	popupStyle := lipgloss.NewStyle().
		PaddingLeft(5).
		PaddingRight(2)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(common.COLOR_LIGHT))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(common.COLOR_BLACK)).
		Background(lipgloss.Color(common.COLOR_BUTTON)).
		Bold(true)

	localBadgeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(common.COLOR_DIM)).
		Italic(true)

	var lines []string
	for i, candidate := range m.filteredCandidates {
		mention := candidate.DisplayMention()

		// Add local/remote indicator
		badge := ""
		if candidate.IsLocal {
			badge = localBadgeStyle.Render(" (local)")
		}

		line := mention + badge
		if i == m.autocompleteIndex {
			line = selectedStyle.Render(mention) + badge
		} else {
			line = normalStyle.Render(mention) + badge
		}
		lines = append(lines, line)
	}

	return popupStyle.Render(strings.Join(lines, "\n"))
}
