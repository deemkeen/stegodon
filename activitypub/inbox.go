package activitypub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// InboxDeps holds dependencies for inbox handlers (for testing)
type InboxDeps struct {
	Database   Database
	HTTPClient HTTPClient
}

// extractKeyIdFromSignature extracts the keyId from an HTTP Signature header
// The header format is: keyId="...",algorithm="...",headers="...",signature="..."
func extractKeyIdFromSignature(signature string) string {
	// Look for keyId="..." in the signature header
	for _, part := range strings.Split(signature, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "keyId=") {
			// Extract the value between quotes
			value := strings.TrimPrefix(part, "keyId=")
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}

// Activity represents a generic ActivityPub activity
type Activity struct {
	Context any    `json:"@context"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	Actor   string `json:"actor"`
	Object  any    `json:"object"`
}

// FollowActivity represents an ActivityPub Follow activity
type FollowActivity struct {
	Context any    `json:"@context"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	Actor   string `json:"actor"`
	Object  string `json:"object"` // URI of the person being followed
}

// HandleInbox processes incoming ActivityPub activities
func HandleInbox(w http.ResponseWriter, r *http.Request, username string, conf *util.AppConfig) {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	HandleInboxWithDeps(w, r, username, conf, deps)
}

// HandleInboxWithDeps processes incoming ActivityPub activities.
// This version accepts dependencies for testing.
func HandleInboxWithDeps(w http.ResponseWriter, r *http.Request, username string, conf *util.AppConfig, deps *InboxDeps) {
	// Verify HTTP signature
	signature := r.Header.Get("Signature")
	if signature == "" {
		log.Printf("Inbox: Missing HTTP signature")
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	// Extract keyId from signature header to determine whose key to use for verification
	// The signer may be different from the activity actor (e.g., relay forwarding content)
	signerKeyId := extractKeyIdFromSignature(signature)
	if signerKeyId == "" {
		log.Printf("Inbox: Could not extract keyId from signature")
		http.Error(w, "Invalid signature format", http.StatusUnauthorized)
		return
	}
	signerActorURI := strings.Split(signerKeyId, "#")[0]

	// Read request body with size limit (1MB max to prevent DoS)
	const maxBodySize = 1 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		log.Printf("Inbox: Failed to read body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Check if body was truncated (too large)
	if len(body) == maxBodySize {
		log.Printf("Inbox: Request body too large")
		http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Parse activity
	var activity Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		log.Printf("Inbox: Failed to parse activity: %v", err)
		http.Error(w, "Invalid activity", http.StatusBadRequest)
		return
	}

	log.Printf("Inbox: Received %s from %s", activity.Type, activity.Actor)

	// Fetch the signer's actor (may be different from activity actor for relay-forwarded content)
	signerActor, err := GetOrFetchActorWithDeps(signerActorURI, deps.HTTPClient, deps.Database)
	if err != nil {
		log.Printf("Inbox: Failed to fetch signer actor %s: %v", signerActorURI, err)
		http.Error(w, "Failed to verify signer", http.StatusBadRequest)
		return
	}

	// Restore body for signature verification (body was consumed during read)
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Verify HTTP signature with signer's public key
	_, err = VerifyRequest(r, signerActor.PublicKeyPem)
	if err != nil {
		log.Printf("Inbox: Signature verification failed: %v", err)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// If signer is different from activity actor, also fetch/cache the activity actor
	var remoteActor *domain.RemoteAccount
	if signerActorURI != activity.Actor {
		log.Printf("Inbox: Activity signed by %s on behalf of %s", signerActorURI, activity.Actor)
		remoteActor, err = GetOrFetchActorWithDeps(activity.Actor, deps.HTTPClient, deps.Database)
		if err != nil {
			log.Printf("Inbox: Failed to fetch activity actor %s: %v", activity.Actor, err)
			// For relay content, we can continue without the original actor
			// The activity will still be processed
		}
	} else {
		remoteActor = signerActor
	}

	// Store activity in database (except for Announce which may need special handling for relays)
	database := deps.Database

	// Extract ObjectURI from the activity's object field
	objectURI := ""
	if activity.Object != nil {
		switch obj := activity.Object.(type) {
		case string:
			// Object is a simple URI string (like in Follow, Undo, etc.)
			objectURI = obj
		case map[string]any:
			// Object is a full object (like in Create, Update)
			if id, ok := obj["id"].(string); ok {
				objectURI = id
			}
		}
	}

	// For Announce activities, defer storage to the handler (relay vs regular boost have different storage needs)
	// Set FromRelay if the signer is different from the activity actor (relay forwarding)
	isFromRelay := signerActorURI != activity.Actor

	// If this is relay content, check if the specific relay is paused
	if isFromRelay {
		relay := findRelayByActorDomain(signerActorURI, database)
		if relay != nil && relay.Paused {
			// This specific relay is paused - log but don't save
			log.Printf("Inbox: Relay content from %s skipped (relay %s is paused)", activity.Actor, relay.ActorURI)
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}

	var activityRecord *domain.Activity
	if activity.Type != "Announce" {
		activityRecord = &domain.Activity{
			Id:           uuid.New(),
			ActivityURI:  activity.ID,
			ActivityType: activity.Type,
			ActorURI:     activity.Actor,
			ObjectURI:    objectURI,
			RawJSON:      string(body),
			Processed:    false,
			Local:        false,
			FromRelay:    isFromRelay,
			CreatedAt:    time.Now(),
		}

		if err := database.CreateActivity(activityRecord); err != nil {
			// Check if this is a duplicate (already processed)
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				log.Printf("Inbox: Activity %s already processed, returning success", activity.ID)
				w.WriteHeader(http.StatusAccepted)
				return
			}
			log.Printf("Inbox: Failed to store activity: %v", err)
			// Don't fail the request, we'll process it anyway
		}
	}

	// Process activity based on type
	switch activity.Type {
	case "Follow":
		if err := handleFollowActivityWithDeps(body, username, remoteActor, conf, deps); err != nil {
			log.Printf("Inbox: Failed to handle Follow: %v", err)
			http.Error(w, "Failed to process Follow", http.StatusInternalServerError)
			return
		}
	case "Undo":
		if err := handleUndoActivityWithDeps(body, username, remoteActor, deps); err != nil {
			log.Printf("Inbox: Failed to handle Undo: %v", err)
			http.Error(w, "Failed to process Undo", http.StatusInternalServerError)
			return
		}
	case "Create":
		if err := handleCreateActivityWithDeps(body, username, isFromRelay, deps); err != nil {
			log.Printf("Inbox: Failed to handle Create: %v", err)
			http.Error(w, "Failed to process Create", http.StatusInternalServerError)
			return
		}
	case "Like":
		if err := handleLikeActivityWithDeps(body, username, deps); err != nil {
			log.Printf("Inbox: Failed to handle Like: %v", err)
			http.Error(w, "Failed to process Like", http.StatusInternalServerError)
			return
		}
	case "Announce":
		if err := handleAnnounceActivityWithDeps(body, username, deps); err != nil {
			log.Printf("Inbox: Failed to handle Announce: %v", err)
			http.Error(w, "Failed to process Announce", http.StatusInternalServerError)
			return
		}
	case "Accept":
		// Accept activities are confirmations of Follow requests
		if err := handleAcceptActivityWithDeps(body, username, deps); err != nil {
			log.Printf("Inbox: Failed to handle Accept: %v", err)
			// Don't fail the request
		}
	case "Update":
		if err := handleUpdateActivityWithDeps(body, username, deps); err != nil {
			log.Printf("Inbox: Failed to handle Update: %v", err)
			http.Error(w, "Failed to process Update", http.StatusInternalServerError)
			return
		}
	case "Delete":
		if err := handleDeleteActivityWithDeps(body, username, deps); err != nil {
			log.Printf("Inbox: Failed to handle Delete: %v", err)
			http.Error(w, "Failed to process Delete", http.StatusInternalServerError)
			return
		}
	default:
		log.Printf("Inbox: Unsupported activity type: %s", activity.Type)
	}

	// Mark activity as processed (only if we stored it above - Announce handles its own storage)
	if activityRecord != nil {
		activityRecord.Processed = true
		if err := database.UpdateActivity(activityRecord); err != nil {
			log.Printf("Inbox: Failed to update activity: %v", err)
			// Continue anyway, this is not critical
		}
	}

	// Return 202 Accepted
	w.WriteHeader(http.StatusAccepted)
}

// handleFollowActivity processes a Follow activity
func handleFollowActivity(body []byte, username string, remoteActor *domain.RemoteAccount, conf *util.AppConfig) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleFollowActivityWithDeps(body, username, remoteActor, conf, deps)
}

// handleFollowActivityWithDeps processes a Follow activity.
// This version accepts dependencies for testing.
func handleFollowActivityWithDeps(body []byte, username string, remoteActor *domain.RemoteAccount, conf *util.AppConfig, deps *InboxDeps) error {
	var follow FollowActivity
	if err := json.Unmarshal(body, &follow); err != nil {
		return fmt.Errorf("failed to parse Follow activity: %w", err)
	}

	log.Printf("Inbox: Processing Follow from %s@%s", remoteActor.Username, remoteActor.Domain)

	// Get local account
	database := deps.Database
	err, localAccount := database.ReadAccByUsername(username)
	if err != nil {
		return fmt.Errorf("local account not found: %w", err)
	}

	// Check if follow relationship already exists
	err, existingFollow := database.ReadFollowByAccountIds(remoteActor.Id, localAccount.Id)
	if err == nil && existingFollow != nil {
		// Follow already exists, just log and continue to send Accept
		log.Printf("Inbox: Follow relationship from %s@%s already exists, skipping duplicate", remoteActor.Username, remoteActor.Domain)
	} else {
		// Create follow relationship
		// When remote actor follows local account:
		// - AccountId = remote actor (the follower)
		// - TargetAccountId = local account (being followed)
		followRecord := &domain.Follow{
			Id:              uuid.New(),
			AccountId:       remoteActor.Id,  // The follower
			TargetAccountId: localAccount.Id, // The target being followed
			URI:             follow.ID,
			Accepted:        true, // Auto-accept for now
			CreatedAt:       time.Now(),
		}

		if err := database.CreateFollow(followRecord); err != nil {
			return fmt.Errorf("failed to create follow: %w", err)
		}

		// Create notification for the followed user
		notification := &domain.Notification{
			Id:               uuid.New(),
			AccountId:        localAccount.Id, // The local user being followed
			NotificationType: domain.NotificationFollow,
			ActorId:          remoteActor.Id,
			ActorUsername:    remoteActor.Username,
			ActorDomain:      remoteActor.Domain,
			Read:             false,
			CreatedAt:        time.Now(),
		}
		if err := database.CreateNotification(notification); err != nil {
			log.Printf("Inbox: Failed to create follow notification: %v", err)
			// Don't fail the request for notification errors
		}
	}

	// Send Accept activity
	if err := SendAcceptWithDeps(localAccount, remoteActor, follow.ID, conf, deps.HTTPClient); err != nil {
		return fmt.Errorf("failed to send Accept: %w", err)
	}

	log.Printf("Inbox: Accepted follow from %s@%s", remoteActor.Username, remoteActor.Domain)
	return nil
}

// handleUndoActivity processes an Undo activity (e.g., Undo Follow)
func handleUndoActivity(body []byte, username string, remoteActor *domain.RemoteAccount) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleUndoActivityWithDeps(body, username, remoteActor, deps)
}

// handleUndoActivityWithDeps processes an Undo activity (e.g., Undo Follow, Undo Like).
// This version accepts dependencies for testing.
func handleUndoActivityWithDeps(body []byte, username string, remoteActor *domain.RemoteAccount, deps *InboxDeps) error {
	// Parse the Undo activity
	var undo struct {
		Type   string          `json:"type"`
		Actor  string          `json:"actor"`
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(body, &undo); err != nil {
		return fmt.Errorf("failed to parse Undo activity: %w", err)
	}

	// Parse the embedded object
	var obj struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Object string `json:"object"` // For Like, this is the URI of the liked note
	}
	if err := json.Unmarshal(undo.Object, &obj); err != nil {
		return fmt.Errorf("failed to parse Undo object: %w", err)
	}

	database := deps.Database

	if obj.Type == "Follow" {
		// Verify authorization: Undo actor must match Follow actor

		// Fetch the follow to verify ownership
		err, follow := database.ReadFollowByURI(obj.ID)
		if err != nil {
			return fmt.Errorf("follow not found: %w", err)
		}
		if follow == nil {
			return fmt.Errorf("follow not found")
		}

		// Verify the Undo actor matches the Follow actor
		// For remote follows, the AccountId is the remote actor who created the follow
		err, followActor := database.ReadRemoteAccountById(follow.AccountId)
		if err != nil || followActor == nil {
			return fmt.Errorf("follow actor not found")
		}
		if followActor.ActorURI != undo.Actor {
			return fmt.Errorf("unauthorized: actor %s cannot undo follow created by %s", undo.Actor, followActor.ActorURI)
		}

		// Authorization passed, delete the follow relationship
		if err := database.DeleteFollowByURI(obj.ID); err != nil {
			return fmt.Errorf("failed to delete follow: %w", err)
		}
		log.Printf("Inbox: Removed follow from %s@%s", remoteActor.Username, remoteActor.Domain)
	} else if obj.Type == "Like" {
		// Handle Undo Like
		// Find the note being unliked
		err, note := database.ReadNoteByURI(obj.Object)
		if err != nil || note == nil {
			log.Printf("Inbox: Note not found for Undo Like object %s", obj.Object)
			return nil // Not an error - note might not exist locally
		}

		// Verify the actor matches (they can only undo their own likes)
		if remoteActor.ActorURI != undo.Actor {
			return fmt.Errorf("unauthorized: actor %s cannot undo like", undo.Actor)
		}

		// Delete the like
		if err := database.DeleteLikeByAccountAndNote(remoteActor.Id, note.Id); err != nil {
			log.Printf("Inbox: Failed to delete like: %v", err)
			return nil // Don't fail if like doesn't exist
		}

		// Decrement like count
		if err := database.DecrementLikeCountByNoteId(note.Id); err != nil {
			log.Printf("Inbox: Failed to decrement like count: %v", err)
		}

		log.Printf("Inbox: Removed like from %s@%s on note %s", remoteActor.Username, remoteActor.Domain, note.Id)
	} else if obj.Type == "Announce" {
		// Handle Undo Announce (unboost)
		// Find the note being unboosted
		err, note := database.ReadNoteByURI(obj.Object)
		if err != nil || note == nil {
			log.Printf("Inbox: Note not found for Undo Announce object %s", obj.Object)
			return nil // Not an error - note might not exist locally
		}

		// Verify the actor matches (they can only undo their own boosts)
		if remoteActor.ActorURI != undo.Actor {
			return fmt.Errorf("unauthorized: actor %s cannot undo boost", undo.Actor)
		}

		// Delete the boost
		if err := database.DeleteBoostByAccountAndNote(remoteActor.Id, note.Id); err != nil {
			log.Printf("Inbox: Failed to delete boost: %v", err)
			return nil // Don't fail if boost doesn't exist
		}

		// Decrement boost count
		if err := database.DecrementBoostCountByNoteId(note.Id); err != nil {
			log.Printf("Inbox: Failed to decrement boost count: %v", err)
		}

		log.Printf("Inbox: Removed boost from %s@%s on note %s", remoteActor.Username, remoteActor.Domain, note.Id)
	}

	return nil
}

// handleCreateActivity processes a Create activity (incoming post/note)
func handleCreateActivity(body []byte, username string, isFromRelay bool) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleCreateActivityWithDeps(body, username, isFromRelay, deps)
}

// handleCreateActivityWithDeps processes a Create activity (incoming post/note).
// This version accepts dependencies for testing.
func handleCreateActivityWithDeps(body []byte, username string, isFromRelay bool, deps *InboxDeps) error {
	var create struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object struct {
			ID           string `json:"id"`
			Type         string `json:"type"`
			Content      string `json:"content"`
			Published    string `json:"published"`
			AttributedTo string `json:"attributedTo"`
			InReplyTo    string `json:"inReplyTo"`
			Tag          []struct {
				Type string `json:"type"`
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"tag"`
		} `json:"object"`
	}

	if err := json.Unmarshal(body, &create); err != nil {
		return fmt.Errorf("failed to parse Create activity: %w", err)
	}

	log.Printf("Inbox: Received post from %s", create.Actor)

	// Log if this is a reply
	if create.Object.InReplyTo != "" {
		log.Printf("Inbox: Post is a reply to %s", create.Object.InReplyTo)
	}

	database := deps.Database

	// Get the local account
	err, localAccount := database.ReadAccByUsername(username)
	if err != nil {
		log.Printf("Inbox: Failed to get local account %s: %v", username, err)
		return fmt.Errorf("failed to get local account: %w", err)
	}
	log.Printf("Inbox: Local account: %s (ID: %s)", localAccount.Username, localAccount.Id)

	// Get the remote actor (try cache first, fetch if not found)
	err, remoteActor := database.ReadRemoteAccountByActorURI(create.Actor)
	if err != nil || remoteActor == nil {
		// Not in cache, try to fetch it
		log.Printf("Inbox: Actor %s not cached, fetching...", create.Actor)
		remoteActor, err = FetchRemoteActorWithDeps(create.Actor, deps.HTTPClient, deps.Database)
		if err != nil {
			log.Printf("Inbox: Failed to fetch actor %s: %v", create.Actor, err)
			return fmt.Errorf("unknown actor")
		}
	}
	log.Printf("Inbox: Remote actor: %s@%s (ID: %s)", remoteActor.Username, remoteActor.Domain, remoteActor.Id)

	// Check if we follow this actor (skip for relay content - isFromRelay is set when signer != actor)
	err, follow := database.ReadFollowByAccountIds(localAccount.Id, remoteActor.Id)
	isFollowing := err == nil && follow != nil

	if isFollowing {
		log.Printf("Inbox: Accepted post from followed user %s@%s (follow accepted: %v)", remoteActor.Username, remoteActor.Domain, follow.Accepted)
	} else if isFromRelay {
		// Relay-forwarded content (signer was different from activity actor)
		log.Printf("Inbox: Accepting relay-forwarded Create from %s", create.Actor)
	} else {
		// Not following - only accept if this is a reply to one of our posts
		isReplyToOurPost := false
		if create.Object.InReplyTo != "" {
			// Check if the parent post belongs to the local user
			err, parentNote := database.ReadNoteByURI(create.Object.InReplyTo)
			if err == nil && parentNote != nil && parentNote.CreatedBy == username {
				isReplyToOurPost = true
				log.Printf("Inbox: This is a reply to our post, accepting without follow check")
			}
		}

		if !isReplyToOurPost {
			log.Printf("Inbox: Rejecting Create from %s - not following and not a reply to our post", create.Actor)
			return fmt.Errorf("not following this actor")
		}
	}

	// Increment reply count on the parent post if this is a reply
	// But skip if this activity is a duplicate of a local note (our own post coming back via federation)
	if create.Object.InReplyTo != "" {
		// Check if this activity's object_uri matches an existing local note
		// This happens when our own post is federated out and comes back
		err, existingNote := database.ReadNoteByURI(create.Object.ID)
		isDuplicate := err == nil && existingNote != nil

		if isDuplicate {
			log.Printf("Inbox: Skipping reply count increment - activity %s is a duplicate of local note", create.Object.ID)
		} else {
			if err := database.IncrementReplyCountByURI(create.Object.InReplyTo); err != nil {
				log.Printf("Inbox: Failed to increment reply count for %s: %v", create.Object.InReplyTo, err)
				// Don't fail the activity processing for this
			} else {
				log.Printf("Inbox: Incremented reply count for %s", create.Object.InReplyTo)
			}

			// Create reply notification for the parent note author
			err, parentNote := database.ReadNoteByURI(create.Object.InReplyTo)
			if err == nil && parentNote != nil {
				err, parentAuthor := database.ReadAccByUsername(parentNote.CreatedBy)
				if err == nil && parentAuthor != nil {
					// Extract plain text preview from HTML content
					preview := util.StripHTMLTags(create.Object.Content)
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
					notification := &domain.Notification{
						Id:               uuid.New(),
						AccountId:        parentAuthor.Id,
						NotificationType: domain.NotificationReply,
						ActorId:          remoteActor.Id,
						ActorUsername:    remoteActor.Username,
						ActorDomain:      remoteActor.Domain,
						NoteURI:          create.Object.ID,
						NotePreview:      preview,
						Read:             false,
						CreatedAt:        time.Now(),
					}
					if err := database.CreateNotification(notification); err != nil {
						log.Printf("Inbox: Failed to create reply notification: %v", err)
					}
				}
			}
		}
	}

	// Process tags (hashtags and mentions) from the incoming activity
	// Store mentions in the database for future notification support
	if len(create.Object.Tag) > 0 {
		// Get the activity record to link mentions to it
		err, activityRecord := database.ReadActivityByObjectURI(create.Object.ID)
		if err != nil || activityRecord == nil {
			log.Printf("Inbox: Could not find activity record for %s, skipping mention storage", create.Object.ID)
		}

		for _, tag := range create.Object.Tag {
			switch tag.Type {
			case "Mention":
				log.Printf("Inbox: Post mentions %s (%s)", tag.Name, tag.Href)

				// Store the mention in the database
				if activityRecord != nil {
					// Parse username and domain from @username@domain format
					mentionName := strings.TrimPrefix(tag.Name, "@")
					parts := strings.SplitN(mentionName, "@", 2)
					if len(parts) == 2 {
						mention := &domain.NoteMention{
							Id:                uuid.New(),
							NoteId:            activityRecord.Id, // Use activity ID as the note reference
							MentionedActorURI: tag.Href,
							MentionedUsername: parts[0],
							MentionedDomain:   parts[1],
							CreatedAt:         time.Now(),
						}
						if err := database.CreateNoteMention(mention); err != nil {
							log.Printf("Inbox: Failed to store mention %s: %v", tag.Name, err)
						} else {
							log.Printf("Inbox: Stored mention %s for activity %s", tag.Name, activityRecord.Id)

							// Create notification if the mentioned user is local
							// Need to check if this domain matches our local domain
							conf, confErr := util.ReadConf()
							if confErr == nil && conf != nil && parts[1] == conf.Conf.SslDomain {
								err, mentionedUser := database.ReadAccByUsername(parts[0])
								if err == nil && mentionedUser != nil {
									preview := util.StripHTMLTags(create.Object.Content)
									if len(preview) > 100 {
										preview = preview[:100] + "..."
									}
									notification := &domain.Notification{
										Id:               uuid.New(),
										AccountId:        mentionedUser.Id,
										NotificationType: domain.NotificationMention,
										ActorId:          remoteActor.Id,
										ActorUsername:    remoteActor.Username,
										ActorDomain:      remoteActor.Domain,
										NoteURI:          create.Object.ID,
										NotePreview:      preview,
										Read:             false,
										CreatedAt:        time.Now(),
									}
									if err := database.CreateNotification(notification); err != nil {
										log.Printf("Inbox: Failed to create mention notification: %v", err)
									}
								}
							}
						}
					}
				}
			case "Hashtag":
				// Hashtags are already included in the stored activity raw JSON
				log.Printf("Inbox: Post contains hashtag %s", tag.Name)
			}
		}
	}

	// Note: Activity is already stored in HandleInbox before this function is called
	// No need to store it again here

	return nil
}

// handleLikeActivity processes a Like activity
func handleLikeActivity(body []byte, username string) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleLikeActivityWithDeps(body, username, deps)
}

// handleLikeActivityWithDeps processes a Like activity.
// This version accepts dependencies for testing.
func handleLikeActivityWithDeps(body []byte, username string, deps *InboxDeps) error {
	log.Printf("Inbox: Processing Like activity for %s", username)

	var likeActivity struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object string `json:"object"` // URI of the liked object (note)
	}

	if err := json.Unmarshal(body, &likeActivity); err != nil {
		return fmt.Errorf("failed to parse Like activity: %w", err)
	}

	if likeActivity.ID == "" {
		return fmt.Errorf("Like activity missing id")
	}
	if likeActivity.Actor == "" {
		return fmt.Errorf("Like activity missing actor")
	}
	if likeActivity.Object == "" {
		return fmt.Errorf("Like activity missing object")
	}

	database := deps.Database

	// Find the note being liked by its object_uri
	err, note := database.ReadNoteByURI(likeActivity.Object)
	if err != nil || note == nil {
		log.Printf("Inbox: Note not found for Like object %s: %v", likeActivity.Object, err)
		return nil // Not an error - the note might not exist locally
	}

	// Get or create remote account for the liker using the existing helper
	remoteAcc, fetchErr := GetOrFetchActorWithDeps(likeActivity.Actor, deps.HTTPClient, database)
	if fetchErr != nil {
		log.Printf("Inbox: Could not fetch actor %s for Like: %v", likeActivity.Actor, fetchErr)
		return nil // Not a fatal error
	}

	// Check if we already have a like from this account on this note (dedupe by account+note)
	exists, err := database.HasLike(remoteAcc.Id, note.Id)
	if err != nil {
		log.Printf("Inbox: Error checking for existing Like: %v", err)
	}
	if exists {
		log.Printf("Inbox: Like from %s on note %s already exists, skipping", likeActivity.Actor, note.Id)
		return nil
	}

	// Create the Like record
	like := &domain.Like{
		Id:        uuid.New(),
		AccountId: remoteAcc.Id,
		NoteId:    note.Id,
		URI:       likeActivity.ID,
		CreatedAt: time.Now(),
	}

	if err := database.CreateLike(like); err != nil {
		return fmt.Errorf("failed to store Like: %w", err)
	}

	// Increment like count on the note
	if err := database.IncrementLikeCountByNoteId(note.Id); err != nil {
		log.Printf("Inbox: Failed to increment like count: %v", err)
	}

	// Create notification for the note author
	err, noteAuthor := database.ReadAccByUsername(note.CreatedBy)
	if err == nil && noteAuthor != nil {
		preview := note.Message
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		notification := &domain.Notification{
			Id:               uuid.New(),
			AccountId:        noteAuthor.Id,
			NotificationType: domain.NotificationLike,
			ActorId:          remoteAcc.Id,
			ActorUsername:    remoteAcc.Username,
			ActorDomain:      remoteAcc.Domain,
			NoteId:           note.Id,
			NoteURI:          note.ObjectURI,
			NotePreview:      preview,
			Read:             false,
			CreatedAt:        time.Now(),
		}
		if err := database.CreateNotification(notification); err != nil {
			log.Printf("Inbox: Failed to create like notification: %v", err)
		}
	}

	log.Printf("Inbox: Stored Like from %s on note %s", likeActivity.Actor, note.Id)
	return nil
}

// handleAnnounceActivity processes an Announce (boost/reblog) activity
func handleAnnounceActivity(body []byte, username string) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleAnnounceActivityWithDeps(body, username, deps)
}

// handleAnnounceActivityWithDeps processes an Announce (boost/reblog) activity.
// This version accepts dependencies for testing.
func handleAnnounceActivityWithDeps(body []byte, username string, deps *InboxDeps) error {
	log.Printf("Inbox: Processing Announce activity for %s", username)

	var announceActivity struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Actor     string `json:"actor"`
		Object    any    `json:"object"`    // Can be string URI or object with id field
		Published string `json:"published"` // For relay-forwarded content
	}

	if err := json.Unmarshal(body, &announceActivity); err != nil {
		return fmt.Errorf("failed to parse Announce activity: %w", err)
	}

	if announceActivity.ID == "" {
		return fmt.Errorf("Announce activity missing id")
	}
	if announceActivity.Actor == "" {
		return fmt.Errorf("Announce activity missing actor")
	}
	if announceActivity.Object == nil {
		return fmt.Errorf("Announce activity missing object")
	}

	database := deps.Database

	// Check if this Announce is from a relay we're subscribed to
	// First try exact match
	err, relay := database.ReadRelayByActorURI(announceActivity.Actor)
	isFromRelay := err == nil && relay != nil

	// If not exact match, check if the actor is from any relay's domain
	// (e.g., relay.fedi.buzz/tag/prints when we subscribed to relay.fedi.buzz/tag/music)
	if !isFromRelay {
		isFromRelay = isActorFromAnyRelay(announceActivity.Actor, database)
	}

	// Extract object URI and possibly the full object from the object field
	var objectURI string
	var embeddedObject map[string]any
	switch obj := announceActivity.Object.(type) {
	case string:
		objectURI = obj
	case map[string]any:
		embeddedObject = obj
		if id, ok := obj["id"].(string); ok {
			objectURI = id
		}
	}
	if objectURI == "" {
		return fmt.Errorf("Announce activity has invalid object format")
	}

	// If this is from a relay, check if paused before storing
	if isFromRelay {
		relay := findRelayByActorDomain(announceActivity.Actor, deps.Database)
		if relay != nil && relay.Paused {
			log.Printf("Inbox: Relay Announce from %s skipped (relay %s is paused)", announceActivity.Actor, relay.ActorURI)
			return nil
		}
		return handleRelayAnnounce(announceActivity.ID, objectURI, embeddedObject, deps)
	}

	// Standard boost handling - find the note being boosted by its object_uri
	err, note := database.ReadNoteByURI(objectURI)
	if err != nil || note == nil {
		// Check if this looks like a relay actor (contains /tag/ in path) but we're not subscribed
		if strings.Contains(announceActivity.Actor, "/tag/") || strings.Contains(announceActivity.Actor, "/relay") {
			log.Printf("Inbox: Ignoring Announce from unsubscribed relay %s (object: %s)", announceActivity.Actor, objectURI)
		} else {
			log.Printf("Inbox: Note not found for Announce object %s: %v", objectURI, err)
		}
		return nil // Not an error - the note might not exist locally
	}

	// Get or create remote account for the booster using the existing helper
	remoteAcc, fetchErr := GetOrFetchActorWithDeps(announceActivity.Actor, deps.HTTPClient, database)
	if fetchErr != nil {
		log.Printf("Inbox: Could not fetch actor %s for Announce: %v", announceActivity.Actor, fetchErr)
		return nil // Not a fatal error
	}

	// Check if we already have a boost from this account on this note (dedupe by account+note)
	exists, err := database.HasBoost(remoteAcc.Id, note.Id)
	if err != nil {
		log.Printf("Inbox: Error checking for existing Boost: %v", err)
	}
	if exists {
		log.Printf("Inbox: Boost from %s on note %s already exists, skipping", announceActivity.Actor, note.Id)
		return nil
	}

	// Create the Boost record
	boost := &domain.Boost{
		Id:        uuid.New(),
		AccountId: remoteAcc.Id,
		NoteId:    note.Id,
		URI:       announceActivity.ID,
		CreatedAt: time.Now(),
	}

	if err := database.CreateBoost(boost); err != nil {
		return fmt.Errorf("failed to store Boost: %w", err)
	}

	// Increment boost count on the note
	if err := database.IncrementBoostCountByNoteId(note.Id); err != nil {
		log.Printf("Inbox: Failed to increment boost count: %v", err)
	}

	// Create notification for the note author
	err, noteAuthor := database.ReadAccByUsername(note.CreatedBy)
	if err == nil && noteAuthor != nil {
		preview := note.Message
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		notification := &domain.Notification{
			Id:               uuid.New(),
			AccountId:        noteAuthor.Id,
			NotificationType: domain.NotificationBoost,
			ActorId:          remoteAcc.Id,
			ActorUsername:    remoteAcc.Username,
			ActorDomain:      remoteAcc.Domain,
			NoteId:           note.Id,
			NoteURI:          note.ObjectURI,
			NotePreview:      preview,
			Read:             false,
			CreatedAt:        time.Now(),
		}
		if err := database.CreateNotification(notification); err != nil {
			log.Printf("Inbox: Failed to create boost notification: %v", err)
		}
	}

	log.Printf("Inbox: Stored Boost from %s on note %s", announceActivity.Actor, note.Id)
	return nil
}

// handleRelayAnnounce processes an Announce from a relay, fetching and storing the announced content
func handleRelayAnnounce(announceID, objectURI string, embeddedObject map[string]any, deps *InboxDeps) error {
	database := deps.Database

	// Check if we already have this announce activity (by activity_uri)
	err, existingByAnnounce := database.ReadActivityByURI(announceID)
	if err == nil && existingByAnnounce != nil {
		log.Printf("Inbox: Relay-forwarded activity %s already exists (matched ID: %s), skipping", announceID, existingByAnnounce.ActivityURI)
		return nil
	}

	// Check if we already have this object (by object_uri)
	err, existingActivity := database.ReadActivityByObjectURI(objectURI)
	if err == nil && existingActivity != nil {
		log.Printf("Inbox: Relay-forwarded object %s already exists (activity: %s), skipping", objectURI, existingActivity.ActivityURI)
		return nil
	}

	// Try to get the object content - either embedded or by fetching
	var objectContent map[string]any
	var actorURI string

	if embeddedObject != nil {
		// Object is embedded in the Announce
		objectContent = embeddedObject
		if actor, ok := embeddedObject["attributedTo"].(string); ok {
			actorURI = actor
		} else if actor, ok := embeddedObject["actor"].(string); ok {
			actorURI = actor
		}
	} else {
		// Need to fetch the object
		log.Printf("Inbox: Fetching relay-forwarded object %s", objectURI)
		fetchedObject, err := fetchActivityPubObject(objectURI, deps.HTTPClient)
		if err != nil {
			log.Printf("Inbox: Failed to fetch relay-forwarded object %s: %v", objectURI, err)
			return nil // Not a fatal error
		}
		objectContent = fetchedObject
		if actor, ok := fetchedObject["attributedTo"].(string); ok {
			actorURI = actor
		} else if actor, ok := fetchedObject["actor"].(string); ok {
			actorURI = actor
		}
	}

	if actorURI == "" {
		log.Printf("Inbox: Relay-forwarded object %s has no attributedTo/actor", objectURI)
		return nil
	}

	// Get the object type
	objectType, _ := objectContent["type"].(string)
	if objectType != "Note" && objectType != "Article" {
		log.Printf("Inbox: Relay-forwarded object %s is type %s, skipping", objectURI, objectType)
		return nil
	}

	// Fetch and cache the actor
	_, err = GetOrFetchActorWithDeps(actorURI, deps.HTTPClient, database)
	if err != nil {
		log.Printf("Inbox: Failed to fetch actor %s for relay-forwarded content: %v", actorURI, err)
		// Continue anyway - we can still store the activity
	}

	// Marshal the object content for storage
	rawJSON, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"type":     "Create",
		"actor":    actorURI,
		"object":   objectContent,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal relay-forwarded object: %w", err)
	}

	// Store as a Create activity so it shows in the timeline
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  announceID, // Use the Announce ID as the activity URI (unique)
		ActivityType: "Create",   // Store as Create so it shows in timeline
		ActorURI:     actorURI,
		ObjectURI:    objectURI,
		RawJSON:      string(rawJSON),
		Processed:    true,
		Local:        false,
		FromRelay:    true, // This is relay-forwarded content
		CreatedAt:    time.Now(),
	}

	if err := database.CreateActivity(activity); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Printf("Inbox: Relay-forwarded activity %s already exists", announceID)
			return nil
		}
		return fmt.Errorf("failed to store relay-forwarded activity: %w", err)
	}

	log.Printf("Inbox: Stored relay-forwarded %s from %s", objectType, actorURI)
	return nil
}

// isActorFromAnyRelay checks if an actor URI belongs to any relay domain we're subscribed to.
// This handles cases where a relay like relay.fedi.buzz sends Announces from different tag actors
// (e.g., /tag/prints) when we subscribed to /tag/music - they're all from the same relay domain.
func isActorFromAnyRelay(actorURI string, database Database) bool {
	relay := findRelayByActorDomain(actorURI, database)
	return relay != nil
}

// findRelayByActorDomain finds a relay subscription that matches the actor's domain.
// Returns nil if no matching relay is found.
func findRelayByActorDomain(actorURI string, database Database) *domain.Relay {
	// Extract domain from the actor URI
	actorDomain := extractDomainFromURI(actorURI)
	if actorDomain == "" {
		return nil
	}

	// Get all active relays
	err, relays := database.ReadActiveRelays()
	if err != nil || relays == nil {
		log.Printf("Inbox: Error reading relays for domain check: %v", err)
		return nil
	}

	// Check if the actor's domain matches any relay's domain
	for i := range *relays {
		relay := &(*relays)[i]
		relayDomain := extractDomainFromURI(relay.ActorURI)
		if relayDomain != "" && relayDomain == actorDomain {
			log.Printf("Inbox: Actor %s is from relay domain %s (matched relay: %s)", actorURI, actorDomain, relay.ActorURI)
			return relay
		}
	}

	return nil
}

// extractDomainFromURI extracts the domain (host) from a URI
func extractDomainFromURI(uri string) string {
	// Simple extraction: find the domain between :// and the next /
	if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
		return ""
	}

	// Find start after ://
	start := strings.Index(uri, "://")
	if start == -1 {
		return ""
	}
	start += 3

	// Find end at next / or end of string
	rest := uri[start:]
	end := strings.Index(rest, "/")
	if end == -1 {
		return rest
	}
	return rest[:end]
}

// fetchActivityPubObject fetches an ActivityPub object by URI
func fetchActivityPubObject(uri string, client HTTPClient) (map[string]any, error) {
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/activity+json, application/ld+json")
	req.Header.Set("User-Agent", "Stegodon/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// handleAcceptActivity processes an Accept activity (response to Follow)
func handleAcceptActivity(body []byte, username string) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleAcceptActivityWithDeps(body, username, deps)
}

// handleAcceptActivityWithDeps processes an Accept activity (response to Follow).
// This version accepts dependencies for testing.
func handleAcceptActivityWithDeps(body []byte, username string, deps *InboxDeps) error {
	var accept struct {
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object any    `json:"object"`
	}

	if err := json.Unmarshal(body, &accept); err != nil {
		return fmt.Errorf("failed to parse Accept activity: %w", err)
	}

	// Extract Follow ID from object (can be string or object)
	var followID string
	switch obj := accept.Object.(type) {
	case string:
		// Object is a simple URI string (common in Accept responses)
		followID = obj
	case map[string]any:
		// Object is a full Follow object
		if id, ok := obj["id"].(string); ok {
			followID = id
		}
	}

	if followID == "" {
		return fmt.Errorf("could not extract Follow ID from Accept object")
	}

	database := deps.Database

	// First check if this is an Accept for a relay subscription
	err, relay := database.ReadRelayByActorURI(accept.Actor)
	if err == nil && relay != nil {
		// This is an Accept from a relay - update relay status to active
		now := time.Now()
		if err := database.UpdateRelayStatus(relay.Id, "active", &now); err != nil {
			return fmt.Errorf("failed to update relay status: %w", err)
		}
		log.Printf("Inbox: Relay %s accepted our subscription", accept.Actor)
		return nil
	}

	// Not a relay - try updating a regular follow to accepted=true
	if err := database.AcceptFollowByURI(followID); err != nil {
		return fmt.Errorf("failed to accept follow: %w", err)
	}

	log.Printf("Inbox: Follow %s was accepted by %s", followID, accept.Actor)
	return nil
}

// handleUpdateActivity processes an Update activity (e.g., profile updates, post edits)
func handleUpdateActivity(body []byte, username string) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleUpdateActivityWithDeps(body, username, deps)
}

// handleUpdateActivityWithDeps processes an Update activity (e.g., profile updates, post edits).
// This version accepts dependencies for testing.
func handleUpdateActivityWithDeps(body []byte, username string, deps *InboxDeps) error {
	var update struct {
		ID     string          `json:"id"`
		Type   string          `json:"type"`
		Actor  string          `json:"actor"`
		Object json.RawMessage `json:"object"`
	}

	if err := json.Unmarshal(body, &update); err != nil {
		return fmt.Errorf("failed to parse Update activity: %w", err)
	}

	// Parse the object to determine what type it is
	var objectType struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(update.Object, &objectType); err != nil {
		return fmt.Errorf("failed to parse Update object: %w", err)
	}

	log.Printf("Inbox: Processing Update for %s (type: %s) from %s", objectType.ID, objectType.Type, update.Actor)

	database := deps.Database

	switch objectType.Type {
	case "Person":
		// Profile update - re-fetch and update cached actor
		remoteActor, err := GetOrFetchActorWithDeps(update.Actor, deps.HTTPClient, deps.Database)
		if err != nil {
			return fmt.Errorf("failed to fetch updated actor: %w", err)
		}
		log.Printf("Inbox: Updated profile for %s@%s", remoteActor.Username, remoteActor.Domain)

	case "Note", "Article":
		// Post edit - find the existing activity that contains this Note/Article
		// The activity is stored with the Create activity ID, but we need to find it by the Note ID
		err, existingActivity := database.ReadActivityByObjectURI(objectType.ID)
		if err != nil || existingActivity == nil {
			// No existing Create activity found - this can happen if:
			// 1. We followed the user after the original post was created
			// 2. The Create activity was lost during delivery
			// In this case, treat the Update as a new post by creating a synthetic Create activity
			log.Printf("Inbox: Note/Article %s not found for update, creating as new post", objectType.ID)

			newActivity := &domain.Activity{
				Id:           uuid.New(),
				ActivityURI:  update.ID, // Use Update activity URI (unique)
				ActivityType: "Create",  // Store as Create so it shows in timeline
				ActorURI:     update.Actor,
				ObjectURI:    objectType.ID,
				RawJSON:      string(body),
				Processed:    true,
				Local:        false,
				CreatedAt:    time.Now(),
			}

			if err := database.CreateActivity(newActivity); err != nil {
				// Check if this is a duplicate (already processed this Update)
				if strings.Contains(err.Error(), "UNIQUE constraint failed") {
					log.Printf("Inbox: Update activity %s already processed", update.ID)
					return nil
				}
				return fmt.Errorf("failed to create activity from Update: %w", err)
			}
			log.Printf("Inbox: Created new post from Update for Note/Article %s", objectType.ID)
			return nil
		}

		// Update the stored activity with new content but keep activity_type as 'Create'
		// so it still shows up in the timeline
		existingActivity.RawJSON = string(body)
		// Don't change the ActivityType - keep it as 'Create' so it shows in timeline
		if err := database.UpdateActivity(existingActivity); err != nil {
			return fmt.Errorf("failed to update activity: %w", err)
		}
		log.Printf("Inbox: Updated Note/Article %s", objectType.ID)

	default:
		log.Printf("Inbox: Unsupported Update object type: %s", objectType.Type)
	}

	return nil
}

// handleDeleteActivity processes a Delete activity (e.g., post deletion, account deletion)
func handleDeleteActivity(body []byte, username string) error {
	deps := &InboxDeps{
		Database:   NewDBWrapper(),
		HTTPClient: defaultHTTPClient,
	}
	return handleDeleteActivityWithDeps(body, username, deps)
}

// handleDeleteActivityWithDeps processes a Delete activity (e.g., post deletion, account deletion).
// This version accepts dependencies for testing.
func handleDeleteActivityWithDeps(body []byte, username string, deps *InboxDeps) error {
	var delete struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object any    `json:"object"`
	}

	if err := json.Unmarshal(body, &delete); err != nil {
		return fmt.Errorf("failed to parse Delete activity: %w", err)
	}

	database := deps.Database

	// Object can be either a string URI or an embedded object
	var objectURI string
	switch obj := delete.Object.(type) {
	case string:
		objectURI = obj
	case map[string]any:
		if id, ok := obj["id"].(string); ok {
			objectURI = id
		}
		if typ, ok := obj["type"].(string); ok && typ == "Tombstone" {
			// Tombstone object indicates a deletion
			if id, ok := obj["id"].(string); ok {
				objectURI = id
			}
		}
	}

	if objectURI == "" {
		return fmt.Errorf("could not determine object URI from Delete activity")
	}

	log.Printf("Inbox: Processing Delete for %s from %s", objectURI, delete.Actor)

	// Check if it's an actor deletion (URI matches the actor)
	if objectURI == delete.Actor {
		// Actor deletion - remove all their activities and follows
		log.Printf("Inbox: Actor %s deleted their account", delete.Actor)

		// Delete remote account
		err, remoteAcc := database.ReadRemoteAccountByActorURI(objectURI)
		if err == nil && remoteAcc != nil {
			// Delete all follows to/from this actor
			database.DeleteFollowsByRemoteAccountId(remoteAcc.Id)
			// Delete the remote account
			database.DeleteRemoteAccount(remoteAcc.Id)
			log.Printf("Inbox: Removed actor %s and all associated data", objectURI)
		}
	} else {
		// Object deletion (post, note, etc.) - find the activity containing this object
		err, activity := database.ReadActivityByObjectURI(objectURI)
		if err != nil || activity == nil {
			log.Printf("Inbox: Activity with object %s not found for deletion, ignoring", objectURI)
			return nil
		}

		// Verify authorization: Delete actor must match Activity actor
		if activity.ActorURI != delete.Actor {
			return fmt.Errorf("unauthorized: actor %s cannot delete content created by %s", delete.Actor, activity.ActorURI)
		}

		// Authorization passed, delete the activity from the database
		if err := database.DeleteActivity(activity.Id); err != nil {
			return fmt.Errorf("failed to delete activity: %w", err)
		}
		log.Printf("Inbox: Deleted activity containing object %s", objectURI)
	}

	return nil
}
