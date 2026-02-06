# GlobalPosts View

This document specifies the GlobalPosts view, which displays the global timeline showing all public posts from local users and federated content.

---

## Overview

The GlobalPosts view shows a unified timeline of all public content:
- Local posts from all users on this server
- Federated posts from relay subscriptions
- Remote posts boosted by followed users

This view is only available when `STEGODON_SHOW_GLOBAL=true` is set.

Posts are sorted in reverse chronological order with automatic refresh every 30 seconds.

---

## Data Structure

```go
type Model struct {
    AccountId          uuid.UUID
    Posts              []domain.GlobalTimelinePost
    Offset             int              // Pagination offset
    Selected           int              // Currently selected post index
    Width              int
    Height             int
    isActive           bool             // Track if view is visible
    tickerRunning      bool             // Prevents multiple ticker chains
    showingURL         bool             // Toggle between content and URL display
    showingEngagement  bool             // Toggle engagement info display
    engagementLikers   []string         // Users who liked selected post
    engagementBoosters []string         // Users who boosted selected post
    LocalDomain        string           // For mention highlighting
}
```

### GlobalTimelinePost Structure

```go
type GlobalTimelinePost struct {
    Id          string       // Activity/Note ID
    NoteId      string       // Local note UUID (empty for remote)
    Username    string       // @user or @user@domain
    UserDomain  string       // Domain for remote users
    ProfileURL  string       // Profile web URL
    Message     string       // Post content
    ObjectURI   string       // ActivityPub canonical URI
    ObjectURL   string       // Human-readable web URL
    IsRemote    bool         // true = remote activity
    CreatedAt   time.Time
    ReplyCount  int
    LikeCount   int
    BoostCount  int
    BoostedBy        string  // "@user@domain" if this is a boosted post
    ProcessedContent string  // Pre-processed content for terminal display (cached to avoid re-processing in View)
}
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ global (128 posts)                                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ 2h ago · 3 replies · ⭐ 5 · 🔁 2                           │  ← Selected
│   @alice                                                     │  ← Local (primary color)
│   Just posted something interesting! #programming            │
│                                                              │
│   5h ago · ⭐ 12                                              │
│   🔁 boosted by @bob@mastodon.social                         │  ← Boost indicator
│   @charlie@remote.instance                                   │  ← Remote (secondary color)
│   Original content from remote user                          │
│                                                              │
│   1d ago                                                     │
│   @dave                                                      │
│   Another local post                                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Boost Indicator

Posts boosted by followed remote users display a boost indicator:

```go
boostIndicatorStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color(common.COLOR_DIM)).
    Italic(true)

if post.BoostedBy != "" {
    boostLine := fmt.Sprintf("🔁 boosted by %s", post.BoostedBy)
    s.WriteString(boostIndicatorStyle.Render(boostLine))
}
```

---

## Navigation

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Open thread (if has replies) |
| `r` | Reply to selected post |
| `l` | Like/unlike selected post |
| `b` | Boost/unboost selected post |
| `i` | Toggle engagement info (likers/boosters) |
| `o` | Toggle URL display |
| `f` | Navigate to follow view (for remote users) |

---

## Boost/Unboost

Press `b` to toggle boost on the selected post:

```go
case "b":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        noteURI := selectedPost.ObjectURI
        var noteID uuid.UUID

        if !selectedPost.IsRemote && selectedPost.NoteId != "" {
            if id, err := uuid.Parse(selectedPost.NoteId); err == nil {
                noteID = id
                if noteURI == "" {
                    noteURI = "local:" + selectedPost.NoteId
                }
            }
        }

        if noteURI != "" || noteID != uuid.Nil {
            return m, func() tea.Msg {
                return common.BoostNoteMsg{
                    NoteURI: noteURI,
                    NoteID:  noteID,
                    IsLocal: !selectedPost.IsRemote,
                }
            }
        }
    }
```

---

## Engagement Info Display

Press `i` to toggle display of likers and boosters:

```go
case "i":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        if selectedPost.LikeCount > 0 || selectedPost.BoostCount > 0 {
            m.showingEngagement = !m.showingEngagement
            if m.showingEngagement {
                return m, loadEngagementInfoGlobal(selectedPost)
            }
        }
    }
```

### Engagement Display

```
┌─────────────────────────────────────────────────────────────┐
│ ▸ 2h ago · ⭐ 5 · 🔁 2                                       │
│   @alice                                                     │
│   ⭐ Liked by:                                               │
│     @bob                                                     │
│     @charlie@mastodon.social                                 │
│   🔁 Boosted by:                                             │
│     @dave@remote.instance                                    │
│                                                              │
│   (press 'i' to toggle back)                                │
└─────────────────────────────────────────────────────────────┘
```

---

## Like/Unlike

Press `l` to toggle like on the selected post:

```go
case "l":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        noteURI := selectedPost.ObjectURI
        var noteID uuid.UUID

        if !selectedPost.IsRemote && selectedPost.NoteId != "" {
            if id, err := uuid.Parse(selectedPost.NoteId); err == nil {
                noteID = id
                if noteURI == "" {
                    noteURI = "local:" + selectedPost.NoteId
                }
            }
        }

        if noteURI != "" || noteID != uuid.Nil {
            return m, func() tea.Msg {
                return common.LikeNoteMsg{
                    NoteURI: noteURI,
                    NoteID:  noteID,
                    IsLocal: !selectedPost.IsRemote,
                }
            }
        }
    }
```

---

## URL Toggle

Press `o` to toggle between content and clickable URL:

```go
case "o":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        displayURL := selectedPost.ObjectURL
        if displayURL == "" {
            displayURL = selectedPost.ObjectURI
        }
        if util.IsURL(displayURL) {
            m.showingURL = !m.showingURL
        }
    }
```

---

## Reply Mode

Press `r` to reply to the selected post:

```go
case "r":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        replyURI := selectedPost.ObjectURI
        if !selectedPost.IsRemote && replyURI == "" {
            replyURI = "local:" + selectedPost.NoteId
        }

        if replyURI != "" {
            return m, func() tea.Msg {
                return common.ReplyToNoteMsg{
                    NoteURI: replyURI,
                    Author:  selectedPost.Username,
                    Preview: truncateFirstLine(selectedPost.Message),
                }
            }
        }
    }
```

---

## Thread View

Press `Enter` to open thread (only if post has replies):

```go
case "enter":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        if selectedPost.ReplyCount == 0 {
            return m, nil
        }

        noteURI := selectedPost.ObjectURI
        var noteID uuid.UUID
        if !selectedPost.IsRemote {
            if id, err := uuid.Parse(selectedPost.NoteId); err == nil {
                noteID = id
                if noteURI == "" {
                    noteURI = "local:" + selectedPost.NoteId
                }
            }
        }

        return m, func() tea.Msg {
            return common.ViewThreadMsg{
                NoteURI:   noteURI,
                NoteID:    noteID,
                Author:    selectedPost.Username,
                Content:   selectedPost.Message,
                CreatedAt: selectedPost.CreatedAt,
                IsLocal:   !selectedPost.IsRemote,
            }
        }
    }
```

---

## Auto-Refresh Pattern

The timeline auto-refreshes while active:

```go
const TimelineRefreshSeconds = 30

type refreshTickMsg struct{}

func tickRefresh() tea.Cmd {
    return tea.Tick(TimelineRefreshSeconds * time.Second, func(t time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}
```

### Ticker Lifecycle

```go
case refreshTickMsg:
    if m.isActive {
        return m, loadGlobalPosts()
    }
    return m, nil

case postsLoadedMsg:
    m.Posts = msg.posts
    // Only start ticker if active AND no ticker already running
    if m.isActive && !m.tickerRunning {
        m.tickerRunning = true
        return m, tickRefresh()
    }
    return m, nil
```

---

## Data Loading

```go
func loadGlobalPosts() tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, posts := database.ReadGlobalTimelinePosts(common.GlobalTimelinePostLimit, 0)
        if err != nil {
            return postsLoadedMsg{posts: []domain.GlobalTimelinePost{}}
        }
        return postsLoadedMsg{posts: *posts}
    }
}
```

---

## Content Processing

### Remote Content

Remote posts have HTML stripped and converted:

```go
if post.IsRemote {
    processedContent = util.StripHTMLTags(processedContent)
    processedContent = util.UnescapeHTML(processedContent)
    processedContent = util.MarkdownLinksToTerminal(processedContent)
}
processedContent = util.LinkifyRawURLsTerminal(processedContent)
highlightedContent := util.HighlightHashtagsTerminal(processedContent)
highlightedContent = util.HighlightMentionsTerminal(highlightedContent, m.LocalDomain)
```

---

## Empty State

```go
if len(m.Posts) == 0 {
    s.WriteString(emptyStyle.Render("No posts yet."))
}
```

---

## Configuration

Global timeline visibility is controlled by environment variable:

```bash
STEGODON_SHOW_GLOBAL=true ./stegodon
```

When disabled, the global tab is hidden from the TUI navigation.

---

## Source Files

- `ui/globalposts/globalposts.go` - GlobalPosts view implementation
- `ui/common/commands.go` - BoostNoteMsg, LikeNoteMsg, ReplyToNoteMsg
- `ui/common/constants.go` - GlobalTimelinePostLimit, TimelineRefreshSeconds
- `db/db.go` - ReadGlobalTimelinePosts
