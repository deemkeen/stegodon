package domain

import (
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestRemoteAccountStruct(t *testing.T) {
	id := uuid.New()
	now := time.Now()

	ra := RemoteAccount{
		Id:            id,
		Username:      "remoteuser",
		Domain:        "example.com",
		ActorURI:      "https://example.com/users/remoteuser",
		DisplayName:   "Remote User",
		Summary:       "Remote user bio",
		InboxURI:      "https://example.com/users/remoteuser/inbox",
		OutboxURI:     "https://example.com/users/remoteuser/outbox",
		PublicKeyPem:  "-----BEGIN RSA PUBLIC KEY-----",
		AvatarURL:     "https://example.com/avatar.png",
		LastFetchedAt: now,
	}

	if ra.Id != id {
		t.Errorf("Expected Id %s, got %s", id, ra.Id)
	}
	if ra.Username != "remoteuser" {
		t.Errorf("Expected Username 'remoteuser', got '%s'", ra.Username)
	}
	if ra.Domain != "example.com" {
		t.Errorf("Expected Domain 'example.com', got '%s'", ra.Domain)
	}
	if ra.ActorURI != "https://example.com/users/remoteuser" {
		t.Errorf("Expected ActorURI 'https://example.com/users/remoteuser', got '%s'", ra.ActorURI)
	}
	if ra.InboxURI != "https://example.com/users/remoteuser/inbox" {
		t.Errorf("Expected InboxURI 'https://example.com/users/remoteuser/inbox', got '%s'", ra.InboxURI)
	}
}

func TestFollowStruct(t *testing.T) {
	id := uuid.New()
	accountId := uuid.New()
	targetId := uuid.New()
	now := time.Now()

	follow := Follow{
		Id:              id,
		AccountId:       accountId,
		TargetAccountId: targetId,
		URI:             "https://example.com/follows/123",
		CreatedAt:       now,
		Accepted:        true,
		IsLocal:         false,
	}

	if follow.Id != id {
		t.Errorf("Expected Id %s, got %s", id, follow.Id)
	}
	if follow.AccountId != accountId {
		t.Errorf("Expected AccountId %s, got %s", accountId, follow.AccountId)
	}
	if follow.TargetAccountId != targetId {
		t.Errorf("Expected TargetAccountId %s, got %s", targetId, follow.TargetAccountId)
	}
	if !follow.Accepted {
		t.Error("Expected Accepted to be true")
	}
	if follow.IsLocal {
		t.Error("Expected IsLocal to be false")
	}
}

func TestLocalFollow(t *testing.T) {
	follow := Follow{
		Id:              uuid.New(),
		AccountId:       uuid.New(),
		TargetAccountId: uuid.New(),
		URI:             "", // Empty for local follows
		CreatedAt:       time.Now(),
		Accepted:        true,
		IsLocal:         true,
	}

	if follow.URI != "" {
		t.Errorf("Expected empty URI for local follow, got '%s'", follow.URI)
	}
	if !follow.IsLocal {
		t.Error("Expected IsLocal to be true")
	}
}

func TestLikeStruct(t *testing.T) {
	id := uuid.New()
	accountId := uuid.New()
	noteId := uuid.New()
	now := time.Now()

	like := Like{
		Id:        id,
		AccountId: accountId,
		NoteId:    noteId,
		URI:       "https://example.com/likes/123",
		CreatedAt: now,
	}

	if like.Id != id {
		t.Errorf("Expected Id %s, got %s", id, like.Id)
	}
	if like.AccountId != accountId {
		t.Errorf("Expected AccountId %s, got %s", accountId, like.AccountId)
	}
	if like.NoteId != noteId {
		t.Errorf("Expected NoteId %s, got %s", noteId, like.NoteId)
	}
	if like.URI != "https://example.com/likes/123" {
		t.Errorf("Expected URI 'https://example.com/likes/123', got '%s'", like.URI)
	}
}

func TestActivityStruct(t *testing.T) {
	id := uuid.New()
	now := time.Now()

	activity := Activity{
		Id:           id,
		ActivityURI:  "https://example.com/activities/123",
		ActivityType: "Create",
		ActorURI:     "https://example.com/users/actor",
		ObjectURI:    "https://example.com/notes/456",
		RawJSON:      `{"type":"Create"}`,
		Processed:    true,
		CreatedAt:    now,
		Local:        false,
	}

	if activity.Id != id {
		t.Errorf("Expected Id %s, got %s", id, activity.Id)
	}
	if activity.ActivityType != "Create" {
		t.Errorf("Expected ActivityType 'Create', got '%s'", activity.ActivityType)
	}
	if !activity.Processed {
		t.Error("Expected Processed to be true")
	}
	if activity.Local {
		t.Error("Expected Local to be false")
	}
}

func TestActivityTypes(t *testing.T) {
	types := []string{"Follow", "Create", "Like", "Announce", "Undo", "Accept", "Delete"}

	for _, actType := range types {
		activity := Activity{
			Id:           uuid.New(),
			ActivityURI:  "https://example.com/activities/test",
			ActivityType: actType,
			ActorURI:     "https://example.com/users/test",
			ObjectURI:    "https://example.com/objects/test",
			RawJSON:      `{}`,
			Processed:    false,
			CreatedAt:    time.Now(),
			Local:        true,
		}

		if activity.ActivityType != actType {
			t.Errorf("Expected ActivityType '%s', got '%s'", actType, activity.ActivityType)
		}
	}
}

func TestDeliveryQueueItemStruct(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	nextRetry := now.Add(1 * time.Hour)

	item := DeliveryQueueItem{
		Id:           id,
		InboxURI:     "https://example.com/inbox",
		ActivityJSON: `{"type":"Create","object":{}}`,
		Attempts:     3,
		NextRetryAt:  nextRetry,
		CreatedAt:    now,
	}

	if item.Id != id {
		t.Errorf("Expected Id %s, got %s", id, item.Id)
	}
	if item.InboxURI != "https://example.com/inbox" {
		t.Errorf("Expected InboxURI 'https://example.com/inbox', got '%s'", item.InboxURI)
	}
	if item.Attempts != 3 {
		t.Errorf("Expected Attempts 3, got %d", item.Attempts)
	}
	if !item.NextRetryAt.Equal(nextRetry) {
		t.Errorf("Expected NextRetryAt %v, got %v", nextRetry, item.NextRetryAt)
	}
}

func TestDeliveryQueueWithZeroAttempts(t *testing.T) {
	item := DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://example.com/inbox",
		ActivityJSON: `{"type":"Create"}`,
		Attempts:     0,
		NextRetryAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	if item.Attempts != 0 {
		t.Errorf("Expected Attempts 0, got %d", item.Attempts)
	}
}
