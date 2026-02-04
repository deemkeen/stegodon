# ProfileView

This document specifies the ProfileView, which displays user profiles for both local and remote (federated) users.

---

## Overview

ProfileView displays a user's profile information including avatar, display name, handle, bio, follow status, and recent posts. The view supports both local users (from the `accounts` table) and remote/federated users (from the `remote_accounts` and `activities` tables).

The view is accessed by pressing Enter on a user in the Followers, Following, or LocalUsers views. It is NOT part of the Tab navigation cycle - users return to their previous view by pressing Esc.

---

## Data Structure

```go
type Model struct {
    AccountId          uuid.UUID             // Viewing user's account ID
    ProfileUser        *domain.Account       // Local user data
    RemoteProfileUser  *domain.RemoteAccount // Remote user data
    IsRemoteProfile    bool                  // Which type is active
    Posts              []domain.Note         // Local user posts
    RemotePosts        []remotePost          // Remote user posts
    IsFollowing        bool                  // Whether viewer follows this user
    FollowPending      bool                  // For pending remote follows
    Selected           int                   // Currently selected post
    Offset             int                   // Pagination offset
    Width              int
    Height             int
    loading            bool
    Status             string                // Success message
    Error              string                // Error message
    LocalDomain        string                // Local server domain
    AvatarRendered     string                // Pre-rendered avatar string
    ReturnView         common.SessionState   // Where to return on Esc
}

// remotePost represents a post from a remote user (extracted from activity raw_json)
type remotePost struct {
    ObjectURI  string
    ObjectURL  string
    Content    string
    CreatedAt  time.Time
    Author     string
    Domain     string
    LikeCount  int
    BoostCount int
}
```

---

## View Layout

### Local Profile

```
┌─────────────────────────────────────────────────────────────────────────┐
│ profile                                                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│ ████████████   Alice Smith                                               │
│ ████████████   @alice                                                    │
│ ████████████                                                             │
│ ████████████   Software developer and coffee enthusiast.                 │
│ ████████████                                                             │
│ ████████████   joined 30d ago · following                                │
│                                                                          │
│ ───────────────────────────────────────────────────────────────────────  │
│                                                                          │
│ recent posts (5)                                                         │
│                                                                          │
│ 2h ago                                                                   │
│ @alice                                                                   │
│ Just pushed a new feature! Check it out.                                 │
│                                                                          │
│ 1d ago                                                           (sel)   │
│ @alice                                                                   │
│ Working on some cool stuff today.                                        │
│                                                                          │
│ showing 1-5 of 5                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Remote Profile

```
┌─────────────────────────────────────────────────────────────────────────┐
│ profile                                                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│ ████████████   Bob Jones                                                 │
│ ████████████   @bob@mastodon.social                                      │
│ ████████████                                                             │
│ ████████████   Open source contributor and cat person.                   │
│ ████████████                                                             │
│ ████████████   remote · following                                        │
│                                                                          │
│ ───────────────────────────────────────────────────────────────────────  │
│                                                                          │
│ recent posts (3)                                                         │
│                                                                          │
│ 5h ago                                                                   │
│ @bob@mastodon.social                                                     │
│ New blog post about Rust is up!                                          │
│                                                                          │
│ showing 1-3 of 3                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Profile Header Components

### Avatar

- **Dimensions**: 12 characters wide × 6 rows tall (using half-block characters)
- **Local users**: Loaded from file path in `AvatarURL` field
- **Remote users**: Fetched via HTTP using `LoadRemoteAvatarImage()`
- **Fallback**: Default stegodon logo if no avatar available

### Display Name

- Shows `DisplayName` if set, otherwise `Username`
- Styled with `COLOR_USERNAME` (#00ff87), bold

### Handle

- **Local**: `@username`
- **Remote**: `@username@domain`
- Styled with `COLOR_SECONDARY` (#5fafff)

### Bio

- Shows `Summary` field if present
- Remote bios have HTML stripped via `StripHTMLTags()`
- Styled with `COLOR_WHITE` (#eeeeee)

### Metadata Line

**Local profiles:**
```
joined 30d ago · following
```

**Remote profiles:**
```
remote · following
```

### Follow Status Badges

| Status | Style | Color |
|--------|-------|-------|
| `following` | followBadgeStyle | COLOR_SUCCESS (#00ff87) |
| `not following` | notFollowBadgeStyle | COLOR_DIM (#585858) |
| `pending` | pendingBadgeStyle | COLOR_WARNING (#ffaf00) |
| `remote` | remoteBadgeStyle | COLOR_SECONDARY (#5fafff) |

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up in posts list |
| `↓` / `j` | Move selection down in posts list |
| `Enter` | View thread for selected post |
| `f` | Toggle follow/unfollow |
| `Esc` | Return to previous view (ReturnView) |

---

## Data Loading

### Local Profile Loading

```go
func loadProfile(viewerAccountId uuid.UUID, username string) tea.Cmd
```

1. Load account from `accounts` table by username
2. Load notes from `notes` table (top-level posts only, max 10)
3. Check follow status via `IsFollowingLocal()`
4. Render avatar (custom or default logo)

### Remote Profile Loading

```go
func loadRemoteProfile(viewerAccountId uuid.UUID, actorURI, username, domain string) tea.Cmd
```

1. Load account from `remote_accounts` table by ActorURI
2. Load activities from `activities` table via `ReadActivitiesByActorURI()`
3. Extract post content from activity `raw_json` field
4. Check follow status via `ReadFollowByAccountIds()`
5. Render avatar (fetched via HTTP or default logo)

---

## Follow/Unfollow Logic

### Local Follow Toggle

```go
func toggleFollow(viewerAccountId uuid.UUID, profileUser *domain.Account, isFollowing bool) tea.Cmd
```

- **Follow**: Creates local follow via `CreateLocalFollow()`, creates notification
- **Unfollow**: Deletes via `DeleteLocalFollow()`

### Remote Follow Toggle

```go
func toggleRemoteFollow(viewerAccountId uuid.UUID, remoteUser *domain.RemoteAccount, isFollowing bool) tea.Cmd
```

- **Follow**: Sends Follow activity via `activitypub.SendFollow()`, sets `followPending=true`
- **Unfollow**: Sends Undo activity via `activitypub.SendUndo()`, deletes local record

---

## ReturnView Mechanism

ProfileView tracks where the user came from and returns there on Esc:

```go
ReturnView common.SessionState // Where to return on Esc
```

When ProfileView receives a `ViewProfileMsg`, it stores the source view. On Esc, it returns that SessionState as a message, causing the main model to switch back.

**Navigation flow:**
```
Followers → (Enter) → ProfileView → (Esc) → Followers
Following → (Enter) → ProfileView → (Esc) → Following
LocalUsers → (Enter) → ProfileView → (Esc) → LocalUsers
```

---

## ViewProfileMsg

The message used to navigate to ProfileView:

```go
type ViewProfileMsg struct {
    Username  string    // Username for display/lookup
    AccountId uuid.UUID // Account ID (local or remote)
    IsRemote  bool      // true for remote/federated users
    ActorURI  string    // ActivityPub actor URI (remote only)
    Domain    string    // Domain for display (remote only)
}
```

---

## Post Display

### Content Processing

Posts are processed before display:
1. `StripHTMLTags()` - Remove HTML (remote posts)
2. `TruncateContent()` - Limit to `MaxDisplayContentLength`
3. `UnescapeHTML()` - Convert HTML entities
4. `MarkdownLinksToTerminal()` - Convert links
5. `LinkifyRawURLsTerminal()` - Make URLs clickable
6. `HighlightHashtagsTerminal()` - Highlight hashtags
7. `HighlightMentionsTerminal()` - Highlight mentions

### Post Layout

Each post displays:
1. Timestamp (relative, e.g., "2h ago")
2. Author handle
3. Content (processed)

### Selection Styling

Selected posts use inverted colors:
- Background: `COLOR_ACCENT` (#5f87ff)
- Text: `COLOR_WHITE` (#eeeeee)

---

## Pagination

- Maximum posts displayed: 10 (`maxProfilePosts`)
- Items per page: `common.DefaultItemsPerPage` (10)
- Shows pagination indicator when more items exist

---

## Status Messages

| Scenario | Message |
|----------|---------|
| Local follow | "Following @username" |
| Remote follow | "Follow request sent to @username@domain" |
| Unfollow | "Unfollowed @username" or "Unfollowed @username@domain" |
| Error | "Failed to toggle follow: {error}" |

Status messages auto-clear after 2 seconds.

---

## Empty States

```go
// Profile not found
"Error: user not found"

// No profile data
"No profile to display"

// Loading
"Loading profile..."

// No posts (local)
"No posts yet."

// No posts (remote)
"No posts available."
```

---

## Initialization

```go
func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
    return Model{
        AccountId:       accountId,
        ProfileUser:     nil,
        RemoteProfileUser: nil,
        IsRemoteProfile: false,
        Posts:           []domain.Note{},
        RemotePosts:     []remotePost{},
        IsFollowing:     false,
        FollowPending:   false,
        Selected:        0,
        Offset:          0,
        Width:           width,
        Height:          height,
        loading:         false,
        Status:          "",
        Error:           "",
        LocalDomain:     localDomain,
        ReturnView:      common.LocalUsersView,
    }
}

func (m Model) Init() tea.Cmd {
    return nil  // Data loaded via ViewProfileMsg
}
```

---

## Constants

```go
const (
    maxProfilePosts = 10  // Maximum posts to show
    avatarCols      = 12  // Avatar width in characters
    avatarRows      = 6   // Avatar height in rows
)
```

---

## Source Files

- `ui/profileview/profileview.go` - ProfileView implementation
- `ui/common/commands.go` - ViewProfileMsg definition
- `util/avatar.go` - Avatar loading and rendering
- `db/db.go` - ReadActivitiesByActorURI, ReadFollowByAccountIds
