package cli

import (
	"fmt"
	"time"

	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// handleLike toggles a like on a post
func (h *Handler) handleLike(args []string) error {
	if len(args) == 0 {
		err := fmt.Errorf("usage: like <post-id>")
		h.output.Error(err)
		return err
	}

	// Parse post ID
	postID, err := uuid.Parse(args[0])
	if err != nil {
		err = fmt.Errorf("invalid post ID: %s", args[0])
		h.output.Error(err)
		return err
	}

	// Try to find the post - first check if it's a local note
	var isLocal bool
	var objectURI string
	var noteID uuid.UUID

	dbErr, note := h.db.ReadNoteId(postID)
	if dbErr == nil && note != nil {
		// It's a local note
		isLocal = true
		objectURI = note.ObjectURI
		noteID = note.Id
	} else {
		// Try to find it as a remote activity
		dbErr, activity := h.db.ReadActivityById(postID)
		if dbErr != nil || activity == nil {
			err = fmt.Errorf("post not found: %s", args[0])
			h.output.Error(err)
			return err
		}
		// It's a remote activity
		isLocal = false
		objectURI = activity.ObjectURI
	}

	// Check if we already liked this post
	var hasLike bool
	if isLocal {
		hasLike, err = h.db.HasLike(h.account.Id, noteID)
	} else {
		hasLike, err = h.db.HasLikeByObjectURI(h.account.Id, objectURI)
	}
	if err != nil {
		err = fmt.Errorf("failed to check like status: %v", err)
		h.output.Error(err)
		return err
	}

	if hasLike {
		// Unlike - remove the like
		var existingLike *domain.Like
		if isLocal {
			_, existingLike = h.db.ReadLikeByAccountAndNote(h.account.Id, noteID)
		} else {
			_, existingLike = h.db.ReadLikeByAccountAndObjectURI(h.account.Id, objectURI)
		}

		// Delete the like
		if isLocal {
			if err := h.db.DeleteLikeByAccountAndNote(h.account.Id, noteID); err != nil {
				err = fmt.Errorf("failed to unlike: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.DecrementLikeCountByNoteId(noteID); err != nil {
				// Log but don't fail
			}
		} else {
			if err := h.db.DeleteLikeByAccountAndObjectURI(h.account.Id, objectURI); err != nil {
				err = fmt.Errorf("failed to unlike: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.DecrementLikeCountByObjectURI(objectURI); err != nil {
				// Log but don't fail
			}
		}

		// Send Undo Like to remote server (background)
		if objectURI != "" && existingLike != nil {
			go func() {
				conf, confErr := util.ReadConf()
				if confErr != nil || !conf.Conf.WithAp {
					return
				}
				activitypub.SendUndoLike(h.account, objectURI, existingLike.URI, conf)
			}()
		}

		// Output response
		if h.output.IsJSON() {
			h.output.JSON(LikeResponse{
				PostID:  postID.String(),
				Liked:   false,
				Message: "Post unliked",
			})
		} else {
			h.output.Print("Unliked post %s\n", postID.String())
		}
	} else {
		// Like - create a new like
		likeURI := ""
		conf, confErr := util.ReadConf()
		if confErr == nil && conf.Conf.WithAp {
			likeURI = fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
		}

		like := &domain.Like{
			Id:        uuid.New(),
			AccountId: h.account.Id,
			NoteId:    noteID, // Will be uuid.Nil for remote posts
			URI:       likeURI,
			CreatedAt: time.Now(),
		}

		// Create the like
		if isLocal {
			if err := h.db.CreateLike(like); err != nil {
				err = fmt.Errorf("failed to like: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.IncrementLikeCountByNoteId(noteID); err != nil {
				// Log but don't fail
			}

			// Create notification for local note author
			dbErr, note := h.db.ReadNoteId(noteID)
			if dbErr == nil && note != nil {
				dbErr, noteAuthor := h.db.ReadAccByUsername(note.CreatedBy)
				if dbErr == nil && noteAuthor != nil && noteAuthor.Id != h.account.Id {
					preview := note.Message
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
					notification := &domain.Notification{
						Id:               uuid.New(),
						AccountId:        noteAuthor.Id,
						NotificationType: domain.NotificationLike,
						ActorId:          h.account.Id,
						ActorUsername:    h.account.Username,
						ActorDomain:      "",
						NoteId:           note.Id,
						NoteURI:          note.ObjectURI,
						NotePreview:      preview,
						Read:             false,
						CreatedAt:        time.Now(),
					}
					h.db.CreateNotification(notification)
				}
			}
		} else {
			if err := h.db.CreateLikeByObjectURI(like, objectURI); err != nil {
				err = fmt.Errorf("failed to like: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.IncrementLikeCountByObjectURI(objectURI); err != nil {
				// Log but don't fail
			}
		}

		// Send Like to remote server (background)
		if objectURI != "" {
			go func() {
				conf, confErr := util.ReadConf()
				if confErr != nil || !conf.Conf.WithAp {
					return
				}
				activitypub.SendLike(h.account, objectURI, conf)
			}()
		}

		// Output response
		if h.output.IsJSON() {
			h.output.JSON(LikeResponse{
				PostID:  postID.String(),
				Liked:   true,
				Message: "Post liked",
			})
		} else {
			h.output.Print("Liked post %s\n", postID.String())
		}
	}

	return nil
}
