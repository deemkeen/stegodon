package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

func TestLike_LocalNote(t *testing.T) {
	noteID := uuid.New()
	db := &mockDatabase{
		note: &domain.Note{
			Id:        noteID,
			CreatedBy: "alice",
			Message:   "Hello world",
			ObjectURI: "https://example.com/notes/1",
		},
		hasLike: false,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"like", noteID.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp LikeResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.PostID != noteID.String() {
		t.Errorf("Expected post ID %s, got %s", noteID.String(), resp.PostID)
	}
	if !resp.Liked {
		t.Errorf("Expected liked=true, got false")
	}
	if !db.likeCreated {
		t.Error("Expected like to be created in database")
	}
}

func TestLike_UnlikeLocalNote(t *testing.T) {
	noteID := uuid.New()
	db := &mockDatabase{
		note: &domain.Note{
			Id:        noteID,
			CreatedBy: "alice",
			Message:   "Hello world",
			ObjectURI: "https://example.com/notes/1",
		},
		hasLike: true,
		like: &domain.Like{
			Id:        uuid.New(),
			AccountId: uuid.New(),
			NoteId:    noteID,
			URI:       "https://example.com/likes/1",
			CreatedAt: time.Now(),
		},
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"like", noteID.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp LikeResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.Liked {
		t.Errorf("Expected liked=false (unliked), got true")
	}
	if !db.likeDeleted {
		t.Error("Expected like to be deleted from database")
	}
}

func TestLike_RemoteActivity(t *testing.T) {
	activityID := uuid.New()
	db := &mockDatabase{
		activity: &domain.Activity{
			Id:        activityID,
			ObjectURI: "https://mastodon.social/users/bob/statuses/123",
		},
		hasLike: false,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"like", activityID.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp LikeResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if !resp.Liked {
		t.Errorf("Expected liked=true, got false")
	}
	if !db.likeCreated {
		t.Error("Expected like to be created in database")
	}
}

func TestLike_TextOutput(t *testing.T) {
	noteID := uuid.New()
	db := &mockDatabase{
		note: &domain.Note{
			Id:        noteID,
			CreatedBy: "alice",
			Message:   "Hello world",
			ObjectURI: "https://example.com/notes/1",
		},
		hasLike: false,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"like", noteID.String()})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Liked post") {
		t.Errorf("Expected 'Liked post' in output, got: %s", result)
	}
}

func TestLike_NoPostID(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"like"})
	if err == nil {
		t.Error("Expected error for missing post ID")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("Expected usage error, got: %v", err)
	}
}

func TestLike_InvalidPostID(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"like", "not-a-uuid"})
	if err == nil {
		t.Error("Expected error for invalid post ID")
	}
	if !strings.Contains(err.Error(), "invalid post ID") {
		t.Errorf("Expected 'invalid post ID' error, got: %v", err)
	}
}

func TestLike_PostNotFound(t *testing.T) {
	db := &mockDatabase{
		note:     nil,
		activity: nil,
	}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"like", uuid.New().String()})
	if err == nil {
		t.Error("Expected error for post not found")
	}
	if !strings.Contains(err.Error(), "post not found") {
		t.Errorf("Expected 'post not found' error, got: %v", err)
	}
}
