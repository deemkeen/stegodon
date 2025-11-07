package common

type SessionState uint

const (
	CreateNoteView SessionState = iota
	ListNotesView
	CreateUserView
	UpdateNoteList
	FollowUserView         // Follow remote users
	FollowersView          // View followers/following
	FederatedTimelineView  // View federated posts
	LocalTimelineView      // View local posts from all local users
	LocalUsersView         // Browse and follow local users
)
