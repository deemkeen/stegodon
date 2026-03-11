# TUI Initialization

This document specifies the BubbleTea program setup for each SSH session.

---

## Overview

Each SSH session gets its own BubbleTea program instance. The `bubbletea.Middleware` from wish v2 handles:
- PTY I/O setup (input/output routing)
- Window resize forwarding
- Program lifecycle management (alt screen, cursor, cleanup)
- Color profile configuration

The `MainTui` middleware handles CLI command interception before the TUI is started.

---

## Middleware Implementation

### Middleware Stack (app/app.go)

```go
wish.WithMiddleware(
    bubbletea.Middleware(middleware.TeaHandler),   // TUI (innermost)
    middleware.MainTui(),                          // CLI interception
    middleware.AuthMiddleware(a.config),            // Auth
    logging.MiddlewareWithLogger(log.Default()),   // Logging (outermost)
)
```

Execution order: Logging → Auth → MainTui → bubbletea.Middleware

### MainTui Function

```go
func MainTui() wish.Middleware {
    return func(next ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            if cmd := s.Command(); len(cmd) > 0 {
                handleCLI(s, cmd)
                return
            }
            next(s)
        }
    }
}
```

CLI commands are intercepted here. Interactive sessions pass through to `bubbletea.Middleware`.

### TeaHandler Function

```go
func TeaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
    pty, _, _ := s.Pty()

    err, acc := db.GetDB().ReadAccBySession(s)
    if err != nil {
        log.Println("Could not retrieve the user:", err)
        return nil, nil
    }

    m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)
    return m, []tea.ProgramOption{
        tea.WithFPS(30),
        tea.WithColorProfile(colorprofile.TrueColor),
    }
}
```

The `bubbletea.Middleware` from wish v2 calls this handler for each session and takes care of:
- Connecting SSH session I/O to the tea.Program
- Forwarding window resize events as `tea.WindowSizeMsg`
- Alt screen and cursor management
- Proper cleanup on session end

---

## Terminal Detection

### PTY Check

PTY detection is handled by `bubbletea.Middleware` internally via `bubbletea.MakeOptions(s)`, which selects the appropriate I/O based on whether a real PTY or emulated PTY is available.

### Window Dimensions

```go
pty.Window.Width   // Terminal width in columns
pty.Window.Height  // Terminal height in rows
```

Dimensions are passed to the model for layout calculations. Resize events are automatically forwarded by `bubbletea.Middleware`.

---

## Color Profile

### TrueColor Mode

```go
tea.WithColorProfile(colorprofile.TrueColor)
```

TrueColor (24-bit color) is set per-program via `WithColorProfile`:
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

The `TeaHandler` returns these options:

```go
[]tea.ProgramOption{
    tea.WithFPS(30),
    tea.WithColorProfile(colorprofile.TrueColor),
}
```

| Option | Description |
|--------|-------------|
| `WithFPS(30)` | 30 frames per second rendering |
| `WithColorProfile(...)` | Set TrueColor rendering |

The `bubbletea.Middleware` automatically adds I/O, environment, and window size options via `MakeOptions(s)`.

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
     ┌────────────────────────┐
     │ bubbletea.Middleware   │
     │ (creates tea.Program)  │
     └────────────────────────┘
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
    return nil, nil
}
```

If account lookup fails, `TeaHandler` returns nil and no program is created.

### No Active Terminal

Handled by `bubbletea.Middleware` internally — sessions without an active PTY are rejected.

---

## Middleware Integration

### Wish v2 bubbletea.Middleware

The TUI is served using wish v2's `bubbletea.Middleware`:
- Calls `TeaHandler` to get the model and options for each session
- Creates the bubbletea program with proper I/O via `MakeOptions(s)`
- Handles window resize forwarding automatically
- Manages alt screen and terminal state cleanup

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

Terminal resize events are forwarded automatically by `bubbletea.Middleware` as `tea.WindowSizeMsg` and propagate to all views:

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

30 frames per second provides a balance between responsive input and bandwidth efficiency over SSH. Actual updates occur only when state changes.

---

## Source Files

- `middleware/maintui.go` - MainTui middleware, TeaHandler
- `ui/supertui.go` - NewModel, MainModel
- `ui/common/layout.go` - DefaultWindowWidth, DefaultWindowHeight
- `app/app.go` - Middleware stack configuration
