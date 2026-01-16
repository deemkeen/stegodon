# Templates & Assets

This document specifies the embedded HTML templates and static files for the web UI.

---

## Overview

Stegodon embeds all templates and static assets directly into the binary using Go's `embed` package. This enables:
- Single binary distribution
- No external file dependencies
- Consistent asset versions

---

## Embedded Assets

### Template Embedding

```go
//go:embed templates/*.html
var embeddedTemplates embed.FS
```

### Static Asset Embedding

```go
//go:embed static/stegologo.png
var embeddedLogo []byte

//go:embed static/style.css
var embeddedCSS []byte
```

---

## Template Files

| Template | Handler | Purpose |
|----------|---------|---------|
| `index.html` | HandleIndex | Home timeline |
| `profile.html` | HandleProfile | User profile page |
| `post.html` | HandleSinglePost | Single post with thread |
| `tag.html` | HandleTagFeed | Hashtag feed |

---

## Template Loading

```go
tmpl, err := template.ParseFS(embeddedTemplates, "templates/*.html")
if err != nil {
    log.Fatal("Failed to parse templates:", err)
}
g.SetHTMLTemplate(tmpl)
```

---

## Template Structure

All templates follow the same structure:

```html
{{define "template-name.html"}}
<!doctype html>
<html lang="en">
    <head>
        <!-- Meta tags, SEO, Open Graph -->
        <link rel="stylesheet" href="/static/style.css">
    </head>
    <body>
        <div class="layout">
            <div class="sidebar"><!-- Sidebar content --></div>
            <div class="main-content"><!-- Page content --></div>
        </div>
        <script><!-- Mobile toggle handler --></script>
    </body>
</html>
{{end}}
```

---

## Common Template Data

All templates receive these common fields:

| Field | Type | Description |
|-------|------|-------------|
| `Title` | string | Page title |
| `Host` | string | Server hostname |
| `SSHPort` | int | SSH connection port |
| `Version` | string | Application version |

---

## Page-Specific Data

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
    ParentPost *PostView   // nil if not a reply
    Replies    []PostView
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
    MessageHTML  template.HTML  // Safe HTML
    TimeAgo      string
    InReplyToURI string
    ReplyCount   int
    LikeCount    int
    BoostCount   int
}
```

---

## SEO Meta Tags

Templates include comprehensive SEO metadata:

### Standard Meta Tags

```html
<meta name="description" content="..." />
<meta name="keywords" content="fediverse, activitypub, ssh, blog, ..." />
<meta name="author" content="stegodon" />
<link rel="canonical" href="https://{{.Host}}/" />
```

### Open Graph Tags

```html
<meta property="og:type" content="website" />
<meta property="og:url" content="https://{{.Host}}/" />
<meta property="og:title" content="{{.Title}} - stegodon" />
<meta property="og:description" content="..." />
<meta property="og:site_name" content="stegodon" />
```

### Twitter Card Tags

```html
<meta name="twitter:card" content="summary" />
<meta name="twitter:url" content="https://{{.Host}}/" />
<meta name="twitter:title" content="{{.Title}}" />
<meta name="twitter:description" content="..." />
```

---

## Favicon

Uses inline SVG data URI:

```html
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><rect width='100' height='100' fill='%23000'/><text x='50' y='70' text-anchor='middle' font-family='monospace' font-size='70' font-weight='bold' fill='%2300ff7f'>S</text></svg>">
```

Green "S" on black background, monospace font.

---

## Static Assets

### Logo

**Route:** `GET /static/stegologo.png`

```go
g.GET("/static/stegologo.png", func(c *gin.Context) {
    c.Header("Content-Type", "image/png")
    c.Header("Cache-Control", "public, max-age=86400")  // 24 hours
    c.Data(200, "image/png", embeddedLogo)
})
```

### Stylesheet

**Route:** `GET /static/style.css`

```go
g.GET("/static/style.css", func(c *gin.Context) {
    c.Header("Content-Type", "text/css; charset=utf-8")
    c.Data(200, "text/css; charset=utf-8", embeddedCSS)
})
```

---

## CSS Structure

### Layout System

```css
.layout {
    display: flex;
    height: 100vh;
}

.sidebar {
    width: 350px;
    position: fixed;
    height: 100vh;
}

.main-content {
    margin-left: 350px;
    flex: 1;
}
```

### Color Scheme

| Element | Color |
|---------|-------|
| Background | `#000` (black) |
| Sidebar | `#111` |
| Text | `#e0e0e0` |
| Primary accent | `#00ff7f` (spring green) |
| Links | `#5fafff` (light blue) |
| Borders | `#333` |
| Muted text | `#666` |

### Font Stack

```css
font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
    "Liberation Mono", "Courier New", monospace;
```

---

## Responsive Design

### Mobile Breakpoint

```css
@media (max-width: 768px) {
    .sidebar {
        width: 100%;
        position: sticky;
        top: 0;
    }
    .main-content {
        margin-left: 0;
    }
}
```

### Sidebar Toggle

Mobile sidebar is collapsible with hamburger menu:

```javascript
const toggleBtn = document.querySelector(".toggle-btn");
toggleBtn.addEventListener("click", (e) => {
    e.stopPropagation();
    sidebar.classList.toggle("minimized");
});
```

---

## Post Display Components

### Post Card

```html
<div class="post">
    <div class="post-meta">
        <a href="/u/{{.Username}}" class="post-author">@{{.Username}}</a>
        <a href="/u/{{.Username}}/{{.NoteId}}" class="post-permalink">#</a>
    </div>
    <div class="post-content">
        <p class="post-time">{{.TimeAgo}}</p>
        <p class="post-text">{{.MessageHTML}}</p>
    </div>
    {{if engagement}}
    <div class="post-footer">
        <span class="like-count">{{.LikeCount}}</span>
        <span class="boost-count">{{.BoostCount}}</span>
        <span class="reply-count">{{.ReplyCount}}</span>
    </div>
    {{end}}
</div>
```

### Terminal-Style Prompts

Post text uses terminal-style prefix:

```css
.post-text::before {
    content: "$";
    color: #5fafff;
}

.post-time::before {
    content: "# ";
}
```

---

## Thread Display

### Parent Post Context

```html
{{if .ParentPost}}
<div class="thread-context">
    <div class="reply-indicator">In reply to:</div>
    <div class="post parent-post"><!-- Parent post --></div>
</div>
{{end}}
```

### Styling

```css
.parent-post {
    border-left: 3px solid #5fafff;
    opacity: 0.8;
}

.main-post {
    border-left: 3px solid #00ff7f;
}

.reply-post {
    margin-left: 20px;
    border-left: 3px solid #444;
}
```

---

## Pagination

```html
<div class="pagination">
    {{if .HasPrev}}
    <a href="/?page={{.PrevPage}}">previous</a>
    {{else}}
    <span>previous</span>
    {{end}}

    {{if .HasNext}}
    <a href="/?page={{.NextPage}}">next</a>
    {{else}}
    <span>next</span>
    {{end}}
</div>
```

---

## Empty States

```html
{{if not .Posts}}
<div class="empty-state">
    no posts yet. connect via SSH and be the first to post!
</div>
{{end}}
```

---

## Hashtag Styling

```css
.hashtag {
    color: #5fafff;
    text-decoration: none;
    font-weight: 500;
}

.hashtag:hover {
    text-decoration: underline;
    color: #7fc8ff;
}
```

---

## Info Box Rendering

Info boxes are rendered in the sidebar on all pages:

### Template Structure

```html
<div class="sidebar">
    {{range .InfoBoxes}}
    <div class="info-section">
        <h3>{{.TitleHTML}}</h3>
        {{.ContentHTML}}
    </div>
    {{end}}
</div>
```

### CSS Styling

```css
.info-section {
    background: #111;
    padding: 20px;
    margin-bottom: 20px;
    border-radius: 4px;
    border: 1px solid #333;
}

.info-section h3 {
    color: #00ff7f;
    margin-top: 0;
    margin-bottom: 15px;
    font-size: 1em;
    display: flex;
    align-items: center;
}

.info-section p {
    margin: 10px 0;
}

.info-section pre {
    background: #000;
    padding: 10px;
    border-radius: 4px;
    overflow-x: auto;
}

.info-section code {
    background: #333;
    padding: 2px 6px;
    border-radius: 3px;
}

.info-section a {
    color: #5fafff;
}
```

### Content Features

| Feature | Rendering |
|---------|-----------|
| Markdown headers | Styled `<h1>`-`<h6>` |
| Code blocks | `<pre><code>` with dark background |
| Links | Blue color, opens in new tab |
| Lists | Standard HTML lists |
| SVG icons in title | Inline rendering |

### Placeholder Replacement

Before rendering, placeholders are replaced:

| Placeholder | Replaced With |
|-------------|---------------|
| `{{SSH_PORT}}` | Configured SSH port value |

---

## Source Files

- `web/templates/index.html` - Home timeline
- `web/templates/profile.html` - User profile
- `web/templates/post.html` - Single post view
- `web/templates/tag.html` - Hashtag feed
- `web/static/style.css` - Stylesheet
- `web/static/stegologo.png` - Logo image
- `web/router.go` - Asset embedding and serving
- `web/ui.go` - Template rendering handlers
