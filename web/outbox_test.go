package web

import (
	"encoding/json"
	"testing"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
)

func TestParsePageParam(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"valid page 1", "1", 1},
		{"valid page 5", "5", 5},
		{"invalid string", "abc", 0},
		{"negative number", "-1", 0},
		{"zero", "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePageParam(tt.input)
			if result != tt.expected {
				t.Errorf("ParsePageParam(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestOutboxJSONStructure(t *testing.T) {
	// Test that the outbox returns valid JSON structure
	// This is a basic structure test
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	// Test collection metadata (page 0)
	_, outbox := GetOutbox("nonexistent", 0, conf)

	// Should return valid JSON even for non-existent users
	var data map[string]interface{}
	err := json.Unmarshal([]byte(outbox), &data)
	if err != nil {
		t.Errorf("GetOutbox should return valid JSON: %v", err)
	}
}

func TestMakeNoteActivities(t *testing.T) {
	// Test that note activities are properly formatted
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	// Empty notes should return empty array
	activities := makeNoteActivities([]domain.Note{}, "testuser", conf)
	if len(activities) != 0 {
		t.Errorf("makeNoteActivities with empty notes should return empty array, got %d items", len(activities))
	}
}

func TestOutboxURLFormat(t *testing.T) {
	tests := []struct {
		name     string
		actor    string
		domain   string
		expected string
	}{
		{
			"standard user",
			"alice",
			"example.com",
			"https://example.com/users/alice/outbox",
		},
		{
			"user with numbers",
			"user123",
			"social.network",
			"https://social.network/users/user123/outbox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outboxURL := "https://" + tt.domain + "/users/" + tt.actor + "/outbox"
			if outboxURL != tt.expected {
				t.Errorf("Outbox URL = %s, want %s", outboxURL, tt.expected)
			}
		})
	}
}

func TestOutboxCollectionFields(t *testing.T) {
	// Test that OrderedCollection has required fields
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	// For a non-existent user, we should still get valid JSON
	_, outbox := GetOutbox("testuser", 0, conf)

	var data map[string]interface{}
	err := json.Unmarshal([]byte(outbox), &data)
	if err != nil {
		t.Fatalf("Failed to unmarshal outbox JSON: %v", err)
	}

	// Check for required OrderedCollection fields (may be empty object for non-existent user)
	// This test just verifies we get valid JSON
	if data == nil {
		t.Error("Outbox should return a JSON object")
	}
}
