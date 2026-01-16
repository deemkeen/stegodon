package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationFollow  NotificationType = "follow"
	NotificationLike    NotificationType = "like"
	NotificationBoost   NotificationType = "boost"
	NotificationReply   NotificationType = "reply"
	NotificationMention NotificationType = "mention"
)

// Notification represents a user notification
type Notification struct {
	Id               uuid.UUID
	AccountId        uuid.UUID        // The local user receiving the notification
	NotificationType NotificationType // follow, like, reply, mention
	ActorId          uuid.UUID        // The account that triggered the notification (local or remote)
	ActorUsername    string           // Denormalized for display (e.g., "alice")
	ActorDomain      string           // Denormalized for display (e.g., "mastodon.social", empty for local)
	NoteId           uuid.UUID        // Reference to the note (for like/reply/mention)
	NoteURI          string           // ActivityPub URI of the note
	NotePreview      string           // First 100 chars of note content
	Read             bool             // Whether the notification has been read
	CreatedAt        time.Time
}

// ActorHandle returns the formatted @user or @user@domain string
func (n *Notification) ActorHandle() string {
	if n.ActorDomain == "" {
		return "@" + n.ActorUsername
	}
	return "@" + n.ActorUsername + "@" + n.ActorDomain
}

// TypeLabel returns a human-readable label for the notification type
func (n *Notification) TypeLabel() string {
	switch n.NotificationType {
	case NotificationFollow:
		return "followed you"
	case NotificationLike:
		return "liked your post"
	case NotificationBoost:
		return "boosted your post"
	case NotificationReply:
		return "replied to your post"
	case NotificationMention:
		return "mentioned you"
	default:
		return ""
	}
}

// TypeIcon returns an emoji icon for the notification type
func (n *Notification) TypeIcon() string {
	switch n.NotificationType {
	case NotificationFollow:
		return "👤"
	case NotificationLike:
		return "❤️"
	case NotificationBoost:
		return "🔁"
	case NotificationReply:
		return "💬"
	case NotificationMention:
		return "@"
	default:
		return "•"
	}
}

// Summary returns a one-line summary of the notification
func (n *Notification) Summary() string {
	return fmt.Sprintf("%s %s %s", n.TypeIcon(), n.ActorHandle(), n.TypeLabel())
}
