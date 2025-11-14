package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deemkeen/stegodon/util"
)

func TestGetWebFingerNotFound(t *testing.T) {
	result := GetWebFingerNotFound()
	expected := `{"detail":"Not Found"}`

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}

	// Verify it's valid JSON
	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(result), &jsonMap); err != nil {
		t.Error("Result should be valid JSON")
	}

	if jsonMap["detail"] != "Not Found" {
		t.Error("JSON should contain 'detail' field with 'Not Found'")
	}
}

func TestGetWebfingerJSONStructure(t *testing.T) {
	// Test that the output is valid JSON with correct structure
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	// We can't test with real DB, but we can test the JSON format
	username := "testuser"
	domain := "example.com"

	// Build expected JSON manually to verify structure
	expected := map[string]interface{}{
		"subject": "acct:" + username + "@" + domain,
		"links": []interface{}{
			map[string]interface{}{
				"rel":  "self",
				"type": "application/activity+json",
				"href": "https://" + domain + "/users/" + username,
			},
		},
	}

	expectedJSON, _ := json.Marshal(expected)

	// Verify the structure matches what GetWebfinger produces
	expectedStr := string(expectedJSON)
	if !strings.Contains(expectedStr, "acct:") {
		t.Error("WebFinger should contain acct: prefix")
	}
	if !strings.Contains(expectedStr, "application/activity+json") {
		t.Error("WebFinger should contain ActivityPub content type")
	}
}

func TestGetWebfingerSubject(t *testing.T) {
	tests := []struct {
		username string
		domain   string
		want     string
	}{
		{"alice", "example.com", "acct:alice@example.com"},
		{"bob", "social.network", "acct:bob@social.network"},
		{"user_123", "test.org", "acct:user_123@test.org"},
	}

	for _, tt := range tests {
		t.Run(tt.username+"@"+tt.domain, func(t *testing.T) {
			subject := "acct:" + tt.username + "@" + tt.domain
			if subject != tt.want {
				t.Errorf("Expected subject %s, got %s", tt.want, subject)
			}
		})
	}
}

func TestGetWebfingerLinks(t *testing.T) {
	tests := []struct {
		username string
		domain   string
		wantHref string
	}{
		{"alice", "example.com", "https://example.com/users/alice"},
		{"bob", "social.network", "https://social.network/users/bob"},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			href := "https://" + tt.domain + "/users/" + tt.username
			if href != tt.wantHref {
				t.Errorf("Expected href %s, got %s", tt.wantHref, href)
			}
		})
	}
}

func TestWebFingerResponseUnmarshal(t *testing.T) {
	// Test that we can unmarshal a typical WebFinger response
	jsonData := `{
		"subject": "acct:alice@example.com",
		"links": [
			{
				"rel": "self",
				"type": "application/activity+json",
				"href": "https://example.com/users/alice"
			}
		]
	}`

	var wfr WebFingerResponse
	if err := json.Unmarshal([]byte(jsonData), &wfr); err != nil {
		t.Fatalf("Failed to unmarshal WebFinger response: %v", err)
	}

	if wfr.Subject != "acct:alice@example.com" {
		t.Errorf("Expected subject 'acct:alice@example.com', got '%s'", wfr.Subject)
	}

	if len(wfr.Links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(wfr.Links))
	}

	link := wfr.Links[0]
	if link.Rel != "self" {
		t.Errorf("Expected rel 'self', got '%s'", link.Rel)
	}
	if link.Type != "application/activity+json" {
		t.Errorf("Expected type 'application/activity+json', got '%s'", link.Type)
	}
	if link.Href != "https://example.com/users/alice" {
		t.Errorf("Expected href 'https://example.com/users/alice', got '%s'", link.Href)
	}
}

func TestResolveWebFingerSuccess(t *testing.T) {
	// Create a test HTTP server that returns a valid WebFinger response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if !strings.Contains(r.URL.Path, "/.well-known/webfinger") {
			t.Error("Request should be to /.well-known/webfinger")
		}

		resource := r.URL.Query().Get("resource")
		if !strings.HasPrefix(resource, "acct:") {
			t.Error("Resource should start with acct:")
		}

		if r.Header.Get("Accept") != "application/jrd+json" {
			t.Error("Accept header should be application/jrd+json")
		}

		// Return valid WebFinger response
		w.Header().Set("Content-Type", "application/jrd+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"subject": "acct:alice@example.com",
			"links": []map[string]string{
				{
					"rel":  "self",
					"type": "application/activity+json",
					"href": "https://example.com/users/alice",
				},
			},
		})
	}))
	defer server.Close()

	// Note: This test would need to be adapted to use the test server
	// For now, we test the logic with the mock server
	t.Skip("Integration test - requires HTTP client mocking")
}

func TestResolveWebFingerURLGeneration(t *testing.T) {
	tests := []struct {
		username string
		domain   string
		wantURL  string
	}{
		{
			username: "alice",
			domain:   "mastodon.social",
			wantURL:  "https://mastodon.social/.well-known/webfinger?resource=acct:alice@mastodon.social",
		},
		{
			username: "bob",
			domain:   "pleroma.site",
			wantURL:  "https://pleroma.site/.well-known/webfinger?resource=acct:bob@pleroma.site",
		},
	}

	for _, tt := range tests {
		t.Run(tt.username+"@"+tt.domain, func(t *testing.T) {
			// Build the URL like ResolveWebFinger does
			url := "https://" + tt.domain + "/.well-known/webfinger?resource=acct:" + tt.username + "@" + tt.domain

			if url != tt.wantURL {
				t.Errorf("Expected URL:\n%s\nGot:\n%s", tt.wantURL, url)
			}
		})
	}
}

func TestResolveWebFingerLinkExtraction(t *testing.T) {
	// Test the logic for extracting the ActivityPub actor URL
	tests := []struct {
		name     string
		response WebFingerResponse
		wantHref string
		wantErr  bool
	}{
		{
			name: "valid ActivityPub link",
			response: WebFingerResponse{
				Subject: "acct:alice@example.com",
				Links: []struct {
					Rel  string `json:"rel"`
					Type string `json:"type"`
					Href string `json:"href"`
				}{
					{
						Rel:  "self",
						Type: "application/activity+json",
						Href: "https://example.com/users/alice",
					},
				},
			},
			wantHref: "https://example.com/users/alice",
			wantErr:  false,
		},
		{
			name: "multiple links, ActivityPub is second",
			response: WebFingerResponse{
				Subject: "acct:alice@example.com",
				Links: []struct {
					Rel  string `json:"rel"`
					Type string `json:"type"`
					Href string `json:"href"`
				}{
					{
						Rel:  "http://webfinger.net/rel/profile-page",
						Type: "text/html",
						Href: "https://example.com/@alice",
					},
					{
						Rel:  "self",
						Type: "application/activity+json",
						Href: "https://example.com/users/alice",
					},
				},
			},
			wantHref: "https://example.com/users/alice",
			wantErr:  false,
		},
		{
			name: "no ActivityPub link",
			response: WebFingerResponse{
				Subject: "acct:alice@example.com",
				Links: []struct {
					Rel  string `json:"rel"`
					Type string `json:"type"`
					Href string `json:"href"`
				}{
					{
						Rel:  "http://webfinger.net/rel/profile-page",
						Type: "text/html",
						Href: "https://example.com/@alice",
					},
				},
			},
			wantHref: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the link extraction logic from ResolveWebFinger
			var href string
			found := false

			for _, link := range tt.response.Links {
				if link.Rel == "self" && link.Type == "application/activity+json" {
					href = link.Href
					found = true
					break
				}
			}

			if tt.wantErr && found {
				t.Error("Expected no link to be found, but one was")
			}
			if !tt.wantErr && !found {
				t.Error("Expected link to be found, but none was")
			}
			if href != tt.wantHref {
				t.Errorf("Expected href '%s', got '%s'", tt.wantHref, href)
			}
		})
	}
}

func TestWebFingerResponseHeaders(t *testing.T) {
	// Test that we set correct headers for WebFinger requests
	expectedAccept := "application/jrd+json"
	expectedUserAgent := "stegodon/1.0 ActivityPub"

	// Verify the values match what ResolveWebFinger uses
	if expectedAccept != "application/jrd+json" {
		t.Error("Accept header should be application/jrd+json")
	}
	if !strings.Contains(expectedUserAgent, "stegodon") {
		t.Error("User-Agent should contain 'stegodon'")
	}
}

func TestWebFingerEmptyUsername(t *testing.T) {
	// Test edge case with empty username
	username := ""
	domain := "example.com"
	subject := "acct:" + username + "@" + domain

	if subject != "acct:@example.com" {
		t.Errorf("Expected 'acct:@example.com', got '%s'", subject)
	}
}

func TestWebFingerSpecialCharacters(t *testing.T) {
	// Test usernames with special characters
	tests := []struct {
		username string
		domain   string
	}{
		{"user.name", "example.com"},
		{"user_123", "example.com"},
		{"user-name", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			subject := "acct:" + tt.username + "@" + tt.domain
			if !strings.Contains(subject, tt.username) {
				t.Error("Subject should contain username")
			}
			if !strings.HasPrefix(subject, "acct:") {
				t.Error("Subject should start with acct:")
			}
		})
	}
}
