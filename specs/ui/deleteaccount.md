# DeleteAccount View

This document specifies the DeleteAccount view, which provides a two-step confirmation flow for account deletion.

---

## Overview

The DeleteAccount view guides users through a two-step confirmation process before permanently deleting their account. It features:
- Initial warning with list of data to be deleted
- Final confirmation before irreversible deletion
- Post-deletion "Bye bye!" message before logout

---

## Data Structure

```go
type Model struct {
    Account        *domain.Account
    ConfirmStep    int     // 0 = initial, 1 = first confirmation, 2 = final confirmation
    Status         string
    Error          string
    DeletionStatus string
    ShowByeBye     bool
}
```

---

## Confirmation Steps

### Step 0: Initial Warning

```
┌─────────────────────────────────────────────────────────────┐
│ delete account                                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ⚠ WARNING: This will permanently delete your account!       │  ← Red, bold
│                                                              │
│ The following data will be deleted:                          │
│   • Your account (@alice)                                    │
│   • All your posts and notes                                 │
│   • All follow relationships                                 │
│   • All your activities                                      │
│                                                              │
│ This action CANNOT be undone!                                │  ← Red, bold
│                                                              │
│ Are you sure you want to delete your account?                │
│                                                              │
│ Press 'y' to continue or 'n'/'esc' to cancel                 │  ← Dim text
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Step 1: Final Confirmation

```
┌─────────────────────────────────────────────────────────────┐
│ delete account                                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ⚠ FINAL WARNING!                                             │  ← Red, bold
│                                                              │
│ You are about to permanently delete account: @alice          │
│                                                              │
│ This is your last chance to cancel.                          │
│ After this, your account and all data will be gone forever.  │
│                                                              │
│ Press 'y' to DELETE PERMANENTLY or 'n'/'esc' to cancel       │  ← Dim text
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Deletion Complete

```
┌─────────────────────────────────────────────────────────────┐
│ delete account                                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ✓ Account deleted successfully                               │
│                                                              │
│ Logging out...                                               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Bye Bye Message

```
┌─────────────────────────────────────────────────────────────┐
│ delete account                                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│                                                              │
│                       Bye bye!                               │  ← Green, bold, centered
│                                                              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `y` / `Y` | Confirm current step |
| `n` / `N` | Cancel and reset |
| `Esc` | Cancel and reset |

---

## Confirmation Flow

```
User enters DeleteAccount view
      │
      ▼
Step 0: Initial Warning
      │
      ├── 'y' → Advance to Step 1
      └── 'n' / 'esc' → Cancel (reset to Step 0, show "Deletion cancelled")
            │
            ▼
Step 1: Final Confirmation
      │
      ├── 'y' → Delete Account
      │     │
      │     ├── Show "Deleting account..."
      │     ├── Delete from database
      │     ├── Show "Account deleted successfully"
      │     ├── Wait 2 seconds
      │     ├── Show "Bye bye!"
      │     ├── Wait 2 seconds
      │     └── Quit application
      │
      └── 'n' / 'esc' → Cancel (reset to Step 0, show "Deletion cancelled")
```

---

## Update Logic

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case clearStatusMsg:
        m.Status = ""
        m.Error = ""
        return m, nil

    case showByeByeMsg:
        m.ShowByeBye = true
        // Wait 2 more seconds then quit
        return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
            return tea.Quit()
        })

    case deleteAccountResultMsg:
        if msg.err != nil {
            m.Error = fmt.Sprintf("Failed to delete account: %v", msg.err)
            m.ConfirmStep = 0
        } else {
            m.DeletionStatus = "completed"
            // Wait 2 seconds then show "Bye bye!"
            return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
                return showByeByeMsg{}
            })
        }
        return m, nil

    case tea.KeyPressMsg:
        switch msg.String() {
        case "y", "Y":
            if m.ConfirmStep == 0 {
                // First confirmation - advance to final step
                m.ConfirmStep = 1
                m.Status = ""
                return m, nil
            } else if m.ConfirmStep == 1 {
                // Final confirmation - delete account
                m.Status = "Deleting account..."
                return m, deleteAccountCmd(m.Account.Id)
            }
        case "n", "N", "esc":
            // Cancel at any step
            m.ConfirmStep = 0
            m.Status = "Deletion cancelled"
            m.Error = ""
            return m, clearStatusAfter(2 * time.Second)
        }
    }

    return m, nil
}
```

---

## Styles

```go
var (
    warningStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_ERROR)).
        Bold(true)

    confirmStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_SECONDARY))

    instructionStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_DIM))
)
```

---

## Delete Command

```go
func deleteAccountCmd(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        err := deleteAccount(accountId)
        return deleteAccountResultMsg{err: err}
    }
}

func deleteAccount(accountId uuid.UUID) error {
    database := db.GetDB()
    err := database.DeleteAccount(accountId)
    if err != nil {
        log.Printf("Failed to delete account %s: %v", accountId, err)
        return err
    }
    log.Printf("Successfully deleted account %s", accountId)
    return nil
}
```

---

## Data Deleted

When an account is deleted, the following data is removed:
- Account record
- All notes/posts created by the user
- All follow relationships (both followers and following)
- All activities (likes, boosts, etc.)
- All notifications
- SSH public key associations

---

## Message Types

```go
// clearStatusMsg is sent after a delay to clear status/error messages
type clearStatusMsg struct{}

// showByeByeMsg is sent after deletion to show goodbye message
type showByeByeMsg struct{}

// deleteAccountResultMsg is sent when the delete operation completes
type deleteAccountResultMsg struct {
    err error
}
```

---

## Timing Sequence

1. User confirms final deletion → `deleteAccountCmd` executes
2. On success → Show "Account deleted successfully"
3. Wait 2 seconds → Show "Bye bye!"
4. Wait 2 more seconds → `tea.Quit()` triggers application exit

---

## View Rendering

```go
func (m Model) View() tea.View {
    var s strings.Builder

    s.WriteString(common.CaptionStyle.Render("delete account"))
    s.WriteString("\n\n")

    // Show "Bye bye!" at the end
    if m.ShowByeBye {
        byeStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color(common.COLOR_SUCCESS)).
            Bold(true).
            Align(lipgloss.Center)
        s.WriteString("\n\n")
        s.WriteString(byeStyle.Render("Bye bye!"))
        s.WriteString("\n\n")
        return s.String()
    }

    // Show completion status
    if m.DeletionStatus == "completed" {
        s.WriteString(confirmStyle.Render("✓ Account deleted successfully"))
        s.WriteString("\n\n")
        s.WriteString(instructionStyle.Render("Logging out..."))
        return s.String()
    }

    // Show confirmation steps
    if m.ConfirmStep == 0 {
        // Initial warning
        s.WriteString(warningStyle.Render("⚠ WARNING: This will permanently delete your account!"))
        s.WriteString("\n\n")
        s.WriteString("The following data will be deleted:\n")
        s.WriteString("  • Your account (@" + m.Account.Username + ")\n")
        s.WriteString("  • All your posts and notes\n")
        s.WriteString("  • All follow relationships\n")
        s.WriteString("  • All your activities\n")
        s.WriteString("\n")
        s.WriteString(warningStyle.Render("This action CANNOT be undone!"))
        s.WriteString("\n\n")
        s.WriteString("Are you sure you want to delete your account?\n\n")
        s.WriteString(instructionStyle.Render("Press 'y' to continue or 'n'/'esc' to cancel"))
    } else if m.ConfirmStep == 1 {
        // Final confirmation
        s.WriteString(warningStyle.Render("⚠ FINAL WARNING!"))
        s.WriteString("\n\n")
        s.WriteString("You are about to permanently delete account: ")
        s.WriteString(warningStyle.Render("@" + m.Account.Username))
        s.WriteString("\n\n")
        s.WriteString("This is your last chance to cancel.\n")
        s.WriteString("After this, your account and all data will be gone forever.\n\n")
        s.WriteString(instructionStyle.Render("Press 'y' to DELETE PERMANENTLY or 'n'/'esc' to cancel"))
    }

    // Status/Error messages
    s.WriteString("\n\n")
    if m.Status != "" {
        s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_SUCCESS)).Render(m.Status))
        s.WriteString("\n")
    }
    if m.Error != "" {
        s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(common.COLOR_ERROR)).Render(m.Error))
        s.WriteString("\n")
    }

    return s.String()
}
```

---

## Initialization

```go
func InitialModel(account *domain.Account) Model {
    return Model{
        Account:     account,
        ConfirmStep: 0,
        Status:      "",
        Error:       "",
    }
}

func (m Model) Init() tea.Cmd {
    return nil
}
```

---

## Source Files

- `ui/deleteaccount/deleteaccount.go` - DeleteAccount view implementation
- `ui/common/styles.go` - Color constants (COLOR_ERROR, COLOR_SUCCESS, etc.)
- `db/db.go` - DeleteAccount (cascading delete of all user data)
