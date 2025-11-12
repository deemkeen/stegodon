package common

import (
	"time"

	"github.com/google/uuid"
)

type SessionState uint

const (
	CreateNoteView SessionState = iota
	ListNotesView
	CreateUserView
	UpdateNoteList
	FollowUserView         // Follow remote users
	FollowersView          // View who follows you
	FollowingView          // View who you're following
	FederatedTimelineView  // View federated posts
	LocalTimelineView      // View local posts from all local users
	LocalUsersView         // Browse and follow local users
)

// EditNoteMsg is sent when user wants to edit an existing note
type EditNoteMsg struct {
	NoteId    uuid.UUID
	Message   string
	CreatedAt time.Time
}

// DeleteNoteMsg is sent when user confirms note deletion
type DeleteNoteMsg struct {
	NoteId uuid.UUID
}
