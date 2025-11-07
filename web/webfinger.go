package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
)

// GetWebfinger returns WebFinger JSON for a local user
func GetWebfinger(user string, conf *util.AppConfig) (error, string) {

	err, acc := db.GetDB().ReadAccByUsername(user)
	if err != nil {
		return err, GetWebFingerNotFound()
	}

	username := acc.Username

	return nil, fmt.Sprintf(
		`{
					"subject": "acct:%s@%s",

					"links": [
						{
							"rel": "self",
							"type": "application/activity+json",
							"href": "https://%s/users/%s"
						}
					]
				}`, username, conf.Conf.SslDomain,
		conf.Conf.SslDomain, username)
}

// GetWebFingerNotFound returns a 404 response
func GetWebFingerNotFound() string {
	return `{"detail":"Not Found"}`
}

// WebFingerResponse represents the response from a WebFinger query
type WebFingerResponse struct {
	Subject string `json:"subject"`
	Links   []struct {
		Rel  string `json:"rel"`
		Type string `json:"type"`
		Href string `json:"href"`
	} `json:"links"`
}

// ResolveWebFinger resolves a user@domain to an ActivityPub actor URI
// Example: ResolveWebFinger("alice", "mastodon.social") -> "https://mastodon.social/users/alice"
func ResolveWebFinger(username, domain string) (string, error) {
	webfingerURL := fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s",
		domain, username, domain)

	req, err := http.NewRequest("GET", webfingerURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/jrd+json")
	req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("webfinger request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("webfinger failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result WebFingerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse webfinger response: %w", err)
	}

	// Find self link with type application/activity+json
	for _, link := range result.Links {
		if link.Rel == "self" && link.Type == "application/activity+json" {
			return link.Href, nil
		}
	}

	return "", fmt.Errorf("no ActivityPub actor found in webfinger response")
}

