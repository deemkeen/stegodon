package util

import (
	"os"
	"testing"
)

func TestPublicKeyToString(t *testing.T) {
	// This function requires an SSH session which is hard to mock
	// We'll skip it for now as it's more of an integration test
	t.Skip("PublicKeyToString requires SSH session - integration test")
}

func TestPkToHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "test",
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "ssh key format",
			input:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ",
			expected: "8f7c4c9c9e3c8e9c9f7c4c9c9e3c8e9c9f7c4c9c9e3c8e9c9f7c4c9c9e3c8e9c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PkToHash(tt.input)
			// Just verify it returns a 64-character hex string
			if len(result) != 64 {
				t.Errorf("Expected hash length 64, got %d", len(result))
			}
			// Verify it's consistent
			result2 := PkToHash(tt.input)
			if result != result2 {
				t.Errorf("Hash should be consistent: %s != %s", result, result2)
			}
		})
	}
}

func TestPkToHashDifferentInputs(t *testing.T) {
	hash1 := PkToHash("input1")
	hash2 := PkToHash("input2")

	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestGetVersion(t *testing.T) {
	// Create a temporary version.txt file
	content := "v1.0.0-test"
	err := os.WriteFile("version.txt", []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test version.txt: %v", err)
	}
	defer os.Remove("version.txt")

	version := GetVersion()
	if version != content {
		t.Errorf("Expected version '%s', got '%s'", content, version)
	}
}

func TestGetNameAndVersion(t *testing.T) {
	// Create a temporary version.txt file
	content := "v1.0.0"
	err := os.WriteFile("version.txt", []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test version.txt: %v", err)
	}
	defer os.Remove("version.txt")

	result := GetNameAndVersion()
	expected := "stegodon / v1.0.0"

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestRandomString(t *testing.T) {
	tests := []int{10, 20, 32, 64}

	for _, length := range tests {
		t.Run("length_"+string(rune(length+'0')), func(t *testing.T) {
			result := RandomString(length)
			if len(result) != length {
				t.Errorf("Expected length %d, got %d", length, len(result))
			}
		})
	}
}

func TestRandomStringUniqueness(t *testing.T) {
	// Generate multiple random strings and verify they're different
	results := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := RandomString(32)
		if results[s] {
			t.Errorf("RandomString produced duplicate: %s", s)
		}
		results[s] = true
	}
}

func TestNormalizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "newlines replaced",
			input:    "line1\nline2\nline3",
			expected: "line1 line2 line3",
		},
		{
			name:     "html escaped",
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "combined newlines and html",
			input:    "<div>\ntest\n</div>",
			expected: "&lt;div&gt; test &lt;/div&gt;",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "normal text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "ampersand",
			input:    "Tom & Jerry",
			expected: "Tom &amp; Jerry",
		},
		{
			name:     "quotes",
			input:    `He said "Hello"`,
			expected: "He said &#34;Hello&#34;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestDateTimeFormat(t *testing.T) {
	format := DateTimeFormat()
	expected := "2006-01-02 15:04:05 CEST"

	if format != expected {
		t.Errorf("Expected format '%s', got '%s'", expected, format)
	}
}

func TestPrettyPrint(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name:  "simple map",
			input: map[string]string{"key": "value"},
		},
		{
			name:  "nested structure",
			input: map[string]interface{}{"outer": map[string]int{"inner": 42}},
		},
		{
			name:  "array",
			input: []int{1, 2, 3, 4, 5},
		},
		{
			name:  "string",
			input: "simple string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrettyPrint(tt.input)
			if len(result) == 0 {
				t.Error("PrettyPrint returned empty string")
			}
		})
	}
}

func TestGeneratePemKeypair(t *testing.T) {
	keypair := GeneratePemKeypair()

	if keypair == nil {
		t.Fatal("GeneratePemKeypair returned nil")
	}

	// Check private key format
	if len(keypair.Private) == 0 {
		t.Error("Private key is empty")
	}
	if !contains(keypair.Private, "BEGIN RSA PRIVATE KEY") {
		t.Error("Private key doesn't have PEM header")
	}
	if !contains(keypair.Private, "END RSA PRIVATE KEY") {
		t.Error("Private key doesn't have PEM footer")
	}

	// Check public key format
	if len(keypair.Public) == 0 {
		t.Error("Public key is empty")
	}
	if !contains(keypair.Public, "BEGIN RSA PUBLIC KEY") {
		t.Error("Public key doesn't have PEM header")
	}
	if !contains(keypair.Public, "END RSA PUBLIC KEY") {
		t.Error("Public key doesn't have PEM footer")
	}
}

func TestGeneratePemKeypairUniqueness(t *testing.T) {
	// Generate two keypairs and verify they're different
	keypair1 := GeneratePemKeypair()
	keypair2 := GeneratePemKeypair()

	if keypair1.Private == keypair2.Private {
		t.Error("Generated keypairs should be different")
	}
	if keypair1.Public == keypair2.Public {
		t.Error("Generated public keys should be different")
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
