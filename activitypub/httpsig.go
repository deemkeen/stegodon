package activitypub

import (
	"code.superseriousbusiness.org/httpsig"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
)

// SignRequest signs an outgoing HTTP request with the given private key
// keyId format: "https://example.com/users/alice#main-key"
func SignRequest(req *http.Request, privateKey *rsa.PrivateKey, keyId string) error {
	// Create signer with required headers
	signer, err := httpsig.NewSigner(
		httpsig.RSA_SHA256,
		httpsig.DigestSha256,
		[]string{"(request-target)", "host", "date", "digest"},
		httpsig.Signature,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	// Sign the request
	return signer.SignRequest(privateKey, keyId, req, nil)
}

// VerifyRequest verifies the HTTP signature on an incoming request
// Returns the actor URI if valid, error otherwise
func VerifyRequest(req *http.Request, publicKeyPem string) (string, error) {
	// Create verifier from request
	verifier, err := httpsig.NewVerifier(req)
	if err != nil {
		return "", fmt.Errorf("failed to create verifier: %w", err)
	}

	// Parse public key from PEM
	block, _ := pem.Decode([]byte(publicKeyPem))
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("not an RSA public key")
	}

	// Verify the signature
	keyId, err := verifier.Verify(rsaPubKey, httpsig.RSA_SHA256)
	if err != nil {
		return "", fmt.Errorf("signature verification failed: %w", err)
	}

	// Extract actor URI from keyId
	// keyId is usually "https://example.com/users/alice#main-key"
	// We want "https://example.com/users/alice"
	actorURI := strings.Split(keyId, "#")[0]

	return actorURI, nil
}

// ParsePrivateKey converts PEM string to *rsa.PrivateKey
func ParsePrivateKey(pemString string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// ParsePublicKey converts PEM string to *rsa.PublicKey
func ParsePublicKey(pemString string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPubKey, nil
}
