package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	notes              []domain.HomePost
	notifications      []domain.Notification
	unreadCount        int
	createError        error
	createdNoteID      uuid.UUID
	deleteAllCalled    bool
	deleteAllError     error
	// For like/boost tests
	note               *domain.Note
	activity           *domain.Activity
	hasLike            bool
	hasBoost           bool
	like               *domain.Like
	boost              *domain.Boost
	likeCreated        bool
	boostCreated       bool
	likeDeleted        bool
	boostDeleted       bool
	likeError          error
	boostError         error
	// For reply tests
	inReplyToURI       string
	// For follow tests
	remoteAccount      *domain.RemoteAccount
	follow             *domain.Follow
	isFollowing        bool
	followCreated      bool
	followDeleted      bool
	followError        error
	localAccounts      map[string]*domain.Account
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

func (m *mockDatabase) CreateNoteWithReply(userId interface{}, message string, inReplyToURI string) (interface{}, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	if m.createdNoteID == uuid.Nil {
		m.createdNoteID = uuid.New()
	}
	m.inReplyToURI = inReplyToURI
	return m.createdNoteID, nil
}

func (m *mockDatabase) ReadNoteIdWithReplyInfo(id interface{}) (error, *domain.Note) {
	if m.note != nil {
		return nil, m.note
	}
	return fmt.Errorf("note not found"), nil
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

func (m *mockDatabase) DeleteAllNotifications(accountId interface{}) error {
	m.deleteAllCalled = true
	return m.deleteAllError
}

// Note/Activity lookups for like/boost
func (m *mockDatabase) ReadNoteId(id interface{}) (error, *domain.Note) {
	if m.note != nil {
		return nil, m.note
	}
	return fmt.Errorf("note not found"), nil
}

func (m *mockDatabase) ReadActivityById(id interface{}) (error, *domain.Activity) {
	if m.activity != nil {
		return nil, m.activity
	}
	return fmt.Errorf("activity not found"), nil
}

func (m *mockDatabase) ReadAccByUsername(username string) (error, *domain.Account) {
	return nil, &domain.Account{Id: uuid.New(), Username: username}
}

// Like operations
func (m *mockDatabase) HasLike(accountId, noteId interface{}) (bool, error) {
	return m.hasLike, m.likeError
}

func (m *mockDatabase) HasLikeByObjectURI(accountId interface{}, objectURI string) (bool, error) {
	return m.hasLike, m.likeError
}

func (m *mockDatabase) CreateLike(like *domain.Like) error {
	m.likeCreated = true
	return m.likeError
}

func (m *mockDatabase) CreateLikeByObjectURI(like *domain.Like, objectURI string) error {
	m.likeCreated = true
	return m.likeError
}

func (m *mockDatabase) DeleteLikeByAccountAndNote(accountId, noteId interface{}) error {
	m.likeDeleted = true
	return m.likeError
}

func (m *mockDatabase) DeleteLikeByAccountAndObjectURI(accountId interface{}, objectURI string) error {
	m.likeDeleted = true
	return m.likeError
}

func (m *mockDatabase) IncrementLikeCountByNoteId(noteId interface{}) error {
	return nil
}

func (m *mockDatabase) DecrementLikeCountByNoteId(noteId interface{}) error {
	return nil
}

func (m *mockDatabase) IncrementLikeCountByObjectURI(objectURI string) error {
	return nil
}

func (m *mockDatabase) DecrementLikeCountByObjectURI(objectURI string) error {
	return nil
}

func (m *mockDatabase) ReadLikeByAccountAndNote(accountId, noteId interface{}) (error, *domain.Like) {
	if m.like != nil {
		return nil, m.like
	}
	return nil, &domain.Like{Id: uuid.New(), URI: "https://example.com/likes/1"}
}

func (m *mockDatabase) ReadLikeByAccountAndObjectURI(accountId interface{}, objectURI string) (error, *domain.Like) {
	if m.like != nil {
		return nil, m.like
	}
	return nil, &domain.Like{Id: uuid.New(), URI: "https://example.com/likes/1"}
}

func (m *mockDatabase) CreateNotification(notification *domain.Notification) error {
	return nil
}

// Boost operations
func (m *mockDatabase) HasBoost(accountId, noteId interface{}) (bool, error) {
	return m.hasBoost, m.boostError
}

func (m *mockDatabase) HasBoostByObjectURI(accountId interface{}, objectURI string) (bool, error) {
	return m.hasBoost, m.boostError
}

func (m *mockDatabase) CreateBoost(boost *domain.Boost) error {
	m.boostCreated = true
	return m.boostError
}

func (m *mockDatabase) CreateBoostByObjectURI(boost *domain.Boost, objectURI string) error {
	m.boostCreated = true
	return m.boostError
}

func (m *mockDatabase) DeleteBoostByAccountAndNote(accountId, noteId interface{}) error {
	m.boostDeleted = true
	return m.boostError
}

func (m *mockDatabase) DeleteBoostByAccountAndObjectURI(accountId interface{}, objectURI string) error {
	m.boostDeleted = true
	return m.boostError
}

func (m *mockDatabase) IncrementBoostCountByNoteId(noteId interface{}) error {
	return nil
}

func (m *mockDatabase) DecrementBoostCountByNoteId(noteId interface{}) error {
	return nil
}

func (m *mockDatabase) IncrementBoostCountByObjectURI(objectURI string) error {
	return nil
}

func (m *mockDatabase) DecrementBoostCountByObjectURI(objectURI string) error {
	return nil
}

func (m *mockDatabase) ReadBoostByAccountAndNote(accountId, noteId interface{}) (error, *domain.Boost) {
	if m.boost != nil {
		return nil, m.boost
	}
	return nil, &domain.Boost{Id: uuid.New(), URI: "https://example.com/boosts/1"}
}

func (m *mockDatabase) ReadBoostByAccountAndObjectURI(accountId interface{}, objectURI string) (error, *domain.Boost) {
	if m.boost != nil {
		return nil, m.boost
	}
	return nil, &domain.Boost{Id: uuid.New(), URI: "https://example.com/boosts/1"}
}

// Follow operations
func (m *mockDatabase) ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount) {
	if m.remoteAccount != nil {
		return nil, m.remoteAccount
	}
	return nil, nil
}

func (m *mockDatabase) ReadFollowByAccountIds(accountId, targetAccountId interface{}) (error, *domain.Follow) {
	if m.follow != nil {
		return nil, m.follow
	}
	return fmt.Errorf("not found"), nil
}

func (m *mockDatabase) CreateLocalFollow(followerAccountId, targetAccountId interface{}) error {
	m.followCreated = true
	return m.followError
}

func (m *mockDatabase) DeleteLocalFollow(followerAccountId, targetAccountId interface{}) error {
	m.followDeleted = true
	return m.followError
}

func (m *mockDatabase) IsFollowingLocal(followerAccountId, targetAccountId interface{}) (bool, error) {
	return m.isFollowing, m.followError
}

func (m *mockDatabase) DeleteFollowByURI(uri string) error {
	m.followDeleted = true
	return m.followError
}

func (m *mockDatabase) ReadAccById(id interface{}) (error, *domain.Account) {
	return nil, &domain.Account{Id: id.(uuid.UUID), Username: "testuser"}
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
