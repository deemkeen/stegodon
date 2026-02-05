package cli

import (
	"fmt"
	"time"

	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// handleBoost toggles a boost on a post
func (h *Handler) handleBoost(args []string) error {
	if len(args) == 0 {
		err := fmt.Errorf("usage: boost <post-id>")
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

	// Check if we already boosted this post
	var hasBoost bool
	if isLocal {
		hasBoost, err = h.db.HasBoost(h.account.Id, noteID)
	} else {
		hasBoost, err = h.db.HasBoostByObjectURI(h.account.Id, objectURI)
	}
	if err != nil {
		err = fmt.Errorf("failed to check boost status: %v", err)
		h.output.Error(err)
		return err
	}

	if hasBoost {
		// Unboost - remove the boost
		var existingBoost *domain.Boost
		if isLocal {
			_, existingBoost = h.db.ReadBoostByAccountAndNote(h.account.Id, noteID)
		} else {
			_, existingBoost = h.db.ReadBoostByAccountAndObjectURI(h.account.Id, objectURI)
		}

		// Delete the boost
		if isLocal {
			if err := h.db.DeleteBoostByAccountAndNote(h.account.Id, noteID); err != nil {
				err = fmt.Errorf("failed to unboost: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.DecrementBoostCountByNoteId(noteID); err != nil {
				// Log but don't fail
			}
		} else {
			if err := h.db.DeleteBoostByAccountAndObjectURI(h.account.Id, objectURI); err != nil {
				err = fmt.Errorf("failed to unboost: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.DecrementBoostCountByObjectURI(objectURI); err != nil {
				// Log but don't fail
			}
		}

		// Send Undo Announce to remote server (background)
		if objectURI != "" && existingBoost != nil {
			go func() {
				conf, confErr := util.ReadConf()
				if confErr != nil || !conf.Conf.WithAp {
					return
				}
				activitypub.SendUndoAnnounce(h.account, objectURI, existingBoost.URI, conf)
			}()
		}

		// Output response
		if h.output.IsJSON() {
			h.output.JSON(BoostResponse{
				PostID:  postID.String(),
				Boosted: false,
				Message: "Post unboosted",
			})
		} else {
			h.output.Print("Unboosted post %s\n", postID.String())
		}
	} else {
		// Boost - create a new boost
		boostURI := ""
		conf, confErr := util.ReadConf()
		if confErr == nil && conf.Conf.WithAp {
			boostURI = fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
		}

		boost := &domain.Boost{
			Id:        uuid.New(),
			AccountId: h.account.Id,
			NoteId:    noteID, // Will be uuid.Nil for remote posts
			ObjectURI: objectURI,
			URI:       boostURI,
			CreatedAt: time.Now(),
		}

		// Create the boost
		if isLocal {
			if err := h.db.CreateBoost(boost); err != nil {
				err = fmt.Errorf("failed to boost: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.IncrementBoostCountByNoteId(noteID); err != nil {
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
						NotificationType: domain.NotificationBoost,
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
			if err := h.db.CreateBoostByObjectURI(boost, objectURI); err != nil {
				err = fmt.Errorf("failed to boost: %v", err)
				h.output.Error(err)
				return err
			}
			if err := h.db.IncrementBoostCountByObjectURI(objectURI); err != nil {
				// Log but don't fail
			}
		}

		// Send Announce to remote server (background)
		if objectURI != "" {
			go func() {
				conf, confErr := util.ReadConf()
				if confErr != nil || !conf.Conf.WithAp {
					return
				}
				activitypub.SendAnnounce(h.account, objectURI, boostURI, conf)
			}()
		}

		// Output response
		if h.output.IsJSON() {
			h.output.JSON(BoostResponse{
				PostID:  postID.String(),
				Boosted: true,
				Message: "Post boosted",
			})
		} else {
			h.output.Print("Boosted post %s\n", postID.String())
		}
	}

	return nil
}
