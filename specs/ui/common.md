# Common Components

This document specifies the shared components, styles, layout constants, and commands used across the TUI.

---

## Overview

The `ui/common` package provides:
- Color constants and style definitions
- Layout calculation helpers
- Session state management
- Shared message types for inter-view communication

The `ui/header` package provides the navigation header component.

---

## Color System

### Primary UI Colors

```go
const (
    COLOR_ACCENT    = "69"  // ANSI 69 (#5f87ff) - Primary accent: borders, selections, header
    COLOR_SECONDARY = "75"  // ANSI 75 (#5fafff) - Secondary accent: timestamps, domains, hashtags
)
```

### Text Colors

```go
const (
    COLOR_WHITE = "255"  // ANSI 255 (#eeeeee) - Primary text, post content
    COLOR_LIGHT = "250"  // ANSI 250 (#bcbcbc) - Secondary text, slightly dimmed
    COLOR_MUTED = "245"  // ANSI 245 (#8a8a8a) - Tertiary text, disabled, hints
    COLOR_DIM   = "240"  // ANSI 240 (#585858) - Very dim text, borders, separators
)
```

### Semantic Colors

```go
const (
    COLOR_USERNAME = "48"   // ANSI 48 (#00ff87) - Usernames stand out
    COLOR_SUCCESS  = "48"   // ANSI 48 (#00ff87) - Success messages
    COLOR_ERROR    = "196"  // ANSI 196 (#ff0000) - Errors, delete actions, warnings
    COLOR_CRITICAL = "9"    // ANSI 9 (#ff5555) - Critical errors, terminal size warnings
    COLOR_WARNING  = "214"  // ANSI 214 (#ffaf00) - Content warnings, caution
)
```

### Interactive Elements

```go
const (
    COLOR_HASHTAG = "75"   // ANSI 75 (#5fafff) - Hashtags
    COLOR_MENTION = "48"   // ANSI 48 (#00ff87) - Mentions
    COLOR_LINK    = "48"   // ANSI 48 (#00ff87) - Hyperlinks
    COLOR_BUTTON  = "117"  // ANSI 117 (#87d7ff) - Button highlights
    COLOR_CAPTION = "170"  // ANSI 170 (#d75fd7) - Section captions, titles
    COLOR_HELP    = "245"  // ANSI 245 (#8a8a8a) - Help text
)
```

### ANSI Escape Sequences

For inline coloring without breaking backgrounds:

```go
const (
    ANSI_WARNING_START = "\033[38;5;214m"  // Start warning color
    ANSI_COLOR_RESET   = "\033[39m"        // Reset foreground to default
)
```

### OSC 8 Hyperlink Colors

RGB format for true color terminals:

```go
const (
    COLOR_LINK_RGB    = "0;255;135"  // RGB for hyperlinks (#00ff87)
    COLOR_MENTION_RGB = "0;255;135"  // RGB for mentions (#00ff87)
)
```

---

## Shared Styles

### Base Styles

```go
var (
    HelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_GREY)).Padding(0, 2)
    CaptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_MAGENTA)).Padding(2)
)
```

### List Item Styles

```go
var (
    // Base style for unselected list items
    ListItemStyle = lipgloss.NewStyle()

    // Selected item text (highlighted color + bold)
    ListItemSelectedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(COLOR_USERNAME)).
        Bold(true)

    // Empty list messages
    ListEmptyStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(COLOR_DIM)).
        Italic(true)

    // Status messages (success, info)
    ListStatusStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(COLOR_SUCCESS))

    // Error messages
    ListErrorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(COLOR_ERROR))

    // Inline badges like [local], [pending], [ADMIN]
    ListBadgeStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(COLOR_DIM))

    // Muted user badge (red)
    ListBadgeMutedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(COLOR_ERROR))

    // Enabled/active status badge (green)
    ListBadgeEnabledStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(COLOR_SUCCESS))
)
```

### Selection Prefixes

```go
const (
    ListSelectedPrefix   = "› "   // Indicator before selected items
    ListUnselectedPrefix = "  "   // Spacing for unselected items (same width)
)
```

---

## Layout Constants

### Dimensions

```go
const (
    HeaderHeight    = 1   // Height of the header bar
    HeaderNewline   = 1   // Newline after header
    FooterHeight    = 1   // Height of help/footer text

    PanelMarginVertical = 2   // Vertical margin (1 top + 1 bottom)
    PanelMarginLeft     = 1   // Left margin
    BorderWidth         = 1   // Normal border width
)
```

### Two-Panel Layout

```go
const (
    TwoPanelBorderWidth = 4   // Total horizontal border space
    TwoPanelMarginWidth = 2   // Total horizontal margin space
)
```

### Content Limits

```go
const (
    DefaultItemHeight       = 3     // Estimated height of a list item
    MinItemsPerPage         = 3     // Minimum items to show
    DefaultItemsPerPage     = 10    // Default when dynamic calculation unavailable
    MaxContentTruncateWidth = 150   // Max width for truncating post content
    ReplyIndentWidth        = 4     // Spaces to indent replies in thread view
)
```

### Refresh and Limits

```go
const (
    TimelineRefreshSeconds  = 10    // Auto-refresh interval (seconds)
    HomeTimelinePostLimit   = 50    // Max posts in home timeline
    MaxNoteDBLength         = 1000  // Max note length in database
)
```

---

## Layout Helpers

### Vertical Layout

```go
// VerticalLayoutOffset returns total vertical space for header, footer, margins
func VerticalLayoutOffset() int {
    return HeaderHeight + HeaderNewline + PanelMarginVertical + FooterHeight
}

// CalculateAvailableHeight returns height for panel content
func CalculateAvailableHeight(totalHeight int) int {
    return totalHeight - VerticalLayoutOffset()
}
```

### Horizontal Layout

```go
// HorizontalPanelOffset returns horizontal space for borders and margins
func HorizontalPanelOffset() int {
    return TwoPanelBorderWidth + TwoPanelMarginWidth
}

// CalculateLeftPanelWidth returns width for left panel (1/3 of total)
func CalculateLeftPanelWidth(totalWidth int) int {
    return totalWidth / 3
}

// CalculateRightPanelWidth returns width for right panel
func CalculateRightPanelWidth(totalWidth, leftPanelWidth int) int {
    return totalWidth - leftPanelWidth - HorizontalPanelOffset()
}

// CalculateContentWidth returns width for content inside a panel
func CalculateContentWidth(panelWidth, padding int) int {
    return panelWidth - (padding * 2)
}
```

### Pagination

```go
// CalculateItemsPerPage returns items that fit in available height
func CalculateItemsPerPage(availableHeight, itemHeight int) int {
    if itemHeight <= 0 {
        itemHeight = DefaultItemHeight
    }
    items := availableHeight / itemHeight
    if items < MinItemsPerPage {
        return MinItemsPerPage
    }
    return items
}
```

### Window Size

```go
// DefaultWindowWidth returns usable width after outer margins
func DefaultWindowWidth(width int) int {
    return width - 10
}

// DefaultWindowHeight returns usable height after outer margins
func DefaultWindowHeight(height int) int {
    return height - 10
}
```

### Measurement

```go
// MeasureHeight returns height of a rendered string
func MeasureHeight(rendered string) int {
    return lipgloss.Height(rendered)
}

// MeasureWidth returns width of a rendered string
func MeasureWidth(rendered string) int {
    return lipgloss.Width(rendered)
}
```

---

## Session State

Session states represent the current active view:

```go
type SessionState uint

const (
    CreateNoteView      SessionState = iota
    HomeTimelineView                  // Unified home timeline
    MyPostsView                       // User's own posts
    CreateUserView
    UpdateNoteList                    // Signal to reload note lists
    FollowUserView                    // Follow remote users
    FollowersView                     // View followers
    FollowingView                     // View following
    LocalUsersView                    // Browse local users
    AdminPanelView                    // Admin panel (admin only)
    RelayManagementView               // Relay management (admin only)
    DeleteAccountView                 // Account deletion
    ThreadView                        // Thread/conversation view
    NotificationsView                 // Notifications
)
```

---

## Message Types

### View Lifecycle

```go
// ActivateViewMsg is sent when a view becomes active (visible)
type ActivateViewMsg struct{}

// DeactivateViewMsg is sent when a view becomes inactive (hidden)
type DeactivateViewMsg struct{}
```

### Note Editing

```go
// EditNoteMsg is sent when user wants to edit an existing note
type EditNoteMsg struct {
    NoteId    uuid.UUID
    Message   string
    CreatedAt time.Time
}

// DeleteNoteMsg is sent when user confirms note deletion
type DeleteNoteMsg struct {
    NoteId uuid.UUID
}
```

### Reply and Thread

```go
// ReplyToNoteMsg is sent when user presses 'r' to reply
type ReplyToNoteMsg struct {
    NoteURI string    // ActivityPub object URI
    Author  string    // Display name or handle
    Preview string    // Truncated preview of content
}

// ViewThreadMsg is sent when user presses Enter to view a thread
type ViewThreadMsg struct {
    NoteURI   string
    NoteID    uuid.UUID
    IsLocal   bool
    Author    string
    Content   string
    CreatedAt time.Time
}
```

### Engagement

```go
// LikeNoteMsg is sent when user presses 'l' to like/unlike
type LikeNoteMsg struct {
    NoteURI string
    NoteID  uuid.UUID
    IsLocal bool
}
```

---

## Header Component

The header displays user info, app version, and notification badge.

### Data Structure

```go
type Model struct {
    Width       int
    Acc         *domain.Account
    UnreadCount int
}
```

### Layout

```
┌─────────────────────────────────────────────────────────────────────────┐
│  🦣 alice [3]          stegodon v1.4.3              joined: 2024-01-15  │
└─────────────────────────────────────────────────────────────────────────┘
```

### Components

- **Left**: Elephant emoji + username + notification badge (if unread)
- **Center**: App name and version
- **Right**: Join date

### Notification Badge

```go
if unreadCount > 0 {
    badgePlain = fmt.Sprintf(" [%d]", unreadCount)
    leftTextPlain += badgePlain
}

// Use raw ANSI for warning color without breaking background
leftText += common.ANSI_WARNING_START + badgePlain + common.ANSI_COLOR_RESET
```

### Rendering

```go
func GetHeaderStyle(acc *domain.Account, width int, unreadCount int) string {
    elephant := "🦣"
    leftTextPlain := fmt.Sprintf("%s %s", elephant, acc.Username)
    centerText := fmt.Sprintf("stegodon v%s", util.GetVersion())
    rightText := fmt.Sprintf("joined: %s", acc.CreatedAt.Format("2006-01-02"))

    // Calculate spacing for even distribution
    totalTextLen := leftLen + centerLen + rightLen
    totalSpacing := max(width - totalTextLen - common.HeaderTotalPadding, 2)
    leftSpacing := totalSpacing / 2
    rightSpacing := totalSpacing - leftSpacing

    // Build header with spacing
    header := fmt.Sprintf("  %s%s%s%s%s  ",
        leftText,
        spaces(leftSpacing),
        centerText,
        spaces(rightSpacing),
        rightText,
    )

    // Apply full-width background
    return lipgloss.NewStyle().
        Width(width).
        MaxWidth(width).
        Background(lipgloss.Color(common.COLOR_ACCENT)).
        Foreground(lipgloss.Color(common.COLOR_WHITE)).
        Bold(true).
        Render(header)
}
```

---

## Selection Highlighting Pattern

Selected items typically use inverted colors:

```go
if i == m.Selected {
    selectedBg := lipgloss.NewStyle().
        Background(lipgloss.Color(common.COLOR_ACCENT)).
        Width(contentWidth)

    timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr))
    authorFormatted := selectedBg.Render(selectedAuthorStyle.Render(author))
    contentFormatted := selectedBg.Render(selectedContentStyle.Render(content))
}
```

---

## Common Time Formatting

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

---

## Auto-Refresh Pattern

Views with auto-refresh should track active state:

```go
type Model struct {
    isActive bool
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case common.ActivateViewMsg:
        m.isActive = true
        return m, tea.Batch(loadData(), tickRefresh())

    case common.DeactivateViewMsg:
        m.isActive = false
        return m, nil  // Stop ticker chain

    case refreshTickMsg:
        if m.isActive {
            return m, tea.Batch(loadData(), tickRefresh())
        }
        return m, nil  // Don't restart if inactive
    }
}
```

---

## Source Files

### common/

- `commands.go` - SessionState enum and message types
- `styles.go` - Color constants and lipgloss styles
- `layout.go` - Layout constants and calculation helpers

### header/

- `header.go` - Header component with notification badge
