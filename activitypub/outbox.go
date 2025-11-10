package activitypub

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// SendActivity sends an activity to a remote inbox
func SendActivity(activity interface{}, inboxURI string, localAccount *domain.Account, conf *util.AppConfig) error {
	// Marshal activity to JSON
	activityJSON, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("failed to marshal activity: %w", err)
	}

	// Calculate digest for HTTP signature
	hash := sha256.Sum256(activityJSON)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])

	// Create HTTP request
	req, err := http.NewRequest("POST", inboxURI, bytes.NewReader(activityJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("Digest", digest)

	// Parse private key for signing
	privateKey, err := ParsePrivateKey(localAccount.WebPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Sign request
	keyID := fmt.Sprintf("https://%s/users/%s#main-key", conf.Conf.SslDomain, localAccount.Username)
	if err := SignRequest(req, privateKey, keyID); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote server returned status: %d", resp.StatusCode)
	}

	log.Printf("Outbox: Sent %T to %s (status: %d)", activity, inboxURI, resp.StatusCode)
	return nil
}

// SendAccept sends an Accept activity in response to a Follow
func SendAccept(localAccount *domain.Account, remoteActor *domain.RemoteAccount, followID string, conf *util.AppConfig) error {
	acceptID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	accept := map[string]interface{}{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       acceptID,
		"type":     "Accept",
		"actor":    actorURI,
		"object": map[string]interface{}{
			"id":     followID,
			"type":   "Follow",
			"actor":  remoteActor.ActorURI,
			"object": actorURI,
		},
	}

	return SendActivity(accept, remoteActor.InboxURI, localAccount, conf)
}

// SendCreate sends a Create activity for a new note
func SendCreate(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig) error {
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, note.Id.String())
	createID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())

	create := map[string]interface{}{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       createID,
		"type":     "Create",
		"actor":    actorURI,
		"published": note.CreatedAt.Format(time.RFC3339),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": []string{
			fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
		},
		"object": map[string]interface{}{
			"id":           noteURI,
			"type":         "Note",
			"attributedTo": actorURI,
			"content":      note.Message,
			"published":    note.CreatedAt.Format(time.RFC3339),
			"to": []string{
				"https://www.w3.org/ns/activitystreams#Public",
			},
			"cc": []string{
				fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
			},
		},
	}

	// Get all followers and queue delivery to their inboxes
	database := db.GetDB()
	err, followers := database.ReadFollowersByAccountId(localAccount.Id)
	if err != nil {
		log.Printf("Outbox: Failed to get followers: %v", err)
		return nil // Don't fail if we can't get followers
	}

	if followers == nil || len(*followers) == 0 {
		log.Printf("Outbox: No followers to deliver to")
		return nil
	}

	// Queue delivery to each follower's inbox
	for _, follower := range *followers {
		// AccountId is the follower (remote actor we need to deliver to)
		err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
		if err != nil {
			log.Printf("Outbox: Failed to get remote actor %s: %v", follower.AccountId, err)
			continue
		}

		// Queue for delivery
		queueItem := &domain.DeliveryQueueItem{
			Id:          uuid.New(),
			InboxURI:    remoteActor.InboxURI,
			ActivityJSON: mustMarshal(create),
			Attempts:    0,
			NextRetryAt: time.Now(),
			CreatedAt:   time.Now(),
		}

		if err := database.EnqueueDelivery(queueItem); err != nil {
			log.Printf("Outbox: Failed to queue delivery to %s: %v", remoteActor.InboxURI, err)
		}
	}

	log.Printf("Outbox: Queued Create activity for note %s to %d followers", note.Id, len(*followers))
	return nil
}

// SendFollow sends a Follow activity to a remote actor
func SendFollow(localAccount *domain.Account, remoteActorURI string, conf *util.AppConfig) error {
	// Fetch remote actor
	remoteActor, err := GetOrFetchActor(remoteActorURI)
	if err != nil {
		return fmt.Errorf("failed to fetch remote actor: %w", err)
	}

	followID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	follow := map[string]interface{}{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       followID,
		"type":     "Follow",
		"actor":    actorURI,
		"object":   remoteActorURI,
	}

	// Store follow relationship as pending
	database := db.GetDB()
	followRecord := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             followID,
		Accepted:        false, // Pending until Accept received
		CreatedAt:       time.Now(),
	}

	if err := database.CreateFollow(followRecord); err != nil {
		return fmt.Errorf("failed to store follow: %w", err)
	}

	// Send Follow activity
	return SendActivity(follow, remoteActor.InboxURI, localAccount, conf)
}

// mustMarshal marshals v to JSON, panicking on error
func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return string(b)
}
