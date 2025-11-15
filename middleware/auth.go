package middleware

import (
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"log"
)

func AuthMiddleware() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			database := db.GetDB()
			found, acc := database.ReadAccBySession(s)

			switch {
			case found == nil:
				// User exists - check if muted
				if acc != nil && acc.Muted {
					log.Printf("Blocked login attempt from muted user: %s", acc.Username)
					s.Write([]byte("Your account has been muted by an administrator.\n"))
					s.Close()
					return
				}
				util.LogPublicKey(s)
			default:
				// User not found - create new account
				database := db.GetDB()
				err, created := database.CreateAccount(s, util.RandomString(10))
				if err != nil {
					log.Println("Could not create a user: ", err)
				}

				if created != false {
					util.LogPublicKey(s)
				} else {
					log.Println("The user is still empty!")
				}

			}
			h(s)
		}
	}
}
