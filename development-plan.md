# Stegodon ActivityPub Federation - Development Plan

**Version**: 1.0  
**Created**: 2025-11-07  
**Target Completion**: 8 weeks from start  
**Status**: Planning Complete ✓

---

## Executive Summary

This plan transforms stegodon from a local/RSS-based blog system into a fully-federated ActivityPub server that can interact with Mastodon, Pleroma, and the broader Fediverse - all through the existing SSH TUI interface.

**Current State**: ~15-20% implemented (WebFinger, actor discovery, keypairs exist)  
**Target State**: Full ActivityPub federation with timeline, follows, likes, and replies  
**Approach**: Phased MVP-first implementation using custom ActivityPub logic with proven libraries

### User Requirements
- **Navigation**: Hybrid approach (Tab for main feeds, number keys 1-5 for quick access)
- **Default Visibility**: Public (federated)
- **Library Strategy**: Custom implementation with `go-ap/activitypub` for types and `superseriousbusiness/httpsig` for signatures

---

## Table of Contents

1. [Phase 1: Database Foundation & Core Types](#phase-1-database-foundation--core-types-week-1-2)
2. [Phase 2: HTTP Signatures & Actor Discovery](#phase-2-http-signatures--actor-discovery-week-2-3)
3. [Phase 3: Core ActivityPub Protocol](#phase-3-core-activitypub-protocol-week-3-5)
4. [Phase 4: TUI Enhancement - MVP Timeline](#phase-4-tui-enhancement---mvp-timeline-week-5-6)
5. [Phase 5: Interactions - Likes & Comments](#phase-5-interactions---likes--comments-week-6-7)
6. [Phase 6: Polish & Production Readiness](#phase-6-polish--production-readiness-week-7-8)
7. [Implementation Checklist](#implementation-checklist)
8. [Technical Decisions](#technical-decisions)
9. [Testing Strategy](#testing-strategy)

---

## PHASE 1: Database Foundation & Core Types (Week 1-2)

### 1.1 Add ActivityPub Dependencies

```bash
go get github.com/go-ap/activitypub@latest
go get codeberg.org/superseriousbusiness/httpsig@latest
```

**Why these libraries?**
- `go-ap/activitypub`: Actively maintained (2025), provides ActivityStreams vocabulary types
- `superseriousbusiness/httpsig`: Battle-tested in GoToSocial, implements HTTP signature spec correctly

### 1.2 Database Schema Migrations

Create new file: `db/migrations.go`

#### New Tables

**1. follows** - Track follow relationships
```sql
CREATE TABLE IF NOT EXISTS follows (
    id uuid NOT NULL PRIMARY KEY,
    account_id uuid NOT NULL,
    target_account_id uuid NOT NULL,
    uri text NOT NULL,
    created_at timestamp DEFAULT current_timestamp,
    accepted boolean DEFAULT false
);

CREATE INDEX idx_follows_account_id ON follows(account_id);
CREATE INDEX idx_follows_target_account_id ON follows(target_account_id);
```

**2. remote_accounts** - Cache federated user data
```sql
CREATE TABLE IF NOT EXISTS remote_accounts (
    id uuid NOT NULL PRIMARY KEY,
    username text NOT NULL,
    domain text NOT NULL,
    actor_uri text UNIQUE NOT NULL,
    display_name text,
    summary text,
    inbox_uri text NOT NULL,
    outbox_uri text,
    public_key_pem text NOT NULL,
    avatar_url text,
    last_fetched_at timestamp DEFAULT current_timestamp,
    UNIQUE(username, domain)
);

CREATE INDEX idx_remote_accounts_actor_uri ON remote_accounts(actor_uri);
```

**3. activities** - Store incoming/outgoing AP activities
```sql
CREATE TABLE IF NOT EXISTS activities (
    id uuid NOT NULL PRIMARY KEY,
    activity_uri text UNIQUE NOT NULL,
    activity_type text NOT NULL,
    actor_uri text NOT NULL,
    object_uri text,
    raw_json text NOT NULL,
    processed boolean DEFAULT false,
    created_at timestamp DEFAULT current_timestamp,
    local boolean DEFAULT false
);

CREATE INDEX idx_activities_uri ON activities(activity_uri);
CREATE INDEX idx_activities_processed ON activities(processed);
```

**4. likes** - Favorites/likes on posts
```sql
CREATE TABLE IF NOT EXISTS likes (
    id uuid NOT NULL PRIMARY KEY,
    account_id uuid NOT NULL,
    note_id uuid NOT NULL,
    uri text NOT NULL,
    created_at timestamp DEFAULT current_timestamp,
    UNIQUE(account_id, note_id)
);

CREATE INDEX idx_likes_note_id ON likes(note_id);
```

**5. delivery_queue** - Outgoing activity delivery queue
```sql
CREATE TABLE IF NOT EXISTS delivery_queue (
    id uuid NOT NULL PRIMARY KEY,
    activity_id uuid NOT NULL,
    inbox_uri text NOT NULL,
    attempts int DEFAULT 0,
    next_retry_at timestamp DEFAULT current_timestamp,
    last_error text,
    created_at timestamp DEFAULT current_timestamp
);

CREATE INDEX idx_delivery_queue_next_retry ON delivery_queue(next_retry_at);
```

#### Extend Existing Tables

```sql
-- Extend accounts table
ALTER TABLE accounts ADD COLUMN display_name text;
ALTER TABLE accounts ADD COLUMN summary text;
ALTER TABLE accounts ADD COLUMN avatar_url text;

-- Extend notes table
ALTER TABLE notes ADD COLUMN visibility text DEFAULT 'public';
ALTER TABLE notes ADD COLUMN in_reply_to_uri text;
ALTER TABLE notes ADD COLUMN object_uri text UNIQUE;
ALTER TABLE notes ADD COLUMN federated boolean DEFAULT true;
ALTER TABLE notes ADD COLUMN sensitive boolean DEFAULT false;
ALTER TABLE notes ADD COLUMN content_warning text;

CREATE INDEX idx_notes_user_id ON notes(user_id);
CREATE INDEX idx_notes_created_at ON notes(created_at DESC);
CREATE INDEX idx_notes_object_uri ON notes(object_uri);
```

### 1.3 Domain Models

Create `domain/activitypub.go`:

```go
package domain

import (
    "github.com/google/uuid"
    "time"
)

type RemoteAccount struct {
    Id            uuid.UUID
    Username      string
    Domain        string
    ActorURI      string
    DisplayName   string
    Summary       string
    InboxURI      string
    OutboxURI     string
    PublicKeyPem  string
    AvatarURL     string
    LastFetchedAt time.Time
}

type Follow struct {
    Id              uuid.UUID
    AccountId       uuid.UUID
    TargetAccountId uuid.UUID
    URI             string
    CreatedAt       time.Time
    Accepted        bool
}

type Like struct {
    Id        uuid.UUID
    AccountId uuid.UUID
    NoteId    uuid.UUID
    URI       string
    CreatedAt time.Time
}

type Activity struct {
    Id           uuid.UUID
    ActivityURI  string
    ActivityType string
    ActorURI     string
    ObjectURI    string
    RawJSON      string
    Processed    bool
    CreatedAt    time.Time
    Local        bool
}
```

---

## PHASE 2: HTTP Signatures & Actor Discovery (Week 2-3)

### 2.1 HTTP Signatures Implementation

Create `activitypub/httpsig.go`:

```go
package activitypub

import (
    "codeberg.org/superseriousbusiness/httpsig"
    "crypto/rsa"
    "net/http"
)

func SignRequest(req *http.Request, privateKey *rsa.PrivateKey, keyId string) error {
    signer, _ := httpsig.NewSigner(
        httpsig.RSA_SHA256,
        httpsig.DigestSha256,
        []string{"(request-target)", "host", "date", "digest"},
        httpsig.Signature,
        0,
    )
    return signer.SignRequest(privateKey, keyId, req, nil)
}

func VerifyRequest(req *http.Request, publicKeyPem string) (string, error) {
    // Parse public key and verify signature
    // Return actor URI if valid
}
```

### 2.2 Remote Actor Fetching

Create `activitypub/actors.go`:

```go
func FetchRemoteActor(actorURI string) (*domain.RemoteAccount, error) {
    // HTTP GET with Accept: application/activity+json
    // Parse actor JSON
    // Store in remote_accounts table
    // Return RemoteAccount
}

func GetOrFetchActor(actorURI string) (*domain.RemoteAccount, error) {
    // Check cache first (< 24h old)
    // If stale or missing, fetch fresh
}
```

### 2.3 WebFinger Client

Extend `web/webfinger.go`:

```go
func ResolveWebFinger(username, domain string) (string, error) {
    // Query https://domain/.well-known/webfinger?resource=acct:user@domain
    // Parse JSON and return actor URI
}
```

---

## PHASE 3: Core ActivityPub Protocol (Week 3-5)

### 3.1 Outbox Implementation

Create `activitypub/outbox.go`:

```go
func GenerateOutbox(username string, page int, conf *util.AppConfig) (string, error) {
    // Get user's public/unlisted notes
    // Convert to Create{Note} activities
    // Return OrderedCollection JSON
}

func NoteToCreateActivity(note domain.Note, account domain.Account) (string, error) {
    // Convert Note to ActivityStreams Create{Note} object
}
```

Update `web/router.go` - Replace stub outbox endpoint with real implementation.

### 3.2 Inbox Processing

Create `activitypub/inbox.go`:

```go
type InboxProcessor struct {
    db *db.DB
}

func (p *InboxProcessor) ProcessActivity(rawJSON []byte, actorURI string) error {
    // Parse activity
    // Check for duplicate (activities table)
    // Route to handler based on type
}

func (p *InboxProcessor) handleFollow(activity map[string]interface{}) error
func (p *InboxProcessor) handleAccept(activity map[string]interface{}) error
func (p *InboxProcessor) handleCreate(activity map[string]interface{}) error
func (p *InboxProcessor) handleLike(activity map[string]interface{}) error
func (p *InboxProcessor) handleUndo(activity map[string]interface{}) error
```

Update `web/inbox.go`:

```go
func HandleInbox(c *gin.Context) {
    // Read body
    // Verify HTTP signature
    // Check for duplicate
    // Pass to InboxProcessor
    // Return 202 Accepted
}
```

### 3.3 Activity Delivery

Create `activitypub/delivery.go`:

```go
func DeliverActivity(activityJSON []byte, inboxURI string, privateKey *rsa.PrivateKey) error {
    // Sign and POST to remote inbox
}

func QueueDelivery(activityId uuid.UUID, inboxURIs []string) error {
    // Add to delivery_queue table
}

func StartDeliveryWorker(ctx context.Context, conf *util.AppConfig) {
    // Background worker to process queue
    // Exponential backoff on failures
}
```

Update `main.go` to start delivery worker.

### 3.4 Follow Workflow

Create `activitypub/follows.go`:

```go
func FollowRemoteUser(localAccountID uuid.UUID, remoteActorURI string) error {
    // Fetch remote actor
    // Create Follow activity
    // Store in follows table (accepted=false)
    // Queue delivery
}

func SendAcceptActivity(localAcc *domain.Account, remoteAcc *domain.RemoteAccount, followURI string) error {
    // Create Accept activity
    // Queue delivery
}

func UnfollowRemoteUser(localAccountID, remoteAccountID uuid.UUID) error {
    // Create Undo{Follow} activity
    // Delete from follows table
    // Queue delivery
}
```

### 3.5 Collections Endpoints

Update `web/router.go` to implement:
- `GET /users/:actor/followers` - Return OrderedCollection of follower URIs
- `GET /users/:actor/following` - Return OrderedCollection of following URIs

---

## PHASE 4: TUI Enhancement - MVP Timeline (Week 5-6)

### 4.1 New Session States

Update `ui/common/commands.go`:

```go
const (
    CreateNoteView SessionState = iota
    LocalTimelineView       // Renamed from ListNotesView
    FederatedTimelineView   // NEW
    NotificationsView       // NEW
    FollowManagementView    // NEW
)
```

### 4.2 Navigation System

Add keyboard shortcuts:
- `Tab` - Cycle between CreateNote and Timeline
- `1` - Jump to Local Timeline
- `2` - Jump to Federated Timeline
- `3` - Jump to Notifications
- `4` - Jump to Follow Management
- `Ctrl+R` - Refresh current view

### 4.3 Federated Timeline View

Create `ui/federatedtimeline/timeline.go`:

```go
type Model struct {
    items      []TimelineItem
    paginator  paginator.Model
    accountId  uuid.UUID
}

type TimelineItem struct {
    Author      string  // username@domain or just username
    AuthorURI   string
    Content     string
    CreatedAt   time.Time
    IsLocal     bool
    LikesCount  int
    IsLiked     bool
}
```

Features:
- Shows posts from followed accounts (from activities table WHERE type='Create')
- Merged with local public posts
- Sorted by created_at DESC
- Keyboard: `l` to like, `r` to reply, `f` to follow/unfollow

### 4.4 Local Timeline View

Rename `ui/listnotes/` to `ui/localtimeline/`:
- Keep existing functionality
- Add like counts and indicators
- Same keyboard shortcuts as federated timeline

### 4.5 Notifications View

Create `ui/notifications/notifications.go`:

```go
type NotificationItem struct {
    Type      string  // "follow", "like", "mention"
    Actor     string
    ActorURI  string
    Content   string
    CreatedAt time.Time
}
```

Load from activities table:
- Follows where object=current_user
- Likes on user's posts
- Mentions in posts

### 4.6 Follow Management View

Create `ui/followmanagement/manage.go`:

Two-column layout:
- Left: Followers list
- Right: Following list
- `f` to follow/unfollow selected user
- `v` to view user's profile/posts

### 4.7 Enhanced Write Note View

Update `ui/writenote/writenote.go`:

Add fields:
- `visibility` (public/unlisted/followers/direct)
- `contentWarning`
- `inReplyTo`

Keyboard:
- `Ctrl+V` - Cycle visibility
- `Ctrl+W` - Add content warning

### 4.8 Main UI Integration

Update `ui/supertui.go`:

```go
type MainModel struct {
    state                  common.SessionState
    writeNoteModel         writenote.Model
    localTimelineModel     localtimeline.Model
    federatedTimelineModel federatedtimeline.Model
    notificationsModel     notifications.Model
    followManagementModel  followmanagement.Model
}
```

Layout:
```
┌────────────────────────────────────────────────┐
│ @username | stegodon v1.0 | Joined: 2025-01-01│
├────────────────────────────────────────────────┤
│  [Write Note]  │  [Timeline View]              │
│                │  👤 user@mastodon.social      │
│  [150 chars]   │  Just set up ActivityPub!     │
│  ┌──────────┐  │  ❤️ 5  💬 2                   │
│  │ Hello... │  │                                │
└────────────────────────────────────────────────┘
 Tab | 1:Local 2:Federated 3:Notifs 4:Follows
```

---

## PHASE 5: Interactions - Likes & Comments (Week 6-7)

### 5.1 Like Functionality

Create `activitypub/likes.go`:

```go
func LikePost(accountId, noteId uuid.UUID) error {
    // Create Like activity
    // Store in likes table
    // If remote post, queue delivery to author's inbox
}

func UnlikePost(accountId, noteId uuid.UUID) error {
    // Create Undo{Like} activity
    // Delete from likes table
    // Queue delivery
}

func HandleLike(actorURI, objectURI string) error {
    // Verify object is local post
    // Store like
}
```

Database functions in `db/db.go`:

```go
func (db *DB) CreateLike(like domain.Like) error
func (db *DB) DeleteLike(accountId, noteId uuid.UUID) error
func (db *DB) GetLikeCount(noteId uuid.UUID) (int, error)
func (db *DB) IsLiked(accountId, noteId uuid.UUID) (bool, error)
```

### 5.2 Reply/Comment Functionality

Update note creation to support replies:

```go
func CreateReply(authorId uuid.UUID, content string, inReplyToURI string) error {
    // Create note with in_reply_to_uri set
    // Create Create{Note} activity
    // If replying to remote post, deliver to author's inbox
}
```

TUI updates:
- Press `r` on timeline item to open write note in reply mode
- Show "↩️ Replying to @user" indicator

---

## PHASE 6: Polish & Production Readiness (Week 7-8)

### 6.1 Error Handling & Logging

- Add structured logging (DEBUG, INFO, WARN, ERROR)
- Create `activitypub/logger.go`
- Log all incoming/outgoing activities
- Error recovery in inbox processing

### 6.2 Rate Limiting & Security

Create `middleware/ratelimit.go`:
- Rate limit inbox (100 req/min per IP)
- Rate limit outgoing deliveries (10/sec per remote server)
- Validate Content-Type headers
- Domain blocklist support

### 6.3 Configuration Updates

Update `config.yaml`:

```yaml
conf:
  host: 127.0.0.1
  sshPort: 23232
  httpPort: 9999
  sslDomain: example.com
  withAp: true
  federationEnabled: true
  deliveryWorkers: 5
  maxRetries: 5
  actorCacheTTL: 86400
```

### 6.4 Database Optimizations

- Add all indices (listed in Phase 1)
- Implement connection pooling in `db/db.go`:

```go
var dbInstance *DB
var dbOnce sync.Once

func GetDB() *DB {
    dbOnce.Do(func() {
        db, _ := sql.Open("sqlite", "database.db")
        db.SetMaxOpenConns(25)
        db.SetMaxIdleConns(5)
        dbInstance = &DB{db: db}
    })
    return dbInstance
}
```

### 6.5 Testing & Documentation

- Unit tests for inbox processing with real Mastodon examples
- Test federation with live Mastodon instance
- Update README.md with:
  - Federation setup instructions
  - SSL domain configuration
  - Firewall requirements
  - TUI navigation guide
- Create `docs/activitypub.md`

---

## File Structure Summary

```
stegodon/
├── activitypub/              # NEW - Core AP logic
│   ├── actors.go            # Remote actor fetching
│   ├── delivery.go          # Activity delivery & queue
│   ├── follows.go           # Follow/unfollow workflows
│   ├── httpsig.go           # HTTP signatures
│   ├── inbox.go             # Inbox processing
│   ├── likes.go             # Like/unlike logic
│   ├── logger.go            # Federation logging
│   ├── outbox.go            # Outbox collections
│   └── posts.go             # Create/reply logic
├── db/
│   ├── db.go                # EXTENDED - New queries
│   └── migrations.go        # NEW - Schema migrations
├── domain/
│   ├── accounts.go          # EXTENDED
│   ├── activitypub.go       # NEW - AP models
│   └── notes.go             # EXTENDED
├── middleware/
│   ├── auth.go
│   ├── maintui.go
│   └── ratelimit.go         # NEW
├── ui/
│   ├── common/
│   │   └── commands.go      # EXTENDED - New states
│   ├── federatedtimeline/   # NEW
│   ├── followmanagement/    # NEW
│   ├── localtimeline/       # RENAMED from listnotes
│   ├── notifications/       # NEW
│   ├── supertui.go          # EXTENDED
│   └── writenote/           # EXTENDED
├── web/
│   ├── actor.go
│   ├── inbox.go             # IMPLEMENT (empty now)
│   ├── router.go            # EXTEND
│   ├── rss.go
│   └── webfinger.go         # EXTEND
├── config.yaml              # EXTENDED
├── main.go                  # EXTEND - Start worker
└── development-plan.md      # THIS FILE
```

---

## Implementation Checklist

### Phase 1: Foundation ✓
- [ ] Add go-ap/activitypub dependency
- [ ] Add superseriousbusiness/httpsig dependency
- [ ] Create db/migrations.go with all new tables
- [ ] Run migrations on startup
- [ ] Create domain/activitypub.go
- [ ] Extend domain/accounts.go and domain/notes.go
- [ ] Add database query functions

### Phase 2: HTTP Signatures ✓
- [ ] Implement activitypub/httpsig.go
- [ ] Implement activitypub/actors.go
- [ ] Add WebFinger client
- [ ] Test signature verification

### Phase 3: Core Protocol ✓
- [ ] Implement activitypub/outbox.go
- [ ] Replace stub outbox endpoint
- [ ] Implement activitypub/inbox.go
- [ ] Replace stub inbox endpoints
- [ ] Implement activitypub/delivery.go
- [ ] Start delivery worker in main.go
- [ ] Implement activitypub/follows.go
- [ ] Replace stub collections endpoints
- [ ] Test follow/accept workflow

### Phase 4: TUI ✓
- [ ] Add new SessionStates
- [ ] Implement keyboard navigation
- [ ] Create ui/federatedtimeline/
- [ ] Rename ui/localtimeline/
- [ ] Create ui/notifications/
- [ ] Create ui/followmanagement/
- [ ] Update ui/writenote/
- [ ] Update ui/supertui.go
- [ ] Test TUI navigation

### Phase 5: Interactions ✓
- [ ] Implement activitypub/likes.go
- [ ] Add like database queries
- [ ] Add like UI controls
- [ ] Implement reply creation
- [ ] Add reply UI
- [ ] Show counts in timeline
- [ ] Test with Mastodon users

### Phase 6: Production ✓
- [ ] Add structured logging
- [ ] Implement rate limiting
- [ ] Add database indices
- [ ] Optimize connection pooling
- [ ] Update config.yaml
- [ ] Write unit tests
- [ ] Test with live federation
- [ ] Update documentation

---

## Technical Decisions

### Why Custom Implementation?
- **go-fed/activity**: Abandoned, no longer maintained
- **superseriousbusiness/activity**: Tied to GoToSocial's needs
- **go-ap**: Very modular but complex
- **Custom**: Use go-ap types + httpsig library + custom logic = full control

### HTTP Signatures
- Required for ActivityPub security
- Using proven `superseriousbusiness/httpsig` library
- Follows draft-cavage spec like Mastodon

### Database Choice
- SQLite perfect for TUI use case
- Will handle thousands of users
- Need connection pooling
- Indices critical for timeline queries

### Navigation Philosophy
- Tab keeps existing muscle memory
- Number keys for quick access
- Escape returns to default
- Ctrl+R refreshes (familiar)

---

## Estimated Timeline

- **Phase 1**: 2 weeks (database foundation)
- **Phase 2**: 1 week (HTTP signatures)
- **Phase 3**: 2 weeks (core protocol)
- **Phase 4**: 1 week (TUI views)
- **Phase 5**: 1 week (interactions)
- **Phase 6**: 1 week (polish)

**Total: 8 weeks** for production-ready federation

---

## Testing Strategy

### Manual Testing Checklist
- [ ] Can follow Mastodon user from stegodon
- [ ] Mastodon user can follow stegodon user
- [ ] Stegodon post appears in Mastodon timeline
- [ ] Mastodon post appears in stegodon timeline
- [ ] Can like Mastodon post
- [ ] Can reply to Mastodon post
- [ ] Mastodon reply appears in notifications
- [ ] Can unfollow Mastodon user
- [ ] Delivery retries work
- [ ] WebFinger resolves stegodon users

---

## Future Enhancements (Post-MVP)

### Phase 7+: Advanced Features
- Media attachments (images, videos)
- Hashtag support
- Trending posts
- Local-only posts
- Followers-only visibility
- Account migration
- Post editing/deletion
- Content warnings
- Emoji reactions
- Polls
- Blocking/muting
- Custom emojis
- Markdown rendering
- OAuth2 for apps

### Phase 8: Scale
- Redis cache
- PostgreSQL option
- Separate delivery workers
- Metrics/monitoring
- Admin dashboard

---

## Success Metrics

### MVP Success Criteria
- [ ] Follow 3+ different Mastodon instances
- [ ] 100% local post federation
- [ ] 95%+ incoming activity processing
- [ ] Timeline loads <1s for 100 follows
- [ ] Zero crashes in 24h test
- [ ] Handle 1000 users, 10k posts

---

## Dependencies to Add

```bash
go get github.com/go-ap/activitypub@latest
go get codeberg.org/superseriousbusiness/httpsig@latest
```

Already have:
- github.com/gin-gonic/gin v1.11.0
- modernc.org/sqlite v1.40.0
- github.com/google/uuid v1.6.0
- github.com/charmbracelet/bubbletea v1.3.10
- github.com/charmbracelet/bubbles v0.21.0

---

**END OF DEVELOPMENT PLAN**

This plan provides a clear, phased approach to implementing full ActivityPub federation in stegodon while maintaining its unique SSH TUI interface.
