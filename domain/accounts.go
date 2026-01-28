package domain

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

const (
	FALSE dbBool = iota
	TRUE
)

type dbBool uint

type Account struct {
	Id             uuid.UUID
	Username       string
	Publickey      string
	CreatedAt      time.Time
	FirstTimeLogin dbBool
	WebPublicKey   string
	WebPrivateKey  string
	// ActivityPub fields
	DisplayName string
	Summary     string
	AvatarURL   string
	// Admin fields
	IsAdmin bool
	Muted   bool
	Banned  bool
	// Connection tracking
	LastIP string
}

func (acc *Account) ToString() string {
	return fmt.Sprintf("\n\tId: %s \n\tUsername: %s \n\tPublickey: %s \n\tCREATED_AT: %s)", acc.Id, acc.Username, acc.Publickey, acc.CreatedAt)
}

// Terms and Conditions
type TermsAndConditions struct {
	Id        int
	Content   string
	UpdatedAt time.Time
}

type UserTermsAcceptance struct {
	Id         int
	UserId     uuid.UUID
	TermsId    int
	AcceptedAt time.Time
}
