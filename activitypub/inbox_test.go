package activitypub

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActivityUnmarshal(t *testing.T) {
	// Test basic Activity struct unmarshaling
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/activities/123",
		"type": "Follow",
		"actor": "https://example.com/users/alice",
		"object": "https://example.com/users/bob"
	}`

	var activity Activity
	if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
		t.Fatalf("Failed to unmarshal Activity: %v", err)
	}

	if activity.ID != "https://example.com/activities/123" {
		t.Errorf("Expected ID 'https://example.com/activities/123', got '%s'", activity.ID)
	}
	if activity.Type != "Follow" {
		t.Errorf("Expected Type 'Follow', got '%s'", activity.Type)
	}
	if activity.Actor != "https://example.com/users/alice" {
		t.Errorf("Expected Actor 'https://example.com/users/alice', got '%s'", activity.Actor)
	}
}

func TestActivityObjectAsString(t *testing.T) {
	// Test Activity with object as simple string URI
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/activities/456",
		"type": "Undo",
		"actor": "https://example.com/users/alice",
		"object": "https://example.com/activities/123"
	}`

	var activity Activity
	if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
		t.Fatalf("Failed to unmarshal Activity with string object: %v", err)
	}

	// Verify we can extract the object URI
	var objectURI string
	switch obj := activity.Object.(type) {
	case string:
		objectURI = obj
	}

	if objectURI != "https://example.com/activities/123" {
		t.Errorf("Expected object URI 'https://example.com/activities/123', got '%s'", objectURI)
	}
}

func TestActivityObjectAsMap(t *testing.T) {
	// Test Activity with object as embedded map
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/activities/789",
		"type": "Create",
		"actor": "https://example.com/users/alice",
		"object": {
			"id": "https://example.com/notes/abc",
			"type": "Note",
			"content": "Hello world"
		}
	}`

	var activity Activity
	if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
		t.Fatalf("Failed to unmarshal Activity with map object: %v", err)
	}

	// Verify we can extract the object URI
	var objectURI string
	switch obj := activity.Object.(type) {
	case map[string]interface{}:
		if id, ok := obj["id"].(string); ok {
			objectURI = id
		}
	}

	if objectURI != "https://example.com/notes/abc" {
		t.Errorf("Expected object URI 'https://example.com/notes/abc', got '%s'", objectURI)
	}
}

func TestFollowActivityUnmarshal(t *testing.T) {
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://mastodon.social/follows/123",
		"type": "Follow",
		"actor": "https://mastodon.social/users/alice",
		"object": "https://stegodon.example/users/bob"
	}`

	var follow FollowActivity
	if err := json.Unmarshal([]byte(jsonData), &follow); err != nil {
		t.Fatalf("Failed to unmarshal FollowActivity: %v", err)
	}

	if follow.ID != "https://mastodon.social/follows/123" {
		t.Errorf("Expected ID, got '%s'", follow.ID)
	}
	if follow.Type != "Follow" {
		t.Errorf("Expected Type 'Follow', got '%s'", follow.Type)
	}
	if follow.Actor != "https://mastodon.social/users/alice" {
		t.Errorf("Expected Actor URL, got '%s'", follow.Actor)
	}
	if follow.Object != "https://stegodon.example/users/bob" {
		t.Errorf("Expected Object URL, got '%s'", follow.Object)
	}
}

func TestUndoActivityStructure(t *testing.T) {
	// Test parsing Undo activity with embedded Follow
	jsonData := `{
		"type": "Undo",
		"actor": "https://example.com/users/alice",
		"object": {
			"type": "Follow",
			"id": "https://example.com/follows/123"
		}
	}`

	var undo struct {
		Type   string          `json:"type"`
		Actor  string          `json:"actor"`
		Object json.RawMessage `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &undo); err != nil {
		t.Fatalf("Failed to unmarshal Undo: %v", err)
	}

	if undo.Type != "Undo" {
		t.Errorf("Expected Type 'Undo', got '%s'", undo.Type)
	}

	// Parse embedded object
	var obj struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(undo.Object, &obj); err != nil {
		t.Fatalf("Failed to unmarshal Undo object: %v", err)
	}

	if obj.Type != "Follow" {
		t.Errorf("Expected embedded Type 'Follow', got '%s'", obj.Type)
	}
	if obj.ID != "https://example.com/follows/123" {
		t.Errorf("Expected embedded ID, got '%s'", obj.ID)
	}
}

func TestCreateActivityStructure(t *testing.T) {
	jsonData := `{
		"id": "https://mastodon.social/users/alice/statuses/123/activity",
		"type": "Create",
		"actor": "https://mastodon.social/users/alice",
		"object": {
			"id": "https://mastodon.social/users/alice/statuses/123",
			"type": "Note",
			"content": "Hello from Mastodon!",
			"published": "2025-11-14T10:00:00Z",
			"attributedTo": "https://mastodon.social/users/alice"
		}
	}`

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

	if err := json.Unmarshal([]byte(jsonData), &create); err != nil {
		t.Fatalf("Failed to unmarshal Create: %v", err)
	}

	if create.Type != "Create" {
		t.Errorf("Expected Type 'Create', got '%s'", create.Type)
	}
	if create.Actor != "https://mastodon.social/users/alice" {
		t.Errorf("Expected Actor URL, got '%s'", create.Actor)
	}
	if create.Object.Type != "Note" {
		t.Errorf("Expected Object Type 'Note', got '%s'", create.Object.Type)
	}
	if create.Object.Content != "Hello from Mastodon!" {
		t.Errorf("Expected content, got '%s'", create.Object.Content)
	}
}

func TestAcceptActivityStructure(t *testing.T) {
	jsonData := `{
		"type": "Accept",
		"actor": "https://mastodon.social/users/bob",
		"object": {
			"id": "https://stegodon.example/follows/456",
			"type": "Follow"
		}
	}`

	var accept struct {
		Type   string          `json:"type"`
		Actor  string          `json:"actor"`
		Object json.RawMessage `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &accept); err != nil {
		t.Fatalf("Failed to unmarshal Accept: %v", err)
	}

	if accept.Type != "Accept" {
		t.Errorf("Expected Type 'Accept', got '%s'", accept.Type)
	}

	// Parse embedded Follow object
	var followObj struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(accept.Object, &followObj); err != nil {
		t.Fatalf("Failed to unmarshal Accept object: %v", err)
	}

	if followObj.ID != "https://stegodon.example/follows/456" {
		t.Errorf("Expected follow ID, got '%s'", followObj.ID)
	}
}

func TestUpdateActivityStructure(t *testing.T) {
	tests := []struct {
		name       string
		jsonData   string
		expectType string
		expectID   string
	}{
		{
			name: "Person update (profile)",
			jsonData: `{
				"id": "https://mastodon.social/users/alice#updates/1",
				"type": "Update",
				"actor": "https://mastodon.social/users/alice",
				"object": {
					"type": "Person",
					"id": "https://mastodon.social/users/alice"
				}
			}`,
			expectType: "Person",
			expectID:   "https://mastodon.social/users/alice",
		},
		{
			name: "Note update (post edit)",
			jsonData: `{
				"id": "https://mastodon.social/users/alice#updates/2",
				"type": "Update",
				"actor": "https://mastodon.social/users/alice",
				"object": {
					"type": "Note",
					"id": "https://mastodon.social/users/alice/statuses/123"
				}
			}`,
			expectType: "Note",
			expectID:   "https://mastodon.social/users/alice/statuses/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var update struct {
				ID     string          `json:"id"`
				Type   string          `json:"type"`
				Actor  string          `json:"actor"`
				Object json.RawMessage `json:"object"`
			}

			if err := json.Unmarshal([]byte(tt.jsonData), &update); err != nil {
				t.Fatalf("Failed to unmarshal Update: %v", err)
			}

			if update.Type != "Update" {
				t.Errorf("Expected Type 'Update', got '%s'", update.Type)
			}

			var objectType struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}
			if err := json.Unmarshal(update.Object, &objectType); err != nil {
				t.Fatalf("Failed to unmarshal Update object: %v", err)
			}

			if objectType.Type != tt.expectType {
				t.Errorf("Expected object type '%s', got '%s'", tt.expectType, objectType.Type)
			}
			if objectType.ID != tt.expectID {
				t.Errorf("Expected object ID '%s', got '%s'", tt.expectID, objectType.ID)
			}
		})
	}
}

func TestDeleteActivityStringObject(t *testing.T) {
	// Test Delete with simple string object
	jsonData := `{
		"id": "https://mastodon.social/users/alice#delete/1",
		"type": "Delete",
		"actor": "https://mastodon.social/users/alice",
		"object": "https://mastodon.social/users/alice/statuses/123"
	}`

	var delete struct {
		ID     string      `json:"id"`
		Type   string      `json:"type"`
		Actor  string      `json:"actor"`
		Object interface{} `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &delete); err != nil {
		t.Fatalf("Failed to unmarshal Delete: %v", err)
	}

	// Extract object URI
	var objectURI string
	switch obj := delete.Object.(type) {
	case string:
		objectURI = obj
	}

	if objectURI != "https://mastodon.social/users/alice/statuses/123" {
		t.Errorf("Expected object URI, got '%s'", objectURI)
	}
}

func TestDeleteActivityTombstone(t *testing.T) {
	// Test Delete with Tombstone object
	jsonData := `{
		"id": "https://mastodon.social/users/alice#delete/2",
		"type": "Delete",
		"actor": "https://mastodon.social/users/alice",
		"object": {
			"id": "https://mastodon.social/users/alice/statuses/456",
			"type": "Tombstone"
		}
	}`

	var delete struct {
		ID     string      `json:"id"`
		Type   string      `json:"type"`
		Actor  string      `json:"actor"`
		Object interface{} `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &delete); err != nil {
		t.Fatalf("Failed to unmarshal Delete: %v", err)
	}

	// Extract object URI from Tombstone
	var objectURI string
	switch obj := delete.Object.(type) {
	case map[string]interface{}:
		if typ, ok := obj["type"].(string); ok && typ == "Tombstone" {
			if id, ok := obj["id"].(string); ok {
				objectURI = id
			}
		}
	}

	if objectURI != "https://mastodon.social/users/alice/statuses/456" {
		t.Errorf("Expected object URI from Tombstone, got '%s'", objectURI)
	}
}

func TestDeleteActivityActorDeletion(t *testing.T) {
	// Test Delete where object is the actor (account deletion)
	jsonData := `{
		"id": "https://mastodon.social/users/alice#delete",
		"type": "Delete",
		"actor": "https://mastodon.social/users/alice",
		"object": "https://mastodon.social/users/alice"
	}`

	var delete struct {
		ID     string      `json:"id"`
		Type   string      `json:"type"`
		Actor  string      `json:"actor"`
		Object interface{} `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &delete); err != nil {
		t.Fatalf("Failed to unmarshal Delete: %v", err)
	}

	var objectURI string
	switch obj := delete.Object.(type) {
	case string:
		objectURI = obj
	}

	// Check if it's an actor deletion
	isActorDeletion := (objectURI == delete.Actor)
	if !isActorDeletion {
		t.Error("Expected actor deletion (object == actor)")
	}
}

func TestActivityTypes(t *testing.T) {
	// Test that we correctly identify different activity types
	activityTypes := []string{
		"Follow",
		"Undo",
		"Create",
		"Like",
		"Accept",
		"Update",
		"Delete",
		"Announce",
		"Reject",
	}

	for _, actType := range activityTypes {
		t.Run(actType, func(t *testing.T) {
			jsonData := `{"type":"` + actType + `"}`
			var activity Activity
			if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
				t.Fatalf("Failed to unmarshal %s activity: %v", actType, err)
			}

			if activity.Type != actType {
				t.Errorf("Expected Type '%s', got '%s'", actType, activity.Type)
			}
		})
	}
}

func TestActivityContextVariants(t *testing.T) {
	// Test different @context formats
	tests := []struct {
		name    string
		context interface{}
	}{
		{
			name:    "string context",
			context: "https://www.w3.org/ns/activitystreams",
		},
		{
			name: "array context",
			context: []interface{}{
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, _ := json.Marshal(map[string]interface{}{
				"@context": tt.context,
				"type":     "Follow",
			})

			var activity Activity
			if err := json.Unmarshal(jsonBytes, &activity); err != nil {
				t.Fatalf("Failed to unmarshal activity with %s: %v", tt.name, err)
			}

			if activity.Type != "Follow" {
				t.Error("Activity type should be preserved regardless of context format")
			}
		})
	}
}

func TestActivityValidation(t *testing.T) {
	// Test validation of required fields
	tests := []struct {
		name      string
		jsonData  string
		wantError bool
	}{
		{
			name: "valid activity",
			jsonData: `{
				"type": "Follow",
				"actor": "https://example.com/users/alice",
				"object": "https://example.com/users/bob"
			}`,
			wantError: false,
		},
		{
			name:      "missing type",
			jsonData:  `{"actor": "https://example.com/users/alice"}`,
			wantError: false, // JSON unmarshal won't error, just empty Type
		},
		{
			name:      "invalid JSON",
			jsonData:  `{invalid json}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var activity Activity
			err := json.Unmarshal([]byte(tt.jsonData), &activity)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestActivityURIExtraction(t *testing.T) {
	// Test extracting various URIs from activities
	tests := []struct {
		name        string
		jsonData    string
		expectActor string
		expectID    string
	}{
		{
			name: "Mastodon follow",
			jsonData: `{
				"id": "https://mastodon.social/12345678-1234-1234-1234-123456789abc",
				"type": "Follow",
				"actor": "https://mastodon.social/users/alice"
			}`,
			expectActor: "https://mastodon.social/users/alice",
			expectID:    "https://mastodon.social/12345678-1234-1234-1234-123456789abc",
		},
		{
			name: "Pleroma create",
			jsonData: `{
				"id": "https://pleroma.site/activities/abcdef",
				"type": "Create",
				"actor": "https://pleroma.site/users/bob"
			}`,
			expectActor: "https://pleroma.site/users/bob",
			expectID:    "https://pleroma.site/activities/abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var activity Activity
			if err := json.Unmarshal([]byte(tt.jsonData), &activity); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if activity.Actor != tt.expectActor {
				t.Errorf("Expected actor '%s', got '%s'", tt.expectActor, activity.Actor)
			}
			if activity.ID != tt.expectID {
				t.Errorf("Expected ID '%s', got '%s'", tt.expectID, activity.ID)
			}
		})
	}
}

func TestObjectURIExtraction(t *testing.T) {
	// Test the logic for extracting objectURI from different object formats
	tests := []struct {
		name      string
		object    interface{}
		wantURI   string
		wantFound bool
	}{
		{
			name:      "string object",
			object:    "https://example.com/notes/123",
			wantURI:   "https://example.com/notes/123",
			wantFound: true,
		},
		{
			name: "map object with id",
			object: map[string]interface{}{
				"id":   "https://example.com/notes/456",
				"type": "Note",
			},
			wantURI:   "https://example.com/notes/456",
			wantFound: true,
		},
		{
			name: "map object without id",
			object: map[string]interface{}{
				"type": "Note",
			},
			wantURI:   "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objectURI string
			switch obj := tt.object.(type) {
			case string:
				objectURI = obj
			case map[string]interface{}:
				if id, ok := obj["id"].(string); ok {
					objectURI = id
				}
			}

			if tt.wantFound && objectURI == "" {
				t.Error("Expected to find object URI but didn't")
			}
			if !tt.wantFound && objectURI != "" {
				t.Error("Expected not to find object URI but did")
			}
			if objectURI != tt.wantURI {
				t.Errorf("Expected URI '%s', got '%s'", tt.wantURI, objectURI)
			}
		})
	}
}

func TestHTMLContentInNote(t *testing.T) {
	// Test that we can handle HTML content in Create activities
	jsonData := `{
		"type": "Create",
		"object": {
			"type": "Note",
			"content": "<p>Hello <strong>world</strong>!</p>"
		}
	}`

	var create struct {
		Type   string `json:"type"`
		Object struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &create); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !strings.Contains(create.Object.Content, "<p>") {
		t.Error("Content should preserve HTML tags")
	}
	if !strings.Contains(create.Object.Content, "<strong>") {
		t.Error("Content should preserve HTML formatting")
	}
}
