package domain

import (
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestNoteToString(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	note := &Note{
		Id:        id,
		CreatedBy: "testuser",
		Message:   "Test message",
		CreatedAt: now,
	}

	result := note.ToString()

	if len(result) == 0 {
		t.Error("ToString() returned empty string")
	}

	if !contains(result, "testuser") {
		t.Errorf("ToString() should contain creator, got: %s", result)
	}

	if !contains(result, "Test message") {
		t.Errorf("ToString() should contain message, got: %s", result)
	}

	if !contains(result, id.String()) {
		t.Errorf("ToString() should contain ID, got: %s", result)
	}
}

func TestSaveNoteStruct(t *testing.T) {
	userId := uuid.New()
	note := SaveNote{
		UserId:  userId,
		Message: "Test message content",
	}

	if note.UserId != userId {
		t.Errorf("Expected UserId %s, got %s", userId, note.UserId)
	}

	if note.Message != "Test message content" {
		t.Errorf("Expected Message 'Test message content', got '%s'", note.Message)
	}
}

func TestNoteStructFields(t *testing.T) {
	editTime := time.Now()
	note := Note{
		Id:             uuid.New(),
		CreatedBy:      "creator",
		Message:        "message content",
		CreatedAt:      time.Now(),
		EditedAt:       &editTime,
		Visibility:     "public",
		InReplyToURI:   "https://example.com/notes/123",
		ObjectURI:      "https://example.com/notes/456",
		Federated:      true,
		Sensitive:      false,
		ContentWarning: "",
	}

	if note.CreatedBy != "creator" {
		t.Errorf("Expected CreatedBy 'creator', got '%s'", note.CreatedBy)
	}

	if note.Message != "message content" {
		t.Errorf("Expected Message 'message content', got '%s'", note.Message)
	}

	if note.Visibility != "public" {
		t.Errorf("Expected Visibility 'public', got '%s'", note.Visibility)
	}

	if note.EditedAt == nil {
		t.Error("Expected EditedAt to be non-nil")
	}

	if !note.Federated {
		t.Error("Expected Federated to be true")
	}

	if note.Sensitive {
		t.Error("Expected Sensitive to be false")
	}

	if note.InReplyToURI != "https://example.com/notes/123" {
		t.Errorf("Expected InReplyToURI 'https://example.com/notes/123', got '%s'", note.InReplyToURI)
	}

	if note.ObjectURI != "https://example.com/notes/456" {
		t.Errorf("Expected ObjectURI 'https://example.com/notes/456', got '%s'", note.ObjectURI)
	}
}

func TestNoteWithNilEditedAt(t *testing.T) {
	note := Note{
		Id:        uuid.New(),
		CreatedBy: "creator",
		Message:   "message",
		CreatedAt: time.Now(),
		EditedAt:  nil,
	}

	if note.EditedAt != nil {
		t.Error("Expected EditedAt to be nil")
	}
}
