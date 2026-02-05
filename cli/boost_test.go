package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

func TestBoost_LocalNote(t *testing.T) {
	noteID := uuid.New()
	db := &mockDatabase{
		note: &domain.Note{
			Id:        noteID,
			CreatedBy: "alice",
			Message:   "Hello world",
			ObjectURI: "https://example.com/notes/1",
		},
		hasBoost: false,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"boost", noteID.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp BoostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.PostID != noteID.String() {
		t.Errorf("Expected post ID %s, got %s", noteID.String(), resp.PostID)
	}
	if !resp.Boosted {
		t.Errorf("Expected boosted=true, got false")
	}
	if !db.boostCreated {
		t.Error("Expected boost to be created in database")
	}
}

func TestBoost_UnboostLocalNote(t *testing.T) {
	noteID := uuid.New()
	db := &mockDatabase{
		note: &domain.Note{
			Id:        noteID,
			CreatedBy: "alice",
			Message:   "Hello world",
			ObjectURI: "https://example.com/notes/1",
		},
		hasBoost: true,
		boost: &domain.Boost{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			NoteId:    noteID,
			URI:       "https://example.com/boosts/1",
			CreatedAt: time.Now(),
		},
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"boost", noteID.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp BoostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.Boosted {
		t.Errorf("Expected boosted=false (unboosted), got true")
	}
	if !db.boostDeleted {
		t.Error("Expected boost to be deleted from database")
	}
}

func TestBoost_RemoteActivity(t *testing.T) {
	activityID := uuid.New()
	db := &mockDatabase{
		activity: &domain.Activity{
			Id:        activityID,
			ObjectURI: "https://mastodon.social/users/bob/statuses/123",
		},
		hasBoost: false,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"boost", activityID.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp BoostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if !resp.Boosted {
		t.Errorf("Expected boosted=true, got false")
	}
	if !db.boostCreated {
		t.Error("Expected boost to be created in database")
	}
}

func TestBoost_TextOutput(t *testing.T) {
	noteID := uuid.New()
	db := &mockDatabase{
		note: &domain.Note{
			Id:        noteID,
			CreatedBy: "alice",
			Message:   "Hello world",
			ObjectURI: "https://example.com/notes/1",
		},
		hasBoost: false,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"boost", noteID.String()})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Boosted post") {
		t.Errorf("Expected 'Boosted post' in output, got: %s", result)
	}
}

func TestBoost_NoPostID(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"boost"})
	if err == nil {
		t.Error("Expected error for missing post ID")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("Expected usage error, got: %v", err)
	}
}

func TestBoost_InvalidPostID(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"boost", "not-a-uuid"})
	if err == nil {
		t.Error("Expected error for invalid post ID")
	}
	if !strings.Contains(err.Error(), "invalid post ID") {
		t.Errorf("Expected 'invalid post ID' error, got: %v", err)
	}
}

func TestBoost_PostNotFound(t *testing.T) {
	db := &mockDatabase{
		note:     nil,
		activity: nil,
	}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"boost", uuid.New().String()})
	if err == nil {
		t.Error("Expected error for post not found")
	}
	if !strings.Contains(err.Error(), "post not found") {
		t.Errorf("Expected 'post not found' error, got: %v", err)
	}
}
