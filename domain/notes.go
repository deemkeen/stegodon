package domain

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type SaveNote struct {
	UserId       uuid.UUID
	Message      string
	InReplyToURI string // URI of parent post (empty for top-level posts)
}

// GlobalTimelinePost represents a post in the global timeline (local + federated)
type GlobalTimelinePost struct {
	NoteId           string
	Username         string
	UserDomain       string
	ProfileURL       string
	ObjectURI        string // ActivityPub object id (canonical URI, for replies/likes)
	ObjectURL        string // ActivityPub object url (human-readable web UI link, preferred for display)
	IsRemote         bool
	Message          string
	ProcessedContent string // Pre-processed content for terminal display (cached to avoid re-processing in View)
	CreatedAt        time.Time
	ReplyCount       int
	LikeCount        int
	BoostCount       int
	BoostedBy        string // if non-empty, this post was boosted by this user (e.g., "@alice" or "@bob@domain")
}

type Note struct {
	Id        uuid.UUID
	CreatedBy string
	Message   string
	CreatedAt time.Time
	EditedAt  *time.Time // When the note was last edited (nil if never edited)
	// ActivityPub fields
	Visibility     string // "public", "unlisted", "followers", "direct"
	InReplyToURI   string // URI of the note this is replying to
	ObjectURI      string // ActivityPub object URI
	Federated      bool   // Whether to federate this note
	Sensitive      bool   // Contains sensitive content
	ContentWarning string // Content warning text
	// Engagement counters
	ReplyCount int // Number of replies
	LikeCount  int // Number of likes
	BoostCount int // Number of boosts
}

func (note *Note) ToString() string {
	return fmt.Sprintf("\n\tId: %s \n\tCreatedBy: %s \n\tMessage: %s \n\tCreatedAt: %s)", note.Id, note.CreatedBy, note.Message, note.CreatedAt)
}

// HomePost represents a unified post in the home timeline (either local or remote)
type HomePost struct {
	ID               uuid.UUID
	Author           string // @user (local) or @user@domain (remote)
	Content          string
	ProcessedContent string // Pre-processed content for terminal display (cached to avoid re-processing in View)
	Time             time.Time
	ObjectURI        string    // ActivityPub object id (canonical URI, returns JSON)
	ObjectURL        string    // ActivityPub object url (human-readable web UI link, preferred for display)
	IsLocal          bool      // true = local note, false = remote activity
	NoteID           uuid.UUID // only set for local posts (for editing/deleting)
	ReplyCount       int       // number of replies to this post
	LikeCount        int       // number of likes on this post
	BoostCount       int       // number of boosts on this post
	BoostedBy        string    // if non-empty, this post was boosted by this user (e.g., "@alice" or "@bob@domain")
}

// SearchResult represents a post found via full-text search
type SearchResult struct {
	ID         uuid.UUID
	Author     string    // @user or @user@domain
	Snippet    string    // FTS5 snippet with highlight markers (<<…>>)
	Time       time.Time
	ObjectURI  string
	ObjectURL  string
	IsLocal    bool
	NoteID     uuid.UUID // Only set for local notes
	SourceID   string    // ID in posts_fts (notes.id or activities.id)
	SourceType string    // "note" or "activity"
}
