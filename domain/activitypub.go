package domain

import (
	"github.com/google/uuid"
	"time"
)

// RemoteAccount represents a cached federated user
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

// Follow represents a follow relationship
type Follow struct {
	Id              uuid.UUID
	AccountId       uuid.UUID // Can be local or remote account
	TargetAccountId uuid.UUID // Can be local or remote account
	URI             string    // ActivityPub Follow activity URI (empty for local follows)
	CreatedAt       time.Time
	Accepted        bool
	IsLocal         bool // true if this is a local-only follow
}

// Like represents a like/favorite on a note
type Like struct {
	Id        uuid.UUID
	AccountId uuid.UUID // Who liked (can be local or remote)
	NoteId    uuid.UUID // Which note was liked
	URI       string    // ActivityPub Like activity URI
	CreatedAt time.Time
}

// Activity represents an ActivityPub activity (for logging/deduplication)
type Activity struct {
	Id           uuid.UUID
	ActivityURI  string
	ActivityType string // Follow, Create, Like, Announce, Undo, etc.
	ActorURI     string
	ObjectURI    string
	RawJSON      string
	Processed    bool
	CreatedAt    time.Time
	Local        bool // true if originated from this server
}

// DeliveryQueueItem represents an item in the delivery queue
type DeliveryQueueItem struct {
	Id           uuid.UUID
	InboxURI     string
	ActivityJSON string // The complete activity to deliver
	Attempts     int
	NextRetryAt  time.Time
	CreatedAt    time.Time
}
