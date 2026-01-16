package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// mockSession implements cli.Session for testing
type mockSession struct {
	reader io.Reader
	writer *bytes.Buffer
}

func newMockSession(input string) *mockSession {
	return &mockSession{
		reader: strings.NewReader(input),
		writer: &bytes.Buffer{},
	}
}

func (m *mockSession) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

func (m *mockSession) Read(p []byte) (n int, err error) {
	return m.reader.Read(p)
}

// mockDatabase implements cli.Database for testing
type mockDatabase struct {
	notes         []domain.HomePost
	notifications []domain.Notification
	unreadCount   int
	createError   error
	createdNoteID uuid.UUID
}

func (m *mockDatabase) CreateNote(userId interface{}, message string) (interface{}, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	if m.createdNoteID == uuid.Nil {
		m.createdNoteID = uuid.New()
	}
	return m.createdNoteID, nil
}

func (m *mockDatabase) ReadHomeTimelinePosts(accountId interface{}, limit int) (error, *[]domain.HomePost) {
	posts := m.notes
	if len(posts) > limit {
		posts = posts[:limit]
	}
	return nil, &posts
}

func (m *mockDatabase) ReadNotificationsByAccountId(accountId interface{}, limit int) (error, *[]domain.Notification) {
	notifs := m.notifications
	if len(notifs) > limit {
		notifs = notifs[:limit]
	}
	return nil, &notifs
}

func (m *mockDatabase) CountUnreadNotifications(accountId interface{}) (int, error) {
	return m.unreadCount, nil
}

func newTestHandler(input string) (*Handler, *bytes.Buffer) {
	session := newMockSession(input)
	db := &mockDatabase{}
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

func TestExecute_Help(t *testing.T) {
	handler, output := newTestHandler("")

	// Test text help
	err := handler.Execute([]string{"--help"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "stegodon CLI") {
		t.Errorf("Expected help output to contain 'stegodon CLI', got: %s", result)
	}
	if !strings.Contains(result, "post") {
		t.Errorf("Expected help output to contain 'post' command, got: %s", result)
	}
	if !strings.Contains(result, "timeline") {
		t.Errorf("Expected help output to contain 'timeline' command, got: %s", result)
	}
}

func TestExecute_HelpJSON(t *testing.T) {
	handler, output := newTestHandler("")

	err := handler.Execute([]string{"--help", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var helpResp HelpResponse
	if err := json.Unmarshal(output.Bytes(), &helpResp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if len(helpResp.Commands) == 0 {
		t.Error("Expected commands in help response")
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	handler, _ := newTestHandler("")

	err := handler.Execute([]string{"unknowncommand"})
	if err == nil {
		t.Error("Expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("Expected 'unknown command' error, got: %v", err)
	}
}

func TestExecute_NoCommand(t *testing.T) {
	handler, output := newTestHandler("")

	err := handler.Execute([]string{})
	if err != nil {
		t.Fatalf("Expected no error (should show help), got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "stegodon CLI") {
		t.Errorf("Expected help output when no command given, got: %s", result)
	}
}

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name         string
		input        []string
		wantArgs     []string
		wantJSONMode bool
	}{
		{
			name:         "no flags",
			input:        []string{"post", "hello"},
			wantArgs:     []string{"post", "hello"},
			wantJSONMode: false,
		},
		{
			name:         "json flag at end",
			input:        []string{"post", "hello", "--json"},
			wantArgs:     []string{"post", "hello"},
			wantJSONMode: true,
		},
		{
			name:         "json flag at start",
			input:        []string{"--json", "post", "hello"},
			wantArgs:     []string{"post", "hello"},
			wantJSONMode: true,
		},
		{
			name:         "short json flag",
			input:        []string{"timeline", "-j"},
			wantArgs:     []string{"timeline"},
			wantJSONMode: true,
		},
		{
			name:         "json flag in middle",
			input:        []string{"post", "--json", "hello"},
			wantArgs:     []string{"post", "hello"},
			wantJSONMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotJSON := parseGlobalFlags(tt.input)

			if gotJSON != tt.wantJSONMode {
				t.Errorf("parseGlobalFlags() jsonMode = %v, want %v", gotJSON, tt.wantJSONMode)
			}

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("parseGlobalFlags() args len = %d, want %d", len(gotArgs), len(tt.wantArgs))
			} else {
				for i, arg := range gotArgs {
					if arg != tt.wantArgs[i] {
						t.Errorf("parseGlobalFlags() args[%d] = %s, want %s", i, arg, tt.wantArgs[i])
					}
				}
			}
		})
	}
}

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"1 minute", 1 * time.Minute, "1 minute ago"},
		{"5 minutes", 5 * time.Minute, "5 minutes ago"},
		{"1 hour", 1 * time.Hour, "1 hour ago"},
		{"3 hours", 3 * time.Hour, "3 hours ago"},
		{"1 day", 24 * time.Hour, "1 day ago"},
		{"3 days", 72 * time.Hour, "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Now().Add(-tt.duration)
			got := FormatTimeAgo(testTime)
			if got != tt.want {
				t.Errorf("FormatTimeAgo() = %s, want %s", got, tt.want)
			}
		})
	}
}
