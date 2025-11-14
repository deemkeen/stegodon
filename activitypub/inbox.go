package activitypub

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// Activity represents a generic ActivityPub activity
type Activity struct {
	Context interface{} `json:"@context"`
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Actor   string      `json:"actor"`
	Object  interface{} `json:"object"`
}

// FollowActivity represents an ActivityPub Follow activity
type FollowActivity struct {
	Context interface{} `json:"@context"`
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Actor   string      `json:"actor"`
	Object  string      `json:"object"` // URI of the person being followed
}

// HandleInbox processes incoming ActivityPub activities
func HandleInbox(w http.ResponseWriter, r *http.Request, username string, conf *util.AppConfig) {
	// Verify HTTP signature
	signature := r.Header.Get("Signature")
	if signature == "" {
		log.Printf("Inbox: Missing HTTP signature")
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Inbox: Failed to read body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse activity
	var activity Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		log.Printf("Inbox: Failed to parse activity: %v", err)
		http.Error(w, "Invalid activity", http.StatusBadRequest)
		return
	}

	log.Printf("Inbox: Received %s from %s", activity.Type, activity.Actor)

	// Fetch remote actor to verify and cache
	remoteActor, err := GetOrFetchActor(activity.Actor)
	if err != nil {
		log.Printf("Inbox: Failed to fetch actor %s: %v", activity.Actor, err)
		http.Error(w, "Failed to verify actor", http.StatusBadRequest)
		return
	}

	// Verify HTTP signature with actor's public key
	_, err = VerifyRequest(r, remoteActor.PublicKeyPem)
	if err != nil {
		log.Printf("Inbox: Signature verification failed: %v", err)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Store activity in database
	database := db.GetDB()

	// Extract ObjectURI from the activity's object field
	objectURI := ""
	if activity.Object != nil {
		switch obj := activity.Object.(type) {
		case string:
			// Object is a simple URI string (like in Follow, Undo, etc.)
			objectURI = obj
		case map[string]interface{}:
			// Object is a full object (like in Create, Update)
			if id, ok := obj["id"].(string); ok {
				objectURI = id
			}
		}
	}

	activityRecord := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  activity.ID,
		ActivityType: activity.Type,
		ActorURI:     activity.Actor,
		ObjectURI:    objectURI,
		RawJSON:      string(body),
		Processed:    false,
		Local:        false,
		CreatedAt:    time.Now(),
	}

	if err := database.CreateActivity(activityRecord); err != nil {
		log.Printf("Inbox: Failed to store activity: %v", err)
		// Don't fail the request, we'll process it anyway
	}

	// Process activity based on type
	switch activity.Type {
	case "Follow":
		if err := handleFollowActivity(body, username, remoteActor, conf); err != nil {
			log.Printf("Inbox: Failed to handle Follow: %v", err)
			http.Error(w, "Failed to process Follow", http.StatusInternalServerError)
			return
		}
	case "Undo":
		if err := handleUndoActivity(body, username, remoteActor); err != nil {
			log.Printf("Inbox: Failed to handle Undo: %v", err)
			http.Error(w, "Failed to process Undo", http.StatusInternalServerError)
			return
		}
	case "Create":
		if err := handleCreateActivity(body, username); err != nil {
			log.Printf("Inbox: Failed to handle Create: %v", err)
			http.Error(w, "Failed to process Create", http.StatusInternalServerError)
			return
		}
	case "Like":
		if err := handleLikeActivity(body, username); err != nil {
			log.Printf("Inbox: Failed to handle Like: %v", err)
			http.Error(w, "Failed to process Like", http.StatusInternalServerError)
			return
		}
	case "Accept":
		// Accept activities are confirmations of Follow requests
		if err := handleAcceptActivity(body, username); err != nil {
			log.Printf("Inbox: Failed to handle Accept: %v", err)
			// Don't fail the request
		}
	case "Update":
		if err := handleUpdateActivity(body, username); err != nil {
			log.Printf("Inbox: Failed to handle Update: %v", err)
			http.Error(w, "Failed to process Update", http.StatusInternalServerError)
			return
		}
	case "Delete":
		if err := handleDeleteActivity(body, username); err != nil {
			log.Printf("Inbox: Failed to handle Delete: %v", err)
			http.Error(w, "Failed to process Delete", http.StatusInternalServerError)
			return
		}
	default:
		log.Printf("Inbox: Unsupported activity type: %s", activity.Type)
	}

	// Mark activity as processed
	activityRecord.Processed = true
	database.UpdateActivity(activityRecord)

	// Return 202 Accepted
	w.WriteHeader(http.StatusAccepted)
}

// handleFollowActivity processes a Follow activity
func handleFollowActivity(body []byte, username string, remoteActor *domain.RemoteAccount, conf *util.AppConfig) error {
	var follow FollowActivity
	if err := json.Unmarshal(body, &follow); err != nil {
		return fmt.Errorf("failed to parse Follow activity: %w", err)
	}

	log.Printf("Inbox: Processing Follow from %s@%s", remoteActor.Username, remoteActor.Domain)

	// Get local account
	database := db.GetDB()
	err, localAccount := database.ReadAccByUsername(username)
	if err != nil {
		return fmt.Errorf("local account not found: %w", err)
	}

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

	// Send Accept activity
	if err := SendAccept(localAccount, remoteActor, follow.ID, conf); err != nil {
		return fmt.Errorf("failed to send Accept: %w", err)
	}

	log.Printf("Inbox: Accepted follow from %s@%s", remoteActor.Username, remoteActor.Domain)
	return nil
}

// handleUndoActivity processes an Undo activity (e.g., Undo Follow)
func handleUndoActivity(body []byte, username string, remoteActor *domain.RemoteAccount) error {
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
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(undo.Object, &obj); err != nil {
		return fmt.Errorf("failed to parse Undo object: %w", err)
	}

	if obj.Type == "Follow" {
		// Delete the follow relationship
		database := db.GetDB()
		if err := database.DeleteFollowByURI(obj.ID); err != nil {
			return fmt.Errorf("failed to delete follow: %w", err)
		}
		log.Printf("Inbox: Removed follow from %s@%s", remoteActor.Username, remoteActor.Domain)
	}

	return nil
}

// handleCreateActivity processes a Create activity (incoming post/note)
func handleCreateActivity(body []byte, username string) error {
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
		} `json:"object"`
	}

	if err := json.Unmarshal(body, &create); err != nil {
		return fmt.Errorf("failed to parse Create activity: %w", err)
	}

	log.Printf("Inbox: Received post from %s", create.Actor)

	// Validate that we follow this actor (prevent spam)
	database := db.GetDB()

	// Get the local account
	err, localAccount := database.ReadAccByUsername(username)
	if err != nil {
		log.Printf("Inbox: Failed to get local account %s: %v", username, err)
		return fmt.Errorf("failed to get local account: %w", err)
	}
	log.Printf("Inbox: Local account: %s (ID: %s)", localAccount.Username, localAccount.Id)

	// Get the remote actor
	err, remoteActor := database.ReadRemoteAccountByActorURI(create.Actor)
	if err != nil || remoteActor == nil {
		log.Printf("Inbox: Rejecting Create from unknown actor %s (not cached)", create.Actor)
		return fmt.Errorf("unknown actor")
	}
	log.Printf("Inbox: Remote actor: %s@%s (ID: %s)", remoteActor.Username, remoteActor.Domain, remoteActor.Id)

	// Check if we follow this actor
	err, follow := database.ReadFollowByAccountIds(localAccount.Id, remoteActor.Id)
	if err != nil || follow == nil {
		log.Printf("Inbox: Rejecting Create from %s - not following (err: %v, follow: %v)", create.Actor, err, follow)
		return fmt.Errorf("not following this actor")
	}

	log.Printf("Inbox: Accepted post from followed user %s@%s (follow accepted: %v)", remoteActor.Username, remoteActor.Domain, follow.Accepted)

	// Use the activity ID, not the object ID
	activityURI := create.ID
	if activityURI == "" {
		// Fallback to object ID if activity ID is missing
		activityURI = create.Object.ID
	}

	// Check if we already have this activity
	err, existingActivity := database.ReadActivityByURI(activityURI)
	if err == nil && existingActivity != nil {
		log.Printf("Inbox: Activity %s already exists, skipping", activityURI)
		return nil
	}

	// Store the incoming post activity
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  activityURI,
		ActivityType: "Create",
		ActorURI:     create.Actor,
		ObjectURI:    create.Object.ID,
		RawJSON:      string(body),
		Processed:    true,
		Local:        false,
		CreatedAt:    time.Now(),
	}

	if err := database.CreateActivity(activity); err != nil {
		log.Printf("Inbox: Failed to store Create activity: %v", err)
		// Don't fail the request
	}

	return nil
}

// handleLikeActivity processes a Like activity
func handleLikeActivity(body []byte, username string) error {
	log.Printf("Inbox: Processing Like activity for %s", username)
	// TODO: Store like in likes table
	return nil
}

// handleAcceptActivity processes an Accept activity (response to Follow)
func handleAcceptActivity(body []byte, username string) error {
	var accept struct {
		Type   string          `json:"type"`
		Actor  string          `json:"actor"`
		Object json.RawMessage `json:"object"`
	}

	if err := json.Unmarshal(body, &accept); err != nil {
		return fmt.Errorf("failed to parse Accept activity: %w", err)
	}

	// Parse the embedded Follow object to get the follow ID
	var followObj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(accept.Object, &followObj); err != nil {
		return fmt.Errorf("failed to parse Accept object: %w", err)
	}

	// Update the follow to accepted=true
	database := db.GetDB()
	if err := database.AcceptFollowByURI(followObj.ID); err != nil {
		return fmt.Errorf("failed to accept follow: %w", err)
	}

	log.Printf("Inbox: Follow %s was accepted by %s", followObj.ID, accept.Actor)
	return nil
}

// handleUpdateActivity processes an Update activity (e.g., profile updates, post edits)
func handleUpdateActivity(body []byte, username string) error {
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

	database := db.GetDB()

	switch objectType.Type {
	case "Person":
		// Profile update - re-fetch and update cached actor
		remoteActor, err := GetOrFetchActor(update.Actor)
		if err != nil {
			return fmt.Errorf("failed to fetch updated actor: %w", err)
		}
		log.Printf("Inbox: Updated profile for %s@%s", remoteActor.Username, remoteActor.Domain)

	case "Note", "Article":
		// Post edit - find the existing activity that contains this Note/Article
		// The activity is stored with the Create activity ID, but we need to find it by the Note ID
		err, existingActivity := database.ReadActivityByObjectURI(objectType.ID)
		if err != nil || existingActivity == nil {
			log.Printf("Inbox: Note/Article %s not found for update, ignoring", objectType.ID)
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
	var delete struct {
		ID     string      `json:"id"`
		Type   string      `json:"type"`
		Actor  string      `json:"actor"`
		Object interface{} `json:"object"`
	}

	if err := json.Unmarshal(body, &delete); err != nil {
		return fmt.Errorf("failed to parse Delete activity: %w", err)
	}

	database := db.GetDB()

	// Object can be either a string URI or an embedded object
	var objectURI string
	switch obj := delete.Object.(type) {
	case string:
		objectURI = obj
	case map[string]interface{}:
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
		database := db.GetDB()
		err, activity := database.ReadActivityByObjectURI(objectURI)
		if err != nil || activity == nil {
			log.Printf("Inbox: Activity with object %s not found for deletion, ignoring", objectURI)
			return nil
		}

		// Delete the activity from the database
		if err := database.DeleteActivity(activity.Id); err != nil {
			return fmt.Errorf("failed to delete activity: %w", err)
		}
		log.Printf("Inbox: Deleted activity containing object %s", objectURI)
	}

	return nil
}
