package common

import (
	"time"

	"github.com/google/uuid"
)

type SessionState uint

const (
	CreateNoteView   SessionState = iota
	HomeTimelineView              // Unified home timeline (local + remote)
	MyPostsView                   // View only your own posts
	GlobalPostsView               // Global timeline (all local + all federated posts)
	CreateUserView
	UpdateNoteList
	FollowUserView        // Follow remote users
	FollowersView         // View who follows you
	FollowingView         // View who you're following
	LocalUsersView        // Browse and follow local users
	AdminPanelView        // Admin panel for user management (admin only)
	RelayManagementView   // Admin panel for relay management (admin only)
	AccountSettingsView   // Account settings (profile, avatar, delete)
	ThreadView            // View thread with parent and replies
	NotificationsView     // View notifications
	ProfileView           // View user profile with recent posts
	TermsAcceptanceView   // Terms acceptance for existing users
)

const (
	// DeleteAccountView is deprecated, use AccountSettingsView
	DeleteAccountView = AccountSettingsView
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

// ActivateViewMsg is sent when a view becomes active (visible)
type ActivateViewMsg struct{}

// DeactivateViewMsg is sent when a view becomes inactive (hidden)
type DeactivateViewMsg struct{}

// ActivateAccountSettingsMsg is sent specifically to accountsettings when it becomes visible
// This avoids conflicts with the generic ActivateViewMsg used by timeline views
type ActivateAccountSettingsMsg struct{}

// DeactivateAccountSettingsMsg is sent specifically to accountsettings when it becomes hidden
type DeactivateAccountSettingsMsg struct{}

// ReplyToNoteMsg is sent when user presses 'r' to reply to a post
type ReplyToNoteMsg struct {
	NoteURI string // ActivityPub object URI of the note being replied to
	Author  string // Display name or handle of the author
	Preview string // Preview of the note content (first line or truncated)
}

// ViewThreadMsg is sent when user presses Enter to view a thread
type ViewThreadMsg struct {
	NoteURI   string    // ActivityPub object URI of the note
	NoteID    uuid.UUID // Local UUID (if local note)
	IsLocal   bool      // Whether this is a local note
	Author    string    // Author name for display
	Content   string    // Full content
	CreatedAt time.Time // Timestamp
}

// LikeNoteMsg is sent when user presses 'l' to like/unlike a post
type LikeNoteMsg struct {
	NoteURI string    // ActivityPub object URI of the note being liked
	NoteID  uuid.UUID // Local UUID (if local note)
	IsLocal bool      // Whether this is a local note
}

// ViewProfileMsg is sent when user presses Enter on a local user to view their profile
type ViewProfileMsg struct {
	Username  string
	AccountId uuid.UUID
}

// BoostNoteMsg is sent when user presses 'b' to boost/unboost a post
type BoostNoteMsg struct {
	NoteURI string    // ActivityPub object URI of the note being boosted
	NoteID  uuid.UUID // Local UUID (if local note)
	IsLocal bool      // Whether this is a local note
}
