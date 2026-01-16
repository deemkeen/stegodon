# Admin Panel View

This document specifies the Admin Panel view, which provides user and content management capabilities for administrators.

---

## Overview

The Admin Panel is an admin-only view with two main functions:
- **User Management**: View, mute, and kick users
- **Info Box Management**: Create, edit, delete, and toggle web UI info boxes

---

## View Architecture

The admin panel uses a menu-based navigation system:

```
┌─────────────────────────────────────────────────────────────┐
│ admin panel                                                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ MenuView (default)                                          │
│   › user management                                         │
│     info boxes                                              │
│                                                              │
│ ─────────────────────────────────────────────────────────   │
│ enter: select                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Data Structure

```go
type AdminView int

const (
    MenuView AdminView = iota
    UsersView
    InfoBoxesView
)

type Model struct {
    AdminId       uuid.UUID
    CurrentView   AdminView
    MenuSelected  int

    // User management
    Users         []domain.Account
    Selected      int
    Offset        int

    // Info boxes management
    InfoBoxes     []domain.InfoBox
    BoxSelected   int
    BoxOffset     int
    Editing       bool
    EditBox       *domain.InfoBox
    EditField     int              // 0=Title, 1=Content, 2=Order
    TitleInput    textarea.Model
    ContentInput  textarea.Model
    OrderInput    textarea.Model
    ConfirmDelete bool
    DeleteBoxId   uuid.UUID

    Width         int
    Height        int
    Status        string
    Error         string
}
```

---

## Menu View

### Layout

```
admin panel
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

› user management
  info boxes
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Enter selected submenu |

---

## User Management View

### Layout

```
admin panel > users (5 users)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

› @alice [ADMIN]
  @bob
  @charlie [MUTED]
  @diana
  @eve

User muted and posts deleted
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `m` | Mute selected user |
| `K` | Kick selected user (capital K to prevent accidents) |
| `Esc` | Return to menu |

### User Badges

| Badge | Style | Meaning |
|-------|-------|---------|
| `[ADMIN]` | Dim color | Admin user |
| `[MUTED]` | Error/red color | Muted user |

### Muting Users

Muting a user:
- Sets `Muted` flag to true
- Deletes all their posts
- Prevents new post creation

**Restrictions:**
- Cannot mute admin users
- Cannot mute yourself
- Cannot mute already-muted users

### Kicking Users

Kicking a user (capital `K` for safety):
- Completely deletes their account
- Removes all notes, follows, activities, notifications

**Restrictions:**
- Cannot kick admin users
- Cannot kick yourself

---

## Info Boxes Management View

### List Layout

```
admin panel > info boxes (3 boxes)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

› [1] ssh-first fediverse blog [ENABLED]
  [2] features [ENABLED]
  [3] github [DISABLED]

Info box saved successfully
```

### List Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `n` | Create new info box |
| `Enter` | Edit selected info box |
| `d` | Delete selected info box |
| `t` | Toggle enabled/disabled |
| `Esc` | Return to menu |

### Edit Mode Layout

```
admin panel > info boxes > editing
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Title:
┌──────────────────────────────────────────────────────────┐
│ ssh-first fediverse blog                                 │
└──────────────────────────────────────────────────────────┘

Content (Markdown):
┌──────────────────────────────────────────────────────────┐
│ Connect via SSH to start posting:                        │
│                                                          │
│ ```                                                      │
│ ssh -p {{SSH_PORT}} YourDomain                          │
│ ```                                                      │
└──────────────────────────────────────────────────────────┘

Order:
┌──────────────────────────────────────────────────────────┐
│ 1                                                        │
└──────────────────────────────────────────────────────────┘

tab/shift+tab: switch • ctrl+s: save • esc: cancel
```

### Edit Mode Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Tab` | Move to next field |
| `Shift+Tab` | Move to previous field |
| `Ctrl+S` | Save changes |
| `Esc` | Cancel editing (discard changes) |

### Edit Fields

| Field | Input Type | Description |
|-------|------------|-------------|
| Title | Single-line textarea | Box title (can include HTML/SVG) |
| Content | Multi-line textarea | Markdown content |
| Order | Single-line textarea | Display order number |

### Textarea Configuration

```go
func createTextarea(placeholder string, maxHeight int) textarea.Model {
    t := textarea.New()
    t.Placeholder = placeholder
    t.CharLimit = 0              // No limit
    t.ShowLineNumbers = false
    t.SetWidth(50)
    t.SetHeight(maxHeight)
    t.Cursor.SetMode(cursor.CursorBlink)
    return t
}
```

| Field | Height | Purpose |
|-------|--------|---------|
| Title | 1 | Single line |
| Content | 8 | Multi-line markdown |
| Order | 1 | Single line number |

---

## Message Types

```go
// User management
type usersLoadedMsg struct {
    users []domain.Account
}
type muteUserMsg struct{}
type kickUserMsg struct{}

// Info box management
type infoBoxesLoadedMsg struct {
    boxes []domain.InfoBox
}
type infoBoxSavedMsg struct{}
type infoBoxDeletedMsg struct{}
type infoBoxToggledMsg struct{}
```

---

## Commands

### User Commands

```go
func loadUsers() tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, users := database.ReadAllAccountsAdmin()
        // ...
        return usersLoadedMsg{users: *users}
    }
}

func muteUser(userId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        database.MuteUser(userId)
        return muteUserMsg{}
    }
}

func kickUser(userId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        database.DeleteAccount(userId)
        return kickUserMsg{}
    }
}
```

### Info Box Commands

```go
func loadInfoBoxes() tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, boxes := database.ReadAllInfoBoxes()
        // ...
        return infoBoxesLoadedMsg{boxes: *boxes}
    }
}

func saveInfoBox(box *domain.InfoBox) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        if box.Id == uuid.Nil {
            box.Id = uuid.New()
            box.CreatedAt = time.Now()
            database.CreateInfoBox(box)
        } else {
            database.UpdateInfoBox(box)
        }
        return infoBoxSavedMsg{}
    }
}

func deleteInfoBox(id uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        database.DeleteInfoBox(id)
        return infoBoxDeletedMsg{}
    }
}

func toggleInfoBox(id uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        database.ToggleInfoBoxEnabled(id)
        return infoBoxToggledMsg{}
    }
}
```

---

## Navigation Flow

```
┌────────────┐
│  MenuView  │ ◄─── Esc ───┐
└─────┬──────┘             │
      │ Enter              │
      ▼                    │
┌────────────┐             │
│ UsersView  │ ────────────┤
└────────────┘             │
      or                   │
┌────────────┐             │
│InfoBoxesView│────────────┤
└─────┬──────┘             │
      │ Enter/n            │
      ▼                    │
┌────────────┐             │
│ Edit Mode  │ ─── Esc ────┘
└────────────┘
      │ Ctrl+S
      ▼
   Save & Return
```

---

## Tab Navigation Blocking

When in admin submenus (UsersView or InfoBoxesView), tab navigation between main TUI panels is blocked:

```go
// In supertui.go
if m.state == common.AdminPanelView && m.adminModel.CurrentView != 0 {
    // Tab/Shift+Tab blocked in submenus
    return m, nil
}
```

This prevents accidental navigation away from admin tasks.

---

## Context-Aware Help Text

The footer shows context-specific help:

| View | Help Text |
|------|-----------|
| MenuView | `↑/↓ • enter: select` |
| UsersView | `↑/↓ • m: mute • K: kick • esc: back` |
| InfoBoxesView (list) | `↑/↓ • n: add • enter: edit • d: delete • t: toggle • esc: back` |
| InfoBoxesView (edit) | `tab/shift+tab: switch • ctrl+s: save • esc: cancel` |

---

## Initialization

```go
func InitialModel(adminId uuid.UUID, width, height int) Model {
    return Model{
        AdminId:      adminId,
        CurrentView:  MenuView,
        MenuSelected: 0,
        Users:        []domain.Account{},
        InfoBoxes:    []domain.InfoBox{},
        Selected:     0,
        BoxSelected:  0,
        Width:        width,
        Height:       height,
        Editing:      false,
    }
}

func (m Model) Init() tea.Cmd {
    return tea.Batch(loadUsers(), loadInfoBoxes())
}
```

---

## Access Control

This view is only accessible to admin users. Access is controlled in supertui.go based on the `Account.IsAdmin` field.

---

## Source Files

- `ui/admin/admin.go` - Admin panel view implementation
- `ui/common/styles.go` - List styles (ListBadgeMutedStyle, ListBadgeEnabledStyle)
- `db/db.go` - Database operations (ReadAllAccountsAdmin, MuteUser, DeleteAccount, InfoBox CRUD)
- `domain/account.go` - Account entity with IsAdmin, Muted fields
- `domain/infobox.go` - InfoBox entity
