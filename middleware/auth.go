package middleware

import (
	"github.com/charmbracelet/wish"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"github.com/gliderlabs/ssh"
	"log"
)

func AuthMiddleware() wish.Middleware {
	return func(h ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			database := db.GetDB()
			_, found := database.ReadAccBySession(s)

			switch {
			case found != nil:
				util.LogPublicKey(s)
			default:
				database := db.GetDB()
				err, created := database.CreateAccount(s, util.RandomString(10))
				if err != nil {
					log.Fatalln("Could not create a user: ", err)
				}

				if created != false {
					util.LogPublicKey(s)
				} else {
					log.Fatalln("The user is still empty!")
				}

			}
			h(s)
		}
	}
}
