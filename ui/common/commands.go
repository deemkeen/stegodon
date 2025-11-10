package common

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
