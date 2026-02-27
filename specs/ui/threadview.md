# ThreadView

This document specifies the ThreadView, which displays conversation threads with parent posts and their replies.

---

## Overview

The ThreadView displays a hierarchical view of a post and all its replies. It supports:
- Viewing the parent post at the top
- Displaying both local and remote replies
- Navigating into nested threads (reply to reply)
- Replying to any post in the thread
- Like/unlike posts
- URL toggle for ActivityPub URIs

---

## Data Structure

```go
type Model struct {
    AccountId       uuid.UUID
    ParentURI       string         // URI of the parent post
    ParentPost      *ThreadPost    // The parent post
    Replies         []ThreadPost   // Replies to the parent
    Selected        int            // -1 = parent, 0+ = reply index
    Offset          int            // Scroll offset
    Width           int
    Height          int
    isActive        bool
    loading         bool
    errorMessage    string
    showingURL      bool           // Toggle URL display

    // For reload support
    parentNoteID    uuid.UUID
    parentIsLocal   bool
    parentAuthor    string
    parentContent   string
    parentCreatedAt time.Time
    pendingSelection int           // Restore selection after reload
    pendingOffset    int
    LocalDomain      string
}
```

### ThreadPost Structure

```go
type ThreadPost struct {
    ID         uuid.UUID
    Author     string
    Content    string
    Time       time.Time
    ObjectURI  string     // ActivityPub object id (canonical URI, returns JSON)
    ObjectURL  string     // ActivityPub object url (human-readable web UI link, preferred for display)
    IsLocal    bool       // Local vs remote post
    IsParent   bool       // Whether this is the parent post
    IsDeleted  bool       // Placeholder for deleted posts
    ReplyCount int
    LikeCount  int
    BoostCount int
}
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ thread (3 replies)                                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ 2h ago · 3 replies · ⭐ 5                                  │  ← Parent (selected)
│   @alice                                                     │
│   This is the original post that started the thread         │
│                                                              │
│     5h ago · 1 reply                                         │  ← Reply (indented)
│     @bob@mastodon.social                                     │
│     Great point! I think...                                  │
│                                                              │
│     6h ago                                                   │
│     @charlie                                                 │
│     Another reply to the thread                              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Reply Indentation

Replies are visually indented from the parent:

```go
const ReplyIndentWidth = 4  // From common package

replyIndent = strings.Repeat(" ", common.ReplyIndentWidth)

// For replies, reduce content width and add left padding
if !isParent {
    indentWidth = len(replyIndent)
    itemWidth = contentWidth - indentWidth
}
```

---

## Navigation

### Selection System

- `Selected = -1`: Parent post is selected
- `Selected = 0`: First reply is selected
- `Selected = N`: Nth reply is selected

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up (resets URL view) |
| `↓` / `j` | Move selection down (resets URL view) |
| `Enter` | Open nested thread (if reply has replies) |
| `r` | Reply to selected post |
| `l` | Like/unlike selected post |
| `b` | Boost/unboost selected post |
| `i` | Toggle engagement info (likers/boosters) |
| `o` | Toggle URL display (only for valid HTTP/HTTPS URLs) |
| `Esc` / `q` | Return to home timeline |

### Navigation URL Reset

When navigating up/down, the URL view is automatically reset:

```go
case "up", "k":
    if m.Selected > 0 {
        m.Selected--
        m.Offset = m.Selected
    }
    m.showingURL = false  // Reset URL view on navigation

case "down", "j":
    if m.Selected < len(m.Replies)-1 {
        m.Selected++
        m.Offset = m.Selected
    }
    m.showingURL = false  // Reset URL view on navigation
```

---

## Thread Loading

### By URI

For posts with an ActivityPub URI:

```go
func loadThread(parentURI string) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()

        // Try local note first
        err, localNote := database.ReadNoteByURI(parentURI)
        if err == nil && localNote != nil {
            parent = &ThreadPost{...}
        }

        // Try activity (federated post)
        err, activity := database.ReadActivityByObjectURI(parentURI)
        if err == nil && activity != nil {
            content, author := parseActivityContent(activity)
            parent = &ThreadPost{...}
        }

        // Load local replies
        err, localReplies := database.ReadRepliesByURI(parentURI)

        // Load remote replies
        err, remoteReplies := database.ReadActivitiesByInReplyTo(parentURI)

        // Sort replies by time
        sort.Slice(replies, func(i, j int) bool {
            return replies[i].Time.Before(replies[j].Time)
        })

        return threadLoadedMsg{parent, replies, nil}
    }
}
```

### By Note ID

For local notes without URI:

```go
func loadThreadByID(noteID uuid.UUID, noteURI, author, content string, createdAt time.Time) tea.Cmd {
    return func() tea.Msg {
        // Create parent from provided data
        parent := &ThreadPost{
            ID:        noteID,
            Author:    author,
            Content:   content,
            Time:      createdAt,
            IsLocal:   true,
            IsParent:  true,
        }

        // Load local replies by note ID
        err, localReplies := database.ReadRepliesByNoteId(noteID)

        // Load remote replies using canonical URI
        canonicalURI := fmt.Sprintf("https://%s/notes/%s", domain, noteID)
        err, remoteReplies := database.ReadActivitiesByInReplyTo(canonicalURI)

        return threadLoadedMsg{parent, replies, nil}
    }
}
```

---

## Deleted Post Handling

If parent post is deleted but has replies:

```go
if parent == nil && len(replies) > 0 {
    parent = &ThreadPost{
        Author:     "[deleted]",
        Content:    "This post has been deleted",
        IsParent:   true,
        IsDeleted:  true,
        ReplyCount: len(replies),
    }
}
```

---

## Reply Action

Press `r` to reply to the selected post:

```go
case "r":
    if m.Selected == -1 && m.ParentPost != nil && !m.ParentPost.IsDeleted {
        // Reply to parent
        replyURI := m.ParentPost.ObjectURI
        if replyURI == "" && m.ParentPost.IsLocal {
            replyURI = "local:" + m.ParentPost.ID.String()
        }
        return m, func() tea.Msg {
            return common.ReplyToNoteMsg{
                NoteURI: replyURI,
                Author:  m.ParentPost.Author,
                Preview: truncateFirstLine(m.ParentPost.Content),
            }
        }
    } else if m.Selected >= 0 && m.Selected < len(m.Replies) {
        // Reply to selected reply
        reply := m.Replies[m.Selected]
        // ... similar logic
    }
```

---

## Nested Thread Navigation

Press `Enter` to open a reply as a new thread:

```go
case "enter":
    if m.Selected >= 0 && m.Selected < len(m.Replies) {
        reply := m.Replies[m.Selected]

        // Skip if no replies
        if reply.ReplyCount == 0 {
            return m, nil
        }

        return m, func() tea.Msg {
            return common.ViewThreadMsg{
                NoteURI:   reply.ObjectURI,
                NoteID:    reply.ID,
                Author:    reply.Author,
                Content:   reply.Content,
                CreatedAt: reply.Time,
                IsLocal:   reply.IsLocal,
            }
        }
    }
```

---

## Like/Unlike

Press `l` to toggle like:

```go
case "l":
    if m.Selected == -1 && m.ParentPost != nil && !m.ParentPost.IsDeleted {
        // Like parent
        return m, func() tea.Msg {
            return common.LikeNoteMsg{
                NoteURI: m.ParentPost.ObjectURI,
                NoteID:  m.ParentPost.ID,
                IsLocal: m.ParentPost.IsLocal,
            }
        }
    } else if m.Selected >= 0 && m.Selected < len(m.Replies) {
        // Like reply
        reply := m.Replies[m.Selected]
        // ... similar logic
    }
```

---

## URL Toggle

Press `o` to toggle between content and URL. The display prefers `ObjectURL` (web UI link) over `ObjectURI` (JSON endpoint) when available:

```go
case "o":
    if m.Selected == -1 && m.ParentPost != nil {
        // Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
        displayURL := m.ParentPost.ObjectURL
        if displayURL == "" {
            displayURL = m.ParentPost.ObjectURI
        }
        if util.IsURL(displayURL) {
            m.showingURL = !m.showingURL
        }
    } else if m.Selected >= 0 && m.Selected < len(m.Replies) {
        reply := m.Replies[m.Selected]
        displayURL := reply.ObjectURL
        if displayURL == "" {
            displayURL = reply.ObjectURI
        }
        if util.IsURL(displayURL) {
            m.showingURL = !m.showingURL
        }
    }
```

### URL Display Rendering

When `showingURL` is true, the post content is replaced with a clickable URL. The display prefers `ObjectURL` (web UI link) over `ObjectURI` (JSON endpoint):

```go
// Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
displayURL := post.ObjectURL
if displayURL == "" {
    displayURL = post.ObjectURI
}
if m.showingURL && displayURL != "" {
    osc8Link := util.FormatClickableURL(displayURL, common.MaxContentTruncateWidth, "🔗 ")
    hintText := "(Cmd+click to open, press 'o' to toggle back)"

    contentStyleBg := lipgloss.NewStyle().
        Background(lipgloss.Color(common.COLOR_ACCENT)).
        Foreground(lipgloss.Color(common.COLOR_WHITE)).
        Width(itemWidth)
    contentFormatted = contentStyleBg.Render(osc8Link + "\n\n" + hintText)
}
```

Uses `util.FormatClickableURL()` for OSC 8 terminal hyperlinks. The `ObjectURL` field is preferred because it points to the human-readable web page, while `ObjectURI` typically returns JSON.

---

## Reload on Like

When notes are liked, the thread reloads to update counts:

```go
case common.SessionState:
    if msg == common.UpdateNoteList && m.isActive && m.ParentURI != "" {
        // Store current selection to restore after reload
        m.pendingSelection = m.Selected
        m.pendingOffset = m.Offset

        // Reload thread
        if m.parentIsLocal && m.parentNoteID != uuid.Nil {
            return m, loadThreadByID(m.parentNoteID, m.ParentURI, ...)
        }
        return m, loadThread(m.ParentURI)
    }

case threadLoadedMsg:
    // Restore selection after reload
    if m.pendingSelection != -2 {
        m.Selected = m.pendingSelection
        m.Offset = m.pendingOffset
        m.pendingSelection = -2
        m.pendingOffset = -2
    }
```

---

## Activity Content Parsing

Remote posts are parsed from ActivityPub JSON:

```go
func parseActivityContent(activity *domain.Activity) (string, string) {
    content := ""
    author := activity.ActorURI

    // Try to get better author name from cached remote account
    err, remoteAcc := database.ReadRemoteAccountByActorURI(activity.ActorURI)
    if err == nil && remoteAcc != nil {
        author = "@" + remoteAcc.Username + "@" + remoteAcc.Domain
    }

    // Parse raw JSON for content
    if activity.RawJSON != "" {
        var wrapper struct {
            Object struct {
                Content string `json:"content"`
            } `json:"object"`
        }
        json.Unmarshal([]byte(activity.RawJSON), &wrapper)
        content = util.StripHTMLTags(wrapper.Object.Content)
    }

    return content, author
}
```

---

## Error and Loading States

```go
func (m Model) View() tea.View {
    if m.loading {
        return emptyStyle.Render("Loading thread...")
    }

    if m.errorMessage != "" {
        return emptyStyle.Render("Error: " + m.errorMessage) +
            "\n\n" + common.HelpStyle.Render("esc: back")
    }

    if m.ParentPost == nil {
        return emptyStyle.Render("No thread to display") +
            "\n\n" + common.HelpStyle.Render("esc: back")
    }

    // Render thread...
}
```

---

## Selection Styling

Selected posts use inverted colors (same as other views):

```go
if isSelected {
    selectedBg := lipgloss.NewStyle().
        Background(lipgloss.Color(common.COLOR_ACCENT)).
        Width(itemWidth)

    timeFormatted := selectedBg.Render(selectedReplyTimeStyle.Render(timeStr))
    authorFormatted := selectedBg.Render(selectedReplyAuthorStyle.Render(author))
    contentFormatted := selectedBg.Render(selectedReplyContentStyle.Render(content))
}
```

---

## Return Navigation

Press `Esc` or `q` to return to home timeline:

```go
case "esc", "q":
    return m, func() tea.Msg {
        return common.HomeTimelineView
    }
```

---

## Source Files

- `ui/threadview/threadview.go` - ThreadView implementation
- `ui/threadview/threadview_test.go` - Tests
- `ui/common/commands.go` - ViewThreadMsg, ReplyToNoteMsg, LikeNoteMsg
- `db/db.go` - ReadRepliesByURI, ReadActivitiesByInReplyTo, etc.
