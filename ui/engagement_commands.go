// engagement_commands.go — Like and boost command handlers for the TUI.
// Principle: Locality over Abstraction — like/boost logic is self-contained here
// rather than scattered across multiple packages. Each command handles the full
// lifecycle: local state update, counter management, notification creation,
// and ActivityPub federation.
package ui

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// likeNoteCmd handles liking/unliking a note
func likeNoteCmd(accountId uuid.UUID, noteURI string, noteID uuid.UUID, isLocal bool, account *domain.Account) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Determine the actual note ID and URI to use
		var actualNoteID uuid.UUID
		var actualNoteURI string
		var isRemotePost bool

		if isLocal && noteID != uuid.Nil {
			actualNoteID = noteID
			// Get the note's ObjectURI for federation
			err, note := database.ReadNoteId(noteID)
			if err != nil {
				log.Printf("Failed to read note for like: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		} else if noteURI != "" && !strings.HasPrefix(noteURI, "local:") {
			// Remote note - find it by ObjectURI
			err, activity := database.ReadActivityByObjectURI(noteURI)
			if err != nil || activity == nil {
				log.Printf("Failed to find activity for like: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = noteURI
			isRemotePost = true
			// Try to find a local note with this URI (federated back)
			err, localNote := database.ReadNoteByURI(noteURI)
			if err == nil && localNote != nil {
				actualNoteID = localNote.Id
				isRemotePost = false // It's actually a local post that was federated back
			}
		} else if strings.HasPrefix(noteURI, "local:") {
			// Parse local: prefix
			idStr := strings.TrimPrefix(noteURI, "local:")
			parsedID, err := uuid.Parse(idStr)
			if err != nil {
				log.Printf("Failed to parse local note ID: %v", err)
				return common.UpdateNoteList
			}
			actualNoteID = parsedID
			// Get the note's ObjectURI for federation
			err, note := database.ReadNoteId(parsedID)
			if err != nil {
				log.Printf("Failed to read note for like: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		}

		// Check if we already liked this post
		var hasLike bool
		var err error
		if isRemotePost {
			hasLike, err = database.HasLikeByObjectURI(accountId, actualNoteURI)
		} else {
			hasLike, err = database.HasLike(accountId, actualNoteID)
		}
		if err != nil {
			log.Printf("Failed to check existing like: %v", err)
			return common.UpdateNoteList
		}

		if hasLike {
			// Unlike - remove the like
			var existingLike *domain.Like
			if isRemotePost {
				err, existingLike = database.ReadLikeByAccountAndObjectURI(accountId, actualNoteURI)
			} else {
				err, existingLike = database.ReadLikeByAccountAndNote(accountId, actualNoteID)
			}
			if err != nil {
				log.Printf("Failed to read existing like: %v", err)
				return common.UpdateNoteList
			}

			// Delete the like
			if isRemotePost {
				if err := database.DeleteLikeByAccountAndObjectURI(accountId, actualNoteURI); err != nil {
					log.Printf("Failed to delete like: %v", err)
					return common.UpdateNoteList
				}
				// Decrement like count on the activity
				if err := database.DecrementLikeCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to decrement activity like count: %v", err)
				}
			} else {
				if err := database.DeleteLikeByAccountAndNote(accountId, actualNoteID); err != nil {
					log.Printf("Failed to delete like: %v", err)
					return common.UpdateNoteList
				}
				// Decrement like count on the note
				if err := database.DecrementLikeCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to decrement like count: %v", err)
				}
			}

			log.Printf("Unliked post %s", actualNoteURI)

			// Send Undo Like to remote server (background task)
			if actualNoteURI != "" && existingLike != nil {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for unlike federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendUndoLike(account, actualNoteURI, existingLike.URI, conf); err != nil {
						log.Printf("Failed to federate unlike: %v", err)
					} else {
						log.Printf("Unlike federated successfully")
					}
				}()
			}
		} else {
			// Like - create a new like
			likeURI := ""
			conf, err := util.ReadConf()
			if err == nil && conf.Conf.WithAp {
				likeURI = fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
			}

			like := &domain.Like{
				Id:        uuid.New(),
				AccountId: accountId,
				NoteId:    actualNoteID, // Will be uuid.Nil for remote posts
				URI:       likeURI,
				CreatedAt: time.Now(),
			}

			// Create the like
			if isRemotePost {
				if err := database.CreateLikeByObjectURI(like, actualNoteURI); err != nil {
					log.Printf("Failed to create like: %v", err)
					return common.UpdateNoteList
				}
				// Increment like count on the activity
				if err := database.IncrementLikeCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to increment activity like count: %v", err)
				}
			} else {
				if err := database.CreateLike(like); err != nil {
					log.Printf("Failed to create like: %v", err)
					return common.UpdateNoteList
				}
				// Increment like count on the note
				if err := database.IncrementLikeCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to increment like count: %v", err)
				}

				// Create notification for local note author
				err, note := database.ReadNoteId(actualNoteID)
				if err == nil && note != nil {
					err, noteAuthor := database.ReadAccByUsername(note.CreatedBy)
					if err == nil && noteAuthor != nil && noteAuthor.Id != accountId {
						// Only notify if liker is not the author
						preview := note.Message
						if len(preview) > 100 {
							preview = preview[:100] + "..."
						}
						notification := &domain.Notification{
							Id:               uuid.New(),
							AccountId:        noteAuthor.Id,
							NotificationType: domain.NotificationLike,
							ActorId:          accountId,
							ActorUsername:     account.Username,
							ActorDomain:      "", // Empty for local users
							NoteId:           note.Id,
							NoteURI:          note.ObjectURI,
							NotePreview:      preview,
							Read:             false,
							CreatedAt:        time.Now(),
						}
						if err := database.CreateNotification(notification); err != nil {
							log.Printf("Failed to create like notification: %v", err)
						}
					}
				}
			}

			log.Printf("Liked post %s", actualNoteURI)

			// Send Like to remote server (background task)
			if actualNoteURI != "" {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for like federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendLike(account, actualNoteURI, conf); err != nil {
						log.Printf("Failed to federate like: %v", err)
					} else {
						log.Printf("Like federated successfully")
					}
				}()
			}
		}

		return common.UpdateNoteList
	}
}

// boostNoteCmd handles boosting/unboosting a note
func boostNoteCmd(accountId uuid.UUID, noteURI string, noteID uuid.UUID, isLocal bool, account *domain.Account) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Determine the actual note ID and URI to use
		var actualNoteID uuid.UUID
		var actualNoteURI string
		var isRemotePost bool

		if isLocal && noteID != uuid.Nil {
			actualNoteID = noteID
			// Get the note's ObjectURI for federation
			err, note := database.ReadNoteId(noteID)
			if err != nil {
				log.Printf("Failed to read note for boost: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		} else if noteURI != "" && !strings.HasPrefix(noteURI, "local:") {
			// Remote note - find it by ObjectURI
			err, activity := database.ReadActivityByObjectURI(noteURI)
			if err != nil || activity == nil {
				log.Printf("Failed to find activity for boost: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = noteURI
			isRemotePost = true
			// Try to find a local note with this URI (federated back)
			err, localNote := database.ReadNoteByURI(noteURI)
			if err == nil && localNote != nil {
				actualNoteID = localNote.Id
				isRemotePost = false // It's actually a local post that was federated back
			}
		} else if strings.HasPrefix(noteURI, "local:") {
			// Parse local: prefix
			idStr := strings.TrimPrefix(noteURI, "local:")
			parsedID, err := uuid.Parse(idStr)
			if err != nil {
				log.Printf("Failed to parse local note ID: %v", err)
				return common.UpdateNoteList
			}
			actualNoteID = parsedID
			// Get the note's ObjectURI for federation
			err, note := database.ReadNoteId(parsedID)
			if err != nil {
				log.Printf("Failed to read note for boost: %v", err)
				return common.UpdateNoteList
			}
			actualNoteURI = note.ObjectURI
		}

		// Check if we already boosted this post
		var hasBoost bool
		var err error
		if isRemotePost {
			hasBoost, err = database.HasBoostByObjectURI(accountId, actualNoteURI)
		} else {
			hasBoost, err = database.HasBoost(accountId, actualNoteID)
		}
		if err != nil {
			log.Printf("Failed to check existing boost: %v", err)
			return common.UpdateNoteList
		}

		if hasBoost {
			// Unboost - remove the boost
			var existingBoost *domain.Boost
			if isRemotePost {
				err, existingBoost = database.ReadBoostByAccountAndObjectURI(accountId, actualNoteURI)
			} else {
				err, existingBoost = database.ReadBoostByAccountAndNote(accountId, actualNoteID)
			}
			if err != nil {
				log.Printf("Failed to read existing boost: %v", err)
				return common.UpdateNoteList
			}

			// Delete the boost
			if isRemotePost {
				if err := database.DeleteBoostByAccountAndObjectURI(accountId, actualNoteURI); err != nil {
					log.Printf("Failed to delete boost: %v", err)
					return common.UpdateNoteList
				}
				// Decrement boost count on the activity
				if err := database.DecrementBoostCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to decrement activity boost count: %v", err)
				}
			} else {
				if err := database.DeleteBoostByAccountAndNote(accountId, actualNoteID); err != nil {
					log.Printf("Failed to delete boost: %v", err)
					return common.UpdateNoteList
				}
				// Decrement boost count on the note
				if err := database.DecrementBoostCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to decrement boost count: %v", err)
				}
			}

			log.Printf("Unboosted post %s", actualNoteURI)

			// Send Undo Announce to remote server (background task)
			if actualNoteURI != "" && existingBoost != nil {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for unboost federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendUndoAnnounce(account, actualNoteURI, existingBoost.URI, conf); err != nil {
						log.Printf("Failed to federate unboost: %v", err)
					} else {
						log.Printf("Unboost federated successfully")
					}
				}()
			}
		} else {
			// Boost - create a new boost
			boostURI := ""
			conf, err := util.ReadConf()
			if err == nil && conf.Conf.WithAp {
				boostURI = fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
			}

			boost := &domain.Boost{
				Id:        uuid.New(),
				AccountId: accountId,
				NoteId:    actualNoteID, // Will be uuid.Nil for remote posts
				URI:       boostURI,
				CreatedAt: time.Now(),
			}

			// Create the boost
			if isRemotePost {
				if err := database.CreateBoostByObjectURI(boost, actualNoteURI); err != nil {
					log.Printf("Failed to create boost: %v", err)
					return common.UpdateNoteList
				}
				// Increment boost count on the activity
				if err := database.IncrementBoostCountByObjectURI(actualNoteURI); err != nil {
					log.Printf("Failed to increment activity boost count: %v", err)
				}
			} else {
				if err := database.CreateBoost(boost); err != nil {
					log.Printf("Failed to create boost: %v", err)
					return common.UpdateNoteList
				}
				// Increment boost count on the note
				if err := database.IncrementBoostCountByNoteId(actualNoteID); err != nil {
					log.Printf("Failed to increment boost count: %v", err)
				}

				// Create notification for local note author
				err, note := database.ReadNoteId(actualNoteID)
				if err == nil && note != nil {
					err, noteAuthor := database.ReadAccByUsername(note.CreatedBy)
					if err == nil && noteAuthor != nil && noteAuthor.Id != accountId {
						// Only notify if booster is not the author
						preview := note.Message
						if len(preview) > 100 {
							preview = preview[:100] + "..."
						}
						notification := &domain.Notification{
							Id:               uuid.New(),
							AccountId:        noteAuthor.Id,
							NotificationType: domain.NotificationBoost,
							ActorId:          accountId,
							ActorUsername:     account.Username,
							ActorDomain:      "", // Empty for local users
							NoteId:           note.Id,
							NoteURI:          note.ObjectURI,
							NotePreview:      preview,
							Read:             false,
							CreatedAt:        time.Now(),
						}
						if err := database.CreateNotification(notification); err != nil {
							log.Printf("Failed to create boost notification: %v", err)
						}
					}
				}
			}

			log.Printf("Boosted post %s", actualNoteURI)

			// Send Announce to remote server (background task)
			if actualNoteURI != "" {
				go func() {
					conf, err := util.ReadConf()
					if err != nil {
						log.Printf("Failed to read config for boost federation: %v", err)
						return
					}

					if !conf.Conf.WithAp {
						return
					}

					if err := activitypub.SendAnnounce(account, actualNoteURI, boostURI, conf); err != nil {
						log.Printf("Failed to federate boost: %v", err)
					} else {
						log.Printf("Boost federated successfully")
					}
				}()
			}
		}

		return common.UpdateNoteList
	}
}
