# CreateUser View

This document specifies the CreateUser view, which handles first-time user registration and profile setup.

---

## Overview

The CreateUser view is displayed when a user connects via SSH without an existing account. It guides users through a 3-step process to create their profile: username selection, display name, and bio.

---

## Data Structure

```go
type Model struct {
    TextInput   textinput.Model  // Username input (step 0)
    DisplayName textinput.Model  // Display name input (step 1)
    Bio         textinput.Model  // Bio input (step 2)
    Step        int              // Current step: 0, 1, or 2
    Error       string           // Validation error message
    Err         util.ErrMsg      // System error
}
```

---

## Registration Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    CreateUser Flow                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   Step 0: Username                                           │
│   ┌───────────────────────────────────────────────────────┐  │
│   │ You don't have a username yet, please choose wisely! │  │
│   │                                                       │  │
│   │ [ ElonMusk666                                    ]    │  │
│   │                                                       │  │
│   │ (enter to continue, ctrl-c to quit)                   │  │
│   └───────────────────────────────────────────────────────┘  │
│         │                                                    │
│         │ Enter (validates & checks uniqueness)              │
│         ▼                                                    │
│   Step 1: Display Name                                       │
│   ┌───────────────────────────────────────────────────────┐  │
│   │ Username: alice                                       │  │
│   │                                                       │  │
│   │ Choose your display name (optional):                  │  │
│   │ [ John Doe                                       ]    │  │
│   │                                                       │  │
│   │ (enter to continue, leave empty to skip)              │  │
│   └───────────────────────────────────────────────────────┘  │
│         │                                                    │
│         │ Enter                                              │
│         ▼                                                    │
│   Step 2: Bio                                                │
│   ┌───────────────────────────────────────────────────────┐  │
│   │ Username: alice                                       │  │
│   │ Display name: Alice Smith                             │  │
│   │                                                       │  │
│   │ Write a short bio (optional):                         │  │
│   │ [ CEO of X, Tesla, SpaceX...                     ]    │  │
│   │                                                       │  │
│   │ (enter to save profile, ctrl-c to quit)               │  │
│   └───────────────────────────────────────────────────────┘  │
│         │                                                    │
│         │ Enter (creates account)                            │
│         ▼                                                    │
│   Account Created → Redirect to HomeTimeline                 │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Field Constraints

### Username (Step 0)

| Property | Value |
|----------|-------|
| Character Limit | 15 |
| Input Width | 20 |
| Required | Yes |
| Placeholder | `ElonMusk666` |

**Validation:**
- Must pass WebFinger format validation
- Must be unique (not already taken)
- Alphanumeric and underscores only

### Display Name (Step 1)

| Property | Value |
|----------|-------|
| Character Limit | 50 |
| Input Width | 50 |
| Required | No |
| Placeholder | `John Doe` |

### Bio (Step 2)

| Property | Value |
|----------|-------|
| Character Limit | 200 |
| Input Width | 60 |
| Required | No |
| Placeholder | `CEO of X, Tesla, SpaceX...` |

---

## Validation

### WebFinger Username Validation

```go
func IsValidWebFingerUsername(username string) (bool, string) {
    // Check length (1-15 characters)
    // Check allowed characters (alphanumeric, underscore)
    // Check doesn't start with number
    // Check reserved words
}
```

### Uniqueness Check

```go
err, existingAcc := db.GetDB().ReadAccByUsername(username)
if err == nil && existingAcc != nil {
    m.Error = fmt.Sprintf("Username '%s' is already taken", username)
    return m, nil
}
```

---

## Update Logic

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        // Clear error when user types
        if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace {
            m.Error = ""
        }

        switch msg.String() {
        case "enter":
            if m.Step == 0 {
                // Validate username
                valid, errMsg := util.IsValidWebFingerUsername(username)
                if !valid {
                    m.Error = errMsg
                    return m, nil
                }
                // Check uniqueness
                // Move to step 1
            } else if m.Step == 1 {
                // Move to step 2
            }
            // Step 2 submission handled by parent
        }
    }

    // Update active input based on step
    switch m.Step {
    case 0:
        m.TextInput, cmd = m.TextInput.Update(msg)
    case 1:
        m.DisplayName, cmd = m.DisplayName.Update(msg)
    case 2:
        m.Bio, cmd = m.Bio.Update(msg)
    }

    return m, cmd
}
```

---

## View Rendering

### Layout

```go
var Style = lipgloss.NewStyle().
    Height(25).Width(80).
    Align(lipgloss.Center, lipgloss.Center).
    BorderStyle(lipgloss.ThickBorder()).
    Margin(0, 3)
```

The view is centered both horizontally and vertically in the terminal.

### View Output

```go
func (m Model) View() tea.View {
    // Build prompt based on step
    // Show accumulated values (username, display name)
    // Show current input field
    // Show help text
    // Show error if present (in red)

    return fmt.Sprintf(
        "Logging into STEGODON v%s\n\n%s\n\n%s\n\n%s",
        util.GetVersion(),
        prompt,
        input,
        help,
    )
}
```

### Error Display

Errors are shown in bold red below the input:

```go
if m.Error != "" {
    errorStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_ERROR)).
        Bold(true)
    baseView += "\n\n" + errorStyle.Render(m.Error)
}
```

---

## Responsive Width

The view adapts to terminal width:

```go
func (m Model) ViewWithWidth(termWidth, termHeight int) string {
    // Account for border (2 chars) and margins (6 chars)
    contentWidth := max(
        termWidth - common.CreateUserDialogBorderAndMargin,
        common.CreateUserMinWidth,
    )

    bordered := Style.Width(contentWidth).Render(m.View())
    return lipgloss.Place(termWidth, termHeight, lipgloss.Center, lipgloss.Center, bordered)
}
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Proceed to next step / Submit |
| `Ctrl+C` | Quit application |
| Typing | Clear error message |

---

## Account Creation

After step 2, the parent (`supertui.go`) creates the account:

```go
func createAccount(username, displayName, bio, sshPubKey string) tea.Cmd {
    return func() tea.Msg {
        // Generate RSA keypair for ActivityPub
        privateKey, publicKey := util.GenerateRSAKeypair()

        account := &domain.Account{
            Id:          uuid.New(),
            Username:    username,
            DisplayName: displayName,
            Summary:     bio,
            SshPubKey:   hashKey(sshPubKey),
            PrivateKey:  privateKey,
            PublicKey:   publicKey,
            CreatedAt:   time.Now(),
        }

        db.GetDB().CreateAccount(account)
        return accountCreatedMsg{account: account}
    }
}
```

---

## Error Messages

| Scenario | Error Message |
|----------|---------------|
| Empty username | "Username cannot be empty" |
| Invalid characters | "Username can only contain letters, numbers, and underscores" |
| Too long | "Username must be 15 characters or less" |
| Already taken | "Username 'xxx' is already taken" |
| Starts with number | "Username cannot start with a number" |

---

## Initialization

```go
func InitialModel() Model {
    ti := textinput.New()
    ti.Placeholder = "ElonMusk666"
    ti.Focus()
    ti.CharLimit = 15
    ti.Width = 20

    displayName := textinput.New()
    displayName.Placeholder = "John Doe"
    displayName.CharLimit = 50
    displayName.Width = 50

    bio := textinput.New()
    bio.Placeholder = "CEO of X, Tesla, SpaceX..."
    bio.CharLimit = 200
    bio.Width = 60

    return Model{
        TextInput:   ti,
        DisplayName: displayName,
        Bio:         bio,
        Step:        0,
    }
}
```

---

## Focus Management

Only the active input field has focus:

```go
// When moving from step 0 to step 1
m.Step = 1
m.DisplayName.Focus()
m.TextInput.Blur()

// When moving from step 1 to step 2
m.Step = 2
m.Bio.Focus()
m.DisplayName.Blur()
```

---

## Source Files

- `ui/createuser/createuser.go` - CreateUser view implementation
- `ui/supertui.go` - Parent view handling account creation
- `util/validation.go` - WebFinger username validation
- `db/db.go` - Account creation in database
