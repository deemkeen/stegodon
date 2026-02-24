package createuser

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInitialModel(t *testing.T) {
	model := InitialModel()

	if model.Step != 0 {
		t.Errorf("Expected Step 0, got %d", model.Step)
	}

	if model.Error != "" {
		t.Errorf("Expected no error, got '%s'", model.Error)
	}

	if !model.TextInput.Focused() {
		t.Error("Expected TextInput to be focused")
	}
}

func TestUsernameValidation_ValidCharacters(t *testing.T) {
	validUsernames := []string{
		"alice",
		"alice123",
		"alice-bob",
		"alice.bob",
		"alice_bob",
		"alice~test",
		"test!$&'()*+,;=",
	}

	for _, username := range validUsernames {
		t.Run(username, func(t *testing.T) {
			model := InitialModel()
			model.TextInput.SetValue(username)

			// Simulate pressing enter
			newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

			// Should progress to step 1 (display name)
			if newModel.Step != 1 {
				t.Errorf("Valid username '%s' should progress to step 1, but stayed at step %d. Error: %s",
					username, newModel.Step, newModel.Error)
			}

			if newModel.Error != "" {
				t.Errorf("Valid username '%s' should not have error, got: %s", username, newModel.Error)
			}
		})
	}
}

func TestUsernameValidation_InvalidCharacters(t *testing.T) {
	invalidUsernames := []struct {
		username      string
		expectedError string
	}{
		{"", "must be at least 1 character"},
		{"älice", "invalid characters"},
		{"alice🔥", "invalid characters"},
		{"alice bob", "invalid characters"},
		{"alice@bob", "invalid characters"},
		{"alice#bob", "invalid characters"},
		{"alice:bob", "invalid characters"},
	}

	for _, test := range invalidUsernames {
		t.Run(test.username, func(t *testing.T) {
			model := InitialModel()
			model.TextInput.SetValue(test.username)

			// Simulate pressing enter
			newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

			// Should stay at step 0
			if newModel.Step != 0 {
				t.Errorf("Invalid username '%s' should stay at step 0, got step %d", test.username, newModel.Step)
			}

			// Should have an error
			if newModel.Error == "" {
				t.Errorf("Invalid username '%s' should have an error", test.username)
			}

			// Error should contain expected message
			if !strings.Contains(strings.ToLower(newModel.Error), strings.ToLower(test.expectedError)) {
				t.Errorf("Expected error containing '%s', got '%s'", test.expectedError, newModel.Error)
			}
		})
	}
}

func TestUsernameValidation_ClearErrorOnTyping(t *testing.T) {
	model := InitialModel()
	model.Error = "Some error"

	// Simulate typing
	newModel, _ := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	if newModel.Error != "" {
		t.Errorf("Expected error to be cleared on typing, got '%s'", newModel.Error)
	}
}

func TestUsernameValidation_ClearErrorOnBackspace(t *testing.T) {
	model := InitialModel()
	model.Error = "Some error"

	// Simulate backspace
	newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	if newModel.Error != "" {
		t.Errorf("Expected error to be cleared on backspace, got '%s'", newModel.Error)
	}
}

func TestDisplayNameStep(t *testing.T) {
	model := InitialModel()
	model.Step = 1
	model.DisplayName.SetValue("Alice Test")
	model.DisplayName.Focus()

	// Simulate pressing enter
	newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Should progress to step 2 (bio)
	if newModel.Step != 2 {
		t.Errorf("Expected step 2, got %d", newModel.Step)
	}
}

func TestBioStep(t *testing.T) {
	model := InitialModel()
	model.Step = 2
	model.Bio.SetValue("Test bio")

	// At step 2, pressing enter should be handled by parent (supertui)
	// The model itself doesn't change
	newModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Step should remain 2
	if newModel.Step != 2 {
		t.Errorf("Expected step 2, got %d", newModel.Step)
	}
}

func TestView_Step0(t *testing.T) {
	model := InitialModel()
	view := model.View()

	if !strings.Contains(view.Content, "choose wisely") {
		t.Error("Step 0 view should contain prompt")
	}

	if !strings.Contains(view.Content, "enter to continue") {
		t.Error("Step 0 view should contain help text")
	}
}

func TestView_Step1(t *testing.T) {
	model := InitialModel()
	model.Step = 1
	model.TextInput.SetValue("alice")
	view := model.View()

	if !strings.Contains(view.Content, "Username: alice") {
		t.Error("Step 1 view should show username")
	}

	if !strings.Contains(view.Content, "display name") {
		t.Error("Step 1 view should mention display name")
	}
}

func TestView_Step2(t *testing.T) {
	model := InitialModel()
	model.Step = 2
	model.TextInput.SetValue("alice")
	model.DisplayName.SetValue("Alice Test")
	view := model.View()

	if !strings.Contains(view.Content, "Username: alice") {
		t.Error("Step 2 view should show username")
	}

	if !strings.Contains(view.Content, "Display name: Alice Test") {
		t.Error("Step 2 view should show display name")
	}

	if !strings.Contains(view.Content, "bio") {
		t.Error("Step 2 view should mention bio")
	}
}

func TestView_WithError(t *testing.T) {
	model := InitialModel()
	model.Error = "Test error message"
	view := model.View()

	if !strings.Contains(view.Content, "Test error message") {
		t.Error("View should display error message")
	}
}

func TestCharLimits(t *testing.T) {
	model := InitialModel()

	if model.TextInput.CharLimit != 15 {
		t.Errorf("Username char limit should be 15, got %d", model.TextInput.CharLimit)
	}

	if model.DisplayName.CharLimit != 50 {
		t.Errorf("Display name char limit should be 50, got %d", model.DisplayName.CharLimit)
	}

	if model.Bio.CharLimit != 200 {
		t.Errorf("Bio char limit should be 200, got %d", model.Bio.CharLimit)
	}
}
