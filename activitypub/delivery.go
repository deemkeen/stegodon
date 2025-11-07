package activitypub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
)

// StartDeliveryWorker starts a background worker that processes the delivery queue
func StartDeliveryWorker(conf *util.AppConfig) {
	log.Println("Starting ActivityPub delivery worker...")

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			processDeliveryQueue(conf)
		}
	}()
}

// processDeliveryQueue processes pending deliveries from the queue
func processDeliveryQueue(conf *util.AppConfig) {
	database := db.GetDB()

	// Get pending deliveries (max 50 at a time)
	err, items := database.ReadPendingDeliveries(50)
	if err != nil {
		log.Printf("DeliveryWorker: Failed to read queue: %v", err)
		return
	}

	if items == nil || len(*items) == 0 {
		return
	}

	log.Printf("DeliveryWorker: Processing %d pending deliveries", len(*items))

	for _, item := range *items {
		if err := deliverActivity(&item, conf); err != nil {
			// Failed delivery - retry with exponential backoff
			item.Attempts++
			backoffMinutes := []int{1, 5, 15, 60, 240, 1440}[min(item.Attempts-1, 5)]
			item.NextRetryAt = time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

			if item.Attempts >= 10 {
				// Give up after 10 attempts
				log.Printf("DeliveryWorker: Giving up on delivery to %s after %d attempts", item.InboxURI, item.Attempts)
				database.DeleteDelivery(item.Id)
			} else {
				log.Printf("DeliveryWorker: Delivery to %s failed (attempt %d), retry in %dm: %v",
					item.InboxURI, item.Attempts, backoffMinutes, err)
				database.UpdateDeliveryAttempt(item.Id, item.Attempts, item.NextRetryAt)
			}
		} else {
			// Successful delivery - remove from queue
			log.Printf("DeliveryWorker: Successfully delivered to %s", item.InboxURI)
			database.DeleteDelivery(item.Id)
		}
	}
}

// deliverActivity attempts to deliver a single activity to an inbox
func deliverActivity(item *domain.DeliveryQueueItem, conf *util.AppConfig) error {
	// Parse the activity JSON
	var activity map[string]interface{}
	if err := json.Unmarshal([]byte(item.ActivityJSON), &activity); err != nil {
		return fmt.Errorf("failed to parse activity JSON: %w", err)
	}

	// Extract actor from activity
	actor, ok := activity["actor"].(string)
	if !ok {
		return fmt.Errorf("activity missing actor field")
	}

	// Extract username from actor URI
	// actor format: "https://example.com/users/alice"
	parts := strings.Split(actor, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid actor URI: %s", actor)
	}
	username := parts[len(parts)-1]

	// Get local account
	database := db.GetDB()
	err, localAccount := database.ReadAccByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to get local account: %w", err)
	}

	// Parse private key
	privateKey, err := ParsePrivateKey(localAccount.WebPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", item.InboxURI, bytes.NewReader([]byte(item.ActivityJSON)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	// Sign request
	keyID := fmt.Sprintf("https://%s/users/%s#main-key", conf.Conf.SslDomain, username)
	if err := SignRequest(req, privateKey, keyID); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote server returned status: %d", resp.StatusCode)
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
