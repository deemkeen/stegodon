package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

func TestTimeline_TextMode(t *testing.T) {
	posts := []domain.HomePost{
		{
			ID:         uuid.New(),
			Author:     "@alice",
			Content:    "Hello from Alice",
			Time:       time.Now().Add(-5 * time.Minute),
			IsLocal:    true,
			ReplyCount: 1,
			LikeCount:  2,
		},
		{
			ID:         uuid.New(),
			Author:     "@bob@mastodon.social",
			Content:    "Hello from Bob",
			Time:       time.Now().Add(-10 * time.Minute),
			IsLocal:    false,
			ReplyCount: 0,
			LikeCount:  1,
		},
	}

	db := &mockDatabase{notes: posts}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"timeline"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "@alice") {
		t.Errorf("Expected '@alice' in output, got: %s", result)
	}
	if !strings.Contains(result, "Hello from Alice") {
		t.Errorf("Expected 'Hello from Alice' in output, got: %s", result)
	}
	if !strings.Contains(result, "@bob@mastodon.social") {
		t.Errorf("Expected '@bob@mastodon.social' in output, got: %s", result)
	}
}

func TestTimeline_JSONMode(t *testing.T) {
	posts := []domain.HomePost{
		{
			ID:         uuid.New(),
			Author:     "@alice",
			Content:    "Hello from Alice",
			Time:       time.Now().Add(-5 * time.Minute),
			IsLocal:    true,
			ReplyCount: 1,
			LikeCount:  2,
			BoostCount: 3,
		},
		{
			ID:         uuid.New(),
			Author:     "@bob@mastodon.social",
			Content:    "Hello from Bob",
			Time:       time.Now().Add(-10 * time.Minute),
			IsLocal:    false,
			ReplyCount: 0,
			LikeCount:  1,
			BoostCount: 0,
		},
	}

	db := &mockDatabase{notes: posts}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"timeline", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp TimelineResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.Count != 2 {
		t.Errorf("Expected count 2, got %d", resp.Count)
	}
	if len(resp.Posts) != 2 {
		t.Errorf("Expected 2 posts, got %d", len(resp.Posts))
	}

	// Check first post (local user)
	if resp.Posts[0].Author != "alice" {
		t.Errorf("Expected author 'alice', got %s", resp.Posts[0].Author)
	}
	if resp.Posts[0].Domain != "" {
		t.Errorf("Expected empty domain for local user, got %s", resp.Posts[0].Domain)
	}
	if resp.Posts[0].LikeCount != 2 {
		t.Errorf("Expected LikeCount 2, got %d", resp.Posts[0].LikeCount)
	}

	// Check second post (remote user)
	if resp.Posts[1].Author != "bob" {
		t.Errorf("Expected author 'bob', got %s", resp.Posts[1].Author)
	}
	if resp.Posts[1].Domain != "mastodon.social" {
		t.Errorf("Expected domain 'mastodon.social', got %s", resp.Posts[1].Domain)
	}
}

func TestTimeline_WithLimit(t *testing.T) {
	posts := make([]domain.HomePost, 30)
	for i := range posts {
		posts[i] = domain.HomePost{
			ID:      uuid.New(),
			Author:  "@user",
			Content: "Post content",
			Time:    time.Now(),
		}
	}

	db := &mockDatabase{notes: posts}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"timeline", "-n", "5", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp TimelineResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if resp.Count != 5 {
		t.Errorf("Expected count 5 with -n flag, got %d", resp.Count)
	}
}

func TestTimeline_InvalidLimit(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"timeline", "-n", "invalid"})
	if err == nil {
		t.Error("Expected error for invalid -n value")
	}
}

func TestTimeline_NegativeLimit(t *testing.T) {
	db := &mockDatabase{}
	handler, _ := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"timeline", "-n", "-5"})
	if err == nil {
		t.Error("Expected error for negative -n value")
	}
}

func TestTimeline_Empty(t *testing.T) {
	db := &mockDatabase{notes: []domain.HomePost{}}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"timeline", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp TimelineResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if resp.Count != 0 {
		t.Errorf("Expected count 0 for empty timeline, got %d", resp.Count)
	}
	if len(resp.Posts) != 0 {
		t.Errorf("Expected 0 posts for empty timeline, got %d", len(resp.Posts))
	}
}

func TestTimeline_HTMLStripping(t *testing.T) {
	posts := []domain.HomePost{
		{
			ID:      uuid.New(),
			Author:  "@alice",
			Content: "<p>Hello <strong>world</strong></p>",
			Time:    time.Now(),
		},
	}

	db := &mockDatabase{notes: posts}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"timeline", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp TimelineResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	// HTML should be stripped
	if strings.Contains(resp.Posts[0].Message, "<p>") {
		t.Errorf("Expected HTML to be stripped, got: %s", resp.Posts[0].Message)
	}
	if !strings.Contains(resp.Posts[0].Message, "Hello") {
		t.Errorf("Expected content to contain 'Hello', got: %s", resp.Posts[0].Message)
	}
}
