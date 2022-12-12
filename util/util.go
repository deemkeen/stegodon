package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"html"
	"log"
	rnd "math/rand"
	"os"
	"strings"
	"time"
)

type RsaKeyPair struct {
	Private string
	Public  string
}

func LogPublicKey(s ssh.Session) {
	log.Println(fmt.Sprintf("%s@%s opened a new ssh-session..", s.User(), s.LocalAddr()))
}

func PublicKeyToString(s ssh.PublicKey) string {
	return strings.TrimSpace(string(gossh.MarshalAuthorizedKey(s)))
}

func PkToHash(pk string) string {
	h := sha256.New()
	// TODO add a pinch of salt
	h.Write([]byte(pk))
	return hex.EncodeToString(h.Sum(nil))
}

func GetVersion() string {
	content, err := os.ReadFile("version.txt")
	if err != nil {
		log.Fatal(err)
	}
	return string(content)
}

func GetNameAndVersion() string {
	return fmt.Sprintf("%s / %s", Name, GetVersion())
}

func RandomString(length int) string {
	rnd.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rnd.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func NormalizeInput(text string) string {
	normalized := strings.Replace(text, "\n", " ", -1)
	normalized = html.EscapeString(normalized)
	return normalized
}

func DateTimeFormat() string {
	return "2006-01-02 15:04:05 CEST"
}

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", " ")
	return string(s)
}

func GeneratePemKeypair() *RsaKeyPair {
	bitSize := 4096

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		panic(err)
	}

	pub := key.Public()

	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)

	pubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(pub.(*rsa.PublicKey)),
		},
	)

	return &RsaKeyPair{Private: string(keyPEM[:]), Public: string(pubPEM[:])}
}
