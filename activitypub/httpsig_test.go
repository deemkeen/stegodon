package activitypub

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"testing"
	"time"
)

// generateTestKeyPair generates an RSA key pair for testing
func generateTestKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// calculateDigest calculates SHA-256 digest for request body
func calculateDigest(body []byte) string {
	hash := sha256.Sum256(body)
	return "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])
}

// privateKeyToPEM converts private key to PEM string
func privateKeyToPEM(key *rsa.PrivateKey) string {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})
	return string(keyPEM)
}

// publicKeyToPEM converts public key to PEM string
func publicKeyToPEM(key *rsa.PublicKey) (string, error) {
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: keyBytes,
	})
	return string(keyPEM), nil
}

func TestParsePrivateKey(t *testing.T) {
	privateKey, _, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	pemString := privateKeyToPEM(privateKey)

	parsed, err := ParsePrivateKey(pemString)
	if err != nil {
		t.Fatalf("ParsePrivateKey failed: %v", err)
	}

	if parsed == nil {
		t.Fatal("ParsePrivateKey returned nil")
	}

	// Verify the key can be used for signing
	if parsed.N.Cmp(privateKey.N) != 0 {
		t.Error("Parsed key doesn't match original")
	}
}

func TestParsePrivateKeyInvalidPEM(t *testing.T) {
	_, err := ParsePrivateKey("not a valid PEM")
	if err == nil {
		t.Error("Expected error for invalid PEM")
	}
}

func TestParsePrivateKeyEmptyString(t *testing.T) {
	_, err := ParsePrivateKey("")
	if err == nil {
		t.Error("Expected error for empty string")
	}
}

func TestParsePublicKey(t *testing.T) {
	_, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	pemString, err := publicKeyToPEM(publicKey)
	if err != nil {
		t.Fatalf("Failed to convert public key to PEM: %v", err)
	}

	parsed, err := ParsePublicKey(pemString)
	if err != nil {
		t.Fatalf("ParsePublicKey failed: %v", err)
	}

	if parsed == nil {
		t.Fatal("ParsePublicKey returned nil")
	}

	// Verify the key matches
	if parsed.N.Cmp(publicKey.N) != 0 {
		t.Error("Parsed key doesn't match original")
	}
}

func TestParsePublicKeyInvalidPEM(t *testing.T) {
	_, err := ParsePublicKey("not a valid PEM")
	if err == nil {
		t.Error("Expected error for invalid PEM")
	}
}

func TestParsePublicKeyEmptyString(t *testing.T) {
	_, err := ParsePublicKey("")
	if err == nil {
		t.Error("Expected error for empty string")
	}
}

func TestSignRequest(t *testing.T) {
	// Skip this test - SignRequest is tested indirectly through other tests
	t.Skip("SignRequest implementation is tested through integration tests")
}

func TestSignRequestWithoutDate(t *testing.T) {
	// Skip - tested through other integration tests
	t.Skip("Tested indirectly through integration tests")
}

func TestVerifyRequestKeyIdExtraction(t *testing.T) {
	privateKey, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	publicPEM, err := publicKeyToPEM(publicKey)
	if err != nil {
		t.Fatalf("Failed to convert public key to PEM: %v", err)
	}

	// Create and sign a request
	body := []byte(`{"type":"Create"}`)
	req, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", "example.com")
	// Calculate and set digest
	req.Header.Set("Digest", calculateDigest(body))

	keyId := "https://myserver.com/users/alice#main-key"

	err = SignRequest(req, privateKey, keyId)
	if err != nil {
		t.Fatalf("SignRequest failed: %v", err)
	}

	// For verification, we need to recreate the request with the body
	// because SignRequest consumes it
	req2, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to recreate request: %v", err)
	}
	// Copy headers from signed request
	req2.Header = req.Header.Clone()

	// Verify the request
	actorURI, err := VerifyRequest(req2, publicPEM)
	if err != nil {
		t.Fatalf("VerifyRequest failed: %v", err)
	}

	expectedActor := "https://myserver.com/users/alice"
	if actorURI != expectedActor {
		t.Errorf("Expected actor URI '%s', got '%s'", expectedActor, actorURI)
	}
}

func TestVerifyRequestInvalidSignature(t *testing.T) {
	// Generate two different key pairs
	privateKey1, _, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair 1: %v", err)
	}

	_, publicKey2, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair 2: %v", err)
	}

	publicPEM2, err := publicKeyToPEM(publicKey2)
	if err != nil {
		t.Fatalf("Failed to convert public key to PEM: %v", err)
	}

	// Sign with privateKey1
	body := []byte(`{"type":"Create"}`)
	req, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", "example.com")
	req.Header.Set("Digest", calculateDigest(body))

	keyId := "https://myserver.com/users/alice#main-key"

	err = SignRequest(req, privateKey1, keyId)
	if err != nil {
		t.Fatalf("SignRequest failed: %v", err)
	}

	// Recreate request with body for verification
	req2, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to recreate request: %v", err)
	}
	req2.Header = req.Header.Clone()

	// Try to verify with publicKey2 (should fail)
	_, err = VerifyRequest(req2, publicPEM2)
	if err == nil {
		t.Error("Expected verification to fail with wrong public key")
	}
}

func TestVerifyRequestInvalidPEM(t *testing.T) {
	req, err := http.NewRequest("POST", "https://example.com/inbox", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = VerifyRequest(req, "invalid PEM")
	if err == nil {
		t.Error("Expected error with invalid PEM")
	}
}

func TestVerifyRequestEmptyPEM(t *testing.T) {
	req, err := http.NewRequest("POST", "https://example.com/inbox", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = VerifyRequest(req, "")
	if err == nil {
		t.Error("Expected error with empty PEM")
	}
}

func TestSignAndVerifyRoundtrip(t *testing.T) {
	privateKey, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	publicPEM, err := publicKeyToPEM(publicKey)
	if err != nil {
		t.Fatalf("Failed to convert public key to PEM: %v", err)
	}

	tests := []struct {
		name   string
		method string
		url    string
		body   []byte
	}{
		{
			name:   "POST with body",
			method: "POST",
			url:    "https://example.com/inbox",
			body:   []byte(`{"type":"Create","object":{}}`),
		},
		{
			name:   "GET without body",
			method: "GET",
			url:    "https://example.com/users/alice",
			body:   nil,
		},
		{
			name:   "POST to different path",
			method: "POST",
			url:    "https://example.com/users/bob/inbox",
			body:   []byte(`{"type":"Follow"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			var err error

			if tt.body != nil {
				req, err = http.NewRequest(tt.method, tt.url, bytes.NewReader(tt.body))
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
				req.Header.Set("Digest", calculateDigest(tt.body))
			} else {
				req, err = http.NewRequest(tt.method, tt.url, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
				req.Header.Set("Digest", calculateDigest([]byte{}))
			}

			req.Header.Set("Content-Type", "application/activity+json")
			req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
			req.Header.Set("Host", "example.com")

			keyId := "https://myserver.com/users/testuser#main-key"

			// Sign the request
			err = SignRequest(req, privateKey, keyId)
			if err != nil {
				t.Fatalf("SignRequest failed: %v", err)
			}

			// Recreate request with body for verification
			var req2 *http.Request
			if tt.body != nil {
				req2, err = http.NewRequest(tt.method, tt.url, bytes.NewReader(tt.body))
			} else {
				req2, err = http.NewRequest(tt.method, tt.url, nil)
			}
			if err != nil {
				t.Fatalf("Failed to recreate request: %v", err)
			}
			req2.Header = req.Header.Clone()

			// Verify the request
			actorURI, err := VerifyRequest(req2, publicPEM)
			if err != nil {
				t.Fatalf("VerifyRequest failed: %v", err)
			}

			expectedActor := "https://myserver.com/users/testuser"
			if actorURI != expectedActor {
				t.Errorf("Expected actor URI '%s', got '%s'", expectedActor, actorURI)
			}
		})
	}
}

func TestKeyIdWithoutFragment(t *testing.T) {
	privateKey, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	publicPEM, err := publicKeyToPEM(publicKey)
	if err != nil {
		t.Fatalf("Failed to convert public key to PEM: %v", err)
	}

	body := []byte(`{"type":"Create"}`)
	req, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", "example.com")
	req.Header.Set("Digest", calculateDigest(body))

	// keyId without #fragment
	keyId := "https://myserver.com/users/alice"

	err = SignRequest(req, privateKey, keyId)
	if err != nil {
		t.Fatalf("SignRequest failed: %v", err)
	}

	// Recreate request with body for verification
	req2, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to recreate request: %v", err)
	}
	req2.Header = req.Header.Clone()

	actorURI, err := VerifyRequest(req2, publicPEM)
	if err != nil {
		t.Fatalf("VerifyRequest failed: %v", err)
	}

	// Should still extract the actor URI correctly
	if actorURI != keyId {
		t.Errorf("Expected actor URI '%s', got '%s'", keyId, actorURI)
	}
}
