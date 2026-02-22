# Stegodon Specifications Overview

This document provides a lookup table for all specification documents in the stegodon project.

---

## Core Architecture

| Topic | Description | Spec File |
|-------|-------------|-----------|
| Application Lifecycle | App initialization, dual-server model, graceful shutdown | [app-lifecycle.md](./app-lifecycle.md) |
| Configuration System | Environment variables, YAML config, defaults | [configuration.md](./configuration.md) |
| Dual Server Model | SSH server (TUI) and HTTP server (web/API) architecture | [dual-server.md](./dual-server.md) |

---

## Domain Models

| Topic | Description | Spec File |
|-------|-------------|-----------|
| Account Entity | User accounts, SSH keys, RSA keypairs, profile fields | [domain/account.md](./domain/account.md) |
| Note Entity | Posts, threading, engagement counters, visibility | [domain/note.md](./domain/note.md) |
| Remote Account | Cached federated user profiles, 24h TTL | [domain/remote-account.md](./domain/remote-account.md) |
| Follow Relationship | Local and remote follows, accepted status | [domain/follow.md](./domain/follow.md) |
| Engagement (Like/Boost) | Like and boost entities with activity URIs | [domain/engagement.md](./domain/engagement.md) |
| Notification | Types (follow, like, reply, mention), read status | [domain/notification.md](./domain/notification.md) |
| Relay | ActivityPub relay subscriptions, pause/resume | [domain/relay.md](./domain/relay.md) |

---

## User Interface (TUI)

| Topic | Description | Spec File |
|-------|-------------|-----------|
| TUI Architecture | BubbleTea MVC pattern, session state, view switching | [ui/architecture.md](./ui/architecture.md) |
| CreateUser View | First-time username selection flow | [ui/createuser.md](./ui/createuser.md) |
| WriteNote View | Note composition, @mention autocomplete, reply mode | [ui/writenote.md](./ui/writenote.md) |
| MyPosts View | User's own posts with edit/delete capabilities | [ui/myposts.md](./ui/myposts.md) |
| HomeTimeline View | Combined local + federated feed with auto-refresh | [ui/hometimeline.md](./ui/hometimeline.md) |
| GlobalPosts View | Global timeline (all local + federated posts) | [ui/globalposts.md](./ui/globalposts.md) |
| ThreadView | Conversation/reply thread display | [ui/threadview.md](./ui/threadview.md) |
| ProfileView | User profile with avatar and recent posts (local and remote) | [ui/profileview.md](./ui/profileview.md) |
| FollowUser View | WebFinger-based remote user follow | [ui/followuser.md](./ui/followuser.md) |
| Followers/Following Views | Paginated relationship lists | [ui/relationships.md](./ui/relationships.md) |
| LocalUsers View | Browse and follow local users | [ui/localusers.md](./ui/localusers.md) |
| Notifications View | Notification center with unread badges | [ui/notifications.md](./ui/notifications.md) |
| Relay Management | Admin relay control (add, pause, delete) | [ui/relay.md](./ui/relay.md) |
| Admin Panel | User management (mute, ban), server message, terms | [ui/admin.md](./ui/admin.md) |
| AccountSettings View | Profile editing, avatar upload, account deletion | [ui/deleteaccount.md](./ui/deleteaccount.md) |
| Search Overlay | Full-text search across all posts (FTS5) | [features/search.md](./features/search.md) |
| Terms View | Terms of service acceptance screen | — |
| Common Components | Header, styles, layout, commands | [ui/common.md](./ui/common.md) |

---

## ActivityPub Federation

| Topic | Description | Spec File |
|-------|-------------|-----------|
| HTTP Signatures | RSA-SHA256 signing and verification | [activitypub/http-signatures.md](./activitypub/http-signatures.md) |
| Inbox Processing | Incoming activity handling, deduplication | [activitypub/inbox.md](./activitypub/inbox.md) |
| Outbox/Sending | Activity creation and delivery | [activitypub/outbox.md](./activitypub/outbox.md) |
| Actor Fetching | WebFinger resolution, profile caching | [activitypub/actors.md](./activitypub/actors.md) |
| Delivery Queue | Background worker, exponential backoff | [activitypub/delivery.md](./activitypub/delivery.md) |
| Supported Activities | Follow, Create, Update, Delete, Like, Announce, Undo | [activitypub/activities.md](./activitypub/activities.md) |
| Relay Support | FediBuzz and YUKIMOCHI relay integration | [activitypub/relays.md](./activitypub/relays.md) |

---

## Database Layer

| Topic | Description | Spec File |
|-------|-------------|-----------|
| Schema Overview | Tables, relationships, indexes | [database/schema.md](./database/schema.md) |
| Connection Management | SQLite WAL mode, connection pooling | [database/connection.md](./database/connection.md) |
| Migrations | Schema evolution, key format migrations | [database/migrations.md](./database/migrations.md) |
| Query Patterns | Common queries, denormalized counters | [database/queries.md](./database/queries.md) |

---

## Web Layer (HTTP)

| Topic | Description | Spec File |
|-------|-------------|-----------|
| HTTP Routes | All endpoints, rate limiting | [web/routes.md](./web/routes.md) |
| Web UI Handlers | Profile pages, post views, tag feeds | [web/handlers.md](./web/handlers.md) |
| ActivityPub Endpoints | Actor, inbox, outbox, followers, following | [web/activitypub-endpoints.md](./web/activitypub-endpoints.md) |
| WebFinger Protocol | User discovery via acct: URIs | [web/webfinger.md](./web/webfinger.md) |
| NodeInfo | Server metadata and statistics | [web/nodeinfo.md](./web/nodeinfo.md) |
| RSS Feeds | Feed generation per user | [web/rss.md](./web/rss.md) |
| Templates & Assets | Embedded HTML templates and static files | [web/templates.md](./web/templates.md) |

---

## SSH/Middleware

| Topic | Description | Spec File |
|-------|-------------|-----------|
| SSH Authentication | Public key auth, account lookup | [middleware/authentication.md](./middleware/authentication.md) |
| Registration Modes | Open, closed, single-user modes | [middleware/registration.md](./middleware/registration.md) |
| TUI Initialization | BubbleTea program setup per session | [middleware/tui-init.md](./middleware/tui-init.md) |

---

## Utilities

| Topic | Description | Spec File |
|-------|-------------|-----------|
| Cryptography | RSA keypair generation, key parsing | [util/cryptography.md](./util/cryptography.md) |
| Text Processing | Hashtag/mention parsing, URL linkification | [util/text-processing.md](./util/text-processing.md) |
| Validation | Username, domain, configuration validation | [util/validation.md](./util/validation.md) |

---

## Deployment & Operations

| Topic | Description | Spec File |
|-------|-------------|-----------|
| Docker Deployment | Container configuration, TrueColor rendering | [ops/docker.md](./ops/docker.md) |
| Single Binary | Embedded assets, distribution | [ops/distribution.md](./ops/distribution.md) |
| Logging | Standard logging, journald integration | [ops/logging.md](./ops/logging.md) |
| Monitoring | pprof profiling, NodeInfo statistics | [ops/monitoring.md](./ops/monitoring.md) |
| Security | Key hashing, HTTP signatures, rate limiting | [ops/security.md](./ops/security.md) |
| Terminal Requirements | Dimensions, color support | [ops/terminal.md](./ops/terminal.md) |

---

## Feature Behaviors

| Topic | Description | Spec File |
|-------|-------------|-----------|
| Note Limits | Character limits (150 visible, 1000 DB) | [features/note-limits.md](./features/note-limits.md) |
| Content Warnings | Sensitive content handling | [features/content-warnings.md](./features/content-warnings.md) |
| Visibility Settings | Public, unlisted, followers, direct | [features/visibility.md](./features/visibility.md) |
| Mention Autocomplete | @user suggestions while composing | [features/autocomplete.md](./features/autocomplete.md) |
| Auto-Refresh | Timeline refresh patterns, goroutine lifecycle | [features/auto-refresh.md](./features/auto-refresh.md) |
| Thread Navigation | Reply chains, parent-child relationships | [features/threading.md](./features/threading.md) |
| Full-Text Search | FTS5 index, search overlay, query sanitization | [features/search.md](./features/search.md) |
