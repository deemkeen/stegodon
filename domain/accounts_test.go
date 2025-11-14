package domain

import (
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestAccountToString(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	acc := &Account{
		Id:             id,
		Username:       "testuser",
		Publickey:      "ssh-rsa AAAAB3...",
		CreatedAt:      now,
		FirstTimeLogin: FALSE,
		WebPublicKey:   "-----BEGIN RSA PUBLIC KEY-----",
		WebPrivateKey:  "-----BEGIN RSA PRIVATE KEY-----",
		DisplayName:    "Test User",
		Summary:        "Test bio",
		AvatarURL:      "https://example.com/avatar.png",
	}

	result := acc.ToString()

	// Check that the string contains expected fields
	if len(result) == 0 {
		t.Error("ToString() returned empty string")
	}

	// Should contain username
	if !contains(result, "testuser") {
		t.Errorf("ToString() should contain username, got: %s", result)
	}

	// Should contain ID
	if !contains(result, id.String()) {
		t.Errorf("ToString() should contain ID, got: %s", result)
	}
}

func TestDbBoolConstants(t *testing.T) {
	if FALSE != 0 {
		t.Errorf("FALSE should be 0, got %d", FALSE)
	}
	if TRUE != 1 {
		t.Errorf("TRUE should be 1, got %d", TRUE)
	}
}

func TestAccountStructFields(t *testing.T) {
	acc := Account{
		Id:             uuid.New(),
		Username:       "user123",
		Publickey:      "pubkey",
		CreatedAt:      time.Now(),
		FirstTimeLogin: TRUE,
		WebPublicKey:   "webpub",
		WebPrivateKey:  "webpriv",
		DisplayName:    "Display Name",
		Summary:        "User bio",
		AvatarURL:      "https://avatar.url",
	}

	// Test that fields are correctly set
	if acc.Username != "user123" {
		t.Errorf("Expected Username 'user123', got '%s'", acc.Username)
	}
	if acc.FirstTimeLogin != TRUE {
		t.Errorf("Expected FirstTimeLogin TRUE, got %d", acc.FirstTimeLogin)
	}
	if acc.DisplayName != "Display Name" {
		t.Errorf("Expected DisplayName 'Display Name', got '%s'", acc.DisplayName)
	}
	if acc.Summary != "User bio" {
		t.Errorf("Expected Summary 'User bio', got '%s'", acc.Summary)
	}
	if acc.AvatarURL != "https://avatar.url" {
		t.Errorf("Expected AvatarURL 'https://avatar.url', got '%s'", acc.AvatarURL)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
