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
}

func (acc *Account) ToString() string {
	return fmt.Sprintf("\n\tId: %s \n\tUsername: %s \n\tPublickey: %s \n\tCREATED_AT: %s)", acc.Id, acc.Username, acc.Publickey, acc.CreatedAt)
}
