# Terminal Requirements

This document specifies terminal dimension requirements and color support.

---

## Overview

Stegodon TUI requires:
- **Minimum dimensions**: 115×28 characters
- **Color support**: TrueColor (24-bit) mode
- **Alt-screen**: Required for TUI rendering

---

## Minimum Dimensions

### Requirements

| Dimension | Minimum | Purpose |
|-----------|---------|---------|
| Width | 115 columns | Layout with sidebar |
| Height | 28 rows | Content display |

### Enforcement

```go
func (m MainModel) View() tea.View {
    // Check minimum terminal size
    minWidth := 115
    minHeight := 28

    if m.width < minWidth || m.height < minHeight {
        message := fmt.Sprintf(
            "Terminal too small!\n\nMinimum required: %dx%d\nCurrent size: %dx%d\n\nPlease resize your terminal.",
            minWidth, minHeight, m.width, m.height,
        )
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color(COLOR_CRITICAL)).
            Render(message)
    }

    // Continue with normal rendering...
}
```

### Error Display

When terminal is too small:

```
Terminal too small!

Minimum required: 115x28
Current size: 80x24

Please resize your terminal.
```

Displayed in critical red color (`#ff5555`).

---

## Window Size Handling

### Initial Size

Terminal dimensions are captured on SSH session start:

```go
func MainTui() wish.Middleware {
    return func(next ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            pty, windowChanges, _ := s.Pty()
            m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)
            // ... create repaintWriter, run program with tea.WithWindowSize() ...
        }
    }
}
```

### Resize Events

Terminal resize events propagate to all views:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    // Update child views...
```

### Layout Constants

```go
const (
    DefaultItemHeight       = 3   // Height of a list item in lines
    MinItemsPerPage         = 3   // Minimum items to show per page
    DefaultItemsPerPage     = 10  // Default when calculation unavailable
    MaxContentTruncateWidth = 150 // Max width for post content
    ReplyIndentWidth        = 4   // Spaces to indent replies
)
```

---

## Color Support

### TrueColor Mode

Stegodon uses TrueColor (24-bit color) for accurate, palette-independent rendering:

```go
// Color profile is set per-program in v2 (global lipgloss.SetColorProfile was removed)
tea.WithColorProfile(colorprofile.TrueColor)
```

### Why TrueColor

| Mode | Colors | Compatibility |
|------|--------|---------------|
| TrueColor (24-bit) | 16M | Modern terminals |
| ANSI256 (8-bit) | 256 | Wide but palette-dependent |
| ANSI (4-bit) | 16 | Universal |

TrueColor was chosen for:
- Hex colors render identically across all terminals
- No dependency on terminal's 256-color palette mapping
- Supported by all modern terminals (Ghostty, iTerm2, Alacritty, etc.)

---

## Color Palette

### Primary UI Colors

| Constant | ANSI | Hex | Purpose |
|----------|------|-----|---------|
| `COLOR_ACCENT` | 69 | `#5f87ff` | Borders, selections, header |
| `COLOR_SECONDARY` | 75 | `#5fafff` | Timestamps, domains, hashtags |

### Text Colors

| Constant | ANSI | Hex | Purpose |
|----------|------|-----|---------|
| `COLOR_WHITE` | 255 | `#eeeeee` | Primary text |
| `COLOR_LIGHT` | 250 | `#bcbcbc` | Secondary text |
| `COLOR_MUTED` | 245 | `#8a8a8a` | Hints, disabled |
| `COLOR_DIM` | 240 | `#585858` | Borders, separators |

### Semantic Colors

| Constant | ANSI | Hex | Purpose |
|----------|------|-----|---------|
| `COLOR_USERNAME` | 48 | `#00ff87` | Usernames, success |
| `COLOR_ERROR` | 196 | `#ff0000` | Errors, delete actions |
| `COLOR_CRITICAL` | 9 | `#ff5555` | Critical errors |
| `COLOR_WARNING` | 214 | `#ffaf00` | Content warnings |

### Interactive Elements

| Constant | ANSI | Hex | Purpose |
|----------|------|-----|---------|
| `COLOR_HASHTAG` | 75 | `#5fafff` | Hashtag links |
| `COLOR_MENTION` | 48 | `#00ff87` | @mentions |
| `COLOR_LINK` | 48 | `#00ff87` | Hyperlinks |
| `COLOR_BUTTON` | 117 | `#87d7ff` | Active elements |

---

## OSC 8 Hyperlinks

### Format

```
\033]8;;URL\033\\TEXT\033]8;;\033\\
```

| Sequence | Purpose |
|----------|---------|
| `\033]8;;URL\033\\` | Start hyperlink |
| `TEXT` | Visible text |
| `\033]8;;\033\\` | End hyperlink |

### RGB Color with Hyperlink

```go
fmt.Sprintf("\033[38;2;0;255;135;4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m",
    url, linkText)
```

| Code | Meaning |
|------|---------|
| `\033[38;2;0;255;135` | RGB foreground (0, 255, 135) |
| `;4m` | Underline |
| `\033[39;24m` | Reset foreground, remove underline |

### Terminal Support

| Terminal | OSC 8 Support |
|----------|---------------|
| Ghostty | Yes |
| iTerm2 | Yes |
| Kitty | Yes |
| VS Code Terminal | Yes |
| macOS Terminal | No |
| Windows Terminal | Yes |

---

## Layout Calculations

### Window Width

```go
func DefaultWindowWidth(width int) int {
    return width - 10  // Accounts for margins, scrollbar, buffer
}
```

### Window Height

```go
func DefaultWindowHeight(height int) int {
    return height - 10  // Accounts for margins, terminal chrome
}
```

### List/Note Split

```go
func DefaultCreateNoteWidth(width int) int {
    return width / 4  // Note sidebar takes 1/4
}

func DefaultListWidth(width int) int {
    return width - DefaultCreateNoteWidth(width)  // List takes 3/4
}
```

---

## Docker Terminal Configuration

### Environment Variable

```dockerfile
ENV TERM=xterm-256color
```

### Required Packages

```dockerfile
RUN apk add --no-cache ncurses-terminfo-base
```

Provides terminal database for 256-color mode.

---

## Terminal Recommendations

### Recommended Terminals

| Terminal | Platform | OSC 8 | True Color |
|----------|----------|-------|------------|
| Ghostty | macOS/Linux | Yes | Yes |
| iTerm2 | macOS | Yes | Yes |
| Kitty | macOS/Linux | Yes | Yes |
| Alacritty | All | Yes | Yes |
| Windows Terminal | Windows | Yes | Yes |

### Minimum Requirements

| Feature | Required |
|---------|----------|
| 256 colors | Yes |
| Alt-screen | Yes |
| Unicode | Yes |
| 115×28 minimum | Yes |
| OSC 8 | Optional (for clickable links) |

---

## Troubleshooting

### Terminal Too Small

```
Terminal too small!
Minimum required: 115x28
```

**Solution**: Resize terminal window or increase font size/decrease zoom.

### Colors Not Displaying

1. Check `TERM` environment variable:
   ```bash
   echo $TERM
   # Should include "256color"
   ```

2. Verify terminal supports 256 colors:
   ```bash
   tput colors
   # Should output: 256
   ```

### Links Not Clickable

- Terminal may not support OSC 8
- Try Ghostty, iTerm2, or Kitty

---

## Source Files

- `ui/supertui.go` - Minimum size check, window resize handling
- `ui/common/styles.go` - Color palette definitions
- `ui/common/layout.go` - Layout constants
- `middleware/maintui.go` - TrueColor mode setup
- `Dockerfile` - `TERM=xterm-256color` configuration
