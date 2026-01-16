package cli

import (
	"github.com/deemkeen/stegodon/util"
)

const defaultNotificationsLimit = 20

// handleNotifications shows unread notifications
func (h *Handler) handleNotifications(args []string) error {
	// Get unread count
	unreadCount, err := h.db.CountUnreadNotifications(h.account.Id)
	if err != nil {
		h.output.Error(err)
		return err
	}

	// Read notifications
	err, notifications := h.db.ReadNotificationsByAccountId(h.account.Id, defaultNotificationsLimit)
	if err != nil {
		h.output.Error(err)
		return err
	}

	if notifications == nil || len(*notifications) == 0 {
		if h.output.IsJSON() {
			h.output.JSON(NotificationsResponse{
				Notifications: []NotificationItem{},
				UnreadCount:   0,
			})
		} else {
			h.output.Println("No notifications.")
		}
		return nil
	}

	// Output response
	if h.output.IsJSON() {
		items := make([]NotificationItem, 0, len(*notifications))
		for _, n := range *notifications {
			// Strip HTML tags from preview
			preview := ""
			if n.NotePreview != "" {
				preview = util.StripHTMLTags(n.NotePreview)
			}

			items = append(items, NotificationItem{
				ID:          n.Id.String(),
				Type:        string(n.NotificationType),
				Actor:       n.ActorHandle(),
				NotePreview: preview,
				CreatedAt:   n.CreatedAt,
			})
		}

		h.output.JSON(NotificationsResponse{
			Notifications: items,
			UnreadCount:   unreadCount,
		})
	} else {
		// Text output
		for _, n := range *notifications {
			h.output.Print("%s (%s)\n", n.Summary(), FormatTimeAgo(n.CreatedAt))
			if n.NotePreview != "" {
				preview := util.StripHTMLTags(n.NotePreview)
				h.output.Print("  %s\n", preview)
			}
			h.output.Println("")
		}

		if unreadCount > 0 {
			h.output.Print("(%d unread)\n", unreadCount)
		}
	}

	return nil
}
