package domain

import "time"

// Ban represents a banned user with their IP address and SSH public key
type Ban struct {
	Id            string    `json:"id"`              // Account ID of banned user
	Username      string    `json:"username"`        // Username for reference
	IPAddress     string    `json:"ip_address"`      // IP address at time of ban
	PublicKeyHash string    `json:"public_key_hash"` // SHA256 hash of SSH public key
	Reason        string    `json:"reason"`          // Reason for ban
	BannedAt      time.Time `json:"banned_at"`       // When the ban was created
}
