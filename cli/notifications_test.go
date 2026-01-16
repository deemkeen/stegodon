package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

func TestNotifications_TextMode(t *testing.T) {
	notifications := []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        uuid.New(),
			NotificationType: domain.NotificationFollow,
			ActorUsername:    "alice",
			ActorDomain:      "",
			CreatedAt:        time.Now().Add(-5 * time.Minute),
		},
		{
			Id:               uuid.New(),
			AccountId:        uuid.New(),
			NotificationType: domain.NotificationLike,
			ActorUsername:    "bob",
			ActorDomain:      "mastodon.social",
			NotePreview:      "Hello world",
			CreatedAt:        time.Now().Add(-10 * time.Minute),
		},
	}

	db := &mockDatabase{
		notifications: notifications,
		unreadCount:   2,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"notifications"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "@alice") {
		t.Errorf("Expected '@alice' in output, got: %s", result)
	}
	if !strings.Contains(result, "followed you") {
		t.Errorf("Expected 'followed you' in output, got: %s", result)
	}
	if !strings.Contains(result, "@bob@mastodon.social") {
		t.Errorf("Expected '@bob@mastodon.social' in output, got: %s", result)
	}
	if !strings.Contains(result, "2 unread") {
		t.Errorf("Expected '2 unread' in output, got: %s", result)
	}
}

func TestNotifications_JSONMode(t *testing.T) {
	notifications := []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        uuid.New(),
			NotificationType: domain.NotificationFollow,
			ActorUsername:    "alice",
			ActorDomain:      "",
			CreatedAt:        time.Now().Add(-5 * time.Minute),
		},
		{
			Id:               uuid.New(),
			AccountId:        uuid.New(),
			NotificationType: domain.NotificationLike,
			ActorUsername:    "bob",
			ActorDomain:      "mastodon.social",
			NotePreview:      "Hello world",
			CreatedAt:        time.Now().Add(-10 * time.Minute),
		},
		{
			Id:               uuid.New(),
			AccountId:        uuid.New(),
			NotificationType: domain.NotificationReply,
			ActorUsername:    "charlie",
			ActorDomain:      "",
			NotePreview:      "Nice post!",
			CreatedAt:        time.Now().Add(-15 * time.Minute),
		},
		{
			Id:               uuid.New(),
			AccountId:        uuid.New(),
			NotificationType: domain.NotificationMention,
			ActorUsername:    "dave",
			ActorDomain:      "fosstodon.org",
			NotePreview:      "Hey @testuser check this out",
			CreatedAt:        time.Now().Add(-20 * time.Minute),
		},
	}

	db := &mockDatabase{
		notifications: notifications,
		unreadCount:   3,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"notifications", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp NotificationsResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v, output: %s", err, output.String())
	}

	if resp.UnreadCount != 3 {
		t.Errorf("Expected UnreadCount 3, got %d", resp.UnreadCount)
	}
	if len(resp.Notifications) != 4 {
		t.Errorf("Expected 4 notifications, got %d", len(resp.Notifications))
	}

	// Check follow notification
	if resp.Notifications[0].Type != "follow" {
		t.Errorf("Expected type 'follow', got %s", resp.Notifications[0].Type)
	}
	if resp.Notifications[0].Actor != "@alice" {
		t.Errorf("Expected actor '@alice', got %s", resp.Notifications[0].Actor)
	}

	// Check like notification
	if resp.Notifications[1].Type != "like" {
		t.Errorf("Expected type 'like', got %s", resp.Notifications[1].Type)
	}
	if resp.Notifications[1].Actor != "@bob@mastodon.social" {
		t.Errorf("Expected actor '@bob@mastodon.social', got %s", resp.Notifications[1].Actor)
	}
	if resp.Notifications[1].NotePreview != "Hello world" {
		t.Errorf("Expected NotePreview 'Hello world', got %s", resp.Notifications[1].NotePreview)
	}

	// Check reply notification
	if resp.Notifications[2].Type != "reply" {
		t.Errorf("Expected type 'reply', got %s", resp.Notifications[2].Type)
	}

	// Check mention notification
	if resp.Notifications[3].Type != "mention" {
		t.Errorf("Expected type 'mention', got %s", resp.Notifications[3].Type)
	}
}

func TestNotifications_Empty(t *testing.T) {
	db := &mockDatabase{
		notifications: []domain.Notification{},
		unreadCount:   0,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"notifications", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp NotificationsResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if resp.UnreadCount != 0 {
		t.Errorf("Expected UnreadCount 0, got %d", resp.UnreadCount)
	}
	if len(resp.Notifications) != 0 {
		t.Errorf("Expected 0 notifications, got %d", len(resp.Notifications))
	}
}

func TestNotifications_EmptyTextMode(t *testing.T) {
	db := &mockDatabase{
		notifications: []domain.Notification{},
		unreadCount:   0,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"notifications"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "No notifications") {
		t.Errorf("Expected 'No notifications' in output, got: %s", result)
	}
}

func TestNotifications_HTMLStripping(t *testing.T) {
	notifications := []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        uuid.New(),
			NotificationType: domain.NotificationLike,
			ActorUsername:    "alice",
			ActorDomain:      "",
			NotePreview:      "<p>Hello <strong>world</strong></p>",
			CreatedAt:        time.Now(),
		},
	}

	db := &mockDatabase{
		notifications: notifications,
		unreadCount:   1,
	}
	handler, output := newTestHandlerWithDB("", db)

	err := handler.Execute([]string{"notifications", "--json"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp NotificationsResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	// HTML should be stripped
	if strings.Contains(resp.Notifications[0].NotePreview, "<p>") {
		t.Errorf("Expected HTML to be stripped, got: %s", resp.Notifications[0].NotePreview)
	}
	if !strings.Contains(resp.Notifications[0].NotePreview, "Hello") {
		t.Errorf("Expected content to contain 'Hello', got: %s", resp.Notifications[0].NotePreview)
	}
}
