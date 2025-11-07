package common

type SessionState uint

const (
	CreateNoteView SessionState = iota
	ListNotesView
	CreateUserView
	UpdateNoteList
	FollowUserView      // New: Follow remote users
	FollowersView       // New: View followers/following
	FederatedTimelineView // New: View federated posts
)
