# Web UI Handlers

This document specifies the web UI handlers for profile pages, post views, and tag feeds.

---

## Overview

Web UI handlers render HTML pages using embedded templates. They provide:
- Public timeline (home page)
- User profile pages
- Single post views with threading
- Hashtag feeds
- Pagination for all list views

---

## Page Data Structures

### IndexPageData

```go
type IndexPageData struct {
    Title     string
    Host      string
    SSHPort   int
    Version   string
    Posts     []PostView
    HasPrev   bool
    HasNext   bool
    PrevPage  int
    NextPage  int
    InfoBoxes []InfoBoxView
}
```

### ProfilePageData

```go
type ProfilePageData struct {
    Title      string
    Host       string
    SSHPort    int
    Version    string
    User       UserView
    Posts      []PostView
    TotalPosts int
    HasPrev    bool
    HasNext    bool
    PrevPage   int
    NextPage   int
    InfoBoxes  []InfoBoxView
}
```

### SinglePostPageData

```go
type SinglePostPageData struct {
    Title      string
    Host       string
    SSHPort    int
    Version    string
    Post       PostView
    User       UserView
    ParentPost *PostView   // Parent if reply (nil otherwise)
    Replies    []PostView  // Replies to this post
    InfoBoxes  []InfoBoxView
}
```

### TagPageData

```go
type TagPageData struct {
    Title      string
    Host       string
    SSHPort    int
    Version    string
    Tag        string
    Posts      []PostView
    TotalPosts int
    HasPrev    bool
    HasNext    bool
    PrevPage   int
    NextPage   int
    InfoBoxes  []InfoBoxView
}
```

### InfoBoxView

```go
type InfoBoxView struct {
    Title       string        // Plain text title (auto-escaped)
    TitleHTML   template.HTML // Title rendered as markdown/HTML
    ContentHTML template.HTML // Sanitized HTML content from markdown
}
```

---

## View Models

### UserView

```go
type UserView struct {
    Username    string
    DisplayName string
    Summary     string
    JoinedAgo   string
}
```

### PostView

```go
type PostView struct {
    NoteId       string
    Username     string
    Message      string
    MessageHTML  template.HTML  // HTML-rendered with links
    TimeAgo      string
    InReplyToURI string         // Parent post URI
    ReplyCount   int
    LikeCount    int
    BoostCount   int
}
```

---

## Handlers

### HandleIndex

**Route:** `GET /`

Displays the public timeline with all top-level posts (excludes replies).

```go
func HandleIndex(c *gin.Context, conf *util.AppConfig) {
    // 1. Parse pagination (?page=1)
    // 2. Get all notes from database
    // 3. Filter out replies (InReplyToURI != "")
    // 4. Apply pagination (20 posts per page)
    // 5. Convert to PostView with HTML rendering
    // 6. Render index.html
}
```

**Features:**
- Filters replies from timeline
- 20 posts per page
- Markdown link conversion
- Hashtag highlighting
- Mention highlighting

---

### HandleProfile

**Route:** `GET /u/:username`

Displays a user's profile with their posts.

```go
func HandleProfile(c *gin.Context, conf *util.AppConfig) {
    username := c.Param("username")

    // 1. Get user account by username
    // 2. Return 404 if not found
    // 3. Parse pagination
    // 4. Get user's notes
    // 5. Filter out replies
    // 6. Apply pagination
    // 7. Render profile.html
}
```

**Features:**
- User info (display name, summary, join date)
- User's top-level posts only
- Paginated (20 per page)

---

### HandleSinglePost

**Route:** `GET /u/:username/:noteid`

Displays a single post with parent (if reply) and replies.

```go
func HandleSinglePost(c *gin.Context, conf *util.AppConfig) {
    username := c.Param("username")
    noteIdStr := c.Param("noteid")

    // 1. Parse note ID (UUID)
    // 2. Get user account
    // 3. Get the note
    // 4. Verify note belongs to user
    // 5. Fetch parent post if reply
    // 6. Fetch replies to this post
    // 7. Render post.html
}
```

**Features:**
- Full post content with HTML
- Parent post context (if reply)
- All direct replies
- User info

---

### HandleTagFeed

**Route:** `GET /tags/:tag`

Displays all posts containing a specific hashtag.

```go
func HandleTagFeed(c *gin.Context, conf *util.AppConfig) {
    tag := c.Param("tag")

    // 1. Parse pagination
    // 2. Count total posts with hashtag
    // 3. Get paginated notes by hashtag
    // 4. Convert to PostView
    // 5. Render tag.html
}
```

**Features:**
- Hashtag in title
- All posts with that tag
- Paginated (20 per page)

---

## Content Rendering

### Markdown Link Conversion

```go
messageHTML := util.MarkdownLinksToHTML(note.Message)
```

Converts `[text](url)` to `<a href="url">text</a>`.

### Hashtag Highlighting

```go
messageHTML = util.HighlightHashtagsHTML(messageHTML)
```

Converts `#tag` to clickable links pointing to `/tags/tag`.

### Mention Highlighting

```go
messageHTML = util.HighlightMentionsHTML(messageHTML, conf.Conf.SslDomain)
```

Converts `@user@domain` to appropriate links.

---

## Time Formatting

```go
func formatTimeAgo(t time.Time) string {
    duration := time.Since(t)

    if duration < time.Minute {
        return "just now"
    } else if duration < time.Hour {
        mins := int(duration.Minutes())
        if mins == 1 {
            return "1 minute ago"
        }
        return fmt.Sprintf("%d minutes ago", mins)
    } else if duration < 24*time.Hour {
        hours := int(duration.Hours())
        if hours == 1 {
            return "1 hour ago"
        }
        return fmt.Sprintf("%d hours ago", hours)
    } else if duration < 30*24*time.Hour {
        days := int(duration.Hours() / 24)
        if days == 1 {
            return "1 day ago"
        }
        return fmt.Sprintf("%d days ago", days)
    } else {
        return t.Format("Jan 2, 2006")
    }
}
```

| Duration | Format |
|----------|--------|
| < 1 minute | "just now" |
| 1 minute | "1 minute ago" |
| < 1 hour | "X minutes ago" |
| 1 hour | "1 hour ago" |
| < 24 hours | "X hours ago" |
| 1 day | "1 day ago" |
| < 30 days | "X days ago" |
| >= 30 days | "Jan 2, 2006" |

---

## Pagination

### Query Parameter

```go
page := 1
if pageStr := c.Query("page"); pageStr != "" {
    if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
        page = p
    }
}
```

### Pagination Logic

```go
postsPerPage := 20
offset := (page - 1) * postsPerPage

// Apply pagination
start := offset
end := offset + postsPerPage
if start > totalPosts {
    start = totalPosts
}
if end > totalPosts {
    end = totalPosts
}

paginatedNotes := notes[start:end]

// Template data
HasPrev:  page > 1,
HasNext:  end < totalPosts,
PrevPage: page - 1,
NextPage: page + 1,
```

---

## Host Selection

Uses SSL domain when federation is enabled:

```go
host := conf.Conf.Host
if conf.Conf.WithAp {
    host = conf.Conf.SslDomain
}
```

---

## Reply Filtering

Top-level posts only (no replies in main feeds):

```go
var topLevelNotes []domain.Note
for _, note := range *notes {
    if note.InReplyToURI == "" {
        topLevelNotes = append(topLevelNotes, note)
    }
}
```

---

## Reply Count

Fetched for each post:

```go
replyCount := 0
if count, err := database.CountRepliesByNoteId(note.Id); err == nil {
    replyCount = count
}
```

---

## Error Handling

### User Not Found

```go
if err != nil {
    log.Printf("User not found: %s", username)
    c.HTML(404, "base.html", gin.H{
        "Title": "Not Found",
        "Error": "User not found",
    })
    return
}
```

### Post Not Found

```go
if err != nil || note == nil {
    log.Printf("Note not found: %s", noteIdStr)
    c.HTML(404, "base.html", gin.H{
        "Title": "Not Found",
        "Error": "Post not found",
    })
    return
}
```

### Post Ownership Verification

```go
if note.CreatedBy != username {
    log.Printf("Note %s does not belong to user %s", noteIdStr, username)
    c.HTML(404, "base.html", gin.H{
        "Title": "Not Found",
        "Error": "Post not found",
    })
    return
}
```

---

## Templates

| Template | Handler | Purpose |
|----------|---------|---------|
| `index.html` | HandleIndex | Home timeline |
| `profile.html` | HandleProfile | User profile |
| `post.html` | HandleSinglePost | Single post with thread |
| `tag.html` | HandleTagFeed | Hashtag feed |
| `base.html` | (errors) | Error pages |

---

## Info Box Loading

All handlers load info boxes for sidebar display:

```go
// Load info boxes for the page
var infoBoxViews []InfoBoxView
err, infoBoxes := database.ReadEnabledInfoBoxes()
if err == nil && infoBoxes != nil {
    for _, box := range *infoBoxes {
        // Replace placeholders first (e.g., {{SSH_PORT}})
        content := util.ReplacePlaceholders(box.Content, conf.Conf.SshPort)

        // Convert markdown to HTML
        htmlContent := convertMarkdownToHTML(content)

        // Render title as markdown too (for HTML/SVG icons)
        titleHTML := convertMarkdownToHTML(box.Title)

        infoBoxViews = append(infoBoxViews, InfoBoxView{
            Title:       box.Title,
            TitleHTML:   template.HTML(titleHTML),
            ContentHTML: template.HTML(htmlContent),
        })
    }
}
```

### Markdown Conversion

Uses the `gomarkdown/markdown` library with extensions:

```go
func convertMarkdownToHTML(md string) string {
    // Extensions: CommonExtensions, AutoHeadingIDs, NoEmptyLineBeforeBlock, Strikethrough
    extensions := parser.CommonExtensions | parser.AutoHeadingIDs |
                  parser.NoEmptyLineBeforeBlock | parser.Strikethrough
    p := parser.NewWithExtensions(extensions)
    doc := p.Parse([]byte(md))

    // HTML renderer with CommonFlags and HrefTargetBlank
    htmlFlags := html.CommonFlags | html.HrefTargetBlank
    opts := html.RendererOptions{Flags: htmlFlags}
    renderer := html.NewRenderer(opts)

    return string(markdown.Render(doc, renderer))
}
```

### Supported Markdown Features

| Feature | Example |
|---------|---------|
| Headers | `# H1`, `## H2` |
| Bold | `**bold**` |
| Italic | `*italic*` |
| Strikethrough | `~~strikethrough~~` |
| Links | `[text](url)` |
| Code blocks | Triple backticks |
| Inline code | Single backticks |
| Lists | `- item` or `1. item` |

---

## Source Files

- `web/ui.go` - All UI handlers
- `web/templates/*.html` - HTML templates (embedded)
- `web/static/style.css` - Stylesheet (embedded)
- `util/text.go` - Text processing utilities
