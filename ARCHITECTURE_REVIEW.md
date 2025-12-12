# Architecture Review

## Overview
- **Product shape:** Stegodon is an SSH-first federated blogging platform that exposes terminal, web, RSS, and ActivityPub interfaces built on Go and Charmbracelet tooling.【F:README.md†L5-L24】
- **Entry point:** `main.go` wires configuration, logging, migrations, ActivityPub delivery worker, and starts both the SSH server (Wish) and the Gin web server concurrently without explicit lifecycle orchestration beyond OS signals.【F:main.go†L26-L138】

## Component breakdown
- **User interfaces:** The SSH/TUI shell is composed from Bubble Tea models in `ui/`, providing views for timelines, posting, follows, admin, and relay management. The main TUI model aggregates many submodels and hits the database directly for mutations.【F:ui/supertui.go†L9-L77】
- **Web and federation layer:** `web/router.go` configures Gin with gzip, embedded static assets/templates, rate limits, RSS endpoints, and ActivityPub routes (actors, inbox/outbox objects). Handlers are defined inline inside the router setup and share the application config and database access through package calls.【F:web/router.go†L32-L200】
- **Data layer:** The `db` package wraps a singleton SQLite handle and exposes user, note, follow, and relay persistence as package functions. Schema definitions live alongside operational queries and are invoked directly by higher layers during migrations and runtime operations.【F:db/db.go†L20-L123】

## Architecture assessment
- **Strengths:** Clear package-oriented structure (ui, web, activitypub, db, domain) keeps related concerns together; embedded assets and configuration defaults simplify deployment; entrypoint logs configuration and runs migrations automatically.
- **Risks/limitations:** Application startup logic, migrations, SSH server, and web server are tightly coupled in `main.go`, limiting composability and testability. Web routing and handler logic are intertwined, reducing opportunities for reuse and structured error handling. UI and activity code depend on the global database singleton, making alternate storage or offline testing difficult.

## Top 3 improvement opportunities
1. **Introduce an application lifecycle/service layer.** Encapsulate config loading, migration execution, worker startup, and SSH/HTTP server creation behind an application struct with explicit `Start`/`Shutdown` methods. This would isolate side effects from `main.go` and enable integration tests that exercise startup without invoking `os.Exit` or global state.【F:main.go†L26-L138】
2. **Refactor web routing into modular handlers with graceful shutdown.** Move inline Gin handlers into dedicated packages (e.g., `web/handlers`) that accept dependencies via interfaces, and initialize an `http.Server` so the web stack can share the same shutdown path as SSH. This would improve readability and allow middleware, validation, and error responses to be standardized across ActivityPub and UI endpoints.【F:web/router.go†L32-L200】
3. **Define data access interfaces and inject them into UI/federation layers.** Replace direct calls to the global DB singleton with interfaces (e.g., `AccountStore`, `NoteStore`) passed into TUI models and ActivityPub functions. This decoupling would make it feasible to mock storage in tests, swap persistence backends, or run in-memory modes without touching UI code.【F:ui/supertui.go†L67-L77】【F:db/db.go†L20-L123】
