package cli

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// parseReplyToFlag extracts --reply-to value and remaining args
func parseReplyToFlag(args []string) ([]string, string) {
	var replyTo string
	var filtered []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--reply-to" && i+1 < len(args) {
			replyTo = args[i+1]
			i++ // Skip the next arg (the value)
		} else {
			filtered = append(filtered, args[i])
		}
	}

	return filtered, replyTo
}

// handlePost creates a new note
func (h *Handler) handlePost(args []string) error {
	var message string
	var inReplyToURI string
	var parentNote *domain.Note // Store parent note for notification

	// Parse --reply-to flag
	args, replyToID := parseReplyToFlag(args)

	if len(args) == 0 {
		err := fmt.Errorf("usage: post <message> [--reply-to <id>]")
		h.output.Error(err)
		return err
	}

	if args[0] == "-" {
		// Read from stdin
		data, err := io.ReadAll(h.session)
		if err != nil {
			h.output.Error(err)
			return err
		}
		message = strings.TrimSpace(string(data))
	} else {
		message = strings.Join(args, " ")
	}

	// Validate message not empty
	if message == "" {
		err := fmt.Errorf("message cannot be empty")
		h.output.Error(err)
		return err
	}

	// Validate visible character count
	visibleChars := util.CountVisibleChars(message)
	maxChars := h.conf.Conf.MaxChars
	if visibleChars > maxChars {
		err := fmt.Errorf("message too long (%d chars, max %d)", visibleChars, maxChars)
		h.output.ErrorWithDetails("message too long", fmt.Sprintf("%d chars, max %d", visibleChars, maxChars))
		return err
	}

	// Validate total note length (including markdown syntax)
	if err := util.ValidateNoteLength(message); err != nil {
		h.output.Error(err)
		return err
	}

	// Resolve --reply-to if provided
	if replyToID != "" {
		postID, err := uuid.Parse(replyToID)
		if err != nil {
			err = fmt.Errorf("invalid reply-to ID: %s", replyToID)
			h.output.Error(err)
			return err
		}

		// Try to find the post - first check if it's a local note
		// Use ReadNoteIdWithReplyInfo to get ObjectURI
		dbErr, note := h.db.ReadNoteIdWithReplyInfo(postID)
		if dbErr == nil && note != nil {
			inReplyToURI = note.ObjectURI
			parentNote = note // Save for notification
		} else {
			// Try to find it as a remote activity
			dbErr, activity := h.db.ReadActivityById(postID)
			if dbErr != nil || activity == nil {
				err = fmt.Errorf("reply-to post not found: %s", replyToID)
				h.output.Error(err)
				return err
			}
			inReplyToURI = activity.ObjectURI
		}
	}

	// Create the note
	var noteId interface{}
	var err error
	if inReplyToURI != "" {
		noteId, err = h.db.CreateNoteWithReply(h.account.Id, message, inReplyToURI)
	} else {
		noteId, err = h.db.CreateNote(h.account.Id, message)
	}
	if err != nil {
		h.output.Error(err)
		return err
	}

	// Create reply notification if replying to a local note
	if parentNote != nil {
		// Get parent author
		dbErr, parentAuthor := h.db.ReadAccByUsername(parentNote.CreatedBy)
		if dbErr == nil && parentAuthor != nil && parentAuthor.Id != h.account.Id {
			// Only notify if replier is not the parent author
			preview := util.StripHTMLTags(message)
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}

			// Convert noteId to uuid.UUID for notification
			var noteUUID uuid.UUID
			switch id := noteId.(type) {
			case uuid.UUID:
				noteUUID = id
			}

			notification := &domain.Notification{
				Id:               uuid.New(),
				AccountId:        parentAuthor.Id,
				NotificationType: domain.NotificationReply,
				ActorId:          h.account.Id,
				ActorUsername:    h.account.Username,
				ActorDomain:      "", // Empty for local users
				NoteId:           noteUUID,
				NotePreview:      preview,
				Read:             false,
				CreatedAt:        time.Now(),
			}
			if err := h.db.CreateNotification(notification); err != nil {
				log.Printf("CLI: Failed to create reply notification: %v", err)
			}
		}
	}

	// Federate the note via ActivityPub (background task)
	go func() {
		// Only federate if ActivityPub is enabled
		if !h.conf.Conf.WithAp {
			return
		}

		// Get the created note from database
		err, createdNote := h.db.ReadNoteIdWithReplyInfo(noteId)
		if err != nil {
			log.Printf("CLI: Failed to read created note for federation: %v", err)
			return
		}

		// Send Create activity to all followers
		if err := activitypub.SendCreate(createdNote, h.account, h.conf); err != nil {
			log.Printf("CLI: Failed to federate note: %v", err)
		} else {
			log.Printf("CLI: Note federated successfully for %s", h.account.Username)
		}
	}()

	// Output response
	if h.output.IsJSON() {
		resp := PostResponse{
			ID:        fmt.Sprintf("%v", noteId),
			Message:   message,
			CreatedAt: time.Now(),
		}
		if inReplyToURI != "" {
			resp.InReplyTo = inReplyToURI
		}
		h.output.JSON(resp)
	} else {
		// Convert noteId to string for display
		var idStr string
		switch id := noteId.(type) {
		case uuid.UUID:
			idStr = id.String()
		default:
			idStr = fmt.Sprintf("%v", id)
		}
		if inReplyToURI != "" {
			h.output.Success("Reply posted: %s\n", idStr)
		} else {
			h.output.Success("Posted: %s\n", idStr)
		}
	}

	return nil
}
