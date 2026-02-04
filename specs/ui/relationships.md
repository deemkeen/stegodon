# Followers/Following Views

This document specifies the Followers and Following views, which display paginated lists of relationship data.

---

## Overview

These views display the user's social connections:
- **Followers**: Accounts that follow the current user
- **Following**: Accounts the current user follows

Both views support pagination, distinguish between local and remote accounts, and the Following view includes unfollow functionality.

---

## Followers View

### Data Structure

```go
type Model struct {
    AccountId uuid.UUID
    Followers []domain.Follow
    Selected  int
    Offset    int              // Pagination offset
    Width     int
    Height    int
}
```

### View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ followers (12)                                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ @alice [local]                                             │  ← Selected
│   @bob@mastodon.social                                       │
│   @charlie [local]                                           │
│   @diana@pleroma.site                                        │
│   @eve [local]                                               │
│                                                              │
│ showing 1-5 of 12                                            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Account Display

Local and remote followers are displayed differently:

```go
if follow.IsLocal {
    // Local follower - lookup in accounts table
    err, localAcc := database.ReadAccById(follow.AccountId)
    username = "@" + localAcc.Username
    badge = " [local]"
} else {
    // Remote follower - lookup in remote_accounts table
    err, remoteAcc := database.ReadRemoteAccountById(follow.AccountId)
    username = fmt.Sprintf("@%s@%s", remoteAcc.Username, remoteAcc.Domain)
    badge = ""
}
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | View profile of selected follower |
| `f` | Follow back (sends follow request for remote users) |

### Empty State

```go
if len(m.Followers) == 0 {
    s.WriteString(common.ListEmptyStyle.Render(
        "No followers yet. Share your profile to get followers!"))
}
```

---

## Following View

### Data Structure

```go
type Model struct {
    AccountId uuid.UUID
    Following []domain.Follow
    Selected  int
    Offset    int              // Pagination offset
    Width     int
    Height    int
    Status    string           // Success message
    Error     string           // Error message
}
```

### View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ following (8)                                                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ @alice [local]                                             │  ← Selected
│   @bob@mastodon.social                                       │
│   @charlie [local] [pending]                                 │  ← Pending follow
│   @diana@pleroma.site                                        │
│                                                              │
│ showing 1-4 of 8                                             │
│                                                              │
│ Unfollowed @bob@mastodon.social                              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Account Display

Shows follow status (pending vs accepted):

```go
if follow.IsLocal {
    err, localAcc := database.ReadAccById(follow.TargetAccountId)
    username = "@" + localAcc.Username
    badge = " [local]"
    if !follow.Accepted {
        badge += " [pending]"
    }
} else {
    err, remoteAcc := database.ReadRemoteAccountById(follow.TargetAccountId)
    username = fmt.Sprintf("@%s@%s", remoteAcc.Username, remoteAcc.Domain)
    badge = ""
    if !follow.Accepted {
        badge = " [pending]"
    }
}
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | View profile of selected user |
| `f` | Unfollow selected account |

---

## Unfollow Flow

### Process

```
User presses 'u' or 'Enter'
      │
      ▼
Get Selected Follow
      │
      ├── Local follow
      │     └── Delete from database directly
      │
      └── Remote follow
            │
            ├── Get local and remote account details
            ├── Send Undo(Follow) activity
            └── Delete from database
                  │
                  ▼
Update UI
      │
      ├── Remove from list
      ├── Adjust selection if needed
      └── Show "Unfollowed @user" status
```

### Unfollow Logic

```go
case "f":
    if len(m.Following) > 0 && m.Selected < len(m.Following) {
        selectedFollow := m.Following[m.Selected]

        // Get display name for status message
        var displayName string
        if selectedFollow.IsLocal {
            err, localAcc := database.ReadAccById(selectedFollow.TargetAccountId)
            displayName = "@" + localAcc.Username
        } else {
            err, remoteAcc := database.ReadRemoteAccountById(selectedFollow.TargetAccountId)
            displayName = fmt.Sprintf("@%s@%s", remoteAcc.Username, remoteAcc.Domain)
        }

        // Delete follow (in background goroutine)
        go func() {
            if selectedFollow.IsLocal {
                database.DeleteFollowByAccountIds(m.AccountId, selectedFollow.TargetAccountId)
            } else {
                // Send Undo activity first
                if conf.WithAp {
                    activitypub.SendUndo(localAccount, &selectedFollow, remoteAccount, conf)
                }
                database.DeleteFollowByURI(selectedFollow.URI)
            }
        }()

        // Update local list immediately
        m.Following = append(m.Following[:m.Selected], m.Following[m.Selected+1:]...)
        if m.Selected >= len(m.Following) && m.Selected > 0 {
            m.Selected--
        }

        m.Status = fmt.Sprintf("Unfollowed %s", displayName)
        return m, clearStatusAfter(2 * time.Second)
    }
```

### Undo Activity

Sent to remote servers when unfollowing:

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Undo",
    "id": "https://stegodon.example/undo/uuid",
    "actor": "https://stegodon.example/users/localuser",
    "object": {
        "type": "Follow",
        "id": "https://stegodon.example/follows/original-uuid",
        "actor": "https://stegodon.example/users/localuser",
        "object": "https://remote.server/users/remoteuser"
    }
}
```

---

## Pagination

Both views share the same pagination logic:

```go
const DefaultItemsPerPage = 10

start := m.Offset
end := min(start + DefaultItemsPerPage, len(items))

for i := start; i < end; i++ {
    // Render item
}

// Pagination indicator
if len(items) > DefaultItemsPerPage {
    paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(items))
    s.WriteString(common.ListBadgeStyle.Render(paginationText))
}
```

### Scroll Behavior

```go
case "up", "k":
    if m.Selected > 0 {
        m.Selected--
        if m.Selected < m.Offset {
            m.Offset = m.Selected
        }
    }

case "down", "j":
    if m.Selected < len(items)-1 {
        m.Selected++
        if m.Selected >= m.Offset + DefaultItemsPerPage {
            m.Offset = m.Selected - DefaultItemsPerPage + 1
        }
    }
```

---

## Data Loading

### Followers

```go
func loadFollowers(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()

        // Clean up orphaned follows
        database.CleanupOrphanedFollows()

        // Load followers where target_account_id = accountId
        err, followers := database.ReadFollowersByAccountId(accountId)
        return followersLoadedMsg{followers: *followers}
    }
}
```

### Following

```go
func loadFollowing(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()

        // Clean up orphaned follows
        database.CleanupOrphanedFollows()

        // Load follows where account_id = accountId
        err, following := database.ReadFollowingByAccountId(accountId)
        return followingLoadedMsg{following: *following}
    }
}
```

---

## Selection Styling

```go
if i == m.Selected {
    text := common.ListItemSelectedStyle.Render(username + badge)
    s.WriteString(common.ListSelectedPrefix + text)
} else {
    text := username + common.ListBadgeStyle.Render(badge)
    s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
}
```

---

## Status Clearing

Following view clears status after 2 seconds:

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
    return m, nil
```

---

## Orphaned Follow Cleanup

Both views clean up orphaned follows before loading:

```go
func CleanupOrphanedFollows() error {
    // Delete follows where account no longer exists
    // in either accounts or remote_accounts table
}
```

This handles cases where remote accounts were removed from the cache.

---

## Initialization

### Followers

```go
func InitialModel(accountId uuid.UUID, width, height int) Model {
    return Model{
        AccountId: accountId,
        Followers: []domain.Follow{},
        Selected:  0,
        Offset:    0,
        Width:     width,
        Height:    height,
    }
}

func (m Model) Init() tea.Cmd {
    return loadFollowers(m.AccountId)
}
```

### Following

```go
func InitialModel(accountId uuid.UUID, width, height int) Model {
    return Model{
        AccountId: accountId,
        Following: []domain.Follow{},
        Selected:  0,
        Offset:    0,
        Width:     width,
        Height:    height,
        Status:    "",
        Error:     "",
    }
}

func (m Model) Init() tea.Cmd {
    return loadFollowing(m.AccountId)
}
```

---

## Empty States

### Followers

```go
if len(m.Followers) == 0 {
    s.WriteString(common.ListEmptyStyle.Render(
        "No followers yet. Share your profile to get followers!"))
}
```

### Following

```go
if len(m.Following) == 0 {
    s.WriteString(common.ListEmptyStyle.Render(
        "You're not following anyone yet.\nUse the follow user view to start following!"))
}
```

---

## Profile Navigation

Both Followers and Following views support pressing Enter to view the selected user's profile:

```go
case "enter":
    if len(m.Followers) > 0 && m.Selected < len(m.Followers) {
        follower := m.Followers[m.Selected]
        database := db.GetDB()

        if follower.IsLocal {
            err, localAcc := database.ReadAccById(follower.AccountId)
            if err == nil && localAcc != nil {
                return m, func() tea.Msg {
                    return common.ViewProfileMsg{
                        Username:  localAcc.Username,
                        AccountId: localAcc.Id,
                        IsRemote:  false,
                    }
                }
            }
        } else {
            err, remoteAcc := database.ReadRemoteAccountById(follower.AccountId)
            if err == nil && remoteAcc != nil {
                return m, func() tea.Msg {
                    return common.ViewProfileMsg{
                        Username:  remoteAcc.Username,
                        AccountId: remoteAcc.Id,
                        IsRemote:  true,
                        ActorURI:  remoteAcc.ActorURI,
                        Domain:    remoteAcc.Domain,
                    }
                }
            }
        }
    }
```

The `ViewProfileMsg` includes the `IsRemote` flag to indicate whether the profile is local or federated, allowing ProfileView to load the appropriate data.

---

## Source Files

- `ui/followers/followers.go` - Followers view implementation
- `ui/following/following.go` - Following view implementation
- `ui/common/styles.go` - List styles
- `ui/common/commands.go` - ViewProfileMsg definition
- `activitypub/outbox.go` - SendUndo for unfollowing
- `db/db.go` - ReadFollowersByAccountId, ReadFollowingByAccountId, DeleteFollowByURI
