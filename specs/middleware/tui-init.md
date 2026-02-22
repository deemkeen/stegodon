# TUI Initialization

This document specifies the BubbleTea program setup for each SSH session.

---

## Overview

Each SSH session gets its own BubbleTea program instance. The MainTui middleware handles:
- Terminal capability detection
- Color profile configuration
- Main model initialization
- Program lifecycle management

---

## Middleware Implementation

### MainTui Function

```go
func MainTui() wish.Middleware {
    teaHandler := func(s ssh.Session) *tea.Program {
        // 1. Check for active terminal
        pty, _, active := s.Pty()
        if !active {
            wish.Println(s, "no active terminal, skipping")
            return nil
        }

        // 2. Load user account
        err, acc := db.GetDB().ReadAccBySession(s)
        if err != nil {
            log.Println("Could not retrieve the user:", err)
            return nil
        }

        // 3. Configure color profile
        lipgloss.SetColorProfile(termenv.TrueColor)

        // 4. Create main model
        m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)

        // 5. Create BubbleTea program
        return tea.NewProgram(m,
            tea.WithFPS(60),
            tea.WithInput(s),
            tea.WithOutput(s),
            tea.WithAltScreen(),
        )
    }
    return bm.MiddlewareWithProgramHandler(teaHandler, termenv.TrueColor)
}
```

---

## Terminal Detection

### PTY Check

```go
pty, _, active := s.Pty()
if !active {
    wish.Println(s, "no active terminal, skipping")
    return nil
}
```

Sessions without an active PTY (e.g., SSH commands without `-t`) are rejected.

### Window Dimensions

```go
pty.Window.Width   // Terminal width in columns
pty.Window.Height  // Terminal height in rows
```

Dimensions are passed to the model for layout calculations.

---

## Color Profile

### TrueColor Mode

```go
lipgloss.SetColorProfile(termenv.TrueColor)
```

TrueColor (24-bit color) is used for accurate hex color rendering:
- Renders colors independent of terminal palette
- Avoids ANSI 256-color palette inconsistencies across terminals
- Supported by all modern terminal emulators (Ghostty, iTerm2, Alacritty, etc.)

### Color Profile Options

| Profile | Colors | Support |
|---------|--------|---------|
| `Ascii` | None | Universal |
| `ANSI` | 16 | Basic terminals |
| `ANSI256` | 256 | Most terminals |
| `TrueColor` | 16M | Modern terminals |

---

## BubbleTea Program Options

### Program Configuration

```go
tea.NewProgram(m,
    tea.WithFPS(60),        // Frame rate
    tea.WithInput(s),       // SSH session input
    tea.WithOutput(s),      // SSH session output
    tea.WithAltScreen(),    // Use alternate screen buffer
)
```

| Option | Description |
|--------|-------------|
| `WithFPS(60)` | 60 frames per second rendering |
| `WithInput(s)` | Read from SSH session |
| `WithOutput(s)` | Write to SSH session |
| `WithAltScreen()` | Use alternate screen buffer |

### Alternate Screen Buffer

Alt screen provides:
- Clean screen on program start
- Original screen restored on exit
- No scroll history pollution
- Standard TUI behavior

---

## Model Initialization

### NewModel Function

```go
func NewModel(acc domain.Account, width int, height int) MainModel {
    // Apply dimension constraints
    width = common.DefaultWindowWidth(width)
    height = common.DefaultWindowHeight(height)

    // Load configuration
    config, _ := util.ReadConf()

    // Cache local domain
    localDomain := ""
    if config != nil {
        localDomain = config.Conf.SslDomain
    }

    // Initialize sub-models
    noteModel := writenote.InitialNote(width, acc.Id)
    headerModel := header.Model{Width: width, Acc: &acc}
    myPostsModel := myposts.NewPager(acc.Id, width, height, localDomain)
    followModel := followuser.InitialModel(acc.Id)
    // ... more sub-models

    return MainModel{
        Acc:              &acc,
        WriteNote:        noteModel,
        Header:           headerModel,
        MyPosts:          myPostsModel,
        // ... more fields
    }
}
```

### Dimension Constraints

```go
func DefaultWindowWidth(width int) int {
    if width < 115 {
        return 115
    }
    return width
}

func DefaultWindowHeight(height int) int {
    if height < 28 {
        return 28
    }
    return height
}
```

| Dimension | Minimum |
|-----------|---------|
| Width | 115 columns |
| Height | 28 rows |

---

## Session Data Flow

```
SSH Session
     │
     ├── Account (from AuthMiddleware)
     │
     ├── Terminal Dimensions (from PTY)
     │
     └── Configuration (from config file)
           │
           ▼
     ┌────────────┐
     │  NewModel  │
     └────────────┘
           │
           ▼
     ┌────────────┐
     │ tea.Program│
     └────────────┘
           │
           ▼
     ┌────────────┐
     │   TUI UI   │
     └────────────┘
```

---

## Sub-Model Initialization

Each view component is initialized with appropriate data:

| Component | Initialization |
|-----------|----------------|
| `writenote` | User ID, width |
| `header` | Width, account |
| `myposts` | User ID, dimensions, domain |
| `followuser` | User ID |
| `hometimeline` | User ID, dimensions, config |
| `threadview` | User ID, dimensions, domain |
| `followers` | User ID, dimensions |
| `following` | User ID, dimensions |
| `localusers` | User ID, dimensions |
| `notifications` | User ID, dimensions |
| `relay` | Config, dimensions |
| `admin` | User ID |
| `deleteaccount` | User ID, username |

---

## Error Handling

### Account Not Found

```go
err, acc := db.GetDB().ReadAccBySession(s)
if err != nil {
    log.Println("Could not retrieve the user:", err)
    return nil
}
```

If account lookup fails, no program is created and the session ends.

### No Active Terminal

```go
if !active {
    wish.Println(s, "no active terminal, skipping")
    return nil
}
```

Non-interactive sessions are rejected with a message.

---

## Middleware Integration

### Wish BubbleTea Middleware

```go
import bm "github.com/charmbracelet/wish/bubbletea"

return bm.MiddlewareWithProgramHandler(teaHandler, termenv.TrueColor)
```

The `wish/bubbletea` middleware:
- Manages program lifecycle
- Handles input/output routing
- Cleans up on session end

---

## View Selection

Initial view depends on account state:

```go
func (m MainModel) Init() tea.Cmd {
    if m.Acc.FirstTimeLogin {
        m.currentView = ViewCreateUser
        return m.CreateUser.Init()
    }
    m.currentView = ViewHomeTimeline
    return m.HomeTimeline.Init()
}
```

| Account State | Initial View |
|---------------|--------------|
| First time login | CreateUser (username selection) |
| Returning user | HomeTimeline |

---

## Window Resize Handling

Terminal resize events propagate to all views:

```go
case tea.WindowSizeMsg:
    m.Width = msg.Width
    m.Height = msg.Height
    // Propagate to sub-models
    m.Header.Width = msg.Width
    m.MyPosts.UpdateSize(msg.Width, msg.Height)
    // ... etc
```

---

## Frame Rate

### 60 FPS Configuration

```go
tea.WithFPS(60)
```

60 frames per second provides:
- Smooth scrolling
- Responsive input
- Good animation support

For SSH, this means up to 60 screen updates per second, though actual updates occur only when state changes.

---

## Source Files

- `middleware/maintui.go` - MainTui middleware
- `ui/supertui.go` - NewModel, MainModel
- `ui/common/layout.go` - DefaultWindowWidth, DefaultWindowHeight
- `app/app.go` - Middleware stack configuration
