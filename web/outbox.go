package web

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
)

// GetOutbox returns an ActivityPub OrderedCollection of a user's public posts
// This allows remote servers to discover posts without following the user
func GetOutbox(actor string, page int, conf *util.AppConfig) (error, string) {
	// Verify the account exists
	err, _ := db.GetDB().ReadAccByUsername(actor)
	if err != nil {
		log.Printf("GetOutbox: User %s not found: %v", actor, err)
		return err, "{}"
	}

	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)
	outboxURL := fmt.Sprintf("%s/users/%s/outbox", baseURL, actor)

	// If no page parameter, return the collection metadata
	if page == 0 {
		// Count total public posts
		err, notes := db.GetDB().ReadPublicNotesByUsername(actor, 999999, 0)
		if err != nil {
			log.Printf("GetOutbox: Failed to count notes for %s: %v", actor, err)
			return err, "{}"
		}
		totalItems := 0
		if notes != nil {
			totalItems = len(*notes)
		}

		collection := map[string]interface{}{
			"@context":   "https://www.w3.org/ns/activitystreams",
			"id":         outboxURL,
			"type":       "OrderedCollection",
			"totalItems": totalItems,
			"first":      fmt.Sprintf("%s?page=1", outboxURL),
		}

		jsonData, err := json.Marshal(collection)
		if err != nil {
			log.Printf("GetOutbox: Failed to marshal collection: %v", err)
			return err, "{}"
		}
		return nil, string(jsonData)
	}

	// Return a paginated collection page
	return getOutboxPage(actor, page, conf)
}

func getOutboxPage(actor string, page int, conf *util.AppConfig) (error, string) {
	itemsPerPage := 20
	offset := (page - 1) * itemsPerPage

	// Fetch notes for this page
	err, notes := db.GetDB().ReadPublicNotesByUsername(actor, itemsPerPage+1, offset)
	if err != nil {
		log.Printf("GetOutbox: Failed to fetch notes page %d for %s: %v", page, actor, err)
		return err, "{}"
	}

	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)
	outboxURL := fmt.Sprintf("%s/users/%s/outbox", baseURL, actor)
	pageURL := fmt.Sprintf("%s?page=%d", outboxURL, page)

	// Check if there are more items
	hasMore := false
	items := []interface{}{}

	if notes != nil {
		if len(*notes) > itemsPerPage {
			hasMore = true
			// Trim the extra item
			pageNotes := (*notes)[:itemsPerPage]
			items = makeNoteActivities(pageNotes, actor, conf)
		} else {
			items = makeNoteActivities(*notes, actor, conf)
		}
	}

	collectionPage := map[string]interface{}{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           pageURL,
		"type":         "OrderedCollectionPage",
		"partOf":       outboxURL,
		"orderedItems": items,
	}

	// Add next link if there are more pages
	if hasMore {
		collectionPage["next"] = fmt.Sprintf("%s?page=%d", outboxURL, page+1)
	}

	// Add prev link if not first page
	if page > 1 {
		collectionPage["prev"] = fmt.Sprintf("%s?page=%d", outboxURL, page-1)
	}

	jsonData, err := json.Marshal(collectionPage)
	if err != nil {
		log.Printf("GetOutbox: Failed to marshal collection page: %v", err)
		return err, "{}"
	}
	return nil, string(jsonData)
}

// makeNoteActivities converts domain.Note objects to ActivityPub Create activities
func makeNoteActivities(notes []domain.Note, actor string, conf *util.AppConfig) []interface{}{
	activities := make([]interface{}, 0, len(notes))
	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)

	for _, note := range notes {
		// Use object_uri if available, otherwise generate one
		objectURI := note.ObjectURI
		if objectURI == "" {
			objectURI = fmt.Sprintf("%s/notes/%s", baseURL, note.Id.String())
		}

		// Build the Note object
		noteObj := map[string]interface{}{
			"id":           objectURI,
			"type":         "Note",
			"attributedTo": fmt.Sprintf("%s/users/%s", baseURL, actor),
			"content":      note.Message,
			"published":    note.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"to": []string{
				"https://www.w3.org/ns/activitystreams#Public",
			},
			"cc": []string{
				fmt.Sprintf("%s/users/%s/followers", baseURL, actor),
			},
		}

		// Add updated field if note was edited
		if note.EditedAt != nil {
			noteObj["updated"] = note.EditedAt.Format("2006-01-02T15:04:05Z")
		}

		// Build the Create activity wrapping the Note
		activityURI := fmt.Sprintf("%s/activities/%s", baseURL, note.Id.String())
		activity := map[string]interface{}{
			"id":        activityURI,
			"type":      "Create",
			"actor":     fmt.Sprintf("%s/users/%s", baseURL, actor),
			"published": note.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"to": []string{
				"https://www.w3.org/ns/activitystreams#Public",
			},
			"cc": []string{
				fmt.Sprintf("%s/users/%s/followers", baseURL, actor),
			},
			"object": noteObj,
		}

		activities = append(activities, activity)
	}

	return activities
}

// ParsePageParam extracts the page parameter from a query string
func ParsePageParam(pageStr string) int {
	if pageStr == "" {
		return 0
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 0 {
		return 0
	}
	return page
}
