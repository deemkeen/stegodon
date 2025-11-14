package activitypub

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestActorResponseUnmarshal(t *testing.T) {
	// Test unmarshaling a typical ActivityPub actor response
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://mastodon.social/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"name": "Alice Example",
		"summary": "Just a test user",
		"inbox": "https://mastodon.social/users/alice/inbox",
		"outbox": "https://mastodon.social/users/alice/outbox",
		"icon": {
			"type": "Image",
			"mediaType": "image/png",
			"url": "https://mastodon.social/avatars/alice.png"
		},
		"publicKey": {
			"id": "https://mastodon.social/users/alice#main-key",
			"owner": "https://mastodon.social/users/alice",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal ActorResponse: %v", err)
	}

	// Verify required fields
	if actor.ID != "https://mastodon.social/users/alice" {
		t.Errorf("Expected ID 'https://mastodon.social/users/alice', got '%s'", actor.ID)
	}
	if actor.Type != "Person" {
		t.Errorf("Expected Type 'Person', got '%s'", actor.Type)
	}
	if actor.PreferredUsername != "alice" {
		t.Errorf("Expected PreferredUsername 'alice', got '%s'", actor.PreferredUsername)
	}
	if actor.Name != "Alice Example" {
		t.Errorf("Expected Name 'Alice Example', got '%s'", actor.Name)
	}
	if actor.Summary != "Just a test user" {
		t.Errorf("Expected Summary 'Just a test user', got '%s'", actor.Summary)
	}
	if actor.Inbox != "https://mastodon.social/users/alice/inbox" {
		t.Errorf("Expected Inbox URL, got '%s'", actor.Inbox)
	}
	if actor.Outbox != "https://mastodon.social/users/alice/outbox" {
		t.Errorf("Expected Outbox URL, got '%s'", actor.Outbox)
	}
	if actor.Icon.URL != "https://mastodon.social/avatars/alice.png" {
		t.Errorf("Expected Icon URL, got '%s'", actor.Icon.URL)
	}
	if !strings.Contains(actor.PublicKey.PublicKeyPem, "BEGIN PUBLIC KEY") {
		t.Error("PublicKeyPem should contain PEM header")
	}
}

func TestActorResponseMinimal(t *testing.T) {
	// Test with minimal required fields
	jsonData := `{
		"id": "https://example.com/users/bob",
		"type": "Person",
		"preferredUsername": "bob",
		"inbox": "https://example.com/users/bob/inbox",
		"publicKey": {
			"id": "https://example.com/users/bob#main-key",
			"owner": "https://example.com/users/bob",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal minimal actor: %v", err)
	}

	// Verify required fields are present
	if actor.ID == "" {
		t.Error("ID should not be empty")
	}
	if actor.Inbox == "" {
		t.Error("Inbox should not be empty")
	}
	if actor.PublicKey.PublicKeyPem == "" {
		t.Error("PublicKeyPem should not be empty")
	}
}

func TestActorResponseValidation(t *testing.T) {
	// Test validation logic for required fields
	tests := []struct {
		name      string
		actor     ActorResponse
		wantValid bool
	}{
		{
			name: "valid actor",
			actor: ActorResponse{
				ID:    "https://example.com/users/alice",
				Inbox: "https://example.com/users/alice/inbox",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			wantValid: true,
		},
		{
			name: "missing ID",
			actor: ActorResponse{
				Inbox: "https://example.com/inbox",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			wantValid: false,
		},
		{
			name: "missing Inbox",
			actor: ActorResponse{
				ID: "https://example.com/users/alice",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			wantValid: false,
		},
		{
			name: "missing PublicKey",
			actor: ActorResponse{
				ID:    "https://example.com/users/alice",
				Inbox: "https://example.com/users/alice/inbox",
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the validation logic from FetchRemoteActor
			isValid := tt.actor.ID != "" && tt.actor.Inbox != "" && tt.actor.PublicKey.PublicKeyPem != ""

			if isValid != tt.wantValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.wantValid, isValid)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name       string
		actorURI   string
		wantDomain string
		wantError  bool
	}{
		{
			name:       "Mastodon user",
			actorURI:   "https://mastodon.social/users/alice",
			wantDomain: "mastodon.social",
			wantError:  false,
		},
		{
			name:       "Pleroma user",
			actorURI:   "https://pleroma.site/users/bob",
			wantDomain: "pleroma.site",
			wantError:  false,
		},
		{
			name:       "Custom port",
			actorURI:   "https://social.example.com:8080/users/charlie",
			wantDomain: "social.example.com:8080",
			wantError:  false,
		},
		{
			name:       "Subdomain",
			actorURI:   "https://masto.subdomain.example.com/users/dave",
			wantDomain: "masto.subdomain.example.com",
			wantError:  false,
		},
		{
			name:       "Invalid URI",
			actorURI:   "://invalid",
			wantDomain: "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := extractDomain(tt.actorURI)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if domain != tt.wantDomain {
				t.Errorf("Expected domain '%s', got '%s'", tt.wantDomain, domain)
			}
		})
	}
}

func TestExtractUsername(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		wantUsername string
	}{
		{
			name:         "standard users path",
			uri:          "https://mastodon.social/users/alice",
			wantUsername: "alice",
		},
		{
			name:         "@ prefix path",
			uri:          "https://mastodon.social/@bob",
			wantUsername: "bob",
		},
		{
			name:         "activity path",
			uri:          "https://example.com/users/charlie/statuses/123",
			wantUsername: "123",
		},
		{
			name:         "simple path",
			uri:          "https://example.com/dave",
			wantUsername: "dave",
		},
		{
			name:         "empty uri",
			uri:          "",
			wantUsername: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username := extractUsername(tt.uri)
			if username != tt.wantUsername {
				t.Errorf("Expected username '%s', got '%s'", tt.wantUsername, username)
			}
		})
	}
}

func TestActorContextVariants(t *testing.T) {
	// Test different @context formats
	tests := []struct {
		name        string
		contextJSON string
	}{
		{
			name:        "string context",
			contextJSON: `"https://www.w3.org/ns/activitystreams"`,
		},
		{
			name:        "array context",
			contextJSON: `["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"]`,
		},
		{
			name:        "complex context",
			contextJSON: `[{"@vocab": "https://www.w3.org/ns/activitystreams"}, "https://w3id.org/security/v1"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := `{
				"@context": ` + tt.contextJSON + `,
				"id": "https://example.com/users/test",
				"type": "Person",
				"inbox": "https://example.com/inbox",
				"publicKey": {
					"publicKeyPem": "test"
				}
			}`

			var actor ActorResponse
			if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
				t.Fatalf("Failed to unmarshal actor with %s: %v", tt.name, err)
			}

			if actor.ID != "https://example.com/users/test" {
				t.Error("Actor fields should be parsed correctly regardless of context format")
			}
		})
	}
}

func TestActorTypeVariants(t *testing.T) {
	// Test different actor types
	actorTypes := []string{"Person", "Application", "Service", "Organization", "Group"}

	for _, actorType := range actorTypes {
		t.Run(actorType, func(t *testing.T) {
			jsonData := `{
				"id": "https://example.com/actor",
				"type": "` + actorType + `",
				"inbox": "https://example.com/inbox",
				"publicKey": {"publicKeyPem": "test"}
			}`

			var actor ActorResponse
			if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
				t.Fatalf("Failed to unmarshal %s actor: %v", actorType, err)
			}

			if actor.Type != actorType {
				t.Errorf("Expected Type '%s', got '%s'", actorType, actor.Type)
			}
		})
	}
}

func TestActorIconVariants(t *testing.T) {
	// Test different icon formats
	tests := []struct {
		name     string
		iconJSON string
		wantURL  string
	}{
		{
			name: "full icon object",
			iconJSON: `{
				"type": "Image",
				"mediaType": "image/png",
				"url": "https://example.com/avatar.png"
			}`,
			wantURL: "https://example.com/avatar.png",
		},
		{
			name:     "missing icon",
			iconJSON: `{}`,
			wantURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := `{
				"id": "https://example.com/actor",
				"type": "Person",
				"inbox": "https://example.com/inbox",
				"icon": ` + tt.iconJSON + `,
				"publicKey": {"publicKeyPem": "test"}
			}`

			var actor ActorResponse
			if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if actor.Icon.URL != tt.wantURL {
				t.Errorf("Expected icon URL '%s', got '%s'", tt.wantURL, actor.Icon.URL)
			}
		})
	}
}

func TestPublicKeyStructure(t *testing.T) {
	// Test PublicKey structure
	jsonData := `{
		"id": "https://example.com/actor",
		"type": "Person",
		"inbox": "https://example.com/inbox",
		"publicKey": {
			"id": "https://example.com/actor#main-key",
			"owner": "https://example.com/actor",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if actor.PublicKey.ID != "https://example.com/actor#main-key" {
		t.Errorf("Expected publicKey.id, got '%s'", actor.PublicKey.ID)
	}
	if actor.PublicKey.Owner != "https://example.com/actor" {
		t.Errorf("Expected publicKey.owner, got '%s'", actor.PublicKey.Owner)
	}
	if !strings.Contains(actor.PublicKey.PublicKeyPem, "BEGIN PUBLIC KEY") {
		t.Error("PublicKeyPem should contain PEM markers")
	}
	if !strings.Contains(actor.PublicKey.PublicKeyPem, "END PUBLIC KEY") {
		t.Error("PublicKeyPem should contain END marker")
	}
}

func TestActorEndpointURLs(t *testing.T) {
	// Test that all endpoint URLs are properly parsed
	jsonData := `{
		"id": "https://mastodon.social/users/alice",
		"type": "Person",
		"inbox": "https://mastodon.social/users/alice/inbox",
		"outbox": "https://mastodon.social/users/alice/outbox",
		"following": "https://mastodon.social/users/alice/following",
		"followers": "https://mastodon.social/users/alice/followers",
		"publicKey": {"publicKeyPem": "test"}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all URLs start with HTTPS
	urls := []string{actor.ID, actor.Inbox, actor.Outbox}
	for _, url := range urls {
		if url != "" && !strings.HasPrefix(url, "https://") {
			t.Errorf("URL should use HTTPS: %s", url)
		}
	}

	// Verify inbox and outbox are different
	if actor.Inbox == actor.Outbox && actor.Inbox != "" {
		t.Error("Inbox and Outbox should typically be different endpoints")
	}
}

func TestActorDisplayNameHandling(t *testing.T) {
	// Test preferredUsername vs name (display name)
	jsonData := `{
		"id": "https://example.com/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"name": "Alice Wonderland 🎭",
		"inbox": "https://example.com/inbox",
		"publicKey": {"publicKeyPem": "test"}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if actor.PreferredUsername != "alice" {
		t.Error("PreferredUsername should be simple username")
	}
	if actor.Name != "Alice Wonderland 🎭" {
		t.Error("Name should support display names with emoji")
	}
	if actor.PreferredUsername == actor.Name {
		t.Error("Username and display name should be different fields")
	}
}

func TestActorSummaryHandling(t *testing.T) {
	// Test summary/bio field with HTML
	jsonData := `{
		"id": "https://example.com/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"summary": "<p>Software developer interested in <a href=\"https://example.com/tags/golang\">#golang</a> and <a href=\"https://example.com/tags/activitypub\">#activitypub</a></p>",
		"inbox": "https://example.com/inbox",
		"publicKey": {"publicKeyPem": "test"}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !strings.Contains(actor.Summary, "<p>") {
		t.Error("Summary should preserve HTML tags")
	}
	if !strings.Contains(actor.Summary, "golang") {
		t.Error("Summary should contain content")
	}
}

func TestCacheFreshnessLogic(t *testing.T) {
	// Test the 24-hour cache freshness logic
	tests := []struct {
		name      string
		age       time.Duration
		wantFresh bool
	}{
		{
			name:      "just fetched",
			age:       1 * time.Minute,
			wantFresh: true,
		},
		{
			name:      "12 hours old",
			age:       12 * time.Hour,
			wantFresh: true,
		},
		{
			name:      "23 hours old",
			age:       23 * time.Hour,
			wantFresh: true,
		},
		{
			name:      "25 hours old",
			age:       25 * time.Hour,
			wantFresh: false,
		},
		{
			name:      "48 hours old",
			age:       48 * time.Hour,
			wantFresh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastFetched := time.Now().Add(-tt.age)
			isFresh := time.Since(lastFetched) < 24*time.Hour

			if isFresh != tt.wantFresh {
				t.Errorf("Expected fresh=%v for age %v, got fresh=%v", tt.wantFresh, tt.age, isFresh)
			}
		})
	}
}
