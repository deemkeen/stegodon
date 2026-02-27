# WriteNote View

This document specifies the WriteNote view, which handles note composition including @mention autocomplete, reply mode, and edit mode.

---

## Overview

The WriteNote view is the primary content creation interface. It supports:
- Creating new notes with character limits
- Replying to existing notes (threading)
- Editing existing notes
- @mention autocomplete for local users
- Content warnings (CW)
- Real-time character counting

---

## Data Structure

```go
type Model struct {
    TextArea      textarea.Model
    Account       *domain.Account
    Width         int
    Height        int
    CharCount     int              // Current character count

    // Reply mode
    ReplyingTo    string           // URI of note being replied to
    ReplyAuthor   string           // Author handle for display
    ReplyPreview  string           // Preview of parent note

    // Edit mode
    EditingNoteId uuid.UUID        // ID of note being edited
    EditCreatedAt time.Time        // Original creation time

    // Autocomplete
    AutocompleteVisible  bool      // Show suggestions
    AutocompleteSuggestions []string  // Up to 5 suggestions
    AutocompleteSelected int       // Currently selected index

    // State
    LocalDomain   string           // For mention resolution
}
```

---

## Character Limits

```go
const (
    MaxLetters      = 150   // Visible character limit in UI
    MaxNoteDBLength = 1000  // Actual database storage limit
)
```

The UI shows 150 as the limit, but the database stores up to 1000 characters. This prevents extremely long notes while allowing some flexibility.

---

## Composition Flow

### New Note

```
┌─────────────────────────────────────────────────────────────┐
│                    Write a Note                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ Hello world! This is my first post on stegodon.      │  │
│  │                                                       │  │
│  │ #introduction @alice                                  │  │
│  │                                                       │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  45/150 characters                                           │
│                                                              │
│  Ctrl+Enter to post • Esc to cancel                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Reply Mode

```
┌─────────────────────────────────────────────────────────────┐
│                    Reply to @bob                             │
│  "This is the original post content being replied to..."    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ @bob I totally agree with your point!                 │  │
│  │                                                       │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  35/150 characters                                           │
│                                                              │
│  Ctrl+Enter to reply • Esc to cancel                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Edit Mode

```
┌─────────────────────────────────────────────────────────────┐
│                    Editing Note                              │
│  Created: 2 hours ago                                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ Updated content with fixed typo.                      │  │
│  │                                                       │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  30/150 characters                                           │
│                                                              │
│  Ctrl+Enter to save • Esc to cancel                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## @Mention Autocomplete

### Trigger

Autocomplete activates when:
1. User types `@`
2. Followed by at least one character
3. Not inside a URL or hashtag

### Suggestion Lookup

```go
func updateAutocomplete(text string) tea.Cmd {
    return func() tea.Msg {
        // Find @ position
        atPos := findLastAtSign(text)
        if atPos < 0 {
            return autocompleteResultMsg{suggestions: nil}
        }

        // Extract partial username
        partial := text[atPos+1:]
        if len(partial) == 0 {
            return autocompleteResultMsg{suggestions: nil}
        }

        // Query database for matching usernames
        suggestions := db.GetDB().SearchLocalUsernames(partial, 5)
        return autocompleteResultMsg{suggestions: suggestions}
    }
}
```

### UI Display

```
┌─────────────────────────────────────────────────────────────┐
│  ┌───────────────────────────────────────────────────────┐  │
│  │ Hey @al                                               │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─────────────────────┐                                     │
│  │ ▸ alice             │  ← Selected                        │
│  │   alex              │                                     │
│  │   alfred            │                                     │
│  └─────────────────────┘                                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Navigation

| Key | Action |
|-----|--------|
| `↓` / `Ctrl+N` | Next suggestion |
| `↑` / `Ctrl+P` | Previous suggestion |
| `Tab` / `Enter` | Accept suggestion |
| `Esc` | Close autocomplete |

### Insertion

```go
func insertAutocompleteSuggestion(text string, suggestion string) string {
    // Find the @ that triggered autocomplete
    atPos := findLastAtSign(text)

    // Replace partial with full username
    // @al → @alice
    return text[:atPos] + "@" + suggestion + " "
}
```

---

## Keyboard Shortcuts

### Composition

| Key | Action |
|-----|--------|
| `Ctrl+Enter` | Submit note |
| `Esc` | Cancel composition |
| `Ctrl+C` | Cancel composition |

### Text Editing

| Key | Action |
|-----|--------|
| Standard textarea keys | Text editing |
| `@` | Trigger autocomplete |

### Autocomplete

| Key | Action |
|-----|--------|
| `↓` / `Ctrl+N` | Next suggestion |
| `↑` / `Ctrl+P` | Previous suggestion |
| `Tab` / `Enter` | Accept suggestion |
| `Esc` | Close autocomplete |

---

## Message Types

### Entering Reply Mode

```go
type ReplyToNoteMsg struct {
    NoteURI string    // ActivityPub URI of parent note
    Author  string    // Author handle for display
    Preview string    // First line of parent content
}
```

### Entering Edit Mode

```go
type EditNoteMsg struct {
    NoteId    uuid.UUID
    Message   string      // Current content
    CreatedAt time.Time   // For display
}
```

### Autocomplete Result

```go
type autocompleteResultMsg struct {
    suggestions []string
}
```

---

## Update Logic

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case common.ReplyToNoteMsg:
        m.ReplyingTo = msg.NoteURI
        m.ReplyAuthor = msg.Author
        m.ReplyPreview = msg.Preview
        m.TextArea.SetValue("@" + extractUsername(msg.Author) + " ")
        return m, nil

    case common.EditNoteMsg:
        m.EditingNoteId = msg.NoteId
        m.EditCreatedAt = msg.CreatedAt
        m.TextArea.SetValue(msg.Message)
        return m, nil

    case autocompleteResultMsg:
        m.AutocompleteSuggestions = msg.suggestions
        m.AutocompleteVisible = len(msg.suggestions) > 0
        m.AutocompleteSelected = 0
        return m, nil

    case tea.KeyPressMsg:
        if m.AutocompleteVisible {
            // Handle autocomplete navigation
            switch msg.String() {
            case "down", "ctrl+n":
                m.AutocompleteSelected = (m.AutocompleteSelected + 1) % len(m.AutocompleteSuggestions)
                return m, nil
            case "up", "ctrl+p":
                m.AutocompleteSelected--
                if m.AutocompleteSelected < 0 {
                    m.AutocompleteSelected = len(m.AutocompleteSuggestions) - 1
                }
                return m, nil
            case "tab", "enter":
                // Insert selected suggestion
                return m, insertSuggestionCmd(m.AutocompleteSuggestions[m.AutocompleteSelected])
            case "esc":
                m.AutocompleteVisible = false
                return m, nil
            }
        }

        switch msg.String() {
        case "ctrl+enter":
            return m, m.submitNote()
        case "esc":
            m.resetForm()
            return m, nil
        }
    }

    // Update textarea
    m.TextArea, cmd = m.TextArea.Update(msg)
    m.CharCount = len(m.TextArea.Value())

    // Trigger autocomplete check on text change
    if containsAt(m.TextArea.Value()) {
        return m, updateAutocomplete(m.TextArea.Value())
    }

    return m, cmd
}
```

---

## Note Submission

### New Note

```go
func (m Model) submitNewNote() tea.Cmd {
    return func() tea.Msg {
        note := &domain.Note{
            Id:        uuid.New(),
            Message:   m.TextArea.Value(),
            CreatedBy: m.Account.Username,
            CreatedAt: time.Now(),
        }

        // Parse hashtags
        hashtags := util.ExtractHashtags(note.Message)

        // Save to database
        db.GetDB().CreateNote(note, hashtags)

        // Federate via ActivityPub
        if conf.WithAp {
            activitypub.SendCreate(note, m.Account)
        }

        return noteCreatedMsg{note: note}
    }
}
```

### Reply

```go
func (m Model) submitReply() tea.Cmd {
    return func() tea.Msg {
        note := &domain.Note{
            Id:        uuid.New(),
            Message:   m.TextArea.Value(),
            CreatedBy: m.Account.Username,
            CreatedAt: time.Now(),
            InReplyTo: m.ReplyingTo,  // Parent URI
        }

        // Save and federate
        // ...

        return noteCreatedMsg{note: note}
    }
}
```

### Edit

```go
func (m Model) submitEdit() tea.Cmd {
    return func() tea.Msg {
        // Update note in database
        db.GetDB().UpdateNote(m.EditingNoteId, m.TextArea.Value())

        // Federate Update activity
        if conf.WithAp {
            activitypub.SendUpdate(m.EditingNoteId, m.Account)
        }

        return noteUpdatedMsg{noteId: m.EditingNoteId}
    }
}
```

---

## View Rendering

```go
func (m Model) View() tea.View {
    var header string

    // Build context-aware header
    if m.EditingNoteId != uuid.Nil {
        header = fmt.Sprintf("Editing Note\nCreated: %s", formatTime(m.EditCreatedAt))
    } else if m.ReplyingTo != "" {
        header = fmt.Sprintf("Reply to %s\n\"%s\"", m.ReplyAuthor, m.ReplyPreview)
    } else {
        header = "Write a Note"
    }

    // Character counter
    counter := fmt.Sprintf("%d/%d characters", m.CharCount, MaxLetters)
    if m.CharCount > MaxLetters {
        counter = warningStyle.Render(counter)
    }

    // Autocomplete dropdown (if visible)
    var autocomplete string
    if m.AutocompleteVisible {
        autocomplete = renderAutocompleteDropdown(m.AutocompleteSuggestions, m.AutocompleteSelected)
    }

    return lipgloss.JoinVertical(lipgloss.Left,
        header,
        m.TextArea.View(),
        autocomplete,
        counter,
        hints,
    )
}
```

---

## Form Reset

After submission or cancel:

```go
func (m *Model) resetForm() {
    m.TextArea.SetValue("")
    m.CharCount = 0
    m.ReplyingTo = ""
    m.ReplyAuthor = ""
    m.ReplyPreview = ""
    m.EditingNoteId = uuid.Nil
    m.AutocompleteVisible = false
    m.AutocompleteSuggestions = nil
}
```

---

## TextArea Configuration

```go
func NewTextArea() textarea.Model {
    ta := textarea.New()
    ta.Placeholder = "What's on your mind?"
    ta.CharLimit = MaxNoteDBLength  // 1000
    ta.SetWidth(60)
    ta.SetHeight(5)
    ta.Focus()
    return ta
}
```

---

## Hashtag Parsing

Hashtags are automatically extracted and linked:

```go
func ExtractHashtags(text string) []string {
    re := regexp.MustCompile(`#(\w+)`)
    matches := re.FindAllStringSubmatch(text, -1)
    // Return unique hashtags
}
```

---

## Mention Parsing

Mentions are detected for notifications:

```go
func ExtractMentions(text string) []string {
    re := regexp.MustCompile(`@(\w+)(?:@([\w.-]+))?`)
    matches := re.FindAllStringSubmatch(text, -1)
    // Return username@domain pairs
}
```

---

## Source Files

- `ui/writenote/writenote.go` - WriteNote view implementation
- `ui/common/commands.go` - EditNoteMsg, ReplyToNoteMsg
- `util/text.go` - Hashtag and mention extraction
- `activitypub/outbox.go` - Create/Update activity sending
- `db/db.go` - Note CRUD operations
