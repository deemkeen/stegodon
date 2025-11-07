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
func HandleInbox(w http.ResponseWriter, r *http.Request, username string) {
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
	activityRecord := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  activity.ID,
		ActivityType: activity.Type,
		ActorURI:     activity.Actor,
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
		if err := handleFollowActivity(body, username, remoteActor); err != nil {
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
func handleFollowActivity(body []byte, username string, remoteActor *domain.RemoteAccount) error {
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
	followRecord := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             follow.ID,
		Accepted:        true, // Auto-accept for now
		CreatedAt:       time.Now(),
	}

	if err := database.CreateFollow(followRecord); err != nil {
		return fmt.Errorf("failed to create follow: %w", err)
	}

	// Send Accept activity
	// TODO: Pass proper config from router
	tempConf := &util.AppConfig{}
	tempConf.Conf.SslDomain = "example.com"
	if err := SendAccept(localAccount, remoteActor, follow.ID, tempConf); err != nil {
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

	log.Printf("Inbox: Received post from %s: %s", create.Actor, create.Object.Content)

	// Store the incoming post activity
	database := db.GetDB()
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  create.Object.ID,
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
