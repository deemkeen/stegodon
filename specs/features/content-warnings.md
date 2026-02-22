# Content Warnings

This document specifies sensitive content handling and content warning display.

---

## Overview

Stegodon supports content warnings (CW) for notes containing sensitive content:
- `Sensitive` boolean flag
- `ContentWarning` text field
- Visual indication in TUI and web

---

## Data Model

### Note Structure

```go
type Note struct {
    // ... other fields
    Sensitive      bool   // Contains sensitive content
    ContentWarning string // Content warning text
    // ...
}
```

### Database Schema

```sql
CREATE TABLE notes (
    -- ... other columns
    sensitive INTEGER DEFAULT 0,
    content_warning TEXT DEFAULT '',
    -- ...
);
```

---

## Content Warning Fields

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `Sensitive` | bool | `false` | Marks post as sensitive |
| `ContentWarning` | string | `""` | Warning text to display |

---

## ActivityPub Integration

### Incoming Activities

Content warnings are parsed from ActivityPub `Create` activities:

```json
{
  "type": "Create",
  "object": {
    "type": "Note",
    "sensitive": true,
    "summary": "CW: Spoilers for movie",
    "content": "The actual content..."
  }
}
```

| ActivityPub Field | Maps To |
|-------------------|---------|
| `object.sensitive` | `Sensitive` |
| `object.summary` | `ContentWarning` |

### Outgoing Activities

When federating notes with content warnings:

```json
{
  "type": "Create",
  "object": {
    "type": "Note",
    "sensitive": true,
    "summary": "Content warning text",
    "content": "The note content..."
  }
}
```

---

## Display Behavior

### TUI Display

Content warnings are displayed with warning styling:

```go
const (
    COLOR_WARNING      = "#ffaf00"              // Content warnings, caution (amber)
    ANSI_WARNING_START = "\033[38;2;255;175;0m" // True-color #ffaf00
    ANSI_COLOR_RESET   = "\033[39m"             // Reset foreground to default
)
```

### Warning Style

| Element | Color |
|---------|-------|
| CW indicator | Amber (#ffaf00) |
| CW text | Amber (#ffaf00) |
| Content | Normal (revealed) |

---

## Storage

### Database Storage

```sql
INSERT INTO notes (sensitive, content_warning, ...)
VALUES (1, 'Spoilers', ...);
```

### Activity Storage

For remote posts, the content warning is preserved in `RawJSON`:

```go
type Activity struct {
    // ...
    RawJSON   string // Full activity JSON including summary field
    // ...
}
```

---

## Visibility Interaction

Content warnings work independently of visibility settings:

| Visibility | CW Applies |
|------------|------------|
| Public | Yes |
| Unlisted | Yes |
| Followers | Yes |
| Direct | Yes |

---

## Web Display

### HTML Template

Content warnings appear before content in web views:

```html
{{if .Note.Sensitive}}
<div class="content-warning">
    <strong>CW:</strong> {{.Note.ContentWarning}}
</div>
{{end}}
<div class="content">
    {{.Note.Message}}
</div>
```

### CSS Styling

```css
.content-warning {
    background-color: #2a2a2a;
    padding: 8px;
    border-left: 3px solid #ffaf00;
    margin-bottom: 10px;
}
```

---

## RSS Feed

### Feed Item

Content warnings are included in RSS item descriptions:

```xml
<item>
    <title>CW: Spoilers</title>
    <description>
        <![CDATA[
        <p><strong>Content Warning:</strong> Spoilers</p>
        <p>The actual note content...</p>
        ]]>
    </description>
</item>
```

---

## Current Status

### Implemented

| Feature | Status |
|---------|--------|
| Data model fields | Yes |
| Database storage | Yes |
| ActivityPub receive | Yes |
| ActivityPub send | Partial |
| TUI warning colors | Yes |

### Not Yet Implemented

| Feature | Status |
|---------|--------|
| TUI CW input field | No |
| Content collapse | No |
| Click to reveal | No |
| CW search/filter | No |

---

## Future Enhancements

### Planned Features

1. **CW Input in TUI** - Add field in writenote for content warning text
2. **Collapse by default** - Hide content behind "Show more" for CW posts
3. **User preference** - Option to auto-expand CW posts
4. **CW filtering** - Option to hide all CW posts

---

## Source Files

- `domain/notes.go` - `Sensitive`, `ContentWarning` fields
- `db/db.go` - Database schema with `sensitive`, `content_warning`
- `activitypub/inbox.go` - Parse `summary` from incoming activities
- `ui/common/styles.go` - `COLOR_WARNING` constant
