package hometimeline

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

func TestInitialModel(t *testing.T) {
	accountId := uuid.New()
	width := 120
	height := 40

	m := InitialModel(accountId, width, height, "")

	if m.AccountId != accountId {
		t.Errorf("Expected AccountId %v, got %v", accountId, m.AccountId)
	}
	if m.Width != width {
		t.Errorf("Expected Width %d, got %d", width, m.Width)
	}
	if m.Height != height {
		t.Errorf("Expected Height %d, got %d", height, m.Height)
	}
	if len(m.Posts) != 0 {
		t.Errorf("Expected empty Posts, got %d", len(m.Posts))
	}
	if m.Offset != 0 {
		t.Errorf("Expected Offset 0, got %d", m.Offset)
	}
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0, got %d", m.Selected)
	}
	if m.isActive {
		t.Error("Expected isActive to be false initially")
	}
	if m.showingURL {
		t.Error("Expected showingURL to be false initially")
	}
}

func TestUpdate_ActivateDeactivate(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	// Activate
	m, cmd := m.Update(common.ActivateViewMsg{})
	if !m.isActive {
		t.Error("Expected isActive true after ActivateViewMsg")
	}
	if cmd == nil {
		t.Error("Expected command to load posts on activation")
	}

	// Deactivate
	m, cmd = m.Update(common.DeactivateViewMsg{})
	if m.isActive {
		t.Error("Expected isActive false after DeactivateViewMsg")
	}
	if cmd != nil {
		t.Error("Expected no command on deactivation")
	}
}

func TestUpdate_PostsLoaded(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = true

	posts := []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "First post",
			Time:       time.Now(),
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			ReplyCount: 0,
		},
		{
			NoteID:     uuid.New(),
			Author:     "@remote@example.com",
			Content:    "Remote post",
			Time:       time.Now(),
			ObjectURI:  "https://remote.example.com/notes/2",
			IsLocal:    false,
			ReplyCount: 3,
		},
	}

	m, cmd := m.Update(postsLoadedMsg{posts: posts})

	if len(m.Posts) != 2 {
		t.Errorf("Expected 2 posts, got %d", len(m.Posts))
	}
	if m.Posts[0].Author != "testuser" {
		t.Errorf("Expected first author 'testuser', got '%s'", m.Posts[0].Author)
	}
	if cmd == nil {
		t.Error("Expected tickRefresh command when active")
	}
}

func TestUpdate_PostsLoaded_InactiveNoTick(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = false // Inactive

	posts := []domain.HomePost{
		{NoteID: uuid.New(), Author: "test", Content: "Test"},
	}

	m, cmd := m.Update(postsLoadedMsg{posts: posts})

	if cmd != nil {
		t.Error("Expected no tick command when inactive")
	}
}

func TestUpdate_RefreshTick_Active(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = true

	_, cmd := m.Update(refreshTickMsg{})

	if cmd == nil {
		t.Error("Expected loadHomePosts command when active")
	}
}

func TestUpdate_RefreshTick_Inactive(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = false

	_, cmd := m.Update(refreshTickMsg{})

	if cmd != nil {
		t.Error("Expected no command when inactive - ticker should stop")
	}
}

func TestUpdate_UpdateNoteList(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	_, cmd := m.Update(common.UpdateNoteList)

	if cmd == nil {
		t.Error("Expected loadHomePosts command on UpdateNoteList")
	}
}

func TestUpdate_Navigation(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{NoteID: uuid.New(), Author: "user1", Content: "Post 1"},
		{NoteID: uuid.New(), Author: "user2", Content: "Post 2"},
		{NoteID: uuid.New(), Author: "user3", Content: "Post 3"},
	}
	m.Selected = 0

	// Move down
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after 'j', got %d", m.Selected)
	}
	if m.showingURL {
		t.Error("Expected showingURL reset after navigation")
	}

	// Move down again
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 after down, got %d", m.Selected)
	}

	// Try to move past last (should stay)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 (stay at last), got %d", m.Selected)
	}

	// Move up
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after 'k', got %d", m.Selected)
	}

	// Move up
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 after up, got %d", m.Selected)
	}

	// Try to move before first (should stay)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 (stay at first), got %d", m.Selected)
	}
}

func TestUpdate_ToggleURL(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:    uuid.New(),
			Author:    "testuser",
			Content:   "Test post",
			ObjectURI: "https://example.com/notes/1",
		},
	}
	m.Selected = 0

	// Toggle URL on
	m, _ = m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if !m.showingURL {
		t.Error("Expected showingURL true after 'o'")
	}

	// Toggle URL off
	m, _ = m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if m.showingURL {
		t.Error("Expected showingURL false after second 'o'")
	}
}

func TestUpdate_ToggleURL_NoObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:    uuid.New(),
			Author:    "testuser",
			Content:   "Test post",
			ObjectURI: "", // No ObjectURI
		},
	}
	m.Selected = 0

	// Try to toggle URL - should not work without ObjectURI
	m, _ = m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if m.showingURL {
		t.Error("Expected showingURL false when no ObjectURI")
	}
}

func TestUpdate_ReplyToPost(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:    uuid.New(),
			Author:    "testuser",
			Content:   "Test post content",
			ObjectURI: "https://example.com/notes/1",
			IsLocal:   true,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	if cmd == nil {
		t.Fatal("Expected command for reply")
	}

	msg := cmd()
	replyMsg, ok := msg.(common.ReplyToNoteMsg)
	if !ok {
		t.Fatalf("Expected ReplyToNoteMsg, got %T", msg)
	}

	if replyMsg.NoteURI != "https://example.com/notes/1" {
		t.Errorf("Expected NoteURI 'https://example.com/notes/1', got '%s'", replyMsg.NoteURI)
	}
	if replyMsg.Author != "testuser" {
		t.Errorf("Expected Author 'testuser', got '%s'", replyMsg.Author)
	}
}

func TestUpdate_ReplyToLocalPostWithoutObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	noteID := uuid.New()
	m.Posts = []domain.HomePost{
		{
			NoteID:    noteID,
			Author:    "testuser",
			Content:   "Local post",
			ObjectURI: "", // No ObjectURI
			IsLocal:   true,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	if cmd == nil {
		t.Fatal("Expected command for reply")
	}

	msg := cmd()
	replyMsg, ok := msg.(common.ReplyToNoteMsg)
	if !ok {
		t.Fatalf("Expected ReplyToNoteMsg, got %T", msg)
	}

	expectedURI := "local:" + noteID.String()
	if replyMsg.NoteURI != expectedURI {
		t.Errorf("Expected NoteURI '%s', got '%s'", expectedURI, replyMsg.NoteURI)
	}
}

func TestUpdate_EnterOnPostWithReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	noteID := uuid.New()
	m.Posts = []domain.HomePost{
		{
			NoteID:     noteID,
			Author:     "testuser",
			Content:    "Post with replies",
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			ReplyCount: 5,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command for thread view")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	if viewMsg.NoteURI != "https://example.com/notes/1" {
		t.Errorf("Expected NoteURI, got '%s'", viewMsg.NoteURI)
	}
	if viewMsg.NoteID != noteID {
		t.Errorf("Expected NoteID %v, got %v", noteID, viewMsg.NoteID)
	}
	if !viewMsg.IsLocal {
		t.Error("Expected IsLocal true")
	}
}

func TestUpdate_EnterOnPostWithoutReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Post without replies",
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			ReplyCount: 0,
		},
	}
	m.Selected = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("Expected no command for post without replies")
	}
}

func TestUpdate_EnterOnLocalPostWithoutObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	noteID := uuid.New()
	m.Posts = []domain.HomePost{
		{
			NoteID:     noteID,
			Author:     "testuser",
			Content:    "Local post",
			ObjectURI:  "", // No ObjectURI
			IsLocal:    true,
			ReplyCount: 2,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command for thread view")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	expectedURI := "local:" + noteID.String()
	if viewMsg.NoteURI != expectedURI {
		t.Errorf("Expected NoteURI '%s', got '%s'", expectedURI, viewMsg.NoteURI)
	}
}

func TestUpdate_SelectionBoundsAfterReload(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Selected = 5 // Selected past current bounds
	m.isActive = true

	posts := []domain.HomePost{
		{NoteID: uuid.New(), Author: "user1", Content: "Post 1"},
		{NoteID: uuid.New(), Author: "user2", Content: "Post 2"},
	}

	m, _ = m.Update(postsLoadedMsg{posts: posts})

	// Selected should be clamped to valid range
	if m.Selected >= len(m.Posts) {
		t.Errorf("Selected %d out of bounds for %d posts", m.Selected, len(m.Posts))
	}
	if m.Selected != 1 {
		t.Errorf("Expected Selected clamped to 1, got %d", m.Selected)
	}
}

func TestView_EmptyPosts(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{}

	view := m.View()

	if !strings.Contains(view.Content, "No posts yet") {
		t.Error("Expected 'No posts yet' message")
	}
	if !strings.Contains(view.Content, "Follow some accounts") {
		t.Error("Expected follow suggestion")
	}
}

func TestView_PostCount(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{NoteID: uuid.New(), Author: "user1", Content: "Post 1", Time: time.Now()},
		{NoteID: uuid.New(), Author: "user2", Content: "Post 2", Time: time.Now()},
		{NoteID: uuid.New(), Author: "user3", Content: "Post 3", Time: time.Now()},
	}

	view := m.View()

	if !strings.Contains(view.Content, "3 posts") {
		t.Error("Expected '3 posts' in header")
	}
}

func TestView_SingularReply(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test",
			Time:       time.Now(),
			ReplyCount: 1,
		},
	}

	view := m.View()

	if !strings.Contains(view.Content, "1 reply") {
		t.Error("Expected singular '1 reply'")
	}
}

func TestView_PluralReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test",
			Time:       time.Now(),
			ReplyCount: 5,
		},
	}

	view := m.View()

	if !strings.Contains(view.Content, "5 replies") {
		t.Error("Expected plural '5 replies'")
	}
}

func TestFormatTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "minutes ago",
			time:     now.Add(-15 * time.Minute),
			expected: "15m ago",
		},
		{
			name:     "hours ago",
			time:     now.Add(-5 * time.Hour),
			expected: "5h ago",
		},
		{
			name:     "days ago",
			time:     now.Add(-3 * 24 * time.Hour),
			expected: "3d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.time)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should be 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should be 5")
	}
	if min(5, 5) != 5 {
		t.Error("min(5, 5) should be 5")
	}
}

func TestMax(t *testing.T) {
	if max(5, 10) != 10 {
		t.Error("max(5, 10) should be 10")
	}
	if max(10, 5) != 10 {
		t.Error("max(10, 5) should be 10")
	}
	if max(5, 5) != 5 {
		t.Error("max(5, 5) should be 5")
	}
}

func TestHomePost_Fields(t *testing.T) {
	noteID := uuid.New()
	now := time.Now()

	post := domain.HomePost{
		NoteID:     noteID,
		Author:     "testuser",
		Content:    "Test content",
		Time:       now,
		ObjectURI:  "https://example.com/notes/123",
		IsLocal:    true,
		ReplyCount: 3,
	}

	if post.NoteID != noteID {
		t.Errorf("Expected NoteID %v, got %v", noteID, post.NoteID)
	}
	if post.Author != "testuser" {
		t.Errorf("Expected Author 'testuser', got '%s'", post.Author)
	}
	if post.Content != "Test content" {
		t.Errorf("Expected Content 'Test content', got '%s'", post.Content)
	}
	if post.ReplyCount != 3 {
		t.Errorf("Expected ReplyCount 3, got %d", post.ReplyCount)
	}
	if !post.IsLocal {
		t.Error("Expected IsLocal true")
	}
}

func TestUpdate_EmptyPosts_NoCrash(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{}

	// Navigation on empty posts should not crash
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})

	// Should complete without panic
}

// ============================================================================
// Engagement Info Tests (i key functionality)
// ============================================================================

func TestUpdate_ToggleEngagementInfo_WithEngagement(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test post",
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			LikeCount:  5,
			BoostCount: 3,
		},
	}
	m.Selected = 0

	// Toggle engagement info on
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})

	if !m.showingEngagement {
		t.Error("Expected showingEngagement true after 'i' key")
	}
	if cmd == nil {
		t.Error("Expected command to load engagement info")
	}

	// Toggle engagement info off
	m, cmd = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})

	if m.showingEngagement {
		t.Error("Expected showingEngagement false after second 'i' key")
	}
}

func TestUpdate_ToggleEngagementInfo_NoEngagement(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test post with no engagement",
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			LikeCount:  0,
			BoostCount: 0,
		},
	}
	m.Selected = 0

	// Try to toggle engagement info on post without engagement
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})

	if m.showingEngagement {
		t.Error("Expected showingEngagement to remain false for post without engagement")
	}
	if cmd != nil {
		t.Error("Expected no command for post without engagement")
	}
}

func TestUpdate_EngagementInfoMsg(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	likers := []string{"alice", "bob@mastodon.social", "charlie"}
	boosters := []string{"dave", "eve@fosstodon.org"}

	m, cmd := m.Update(engagementInfoMsg{
		likers:   likers,
		boosters: boosters,
	})

	if cmd != nil {
		t.Error("Expected no command from engagementInfoMsg")
	}

	if len(m.engagementLikers) != 3 {
		t.Errorf("Expected 3 likers, got %d", len(m.engagementLikers))
	}
	if len(m.engagementBoosters) != 2 {
		t.Errorf("Expected 2 boosters, got %d", len(m.engagementBoosters))
	}

	if m.engagementLikers[0] != "alice" {
		t.Errorf("Expected first liker 'alice', got '%s'", m.engagementLikers[0])
	}
	if m.engagementLikers[1] != "bob@mastodon.social" {
		t.Errorf("Expected second liker 'bob@mastodon.social', got '%s'", m.engagementLikers[1])
	}
	if m.engagementBoosters[0] != "dave" {
		t.Errorf("Expected first booster 'dave', got '%s'", m.engagementBoosters[0])
	}
	if m.engagementBoosters[1] != "eve@fosstodon.org" {
		t.Errorf("Expected second booster 'eve@fosstodon.org', got '%s'", m.engagementBoosters[1])
	}
}

func TestUpdate_NavigationResetsEngagementInfo(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser1",
			Content:    "Post 1",
			LikeCount:  5,
			BoostCount: 3,
		},
		{
			NoteID:     uuid.New(),
			Author:     "testuser2",
			Content:    "Post 2",
			LikeCount:  2,
			BoostCount: 1,
		},
	}
	m.Selected = 0
	m.showingEngagement = true // Set engagement info as visible

	// Navigate down
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	if m.showingEngagement {
		t.Error("Expected showingEngagement reset to false after navigation")
	}
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1, got %d", m.Selected)
	}

	// Set engagement visible again
	m.showingEngagement = true

	// Navigate up
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	if m.showingEngagement {
		t.Error("Expected showingEngagement reset to false after up navigation")
	}
}

func TestUpdate_EngagementInfo_OnlyLikes(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Post with only likes",
			LikeCount:  5,
			BoostCount: 0,
		},
	}
	m.Selected = 0

	// Should work with only likes
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})

	if !m.showingEngagement {
		t.Error("Expected showingEngagement true for post with likes only")
	}
	if cmd == nil {
		t.Error("Expected command to load engagement info")
	}
}

func TestUpdate_EngagementInfo_OnlyBoosts(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Post with only boosts",
			LikeCount:  0,
			BoostCount: 3,
		},
	}
	m.Selected = 0

	// Should work with only boosts
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})

	if !m.showingEngagement {
		t.Error("Expected showingEngagement true for post with boosts only")
	}
	if cmd == nil {
		t.Error("Expected command to load engagement info")
	}
}

func TestUpdate_EngagementInfo_EmptyPosts(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{}

	// Should not crash with empty posts
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})

	if m.showingEngagement {
		t.Error("Expected showingEngagement false with empty posts")
	}
	if cmd != nil {
		t.Error("Expected no command with empty posts")
	}
}

func TestView_EngagementInfoDisplay(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "example.com")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test post",
			Time:       time.Now(),
			LikeCount:  3,
			BoostCount: 2,
			IsLocal:    true,
		},
	}
	m.Selected = 0
	m.showingEngagement = true
	m.engagementLikers = []string{"alice", "bob@mastodon.social", "charlie"}
	m.engagementBoosters = []string{"dave", "eve@fosstodon.org"}

	view := m.View()

	if !strings.Contains(view.Content, "⭐ Liked by:") {
		t.Error("Expected 'Liked by:' section in view")
	}
	if !strings.Contains(view.Content, "🔁 Boosted by:") {
		t.Error("Expected 'Boosted by:' section in view")
	}
	if !strings.Contains(view.Content, "@alice") {
		t.Error("Expected '@alice' in likers")
	}
	if !strings.Contains(view.Content, "@bob@mastodon.social") {
		t.Error("Expected '@bob@mastodon.social' in likers")
	}
	if !strings.Contains(view.Content, "@dave") {
		t.Error("Expected '@dave' in boosters")
	}
	if !strings.Contains(view.Content, "@eve@fosstodon.org") {
		t.Error("Expected '@eve@fosstodon.org' in boosters")
	}
	if !strings.Contains(view.Content, "(Press 'i' to toggle back)") {
		t.Error("Expected toggle hint in view")
	}
}

func TestView_EngagementInfoDisplay_OnlyLikers(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "example.com")
	m.Posts = []domain.HomePost{
		{
			NoteID:    uuid.New(),
			Author:    "testuser",
			Content:   "Test post",
			Time:      time.Now(),
			LikeCount: 2,
			IsLocal:   true,
		},
	}
	m.Selected = 0
	m.showingEngagement = true
	m.engagementLikers = []string{"alice", "bob"}
	m.engagementBoosters = []string{} // No boosters

	view := m.View()

	if !strings.Contains(view.Content, "⭐ Liked by:") {
		t.Error("Expected 'Liked by:' section in view")
	}
	if strings.Contains(view.Content, "🔁 Boosted by:") {
		t.Error("Did not expect 'Boosted by:' section when no boosters")
	}
	if !strings.Contains(view.Content, "@alice") {
		t.Error("Expected '@alice' in likers")
	}
}

func TestView_EngagementInfoDisplay_OnlyBoosters(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "example.com")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test post",
			Time:       time.Now(),
			BoostCount: 2,
			IsLocal:    true,
		},
	}
	m.Selected = 0
	m.showingEngagement = true
	m.engagementLikers = []string{} // No likers
	m.engagementBoosters = []string{"dave", "eve"}

	view := m.View()

	if strings.Contains(view.Content, "⭐ Liked by:") {
		t.Error("Did not expect 'Liked by:' section when no likers")
	}
	if !strings.Contains(view.Content, "🔁 Boosted by:") {
		t.Error("Expected 'Boosted by:' section in view")
	}
	if !strings.Contains(view.Content, "@dave") {
		t.Error("Expected '@dave' in boosters")
	}
}

func TestView_EngagementInfoDisplay_NoData(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "example.com")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test post",
			Time:       time.Now(),
			LikeCount:  1, // Has count but no data yet
			BoostCount: 1,
			IsLocal:    true,
		},
	}
	m.Selected = 0
	m.showingEngagement = true
	m.engagementLikers = []string{}   // Empty - data not loaded yet
	m.engagementBoosters = []string{} // Empty - data not loaded yet

	view := m.View()

	if !strings.Contains(view.Content, "No engagement information available yet") {
		t.Error("Expected 'No engagement information available yet' message")
	}
	if !strings.Contains(view.Content, "(Likes and boosts by local users will appear here)") {
		t.Error("Expected fallback explanation message")
	}
}

func TestView_EngagementInfoDisplay_ManyUsers(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "example.com")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Popular post",
			Time:       time.Now(),
			LikeCount:  15,
			BoostCount: 12,
			IsLocal:    true,
		},
	}
	m.Selected = 0
	m.showingEngagement = true

	// Create list of 15 likers
	m.engagementLikers = make([]string, 15)
	for i := 0; i < 15; i++ {
		m.engagementLikers[i] = fmt.Sprintf("user%d", i+1)
	}

	// Create list of 12 boosters
	m.engagementBoosters = make([]string, 12)
	for i := 0; i < 12; i++ {
		m.engagementBoosters[i] = fmt.Sprintf("booster%d", i+1)
	}

	view := m.View()

	// Should show first 10 likers + "and X more" message
	if !strings.Contains(view.Content, "...and 5 more") {
		t.Error("Expected '...and 5 more' for likers")
	}

	// Should show first 10 boosters + "and X more" message
	if !strings.Contains(view.Content, "...and 2 more") {
		t.Error("Expected '...and 2 more' for boosters")
	}

	// Verify we see the first users but not the ones beyond 10
	if !strings.Contains(view.Content, "@user1") {
		t.Error("Expected '@user1' in view")
	}
	if !strings.Contains(view.Content, "@user10") {
		t.Error("Expected '@user10' in view")
	}
	if strings.Contains(view.Content, "@user11") {
		t.Error("Did not expect '@user11' (should be truncated)")
	}
}

func TestView_EngagementInfo_NotShowing(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "example.com")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test post",
			Time:       time.Now(),
			LikeCount:  3,
			BoostCount: 2,
			IsLocal:    true,
		},
	}
	m.Selected = 0
	m.showingEngagement = false // Not showing
	m.engagementLikers = []string{"alice", "bob"}
	m.engagementBoosters = []string{"charlie"}

	view := m.View()

	// Should show normal content, not engagement info
	if strings.Contains(view.Content, "⭐ Liked by:") {
		t.Error("Did not expect 'Liked by:' when not showing engagement")
	}
	if strings.Contains(view.Content, "🔁 Boosted by:") {
		t.Error("Did not expect 'Boosted by:' when not showing engagement")
	}
	if !strings.Contains(view.Content, "Test post") {
		t.Error("Expected normal content to be displayed")
	}
}
