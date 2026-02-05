package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

func newTestHandlerWithDB(input string, db *mockDatabase) (*Handler, *bytes.Buffer) {
	session := newMockSession(input)
	account := &domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}
	conf := &util.AppConfig{}
	conf.Conf.MaxChars = 150

	handler := &Handler{
		session: session,
		db:      db,
		account: account,
		conf:    conf,
	}

	return handler, session.writer
}

func TestPost_TextMode(t *testing.T) {
	db := &mockDatabase{
		createdNoteID: uuid.New(),
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "Hello from CLI"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Posted:") {
		t.Errorf("Expected 'Posted:' in output, got: %s", result)
	}
	if !strings.Contains(result, db.createdNoteID.String()) {
		t.Errorf("Expected note ID in output, got: %s", result)
	}
}

func TestPost_JSONMode(t *testing.T) {
	db := &mockDatabase{
		createdNoteID: uuid.New(),
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "Hello from CLI", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp PostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.ID != db.createdNoteID.String() {
		t.Errorf("Expected ID %s, got %s", db.createdNoteID.String(), resp.ID)
	}
	if resp.Message != "Hello from CLI" {
		t.Errorf("Expected message 'Hello from CLI', got %s", resp.Message)
	}
}

func TestPost_MultipleWords(t *testing.T) {
	db := &mockDatabase{
		createdNoteID: uuid.New(),
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "Hello", "from", "CLI", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp PostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if resp.Message != "Hello from CLI" {
		t.Errorf("Expected message 'Hello from CLI', got %s", resp.Message)
	}
}

func TestPost_Stdin(t *testing.T) {
	db := &mockDatabase{
		createdNoteID: uuid.New(),
	}
	handler, output := newTestHandlerWithDB("Content from stdin", db)

	err := handler.Execute([]string{"post", "-", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp PostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if resp.Message != "Content from stdin" {
		t.Errorf("Expected message 'Content from stdin', got %s", resp.Message)
	}
}

func TestPost_EmptyMessage(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post"})
	if err == nil {
		t.Error("Expected error for empty message")
	}
}

func TestPost_EmptyStdin(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "-"})
	if err == nil {
		t.Error("Expected error for empty stdin message")
	}
}

func TestPost_TooLong(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	// Create a message longer than 150 chars
	longMessage := strings.Repeat("a", 200)
	err := handler.Execute([]string{"post", longMessage})
	if err == nil {
		t.Error("Expected error for message too long")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Errorf("Expected 'too long' error, got: %v", err)
	}
}

func TestPost_DatabaseError(t *testing.T) {
	db := &mockDatabase{
		createError: errors.New("database error"),
	}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "Hello"})
	if err == nil {
		t.Error("Expected error from database")
	}
	if !strings.Contains(err.Error(), "database error") {
		t.Errorf("Expected 'database error', got: %v", err)
	}
}

func TestPost_ReplyToLocalNote(t *testing.T) {
	noteId := uuid.New()
	db := &mockDatabase{
		createdNoteID: uuid.New(),
		note: &domain.Note{
			Id:        noteId,
			ObjectURI: "https://example.com/notes/123",
			Message:   "Original post",
		},
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "My reply", "--reply-to", noteId.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp PostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.Message != "My reply" {
		t.Errorf("Expected message 'My reply', got %s", resp.Message)
	}
	if resp.InReplyTo != "https://example.com/notes/123" {
		t.Errorf("Expected InReplyTo 'https://example.com/notes/123', got %s", resp.InReplyTo)
	}
	if db.inReplyToURI != "https://example.com/notes/123" {
		t.Errorf("Expected database to receive inReplyToURI 'https://example.com/notes/123', got %s", db.inReplyToURI)
	}
}

func TestPost_ReplyToRemoteActivity(t *testing.T) {
	activityId := uuid.New()
	db := &mockDatabase{
		createdNoteID: uuid.New(),
		activity: &domain.Activity{
			Id:        activityId,
			ObjectURI: "https://remote.example.com/notes/456",
		},
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "Reply to remote", "--reply-to", activityId.String(), "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp PostResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.InReplyTo != "https://remote.example.com/notes/456" {
		t.Errorf("Expected InReplyTo 'https://remote.example.com/notes/456', got %s", resp.InReplyTo)
	}
}

func TestPost_ReplyToInvalidId(t *testing.T) {
	db := &mockDatabase{
		createdNoteID: uuid.New(),
	}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "My reply", "--reply-to", "invalid-uuid"})
	if err == nil {
		t.Error("Expected error for invalid UUID")
	}
	if !strings.Contains(err.Error(), "invalid reply-to ID") {
		t.Errorf("Expected 'invalid reply-to ID' error, got: %v", err)
	}
}

func TestPost_ReplyToNotFound(t *testing.T) {
	db := &mockDatabase{
		createdNoteID: uuid.New(),
		// note and activity are nil - post not found
	}
	handler, _ := newTestHandlerWithDB("", db)

	postId := uuid.New()
	err := handler.Execute([]string{"post", "My reply", "--reply-to", postId.String()})
	if err == nil {
		t.Error("Expected error for post not found")
	}
	if !strings.Contains(err.Error(), "reply-to post not found") {
		t.Errorf("Expected 'reply-to post not found' error, got: %v", err)
	}
}

func TestPost_ReplyTextMode(t *testing.T) {
	noteId := uuid.New()
	db := &mockDatabase{
		createdNoteID: uuid.New(),
		note: &domain.Note{
			Id:        noteId,
			ObjectURI: "https://example.com/notes/123",
			Message:   "Original post",
		},
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"post", "My reply", "--reply-to", noteId.String()})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Reply posted:") {
		t.Errorf("Expected 'Reply posted:' in output, got: %s", result)
	}
}

func TestParseReplyToFlag(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		wantArgs    []string
		wantReplyTo string
	}{
		{
			name:        "no flag",
			input:       []string{"Hello", "world"},
			wantArgs:    []string{"Hello", "world"},
			wantReplyTo: "",
		},
		{
			name:        "flag at end",
			input:       []string{"Hello", "--reply-to", "abc-123"},
			wantArgs:    []string{"Hello"},
			wantReplyTo: "abc-123",
		},
		{
			name:        "flag at start",
			input:       []string{"--reply-to", "abc-123", "Hello"},
			wantArgs:    []string{"Hello"},
			wantReplyTo: "abc-123",
		},
		{
			name:        "flag in middle",
			input:       []string{"Hello", "--reply-to", "abc-123", "world"},
			wantArgs:    []string{"Hello", "world"},
			wantReplyTo: "abc-123",
		},
		{
			name:        "flag without value",
			input:       []string{"Hello", "--reply-to"},
			wantArgs:    []string{"Hello", "--reply-to"},
			wantReplyTo: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotReplyTo := parseReplyToFlag(tt.input)

			if gotReplyTo != tt.wantReplyTo {
				t.Errorf("parseReplyToFlag() replyTo = %v, want %v", gotReplyTo, tt.wantReplyTo)
			}

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("parseReplyToFlag() args len = %d, want %d", len(gotArgs), len(tt.wantArgs))
			} else {
				for i, arg := range gotArgs {
					if arg != tt.wantArgs[i] {
						t.Errorf("parseReplyToFlag() args[%d] = %s, want %s", i, arg, tt.wantArgs[i])
					}
				}
			}
		})
	}
}
