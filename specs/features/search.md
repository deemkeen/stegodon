# Full-Text Search

Stegodon provides full-text search across all posts (local notes and federated activities) using SQLite FTS5. Search is available as a TUI overlay activated from timeline views.

---

## Architecture

### FTS5 Index

The search index uses a **slim standalone FTS5 table** (`posts_fts`) that only stores the two searchable columns:

- `content` — Post text
- `author` — Author handle (`@user` or `@user@domain`)

All other metadata is stored separately:
- `source_type` and `created_at` in the `posts_fts_lookup` table
- `object_uri` and `object_url` loaded via LEFT JOIN from `notes`/`activities` at query time

This design minimizes memory usage — FTS5 UNINDEXED columns still consume memory in FTS5's internal B-tree, so they were removed entirely.

### Lookup Table

`posts_fts_lookup` maps between source records and FTS5 rowids:

| Column | Description |
|--------|-------------|
| `source_id` | UUID from `notes.id` or `activities.id` (primary key) |
| `fts_rowid` | Integer rowid in `posts_fts` |
| `source_type` | `note` or `activity` |
| `created_at` | Timestamp for ordering |

The lookup table is needed because:
1. FTS5 uses integer rowids internally, not UUIDs
2. Delete operations require looking up the rowid first
3. Metadata (source_type, created_at) must be stored somewhere outside FTS5

---

## FTS Sync

The FTS index is updated **synchronously** (not in goroutines) on every mutation:

| Operation | FTS Action |
|-----------|-----------|
| `CreateNoteWithReply` | `InsertNoteFTS` — inserts content+author into FTS, stores metadata in lookup |
| `UpdateNote` | `UpdateNoteFTS` — delete old entry + insert new |
| `DeleteNoteById` | `DeleteFromFTS` — lookup rowid, delete from FTS + lookup |
| `CreateActivity` | `InsertActivityFTS` — extracts content from raw JSON, inserts |
| `UpdateActivity` | Delete + re-insert |
| `DeleteActivity` | `DeleteFromFTS` |
| `DeleteRelayActivities` | `DeleteRelayActivitiesFTS` — batch cleanup |

Synchronous sync avoids race conditions with in-memory test databases.

---

## Query Processing

### Input Sanitization

User input is sanitized in `sanitizeFTSQuery()` before being passed to FTS5 MATCH:

1. **Strip operators** — Remove FTS5 special syntax: `AND`, `OR`, `NOT`, `NEAR`, `*`, `"`, `(`, `)`, `^`, `:`
2. **Wrap tokens** — Each remaining token becomes a prefix query: `"token"*`
3. **Implicit AND** — Multiple tokens are ANDed: `hello world` → `"hello"* "world"*`

### Search Query

```sql
SELECT l.source_id, l.source_type, f.author,
       snippet(posts_fts, 0, '<<', '>>', '...', 32),
       l.created_at,
       CASE l.source_type WHEN 'note' THEN COALESCE(n.object_uri, '') ... END,
       CASE l.source_type WHEN 'activity' THEN COALESCE(a.object_url, '') ... END
FROM posts_fts f
JOIN posts_fts_lookup l ON l.fts_rowid = f.rowid
LEFT JOIN notes n ON l.source_type = 'note' AND l.source_id = n.id
LEFT JOIN activities a ON l.source_type = 'activity' AND l.source_id = a.id
WHERE posts_fts MATCH ?
ORDER BY rank, l.created_at DESC
LIMIT 20
```

- `rank` — FTS5 BM25 relevance score (lower = more relevant)
- `snippet()` — Generates highlighted excerpts with `<<`/`>>` markers
- LEFT JOINs on primary keys — at most 20 PK lookups (negligible performance impact)

---

## TUI Search Overlay

### Activation

- Press `/` from Home Timeline, My Posts, or Global Posts view
- Search is an **overlay**, not a separate view — it replaces the right panel
- Not available when the reply editor is active

### UI Components

- **Text input** at the top of the overlay (bubbles/textinput)
- **Result list** below with snippet preview and author
- Results display highlighted matches using `<<`/`>>` markers rendered as bold text

### Key Bindings

| Key | Action |
|-----|--------|
| `/` | Activate search (from timeline views) |
| Type | Input search query |
| Up/Down | Navigate results |
| Enter | Open thread for selected result |
| Esc | Close search overlay |
| Tab/Shift+Tab | Close search and switch view |

### Performance

- **200ms debounce** — Search query only fires after 200ms of no typing
- **Lazy rendering** — Search overlay View() only computed when `Active == true`
- **No tea.Batch** — Debounce returns only the tick cmd, not batched with textinput blink
- **Max 20 results** — Limits query cost

---

## Migration

`MigrateFTS5Search()` runs on startup:

1. **Schema detection** — Checks if the old 6-column FTS schema is present (via `pragma_table_info`)
2. **Auto-rebuild** — If old schema detected, drops and recreates tables with slim schema
3. **Backfill** — If FTS table is empty, backfills from `notes` and `activities`
4. **Idempotent** — Skips backfill if FTS already populated

---

## Domain Type

```go
type SearchResult struct {
    ID         uuid.UUID
    Author     string    // @user or @user@domain
    Snippet    string    // FTS5 snippet with <<…>> highlight markers
    Time       time.Time
    ObjectURI  string
    ObjectURL  string
    IsLocal    bool
    NoteID     uuid.UUID // Only set for local notes
    SourceID   string    // notes.id or activities.id
    SourceType string    // "note" or "activity"
}
```

---

## Source Files

- `ui/search/search.go` — Search overlay Model, Update, View
- `db/db.go` — `SearchPosts`, `InsertNoteFTS`, `InsertActivityFTS`, `DeleteFromFTS`, `UpdateNoteFTS`, `sanitizeFTSQuery`
- `db/migrations.go` — `MigrateFTS5Search`, backfill functions
- `domain/notes.go` — `SearchResult` struct
- `ui/supertui.go` — Search activation (`/` key), overlay integration
