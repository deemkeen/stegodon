package web

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"strings"
)

type action uint

const (
	id action = iota
	inbox
	outbox
	followers
	following
	sharedInbox
)

func GetActor(actor string, conf *util.AppConfig) (error, string) {
	err, acc := db.GetDB().ReadAccByUsername(actor)
	if err != nil {
		return err, "{}"
	}

	username := acc.Username
	pubKey := strings.Replace(acc.WebPublicKey, "\n", "\\n", -1)

	// Use DisplayName if available, otherwise use username
	displayName := acc.DisplayName
	if displayName == "" {
		displayName = username
	}

	// Escape any quotes in summary for JSON
	summary := strings.Replace(acc.Summary, "\"", "\\\"", -1)
	summary = strings.Replace(summary, "\n", "\\n", -1)

	return nil, fmt.Sprintf(
		`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],

					"id": "%s",
					"type": "Person",
					"preferredUsername": "%s",
					"name" : "%s",
					"summary": "%s",
					"inbox": "%s",
					"outbox": "%s",
					"followers": "%s",
					"following": "%s",
					"url": "%s",
  					"manuallyApprovesFollowers": false,
					"discoverable": true,
  					"endpoints": {
    					"sharedInbox": "%s"
  					},
					"publicKey": {
						"id": "%s#main-key",
						"owner": "%s",
						"publicKeyPem": "%s"
					}
				}`,
		getIRI(conf.Conf.SslDomain, username, id),
		username, displayName, summary,
		getIRI(conf.Conf.SslDomain, username, inbox),
		getIRI(conf.Conf.SslDomain, username, outbox),
		getIRI(conf.Conf.SslDomain, username, followers),
		getIRI(conf.Conf.SslDomain, username, following),
		getIRI(conf.Conf.SslDomain, username, id),
		getIRI(conf.Conf.SslDomain, username, sharedInbox),
		getIRI(conf.Conf.SslDomain, username, id),
		getIRI(conf.Conf.SslDomain, username, id), pubKey)
}

func getIRI(domain string, username string, action action) string {

	prefix := fmt.Sprintf("https://%s/users/%s", domain, username)
	switch action {
	case inbox:
		return fmt.Sprintf("%s/inbox", prefix)
	case outbox:
		return fmt.Sprintf("%s/outbox", prefix)
	case followers:
		return fmt.Sprintf("%s/followers", prefix)
	case following:
		return fmt.Sprintf("%s/following", prefix)
	case id:
		return prefix
	case sharedInbox:
		return fmt.Sprintf("https://%s/inbox", domain)
	default:
		return ""
	}
}

// GetNoteObject returns a Note object as ActivityPub JSON
func GetNoteObject(noteId uuid.UUID, conf *util.AppConfig) (error, string) {
	database := db.GetDB()
	err, note := database.ReadNoteId(noteId)
	if err != nil {
		return err, "{}"
	}

	// Get the account to build actor URI
	err, account := database.ReadAccByUsername(note.CreatedBy)
	if err != nil {
		return err, "{}"
	}

	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, account.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, note.Id.String())

	// Build the Note object
	noteObj := map[string]interface{}{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           noteURI,
		"type":         "Note",
		"attributedTo": actorURI,
		"content":      note.Message,
		"published":    note.CreatedAt.Format(time.RFC3339),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": []string{
			fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, account.Username),
		},
	}

	// Add updated field if note was edited
	if note.EditedAt != nil {
		noteObj["updated"] = note.EditedAt.Format(time.RFC3339)
	}

	jsonBytes, err := json.Marshal(noteObj)
	if err != nil {
		return err, "{}"
	}

	return nil, string(jsonBytes)
}
