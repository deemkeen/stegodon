# MyPosts View

This document specifies the MyPosts view, which displays the user's own posts with edit and delete capabilities.

---

## Overview

The MyPosts view shows a paginated list of the authenticated user's notes in reverse chronological order. Users can navigate through their posts, edit existing notes, delete notes (with confirmation), and like their own posts.

---

## Data Structure

```go
type Model struct {
    Notes            []domain.Note
    Offset           int              // Scroll position
    Selected         int              // Currently selected note index
    Width            int
    Height           int
    userId           uuid.UUID        // Owner's account ID

    // Delete confirmation
    confirmingDelete bool             // Show delete prompt
    deleteTargetId   uuid.UUID        // Note pending deletion

    // Display
    LocalDomain      string           // For mention highlighting
}
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ my posts (12 notes)                                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ 2h ago (edited)  · ⭐ 5 · 🔁 2                             │  ← Selected
│   @alice                                                     │
│   This is my latest post with some updates...               │
│                                                              │
│   5d ago                                                     │
│   @alice                                                     │
│   Previous post about #programming                          │
│                                                              │
│   1w ago  · ⭐ 12                                            │
│   @alice                                                     │
│   My most liked post ever!                                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Selection Highlighting

Selected notes use inverted colors (light text on accent background):

```go
var (
    // Normal styles
    timeStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_DIM))

    authorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_USERNAME)).
        Bold(true)

    contentStyle = lipgloss.NewStyle()

    // Selected styles (inverted)
    selectedTimeStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_WHITE))

    selectedAuthorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_WHITE)).
        Bold(true)

    selectedContentStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_WHITE))
)
```

---

## Navigation

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `u` | Edit selected note |
| `d` | Delete selected note (shows confirmation) |
| `l` | Like/unlike selected note |
| `b` | Boost/unboost selected note |
| `i` | Toggle engagement info (likers/boosters) |

### Scroll Behavior

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg.String() {
    case "up", "k":
        if m.Selected > 0 {
            m.Selected--
            m.Offset = m.Selected  // Keep selected at top
        }
    case "down", "j":
        if m.Selected < len(m.Notes)-1 {
            m.Selected++
            m.Offset = m.Selected  // Keep selected at top
        }
    }
}
```

The selected item is always kept at the top of the visible area (not centered).

---

## Delete Confirmation

### Flow

```
User presses 'd'
      │
      ▼
Show Confirmation
      │
      ├── 'y' / 'Y' → Delete note
      ├── 'n' / 'N' → Cancel
      └── 'Esc' → Cancel
```

### Display

```
┌─────────────────────────────────────────────────────────────┐
│ ▸ 2h ago                                                     │
│   @alice                                                     │
│   This note will be deleted...                               │
│                                                              │
│   Delete this note? Press y to confirm, n to cancel          │  ← Red text
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Logic

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    if m.confirmingDelete {
        switch msg.String() {
        case "y", "Y":
            noteId := m.deleteTargetId
            m.confirmingDelete = false
            m.deleteTargetId = uuid.Nil
            return m, deleteNoteCmd(noteId)
        case "n", "N", "esc":
            m.confirmingDelete = false
            m.deleteTargetId = uuid.Nil
        }
        return m, nil
    }

    switch msg.String() {
    case "d":
        if len(m.Notes) > 0 && m.Selected < len(m.Notes) {
            m.confirmingDelete = true
            m.deleteTargetId = m.Notes[m.Selected].Id
        }
    }
}
```

---

## Edit Mode

Pressing `u` on a selected note triggers edit mode:

```go
case "u":
    if len(m.Notes) > 0 && m.Selected < len(m.Notes) {
        selectedNote := m.Notes[m.Selected]
        return m, func() tea.Msg {
            return common.EditNoteMsg{
                NoteId:    selectedNote.Id,
                Message:   selectedNote.Message,
                CreatedAt: selectedNote.CreatedAt,
            }
        }
    }
```

This switches to the WriteNote view in edit mode with the note's content pre-filled.

---

## Like/Unlike

Pressing `l` toggles like on the selected note:

```go
case "l":
    if len(m.Notes) > 0 && m.Selected < len(m.Notes) {
        selectedNote := m.Notes[m.Selected]
        noteURI := selectedNote.ObjectURI
        if noteURI == "" {
            noteURI = "local:" + selectedNote.Id.String()
        }
        return m, func() tea.Msg {
            return common.LikeNoteMsg{
                NoteURI: noteURI,
                NoteID:  selectedNote.Id,
                IsLocal: true,
            }
        }
    }
```

---

## Delete With Federation

When a note is deleted, it's also federated via ActivityPub:

```go
func deleteNoteCmd(noteId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()

        // Get note details for federation
        err, note := database.ReadNoteId(noteId)
        var accountUsername string
        if err == nil && note != nil {
            accountUsername = note.CreatedBy
        }

        // Delete from database
        database.DeleteNoteById(noteId)

        // Federate deletion (background)
        if accountUsername != "" {
            go func() {
                if conf.WithAp {
                    activitypub.SendDelete(noteId, account, conf)
                }
            }()
        }

        return common.DeleteNoteMsg{NoteId: noteId}
    }
}
```

---

## Data Loading

### On Activation

```go
case common.ActivateViewMsg:
    // Reset state when view becomes active
    m.Selected = 0
    m.Offset = 0
    m.confirmingDelete = false
    m.deleteTargetId = uuid.Nil
    return m, loadNotes(m.userId)
```

### On Note List Update

```go
case common.SessionState:
    if msg == common.UpdateNoteList {
        return m, loadNotes(m.userId)
    }
```

### Load Command

```go
func loadNotes(userId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        err, notes := db.GetDB().ReadNotesByUserId(userId)
        if err != nil {
            return notesLoadedMsg{notes: []domain.Note{}}
        }
        return notesLoadedMsg{notes: *notes}
    }
}
```

---

## Note Display

### Timestamp Formatting

```go
func formatTime(t time.Time) string {
    duration := time.Since(t)

    if duration < time.Minute {
        return "just now"
    } else if duration < time.Hour {
        return fmt.Sprintf("%dm ago", int(duration.Minutes()))
    } else if duration < 24*time.Hour {
        return fmt.Sprintf("%dh ago", int(duration.Hours()))
    } else {
        return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
    }
}
```

### Edited Indicator

```go
timeStr := formatTime(note.CreatedAt)
if note.EditedAt != nil {
    timeStr += " (edited)"
}
```

### Engagement Stats

```go
engagementStr := ""
if note.LikeCount > 0 {
    engagementStr = fmt.Sprintf(" · ⭐ %d", note.LikeCount)
}
if note.BoostCount > 0 {
    engagementStr += fmt.Sprintf(" · 🔁 %d", note.BoostCount)
}
```

---

## Content Processing

### Link Conversion

Markdown links are converted to terminal hyperlinks (OSC 8):

```go
messageWithLinks := util.MarkdownLinksToTerminal(note.Message)
```

### Hashtag Highlighting

```go
messageWithHashtags := util.HighlightHashtagsTerminal(messageWithLinks)
```

### Mention Highlighting

```go
messageWithMentions := util.HighlightMentionsTerminal(messageWithHashtags, m.LocalDomain)
```

### Content Truncation

```go
content := util.TruncateVisibleLength(message, common.MaxContentTruncateWidth)
```

---

## Pagination

### Items Per Page

```go
const DefaultItemsPerPage = 10  // From common package
```

### Rendering

```go
func (m Model) View() tea.View {
    start := m.Offset
    end := min(start + common.DefaultItemsPerPage, len(m.Notes))

    for i := start; i < end; i++ {
        note := m.Notes[i]
        // Render note with selection highlighting
    }
}
```

---

## Empty State

```go
if len(m.Notes) == 0 {
    s.WriteString(emptyStyle.Render("No notes yet.\nCreate your first note!"))
}
```

---

## View Header

```go
s.WriteString(common.CaptionStyle.Render(
    fmt.Sprintf("my posts (%d notes)", len(m.Notes))
))
```

---

## Width Calculation

Uses layout helpers for consistent column widths:

```go
leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)  // 2 = padding
```

---

## Selection Bounds

After notes reload, selection is kept within bounds:

```go
case notesLoadedMsg:
    m.Notes = msg.notes
    if m.Selected >= len(m.Notes) {
        m.Selected = max(0, len(m.Notes)-1)
    }
    m.Offset = m.Selected
    return m, nil
```

---

## Initialization

```go
func NewPager(userId uuid.UUID, width, height int, localDomain string) Model {
    return Model{
        Notes:            []domain.Note{},
        Offset:           0,
        Selected:         0,
        Width:            width,
        Height:           height,
        userId:           userId,
        confirmingDelete: false,
        deleteTargetId:   uuid.Nil,
        LocalDomain:      localDomain,
    }
}
```

---

## Source Files

- `ui/myposts/notepager.go` - MyPosts view implementation
- `ui/myposts/notepager_test.go` - Tests
- `ui/common/commands.go` - EditNoteMsg, DeleteNoteMsg, LikeNoteMsg
- `ui/common/layout.go` - Width calculation helpers
- `activitypub/outbox.go` - SendDelete for federation
- `db/db.go` - ReadNotesByUserId, DeleteNoteById
