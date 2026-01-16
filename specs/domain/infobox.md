# InfoBox

This document specifies the InfoBox domain model for customizable web UI information boxes.

---

## Overview

InfoBox represents a customizable information panel shown on the web interface sidebar. Administrators can:
- Create multiple info boxes with custom titles and content
- Order boxes by display priority
- Enable/disable individual boxes
- Edit content using Markdown

---

## Data Structure

```go
type InfoBox struct {
    Id        uuid.UUID `json:"id"`
    Title     string    `json:"title"`      // Title of the info box (supports HTML for icons)
    Content   string    `json:"content"`    // Content in markdown format
    OrderNum  int       `json:"order_num"`  // Display order (lower numbers first)
    Enabled   bool      `json:"enabled"`    // Whether this box is shown
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

---

## Fields

| Field | Type | Description |
|-------|------|-------------|
| `Id` | `uuid.UUID` | Unique identifier |
| `Title` | `string` | Box title (can include HTML/SVG for icons) |
| `Content` | `string` | Markdown-formatted content |
| `OrderNum` | `int` | Display order (lower = first) |
| `Enabled` | `bool` | Whether box is visible on web UI |
| `CreatedAt` | `time.Time` | When the box was created |
| `UpdatedAt` | `time.Time` | When the box was last modified |

---

## Content Features

### Markdown Support

InfoBox content supports full Markdown:
- Headers (`#`, `##`, etc.)
- Code blocks (inline and fenced)
- Links `[text](url)`
- Lists (ordered and unordered)
- Bold and italic text
- Strikethrough

### Template Placeholders

Content can include placeholders that get replaced at render time:

| Placeholder | Replacement |
|-------------|-------------|
| `{{SSH_PORT}}` | Configured SSH port |

Example usage:
```markdown
Connect via SSH:
```
ssh -p {{SSH_PORT}} your-domain.com
```
```

### Title HTML Support

Titles can include inline HTML/SVG for icons:
```html
<svg style="width: 1.2em; height: 1.2em; vertical-align: middle;" viewBox="0 0 16 16" fill="currentColor">
  <path d="..."/>
</svg>github
```

---

## Default Info Boxes

On first run, three default info boxes are created:

| Order | Title | Purpose |
|-------|-------|---------|
| 1 | "ssh-first fediverse blog" | SSH connection instructions |
| 2 | "features" | Feature list with RSS link |
| 3 | "(github icon) github" | Link to GitHub repository |

---

## Display Behavior

### Ordering

Boxes are sorted by `OrderNum` ascending:
- Lower numbers appear first
- Boxes with same `OrderNum` maintain insertion order

### Visibility

- Only boxes with `Enabled = true` are shown on web UI
- Disabled boxes are still visible in admin panel (for re-enabling)
- Admin panel shows all boxes regardless of enabled status

---

## Rendering Pipeline

```
┌──────────────────┐
│ InfoBox.Content  │
│ (Markdown + {{}} │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ ReplacePlaceholders │
│ {{SSH_PORT}} → 23232 │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ convertMarkdownToHTML │
│ (gomarkdown library)  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ template.HTML    │
│ (safe rendering) │
└──────────────────┘
```

---

## Source Files

- `domain/infobox.go` - InfoBox struct definition
- `db/db.go` - InfoBox CRUD operations
- `db/migrations.go` - Default seeding
- `web/ui.go` - InfoBox loading and rendering
- `ui/admin/admin.go` - InfoBox management UI
