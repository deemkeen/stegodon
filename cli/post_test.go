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
