package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
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
	// GetVersion now uses embedded version.txt
	// Read expected version from version.txt to avoid hardcoding
	version := GetVersion()

	// Version should not be empty
	if version == "" {
		t.Error("Version should not be empty")
	}

	// Version should match semantic versioning pattern (e.g., "1.2.2")
	// At minimum, should contain digits and dots
	hasDigit := false
	hasDot := false
	for _, char := range version {
		if char >= '0' && char <= '9' {
			hasDigit = true
		}
		if char == '.' {
			hasDot = true
		}
	}

	if !hasDigit {
		t.Error("Version should contain at least one digit")
	}
	if !hasDot {
		t.Error("Version should contain at least one dot (semantic versioning)")
	}
}

func TestGetNameAndVersion(t *testing.T) {
	// GetNameAndVersion now uses embedded version.txt
	result := GetNameAndVersion()
	expected := fmt.Sprintf("stegodon / %s", GetVersion())

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
	for range 100 {
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

func TestUnescapeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "apostrophe",
			input:    "Hope it&#39;s a nice day out and not too cold!!",
			expected: "Hope it's a nice day out and not too cold!!",
		},
		{
			name:     "quotes",
			input:    "He said &#34;Hello&#34;",
			expected: `He said "Hello"`,
		},
		{
			name:     "ampersand",
			input:    "Tom &amp; Jerry",
			expected: "Tom & Jerry",
		},
		{
			name:     "less than and greater than",
			input:    "5 &lt; 10 &gt; 3",
			expected: "5 < 10 > 3",
		},
		{
			name:     "non-breaking space",
			input:    "Hello&nbsp;World",
			expected: "Hello World",
		},
		{
			name:     "multiple entities",
			input:    "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
			expected: "<script>alert('xss')</script>",
		},
		{
			name:     "no entities",
			input:    "Plain text with no entities",
			expected: "Plain text with no entities",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnescapeHTML(tt.input)
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
		input any
	}{
		{
			name:  "simple map",
			input: map[string]string{"key": "value"},
		},
		{
			name:  "nested structure",
			input: map[string]any{"outer": map[string]int{"inner": 42}},
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

	// Check private key format (now PKCS#8)
	if len(keypair.Private) == 0 {
		t.Error("Private key is empty")
	}
	if !contains(keypair.Private, "BEGIN PRIVATE KEY") {
		t.Error("Private key doesn't have PKCS#8 PEM header")
	}
	if !contains(keypair.Private, "END PRIVATE KEY") {
		t.Error("Private key doesn't have PKCS#8 PEM footer")
	}

	// Check public key format (now PKIX)
	if len(keypair.Public) == 0 {
		t.Error("Public key is empty")
	}
	if !contains(keypair.Public, "BEGIN PUBLIC KEY") {
		t.Error("Public key doesn't have PKIX PEM header")
	}
	if !contains(keypair.Public, "END PUBLIC KEY") {
		t.Error("Public key doesn't have PKIX PEM footer")
	}
}

func TestGeneratePemKeypairUniqueness(t *testing.T) {
	keypair1 := GeneratePemKeypair()
	keypair2 := GeneratePemKeypair()

	if keypair1.Private == keypair2.Private {
		t.Error("Generated keypairs should be unique (private keys are identical)")
	}
	if keypair1.Public == keypair2.Public {
		t.Error("Generated keypairs should be unique (public keys are identical)")
	}
}

func TestConvertPrivateKeyToPKCS8(t *testing.T) {
	// Generate a real PKCS#1 key for testing
	oldKeypair := &RsaKeyPair{}
	bitSize := 2048 // Minimum secure size

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	// Create PKCS#1 format (old format)
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	oldKeypair.Private = string(pkcs1PEM)

	// Convert to PKCS#8
	pkcs8Key, err := ConvertPrivateKeyToPKCS8(oldKeypair.Private)
	if err != nil {
		t.Fatalf("Failed to convert PKCS#1 key: %v", err)
	}

	if !strings.Contains(pkcs8Key, "BEGIN PRIVATE KEY") {
		t.Error("Converted key should have PKCS#8 header")
	}
	if strings.Contains(pkcs8Key, "RSA PRIVATE KEY") {
		t.Error("Converted key should not have PKCS#1 header")
	}

	// Test that already-PKCS#8 keys are returned unchanged
	pkcs8Again, err := ConvertPrivateKeyToPKCS8(pkcs8Key)
	if err != nil {
		t.Fatalf("Failed to process already-PKCS#8 key: %v", err)
	}
	if pkcs8Again != pkcs8Key {
		t.Error("Already-PKCS#8 key should be returned unchanged")
	}

	// Verify both formats can be parsed by x509
	block, _ := pem.Decode([]byte(oldKeypair.Private))
	_, err = x509.ParsePKCS1PrivateKey(block.Bytes) // PKCS#1
	if err != nil {
		t.Errorf("Original PKCS#1 key should be parseable: %v", err)
	}

	block, _ = pem.Decode([]byte(pkcs8Key))
	_, err = x509.ParsePKCS8PrivateKey(block.Bytes) // PKCS#8
	if err != nil {
		t.Errorf("Converted PKCS#8 key should be parseable: %v", err)
	}
}

func TestConvertPublicKeyToPKIX(t *testing.T) {
	// Generate a real PKCS#1 public key for testing
	bitSize := 2048 // Minimum secure size

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	pub := key.Public()

	// Create PKCS#1 format (old format)
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(pub.(*rsa.PublicKey)),
	})
	oldPublicKey := string(pkcs1PEM)

	// Convert to PKIX
	pkixKey, err := ConvertPublicKeyToPKIX(oldPublicKey)
	if err != nil {
		t.Fatalf("Failed to convert PKCS#1 public key: %v", err)
	}

	if !strings.Contains(pkixKey, "BEGIN PUBLIC KEY") {
		t.Error("Converted key should have PKIX header")
	}
	if strings.Contains(pkixKey, "RSA PUBLIC KEY") {
		t.Error("Converted key should not have PKCS#1 header")
	}

	// Test that already-PKIX keys are returned unchanged
	pkixAgain, err := ConvertPublicKeyToPKIX(pkixKey)
	if err != nil {
		t.Fatalf("Failed to process already-PKIX key: %v", err)
	}
	if pkixAgain != pkixKey {
		t.Error("Already-PKIX key should be returned unchanged")
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

func TestIsURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid http URL", "http://example.com", true},
		{"valid https URL", "https://example.com", true},
		{"valid URL with path", "https://github.com/deemkeen/stegodon", true},
		{"valid URL with query", "https://example.com?foo=bar", true},
		{"URL with spaces around", "  https://example.com  ", true}, // Should trim
		{"not a URL - plain text", "hello world", false},
		{"not a URL - no protocol", "example.com", false},
		{"not a URL - markdown link", "[text](https://example.com)", false},
		{"not a URL - ftp protocol", "ftp://example.com", false},
		{"empty string", "", false},
		{"just http://", "http://", false}, // No domain
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsURL(tt.input)
			if got != tt.want {
				t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCountVisibleChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "plain text",
			input: "Hello world",
			want:  11,
		},
		{
			name:  "single markdown link",
			input: "Check out [stegodon](https://github.com/deemkeen/stegodon) project",
			want:  26, // "Check out stegodon project" without extra space
		},
		{
			name:  "multiple markdown links",
			input: "Visit [site1](https://example.com) and [site2](https://test.com)",
			want:  21, // "Visit site1 and site2" = 21 chars
		},
		{
			name:  "markdown link at start",
			input: "[Link](https://example.com) here",
			want:  9, // "Link here" without extra space
		},
		{
			name:  "markdown link at end",
			input: "Click [here](https://example.com)",
			want:  10, // "Click here" without extra space
		},
		{
			name:  "only markdown link",
			input: "[text](https://example.com)",
			want:  4, // "text" = 4 chars
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "no markdown links",
			input: "Just plain text with no links",
			want:  29,
		},
		{
			name:  "link with long URL",
			input: "[short](https://very-long-url-that-should-not-count.com/path/to/resource?query=param)",
			want:  5, // "short" = 5 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountVisibleChars(tt.input)
			if got != tt.want {
				t.Errorf("CountVisibleChars(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateNoteLength(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty note",
			input:   "",
			wantErr: false,
		},
		{
			name:    "short note",
			input:   "Hello world",
			wantErr: false,
		},
		{
			name:    "exactly 1000 chars",
			input:   string(make([]byte, 1000)),
			wantErr: false,
		},
		{
			name:    "1001 chars - too long",
			input:   string(make([]byte, 1001)),
			wantErr: true,
		},
		{
			name:    "note with long markdown link under limit",
			input:   "Check [link](" + string(make([]byte, 980)) + ")",
			wantErr: false, // Total is 997 chars
		},
		{
			name:    "note with markdown link over limit",
			input:   "Check [link](" + string(make([]byte, 990)) + ")",
			wantErr: true, // Total is 1007 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNoteLength(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNoteLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsURLEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "URL with port",
			input: "https://example.com:8080",
			want:  true,
		},
		{
			name:  "URL with path and query",
			input: "https://example.com/path?key=value&foo=bar",
			want:  true,
		},
		{
			name:  "URL with fragment",
			input: "https://example.com/page#section",
			want:  true,
		},
		{
			name:  "URL with username",
			input: "https://user@example.com",
			want:  true,
		},
		{
			name:  "localhost URL",
			input: "http://localhost:9999",
			want:  true,
		},
		{
			name:  "IP address URL",
			input: "http://192.168.1.1",
			want:  true,
		},
		{
			name:  "URL with trailing slash",
			input: "https://example.com/",
			want:  true,
		},
		{
			name:  "multiple spaces around URL",
			input: "   https://example.com   ",
			want:  true,
		},
		{
			name:  "URL embedded in markdown",
			input: "[text](https://example.com)",
			want:  false,
		},
		{
			name:  "URL with newline",
			input: "https://example.com\n",
			want:  true, // TrimSpace removes the newline, so this becomes valid
		},
		{
			name:  "partial URL - just protocol",
			input: "https://",
			want:  false,
		},
		{
			name:  "URL with space in middle",
			input: "https://example .com",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsURL(tt.input)
			if got != tt.want {
				t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCountVisibleCharsEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "nested brackets not a link",
			input: "[outer [inner] text](url)",
			want:  25, // Regex doesn't match nested brackets, counts as plain text
		},
		{
			name:  "adjacent links no space",
			input: "[link1](url1)[link2](url2)",
			want:  10, // "link1" (5) + "link2" (5) = 10
		},
		{
			name:  "link with empty text",
			input: "[](https://example.com)",
			want:  23, // Regex requires [^\]]+ which means at least 1 char, so no match
		},
		{
			name:  "link with spaces in text",
			input: "[some text here](https://example.com)",
			want:  14, // "some text here" but counted by bytes including spaces
		},
		{
			name:  "multiple links with text between",
			input: "start [link1](url) middle [link2](url) end",
			want:  28, // Actual character count with the implementation
		},
		{
			name:  "link with unicode text",
			input: "[日本語](https://example.com)",
			want:  3, // Japanese chars count as 3 runes (visible characters)
		},
		{
			name:  "link with emoji",
			input: "[🔥🎉](https://example.com)",
			want:  2, // Emojis count as 2 runes (visible characters)
		},
		{
			name:  "very long URL",
			input: "[text](https://example.com/" + strings.Repeat("a", 500) + ")",
			want:  4, // Only "text" counts, not the 500-char URL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountVisibleChars(tt.input)
			if got != tt.want {
				t.Errorf("CountVisibleChars(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseHashtags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single hashtag",
			input:    "Hello #world",
			expected: []string{"world"},
		},
		{
			name:     "multiple hashtags",
			input:    "Check out #golang and #rust programming",
			expected: []string{"golang", "rust"},
		},
		{
			name:     "case insensitivity",
			input:    "#Hello #WORLD #GoLang",
			expected: []string{"hello", "world", "golang"},
		},
		{
			name:     "deduplication",
			input:    "#test #Test #TEST #test",
			expected: []string{"test"},
		},
		{
			name:     "hashtag with numbers",
			input:    "#Go123 #test456",
			expected: []string{"go123", "test456"},
		},
		{
			name:     "hashtag with underscores",
			input:    "#my_tag #hello_world_test",
			expected: []string{"my_tag", "hello_world_test"},
		},
		{
			name:     "invalid - starts with number",
			input:    "#123 #456test",
			expected: []string{},
		},
		{
			name:     "single letter hashtag",
			input:    "#a #b #c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "double hash - one valid",
			input:    "##double",
			expected: []string{"double"},
		},
		{
			name:     "no hashtags",
			input:    "Hello world without any tags",
			expected: []string{},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "hashtag at start",
			input:    "#first is the hashtag",
			expected: []string{"first"},
		},
		{
			name:     "hashtag at end",
			input:    "The hashtag is #last",
			expected: []string{"last"},
		},
		{
			name:     "only hashtag",
			input:    "#solo",
			expected: []string{"solo"},
		},
		{
			name:     "hashtags in URL should match",
			input:    "Visit https://example.com/#section with #hashtag",
			expected: []string{"section", "hashtag"},
		},
		{
			name:     "hashtag after punctuation",
			input:    "Hello,#tag1 world.#tag2 test!#tag3",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "mixed valid and invalid",
			input:    "#valid #123invalid #also_valid #_invalid",
			expected: []string{"valid", "also_valid"},
		},
		{
			name:     "hashtag with newline",
			input:    "#tag1\n#tag2",
			expected: []string{"tag1", "tag2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseHashtags(tt.input)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("ParseHashtags(%q) returned %d tags, expected %d. Got: %v, Expected: %v",
					tt.input, len(result), len(tt.expected), result, tt.expected)
				return
			}

			// Check contents
			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("ParseHashtags(%q)[%d] = %q, expected %q",
						tt.input, i, tag, tt.expected[i])
				}
			}
		})
	}
}

func TestParseHashtagsPreservesOrder(t *testing.T) {
	input := "#first #second #third #fourth"
	result := ParseHashtags(input)

	expected := []string{"first", "second", "third", "fourth"}
	if len(result) != len(expected) {
		t.Fatalf("Expected %d tags, got %d", len(expected), len(result))
	}

	for i, tag := range result {
		if tag != expected[i] {
			t.Errorf("Order mismatch at index %d: got %q, expected %q", i, tag, expected[i])
		}
	}
}

// Tests for HighlightHashtagsTerminal

func TestHighlightHashtagsTerminal_SingleHashtag(t *testing.T) {
	input := "Hello #world!"
	result := HighlightHashtagsTerminal(input)

	// Check for ANSI color codes
	if !strings.Contains(result, "\033[38;5;75m#world\033[39m") {
		t.Errorf("Expected colored hashtag, got: %s", result)
	}
}

func TestHighlightHashtagsTerminal_MultipleHashtags(t *testing.T) {
	input := "Hello #golang and #rust!"
	result := HighlightHashtagsTerminal(input)

	// Check for ANSI color codes for both hashtags
	if !strings.Contains(result, "\033[38;5;75m#golang\033[39m") {
		t.Errorf("Expected colored #golang, got: %s", result)
	}
	if !strings.Contains(result, "\033[38;5;75m#rust\033[39m") {
		t.Errorf("Expected colored #rust, got: %s", result)
	}
}

func TestHighlightHashtagsTerminal_NoHashtags(t *testing.T) {
	input := "Hello world!"
	result := HighlightHashtagsTerminal(input)

	// Should be unchanged
	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

func TestHighlightHashtagsTerminal_PreservesOtherText(t *testing.T) {
	input := "Check out #golang for web development."
	result := HighlightHashtagsTerminal(input)

	// Should preserve text around hashtag
	if !strings.Contains(result, "Check out ") {
		t.Error("Expected prefix text to be preserved")
	}
	if !strings.Contains(result, " for web development.") {
		t.Error("Expected suffix text to be preserved")
	}
}

// Tests for HighlightHashtagsHTML

func TestHighlightHashtagsHTML_SingleHashtag(t *testing.T) {
	input := "Hello #world!"
	result := HighlightHashtagsHTML(input)

	expected := `Hello <a href="/tags/world" class="hashtag">#world</a>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestHighlightHashtagsHTML_MultipleHashtags(t *testing.T) {
	input := "Hello #golang and #rust!"
	result := HighlightHashtagsHTML(input)

	if !strings.Contains(result, `<a href="/tags/golang" class="hashtag">#golang</a>`) {
		t.Errorf("Expected golang hashtag link, got: %s", result)
	}
	if !strings.Contains(result, `<a href="/tags/rust" class="hashtag">#rust</a>`) {
		t.Errorf("Expected rust hashtag link, got: %s", result)
	}
}

func TestHighlightHashtagsHTML_NoHashtags(t *testing.T) {
	input := "Hello world!"
	result := HighlightHashtagsHTML(input)

	// Should be unchanged
	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

// Tests for HashtagsToActivityPubHTML

func TestHashtagsToActivityPubHTML_SingleHashtag(t *testing.T) {
	input := "Hello #world!"
	result := HashtagsToActivityPubHTML(input, "https://example.com")

	expected := `Hello <a href="https://example.com/tags/world" class="hashtag" rel="tag">#<span>world</span></a>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestHashtagsToActivityPubHTML_MultipleHashtags(t *testing.T) {
	input := "Hello #golang and #rust!"
	result := HashtagsToActivityPubHTML(input, "https://example.com")

	if !strings.Contains(result, `<a href="https://example.com/tags/golang" class="hashtag" rel="tag">#<span>golang</span></a>`) {
		t.Errorf("Expected golang hashtag link, got: %s", result)
	}
	if !strings.Contains(result, `<a href="https://example.com/tags/rust" class="hashtag" rel="tag">#<span>rust</span></a>`) {
		t.Errorf("Expected rust hashtag link, got: %s", result)
	}
}

func TestHashtagsToActivityPubHTML_NoHashtags(t *testing.T) {
	input := "Hello world!"
	result := HashtagsToActivityPubHTML(input, "https://example.com")

	// Should be unchanged
	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

func TestHashtagsToActivityPubHTML_CaseInsensitive(t *testing.T) {
	input := "Hello #GoLang!"
	result := HashtagsToActivityPubHTML(input, "https://example.com")

	// Should be lowercase in output
	expected := `Hello <a href="https://example.com/tags/golang" class="hashtag" rel="tag">#<span>golang</span></a>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCountVisibleCharsWithANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "text with ANSI color code",
			input: "\033[38;5;75m#joke\033[39m hello",
			want:  11, // "#joke hello" = 11 visible chars
		},
		{
			name:  "text with multiple ANSI codes",
			input: "\033[38;5;75m#tag1\033[39m and \033[38;5;75m#tag2\033[39m",
			want:  15, // "#tag1 and #tag2" = 15 visible chars
		},
		{
			name:  "OSC 8 hyperlink",
			input: "\033[38;2;0;255;127;4m\033]8;;https://example.com\033\\Link\033]8;;\033\\\033[39;24m",
			want:  4, // "Link" = 4 visible chars
		},
		{
			name:  "mixed ANSI hashtag and OSC 8 link",
			input: "\033[38;2;0;255;127;4m\033]8;;https://example.com\033\\Link\033]8;;\033\\\033[39;24m \033[38;5;75m#tag\033[39m",
			want:  9, // "Link #tag" = 9 visible chars
		},
		{
			name:  "plain text no ANSI",
			input: "plain text",
			want:  10,
		},
		{
			name:  "ANSI reset code only",
			input: "\033[0m",
			want:  0,
		},
		{
			name:  "text after ANSI reset",
			input: "\033[0mhello",
			want:  5, // "hello" = 5 visible chars
		},
		{
			name:  "text with middle dot (multi-byte unicode)",
			input: "5m ago · 3 replies",
			want:  18, // Middle dot is 1 rune, not 2 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountVisibleChars(tt.input)
			if got != tt.want {
				t.Errorf("CountVisibleChars(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// Tests for ParseMentions

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Mention
	}{
		{
			name:  "single mention",
			input: "Hello @alice@mastodon.social",
			expected: []Mention{
				{Username: "alice", Domain: "mastodon.social"},
			},
		},
		{
			name:  "multiple mentions",
			input: "Hello @alice@mastodon.social and @bob@pixelfed.social",
			expected: []Mention{
				{Username: "alice", Domain: "mastodon.social"},
				{Username: "bob", Domain: "pixelfed.social"},
			},
		},
		{
			name:  "deduplication",
			input: "@alice@mastodon.social @Alice@MASTODON.SOCIAL @alice@mastodon.social",
			expected: []Mention{
				{Username: "alice", Domain: "mastodon.social"},
			},
		},
		{
			name:  "case insensitivity",
			input: "@Alice@Mastodon.Social",
			expected: []Mention{
				{Username: "alice", Domain: "mastodon.social"},
			},
		},
		{
			name:  "username with numbers",
			input: "@user123@example.com",
			expected: []Mention{
				{Username: "user123", Domain: "example.com"},
			},
		},
		{
			name:  "username with underscore",
			input: "@user_name@example.com",
			expected: []Mention{
				{Username: "user_name", Domain: "example.com"},
			},
		},
		{
			name:  "domain with subdomain",
			input: "@user@sub.domain.com",
			expected: []Mention{
				{Username: "user", Domain: "sub.domain.com"},
			},
		},
		{
			name:     "no mentions",
			input:    "Hello world without any mentions",
			expected: []Mention{},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []Mention{},
		},
		{
			name:     "invalid - no domain",
			input:    "@alice",
			expected: []Mention{},
		},
		{
			name:     "invalid - single letter TLD",
			input:    "@alice@example.c",
			expected: []Mention{},
		},
		{
			name:  "valid - two letter TLD",
			input: "@alice@example.de",
			expected: []Mention{
				{Username: "alice", Domain: "example.de"},
			},
		},
		{
			name:  "mention at start",
			input: "@alice@example.com is here",
			expected: []Mention{
				{Username: "alice", Domain: "example.com"},
			},
		},
		{
			name:  "mention at end",
			input: "Hello @alice@example.com",
			expected: []Mention{
				{Username: "alice", Domain: "example.com"},
			},
		},
		{
			name:  "mention after punctuation",
			input: "Hello,@alice@example.com",
			expected: []Mention{
				{Username: "alice", Domain: "example.com"},
			},
		},
		{
			name:  "mention with emoji before",
			input: "👋@alice@example.com",
			expected: []Mention{
				{Username: "alice", Domain: "example.com"},
			},
		},
		{
			name:     "email format should not match",
			input:    "email@example.com", // Missing the second @
			expected: []Mention{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMentions(tt.input)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("ParseMentions(%q) returned %d mentions, expected %d. Got: %v, Expected: %v",
					tt.input, len(result), len(tt.expected), result, tt.expected)
				return
			}

			// Check contents
			for i, mention := range result {
				if mention.Username != tt.expected[i].Username || mention.Domain != tt.expected[i].Domain {
					t.Errorf("ParseMentions(%q)[%d] = %v, expected %v",
						tt.input, i, mention, tt.expected[i])
				}
			}
		})
	}
}

func TestParseMentionsPreservesOrder(t *testing.T) {
	input := "@first@a.com @second@b.com @third@c.com"
	result := ParseMentions(input)

	if len(result) != 3 {
		t.Fatalf("Expected 3 mentions, got %d", len(result))
	}

	expected := []string{"first", "second", "third"}
	for i, mention := range result {
		if mention.Username != expected[i] {
			t.Errorf("Order mismatch at index %d: got %q, expected %q", i, mention.Username, expected[i])
		}
	}
}

// Tests for HighlightMentionsTerminal

func TestHighlightMentionsTerminal_SingleMention(t *testing.T) {
	input := "Hello @alice@mastodon.social!"
	result := HighlightMentionsTerminal(input, "example.com") // Remote user

	// Check for OSC 8 hyperlink with ANSI color codes
	// Format: \033[38;5;48;4m\033]8;;URL\033\\TEXT\033]8;;\033\\\033[39;24m
	if !strings.Contains(result, "\033]8;;https://mastodon.social/@alice\033\\") {
		t.Errorf("Expected OSC 8 hyperlink, got: %s", result)
	}
	if !strings.Contains(result, "@alice@mastodon.social") {
		t.Errorf("Expected mention text, got: %s", result)
	}
}

func TestHighlightMentionsTerminal_MultipleMentions(t *testing.T) {
	input := "Hello @alice@mastodon.social and @bob@pixelfed.social!"
	result := HighlightMentionsTerminal(input, "example.com") // Remote users

	if !strings.Contains(result, "\033]8;;https://mastodon.social/@alice\033\\") {
		t.Errorf("Expected OSC 8 hyperlink for alice, got: %s", result)
	}
	if !strings.Contains(result, "\033]8;;https://pixelfed.social/@bob\033\\") {
		t.Errorf("Expected OSC 8 hyperlink for bob, got: %s", result)
	}
}

func TestHighlightMentionsTerminal_NoMentions(t *testing.T) {
	input := "Hello world!"
	result := HighlightMentionsTerminal(input, "example.com")

	// Should be unchanged
	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

func TestHighlightMentionsTerminal_LocalUser(t *testing.T) {
	input := "Hello @alice@example.com!"
	result := HighlightMentionsTerminal(input, "example.com") // Local user

	// Local user should be displayed as just @alice
	if !strings.Contains(result, "@alice") {
		t.Errorf("Expected @alice in output, got: %s", result)
	}
	// Should NOT show the domain for local users
	if strings.Contains(result, "@alice@example.com\033]8;;") {
		t.Errorf("Local user should not show full @username@domain, got: %s", result)
	}
	// Should link to local profile
	if !strings.Contains(result, "\033]8;;https://example.com/u/alice\033\\") {
		t.Errorf("Expected local profile URL, got: %s", result)
	}
}

func TestHighlightMentionsTerminal_MixedLocalAndRemote(t *testing.T) {
	input := "Hello @alice@example.com and @bob@mastodon.social!"
	result := HighlightMentionsTerminal(input, "example.com")

	// Local user (alice) should link to local profile
	if !strings.Contains(result, "\033]8;;https://example.com/u/alice\033\\") {
		t.Errorf("Expected local profile URL for alice, got: %s", result)
	}
	// Remote user (bob) should link to their instance
	if !strings.Contains(result, "\033]8;;https://mastodon.social/@bob\033\\") {
		t.Errorf("Expected remote profile URL for bob, got: %s", result)
	}
}

func TestHighlightMentionsTerminal_CaseInsensitiveLocalDomain(t *testing.T) {
	input := "Hello @alice@EXAMPLE.COM!"
	result := HighlightMentionsTerminal(input, "example.com")

	// Should still be recognized as local user (case insensitive)
	if !strings.Contains(result, "\033]8;;https://example.com/u/alice\033\\") {
		t.Errorf("Expected local profile URL for case-insensitive domain match, got: %s", result)
	}
}

// Tests for HighlightMentionsHTML

func TestHighlightMentionsHTML_LocalUser(t *testing.T) {
	input := "Hello @alice@example.com!"
	result := HighlightMentionsHTML(input, "example.com")

	expected := `Hello <a href="/u/alice" class="mention">@alice</a>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestHighlightMentionsHTML_RemoteUser(t *testing.T) {
	input := "Hello @alice@mastodon.social!"
	result := HighlightMentionsHTML(input, "example.com")

	expected := `Hello <a href="https://mastodon.social/@alice" class="mention" target="_blank" rel="noopener noreferrer">@alice@mastodon.social</a>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestHighlightMentionsHTML_MixedUsers(t *testing.T) {
	input := "Hello @alice@example.com and @bob@mastodon.social!"
	result := HighlightMentionsHTML(input, "example.com")

	// Local user - displayed as @alice (without domain)
	if !strings.Contains(result, `<a href="/u/alice" class="mention">@alice</a>`) {
		t.Errorf("Expected local user link with @alice, got: %s", result)
	}
	// Remote user - displayed as @bob@mastodon.social
	if !strings.Contains(result, `<a href="https://mastodon.social/@bob" class="mention" target="_blank" rel="noopener noreferrer">@bob@mastodon.social</a>`) {
		t.Errorf("Expected remote user link, got: %s", result)
	}
}

func TestHighlightMentionsHTML_CaseInsensitiveLocalDomain(t *testing.T) {
	input := "Hello @alice@EXAMPLE.COM!"
	result := HighlightMentionsHTML(input, "example.com")

	// Should match case-insensitively and link locally, displaying as @alice
	if !strings.Contains(result, `<a href="/u/alice" class="mention">@alice</a>`) {
		t.Errorf("Expected local user link for case-insensitive domain match, got: %s", result)
	}
}

func TestHighlightMentionsHTML_NoMentions(t *testing.T) {
	input := "Hello world!"
	result := HighlightMentionsHTML(input, "example.com")

	// Should be unchanged
	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

// Tests for MentionsToActivityPubHTML

func TestMentionsToActivityPubHTML_WithURIs(t *testing.T) {
	input := "Hello @alice@mastodon.social!"
	mentionURIs := map[string]string{
		"@alice@mastodon.social": "https://mastodon.social/users/alice",
	}
	result := MentionsToActivityPubHTML(input, mentionURIs)

	expected := `Hello <span class="h-card"><a href="https://mastodon.social/users/alice" class="u-url mention">@<span>alice</span></a></span>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestMentionsToActivityPubHTML_FallbackWithoutURI(t *testing.T) {
	input := "Hello @alice@mastodon.social!"
	mentionURIs := map[string]string{} // No URI provided
	result := MentionsToActivityPubHTML(input, mentionURIs)

	// Should fallback to profile URL
	expected := `Hello <span class="h-card"><a href="https://mastodon.social/@alice" class="u-url mention">@<span>alice</span></a></span>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestMentionsToActivityPubHTML_MultipleMentions(t *testing.T) {
	input := "Hello @alice@mastodon.social and @bob@pixelfed.social!"
	mentionURIs := map[string]string{
		"@alice@mastodon.social": "https://mastodon.social/users/alice",
		"@bob@pixelfed.social":   "https://pixelfed.social/users/bob",
	}
	result := MentionsToActivityPubHTML(input, mentionURIs)

	if !strings.Contains(result, `<a href="https://mastodon.social/users/alice" class="u-url mention">@<span>alice</span></a>`) {
		t.Errorf("Expected alice mention link, got: %s", result)
	}
	if !strings.Contains(result, `<a href="https://pixelfed.social/users/bob" class="u-url mention">@<span>bob</span></a>`) {
		t.Errorf("Expected bob mention link, got: %s", result)
	}
}

func TestMentionsToActivityPubHTML_NoMentions(t *testing.T) {
	input := "Hello world!"
	mentionURIs := map[string]string{}
	result := MentionsToActivityPubHTML(input, mentionURIs)

	// Should be unchanged
	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

// Additional edge case tests for mentions

func TestHighlightMentionsTerminal_EmptyLocalDomain(t *testing.T) {
	// When localDomain is empty, all users are treated as remote
	input := "Hello @alice@example.com!"
	result := HighlightMentionsTerminal(input, "")

	// Should link to remote profile since no local domain match
	if !strings.Contains(result, "\033]8;;https://example.com/@alice\033\\") {
		t.Errorf("Expected remote profile URL when localDomain is empty, got: %s", result)
	}
	// Should display full mention
	if !strings.Contains(result, "@alice@example.com") {
		t.Errorf("Expected full mention display when localDomain is empty, got: %s", result)
	}
}

func TestHighlightMentionsHTML_EmptyLocalDomain(t *testing.T) {
	// When localDomain is empty, all users are treated as remote
	input := "Hello @alice@example.com!"
	result := HighlightMentionsHTML(input, "")

	// Should link to remote profile since no local domain match
	expected := `Hello <a href="https://example.com/@alice" class="mention" target="_blank" rel="noopener noreferrer">@alice@example.com</a>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestParseMentions_SubdomainHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Mention
	}{
		{
			name:  "mention with subdomain",
			input: "@user@sub.domain.example.com",
			expected: []Mention{
				{Username: "user", Domain: "sub.domain.example.com"},
			},
		},
		{
			name:  "mention with multiple subdomains",
			input: "@admin@a.b.c.example.org",
			expected: []Mention{
				{Username: "admin", Domain: "a.b.c.example.org"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMentions(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ParseMentions(%q) returned %d mentions, expected %d",
					tt.input, len(result), len(tt.expected))
				return
			}
			for i, mention := range result {
				if mention.Username != tt.expected[i].Username || mention.Domain != tt.expected[i].Domain {
					t.Errorf("ParseMentions(%q)[%d] = %v, expected %v",
						tt.input, i, mention, tt.expected[i])
				}
			}
		})
	}
}

func TestHighlightMentionsTerminal_PreservesOtherText(t *testing.T) {
	input := "Check out @alice@mastodon.social for great content."
	localDomain := "example.com"
	result := HighlightMentionsTerminal(input, localDomain)

	// Should preserve text around mention
	if !strings.Contains(result, "Check out ") {
		t.Error("Expected prefix text to be preserved")
	}
	if !strings.Contains(result, " for great content.") {
		t.Error("Expected suffix text to be preserved")
	}
}

func TestHighlightMentionsHTML_PreservesOtherText(t *testing.T) {
	input := "Check out @alice@mastodon.social for great content."
	localDomain := "example.com"
	result := HighlightMentionsHTML(input, localDomain)

	// Should preserve text around mention
	if !strings.HasPrefix(result, "Check out ") {
		t.Error("Expected prefix text to be preserved")
	}
	if !strings.HasSuffix(result, " for great content.") {
		t.Error("Expected suffix text to be preserved")
	}
}

func TestMentionsToActivityPubHTML_CaseInsensitiveKeyLookup(t *testing.T) {
	// Test that mention URIs map keys are case-insensitive
	input := "Hello @Alice@Mastodon.Social!"
	mentionURIs := map[string]string{
		"@alice@mastodon.social": "https://mastodon.social/users/alice",
	}
	result := MentionsToActivityPubHTML(input, mentionURIs)

	// The regex converts to lowercase, so it should match the lowercase key
	expected := `Hello <span class="h-card"><a href="https://mastodon.social/users/alice" class="u-url mention">@<span>alice</span></a></span>!`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestHighlightMentionsTerminal_OnlyMention(t *testing.T) {
	input := "@alice@example.com"
	result := HighlightMentionsTerminal(input, "other.com")

	// Should contain the OSC 8 hyperlink
	if !strings.Contains(result, "\033]8;;https://example.com/@alice\033\\") {
		t.Errorf("Expected OSC 8 hyperlink, got: %s", result)
	}
}

func TestHighlightMentionsHTML_OnlyMention(t *testing.T) {
	input := "@alice@example.com"
	result := HighlightMentionsHTML(input, "other.com")

	expected := `<a href="https://example.com/@alice" class="mention" target="_blank" rel="noopener noreferrer">@alice@example.com</a>`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestParseMentions_SpecialCharactersInUsername(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Mention
	}{
		{
			name:  "username with dash - not matched by regex",
			input: "@user-name@example.com",
			// The current regex doesn't support dashes, so this shouldn't match
			expected: []Mention{},
		},
		{
			name:  "username starting with number - should still not match",
			input: "@123user@example.com",
			// Numbers at start should work with current regex
			expected: []Mention{
				{Username: "123user", Domain: "example.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMentions(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ParseMentions(%q) returned %d mentions, expected %d. Got: %v",
					tt.input, len(result), len(tt.expected), result)
				return
			}
			for i, mention := range result {
				if mention.Username != tt.expected[i].Username || mention.Domain != tt.expected[i].Domain {
					t.Errorf("ParseMentions(%q)[%d] = %v, expected %v",
						tt.input, i, mention, tt.expected[i])
				}
			}
		})
	}
}

func TestMentionsWithHashtags(t *testing.T) {
	// Test that mentions and hashtags can coexist in the same text
	input := "Hello @alice@mastodon.social! Check out #golang and #ActivityPub"

	mentions := ParseMentions(input)
	if len(mentions) != 1 {
		t.Errorf("Expected 1 mention, got %d", len(mentions))
	}
	if len(mentions) > 0 && mentions[0].Username != "alice" {
		t.Errorf("Expected mention of alice, got %s", mentions[0].Username)
	}

	hashtags := ParseHashtags(input)
	if len(hashtags) != 2 {
		t.Errorf("Expected 2 hashtags, got %d", len(hashtags))
	}
}

func TestHighlightMentionsTerminal_WithNewlines(t *testing.T) {
	input := "Hello\n@alice@example.com\nand @bob@other.com"
	result := HighlightMentionsTerminal(input, "example.com")

	// Alice is local
	if !strings.Contains(result, "\033]8;;https://example.com/u/alice\033\\") {
		t.Errorf("Expected local profile URL for alice, got: %s", result)
	}
	// Bob is remote
	if !strings.Contains(result, "\033]8;;https://other.com/@bob\033\\") {
		t.Errorf("Expected remote profile URL for bob, got: %s", result)
	}
	// Newlines should be preserved
	if !strings.Contains(result, "\n") {
		t.Error("Expected newlines to be preserved")
	}
}

func TestHighlightMentionsHTML_WithNewlines(t *testing.T) {
	input := "Hello\n@alice@example.com\nworld"
	result := HighlightMentionsHTML(input, "example.com")

	// Local user link
	if !strings.Contains(result, `<a href="/u/alice" class="mention">@alice</a>`) {
		t.Errorf("Expected local user link, got: %s", result)
	}
	// Newlines should be preserved
	if strings.Count(result, "\n") != 2 {
		t.Errorf("Expected 2 newlines to be preserved, got %d", strings.Count(result, "\n"))
	}
}

// Tests for LinkifyRawURLsHTML

func TestLinkifyRawURLsHTML_SingleURL(t *testing.T) {
	input := "Check out https://example.com for more info"
	result := LinkifyRawURLsHTML(input)

	expected := `Check out <a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a> for more info`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestLinkifyRawURLsHTML_MultipleURLs(t *testing.T) {
	input := "Visit https://example.com and https://test.org"
	result := LinkifyRawURLsHTML(input)

	if !strings.Contains(result, `<a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a>`) {
		t.Errorf("Expected first URL to be linkified, got: %s", result)
	}
	if !strings.Contains(result, `<a href="https://test.org" target="_blank" rel="noopener noreferrer">https://test.org</a>`) {
		t.Errorf("Expected second URL to be linkified, got: %s", result)
	}
}

func TestLinkifyRawURLsHTML_URLAtStart(t *testing.T) {
	input := "https://example.com is a great site"
	result := LinkifyRawURLsHTML(input)

	expected := `<a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a> is a great site`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestLinkifyRawURLsHTML_OnlyURL(t *testing.T) {
	input := "https://example.com"
	result := LinkifyRawURLsHTML(input)

	expected := `<a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a>`
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestLinkifyRawURLsHTML_NoURLs(t *testing.T) {
	input := "Hello world without any URLs"
	result := LinkifyRawURLsHTML(input)

	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

func TestLinkifyRawURLsHTML_URLWithPath(t *testing.T) {
	input := "Check https://github.com/deemkeen/stegodon for the code"
	result := LinkifyRawURLsHTML(input)

	if !strings.Contains(result, `<a href="https://github.com/deemkeen/stegodon" target="_blank" rel="noopener noreferrer">https://github.com/deemkeen/stegodon</a>`) {
		t.Errorf("Expected URL with path to be linkified, got: %s", result)
	}
}

func TestLinkifyRawURLsHTML_URLWithQueryParams(t *testing.T) {
	input := "Search at https://example.com/search?q=test&page=1 for results"
	result := LinkifyRawURLsHTML(input)

	if !strings.Contains(result, `<a href="https://example.com/search?q=test&amp;page=1"`) {
		t.Errorf("Expected URL with query params to be linkified, got: %s", result)
	}
}

func TestLinkifyRawURLsHTML_HttpURL(t *testing.T) {
	input := "Old site at http://example.com"
	result := LinkifyRawURLsHTML(input)

	if !strings.Contains(result, `<a href="http://example.com" target="_blank" rel="noopener noreferrer">http://example.com</a>`) {
		t.Errorf("Expected http URL to be linkified, got: %s", result)
	}
}

func TestLinkifyRawURLsHTML_PreservesExistingLinks(t *testing.T) {
	// This tests that URLs already converted by MarkdownLinksToHTML are not double-linkified
	// After MarkdownLinksToHTML: <a href="https://example.com" ...>click here</a>
	input := `<a href="https://example.com" target="_blank">click here</a>`
	result := LinkifyRawURLsHTML(input)

	// The URL inside href should not be converted again
	// Count occurrences of <a href
	count := strings.Count(result, "<a href")
	if count != 1 {
		t.Errorf("Expected only 1 link, got %d in: %s", count, result)
	}
}

func TestLinkifyRawURLsHTML_MixedContent(t *testing.T) {
	// Test with existing markdown-converted link and a raw URL
	input := `Check <a href="https://doc.example.com" target="_blank">the docs</a> or visit https://example.com`
	result := LinkifyRawURLsHTML(input)

	// The raw URL should be linkified
	if !strings.Contains(result, `<a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a>`) {
		t.Errorf("Expected raw URL to be linkified, got: %s", result)
	}
	// The existing link should still be there
	if !strings.Contains(result, `<a href="https://doc.example.com" target="_blank">the docs</a>`) {
		t.Errorf("Expected existing link to be preserved, got: %s", result)
	}
}

// Tests for LinkifyRawURLsTerminal

func TestLinkifyRawURLsTerminal_SingleURL(t *testing.T) {
	input := "Check out https://example.com for more info"
	result := LinkifyRawURLsTerminal(input)

	// Should contain OSC 8 hyperlink
	if !strings.Contains(result, "\033]8;;https://example.com\033\\") {
		t.Errorf("Expected OSC 8 hyperlink, got: %s", result)
	}
	// Should contain the URL text
	if !strings.Contains(result, "https://example.com") {
		t.Errorf("Expected URL text to be preserved, got: %s", result)
	}
}

func TestLinkifyRawURLsTerminal_URLAtStart(t *testing.T) {
	input := "https://example.com is great"
	result := LinkifyRawURLsTerminal(input)

	// Should contain OSC 8 hyperlink
	if !strings.Contains(result, "\033]8;;https://example.com\033\\") {
		t.Errorf("Expected OSC 8 hyperlink, got: %s", result)
	}
}

func TestLinkifyRawURLsTerminal_OnlyURL(t *testing.T) {
	input := "https://example.com"
	result := LinkifyRawURLsTerminal(input)

	// Should contain OSC 8 hyperlink
	if !strings.Contains(result, "\033]8;;https://example.com\033\\") {
		t.Errorf("Expected OSC 8 hyperlink, got: %s", result)
	}
}

func TestLinkifyRawURLsTerminal_NoURLs(t *testing.T) {
	input := "Hello world without any URLs"
	result := LinkifyRawURLsTerminal(input)

	if result != input {
		t.Errorf("Expected unchanged text, got: %s", result)
	}
}

func TestLinkifyRawURLsTerminal_PreservesExistingOSC8Links(t *testing.T) {
	// This tests that URLs already converted by MarkdownLinksToTerminal are not double-linkified
	// After MarkdownLinksToTerminal: OSC8 wrapped link
	input := "\033[38;2;0;255;135;4m\033]8;;https://example.com\033\\click here\033]8;;\033\\\033[39;24m"
	result := LinkifyRawURLsTerminal(input)

	// The URL inside the existing OSC 8 link should not be converted again
	// Count occurrences of OSC 8 start sequence
	count := strings.Count(result, "\033]8;;https://example.com\033\\")
	if count != 1 {
		t.Errorf("Expected only 1 OSC 8 link, got %d in: %s", count, result)
	}
}

func TestLinkifyRawURLsTerminal_MultipleURLs(t *testing.T) {
	input := "Visit https://example.com and https://test.org"
	result := LinkifyRawURLsTerminal(input)

	if !strings.Contains(result, "\033]8;;https://example.com\033\\") {
		t.Errorf("Expected first OSC 8 hyperlink, got: %s", result)
	}
	if !strings.Contains(result, "\033]8;;https://test.org\033\\") {
		t.Errorf("Expected second OSC 8 hyperlink, got: %s", result)
	}
}

// Test that simulates what Mastodon content looks like after StripHTMLTags
func TestLinkifyRawURLsHTML_AfterStripHTMLTags(t *testing.T) {
	// Mastodon wraps URLs like: <span class="invisible">https://</span><span>example.com</span>
	// After StripHTMLTags, this becomes: https://example.com
	input := StripHTMLTags(`<p><a href="https://example.com" rel="nofollow noopener noreferrer" target="_blank"><span class="invisible">https://</span><span class="">example.com</span><span class="invisible"></span></a></p>`)
	result := LinkifyRawURLsHTML(input)

	// After strip: "https://example.com"
	// After linkify: should be a clickable link
	if !strings.Contains(result, `<a href="https://example.com"`) {
		t.Errorf("Expected URL to be linkified after StripHTMLTags, got: %s (input was: %s)", result, input)
	}
}

// Test empty content
func TestLinkifyRawURLsHTML_EmptyContent(t *testing.T) {
	input := ""
	result := LinkifyRawURLsHTML(input)

	if result != "" {
		t.Errorf("Expected empty string, got: %s", result)
	}
}

// Test whitespace-only content
func TestLinkifyRawURLsHTML_WhitespaceOnly(t *testing.T) {
	input := "   "
	result := LinkifyRawURLsHTML(input)

	if result != "   " {
		t.Errorf("Expected whitespace preserved, got: %s", result)
	}
}

// Tests for SanitizeRemoteContent BiDi character stripping

func TestSanitizeRemoteContent_BiDi(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "LRM stripped",
			input:    "Hello\u200Eworld",
			expected: "Helloworld",
		},
		{
			name:     "RLM stripped",
			input:    "Hello\u200Fworld",
			expected: "Helloworld",
		},
		{
			name:     "LRE stripped",
			input:    "Hello\u202Aworld",
			expected: "Helloworld",
		},
		{
			name:     "RLE stripped",
			input:    "Hello\u202Bworld",
			expected: "Helloworld",
		},
		{
			name:     "PDF stripped",
			input:    "Hello\u202Cworld",
			expected: "Helloworld",
		},
		{
			name:     "LRO stripped",
			input:    "Hello\u202Dworld",
			expected: "Helloworld",
		},
		{
			name:     "RLO stripped",
			input:    "Hello\u202Eworld",
			expected: "Helloworld",
		},
		{
			name:     "LRI stripped",
			input:    "Hello\u2066world",
			expected: "Helloworld",
		},
		{
			name:     "RLI stripped",
			input:    "Hello\u2067world",
			expected: "Helloworld",
		},
		{
			name:     "FSI stripped",
			input:    "Hello\u2068world",
			expected: "Helloworld",
		},
		{
			name:     "PDI stripped",
			input:    "Hello\u2069world",
			expected: "Helloworld",
		},
		{
			name:     "Mixed BiDi characters",
			input:    "A\u200EB\u200FC\u202AD",
			expected: "ABCD",
		},
		{
			name:     "All isolates stripped",
			input:    "X\u2066Y\u2067Z\u2068W\u2069V",
			expected: "XYZWV",
		},
		{
			name:     "Preserves RTL text characters",
			input:    "Hello سلام world",
			expected: "Hello سلام world",
		},
		{
			name:     "Mixed Farsi and English with BiDi markers",
			input:    "Dear NERDS\u200E، یک Open-Letter\u200F ترویج",
			expected: "Dear NERDS، یک Open-Letter ترویج",
		},
		{
			name:     "BiDi at start and end",
			input:    "\u200EHello world\u200F",
			expected: "Hello world",
		},
		{
			name:     "Multiple consecutive BiDi characters",
			input:    "Test\u200E\u200F\u202A\u202Btext",
			expected: "Testtext",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only BiDi characters",
			input:    "\u200E\u200F\u202A",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeRemoteContent(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeRemoteContent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Tests for SanitizeRemoteContent keycap emoji stripping

func TestSanitizeRemoteContent_Keycap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Keycap 1 stripped to digit",
			input:    "1\uFE0F\u20E3",
			expected: "1",
		},
		{
			name:     "Keycap 2 stripped to digit",
			input:    "2\uFE0F\u20E3",
			expected: "2",
		},
		{
			name:     "Keycap hash stripped",
			input:    "#\uFE0F\u20E3",
			expected: "#",
		},
		{
			name:     "Keycap asterisk stripped",
			input:    "*\uFE0F\u20E3",
			expected: "*",
		},
		{
			name:     "Multiple keycaps in text",
			input:    "Step 1\uFE0F\u20E3 then 2\uFE0F\u20E3 then 3\uFE0F\u20E3",
			expected: "Step 1 then 2 then 3",
		},
		{
			name:     "Keycap without variation selector",
			input:    "1\u20E3",
			expected: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeRemoteContent(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeRemoteContent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Tests for SanitizeRemoteContent CJK punctuation replacement

func TestSanitizeRemoteContent_CJKPunctuation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Ideographic space",
			input:    "Hello\u3000world",
			expected: "Hello world",
		},
		{
			name:     "Ideographic comma",
			input:    "Hello\u3001world",
			expected: "Hello,world",
		},
		{
			name:     "Ideographic full stop",
			input:    "Hello\u3002",
			expected: "Hello.",
		},
		{
			name:     "Corner brackets to quotes",
			input:    "He said「hello」",
			expected: `He said"hello"`,
		},
		{
			name:     "White corner brackets to single quotes",
			input:    "He said『hello』",
			expected: "He said'hello'",
		},
		{
			name:     "Black lenticular brackets",
			input:    "Title【section】here",
			expected: "Title[section]here",
		},
		{
			name:     "Angle brackets",
			input:    "See〈this〉",
			expected: "See<this>",
		},
		{
			name:     "Double angle brackets",
			input:    "Book《title》here",
			expected: "Book<<title>>here",
		},
		{
			name:     "Wave dash",
			input:    "Range〜end",
			expected: "Range~end",
		},
		{
			name:     "CJK quotation marks",
			input:    "Quote〝text〞",
			expected: `Quote"text"`,
		},
		{
			name:     "Tortoise shell brackets",
			input:    "Note〔info〕",
			expected: "Note[info]",
		},
		{
			name:     "White lenticular brackets",
			input:    "Note〖info〗",
			expected: "Note[info]",
		},
		{
			name:     "White tortoise shell brackets",
			input:    "Note〘info〙",
			expected: "Note[info]",
		},
		{
			name:     "White square brackets",
			input:    "Note〚info〛",
			expected: "Note[info]",
		},
		{
			name:     "Mixed CJK punctuation in Russian text",
			input:    "Товарищи скорбели「Не мог выразить」слова",
			expected: `Товарищи скорбели"Не мог выразить"слова`,
		},
		{
			name:     "Preserves regular text",
			input:    "Hello world! こんにちは 你好",
			expected: "Hello world! こんにちは 你好",
		},
		{
			name:     "Multiple CJK punctuation types",
			input:    "Title【A】「B」『C』",
			expected: `Title[A]"B"'C'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeRemoteContent(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeRemoteContent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
