package middleware

import (
	"log"
	"net"
	"strings"

	"charm.land/wish/v2"
	"github.com/charmbracelet/ssh"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
)

func AuthMiddleware(conf *util.AppConfig) wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			database := db.GetDB()

			// Check if IP or public key is banned
			remoteAddr := s.RemoteAddr().String()
			// Extract just the IP (remove port) - handles IPv4 and IPv6
			ip := extractIP(remoteAddr)

			// Check IP ban
			if database.IsIPBanned(ip) {
				log.Printf("Blocked connection from banned IP: %s", ip)
				s.Write([]byte("\n"))
				s.Write([]byte("Your IP address is banned.\n"))
				s.Write([]byte("\n"))
				s.Write([]byte("If you think this is a mistake, please contact the administrator.\n"))
				s.Write([]byte("\n"))
				s.Close()
				return
			}

			// Check public key ban
			publicKeyHash := util.PkToHash(util.PublicKeyToString(s.PublicKey()))
			if database.IsPublicKeyBanned(publicKeyHash) {
				log.Printf("Blocked connection from banned public key: %s", publicKeyHash[:16])
				s.Write([]byte("You have been banned from this server.\n"))
				s.Close()
				return
			}

			found, acc := database.ReadAccBySession(s)

			switch {
			case found == nil:
				// User exists - check if banned
				if acc != nil && acc.Banned {
					log.Printf("Blocked login attempt from banned user: %s", acc.Username)
					s.Write([]byte("You have been banned from this server.\n"))
					s.Close()
					return
				}
				// Check if muted
				if acc != nil && acc.Muted {
					log.Printf("Blocked login attempt from muted user: %s", acc.Username)
					s.Write([]byte("Your account has been muted by an administrator.\n"))
					s.Close()
					return
				}
				// Update last IP for the account
				if acc != nil {
					database.UpdateAccountLastIP(acc.Id, ip)
				}
				util.LogPublicKey(s)
			default:
				// User not found - check if registration is closed
				if conf.Conf.Closed {
					log.Printf("Rejected new user registration - registration is closed")
					s.Write([]byte("Registration is closed, but you can host your own stegodon!\n"))
					s.Write([]byte("More on: https://github.com/deemkeen/stegodon\n"))
					s.Close()
					return
				}

				// Check single-user mode
				if conf.Conf.Single {
					count, err := database.CountAccounts()
					if err != nil {
						log.Printf("Error counting accounts: %v", err)
						s.Write([]byte("An error occurred. Please try again later.\n"))
						s.Close()
						return
					}
					if count >= 1 {
						log.Printf("Rejected new user registration in single-user mode")
						s.Write([]byte("This blog is in single-user mode, but you can host your own stegodon!\n"))
						s.Write([]byte("More on: https://github.com/deemkeen/stegodon\n"))
						s.Close()
						return
					}
				}

				// Create new account
				database := db.GetDB()
				err, created := database.CreateAccount(s, util.RandomString(10))
				if err != nil {
					log.Println("Could not create a user: ", err)
				}

				if created != false {
					util.LogPublicKey(s)
					// Update last IP for the new account
					database.UpdateAccountLastIPByPkHash(publicKeyHash, ip)
				} else {
					log.Println("The user is still empty!")
				}

			}
			h(s)
		}
	}
}

// extractIP extracts the IP address from a remote address string.
// Handles IPv4, IPv6 with brackets, and raw IPv6 without port.
func extractIP(remoteAddr string) string {
	// Try net.SplitHostPort first - works for "ip:port" and "[ip]:port"
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}

	// SplitHostPort failed - check if it's bracketed IPv6 without port
	if strings.HasPrefix(remoteAddr, "[") && strings.HasSuffix(remoteAddr, "]") {
		return remoteAddr[1 : len(remoteAddr)-1]
	}

	// Otherwise return as-is (raw IPv6 or IPv4 without port)
	return remoteAddr
}
