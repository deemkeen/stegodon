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
    return func(next ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            // 1. Check for active terminal
            pty, windowChanges, active := s.Pty()
            if !active {
                wish.Println(s, "no active terminal, skipping")
                next(s)
                return
            }

            // 2. Load user account
            err, acc := db.GetDB().ReadAccBySession(s)
            if err != nil {
                log.Println("Could not retrieve the user:", err)
                next(s)
                return
            }

            // 3. Create main model with ViewStore for repaint workaround
            m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)
            viewStore := &ui.ViewStore{}
            m.ViewContent = viewStore

            // 4. Create repaintWriter (bypasses cursed renderer artifacts over SSH)
            in, out := ptyIO(s)
            rw := &repaintWriter{w: out, store: viewStore}

            // 5. Create BubbleTea program
            p := tea.NewProgram(m,
                tea.WithFPS(30),
                tea.WithInput(in),
                tea.WithOutput(rw),
                tea.WithEnvironment(s.Environ()),
                tea.WithColorProfile(colorprofile.TrueColor),
                tea.WithWindowSize(pty.Window.Width, pty.Window.Height),
            )

            // 6. Handle window resizes
            go func() { /* forward windowChanges to p.Send(tea.WindowSizeMsg{...}) */ }()

            p.Run()
            p.Kill()
            next(s)
        }
    }
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
tea.WithColorProfile(colorprofile.TrueColor)
```

TrueColor (24-bit color) is set per-program via `WithColorProfile` (the global `lipgloss.SetColorProfile` was removed in v2):
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
    tea.WithFPS(30),
    tea.WithInput(in),
    tea.WithOutput(rw),                          // repaintWriter
    tea.WithEnvironment(s.Environ()),
    tea.WithColorProfile(colorprofile.TrueColor),
    tea.WithWindowSize(pty.Window.Width, pty.Window.Height),
)
```

| Option | Description |
|--------|-------------|
| `WithFPS(30)` | 30 frames per second rendering |
| `WithInput(in)` | Read from PTY input |
| `WithOutput(rw)` | Write to repaintWriter (full-repaint workaround) |
| `WithEnvironment(...)` | Pass SSH session environment |
| `WithColorProfile(...)` | Set TrueColor rendering |
| `WithWindowSize(...)` | Set initial terminal dimensions (required over SSH) |

### RepaintWriter

The `repaintWriter` wraps the SSH output and replaces the cursed renderer's differential output with a full-repaint on every frame. This works around bubbletea v2's cursed renderer producing rendering artifacts over SSH (see [wish#392](https://github.com/charmbracelet/wish/pull/392)).

- On first write: enters alt screen (`\033[?1049h`) and hides cursor (`\033[?25l`)
- On each frame: discards cursed renderer output, overwrites in place with per-line erase, buffered into a single write
- Uses synchronized output (BSU/ESU `\033[?2026h`/`\033[?2026l`) to prevent tearing
- On shutdown: restores cursor and leaves alt screen

### ViewStore

`ViewStore` is a mutex-protected string store in `ui/supertui.go`. `MainModel.View()` stores its rendered content via `storeView()` before returning, so `repaintWriter` can access the latest full view independently of the cursed renderer.

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

### Wish Middleware

The TUI middleware is implemented as a standard `wish.Middleware` closure (the `wish/bubbletea` middleware package is not used with v2):
- Creates the bubbletea program directly with `tea.NewProgram`
- Uses `repaintWriter` for output to work around cursed renderer artifacts
- Handles window resize forwarding via goroutine
- Restores terminal state on shutdown

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

### 30 FPS Configuration

```go
tea.WithFPS(30)
```

30 frames per second provides a balance between responsive input and bandwidth efficiency over SSH. The `repaintWriter` sends a full screen repaint on every frame, so a lower FPS reduces bandwidth overhead. Actual updates occur only when state changes.

---

## Source Files

- `middleware/maintui.go` - MainTui middleware
- `ui/supertui.go` - NewModel, MainModel
- `ui/common/layout.go` - DefaultWindowWidth, DefaultWindowHeight
- `app/app.go` - Middleware stack configuration
