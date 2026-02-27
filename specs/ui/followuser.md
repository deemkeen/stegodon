# FollowUser View

This document specifies the FollowUser view, which handles WebFinger-based remote user following.

---

## Overview

The FollowUser view allows users to follow remote ActivityPub accounts by entering their handle in `user@domain` format. It performs:
- WebFinger resolution to discover the actor URI
- ActivityPub Follow activity sending
- Duplicate and self-follow prevention
- Status feedback for success/error states

---

## Data Structure

```go
type Model struct {
    TextInput textinput.Model
    AccountId uuid.UUID        // Local user performing the follow
    Status    string           // Success message
    Error     string           // Error message
}
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ follow remote user                                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Enter ActivityPub address:                                   │
│ (e.g., user@mastodon.social or @user@mastodon.social)       │
│                                                              │
│ ▸ [ user@domain.com                                     ]   │
│                                                              │
│ ✓ Sent follow request to user@domain.com                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Input Handling

### Input Format

Users can enter handles in two formats:
- `user@domain.com`
- `@user@domain.com`

```go
// Remove leading @ if present
input = strings.TrimPrefix(input, "@")

parts := strings.Split(input, "@")
if len(parts) != 2 {
    m.Error = "Invalid format. Use: user@domain.com or @user@domain.com"
    return m, clearStatusAfter(2 * time.Second)
}

username := parts[0]
domain := parts[1]
```

### Input Validation

```go
ti := textinput.New()
ti.Placeholder = "user@domain or @user@domain"
ti.Prompt = common.ListSelectedPrefix
ti.CharLimit = 100
ti.Width = 50
```

---

## Follow Flow

```
User enters: user@domain.com
      │
      ▼
Validate Input Format
      │
      ├── Invalid → Show error, clear after 2s
      └── Valid → Continue
            │
            ▼
Check if Local User
      │
      ├── Local → "This user is local. Follow them directly on this server."
      └── Remote → Continue
            │
            ▼
Show "Following user@domain..."
      │
      ▼
WebFinger Resolution
      │
      ├── GET https://domain/.well-known/webfinger?resource=acct:user@domain
      └── Extract actor URI from links
            │
            ▼
Send Follow Activity
      │
      ├── Create Follow with generated URI
      ├── Sign with local account's key
      └── POST to remote inbox
            │
            ▼
Result
      │
      ├── Success → "✓ Sent follow request to user@domain"
      ├── Already following → "ℹ Already following user@domain"
      ├── Pending → "ℹ Follow request pending for user@domain"
      └── Error → "Failed: <error message>"
```

---

## WebFinger Resolution

```go
func ResolveWebFinger(username, domain string) (string, error) {
    // Build WebFinger URL
    url := fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s",
        domain, username, domain)

    // Fetch and parse
    resp, err := http.Get(url)
    // ...

    // Find self link with ActivityPub type
    for _, link := range webfinger.Links {
        if link.Rel == "self" && link.Type == "application/activity+json" {
            return link.Href, nil  // Actor URI
        }
    }
}
```

---

## Follow Activity

```go
func SendFollow(localAccount *domain.Account, actorURI string, conf *util.Config) error {
    // Check for existing follow
    // ...

    // Create Follow activity
    follow := Follow{
        Context: "https://www.w3.org/ns/activitystreams",
        Type:    "Follow",
        ID:      generateFollowURI(),
        Actor:   localAccountActorURI,
        Object:  actorURI,
    }

    // Sign and send to remote inbox
    // ...
}
```

---

## Error Handling

### Local User Prevention

```go
conf, err := util.ReadConf()
if err == nil && strings.EqualFold(domain, conf.Conf.SslDomain) {
    m.Error = "This user is local. Follow them directly on this server."
    return m, clearStatusAfter(3 * time.Second)
}
```

### Result Messages

| Scenario | Message |
|----------|---------|
| Success | `✓ Sent follow request to user@domain` |
| Already following | `ℹ Already following user@domain` |
| Follow pending | `ℹ Follow request pending for user@domain` |
| Self-follow | `ℹ Self-follow not allowed on stegodon for now` |
| WebFinger failure | `Failed: webfinger resolution failed: ...` |
| Other error | `Failed: <error message>` |

---

## Status Clearing

Status and error messages auto-clear after 2 seconds:

```go
type clearStatusMsg struct{}

func clearStatusAfter(d time.Duration) tea.Cmd {
    return tea.Tick(d, func(t time.Time) tea.Msg {
        return clearStatusMsg{}
    })
}

case clearStatusMsg:
    m.Status = ""
    m.Error = ""
    m.TextInput.SetValue("")
    return m, nil
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Submit follow request |
| `Esc` | Clear input and messages |

---

## Update Logic

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case clearStatusMsg:
        m.Status = ""
        m.Error = ""
        m.TextInput.SetValue("")
        return m, nil

    case followResultMsg:
        if msg.err != nil {
            errMsg := strings.ToLower(msg.err.Error())
            if strings.Contains(errMsg, "already following") {
                m.Status = fmt.Sprintf("ℹ Already following %s", msg.username)
            } else if strings.Contains(errMsg, "follow pending") {
                m.Status = fmt.Sprintf("ℹ Follow request pending for %s", msg.username)
            } else if strings.Contains(errMsg, "self-follow not allowed") {
                m.Status = "ℹ Self-follow not allowed on stegodon for now"
            } else {
                m.Error = fmt.Sprintf("Failed: %v", msg.err)
            }
        } else {
            m.Status = fmt.Sprintf("✓ Sent follow request to %s", msg.username)
        }
        return m, clearStatusAfter(2 * time.Second)

    case tea.KeyPressMsg:
        switch msg.String() {
        case "enter":
            // Validate and follow
        case "esc":
            m.TextInput.SetValue("")
            m.Status = ""
            m.Error = ""
        }
    }

    m.TextInput, cmd = m.TextInput.Update(msg)
    return m, cmd
}
```

---

## View Rendering

```go
func (m Model) View() tea.View {
    var s strings.Builder

    s.WriteString(common.CaptionStyle.Render("follow remote user"))
    s.WriteString("\n\n")
    s.WriteString("Enter ActivityPub address:\n")
    s.WriteString("(e.g., user@mastodon.social or @user@mastodon.social)\n\n")
    s.WriteString(m.TextInput.View())
    s.WriteString("\n\n")

    if m.Status != "" {
        s.WriteString(lipgloss.NewStyle().
            Foreground(lipgloss.Color(common.COLOR_SUCCESS)).
            Render(m.Status))
        s.WriteString("\n")
    }

    if m.Error != "" {
        s.WriteString(lipgloss.NewStyle().
            Foreground(lipgloss.Color(common.COLOR_ERROR)).
            Render(m.Error))
        s.WriteString("\n")
    }

    return s.String()
}
```

---

## Initialization

```go
func InitialModel(accountId uuid.UUID) Model {
    ti := textinput.New()
    ti.Placeholder = "user@domain or @user@domain"
    ti.Prompt = common.ListSelectedPrefix
    ti.Focus()
    ti.CharLimit = 100
    ti.Width = 50

    return Model{
        TextInput: ti,
        AccountId: accountId,
        Status:    "",
        Error:     "",
    }
}
```

---

## Background Follow Command

```go
func followRemoteUserCmd(accountId uuid.UUID, username, domain, fullUsername string) tea.Cmd {
    return func() tea.Msg {
        err := followRemoteUser(accountId, username, domain)
        return followResultMsg{
            username: fullUsername,
            err:      err,
        }
    }
}

func followRemoteUser(accountId uuid.UUID, username, domain string) error {
    // Get local account
    database := db.GetDB()
    err, localAccount := database.ReadAccById(accountId)

    // Resolve WebFinger
    actorURI, err := web.ResolveWebFinger(username, domain)

    // Get config
    conf, err := util.ReadConf()

    // Send Follow activity
    return activitypub.SendFollow(localAccount, actorURI, conf)
}
```

---

## Source Files

- `ui/followuser/followuser.go` - FollowUser view implementation
- `ui/followuser/followuser_test.go` - Tests
- `web/webfinger.go` - WebFinger resolution
- `activitypub/outbox.go` - SendFollow activity
